package util

import (
	"time"
)

// CN servers serialize Mabi time as a timezone-less .NET timestamp using
// China Standard Time.  The upstream KR client used UTC+9 here, which makes
// every CN condition expiry one hour too early and therefore already expired
// as soon as it is received.
const cnServerOffset = 8 * 60 * 60

var cnServerTime = time.FixedZone("CST", cnServerOffset)

func ParseMabiTime(t uint64) time.Time {
	// The wire value is a .NET-style millisecond timestamp. Keep the
	// sub-second part: Buff expiry packets use it to avoid a second round of
	// upward rounding in the desktop countdown.
	const dotNetUnixEpochMs = int64(62135596800000)
	unixMs := int64(t) - dotNetUnixEpochMs - int64(cnServerOffset)*1000
	return time.UnixMilli(unixMs).In(cnServerTime)
}
