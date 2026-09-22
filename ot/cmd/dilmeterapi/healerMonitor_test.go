package main

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func healerFixture(t *testing.T) (*nativeReminderRuntime, *[]nativeReminderSoundRequest) {
	t.Helper()
	sounds := []nativeReminderSoundRequest{}
	runtime := newNativeReminderRuntimeForTest(nativeReminderSettings{}, &sounds)
	runtime.healer = newHealerMonitor(filepath.Join(t.TempDir(), "healer.json"))
	runtime.healer.settings.Enabled = true
	runtime.healer.settings.Members = []healerMemberSelection{{ID: "ally", Health: true, Overture: true, Vivace: true}}
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "self", At: 100}})
	runtime.onEvent(liveTestAppear("self", "自己", 10001, 100))
	runtime.onEvent(liveTestAppear("ally", "队友", 10001, 100))
	return runtime, &sounds
}

func healerHP(id string, at int64, current, maximum float64) *event.EventStatUpdate {
	return &event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: id, At: at}, Stats: []event.EventStatUpdateEntry{{StatId: 28, Value: current}, {StatId: 30, Value: maximum}}}
}

func healerTick(runtime *nativeReminderRuntime, atMs int64) {
	runtime.healer.evaluate(runtime, time.UnixMilli(atMs), true)
}

func TestHealerUnknownAndUnselectedPlayersNeverAlert(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.onEvent(liveTestAppear("other", "未选择角色", 10001, 100))
	runtime.onEvent(healerHP("other", 100, 1, 1000))
	pet := liveTestAppear("pet", "宠物", 10001, 100)
	pet.OwnerId = "self"
	runtime.onEvent(pet)
	runtime.onEvent(healerHP("pet", 100, 1, 1000))
	runtime.onEvent(&event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: "ally", At: 100}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 1000}}})
	healerTick(runtime, 100_000)
	healerTick(runtime, 102_000)
	state := runtime.healer.state
	if len(state.Members) != 2 || len(state.Alerts) != 0 || len(*sounds) != 0 {
		t.Fatalf("unknown/unselected actors raised an alarm: %+v", state)
	}
	ally := state.Members[0]
	if ally.ID != "ally" || ally.HealthPercent != nil || ally.HealthState != "unknown" || ally.Overture.State != "unknown" {
		t.Fatalf("unobserved values were fabricated: %+v", ally)
	}
}

func TestHealerLowHealthDebounceRecoveryAndBackgroundSound(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.onEvent(healerHP("ally", 100, 250, 1000))
	healerTick(runtime, 100_000)
	if len(*sounds) != 0 {
		t.Fatal("low health did not debounce")
	}
	healerTick(runtime, 100_500)
	healerTick(runtime, 103_000)
	if len(*sounds) != 1 || len(runtime.healer.state.Alerts) != 1 {
		t.Fatal("native monitoring must warn once without a WebView")
	}
	runtime.onEvent(healerHP("ally", 104, 440, 1000))
	healerTick(runtime, 104_000)
	if runtime.healer.state.Members[0].HealthState != "low" {
		t.Fatal("threshold jitter rearmed a low-health warning")
	}
	runtime.onEvent(healerHP("ally", 105, 600, 1000))
	healerTick(runtime, 105_000)
	if len(runtime.healer.state.Alerts) != 0 {
		t.Fatal("healing did not clear the warning")
	}
	runtime.onEvent(healerHP("ally", 106, 390, 1000))
	healerTick(runtime, 106_000)
	healerTick(runtime, 106_500)
	if len(*sounds) != 2 {
		t.Fatal("a new low-health episode did not warn")
	}
	// Unrelated live traffic does not make old HP fresh again.
	runtime.onEvent(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "self", At: 137}})
	healerTick(runtime, 137_000)
	if runtime.healer.state.Members[0].HealthState != "stale" || runtime.healer.state.Members[0].HealthPercent != nil || len(runtime.healer.state.Alerts) != 0 {
		t.Fatal("stale HP remained actionable")
	}
}

