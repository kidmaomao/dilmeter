package main

import (
	"math"
	"strconv"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

const lifeCurtainConditionID = 999

type healerCurtain struct {
	amount     int64
	observedAt int64
	expiresAt  int64
}

func readHealerCurtain(e *event.EventCharacterConditionEnable) (healerCurtain, bool) {
	// The server sends two independently consumed absorption pools. A shield
	// with SBBV=0 can still have SBBSB remaining (and vice versa).
	var total float64
	seen := false
	for _, field := range strings.Split(e.Metadata, ";") {
		key, value, found := strings.Cut(strings.TrimSpace(field), ":")
		if !found || key != "SBBV" && key != "SBBSB" {
			continue
		}
		if _, typedValue, typed := strings.Cut(value, ":"); typed {
			value = typedValue
		}
		amount, err := strconv.ParseFloat(value, 64)
		if err != nil || amount < 0 || math.IsNaN(amount) || math.IsInf(amount, 0) {
			return healerCurtain{}, false
		}
		seen = true
		total += amount
	}
	if !seen || total >= math.MaxInt64 {
		return healerCurtain{}, false
	}
	observedAt := nativeEventAtMs(e.At, e.AtMs)
	if total == 0 {
		return healerCurtain{observedAt: observedAt}, true
	}
	expiresAt := e.DisableAtMs
	if expiresAt == 0 {
		expiresAt = e.DisableAt * 1000
	}
	if expiresAt == 0 && e.DurationMs > 0 && !e.Snapshot {
		expiresAt = observedAt + e.DurationMs
	}
	return healerCurtain{amount: max(1, int64(math.Round(total))), expiresAt: expiresAt, observedAt: observedAt}, true
}

func (c healerCurtain) remaining(nowMs int64) *int64 {
	if c.amount == 0 || c.expiresAt > 0 && c.expiresAt <= nowMs {
		return nil
	}
	return &c.amount
}
