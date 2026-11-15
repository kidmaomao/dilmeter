package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func TestHealerArcanaPresets(t *testing.T) {
	runtime, _ := healerFixture(t)
	h := runtime.healer
	cases := []struct {
		observedSkill uint16
		music         uint32
		ultimate      uint16
		cooldown      float64
	}{
		{59001, 680, 59005, 180},
		{59026, 680, 0, 0},
		{59040, 192, 59047, 36},
		{59060, 680, 59064, 36},
		{59085, 680, 0, 0},
		{59101, 680, 59106, 30},
		{59121, 680, 59123, 25},
		{59144, 192, 59143, 60},
		{59165, 680, 59163, 45},
		{59185, 680, 59187, 30},
	}
	for index, tc := range cases {
		at := int64(101 + index)
		runtime.onEvent(&event.EventDamage{EventBase: event.EventBase{Id: "ally", At: at}, SkillId: tc.observedSkill, TargetId: "self", Damage: 100})
		member := h.withArcanaPreset(healerMemberSelection{Name: "队友", ID: "ally"})
		if !member.BuffSettings.Enabled || len(member.BuffSettings.Rules) != 1 || member.BuffSettings.Rules[0].CCID != tc.music {
			t.Fatalf("skill %d supplied the wrong music: %+v", tc.observedSkill, member.BuffSettings)
		}
		rules := member.SkillSettings.Rules
		if tc.ultimate == 0 {
			if len(rules) != 0 {
				t.Fatalf("skill %d supplied an unexpected ultimate: %+v", tc.observedSkill, rules)
			}
		} else if !member.SkillSettings.Enabled || len(rules) != 1 || rules[0].SkillID != tc.ultimate || rules[0].CooldownSeconds != tc.cooldown {
			t.Fatalf("skill %d supplied the wrong ultimate: %+v", tc.observedSkill, rules)
		}
	}
}

func TestHealerInstanceEditsAndPromotion(t *testing.T) {
	runtime, _ := healerFixture(t)
	h := runtime.healer
	h.settings = healerDungeonSettings()
	h.settings.Templates[0].SkillSettings.Rules = nil
	h.settings.Members = []healerMemberSelection{{Name: "队友", Favorite: true, Included: true}}

	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	save := func(settings healerSettings, session uint64, temporary ...healerMemberSelection) *httptest.ResponseRecorder {
		t.Helper()
		body, err := json.Marshal(struct {
			healerSettings
			Session   uint64                  `json:"dungeonSession"`
			Temporary []healerMemberSelection `json:"temporaryMembers"`
		}{settings, session, temporary})
		if err != nil {
			t.Fatal(err)
		}
		request := httptest.NewRequest(http.MethodPut, "/api/healer_monitor", bytes.NewReader(body))
		request.RemoteAddr = "127.0.0.1:31900"
		response := httptest.NewRecorder()
		handleHealerMonitor(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("save status %d: %s", response.Code, response.Body)
		}
		return response
	}
	enter := func(at int64, region uint32, dungeon bool) {
		runtime.onEvent(&event.EventLocalRegion{EventBase: event.EventBase{Id: "self", At: at}, RegionID: region, Dungeon: dungeon})
	}
	appear := func(at int64, region uint32) {
		entity := liveTestAppear("temp", "临时队友", 9001, at)
		entity.RegionID = region
		runtime.onEvent(entity)
	}
	temporary := func(at int64) healerMemberSelection {
		t.Helper()
		healerTick(runtime, at*1000)
		if len(h.state.TemporaryMembers) != 1 {
			t.Fatalf("expected one temporary teammate, got %+v", h.state.TemporaryMembers)
		}
		return h.state.TemporaryMembers[0]
	}

	enter(101, 35022, true)
	appear(101, 35022)
	runtime.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "temp", At: 101}, SkillId: 59047, SourceId: "temp", AtMs: 101000})
	runtime.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: "ally", At: 101}, SkillId: 59123, SourceId: "ally", AtMs: 101000})
	choice := temporary(101)
	if choice.SkillSettings.Rules[0].CooldownSeconds != 36 || choice.BuffSettings.Rules[0].CCID != 192 ||
		h.settings.Members[0].SkillSettings.Rules[0].CooldownSeconds != 25 {
		t.Fatal("automatic and favorite teammates did not receive their respective Arcana presets")
	}
	if len(h.settings.Templates[0].SkillSettings.Rules) != 0 {
		t.Fatal("class-specific presets leaked into the shared template")
	}

	choice.SkillSettings.Rules = nil // Deliberately remove the suggested skill.
	choice.HealthSettings.Threshold = 72
	choice.Mana = true // An older client can still send a temporary teammate's mana preference.
	session := h.state.DungeonSession
	save(h.settings, session, choice)
	if members := newHealerMonitor(h.settingsPath).settings.Members; len(members) != 1 || members[0].Name != "队友" {
		t.Fatalf("temporary tuning persisted as a favorite: %+v", members)
	}
	enter(102, 35023, true) // A new floor in the same run retains the tuning.
	appear(102, 35023)
	if member := temporary(102); len(member.SkillSettings.Rules) != 0 || member.HealthSettings.Threshold != 72 || member.Mana || member.ManaSettings != nil || h.state.DungeonSession != session {
		t.Fatalf("temporary edits lost during the run: %+v", member)
	}
	enter(103, 4016, false)
	enter(104, 35022, true)
	appear(104, 35022)
	if member := temporary(104); len(member.SkillSettings.Rules) != 1 || member.HealthSettings.Threshold == 72 {
		t.Fatalf("new run reused temporary preferences: %+v", member)
	}
	response := save(h.settings, session, choice)
	if response.Header().Get("X-Dilmeter-Dungeon-Changed") != "1" || temporary(105).HealthSettings.Threshold == 72 {
		t.Fatal("stale save changed the new dungeon's roster")
	}

	settings := h.settings
	choice.Favorite = true
	settings.Members = append(settings.Members, choice)
	save(settings, h.state.DungeonSession)
	healerTick(runtime, 106000)
	if len(h.state.TemporaryMembers) != 0 {
		t.Fatal("promoted member appeared twice in the roster")
	}
	members := newHealerMonitor(h.settingsPath).settings.Members
	if len(members) != 2 || !members[1].Favorite || members[1].ID != "" || members[1].HealthSettings.Threshold != 72 || members[1].Mana || len(members[1].SkillSettings.Rules) != 0 {
		t.Fatalf("promotion did not retain the edits by character name: %+v", members)
	}
}
