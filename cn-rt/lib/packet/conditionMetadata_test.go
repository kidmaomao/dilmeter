package packet

import (
	"sync"
	"testing"
	"time"
)

func resetConditionClockForTest(t *testing.T, fallbackMs int64) {
	t.Helper()
	resetConditionServerClockCalibration(fallbackMs)
	t.Cleanup(func() {
		resetConditionServerClockCalibration(conditionClockFallbackMs)
	})
}

func TestParseConditionDurationMs(t *testing.T) {
	tests := []struct {
		name     string
		metadata string
		want     int64
	}{
		{name: "key value", metadata: "DUR=60000", want: 60000},
		{name: "json like", metadata: `{"DUR":"180000","SDUR":1}`, want: 180000},
		{name: "xml", metadata: "<DUR>300000</DUR>", want: 300000},
		{name: "server duration fallback", metadata: "SDUR: 10000", want: 10000},
		{
			name:     "cn typed server duration",
			metadata: "SSAD:f:10;MCAGT:8:63921310753919;SDUR:4:300000;SBT:8:63921311053919;",
			want:     300000,
		},
		{name: "cn typed duration priority", metadata: "SDUR:4:300000;DUR:4:120000", want: 120000},
		{name: "duration priority", metadata: "SDUR=10000;DUR=20000", want: 20000},
		{
			name:     "cn metadata timestamp pair",
			metadata: "MCMBAMIN:f:39.600002;MCAGT:8:63920974340598;MCMBAMAX:f:39.600002;SBT:8:63920974453598;",
			want:     113000,
		},
		{
			name:     "full dotnet ticks timestamp pair",
			metadata: "MCAGT:8:639209743405980000;SBT:8:639209744535980000;",
			want:     113000,
		},
		{name: "invalid timestamp order", metadata: "MCAGT:8:2000;SBT:8:1000;", want: 0},
		{name: "duration over seven days", metadata: "DUR:4:604800001;", want: 0},
		{name: "missing", metadata: "MCSTCT=3", want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseConditionDurationMs(tt.metadata); got != tt.want {
				t.Fatalf("parseConditionDurationMs(%q) = %d, want %d", tt.metadata, got, tt.want)
			}
		})
	}
}

func TestParseConditionDurationAtMs(t *testing.T) {
	resetConditionClockForTest(t, 10000)
	observedAt := time.UnixMilli(1785349080000)
	if got := parseConditionDurationAtMs("SBT:8:63920974748990;", observedAt, 1785349148); got != 58990 {
		t.Fatalf("single SBT duration = %d, want 58990", got)
	}
	if got := parseConditionDurationAtMs("SBT:8:63920974748990;", time.Unix(1785349150, 0), 1785349079); got != 0 {
		t.Fatalf("expired single SBT duration = %d, want 0", got)
	}
}

func TestConditionServerClockCalibration(t *testing.T) {
	resetConditionClockForTest(t, 10000)
	observedAt := time.UnixMilli(1785348731000)
	metadata := "MCAGT:8:63920974340598;SBT:8:63920974453598;"
	if got := parseConditionDurationAtMs(metadata, observedAt, 0); got != 113000 {
		t.Fatalf("paired timestamp duration = %d, want 113000", got)
	}
	if got := conditionServerClockLeadMs.Load(); got != 9598 {
		t.Fatalf("calibrated server lead = %d, want 9598", got)
	}
}
func TestNormalizeConditionDisableAtWithLiveCNPackets(t *testing.T) {
	tests := []struct {
		name       string
		observedAt int64
		disableAt  int64
		metadata   string
		wantMs     int64
		wantExpiry int64
	}{
		{
			name:       "battle overture",
			observedAt: 1785847459,
			disableAt:  1785847557,
			metadata:   "MCMBAMIN:f:39.600002;MCAGT:8:63921473044130;MCMBAMAX:f:39.600002;MCMBAC:f:21.779999;MCMBGA:b:false;SBT:8:63921473157130;",
			wantMs:     113000,
			wantExpiry: 1785847572,
		},
		{
			name:       "condition support sharp",
			observedAt: 1785847479,
			disableAt:  1785847763,
			metadata:   "SSAD:f:10;SSCRI:f:10;SSRT:f:20;MCAGT:8:63921473063398;SDUR:4:300000;SIPM:b:true;SBT:8:63921473363398;",
			wantMs:     300000,
			wantExpiry: 1785847779,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetConditionClockForTest(t, 0)
			if got := parseConditionDurationAtMs(tt.metadata, time.Unix(tt.observedAt, 0), tt.disableAt); got != tt.wantMs {
				t.Fatalf("duration = %d, want %d", got, tt.wantMs)
			}
			if got := normalizeConditionDisableAt(tt.disableAt); got != tt.wantExpiry {
				t.Fatalf("normalized expiry = %d, want %d", got, tt.wantExpiry)
			}
		})
	}
}

