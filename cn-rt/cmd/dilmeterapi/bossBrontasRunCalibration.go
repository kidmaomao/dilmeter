package main

import "gitlab.com/prilus/mabidilmeter/lib/packet"

const brontasOwnRunRecallMs int64 = 5000

// Only called after the local-character identity guard. Mount/party movement
// cannot update this calibration. Preserve its original measurement time across
// short mode changes and stops; changing region or local identity clears it.
func brontasRememberOwnRun(previous nativeBrontasPosition, next *nativeBrontasPosition) {
	if next.RegionID != previous.RegionID || next.AtMs < previous.AtMs {
		return
	}
	if previous.RunSpeedAtMs > 0 && next.AtMs-previous.RunSpeedAtMs < brontasOwnRunRecallMs {
		next.RunSpeed, next.RunSpeedAtMs = previous.RunSpeed, previous.RunSpeedAtMs
	}
	if next.Mode == uint64(packet.OpcodeRunning) && next.SpeedSamples >= 2 && next.SpeedAtMs > next.RunSpeedAtMs {
		next.RunSpeed, next.RunSpeedAtMs = next.Speed, next.SpeedAtMs
	}
}

func brontasRecentOwnRun(p nativeBrontasPosition, now int64) bool {
	return p.RunSpeed >= 100 && p.RunSpeed <= 3000 && p.RunSpeedAtMs > 0 && now >= p.RunSpeedAtMs && now-p.RunSpeedAtMs < brontasOwnRunRecallMs &&
		(!p.Moving || p.Mode == uint64(packet.OpcodeRunning))
}
