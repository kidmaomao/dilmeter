package packet

import (
	"regexp"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Character-condition metadata has appeared in several text encodings across
// regions and client revisions. Keep the raw metadata and extract only the
// duration values whose meaning is established. DUR has priority over SDUR.
var conditionDurationPatterns = []*regexp.Regexp{
	// CN typed metadata uses key:type:value (for example SDUR:4:300000).
	// The middle number is the value type, not the duration.
	regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])DUR["']?\s*[=:]\s*[0-9]+\s*:\s*([0-9]+)`),
	regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])DUR["']?\s*[=:]\s*["']?([0-9]+)`),
	regexp.MustCompile(`(?i)<DUR>\s*([0-9]+)\s*</DUR>`),
	regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])SDUR["']?\s*[=:]\s*[0-9]+\s*:\s*([0-9]+)`),
	regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])SDUR["']?\s*[=:]\s*["']?([0-9]+)`),
	regexp.MustCompile(`(?i)<SDUR>\s*([0-9]+)\s*</SDUR>`),
}

var (
	conditionAppliedTimePattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])MCAGT:8:([0-9]+)`)
	conditionEndTimePattern     = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])SBT:8:([0-9]+)`)
	conditionServerClockLeadMs  atomic.Int64
	conditionClockCalibration   = struct {
		sync.Mutex
		stableSamples    []int64
		jumpSamples      []int64
		lastSampleAtMs   int64
		recentSampleKeys []int64
	}{}
)

const (
	dotNetUnixEpochMs = int64(62135596800000)
	cnUTCOffsetMs     = int64(8 * 60 * 60 * 1000)
	maxConditionMs    = int64(7 * 24 * 60 * 60 * 1000)

	conditionClockFallbackMs       = int64(10000)
	conditionClockMaxAbsLeadMs     = int64(15 * 60 * 1000)
	conditionClockSampleTolerance  = int64(3000)
	conditionClockStableWindowSize = 7
	conditionClockJumpConfirmCount = 3
	conditionClockStaleAfterMs     = int64(5 * 60 * 1000)
	conditionClockRecentKeyLimit   = 64
	conditionClockSampleKeyNearMs  = int64(500)
)

func init() {
	// Used only until the first trusted live condition packet arrives. Every
	// subsequent fresh Buff contributes to dynamic calibration.
	resetConditionServerClockCalibration(conditionClockFallbackMs)
}

func resetConditionServerClockCalibration(fallbackMs int64) {
	conditionClockCalibration.Lock()
	conditionClockCalibration.stableSamples = nil
	conditionClockCalibration.jumpSamples = nil
	conditionClockCalibration.lastSampleAtMs = 0
	conditionClockCalibration.recentSampleKeys = nil
	conditionServerClockLeadMs.Store(fallbackMs)
	conditionClockCalibration.Unlock()
}

