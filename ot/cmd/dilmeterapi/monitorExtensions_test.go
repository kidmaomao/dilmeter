package main

import (
	"context"
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestHealerManaIndependentMissingStaleRecovery(t *testing.T) {
	r, sounds := healerFixture(t)
	r.healer.settings.Members = []healerMemberSelection{{ID: "ally", Mana: true, ManaSettings: &healerHealthSettings{Threshold: 25, SoundEnabled: true, Sound: healerSound{Kind: "electronic"}, RepeatCount: 2, RepeatIntervalSeconds: 2, Overlay: healerPosition{Enabled: true, X: -150, Y: 90}}}}
	r.onEvent(healerHP("ally", 100, 1000, 1000))
	healerTick(r, 100000)
	healerTick(r, 100600)
	if len(*sounds) != 0 || r.healer.state.Members[0].ManaPercent != nil {
		t.Fatal("life data fabricated mana")
	}
	mp := func(at int64, entries ...event.EventStatUpdateEntry) {
		r.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: "ally", At: at}, Stats: entries})
	}
	mp(101, event.EventStatUpdateEntry{StatId: 32, Value: 200}, event.EventStatUpdateEntry{StatId: 34, Value: 500})
	healerTick(r, 101000)
	if r.healer.state.Members[0].ManaPercent != nil {
		t.Fatal("missing bonus fabricated a maximum")
	}
	mp(101, event.EventStatUpdateEntry{StatId: 35, Value: 500})
	healerTick(r, 101000)
	healerTick(r, 101250)
	healerTick(r, 101750)
	healerTick(r, 103750)
	if len(*sounds) != 2 || *r.healer.state.Members[0].ManaPercent != 20 || len(r.healer.state.Cards) != 1 || r.healer.state.Cards[0].Category != "mana" {
		t.Fatalf("mana alerts: %+v sounds %d", r.healer.state, len(*sounds))
	}
	frame := r.healer.overlaySnapshot(103750)
	if len(frame.Groups) != 1 || frame.Groups[0].Kind != "mana" || frame.Groups[0].X != -150 {
		t.Fatalf("mana was grouped as Buff: %+v", frame)
	}
	mp(132, event.EventStatUpdateEntry{StatId: 28, Value: 1000})
	healerTick(r, 132000)
	if r.healer.state.Members[0].ManaState != "stale" || len(r.healer.state.Cards) != 0 {
		t.Fatal("stale mana remained visible")
	}
	mp(133, event.EventStatUpdateEntry{StatId: 32, Value: 0})
	healerTick(r, 133000)
	healerTick(r, 133600)
	if len(*sounds) != 2 {
		t.Fatal("same episode replenished after stale data")
	}
	mp(134, event.EventStatUpdateEntry{StatId: 32, Value: 310})
	healerTick(r, 134000)
	mp(135, event.EventStatUpdateEntry{StatId: 32, Value: 0})
	healerTick(r, 135000)
	healerTick(r, 135500)
	if len(*sounds) != 3 || *r.healer.state.Members[0].ManaPercent != 0 {
		t.Fatal("real zero mana or recovery did not start a new episode")
	}
	r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 136}})
	healerTick(r, 136000)
	if len(r.healer.state.Cards) != 0 {
		t.Fatal("departed teammate still visible")
	}
}

func TestHealerHealthIncludesObservedMaximumBonus(t *testing.T) {
	r, _ := healerFixture(t)
	r.onEvent(healerHP("ally", 100, 250, 500))
	r.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: "ally", At: 100}, Stats: []event.EventStatUpdateEntry{{StatId: 31, Value: 500}}})
	healerTick(r, 100000)
	if got := r.healer.state.Members[0].HealthPercent; got == nil || *got != 25 {
		t.Fatalf("bonus omitted: %v", got)
	}
}

func TestManaSettingsTemplatesAndOldProfiles(t *testing.T) {
	s := normalizeHealerSettings(healerSettings{Members: []healerMemberSelection{{Name: "A", Favorite: true, Mana: true, ManaSettings: &healerHealthSettings{Threshold: 35, RepeatCount: 3, RepeatIntervalSeconds: 8, Overlay: healerPosition{X: -90, Y: 200}}}}, Templates: []healerMemberTemplate{{ID: "m", Name: "Mana", Mana: true, ManaSettings: &healerHealthSettings{Threshold: 15}}}})
	s.Members = healerFavoriteMembers(s.Members)
	data, _ := json.Marshal(s)
	var restored healerSettings
	_ = json.Unmarshal(data, &restored)
	restored = normalizeHealerSettings(restored)
	if !restored.Members[0].Mana || restored.Members[0].ManaSettings.Threshold != 35 || restored.Members[0].ManaSettings.Overlay.X != -90 || !restored.Templates[0].Mana || restored.Templates[0].ManaSettings.Threshold != 15 {
		t.Fatal("mana preferences lost on persistence")
	}
	old := normalizeHealerMember(healerMemberSelection{Name: "old"}, defaultHealerSettings(), 0)
	if old.Mana || old.ManaSettings == nil || old.ManaSettings.Sound.Kind != "electronic" {
		t.Fatal("old settings enabled new alerts")
	}
}