func TestHealerRecipientSongsExpiryAndRefresh(t *testing.T) {
	runtime, sounds := healerFixture(t)
	enable := func(id string, cc uint32, at, end int64) {
		runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: id, At: at}, CCId: cc, DisableAt: end, AttackerId: "self"})
	}
	enable("ally", 680, 100, 120)
	enable("self", 192, 100, 160)
	healerTick(runtime, 100_000)
	if runtime.healer.state.Members[0].Vivace.State != "unknown" {
		t.Fatal("caster's Buff leaked onto recipient")
	}
	healerTick(runtime, 110_000)
	healerTick(runtime, 111_200)
	if len(*sounds) != 1 || runtime.healer.state.Members[0].Overture.State != "expiring" {
		t.Fatal("song expiry warning missing")
	}
	enable("ally", 680, 112, 180)
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 112}, CCId: 680})
	healerTick(runtime, 112_000)
	if runtime.healer.state.Members[0].Overture.State != "active" || len(runtime.healer.state.Alerts) != 0 {
		t.Fatal("refresh/remove ordering produced a false missing-song alarm")
	}
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 114}, CCId: 680})
	healerTick(runtime, 114_000)
	healerTick(runtime, 115_200)
	if len(*sounds) != 2 || runtime.healer.state.Members[0].Overture.State != "missing" {
		t.Fatal("confirmed removal did not warn")
	}
	enable("ally", 192, 116, 117)
	healerTick(runtime, 116_000)
	healerTick(runtime, 118_000)
	if runtime.healer.state.Members[0].Vivace.State != "missing" {
		t.Fatal("expired Buff stayed active without a removal packet")
	}
}

func TestHealerVisibilityReconnectAndDisabledCapture(t *testing.T) {
	runtime, sounds := healerFixture(t)
	runtime.onEvent(healerHP("ally", 100, 100, 1000))
	healerTick(runtime, 100_000)
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 101}})
	healerTick(runtime, 101_000)
	runtime.onEvent(liveTestAppear("ally", "队友", 10001, 102))
	healerTick(runtime, 102_000)
	if runtime.healer.state.Members[0].HealthState != "unknown" || len(*sounds) != 0 {
		t.Fatal("reappearance reused old HP")
	}
	runtime.onEvent(healerHP("ally", 103, 100, 1000))
	runtime.healer.evaluate(runtime, time.Unix(104, 0), false)
	if runtime.healer.state.Capturing || len(runtime.healer.state.Alerts) != 0 || len(*sounds) != 0 {
		t.Fatal("capture loss did not suspend warnings")
	}
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "self", At: 105}, Reset: true})
	healerTick(runtime, 105_000)
	if len(runtime.healer.settings.Members) != 0 || len(runtime.healer.observed) != 0 {
		t.Fatal("connection reset kept old character selections")
	}
}

