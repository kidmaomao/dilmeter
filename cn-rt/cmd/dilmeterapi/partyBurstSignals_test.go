package main

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func publicBurstPacket(id uint64, op packet.OpCode, atMs int64, msg packet.Message) *packet.GamePacket {
	return &packet.GamePacket{Id: id, Op: op, At: time.UnixMilli(atMs), Msg: msg}
}

func TestPartyPublicBurstPrepareCancelAndNoSyntheticCooldown(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId, publisher.lastSentEventAt = 100, time.Now()
	publisher.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
	start := publicBurstPacket(200, 37011, 100000, packet.Message{packet.NewMessageElemInt(778), packet.NewMessageElemInt(1)})
	publisher.publishPartyBurstSignal(start)
	publisher.publishPartyPreparation(publicBurstPacket(200, 27012, 100001, packet.Message{packet.NewMessageElemShort(59005)}), true, false)
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("public animation and private ACK were counted twice")
	}
	value := publisher.pendingEvents[0].(*event.EventSkillState)
	if value.Id != "200" || value.SkillId != 59005 || !value.Active {
		t.Fatalf("public cast lost its real caster: %+v", value)
	}
	publisher.publishPartyBurstSignal(publicBurstPacket(200, 28006, 100500, packet.Message{packet.NewMessageElemByte(0)}))
	publisher.publishPartyBurstSignal(publicBurstPacket(200, 28002, 100501, packet.Message{packet.NewMessageElemInt(205), packet.NewMessageElemInt(5), packet.NewMessageElemByte(0), packet.NewMessageElemShort(0), packet.NewMessageElemShort(0)}))
	publisher.publishPartyBurstSignal(publicBurstPacket(200, 37011, 101501, packet.Message{packet.NewMessageElemInt(615), packet.NewMessageElemByte(1)}))
	if len(publisher.pendingEvents) != 5 {
		t.Fatalf("lost cancellation, next power preparation or completion: %+v", publisher.pendingEvents)
	}
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeTeammates = true, true
	for _, raw := range publisher.pendingEvents {
		state := raw.(*event.EventSkillState)
		state.Id = "ally"
		runtime.onEvent(state)
		popups, _ := runtime.burstOverlay(time.UnixMilli(state.AtMs))
		if state.Active && (len(popups) != 1 || popups[0].ActorID != "ally" || popups[0].SkillID != state.SkillId || popups[0].Phase != map[string]string{"prepare": "cast", "burst-effect": "effect"}[state.Scope]) {
			t.Fatalf("missing structured cast popup: %+v", popups)
		}
		if !state.Active && len(popups) != 0 {
			t.Fatal("cancelled/completed preparation continued counting")
		}
	}
	for _, state := range runtime.partySkills["ally"] {
		if state.UsedAtMs != 0 {
			t.Fatal("preparation fabricated a cooldown")
		}
	}
	publisher.pendingEvents = nil
	for _, id := range []uint64{0, 300} {
		start.Id = id
		publisher.publishPartyBurstSignal(start)
	}
	publisher.entityCache[400] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 400, RaceId: 7603}}
	start.Id = 400
	publisher.publishPartyBurstSignal(start)
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("unknown actors/monsters triggered teammate casts")
	}
}

func TestPowerCooldownRemovedFromLegacySettings(t *testing.T) {
	settings := normalizeNativeBurstSettings(nativeBurstSettings{Rules: map[uint16]nativeBurstRule{58014: {CooldownSeconds: 360, Ready: nativeBurstDisplay{Enabled: true}}}})
	if rule := settings.Rules[58014]; rule.CooldownSeconds != 0 || rule.Ready.Enabled {
		t.Fatal("old power cooldown was re-enabled")
	}
	skills := normalizeHealerSkills(&healerMemberSkillSettings{Rules: []healerSkillRule{{SkillID: 58014}, {SkillID: 59005}}}, 0)
	if len(skills.Rules) != 1 || skills.Rules[0].SkillID != 59005 {
		t.Fatal("legacy teammate power cooldown survived migration")
	}
}

func TestCollapseResultConfirmsCooldownButSnapshotDoesNot(t *testing.T) {
	runtime, _ := partyFixture(t)
	runtime.onEvent(liveTestAppear("boss", "Boss", 7603, 100))
	condition := &event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "boss", At: 100}, CCId: 803, AttackerId: "ally", DurationMs: 5000, Snapshot: true}
	runtime.onEvent(condition)
	if runtime.partySkills["ally"][59005].UsedAtMs != 0 {
		t.Fatal("existing Boss condition snapshot fabricated a new release")
	}
	condition.Snapshot, condition.At = false, 101
	runtime.onEvent(condition)
	if runtime.partySkills["ally"][59005].UsedAtMs != 101000 {
		t.Fatal("actual caster's CC803 result did not confirm release")
	}
	condition.At = 102
	runtime.onEvent(condition)
	if runtime.partySkills["ally"][59005].UsedAtMs != 101000 {
		t.Fatal("repeated area result refreshed the same cooldown")
	}
}

