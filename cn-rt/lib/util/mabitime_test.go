package util

import "testing"

func TestParseMabiTimeUsesCNServerOffset(t *testing.T) {
	// Live CN packet SBT 63920974748990 ms (2026-07-30 02:19:08 CST).
	const raw = uint64(63920974748990)
	const wantUnix = int64(1785349148)
	const wantUnixMs = int64(1785349148990)
	if got := ParseMabiTime(raw).Unix(); got != wantUnix {
		t.Fatalf("ParseMabiTime(%d) = %d, want %d", raw, got, wantUnix)
	}
	if got := ParseMabiTime(raw).UnixMilli(); got != wantUnixMs {
		t.Fatalf("ParseMabiTime(%d) milliseconds = %d, want %d", raw, got, wantUnixMs)
	}
}