func TestGreenCurtainPersistentTransitionAndIndependentAppearance(t *testing.T) {
	preserveBossPreviewState(t)
	var sounds []nativeReminderSoundRequest
	r := newGreenCurtainTestRuntime(&sounds)
	rule := r.settings.BossMechanics.Rules[greenCurtainKey]
	rule.AlwaysVisible = true
	rule.PersistentStyle = "bar"
	rule.PersistentX = -120
	rule.PersistentY = 77
	rule.PersistentScale = 125
	rule.PersistentOpacity = 65
	rule.X = 700
	rule.Y = 200
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	greenCurtainAppear(r, "boss", 7615, 100)
	r.evaluateGreenCurtain(time.UnixMilli(101000))
	if !r.bossMechanics[greenCurtainKey].TimingUnknown {
		t.Fatal("opening fabricated a countdown")
	}
	greenCurtainObserved(r, "first", 107000, "200", false)
	r.evaluateGreenCurtain(time.UnixMilli(108000))
	r.publishNativeSkillState(time.UnixMilli(108000))
	m := readBurstPreviewState(t).Mechanics
	if len(m) != 1 || m[0].PassiveStyle != "bar" || m[0].X != -120 || m[0].ScalePercent != 125 || m[0].OpacityPercent != 65 || m[0].EndsAtMs != 170000 || len(sounds) != 1 {
		t.Fatalf("persistent appearance %+v sounds%d", m, len(sounds))
	}
	r.evaluateGreenCurtain(time.UnixMilli(160000))
	r.publishNativeSkillState(time.UnixMilli(160000))
	m = readBurstPreviewState(t).Mechanics
	if len(m) != 1 || m[0].PassiveStyle != "" || m[0].X != 700 || m[0].Y != 200 || len(sounds) != 2 {
		t.Fatalf("warning transition %+v", m)
	}
	r.evaluateGreenCurtain(time.UnixMilli(161000))
	if len(sounds) != 2 {
		t.Fatal("duplicate warning")
	}
	r.evaluateGreenCurtain(time.UnixMilli(170000))
	if !r.bossMechanics[greenCurtainKey].Passive {
		t.Fatal("next cycle not persistent")
	}
	rule.ShowCountdown = false
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	r.evaluateGreenCurtain(time.UnixMilli(223000))
	if !r.bossMechanics[greenCurtainKey].Passive {
		t.Fatal("always-visible display disappeared when warning card disabled")
	}
	r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "boss", At: 224}})
	r.evaluateGreenCurtain(time.UnixMilli(224000))
	if len(r.bossMechanics) != 0 {
		t.Fatal("curtain survived encounter")
	}
}

func TestAncientGiantUsesOnlyLocalSelection(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{AncientGiantHealthEnabled: true}}, &sounds)
	r.localID = "self"
	for i, id := range []string{"a", "b", "c", "d"} {
		r.onEvent(liveTestAppear(id, "giant", uint32(7611+i), 100))
		r.onEvent(healerHP(id, 100, float64(200+i), 1000))
	}
	selectTarget := func(owner, target string) {
		r.onEvent(&event.EventCombatTarget{EventBase: event.EventBase{EventId: 19, Id: owner, At: 100}, TargetId: target})
	}
	if r.selectedTargetHealth() != nil {
		t.Fatal("unselected giant appeared")
	}
	selectTarget("self", "b")
	if got := r.selectedTargetHealth(); got == nil || got.EntityID != "b" || got.CurrentHealth != 201 {
		t.Fatalf("wrong target %+v", got)
	}
	r.onEvent(&event.EventDamage{EventBase: event.EventBase{EventId: 3, Id: "self", At: 100}, TargetId: "c", Damage: 100})
	selectTarget("other", "d")
	if r.selectedTargetHealth().EntityID != "b" {
		t.Fatal("AOE/teammate changed selection")
	}
	selectTarget("self", "")
	if r.selectedTargetHealth() != nil {
		t.Fatal("cancelled target still visible")
	}
	selectTarget("self", "a")
	r.onEvent(healerHP("a", 101, 0, 1000))
	if r.selectedTargetHealth() != nil {
		t.Fatal("dead giant still visible")
	}
}