func TestNormalizeConditionDisableAtKeepsSnapshotExpiry(t *testing.T) {
	resetConditionClockForTest(t, 0)
	metadata := "MCMBAMIN:f:39.600002;MCAGT:8:63921473044130;MCMBAMAX:f:39.600002;SBT:8:63921473157130;"
	if got := parseConditionDurationAtMs(metadata, time.Unix(1785847459, 0), 1785847557); got != 113000 {
		t.Fatalf("live duration = %d, want 113000", got)
	}

	// A snapshot observed 40 seconds later must retain the original absolute
	// expiry instead of starting another full 113-second countdown.
	const snapshotObservedAt = int64(1785847499)
	const wantExpiry = int64(1785847572)
	if got := normalizeConditionDisableAt(1785847557); got != wantExpiry {
		t.Fatalf("snapshot expiry = %d, want %d (remaining %d seconds)", got, wantExpiry, wantExpiry-snapshotObservedAt)
	}
}

func TestConditionClockRollingMedianRejectsOutlier(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	for i, sample := range []int64{9600, 9800, 9400, 9700, 9500} {
		observeConditionServerClockLead(sample, base.Add(time.Duration(i)*time.Second))
	}
	if got := conditionServerClockLeadMs.Load(); got != 9600 {
		t.Fatalf("stable median = %d, want 9600", got)
	}

	observeConditionServerClockLead(45000, base.Add(6*time.Second))
	if got := conditionServerClockLeadMs.Load(); got != 9600 {
		t.Fatalf("one outlier changed lead to %d, want 9600", got)
	}
	observeConditionServerClockLead(9550, base.Add(7*time.Second))
	if len(conditionClockCalibration.jumpSamples) != 0 {
		t.Fatalf("stable sample did not clear pending jump: %v", conditionClockCalibration.jumpSamples)
	}
}

func TestConditionClockUsesRecentSampleWindow(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	for i := 0; i < 10; i++ {
		observeConditionServerClockLead(int64(10000+i*100), base.Add(time.Duration(i)*time.Second))
	}
	if got := len(conditionClockCalibration.stableSamples); got != conditionClockStableWindowSize {
		t.Fatalf("stable window length = %d, want %d", got, conditionClockStableWindowSize)
	}
	if got := conditionServerClockLeadMs.Load(); got != 10600 {
		t.Fatalf("recent-window median = %d, want 10600", got)
	}
}

func TestConditionClockRequiresConsensusForJump(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	for i, sample := range []int64{-15000, -14900, -15100} {
		observeConditionServerClockLead(sample, base.Add(time.Duration(i)*time.Second))
	}
	if got := conditionServerClockLeadMs.Load(); got != -15000 {
		t.Fatalf("initial median = %d, want -15000", got)
	}

	for i, sample := range []int64{20000, 20500} {
		observeConditionServerClockLead(sample, base.Add(time.Duration(i+3)*time.Second))
		if got := conditionServerClockLeadMs.Load(); got != -15000 {
			t.Fatalf("jump switched after %d samples: lead=%d", i+1, got)
		}
	}
	observeConditionServerClockLead(19800, base.Add(5*time.Second))
	if got := conditionServerClockLeadMs.Load(); got != 20000 {
		t.Fatalf("confirmed jump lead = %d, want 20000", got)
	}
}

func TestConditionClockJumpIgnoresDuplicateGroupBuffPackets(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	observeConditionServerClockLeadSample(-15000, base, 1000)

	for i := 0; i < conditionClockJumpConfirmCount+2; i++ {
		observeConditionServerClockLeadSample(20000, base.Add(time.Duration(i+1)*time.Second), 2000)
	}
	if got := conditionServerClockLeadMs.Load(); got != -15000 {
		t.Fatalf("duplicate group Buff packets switched lead to %d", got)
	}
	if got := len(conditionClockCalibration.jumpSamples); got != 1 {
		t.Fatalf("duplicate group Buff count = %d, want 1", got)
	}

	observeConditionServerClockLeadSample(20100, base.Add(10*time.Second), 3000)
	observeConditionServerClockLeadSample(19900, base.Add(11*time.Second), 4000)
	if got := conditionServerClockLeadMs.Load(); got != 20000 {
		t.Fatalf("three distinct application times produced lead %d, want 20000", got)
	}
}

