package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func perlonTestRuntime(sounds *[]nativeReminderSoundRequest) *nativeReminderRuntime {
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{PerlonHealth: nativePerlonHealthSettings{Enabled: true}, Rules: map[string]nativeBossMechanicRule{
		perlonTornadoKey:   {Enabled: true, ShowCountdown: true, SoundMode: "dedicated"},
		perlonResonanceKey: {Enabled: true, ShowCountdown: true, SoundMode: "dedicated"},
	}}}, sounds)
	r.localID = "self"
	return r
}

func perlonPacketTestPublisher() (*eventPublisher, chan []event.IEvent) {
	p := newSkillTestPublisher()
	p.lastSentEventAt = time.Now()
	events := make(chan []event.IEvent, 16)
	p.clientMap = map[uint32]*eventClient{1: {ctx: context.Background(), ch: events}}
	return p, events
}

func TestPerlonTornadoPacketIdentityAndDedup(t *testing.T) {
	p, batches := perlonPacketTestPublisher()
	p.entityCache[100] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 100, RaceId: perlonRaceID}}
	wire := &packet.GamePacket{At: time.UnixMilli(100000), Id: 100, Op: 37011, Msg: packet.Message{packet.NewMessageElemInt(967), packet.NewMessageElemInt(1), packet.NewMessageElemLong(900), packet.NewMessageElemFloat(97780), packet.NewMessageElemFloat(137046), packet.NewMessageElemFloat(300), packet.NewMessageElemInt(1000), packet.NewMessageElemInt(3000)}}
	p.publishPerlonPacket(wire)
	p.publishPerlonPacket(wire)
	wire.At = time.UnixMilli(120000)
	wire.Msg[2] = packet.NewMessageElemLong(901)
	p.publishPerlonPacket(wire)
	wire.Id = 101
	p.publishPerlonPacket(wire)
	wire.Id = 100
	wire.Msg[1] = packet.NewMessageElemInt(4)
	p.publishPerlonPacket(wire)
	if len(p.pendingEvents) != 0 || len(batches) != 2 {
		t.Fatalf("queued=%d batches=%d, want two immediate casts only", len(p.pendingEvents), len(batches))
	}
	for len(batches) > 0 {
		v := (<-batches)[0].(*event.EventSkillAction)
		if v.SkillId != 0 || v.MechanicSignal != perlonTornadoKey || v.MechanicDurationMs != 1000 {
			t.Fatalf("bad signal %+v", v)
		}
	}
}

func TestPerlonPhantomsAndPhaseClock(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	r.onEvent(liveTestAppear("boss", "佩隆", perlonRaceID, 100))
	for i, id := range []string{"p1", "p2", "p3"} {
		r.onEvent(liveTestAppear(id, "幻影", uint32(193811+i), 110))
	}
	if r.perlon.Wave != 1 {
		t.Fatal("three phantoms counted as three phases")
	}
	r.selectedTargetID = "p3"
	if r.selectedTargetHealth() != nil {
		t.Fatal("invented unsynchronized health")
	}
	r.onEvent(healerHP("p3", 111, 20000000, 50000000))
	if got := r.selectedTargetHealth(); got == nil || got.Kind != "perlon-phantom" || got.PhaseLabel != "90% 幻影" || got.CurrentHealth != 20000000 {
		t.Fatalf("health %+v", got)
	}
	r.selectedTargetID = "p1"
	if r.selectedTargetHealth() != nil {
		t.Fatal("old target health leaked into another phantom")
	}
	signal := func(key string, ms int64) {
		r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "self", At: ms / 1000}, AtMs: ms, MechanicSignal: key})
	}
	signal("perlon-absorb", 130000)
	if r.perlon.ModeDue != 143000 {
		t.Fatal("opening estimate")
	}
	signal("perlon-release", 143150)
	if r.perlon.ModeDue != 158150 {
		t.Fatal("15-second phase")
	}
	signal("perlon-shield", 148000)
	if len(r.perlonOverlay(time.UnixMilli(148000))) != 0 {
		t.Fatal("60% shield kept a stale clock")
	}
	r.onEvent(liveTestAppear("p4", "幻影", 193813, 200))
	signal("perlon-absorb", 220000)
	if r.perlon.ModeDue != 228000 {
		t.Fatal("second opening estimate")
	}
	signal("perlon-release", 228100)
	if r.perlon.ModeDue != 238100 {
		t.Fatal("10-second phase")
	}
	signal("perlon-shield", 230000)
	if r.perlon.Mode == "" {
		t.Fatal("25% shield incorrectly paused resonance")
	}
	items := r.perlonOverlay(time.UnixMilli(238200))
	if len(items) != 1 || !items[0].TimingUnknown || r.perlon.Mode != "perlon-release" {
		t.Fatal("timer fabricated a phase transition")
	}
	signal("perlon-rampage", 240000)
	if len(r.perlonOverlay(time.UnixMilli(240000))) != 0 {
		t.Fatal("rampage kept clock")
	}
	r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "boss", At: 241}, AtMs: 241000, MechanicSignal: perlonTornadoKey, MechanicDurationMs: 1000})
	if len(r.perlonOverlay(time.UnixMilli(241100))) != 1 || len(r.perlonOverlay(time.UnixMilli(243999))) != 1 || len(r.perlonOverlay(time.UnixMilli(244000))) != 0 {
		t.Fatal("tornado startup lifetime")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "self"}, Reset: true})
	if r.perlon.BossID != "" {
		t.Fatal("connection reset retained encounter")
	}
}

