package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"testing"
	"time"
)

func newGreenCurtainTestRuntime(sounds *[]nativeReminderSoundRequest) *nativeReminderRuntime {
	return newNativeReminderRuntimeForTest(nativeReminderSettings{BossMechanics: nativeBossMechanicSettings{Volume: 80, Rules: map[string]nativeBossMechanicRule{greenCurtainKey: {Key: greenCurtainKey, Enabled: true, ShowCountdown: true, SoundMode: "dedicated"}}}}, sounds)
}
func greenCurtainAppear(r *nativeReminderRuntime, id string, race uint32, at int64) {
	r.onEvent(&event.EventEntityAppear{EventBase: event.EventBase{Id: id, At: at}, RaceId: race})
}
func greenCurtainObserved(r *nativeReminderRuntime, id string, ms int64, height string, snapshot bool) {
	greenCurtainAppear(r, id, 7622, ms/1000)
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: id, At: ms / 1000}, AtMs: ms, CCId: 117, Metadata: "HGHT:f:" + height + ";DURA:4:250;ABSPOS:b:false;", Snapshot: snapshot})
}
func TestGreenCurtainOpeningAndActualAnchor(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newGreenCurtainTestRuntime(&sounds)
	greenCurtainAppear(r, "boss", 7615, 100)
	if len(sounds) != 1 || sounds[0].Kind != "boss-miel-green-curtain" || !sounds[0].Priority {
		t.Fatal("opening must immediately announce once")
	}
	greenCurtainAppear(r, "boss", 7615, 102)
	greenCurtainObserved(r, "decoy", 103000, "250", false)
	greenCurtainObserved(r, "snapshot", 104000, "200", true)
	r.evaluateBossMechanics(time.UnixMilli(170000))
	if len(sounds) != 1 || len(r.bossMechanics) != 0 {
		t.Fatal("spawn/decoy/snapshot started a false clock")
	}
	// Ten seconds of invulnerability, then first curtain at countdown 14:53.
	greenCurtainObserved(r, "first", 117123, "200", false)
	r.evaluateBossMechanics(time.UnixMilli(170122))
	if len(sounds) != 1 {
		t.Fatal("second warning early")
	}
	r.evaluateBossMechanics(time.UnixMilli(170123))
	r.evaluateBossMechanics(time.UnixMilli(170200))
	if len(sounds) != 2 || r.bossMechanics[greenCurtainKey].EndsAtMs != 180123 {
		t.Fatal("second curtain must be 13:50, warning 14:00")
	}
	r.resetSettingsDependentState()
	r.evaluateBossMechanics(time.UnixMilli(175000))
	if len(sounds) != 2 || r.bossMechanics[greenCurtainKey].EndsAtMs != 180123 {
		t.Fatal("settings save moved phase or replayed audio")
	}
	r.evaluateBossMechanics(time.UnixMilli(180123))
	if len(r.bossMechanics) != 0 {
		t.Fatal("expired overlay remains")
	}
	greenCurtainObserved(r, "second", 180423, "200", false)
	greenCurtainObserved(r, "first", 181000, "200", false)
	r.evaluateBossMechanics(time.UnixMilli(233422))
	if len(sounds) != 2 {
		t.Fatal("duplicate entity moved phase")
	}
	r.evaluateBossMechanics(time.UnixMilli(233423))
	if len(sounds) != 3 || r.bossMechanics[greenCurtainKey].EndsAtMs != 243423 {
		t.Fatal("actual curtain did not calibrate next cycle")
	}
}
func TestGreenCurtainLeadAndSkippedCycles(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newGreenCurtainTestRuntime(&sounds)
	rule := r.settings.BossMechanics.Rules[greenCurtainKey]
	lead := 20.5
	rule.WarningSeconds, rule.SoundMode, rule.CustomSoundID = &lead, "custom", "example"
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	greenCurtainAppear(r, "boss", 7615, 100)
	greenCurtainObserved(r, "first", 107000, "200", false)
	for _, ms := range []int64{149499, 300000} {
		r.evaluateBossMechanics(time.UnixMilli(ms))
	}
	if len(sounds) != 1 {
		t.Fatal("early or expired warning replayed")
	}
	r.evaluateBossMechanics(time.UnixMilli(338500))
	if len(sounds) != 2 || sounds[1].SoundID != "example" || r.bossMechanics[greenCurtainKey].EndsAtMs != 359000 {
		t.Fatal("custom lead/late tick drift")
	}
}
func TestGreenCurtainLifecycle(t *testing.T) {
	for _, stop := range []string{"disappear", "finish", "zero-health", "zone-reset", "local-disappear"} {
		t.Run(stop, func(t *testing.T) {
			var sounds []nativeReminderSoundRequest
			r := newGreenCurtainTestRuntime(&sounds)
			r.localID = "player"
			greenCurtainAppear(r, "third-boss", 7603, 100)
			greenCurtainObserved(r, "unrelated", 107000, "200", false)
			if len(sounds) != 0 || r.greenCurtain.BossID != "" {
				t.Fatal("other boss triggered curtain")
			}
			greenCurtainAppear(r, "boss", 7615, 200)
			greenCurtainObserved(r, "first", 207000, "200", false)
			switch stop {
			case "disappear":
				r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "boss", At: 210}})
			case "finish":
				r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "boss", At: 210}})
			case "zero-health":
				r.entities["boss"].Defeated = true
			case "zone-reset":
				r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "player", At: 210}, Reset: true})
			case "local-disappear":
				r.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "player", At: 210}})
			}
			r.evaluateBossMechanics(time.UnixMilli(260000))
			if len(sounds) != 1 || r.greenCurtain.BossID != "" {
				t.Fatal("warning survived exit")
			}
			greenCurtainAppear(r, "new-boss", 7615, 300)
			if len(sounds) != 2 {
				t.Fatal("new encounter opening missing")
			}
		})
	}
}
func TestGreenCurtainDisableMuteEnable(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := newGreenCurtainTestRuntime(&sounds)
	rule := r.settings.BossMechanics.Rules[greenCurtainKey]
	rule.Enabled = false
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	greenCurtainAppear(r, "boss", 7615, 100)
	greenCurtainObserved(r, "first", 107000, "200", false)
	r.evaluateBossMechanics(time.UnixMilli(160000))
	if len(sounds) != 0 || len(r.bossMechanics) != 0 {
		t.Fatal("disabled warned")
	}
	rule.Enabled, rule.SoundMode = true, "none"
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	r.evaluateBossMechanics(time.UnixMilli(223000))
	if len(sounds) != 0 || r.bossMechanics[greenCurtainKey].EndsAtMs != 233000 {
		t.Fatal("mute/midfight enable failed")
	}
	rule.SoundMode = "dedicated"
	r.settings.BossMechanics.Rules[greenCurtainKey] = rule
	r.evaluateBossMechanics(time.UnixMilli(223100))
	if len(sounds) != 0 {
		t.Fatal("unmuting replayed current warning")
	}
	r.evaluateBossMechanics(time.UnixMilli(286000))
	if len(sounds) != 1 {
		t.Fatal("following cycle silent")
	}
}
