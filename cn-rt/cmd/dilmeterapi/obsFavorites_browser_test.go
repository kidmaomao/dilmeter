package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"golang.org/x/net/websocket"
)

// Uses the production settings, persistence and favorite HTTP handlers. The only
// synthetic part is the game event source. No game/capture/native window starts.
func TestObsFavoriteBrowserHarness(t *testing.T) {
	readyFile := os.Getenv("DILMETER_OBS_FAVORITE_BROWSER_READY")
	if readyFile == "" {
		t.Skip("opt-in local browser fixture")
	}
	runtime, _ := healerFixture(t)
	dataDir := t.TempDir()
	runtime.healer = newHealerMonitor(filepath.Join(dataDir, "healer-monitor.json"))
	nativeReminderRuntimeHolder.Lock()
	previous := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	defer func() {
		nativeReminderRuntimeHolder.Lock()
		nativeReminderRuntimeHolder.runtime = previous
		nativeReminderRuntimeHolder.Unlock()
	}()
	var mu sync.Mutex
	scope := "198.51.100.1"
	reliable := true
	legacy := false
	sequence := uint64(0)
	saves := 0
	players := map[string]string{"1": "本人", "2": "常用队友", "3": "附近路人"}
	sockets := map[*websocket.Conn]bool{}
	identity := func(reset bool) *event.EventLocalEntity {
		return &event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "1", At: time.Now().Unix()}, Reliable: reliable, Reset: reset, ServerScope: scope}
	}
	initial := func() []event.IEvent {
		batch := []event.IEvent{identity(false)}
		for id, name := range players {
			batch = append(batch, liveTestAppear(id, name, 9001, time.Now().Unix()))
		}
		return batch
	}
	for _, e := range initial() {
		runtime.onEvent(e)
	}
	runtime.healer.evaluate(runtime, time.Now(), true)
	emit := func(e event.IEvent) {
		sequence++
		e.(interface{ GetEventBase() *event.EventBase }).GetEventBase().Sequence = sequence
		runtime.onEvent(e)
		runtime.healer.lastTickMs = 0
		runtime.healer.evaluate(runtime, time.Now(), true)
		for ws := range sockets {
			if websocket.JSON.Send(ws, []event.IEvent{e}) != nil {
				ws.Close()
				delete(sockets, ws)
			}
		}
	}
	stop := make(chan struct{})
	var stopOnce sync.Once
	mux := http.NewServeMux()
	mux.HandleFunc("/api/app_info", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"name": "DilmeterCN test harness", "version": "1.6.1-obs.4"})
	})
	mux.HandleFunc("/api/healer_monitor", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if r.Method == http.MethodPut {
			saves++
		}
		handleHealerMonitor(w, r)
	})
	mux.HandleFunc("/api/obs_favorites", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		if legacy {
			http.NotFound(w, r)
			return
		}
		handleObsFavorites(w, r)
	})
	mux.Handle("/ws", websocket.Handler(func(ws *websocket.Conn) {
		mu.Lock()
		sockets[ws] = true
		_ = websocket.JSON.Send(ws, initial())
		mu.Unlock()
		defer func() { mu.Lock(); delete(sockets, ws); mu.Unlock(); ws.Close() }()
		for {
			var value string
			if websocket.Message.Receive(ws, &value) != nil {
				return
			}
		}
	}))
	mux.HandleFunc("/test/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", 405)
			return
		}
		mu.Lock()
		defer mu.Unlock()
		q := r.URL.Query()
		switch r.URL.Path {
		case "/test/cast":
			id := q.Get("id")
			skill, _ := strconv.Atoi(q.Get("skill"))
			if skill == 0 {
				skill = 59047
			}
			emit(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: id, At: time.Now().Unix()}, AtMs: time.Now().UnixMilli(), SkillId: uint16(skill), CombatActionId: uint32(sequence + 1), SourceId: id, IsLocal: id == "1", IsFallback: q.Get("fallback") == "1"})
		case "/test/identity":
			if q.Has("scope") {
				scope = q.Get("scope")
			}
			reliable = q.Get("reliable") != "0"
			reset := q.Get("reset") == "1"
			if reset {
				players = map[string]string{"1": "本人"}
			}
			emit(identity(reset))
		case "/test/appear":
			id, name := q.Get("id"), q.Get("name")
			players[id] = name
			emit(liveTestAppear(id, name, 9001, time.Now().Unix()))
		case "/test/disappear":
			id := q.Get("id")
			delete(players, id)
			emit(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: id, At: time.Now().Unix()}})
		case "/test/disconnect":
			for ws := range sockets {
				ws.Close()
			}
		case "/test/restart":
			for ws := range sockets {
				ws.Close()
			}
			runtime.healer = newHealerMonitor(filepath.Join(dataDir, "healer-monitor.json"))
			for _, e := range initial() {
				runtime.onEvent(e)
			}
			runtime.healer.evaluate(runtime, time.Now(), true)
		case "/test/legacy":
			legacy = q.Get("on") == "1"
		case "/test/stop":
			stopOnce.Do(func() { close(stop) })
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"savedRequests": saves, "dataDirectory": dataDir})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<!doctype html><html><body><div id="app"></div><script type="module" src="http://127.0.0.1:18031/tests/obs-favorites-entry.js"></script></body></html>`)
	})
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: mux}
	go server.Serve(listener)
	defer server.Close()
	if err := os.WriteFile(readyFile, []byte("http://"+listener.Addr().String()), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stop:
	case <-time.After(150 * time.Second):
		t.Fatal("browser fixture timeout")
	}
	mu.Lock()
	for ws := range sockets {
		ws.Close()
	}
	mu.Unlock()
}