func TestSimultaneousBurstPopupsDoNotOverlap(t *testing.T) {
	first := nativeBossMechanicOverlayItem{X: 600, Y: 350, ScalePercent: 100}
	second := separateBurstPopup([]nativeBossMechanicOverlayItem{first}, nativeBossMechanicOverlayItem{X: 600, Y: 440, ScalePercent: 125})
	if second.Y < first.Y+112 {
		t.Fatal("simultaneous collapse and power cards overlap")
	}
	separate := nativeBossMechanicOverlayItem{X: 100, Y: 440, ScalePercent: 100}
	if !reflect.DeepEqual(separateBurstPopup([]nativeBossMechanicOverlayItem{first}, separate), separate) {
		t.Fatal("non-overlapping user position was changed")
	}
}

// Opt-in integration check keeps user captures outside the repository.
func TestReplayPartyPublicBurstCapture(t *testing.T) {
	file := os.Getenv("DILMETER_PARTY_BURST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PARTY_BURST_PCAP to replay public teammate preparations")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	runtime, _ := partyFixture(t)
	runtime.settings.Burst.Enabled, runtime.settings.Burst.IncludeTeammates = true, true
	const ally = "4503599631374038"
	runtime.healer.settings.Members[0].ID = ally
	runtime.healer.settings.Members[0].Favorite = true
	counts, popups := map[uint16]int{}, map[uint16]int{}
	effectStarts, effectEnds, collapseReleases := 0, 0, 0
	var powerDeadline int64
	last := time.Now()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case batch := <-events:
			last = time.Now()
			for _, raw := range batch {
				if appearance, ok := raw.(*event.EventEntityAppear); ok && appearance.Id == ally {
					runtime.healer.settings.Members[0].ID, runtime.healer.settings.Members[0].Name = ally, appearance.Name
				}
				runtime.onEvent(raw)
				if state, ok := raw.(*event.EventSkillState); ok && state.Id == ally && state.Scope == "burst-effect" {
					items, _ := runtime.burstOverlay(time.UnixMilli(state.AtMs))
					active := false
					for _, item := range items {
						if item.ActorID == ally && item.SkillID == 58014 && item.Phase == "effect" {
							active = true
							if item.TimingUnknown || item.EndsAtMs != runtime.partySkills[ally][58014].EffectAtMs+powerAwakeningDurationMs {
								t.Fatalf("Power is not capped at ten seconds: %+v", item)
							}
						}
					}
					if state.Active {
						effectStarts++
						powerDeadline = state.AtMs + powerAwakeningDurationMs
						if !active {
							t.Fatal("public power activation missing from overlay")
						}
					} else {
						effectEnds++
						if active != (powerDeadline > state.AtMs) {
							t.Fatal("held-skill end changed Awakening's independent ten-second window")
						}
					}
				}
				if state, ok := raw.(*event.EventSkillState); ok && state.Id == ally && state.Scope == "burst-release" && state.Active {
					collapseReleases++
					healerTick(runtime, state.AtMs)
					observed := runtime.healer.state.Members[0].Skills[59005]
					if observed.State != "cooling" || observed.RemainingSeconds == nil || *observed.RemainingSeconds != 10 {
						t.Fatalf("release missing from teammate information: %+v", observed)
					}
				}
				if state, ok := raw.(*event.EventSkillState); ok && state.Id == ally && state.Scope == "prepare" && state.Active {
					counts[state.SkillId]++
					items, _ := runtime.burstOverlay(time.UnixMilli(state.AtMs))
					for _, popup := range items {
						if popup.ActorID == ally && popup.SkillID == state.SkillId {
							popups[state.SkillId]++
						}
					}
				}
			}
		case <-ticker.C:
			if _, parsed, _ := reader.GetStats(); parsed > 0 && time.Since(last) > 2*time.Second {
				expectedCollapse, expectedPower := 11, 6
				if strings.Contains(file, "1791015423") {
					expectedCollapse, expectedPower = 6, 2
					if effectStarts != 2 || effectEnds != 2 {
						t.Fatalf("power start/end = %d/%d", effectStarts, effectEnds)
					}
				}
				if strings.Contains(file, "1791021514") {
					expectedCollapse, expectedPower = 1, 4
					if effectStarts != 1 || effectEnds != 4 || collapseReleases != 1 {
						t.Fatalf("latest capture: power=%d/%d collapse=%d", effectStarts, effectEnds, collapseReleases)
					}
				}
				if counts[59005] != expectedCollapse || counts[58014] != expectedPower || popups[59005] != counts[59005] || popups[58014] != counts[58014] {
					t.Fatalf("public events did not reach teammate popup: events=%v popups=%v", counts, popups)
				}
				t.Logf("October 3 replay verified: preparations=%v popups=%v Power=%d/%d Collapse releases=%d", counts, popups, effectStarts, effectEnds, collapseReleases)
				return
			}
		case <-ctx.Done():
			t.Fatalf("replay timed out: %v", counts)
		}
	}
}
