package main

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestSpiritPublisherSlotIdentityAndReset(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 1
	send := func(owner uint64, msg packet.Message) {
		p.publishSpiritWeaponPacket(&packet.GamePacket{Id: owner, At: time.Unix(100, 0), Msg: msg})
	}
	delta := packet.Message{packet.NewMessageElemInt(2), packet.NewMessageElemInt(0), packet.NewMessageElemInt(1 << 14), packet.NewMessageElemInt(60000)}
	send(1, delta)
	if len(spiritWeaponEnergies(p.spiritWeapons)) != 0 {
		t.Fatal("unknown positional slot guessed a weapon")
	}
	snapshot := packet.Message{packet.NewMessageElemInt(2),
		packet.NewMessageElemInt(1 | 1<<14), packet.NewMessageElemShort(12), packet.NewMessageElemInt(30000),
		packet.NewMessageElemInt(1 | 1<<14), packet.NewMessageElemShort(20), packet.NewMessageElemInt(60000)}
	send(2, snapshot)
	if len(spiritWeaponEnergies(p.spiritWeapons)) != 0 {
		t.Fatal("another player's roster accepted")
	}
	send(1, snapshot)
	p.clientMap = make(map[uint32]*eventClient)
	ch := make(chan []event.IEvent, 20)
	p.addClient(context.Background(), ch)
	initial := <-ch
	full := false
	for _, raw := range initial {
		if energy, ok := raw.(*event.EventSkillEnergy); ok && energy.SkillId == 35130 {
			full = energy.Active && energy.Percent == 100
		}
	}
	if !full {
		t.Fatal("new frontend client missed retained full charge")
	}
	delta[3] = packet.NewMessageElemInt(0)
	send(1, delta)
	states := spiritWeaponEnergies(p.spiritWeapons)
	if len(states) != 2 || states[35122].Percent != 50 || states[35130].Percent != 0 {
		t.Fatalf("weapons mixed: %+v", states)
	}
	send(1, snapshot[:len(snapshot)-1])
	if !reflect.DeepEqual(states, spiritWeaponEnergies(p.spiritWeapons)) {
		t.Fatal("partial packet mutated roster")
	}
	p.publish(&event.EventLocalEntity{EventBase: event.EventBase{Id: "1"}, Reset: true})
	send(1, delta)
	if len(spiritWeaponEnergies(p.spiritWeapons)) != 0 {
		t.Fatal("same-character reconnect retained positional identity")
	}
	send(1, snapshot)
	send(1, packet.Message{packet.NewMessageElemInt(1), packet.NewMessageElemInt(1 << 14), packet.NewMessageElemInt(60000)})
	if len(spiritWeaponEnergies(p.spiritWeapons)) != 0 {
		t.Fatal("changed slot count reused an old identity")
	}
}

