//go:build windows && !dilmeter_rt

package main

import "gitlab.com/prilus/mabidilmeter/lib/packet"

// tryAlternateCapture is deliberately empty in DilmeterCN/OT builds. Keeping
// this stub in a build-tagged file prevents the linker from ever seeing the
// WinDivert loader, driver API, or route-mode packet reader.
func tryAlternateCapture(_ *packet.GameServerPacketReader) (attempted bool, started bool) {
	return false, false
}
