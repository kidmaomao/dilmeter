package main

import (
	"encoding/json"
	"gitlab.com/prilus/mabidilmeter/lib/event"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTargetHealthPreviewSurvivesLiveFrames(t *testing.T) {
	preserveBossPreviewState(t)
	skillOverlayState.Lock()
	old := skillOverlayState.healthPreview
	skillOverlayState.healthPreview = nil
	skillOverlayState.Unlock()
	t.Cleanup(func() { skillOverlayState.Lock(); skillOverlayState.healthPreview = old; skillOverlayState.Unlock() })
	for _, kind := range []string{"ancient-giant", "miel-shard"} {
		start := int64(100000)
		preview := nativeTargetHealthOverlayItem{Kind: kind, EntityID: "preview", CurrentHealth: 60, MaximumHealth: 100, X: 500, Y: 200, PreviewEndsAt: start + 8000, Generation: 1}
		send := func(target *nativeTargetHealthOverlayItem, at int64) *nativeTargetHealthOverlayItem {
			data, _ := json.Marshal(nativeSkillOverlayMessage{TargetHealth: target})
			if !setSkillOverlayStateAt(data, at) {
				t.Fatal("state rejected")
			}
			return readBurstPreviewState(t).TargetHealth
		}
		send(&preview, start)
		for _, elapsed := range []int64{150, 1000, 7000, 7999} {
			got := send(nil, start+elapsed)
			if got == nil || got.Kind != kind || got.PreviewEndsAt != start+8000 {
				t.Fatalf("preview overwritten at %d: %+v", elapsed, got)
			}
		}
		live := &nativeTargetHealthOverlayItem{EntityID: "real", CurrentHealth: 20, MaximumHealth: 100}
		if got := send(live, start+8000); got == nil || got.EntityID != "real" {
			t.Fatal("real target did not resume")
		}
		if got := send(&preview, start+8100); got != nil {
			t.Fatal("expired echoed preview resurrected")
		}
		preview.Generation = 2
		preview.PreviewEndsAt = start + 17000
		send(&preview, start+9000)
		preview.PreviewEndsAt = start + 18000
		if got := send(&preview, start+10000); got.PreviewEndsAt != start+17000 {
			t.Fatal("same generation extended deadline")
		}
		send(nil, start+17000)
	}
}

func TestSelfManaFollowsLocalIdentityAndResets(t *testing.T) {
	r, sounds := healerFixture(t)
	r.healer.settings.Members = []healerMemberSelection{{Self: true, Mana: true}}
	mp := func(id string, at int64, value float64) {
		r.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: id, At: at}, Stats: []event.EventStatUpdateEntry{{StatId: 32, Value: value}, {StatId: 34, Value: 500}, {StatId: 35, Value: 500}}})
	}
	mp("self", 101, 200)
	healerTick(r, 101000)
	healerTick(r, 101600)
	var own healerMemberState
	for _, m := range r.healer.state.Members {
		if m.Key == "self" {
			own = m
		}
	}
	if own.ID != "self" || own.Name != "自己" || own.ManaPercent == nil || *own.ManaPercent != 20 || len(r.healer.state.Cards) != 1 || r.healer.state.Cards[0].Category != "mana" || len(*sounds) != 1 {
		t.Fatalf("self not observed: %+v", r.healer.state)
	}
	stored := healerFavoriteMembers(normalizeHealerSettings(r.healer.settings).Members)
	if len(stored) != 1 || !stored[0].Self || stored[0].ID != "" {
		t.Fatal("self setting not persistent or saved an old entity ID")
	}
	r.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "next", At: 102}, Reset: true})
	healerTick(r, 102000)
	if len(r.healer.state.Cards) != 0 || r.healer.state.Members[0].ManaPercent != nil {
		t.Fatal("old character mana leaked across login")
	}
	r.onEvent(liveTestAppear("next", "新角色", 10001, 103))
	mp("next", 103, 600)
	healerTick(r, 103000)
	own = r.healer.state.Members[0]
	if own.Key != "self" || own.ID != "next" || own.Name != "新角色" || own.ManaPercent == nil || *own.ManaPercent != 60 {
		t.Fatalf("did not follow new local character: %+v", own)
	}
}

func TestManaPreviewUsesManaLayoutForEightSeconds(t *testing.T) {
	r, _ := healerFixture(t)
	nativeReminderRuntimeHolder.Lock()
	old := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = r
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = old
		nativeReminderRuntimeHolder.Unlock()
	})
	request := httptest.NewRequest("POST", "/api/healer_monitor/text_preview", strings.NewReader(`{"kind":"mana","member":{"self":true,"mana":true,"manaSettings":{"overlay":{"x":350,"y":200}}},"text":{"fontSize":16},"iconSize":30}`))
	request.RemoteAddr = "127.0.0.1:8000"
	response := httptest.NewRecorder()
	handleHealerTextPreview(response, request)
	if response.Code != 204 {
		t.Fatal(response.Body.String())
	}
	deadline := r.healer.previewUntilMs
	for _, at := range []int64{deadline - 7900, deadline - 1000, deadline - 1} {
		healerTick(r, at)
		frame := r.healer.overlaySnapshot(at)
		if !frame.Preview || len(frame.Groups) != 1 || frame.Groups[0].Kind != "mana" || frame.Groups[0].X != 350 || frame.Groups[0].Cards[0].Value != "20%" {
			t.Fatalf("mana preview: %+v", frame)
		}
	}
	if frame := r.healer.overlaySnapshot(deadline); frame.Preview {
		t.Fatal("preview not expired")
	}
}