func TestSpiritChargeReadinessAndOneExtraUse(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{
		Rules: map[uint16]nativeSkillCooldownRule{
			35130: {SkillID: 35130, Enabled: true, AlwaysVisible: true, CooldownSeconds: 60, ProgressThresholdPercent: 80, SoundMode: "default"},
			35122: {SkillID: 35122, Enabled: true, AlwaysVisible: true, SoundMode: "default"},
		},
	}}, &sounds)
	r.localID = "self"
	now := time.Unix(100, 0)
	gauge := func(owner string, skill uint16, percent float64) {
		r.onEvent(&event.EventSkillEnergy{EventBase: event.EventBase{Id: owner}, SkillId: skill, Active: true, Percent: percent})
		r.evaluateEnergySkills(now)
	}
	if r.settings.SkillCooldowns.Rules[35130].ProgressThresholdPercent != 100 {
		t.Fatal("legacy early threshold retained")
	}
	gauge("other", 35130, 100)
	gauge("self", 35122, 50)
	gauge("self", 35130, 99)
	if len(sounds) != 0 || r.energyReady[35130] {
		t.Fatal("incomplete or another player's charge announced ready")
	}
	gauge("self", 35130, 100)
	if !r.energyReady[35130] || r.energyReady[35122] || len(sounds) != 1 {
		t.Fatal("full charge did not become independently ready without a timer")
	}
	// First cast procs 58015: no zero update arrives, so retain readiness.
	r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "self", At: 100}, SkillId: 35130})
	r.evaluateEnergySkills(now.Add(time.Second))
	gauge("self", 35130, 100)
	if !r.energyReady[35130] || len(sounds) != 1 || r.skillCooldowns[35130].UsedAtMs != 0 {
		t.Fatal("retained charge replaced by a timer or duplicate sound")
	}
	// The one extra cast consumes the authoritative gauge.
	gauge("self", 35130, 0)
	r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "self", At: 102}, SkillId: 35130})
	r.evaluateEnergySkills(now.Add(time.Hour))
	if r.energyReady[35130] {
		t.Fatal("spent charge became ready through elapsed time")
	}
	gauge("self", 35130, 100)
	if len(sounds) != 2 {
		t.Fatal("new full charge did not announce once")
	}
	r.publishNativeSkillState(now)
	var overlay struct {
		Items []nativeSkillOverlayItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(r.lastSkillStateKey), &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Items) != 2 {
		t.Fatalf("missing weapon reminders: %+v", overlay.Items)
	}
	for _, item := range overlay.Items {
		if !item.EnergyGate || item.ProgressObserved == nil || !*item.ProgressObserved || item.EnergyReady != (item.SkillID == 35130) {
			t.Fatalf("incorrect overlay state: %+v", item)
		}
	}
	rule := r.settings.SkillCooldowns.Rules[35130]
	delete(r.settings.SkillCooldowns.Rules, 35130)
	r.evaluateEnergySkills(now)
	r.settings.SkillCooldowns.Rules[35130] = rule
	r.evaluateEnergySkills(now)
	if len(sounds) != 3 || !r.energyReady[35130] {
		t.Fatal("re-added reminder retained a removed rule's readiness latch")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "self"}, Reset: true})
	r.evaluateEnergySkills(now)
	if len(r.spiritEnergy) != 0 || r.energyReady[35130] {
		t.Fatal("reconnect retained stale charge")
	}
}

// Original capture stays outside the repository. The October 14 sample has
// seven orb casts, of which the second and sixth retain charge through 58015.
func TestReplaySpiritAwakeningCapture(t *testing.T) {
	file := os.Getenv("DILMETER_SPIRIT_PCAP")
	if file == "" {
		t.Skip("set DILMETER_SPIRIT_PCAP to the October 14 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	p := newEventPublisher(ctx, reader)
	ch := make(chan []event.IEvent, 1000)
	p.addClient(ctx, ch)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	localID := ""
	percent := -1.0
	casts := []float64{}
	weapons := make(map[uint16]bool)
	for {
		select {
		case batch := <-ch:
			for _, raw := range batch {
				switch v := raw.(type) {
				case *event.EventLocalEntity:
					localID = v.Id
					if v.Reset {
						percent = -1
					}
				case *event.EventSkillEnergy:
					if v.Id == localID && packet.IsSpiritAwakeningSkill(v.SkillId) {
						weapons[v.SkillId] = true
						if v.SkillId == 35130 {
							percent = v.Percent
						}
					}
				case *event.EventSkillAction:
					if v.Id == localID && v.SkillId == 35130 && !v.IsFallback {
						casts = append(casts, percent)
					}
				}
			}
			if len(casts) >= 7 {
				if !reflect.DeepEqual(casts, []float64{0, 100, 0, 0, 0, 100, 0}) || len(weapons) != 9 {
					t.Fatalf("unexpected charge at casts=%v, weapons=%d", casts, len(weapons))
				}
				t.Logf("seven casts, nine weapons, charge at each cast=%v", casts)
				return
			}
		case <-ctx.Done():
			t.Fatalf("incomplete replay casts=%v, weapons=%d", casts, len(weapons))
		}
	}
}