func TestConditionClockDeduplicatesNearSimultaneousGroupBuffPackets(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	observedAt := time.UnixMilli(1800000000000)
	baseAppliedAt := dotNetUnixEpochMs + cnUTCOffsetMs + observedAt.UnixMilli() + 10499
	lead1, key1, ok1 := deriveConditionServerClockLead(baseAppliedAt, true, 0, false, 0, 0, observedAt)
	lead2, key2, ok2 := deriveConditionServerClockLead(baseAppliedAt+2, true, 0, false, 0, 0, observedAt.Add(2*time.Millisecond))
	if !ok1 || !ok2 || lead1 != 10499 || lead2 != 10499 {
		t.Fatalf("group samples not derived: (%d,%v) (%d,%v)", lead1, ok1, lead2, ok2)
	}
	observeConditionServerClockLeadSample(lead1, observedAt, key1)
	observeConditionServerClockLeadSample(lead2, observedAt.Add(2*time.Millisecond), key2)
	if got := len(conditionClockCalibration.stableSamples); got != 1 {
		t.Fatalf("near-simultaneous group Buff count = %d, want 1", got)
	}
}

func TestConditionClockUsesCanonicalApplicationTimeAcrossSources(t *testing.T) {
	observedAt := time.UnixMilli(1800000000000)
	const durationMs = int64(300000)
	const rawDisableAt = int64(1800000312)
	canonicalAppliedAt := rawDisableAt*1000 - durationMs
	metadataAppliedAt := canonicalAppliedAt + dotNetUnixEpochMs + cnUTCOffsetMs
	_, metadataKey, metadataOK := deriveConditionServerClockLead(metadataAppliedAt, true, 0, false, 0, durationMs, observedAt)
	_, rawKey, rawOK := deriveConditionServerClockLead(0, false, 0, false, rawDisableAt, durationMs, observedAt)
	if !metadataOK || !rawOK || metadataKey != rawKey {
		t.Fatalf("application keys differ across sources: metadata=%d (%v), raw=%d (%v)", metadataKey, metadataOK, rawKey, rawOK)
	}
}

func TestConditionClockJumpConsensusHasBoundedRange(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	observeConditionServerClockLeadSample(-15000, base, 1000)
	observeConditionServerClockLeadSample(20000, base.Add(time.Second), 2000)
	observeConditionServerClockLeadSample(23000, base.Add(2*time.Second), 3000)
	observeConditionServerClockLeadSample(26000, base.Add(3*time.Second), 4000)
	if got := conditionServerClockLeadMs.Load(); got != -15000 {
		t.Fatalf("wide jump cluster changed lead to %d", got)
	}
	if got := len(conditionClockCalibration.jumpSamples); got != 1 {
		t.Fatalf("wide jump cluster retained %d samples, want 1", got)
	}
}

func TestConditionClockClearsPendingJumpAfterLongGap(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	observeConditionServerClockLead(-15000, base)
	observeConditionServerClockLead(20000, base.Add(time.Second))
	observeConditionServerClockLead(20100, base.Add(time.Duration(conditionClockStaleAfterMs+1000)*time.Millisecond))
	if got := conditionServerClockLeadMs.Load(); got != -15000 {
		t.Fatalf("one post-gap sample changed lead to %d", got)
	}
	observeConditionServerClockLead(19900, base.Add(time.Duration(conditionClockStaleAfterMs+2000)*time.Millisecond))
	if got := conditionServerClockLeadMs.Load(); got != -15000 {
		t.Fatalf("two post-gap samples changed lead to %d", got)
	}
	observeConditionServerClockLead(20000, base.Add(time.Duration(conditionClockStaleAfterMs+3000)*time.Millisecond))
	if got := conditionServerClockLeadMs.Load(); got != 20000 {
		t.Fatalf("confirmed post-gap lead = %d, want 20000", got)
	}
}

