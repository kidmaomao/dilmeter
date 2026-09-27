package main

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func energyMessage(value uint32) packet.Message {
	return packet.Message{packet.NewMessageElemByte(2), packet.NewMessageElemShort(1), packet.NewMessageElemByte(1), packet.NewMessageElemString("MECLIDVA"), packet.NewMessageElemByte(3), packet.NewMessageElemInt(value)}
}
func TestDarkEnergyPropertyPacket(t *testing.T) {
	for _, value := range []uint32{0, 5, 95, 100} {
		percent, active, ok := parseDarkEnergy(energyMessage(value))
		if !ok || !active || percent != float64(value) {
			t.Fatalf("gauge %d: %v %v %v", value, percent, active, ok)
		}
	}
	if _, _, ok := parseDarkEnergy(energyMessage(101)); ok {
		t.Fatal("invalid gauge accepted")
	}
	msg := append(packet.Message{packet.NewMessageElemByte(2), packet.NewMessageElemShort(2), packet.NewMessageElemByte(0), packet.NewMessageElemString("unrelated")}, energyMessage(100)[2:]...)
	if value, active, ok := parseDarkEnergy(msg); !ok || !active || value != 100 {
		t.Fatal("batched property not parsed")
	}
	if _, _, ok := parseDarkEnergy(msg[:len(msg)-1]); ok {
		t.Fatal("truncated batch accepted")
	}
	removed := packet.Message{packet.NewMessageElemByte(2), packet.NewMessageElemShort(1), packet.NewMessageElemByte(0), packet.NewMessageElemString("MECLIDVA")}
	if _, active, ok := parseDarkEnergy(removed); !ok || active {
		t.Fatal("removal must disable gauge")
	}
}

func TestEnergyAndCooldownReadiness(t *testing.T) {
	settings := nativeReminderSettings{SkillCooldowns: nativeSkillCooldownSettings{Rules: map[uint16]nativeSkillCooldownRule{
		59047: {SkillID: 59047, Enabled: true, CooldownSeconds: 30, ProgressThresholdPercent: 100, SoundMode: "default"},
		59085: {SkillID: 59085, Enabled: true, ProgressThresholdPercent: 100, SoundMode: "default"},
	}}}
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(settings, &sounds)
	r.localID = "self"
	gauge := func(id string, percent float64) {
		r.onEvent(&event.EventSkillEnergy{EventBase: event.EventBase{Id: id, At: 100}, SkillId: 59047, Percent: percent, Active: true})
	}
	gauge("other", 100)
	r.evaluateEnergySkills(time.Unix(100, 0))
	if r.darkEnergy.Active {
		t.Fatal("tracked another player")
	}
	gauge("self", 100)
	r.evaluateEnergySkills(time.Unix(100, 0))
	if len(sounds) != 0 {
		t.Fatal("unknown CD announced ready")
	}
	r.observeSkillCooldown(59047, 100000, false)
	r.evaluateEnergySkills(time.Unix(129, 0))
	if len(sounds) != 0 {
		t.Fatal("energy bypassed cooldown")
	}
	gauge("self", 95)
	r.evaluateEnergySkills(time.Unix(130, 0))
	r.evaluateSkillCooldowns(time.Unix(130, 0))
	if len(sounds) != 0 {
		t.Fatal("cooldown bypassed energy")
	}
	gauge("self", 100)
	r.evaluateEnergySkills(time.Unix(131, 0))
	r.evaluateEnergySkills(time.Unix(132, 0))
	if len(sounds) != 1 || !r.energyReady[59047] {
		t.Fatal("must notify once and retain full gauge")
	}
	gauge("self", 0)
	r.evaluateEnergySkills(time.Unix(133, 0))
	if r.energyReady[59047] {
		t.Fatal("spent gauge still ready")
	}
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "self", At: 134}, CCId: 1179, Metadata: "MCNPGV:f:100;"})
	r.evaluateEnergySkills(time.Unix(134, 0))
	r.evaluateEnergySkills(time.Unix(135, 0))
	if len(sounds) != 2 || !r.energyReady[59085] {
		t.Fatal("holy gauge must not require CD")
	}
	r.publishNativeSkillState(time.Unix(135, 0))
	var overlay struct {
		Items []nativeSkillOverlayItem `json:"items"`
	}
	if err := json.Unmarshal([]byte(r.lastSkillStateKey), &overlay); err != nil {
		t.Fatal(err)
	}
	if len(overlay.Items) != 2 {
		t.Fatalf("both energy reminders must reach desktop overlay: %+v", overlay.Items)
	}
	for _, item := range overlay.Items {
		if !item.EnergyGate || item.ProgressPercent == nil || item.ProgressObserved == nil || !*item.ProgressObserved {
			t.Fatalf("overlay lacks gauge state: %+v", item)
		}
		if item.SkillID == 59085 && (!item.EnergyReady || *item.ProgressPercent != 100 || !nativeSkillReminderItemVisible(item, 135000)) {
			t.Fatalf("full holy energy must remain visible: %+v", item)
		}
		if item.SkillID == 59047 && (item.EnergyReady || nativeSkillReminderItemVisible(item, 135000)) {
			t.Fatalf("spent dark energy must remain hidden: %+v", item)
		}
	}
	r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "self", At: 136}, CCId: 1179})
	r.evaluateEnergySkills(time.Unix(136, 0))
	if r.energyReady[59085] {
		t.Fatal("inactive oath retained ready state")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "new", At: 140}})
	if r.darkEnergy.Active || r.skillCooldowns[59047].UsedAtMs != 0 {
		t.Fatal("identity change retained energy or cooldown")
	}
}

// Original captures stay on the user's machine. Replays validate the complete
// wire -> publisher path including repeated casts at the same target.
func TestReplayEnergyCaptures(t *testing.T) {
	for _, tc := range []struct {
		env   string
		skill uint16
		casts int
	}{{"DILMETER_DARK_ENERGY_PCAP", 59047, 3}, {"DILMETER_HOLY_ENERGY_PCAP", 59085, 2}} {
		t.Run(tc.env, func(t *testing.T) {
			file := os.Getenv(tc.env)
			if file == "" {
				t.Skip("set " + tc.env)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
			if err != nil {
				t.Fatal(err)
			}
			defer reader.Close()
			publisher := newEventPublisher(ctx, reader)
			ch := make(chan []event.IEvent, 1000)
			publisher.addClient(ctx, ch)
			if err := reader.OpenFile(file); err != nil {
				t.Fatal(err)
			}
			casts, full, spent := 0, false, false
			for {
				select {
				case batch := <-ch:
					for _, raw := range batch {
						switch v := raw.(type) {
						case *event.EventSkillEnergy:
							if tc.skill == 59047 {
								if v.Percent == 100 {
									full = true
								}
								if full && v.Percent == 0 {
									spent = true
								}
							}
						case *event.EventCharacterConditionEnable:
							if tc.skill == 59085 && v.CCId == 1179 {
								p, ok := holyEnergyPercent(v.Metadata)
								if ok && p == 100 {
									full = true
								}
								if full && ok && p == 0 {
									spent = true
								}
							}
						case *event.EventSkillAction:
							if v.SkillId == tc.skill && !v.IsFallback {
								casts++
							}
						}
					}
					if casts >= tc.casts && full && spent {
						t.Logf("casts=%d, full=%t, spent=%t", casts, full, spent)
						return
					}
				case <-ctx.Done():
					t.Fatalf("incomplete replay casts=%d full=%t spent=%t", casts, full, spent)
				}
			}
		})
	}
}