// Replays the user's complete capture without checking private recordings into git.
func TestPerlonCaptureReplay(t *testing.T) {
	path := os.Getenv("DILMETER_PERLON_PCAP")
	if path == "" {
		t.Skip("set DILMETER_PERLON_PCAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	bosses, phantoms := map[string]bool{}, map[string]bool{}
	counts := map[string]int{}
	forecasts, maxLateMs := 0, int64(0)
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(12 * time.Second)
	defer idle.Stop()
	for {
		select {
		case batch := <-events:
			idle.Reset(5 * time.Second)
			for _, raw := range batch {
				before := r.perlon
				if e, ok := raw.(*event.EventSkillAction); ok && (e.MechanicSignal == "perlon-absorb" || e.MechanicSignal == "perlon-release") && before.Mode != "" && before.ModeDue > 0 {
					delta := e.AtMs - before.ModeDue
					if delta < 0 || delta > 500 {
						t.Fatalf("forecast drift %d ms", delta)
					}
					forecasts++
					maxLateMs = max(maxLateMs, delta)
				}
				r.onEvent(raw)
				if e, ok := raw.(*event.EventEntityAppear); ok && e.RaceId == perlonRaceID {
					bosses[e.Id] = true
				}
				if bar := r.selectedTargetHealth(); bar != nil && bar.Kind == "perlon-phantom" {
					phantoms[bar.EntityID] = true
				}
				if e, ok := raw.(*event.EventSkillAction); ok && strings.HasPrefix(e.MechanicSignal, "perlon-") {
					counts[e.MechanicSignal]++
					if e.MechanicSignal == perlonTornadoKey {
						switch e.MechanicTargetId {
						case "":
							counts["tornado-unknown"]++
						case r.localID:
							counts["tornado-self"]++
						default:
							counts["tornado-teammate"]++
						}
					}
					if e.MechanicSignal == "perlon-release-area-warning" {
						items := r.perlonOverlay(time.UnixMilli(e.AtMs))
						found := false
						for _, item := range items {
							if item.Key == perlonResonanceKey && item.ResonanceMode == "release" && item.AreaWarningUntilMs == e.AtMs+perlonAreaWarningFreshMs {
								found = true
							}
						}
						if !found {
							t.Fatalf("personal area warning lost %+v state %+v", e, r.perlon)
						}
					}
					if e.MechanicSignal == perlonTornadoKey && r.perlon.TornadoDue != e.AtMs+1000 {
						t.Fatalf("cast failed %+v state %+v", e, r.perlon)
					}
					if e.MechanicSignal == perlonTornadoKey {
						// A frame arriving after the one-second startup must still
						// contain the dodge warning, with the original startup intact.
						visible := false
						for _, item := range r.perlonOverlay(time.UnixMilli(e.AtMs + 1500)) {
							if item.Key == perlonTornadoKey && item.TornadoStartsAtMs == e.AtMs+1000 && item.EndsAtMs == e.AtMs+3000 {
								visible = true
							}
						}
						if !visible {
							t.Fatalf("delayed tornado frame lost: %+v", e)
						}
					}
					if e.MechanicSignal == "perlon-absorb" || e.MechanicSignal == "perlon-release" {
						if r.perlon.Mode != e.MechanicSignal || r.perlon.ModeDue <= e.AtMs {
							t.Fatalf("mode not anchored %+v state %+v", e, r.perlon)
						}
					}
				}
			}
		case <-idle.C:
			t.Logf("bosses=%d health targets=%d signals=%v sounds=%d", len(bosses), len(phantoms), counts, len(sounds))
			if len(bosses) != 8 || len(phantoms) < 16 || counts[perlonTornadoKey] != 63 || counts["perlon-absorb"] != 57 || counts["perlon-release"] != 46 || counts["perlon-shield"] != 16 || counts["perlon-rampage"] != 8 || counts["perlon-release-area-warning"] != 109 {
				t.Fatal("capture regression mismatch")
			}
			t.Logf("forecast transitions=%d maximum observed delay=%d ms", forecasts, maxLateMs)
			if counts["tornado-self"] != 15 || counts["tornado-teammate"] != 48 || counts["tornado-unknown"] != 0 {
				t.Fatalf("tornado target attribution changed: %v", counts)
			}
			if forecasts != 87 {
				t.Fatalf("forecast count %d", forecasts)
			}
			if len(sounds) != 166 {
				t.Fatalf("want one sound per deployment/switch; got %d", len(sounds))
			}
			tornadoSounds := 0
			for _, sound := range sounds {
				if sound.Tornado {
					tornadoSounds++
				}
			}
			if tornadoSounds != 63 {
				t.Fatalf("independent tornado channel received %d sounds, want 63", tornadoSounds)
			}
			if len(r.perlonOverlay(time.Now())) != 0 {
				t.Fatal("overlay survived encounter end")
			}
			return
		case <-ctx.Done():
			t.Fatal("replay timeout")
		}
	}
}
