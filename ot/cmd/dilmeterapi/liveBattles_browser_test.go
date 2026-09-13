package main

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"golang.org/x/net/websocket"
)

// Opt-in local fixture for exercising the real Vue/Worker/HTTP flow without
// starting packet capture, native overlays, updates, or a game client.
func TestLiveBattleBrowserHarness(t *testing.T) {
	if os.Getenv("DILMETER_LIVE_BROWSER_TEST") != "1" {
		t.Skip("opt-in browser harness")
	}
	f := newLiveBattleFixture(t)
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 1700000000}, Reliable: true})
	f.add(liveTestAppear("player", "验证角色", 10001, 1700000000))
	initialQuantity := dorchaStat("player", true, 15)
	initialQuantity.At = 1700000000
	f.add(initialQuantity)
	// Real monster packets may carry only a numeric placeholder for Name.
	f.add(liveTestAppear("old", "4767482428237412", 7602, 1700000000))
	f.add(liveTestHP("old", 1700000000))
	for i := 0; i < 5000; i++ {
		f.add(liveTestDamage("old", 1700000001+int64(i/100)))
	}
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 1700000052}, AttackerId: "player"})
	f.add(liveTestAppear("new", "当前场次验证首领", 7615, 1700000053))
	f.add(liveTestHP("new", 1700000053))
	for i := 0; i < 20; i++ {
		f.add(liveTestDamage("new", 1700000054+int64(i)))
	}
	previous := currentLiveBattles.Swap(f.index)
	defer currentLiveBattles.Store(previous)
	var mu sync.Mutex
	reminders, sounds := newDorchaReminderTestRuntime("none")
	delete(reminders.settings.SkillCooldowns.Rules, dorchaMasterySkillID)
	reminders.onEvent(initialQuantity)
	requests := []string{}
	stop := make(chan struct{})
	var stopOnce sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/api/skill_overlay/state", handleSkillOverlayState)
	mux.HandleFunc("/api/live_battles", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests = append(requests, r.URL.RawQuery)
		mu.Unlock()
		handleLiveBattles(w, r)
	})
	mux.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()
		for {
			var message string
			if websocket.Message.Receive(ws, &message) != nil {
				return
			}
		}
	}))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var data any = map[string]any{}
		switch r.URL.Path {
		case "/api/reminder_runtime/settings":
			mu.Lock()
			if r.Method == http.MethodPut {
				var settings nativeReminderSettings
				if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
					mu.Unlock()
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				reminders.settings = normalizeNativeReminderSettings(settings)
				reminders.dorcha.Below = reminders.dorcha.Observed && reminders.dorcha.Quantity < nativeDorchaThreshold(reminders.settings.SkillCooldowns.Rules[dorchaMasterySkillID].QuantityThreshold)
				reminders.lastSkillStateKey = ""
				reminders.publishNativeSkillState(time.Now())
			}
			data = reminders.settings
			mu.Unlock()
		case "/api/test/dorcha":
			quantity, err := strconv.ParseFloat(r.URL.Query().Get("quantity"), 64)
			if err != nil || quantity < 0 || quantity > 15 {
				http.Error(w, "invalid quantity", http.StatusBadRequest)
				return
			}
			mu.Lock()
			update := dorchaStat("player", true, quantity)
			update.At = 1700000076
			f.add(update)
			reminders.onEvent(update)
			reminders.publishNativeSkillState(time.Now())
			data = map[string]any{"quantity": reminders.dorcha.Quantity, "sounds": len(*sounds)}
			mu.Unlock()
		case "/api/status":
			data = map[string]any{"state": "capturing", "capturing": true, "gameRunning": true, "message": "场次切换验证"}
		case "/api/app_info":
			data = map[string]any{"name": "DilmeterCN", "version": "1.4.2-test"}
		case "/api/game_server":
			data = map[string]any{"selected": "irusha", "options": []any{map[string]any{"id": "irusha", "name": "CN服伊鲁夏"}}}
		case "/api/dps_recording":
			data = map[string]any{"enabled": true}
		case "/api/update":
			data = map[string]any{"available": false, "currentVersion": "1.4.2-test", "latestVersion": "1.4.2-test"}
		case "/api/battle_records":
			data = map[string]any{"records": []any{}, "total": 0, "hasMore": false}
		case "/api/test/metrics":
			mu.Lock()
			data = append([]string{}, requests...)
			mu.Unlock()
		case "/api/test/advance":
			mu.Lock()
			for i := 0; i < 2; i++ {
				f.add(liveTestDamage("new", 1700000075))
			}
			mu.Unlock()
		case "/api/test/stop":
			stopOnce.Do(func() { close(stop) })
		}
		_ = json.NewEncoder(w).Encode(data)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:8030")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	t.Log("Live-battle browser fixture ready at http://127.0.0.1:8030")
	<-stop
	server.Close()
}
