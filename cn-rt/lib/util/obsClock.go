package util

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Process-relative time retains Go's monotonic clock; it never uses a capture
// driver's timestamp or changes when Windows adjusts the wall clock.
var obsClockStart = time.Now()
var ObsClockSession = newObsClockSession()

func newObsClockSession() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id[:])
}

func ObsClockNow() int64            { return time.Since(obsClockStart).Milliseconds() }
func ObsClockAt(at time.Time) int64 { return at.Sub(obsClockStart).Milliseconds() }
