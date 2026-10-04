package main

import (
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"testing"
	"time"
)

func TestPowerAwakeningTenSecondsIndependentOfHeldSkill(t *testing.T) {
	for _, releaseAt := range []int64{0, 104000, 114000} {
		t.Run(time.UnixMilli(releaseAt).String(), func(t *testing.T) {
			r, _ := partyFixture(t)
			r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
			signal := func(active bool, at int64) {
				r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: at / 1000}, AtMs: at, SkillId: 58014, Scope: "burst-effect", Active: active})
			}
			signal(true, 100123)
			if releaseAt > 0 && releaseAt < 110123 {
				signal(false, releaseAt)
			} else {
				signal(true, 104000)
			} // Duplicate on must not restart the timer.
			items, _ := r.burstOverlay(time.UnixMilli(110122))
			if len(items) != 1 || items[0].EndsAtMs != 110123 || items[0].StartedAtMs != 100123 || items[0].TimingUnknown {
				t.Fatalf("lost fixed deadline: %+v", items)
			}
			items, _ = r.burstOverlay(time.UnixMilli(110123))
			if len(items) != 0 {
				t.Fatal("Awakening continued past ten seconds")
			}
			if releaseAt > 110123 {
				signal(false, releaseAt)
			}
			items, _ = r.burstOverlay(time.UnixMilli(115000))
			if len(items) != 0 {
				t.Fatal("release revived expired Awakening")
			}
			if r.partySkills["ally"][58014].UsedAtMs != 0 {
				t.Fatal("Power cooldown was created")
			}
		})
	}
}

func TestPowerConditionFallbackAndActualRemoval(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	cc := &event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 100}, CCId: 516, DisableAtMs: 85000}
	r.onEvent(cc)
	items, _ := r.burstOverlay(time.UnixMilli(100000))
	if len(items) != 1 || items[0].EndsAtMs != 110000 || items[0].TimingUnknown {
		t.Fatalf("invalid server expiry did not use ten seconds: %+v", items)
	}
	items, _ = r.burstOverlay(time.UnixMilli(110000))
	if len(items) != 0 {
		t.Fatal("stale CC kept the effect alive")
	}
	r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 111}, CCId: 516})
	cc.At, cc.Snapshot = 120, true
	r.onEvent(cc)
	items, _ = r.burstOverlay(time.UnixMilli(120000))
	if len(items) != 0 {
		t.Fatal("snapshot restarted ten seconds without a start time")
	}
	r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 122}, CCId: 516})
	r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: 130}, AtMs: 130100, SkillId: 58014, Scope: "burst-effect", Active: true})
	cc.At, cc.Snapshot = 130, false
	r.onEvent(cc)
	r.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{Id: "ally", At: 133}, CCId: 516})
	items, _ = r.burstOverlay(time.UnixMilli(133000))
	if len(items) != 0 {
		t.Fatal("actual CC removal was ignored")
	}
}

func TestPowerNewActivationSupersedesOldCondition(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 100}, CCId: 516, DurationMs: 10000})
	r.onEvent(&event.EventSkillState{EventBase: event.EventBase{Id: "ally", At: 105}, AtMs: 105123, SkillId: 58014, Scope: "burst-effect", Active: true})
	items, _ := r.burstOverlay(time.UnixMilli(110500))
	if len(items) != 1 || items[0].EndsAtMs != 115123 {
		t.Fatalf("old CC masked new activation: %+v", items)
	}
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 105}, CCId: 516, DisableAtMs: 110000})
	items, _ = r.burstOverlay(time.UnixMilli(110500))
	if len(items) != 0 {
		t.Fatal("public fallback extended actual CC expiry")
	}
}

func TestPowerKeepsValidMillisecondConditionDeadline(t *testing.T) {
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	r.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{Id: "ally", At: 100}, CCId: 516, DisableAtMs: 110988, DurationMs: 9995})
	items, _ := r.burstOverlay(time.UnixMilli(100993))
	if len(items) != 1 || items[0].EndsAtMs != 110988 || items[0].StartedAtMs != 100993 {
		t.Fatalf("CC precision lost: %+v", items)
	}
}

func TestPublicCollapseCompletionStartsTeammateCooldown(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId, p.lastSentEventAt = 100, time.Now()
	p.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, RaceId: 10001}}
	r, _ := partyFixture(t)
	r.settings.Burst.Enabled, r.settings.Burst.IncludeTeammates = true, true
	feed := func(op packet.OpCode, at int64, msg packet.Message) {
		p.pendingEvents = nil
		p.publishPartyBurstSignal(publicBurstPacket(200, op, at, msg))
		for _, raw := range p.pendingEvents {
			v := raw.(*event.EventSkillState)
			v.Id = "ally"
			r.onEvent(v)
		}
	}
	feed(37011, 100000, packet.Message{packet.NewMessageElemInt(778), packet.NewMessageElemInt(1)})
	feed(28006, 100400, packet.Message{packet.NewMessageElemByte(0)})
	healerTick(r, 100500)
	if r.healer.state.Members[0].Skills[59005].State != "unknown" {
		t.Fatal("cancelled preparation started cooldown")
	}
	feed(37011, 102000, packet.Message{packet.NewMessageElemInt(778), packet.NewMessageElemInt(1)})
	feed(28006, 103500, packet.Message{packet.NewMessageElemByte(0)})
	feed(37011, 103500, packet.Message{packet.NewMessageElemInt(778), packet.NewMessageElemInt(2)})
	feed(37011, 103600, packet.Message{packet.NewMessageElemInt(778), packet.NewMessageElemInt(2)})
	r.onEvent(partyUse("ally", 59005, 103601, false, 9))
	healerTick(r, 104500)
	v := r.healer.state.Members[0].Skills[59005]
	if v.State != "cooling" || v.RemainingSeconds == nil || *v.RemainingSeconds != 9 || r.partySkills["ally"][59005].UsedAtMs != 103500 {
		t.Fatalf("release failed or duplicate restarted cooldown: %+v", v)
	}
	items, _ := r.burstOverlay(time.UnixMilli(104500))
	if len(items) != 0 {
		t.Fatal("completion without CC803 invented an effect or repeated cast")
	}
	healerTick(r, 113500)
	if r.healer.state.Members[0].Skills[59005].State != "ready" {
		t.Fatal("completed Collapse never became ready")
	}
}
