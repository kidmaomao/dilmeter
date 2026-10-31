package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"testing"
)

func TestBrontasOwnRunSurvivesShortDismountWithoutPredictingPosition(t *testing.T) {
	p := nativeBrontasPosition{AtMs: 100000, RegionID: 1, Mode: uint64(packet.OpcodeRunning), Speed: 1100, SpeedSamples: 2, SpeedAtMs: 100000}
	brontasRememberOwnRun(p, &p)
	if p.RunSpeed != 1100 || p.RunSpeedAtMs != 100000 {
		t.Fatal("measured own speed not remembered", p)
	}
	stop := nativeBrontasPosition{AtMs: 100100, RegionID: 1, Mode: uint64(packet.OpcodeWalking), Moving: true, X: 23000, Y: 37000, DestX: 23001, DestY: 37000}
	brontasRememberOwnRun(p, &stop)
	dismount := nativeBrontasPosition{AtMs: 102900, RegionID: 1, Mode: stop.Mode, X: 25160, Y: 40967}
	brontasRememberOwnRun(stop, &dismount)
	r := brontasLiveFixture()
	r.brontasPosition = dismount
	player, _ := r.brontasStationPlayer(12, 103000)
	if player == nil || player.Speed != 1100*.60/1269 {
		t.Fatal("dismount lost conservative own calibration", player)
	}
	if dismount.SpeedSamples != 0 || brontasStationMotion(dismount, 12, 103000) != nil {
		t.Fatal("recalled speed projected a moving dot")
	}
	if p := brontasPositionAt(dismount, 103000); p.X != dismount.X || p.Y != dismount.Y {
		t.Fatal("cached speed changed confirmed location")
	}
	if brontasRecentOwnRun(dismount, 105000) {
		t.Fatal("stop refreshed expired calibration")
	}
	dismount.Moving = true
	if brontasRecentOwnRun(dismount, 103000) {
		t.Fatal("walking used cached running speed")
	}
	other := nativeBrontasPosition{AtMs: 103100, RegionID: 2}
	brontasRememberOwnRun(dismount, &other)
	if other.RunSpeed != 0 {
		t.Fatal("cache crossed regions")
	}
}

func TestBrontasPetCannotSupplyOwnRunCalibration(t *testing.T) {
	r := brontasLiveFixture()
	r.brontasPosition.RunSpeed, r.brontasPosition.RunSpeedAtMs = 1000, 100000
	before := r.brontasPosition
	r.observeBrontasRay(&event.EventBrontasRay{EventBase: event.EventBase{Id: "owned-pet"}, AtMs: 101000, Kind: "position", X: 25160, Y: 40967, Moving: true, DestX: 26000, DestY: 40967, MoveMode: uint64(packet.OpcodeRunning), Live: true})
	if r.brontasPosition != before {
		t.Fatal("non-character movement contaminated own speed")
	}
}