func TestConditionClockFallbackSampleSources(t *testing.T) {
	const observedAtMs = int64(1800000000000)
	observedAt := time.UnixMilli(observedAtMs)

	t.Run("SBT plus duration", func(t *testing.T) {
		resetConditionClockForTest(t, conditionClockFallbackMs)
		metadata := "SDUR:4:180000;SBT:8:63935625792000;"
		if got := parseConditionDurationAtMs(metadata, observedAt, 0); got != 180000 {
			t.Fatalf("duration = %d, want 180000", got)
		}
		if got := conditionServerClockLeadMs.Load(); got != 12000 {
			t.Fatalf("SBT calibration = %d, want 12000", got)
		}
	})

	t.Run("raw DisableAt plus duration", func(t *testing.T) {
		resetConditionClockForTest(t, conditionClockFallbackMs)
		if got := parseConditionDurationAtMs("DUR:4:300000;", observedAt, 1800000312); got != 300000 {
			t.Fatalf("duration = %d, want 300000", got)
		}
		if got := conditionServerClockLeadMs.Load(); got != 12000 {
			t.Fatalf("DisableAt calibration = %d, want 12000", got)
		}
	})

	t.Run("invalid MCAGT falls back to SBT", func(t *testing.T) {
		resetConditionClockForTest(t, conditionClockFallbackMs)
		metadata := "MCAGT:8:1;SDUR:4:180000;SBT:8:63935625792000;"
		parseConditionDurationAtMs(metadata, observedAt, 0)
		if got := conditionServerClockLeadMs.Load(); got != 12000 {
			t.Fatalf("fallback calibration = %d, want 12000", got)
		}
	})
}

func TestConditionClockRejectsInvalidSamples(t *testing.T) {
	resetConditionClockForTest(t, 7777)
	base := time.Unix(1800000000, 0)
	observeConditionServerClockLead(conditionClockMaxAbsLeadMs+1, base)
	if got := conditionServerClockLeadMs.Load(); got != 7777 {
		t.Fatalf("invalid lead changed calibration to %d", got)
	}
}

func TestConditionDisableAtFromDurationIgnoresClockCalibration(t *testing.T) {
	resetConditionClockForTest(t, -45000)
	observedAt := time.UnixMilli(1800000000123)
	if got := conditionDisableAtMsFromDuration(observedAt, 180000); got != 1800000180123 {
		t.Fatalf("fresh millisecond expiry = %d, want 1800000180123", got)
	}
	if got := conditionDisableAtFromDuration(observedAt, 180000); got != 1800000181 {
		t.Fatalf("fresh expiry = %d, want 1800000181", got)
	}
	conditionServerClockLeadMs.Store(60000)
	if got := conditionDisableAtFromDuration(observedAt, 180000); got != 1800000181 {
		t.Fatalf("clock-dependent fresh expiry = %d", got)
	}
	for _, duration := range []int64{0, -1, maxConditionMs + 1} {
		if got := conditionDisableAtFromDuration(observedAt, duration); got != 0 {
			t.Fatalf("invalid duration %d produced expiry %d", duration, got)
		}
	}
}

func TestConditionClockResetClearsPendingState(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	base := time.Unix(1800000000, 0)
	observeConditionServerClockLead(9500, base)
	observeConditionServerClockLead(30000, base.Add(time.Second))
	resetConditionServerClockCalibration(12345)
	if got := conditionServerClockLeadMs.Load(); got != 12345 {
		t.Fatalf("reset lead = %d, want 12345", got)
	}
	if len(conditionClockCalibration.stableSamples) != 0 || len(conditionClockCalibration.jumpSamples) != 0 {
		t.Fatalf("reset retained samples: stable=%v jump=%v", conditionClockCalibration.stableSamples, conditionClockCalibration.jumpSamples)
	}
	if conditionClockCalibration.lastSampleAtMs != 0 || len(conditionClockCalibration.recentSampleKeys) != 0 {
		t.Fatalf("reset retained timing/dedup state: last=%d keys=%v", conditionClockCalibration.lastSampleAtMs, conditionClockCalibration.recentSampleKeys)
	}
}

func TestConditionClockConcurrentObservation(t *testing.T) {
	resetConditionClockForTest(t, conditionClockFallbackMs)
	observedAt := time.Unix(1800000000, 0)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			observeConditionServerClockLeadSample(int64(9900+i%3*100), observedAt, int64(i+1)*1000)
			_ = normalizeConditionDisableAt(1800000200)
		}(i)
	}
	wg.Wait()
	lead := conditionServerClockLeadMs.Load()
	if lead < 9900 || lead > 10100 {
		t.Fatalf("concurrent median = %d, want 9900..10100", lead)
	}
	if got := len(conditionClockCalibration.stableSamples); got != conditionClockStableWindowSize {
		t.Fatalf("concurrent stable window length = %d, want %d", got, conditionClockStableWindowSize)
	}
	if got := len(conditionClockCalibration.recentSampleKeys); got != conditionClockRecentKeyLimit {
		t.Fatalf("recent sample-key count = %d, want %d", got, conditionClockRecentKeyLimit)
	}
}