func TestHealerSettingsPersistenceAndHTTP(t *testing.T) {
	runtime, _ := healerFixture(t)
	healerTick(runtime, 100_000)
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	t.Cleanup(func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	})
	request := func(method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/healer_monitor", strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:10000"
		w := httptest.NewRecorder()
		handleHealerMonitor(w, r)
		return w
	}
	body := `{"enabled":true,"lowHealthPercent":35,"buffWarningSeconds":8,"soundEnabled":false,"volume":55,"members":[{"id":"ally","health":true,"vivace":true}]}`
	if w := request("PUT", body); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := request("GET", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"lowHealthPercent":35`) {
		t.Fatal(w.Body.String())
	}
	loaded := newHealerMonitor(runtime.healer.settingsPath)
	if loaded.settings.LowHealthPercent != 35 || loaded.settings.SoundEnabled || len(loaded.settings.Members) != 0 {
		t.Fatal("preferences were lost or connection-specific members were persisted")
	}
	for _, invalid := range []string{`{`, body + `{}`, strings.Repeat(" ", 1025<<10) + body} {
		if request("PUT", invalid).Code != 400 {
			t.Fatal("invalid settings accepted")
		}
	}
	if request("POST", "").Code != 405 {
		t.Fatal("unsupported method accepted")
	}
	runtime.healer.settingsPath = filepath.Join(t.TempDir(), "missing", "healer.json")
	if request("PUT", `{"enabled":false}`).Code != 500 || !runtime.healer.settings.Enabled {
		t.Fatal("failed persistence changed running preferences")
	}
	data, _ := json.Marshal(loaded.settings)
	if !json.Valid(data) {
		t.Fatal("invalid settings JSON")
	}
}

func TestHealerRevivalNeedsFreshHealthButNotReappearance(t *testing.T) {
	runtime, _ := healerFixture(t)
	runtime.onEvent(healerHP("ally", 100, 800, 1000))
	runtime.onEvent(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "ally", At: 101}})
	healerTick(runtime, 101_000)
	if runtime.healer.state.Members[0].Active {
		t.Fatal("old HP revived a finished player")
	}
	runtime.onEvent(healerHP("ally", 102, 200, 1000))
	healerTick(runtime, 102_000)
	if !runtime.healer.state.Members[0].Active || runtime.healer.state.Members[0].HealthState != "low" {
		t.Fatal("fresh HP did not resume monitoring after in-place revival")
	}
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "ally", At: 103}})
	runtime.onEvent(healerHP("ally", 104, 200, 1000))
	healerTick(runtime, 104_000)
	if runtime.healer.state.Members[0].Active {
		t.Fatal("late HP resurrected an out-of-view player")
	}
}

func TestHealerCustomBuffAndIndependentSoundCategories(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	h.settings.Buffs = []healerBuffRule{{CCID: 681, Name: "忍耐之歌", WarningSeconds: 8}, {CCID: 123456, Name: "未观测", WarningSeconds: 5}}
	h.settings.Members[0].Buffs = []uint32{681, 123456}
	h.settings.HealthSound = healerSound{Kind: "custom", SoundID: "chosen-health-audio"}
	h.settings.BuffSound = healerSound{Kind: "skill-ready"}
	runtime.onEvent(healerHP("ally", 100, 200, 1000))
	for _, cc := range []uint32{680, 681} {
		runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: cc, DisableAt: 105})
	}
	healerTick(runtime, 100_000)
	healerTick(runtime, 100_500)
	healerTick(runtime, 101_500)
	healerTick(runtime, 102_500)
	healerTick(runtime, 104_500)
	if len(*sounds) != 3 || (*sounds)[0].SoundID != "chosen-health-audio" || (*sounds)[1].Kind != "healer-music" || (*sounds)[2].Kind != "skill-ready" {
		t.Fatalf("simultaneous warning categories lost their selected sound: %+v", *sounds)
	}
	if len(h.state.Alerts) != 3 || h.state.Members[0].Buffs[123456].State != "unknown" {
		t.Fatalf("custom Buff state was fabricated: %+v", h.state)
	}
	healerTick(runtime, 107_000)
	if len(*sounds) != 3 {
		t.Fatal("continued low/expired state repeated sound")
	}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 108}, CCId: 681})
	healerTick(runtime, 108_000)
	if value := h.state.Members[0].Buffs[681]; value.State != "active" || value.RemainingSeconds != nil {
		t.Fatalf("duration was invented: %+v", value)
	}
	runtime.onEvent(&event.EventCharacterConditionDisable{EventBase: event.EventBase{EventId: 5, Id: "ally", At: 110}, CCId: 681})
	healerTick(runtime, 110_000)
	healerTick(runtime, 111_500)
	if len(*sounds) != 4 {
		t.Fatal("custom Buff did not rearm after renewal")
	}
	// Removing a member clears all of their live warnings, including text.
	h.settings.Members = nil
	healerTick(runtime, 112_000)
	if len(h.state.Alerts) != 0 {
		t.Fatal("removed member still warned")
	}
}

func TestHealerTextIndependentOfSoundAndPreview(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	h.settings.SoundEnabled = false
	h.settings.Text = healerTextSettings{Enabled: true, FontSize: 36, X: -500, Y: 220, Width: 480}
	runtime.onEvent(healerHP("ally", 100, 200, 1000))
	healerTick(runtime, 100_000)
	healerTick(runtime, 100_500)
	settings, lines, preview := h.textSnapshot(100_500)
	if len(*sounds) != 0 || preview || len(lines) != 1 || (lines[0].Name != "队友" || lines[0].Title != "血量≤40%" || lines[0].Value != "20%") || settings.FontSize != 36 || settings.X != -500 {
		t.Fatalf("text warning ignored choices: %+v %v", settings, lines)
	}
	h.settings.Text.Enabled = false
	if _, lines, _ := h.textSnapshot(100_500); len(lines) != 0 {
		t.Fatal("text disable ignored")
	}
	h.previewText = healerTextSettings{FontSize: 48, X: 10, Y: 20, Width: 800}
	h.previewUntilMs = 108_000
	if settings, lines, preview := h.textSnapshot(101_000); !preview || len(lines) != 2 || settings.FontSize != 48 {
		t.Fatal("preview did not use draft appearance")
	}
	if _, lines, preview := h.textSnapshot(108_000); preview || len(lines) != 0 {
		t.Fatal("preview did not expire")
	}
	h.settings.Text.Enabled = true
	if _, lines, _ := h.textSnapshot(110_000); len(lines) != 0 {
		t.Fatal("stale text survived stopped runtime")
	}
}

func TestHealerNewPreferencesNormalization(t *testing.T) {
	s := defaultHealerSettings()
	s.Buffs = []healerBuffRule{{CCID: 681, Name: "忍耐之歌", WarningSeconds: 8}, {CCID: 681}, {CCID: 680}, {CCID: 0, Name: "中毒"}}
	s.Members = []healerMemberSelection{{ID: "ally", Included: true}, {ID: "buff-only", Buffs: []uint32{681, 681, 99999}}, {ID: "other"}}
	s.Text = healerTextSettings{Enabled: true, FontSize: 999, X: -1200, Y: 250, Width: 1}
	s = normalizeHealerSettings(s)
	if len(s.Buffs) != 2 || len(s.Members) != 2 || len(s.Members[1].Buffs) != 1 || s.Text.FontSize != 24 || s.Text.X != -1200 || s.Text.Width != 660 {
		t.Fatalf("invalid or duplicate preferences survived: %+v", s)
	}
}

func TestHealerRuleManualDurationFlashAndSoundAreIndependent(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	flashAt := 25
	rule := normalizeHealerBuffRule(healerBuffRule{CCID: 681, Name: "忍耐之歌", DurationMode: "manual", ManualDurationSeconds: 30, WarningSeconds: 5, FlashSeconds: &flashAt}, healerSound{Kind: "healer-buff"})
	h.settings.Buffs = []healerBuffRule{rule}
	h.settings.Members[0].Buffs = []uint32{681}
	runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: 681})
	healerTick(runtime, 100_000)
	if len(h.state.Cards) != 1 || h.state.Cards[0].Value != "30s" || h.state.Cards[0].Flash {
		t.Fatalf("active manual card incorrect: %+v", h.state.Cards)
	}
	healerTick(runtime, 105_000)
	healerTick(runtime, 106_500)
	if !h.state.Cards[0].Flash || len(*sounds) != 0 {
		t.Fatal("visual threshold prematurely played audio")
	}
	healerTick(runtime, 125_000)
	if len(*sounds) != 1 || (*sounds)[0].Kind != "healer-buff" {
		t.Fatal("earlier visual warning consumed sound latch")
	}
	healerTick(runtime, 130_000)
	if h.state.Members[0].Buffs[681].State != "missing" || len(*sounds) != 1 {
		t.Fatal("expiry should retain missing card without repeating audio")
	}
}

func TestHealerEachSongKeepsItsOwnSoundAndIconSetting(t *testing.T) {
	runtime, sounds := healerFixture(t)
	h := runtime.healer
	h.settings.OvertureRule.Sound = &healerSound{Kind: "custom", SoundID: "overture-clip"}
	h.settings.VivaceRule.Sound = &healerSound{Kind: "custom", SoundID: "vivace-clip"}
	hide := false
	h.settings.OvertureRule.OverlayEnabled = &hide
	for _, cc := range []uint32{680, 192} {
		runtime.onEvent(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "ally", At: 100}, CCId: cc, DisableAt: 108})
	}
	healerTick(runtime, 100_000)
	healerTick(runtime, 101_500)
	healerTick(runtime, 103_500)
	if len(*sounds) != 2 || (*sounds)[0].SoundID != "overture-clip" || (*sounds)[1].SoundID != "vivace-clip" {
		t.Fatalf("per-song sounds were coalesced: %+v", *sounds)
	}
	if len(h.state.Cards) != 1 || h.state.Cards[0].CCID != 192 {
		t.Fatalf("hidden card still rendered: %+v", h.state.Cards)
	}
}

func TestHealerOldSettingsMigrateToPerRuleControls(t *testing.T) {
	s := defaultHealerSettings()
	s.HealthSound.Kind = "healer-angel"
	s.BuffWarningSeconds = 25
	s.MusicSound = healerSound{Kind: "custom", SoundID: "saved-music"}
	s.Buffs = []healerBuffRule{{CCID: 681, Name: "忍耐之歌", WarningSeconds: 12}}
	s.BuffSound = healerSound{Kind: "custom", SoundID: "saved-buff"}
	s = normalizeHealerSettings(s)
	if s.HealthSound.Kind != "healer-health" || s.OvertureRule.WarningSeconds != 25 || s.VivaceRule.Sound.SoundID != "saved-music" || s.Buffs[0].Sound.SoundID != "saved-buff" || *s.Buffs[0].FlashSeconds != 12 {
		t.Fatalf("legacy preferences lost: %+v", s)
	}
}