func TestBrontasRocksActualSocketsRemovalAndRotation(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{BrontasRocks: nativeBrontasRockSettings{Enabled: true, Rotation: 90, ScalePercent: 100}}}, &sounds)
	r.onEvent(liveTestAppear("boss", "brontas", 7602, 100))
	r.onEvent(healerHP("boss", 100, 500000, 1000000))
	spawn := func(id string, x, y float64) {
		r.onEvent(&event.EventWorldProp{EventBase: event.EventBase{EventId: 24, Id: id, At: 101}, AtMs: 101000, PropID: 49205, RegionID: 35027, X: x, Y: y, State: "landing", Active: true})
	}
	spawn("random", 25231, 35028)
	if r.brontasRockOverlay(time.Unix(101, 0)) != nil {
		t.Fatal("random mineral marked as mechanic")
	}
	spawn("west", 20756, 36745)
	spawn("north", 25156, 41145)
	got := r.brontasRockOverlay(time.Unix(102, 0))
	if got == nil || !reflect.DeepEqual(got.Rocks, []nativeBrontasRockMarker{{Clock: 3}, {Clock: 12}}) {
		t.Fatalf("two observed stones/rotation %+v", got)
	}
	r.onEvent(&event.EventWorldProp{EventBase: event.EventBase{EventId: 24, Id: "west", At: 103}})
	if len(r.brontasRockOverlay(time.Unix(103, 0)).Rocks) != 1 {
		t.Fatal("removed rock retained")
	}
	spawn("north", 25231, 35028)
	if r.brontasRockOverlay(time.Unix(104, 0)) != nil {
		t.Fatal("reused prop ID retained old marker")
	}
	spawn("south", 25156, 32345)
	if r.brontasRockOverlay(time.Unix(147, 0)) != nil {
		t.Fatal("lost removal never expires")
	}
}

// Full local wire replay includes the actual parser and event publisher. No
// capture or identifying raw record is checked into the repository.
func TestMonitorExtensionsOctoberCapture(t *testing.T) {
	path := os.Getenv("DILMETER_MONITOR_PCAP")
	if path == "" {
		t.Skip("set DILMETER_MONITOR_PCAP to October 20 capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	events := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, events)
	<-events
	var sounds []nativeReminderSoundRequest
	r := newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{AncientGiantHealthEnabled: true, BrontasRocks: nativeBrontasRockSettings{Enabled: true}}}, &sounds)
	if err = reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(4 * time.Second)
	defer idle.Stop()
	giants := map[string]bool{}
	clocks := []int{}
	firstUpdates := map[string][]float64{}
	for {
		select {
		case batch := <-events:
			idle.Reset(4 * time.Second)
			for _, e := range batch {
				r.onEvent(e)
				if prop, ok := e.(*event.EventWorldProp); ok && prop.Active {
					if rock, exists := r.brontasRocks[prop.Id]; exists {
						clocks = append(clocks, rock.Clock)
					}
				}
				if target := r.selectedTargetHealth(); target != nil && target.Kind == "ancient-giant" {
					if target.EntityID != r.selectedTargetID {
						t.Fatal("giant not explicitly selected")
					}
					giants[target.EntityID] = true
				}
				if st, ok := e.(*event.EventStatUpdate); ok {
					entity := r.entities[st.Id]
					if entity != nil && entity.RaceID == 7601 && len(firstUpdates[st.Id]) < 3 {
						for _, stat := range st.Stats {
							if stat.StatId == 28 {
								firstUpdates[st.Id] = append(firstUpdates[st.Id], stat.Value)
							}
						}
					}
				}
			}
		case <-idle.C:
			if len(giants) != 24 {
				t.Fatalf("expected all 24 independently selected giants, got %d", len(giants))
			}
			if !reflect.DeepEqual(clocks, []int{9, 12, 3, 6, 12, 6, 3, 6}) {
				t.Fatalf("actual rock sockets: %v", clocks)
			}
			if len(firstUpdates) != 3 {
				t.Fatalf("second-stage IDs %v", firstUpdates)
			}
			for _, hp := range firstUpdates {
				if len(hp) < 2 || hp[1]/hp[0] < .4999 || hp[1]/hp[0] > .5001 {
					t.Fatalf("second stage does not initialize at 50%%: %v", hp)
				}
			}
			t.Logf("24 selected giants, 4 actual two-rock cycles %v, 3 second-stage IDs corrected immediately to 50%%", clocks)
			return
		case <-ctx.Done():
			t.Fatal("capture replay timeout")
		}
	}
}

func TestRemovedBarPreferencesCannotRestoreHiddenRules(t *testing.T) {
	s := nativeReminderSettings{}
	s.SkillCooldowns.Rules = map[uint16]nativeSkillCooldownRule{59047: {BarOnly: true, Enabled: true}, 59041: {Enabled: true, CooldownSeconds: 17}}
	s = normalizeNativeReminderSettings(s)
	if _, ok := s.SkillCooldowns.Rules[59047]; ok {
		t.Fatal("removed bar rule survived normalization")
	}
	if !s.SkillCooldowns.Rules[59041].Enabled || s.SkillCooldowns.Rules[59041].CooldownSeconds != 17 {
		t.Fatal("ordinary reminder changed")
	}
}