func medianConditionClockSample(samples []int64) int64 {
	ordered := append([]int64(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	return ordered[len(ordered)/2]
}

func conditionClockDistance(a, b int64) int64 {
	if a >= b {
		return a - b
	}
	return b - a
}

func conditionClockJumpClusterFits(samples []int64, candidateMs int64) bool {
	minValue, maxValue := candidateMs, candidateMs
	for _, sample := range samples {
		if sample < minValue {
			minValue = sample
		}
		if sample > maxValue {
			maxValue = sample
		}
	}
	return maxValue-minValue <= conditionClockSampleTolerance
}

func appendConditionClockSample(samples []int64, sample int64, limit int) []int64 {
	samples = append(samples, sample)
	if len(samples) > limit {
		return append([]int64(nil), samples[len(samples)-limit:]...)
	}
	return samples
}

// observeConditionServerClockLead keeps a rolling median of trusted fresh-Buff
// samples. One delayed or malformed packet cannot move the active calibration.
// If maintenance or clock synchronization causes a real jump, three mutually
// consistent new samples atomically replace the old calibration. A long idle
// period clears unfinished jump votes, but never lets one packet replace a
// known-good calibration by itself.
func observeConditionServerClockLead(candidateMs int64, observedAt time.Time) {
	observeConditionServerClockLeadSample(candidateMs, observedAt, observedAt.UnixMilli())
}

func observeConditionServerClockLeadSample(candidateMs int64, observedAt time.Time, sampleKey int64) {
	if observedAt.IsZero() {
		return
	}
	if candidateMs < -conditionClockMaxAbsLeadMs || candidateMs > conditionClockMaxAbsLeadMs {
		return
	}
	observedAtMs := observedAt.UnixMilli()

	conditionClockCalibration.Lock()
	defer conditionClockCalibration.Unlock()
	if sampleKey != 0 {
		for _, recentKey := range conditionClockCalibration.recentSampleKeys {
			if conditionClockDistance(sampleKey, recentKey) <= conditionClockSampleKeyNearMs {
				return
			}
		}
		conditionClockCalibration.recentSampleKeys = append(conditionClockCalibration.recentSampleKeys, sampleKey)
		if len(conditionClockCalibration.recentSampleKeys) > conditionClockRecentKeyLimit {
			conditionClockCalibration.recentSampleKeys = conditionClockCalibration.recentSampleKeys[1:]
		}
	}

	lastSampleAtMs := conditionClockCalibration.lastSampleAtMs
	if len(conditionClockCalibration.stableSamples) == 0 {
		conditionClockCalibration.stableSamples = []int64{candidateMs}
		conditionClockCalibration.jumpSamples = nil
		conditionClockCalibration.lastSampleAtMs = observedAtMs
		conditionServerClockLeadMs.Store(candidateMs)
		return
	}
	if lastSampleAtMs > 0 && observedAtMs-lastSampleAtMs >= conditionClockStaleAfterMs {
		conditionClockCalibration.jumpSamples = nil
	}
	if observedAtMs > lastSampleAtMs {
		conditionClockCalibration.lastSampleAtMs = observedAtMs
	}

	stableMedian := medianConditionClockSample(conditionClockCalibration.stableSamples)
	if conditionClockDistance(candidateMs, stableMedian) <= conditionClockSampleTolerance {
		conditionClockCalibration.stableSamples = appendConditionClockSample(
			conditionClockCalibration.stableSamples,
			candidateMs,
			conditionClockStableWindowSize,
		)
		conditionClockCalibration.jumpSamples = nil
		conditionServerClockLeadMs.Store(medianConditionClockSample(conditionClockCalibration.stableSamples))
		return
	}

	if !conditionClockJumpClusterFits(conditionClockCalibration.jumpSamples, candidateMs) {
		conditionClockCalibration.jumpSamples = nil
	}
	conditionClockCalibration.jumpSamples = appendConditionClockSample(
		conditionClockCalibration.jumpSamples,
		candidateMs,
		conditionClockJumpConfirmCount,
	)
	if len(conditionClockCalibration.jumpSamples) < conditionClockJumpConfirmCount {
		return
	}

	conditionClockCalibration.stableSamples = append([]int64(nil), conditionClockCalibration.jumpSamples...)
	conditionClockCalibration.jumpSamples = nil
	conditionServerClockLeadMs.Store(medianConditionClockSample(conditionClockCalibration.stableSamples))
}

func deriveConditionServerClockLead(
	appliedAt int64,
	appliedOK bool,
	endAt int64,
	endOK bool,
	disableAt int64,
	durationMs int64,
	observedAt time.Time,
) (int64, int64, bool) {
	if observedAt.IsZero() {
		return 0, 0, false
	}
	observedAtMs := observedAt.UnixMilli()
	type keyedCandidate struct {
		leadMs int64
		key    int64
	}
	candidates := make([]keyedCandidate, 0, 3)
	if appliedOK {
		candidates = append(candidates, keyedCandidate{
			leadMs: appliedAt - dotNetUnixEpochMs - cnUTCOffsetMs - observedAtMs,
			key:    appliedAt - dotNetUnixEpochMs - cnUTCOffsetMs,
		})
	}
	if durationMs > 0 {
		if endOK {
			candidates = append(candidates, keyedCandidate{
				leadMs: endAt - dotNetUnixEpochMs - cnUTCOffsetMs - observedAtMs - durationMs,
				key:    endAt - dotNetUnixEpochMs - cnUTCOffsetMs - durationMs,
			})
		}
		if disableAt > 0 {
			candidates = append(candidates, keyedCandidate{
				leadMs: disableAt*1000 - observedAtMs - durationMs,
				key:    disableAt*1000 - durationMs,
			})
		}
	}
	for _, candidate := range candidates {
		if candidate.leadMs >= -conditionClockMaxAbsLeadMs && candidate.leadMs <= conditionClockMaxAbsLeadMs {
			return candidate.leadMs, candidate.key, true
		}
	}
	return 0, 0, false
}

func parseConditionTimestampMs(pattern *regexp.Regexp, metadata string) (int64, bool) {
	match := pattern.FindStringSubmatch(metadata)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.ParseInt(match[1], 10, 64)
	if err != nil || value <= 0 {
		return 0, false
	}
	// Some client revisions expose full 100 ns .NET ticks instead of ms.
	if value > 1000000000000000 {
		value /= 10000
	}
	return value, true
}

func parseConditionDurationMs(metadata string) int64 {
	for _, pattern := range conditionDurationPatterns {
		match := pattern.FindStringSubmatch(metadata)
		if len(match) != 2 {
			continue
		}
		value, err := strconv.ParseInt(match[1], 10, 64)
		if err != nil || value <= 0 || value > maxConditionMs {
			return 0
		}
		return value
	}

	// CN condition packets commonly omit DUR/SDUR but carry two .NET-style
	// timestamps in milliseconds: MCAGT is the application time and SBT is the
	// scheduled end time. Their difference is the Buff duration and is immune
	// to the server/local timezone offset that makes DisableAt unreliable.
	appliedAt, appliedOK := parseConditionTimestampMs(conditionAppliedTimePattern, metadata)
	endAt, endOK := parseConditionTimestampMs(conditionEndTimePattern, metadata)
	if appliedOK && endOK {
		duration := endAt - appliedAt
		if duration > 0 && duration <= maxConditionMs {
			return duration
		}
	}
	return 0
}

// parseConditionDurationAtMs also handles CN condition packets that only
// include SBT. In that form the parsed DisableAt is the authoritative expiry;
// compare it with the packet capture time to obtain the remaining duration.
func parseConditionDurationAtMs(metadata string, observedAt time.Time, disableAt int64) int64 {
	appliedAt, appliedOK := parseConditionTimestampMs(conditionAppliedTimePattern, metadata)
	endAt, endOK := parseConditionTimestampMs(conditionEndTimePattern, metadata)
	duration := parseConditionDurationMs(metadata)
	if candidate, sampleKey, ok := deriveConditionServerClockLead(
		appliedAt,
		appliedOK,
		endAt,
		endOK,
		disableAt,
		duration,
		observedAt,
	); ok {
		observeConditionServerClockLeadSample(candidate, observedAt, sampleKey)
	}

	if duration > 0 {
		return duration
	}
	if observedAt.IsZero() {
		return 0
	}

	lead := conditionServerClockLeadMs.Load()
	if endOK {
		remaining := endAt - dotNetUnixEpochMs - cnUTCOffsetMs - observedAt.UnixMilli() - lead
		if remaining > 0 && remaining <= maxConditionMs {
			return remaining
		}
	}
	if disableAt <= 0 {
		return 0
	}
	remaining := disableAt*1000 - observedAt.UnixMilli() - lead
	if remaining > 0 && remaining <= maxConditionMs {
		return remaining
	}
	return 0
}

// normalizeConditionDisableAt converts the server's timezone-less absolute
// timestamp into the local Unix clock used by the UI. CN condition timestamps
// can differ from the PC clock by several seconds; using the raw value makes
// every countdown consistently early or late even when DurationMs is correct.
//
// The server-clock lead is calibrated from MCAGT on live condition packets.
// Snapshot and reconnect paths then reuse the same calibration so they retain
// the original absolute expiry instead of restarting a full Buff duration.
func normalizeConditionDisableAt(disableAt int64) int64 {
	if disableAt <= 0 {
		return 0
	}
	return conditionDisableAtSeconds(normalizeConditionDisableAtMs(disableAt * 1000))
}

// normalizeConditionDisableAtMs is the millisecond-precision counterpart used
// by new events. The second-precision wrapper remains for old logs and callers.
func normalizeConditionDisableAtMs(disableAtMs int64) int64 {
	if disableAtMs <= 0 {
		return 0
	}

	correctedMs := disableAtMs - conditionServerClockLeadMs.Load()
	if correctedMs <= 0 {
		return 0
	}
	return correctedMs
}

// A freshly received enable packet can use its relative duration directly and
// is therefore independent of both server clock offset and calibration state.
func conditionDisableAtFromDuration(observedAt time.Time, durationMs int64) int64 {
	return conditionDisableAtSeconds(conditionDisableAtMsFromDuration(observedAt, durationMs))
}

func conditionDisableAtMsFromDuration(observedAt time.Time, durationMs int64) int64 {
	if observedAt.IsZero() || durationMs <= 0 || durationMs > maxConditionMs {
		return 0
	}
	return observedAt.UnixMilli() + durationMs
}

func conditionDisableAtSeconds(disableAtMs int64) int64 {
	if disableAtMs <= 0 {
		return 0
	}
	// Whole-second consumers keep the historical non-early-expiry guarantee.
	return (disableAtMs + 999) / 1000
}
