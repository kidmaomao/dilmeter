package main

import (
	"bufio"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

type liveBattleFixture struct {
	index    *liveBattleIndex
	file     *os.File
	sequence uint64
	t        *testing.T
}

func newLiveBattleFixture(t *testing.T) *liveBattleFixture {
	t.Helper()
	path := filepath.Join(t.TempDir(), "packet_log_2026-09-12_12-00-00.ndjson")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	index, err := newLiveBattleIndex(path)
	if err != nil {
		t.Fatal(err)
	}
	f := &liveBattleFixture{index: index, file: file, t: t}
	t.Cleanup(func() { file.Close(); index.checkpoints.Close() })
	return f
}

func (f *liveBattleFixture) add(e event.IEvent) {
	f.t.Helper()
	f.sequence++
	e.(interface{ GetEventBase() *event.EventBase }).GetEventBase().Sequence = f.sequence
	raw, err := json.Marshal(e)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.index.append(f.file, e, append(raw, '\n')); err != nil {
		f.t.Fatal(err)
	}
}

func liveTestAppear(id, name string, race uint32, at int64) *event.EventEntityAppear {
	return &event.EventEntityAppear{EventBase: event.EventBase{EventId: 1, Id: id, At: at}, RaceId: race, Name: name, Height: 1, Weight: 1, Upper: 1, Lower: 1}
}
func liveTestDamage(target string, at int64) *event.EventDamage {
	return &event.EventDamage{EventBase: event.EventBase{EventId: 3, Id: "player", At: at}, TargetId: target, SkillId: 35024, Damage: 100}
}
func liveTestHP(id string, at int64) *event.EventStatUpdate {
	return &event.EventStatUpdate{EventBase: event.EventBase{EventId: 17, Id: id, At: at}, Stats: []event.EventStatUpdateEntry{{StatId: 30, Value: 200_000_000}, {StatId: 28, Value: 200_000_000}}}
}
func (f *liveBattleFixture) read(query string) (*httptest.ResponseRecorder, []map[string]any) {
	f.t.Helper()
	r := httptest.NewRequest("GET", "/api/live_battles?"+query, nil)
	w := httptest.NewRecorder()
	f.index.serveWindow(w, r, r.URL.Query().Get("session"))
	rows := []map[string]any{}
	if w.Code == 200 {
		scanner := bufio.NewScanner(strings.NewReader(w.Body.String()))
		for scanner.Scan() {
			var row map[string]any
			if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
				f.t.Fatal(err)
			}
			rows = append(rows, row)
		}
		if err := scanner.Err(); err != nil {
			f.t.Fatal(err)
		}
	}
	return w, rows
}

func TestLiveBattleWindowsReadOnlySelectedDetails(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, At: 100, Id: "player"}, Reliable: true})
	f.add(liveTestAppear("player", "测试玩家", 10001, 100))
	f.add(liveTestAppear("old", "旧首领", 7603, 100))
	f.add(liveTestHP("old", 100))
	f.add(&event.EventCharacterConditionEnable{EventBase: event.EventBase{EventId: 4, Id: "player", At: 100}, CCId: 123, DisableAt: 10000})
	for i := 0; i < 5000; i++ {
		f.add(liveTestDamage("old", 101))
	}
	oldKey := f.index.current.key
	f.add(&event.EventFinish{EventBase: event.EventBase{EventId: 6, Id: "old", At: 102}, AttackerId: "player"})
	f.add(liveTestAppear("new", "新首领", 7615, 103))
	f.add(liveTestHP("new", 103))
	f.add(liveTestDamage("new", 104))
	newKey := f.index.current.key
	if oldKey == newKey {
		t.Fatal("consecutive completed encounters were not separated")
	}
	response, rows := f.read("session=latest")
	if response.Code != 200 || len(rows) > 30 {
		t.Fatalf("latest window loaded the old session: %d rows, HTTP %d", len(rows), response.Code)
	}
	hits, hasPlayer, hasBuff := 0, false, false
	for _, row := range rows {
		if row["EventId"] == float64(3) {
			hits++
			if row["TargetId"] != "new" {
				t.Fatal("old damage leaked into live window")
			}
		}
		if row["Name"] == "测试玩家" {
			hasPlayer = true
		}
		if row["CCId"] == float64(123) {
			hasBuff = true
		}
	}
	if hits != 1 || !hasPlayer || !hasBuff {
		t.Fatalf("recent checkpoint lost required context: hits %d, player %t, buff %t", hits, hasPlayer, hasBuff)
	}
	_, oldRows := f.read("session=" + oldKey)
	oldHits := 0
	for _, row := range oldRows {
		if row["EventId"] != float64(3) {
			continue
		}
		oldHits++
		if row["TargetId"] != "old" {
			t.Fatal("new fight leaked into archive")
		}
	}
	if oldHits != 5000 {
		t.Fatalf("archive lost identical multihits: %d", oldHits)
	}
	if len(f.index.windows[oldKey].anchors) != 0 {
		t.Fatal("old seek accelerators retained in memory")
	}
	if len(f.index.windows[oldKey].targets) != 1 {
		t.Fatal("old target metadata lost")
	}
}

func TestLiveBattleLongMechanicAndUnknownTargets(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("boss", "首领", 7603, 100))
	f.add(liveTestHP("boss", 100))
	f.add(liveTestDamage("boss", 101))
	key := f.index.current.key
	f.add(&event.EventSkillAction{EventBase: event.EventBase{EventId: 10, Id: "boss", At: 800}, SkillId: 100})
	f.add(liveTestDamage("boss", 801))
	if f.index.current.key != key {
		t.Fatal("a long living-boss mechanic split an ongoing fight")
	}
	f.add(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "boss", At: 802}})
	f.add(liveTestDamage("unknown", 803))
	unknownKey := f.index.current.key
	for i := 0; i < 5; i++ {
		f.add(liveTestDamage("unknown", 804))
	}
	if unknownKey == key || f.index.current.key != unknownKey {
		t.Fatal("unknown target was mistaken for a completed fight")
	}
	f.add(&event.EventLocalEntity{EventBase: event.EventBase{EventId: 11, Id: "player", At: 805}, Reset: true})
	if f.index.current.key == unknownKey {
		t.Fatal("connection reset did not start a fresh window")
	}
}

func TestLiveBattleIncrementalResumeHasSequenceBoundary(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("boss", "首领", 7603, 100))
	for i := 0; i < 600; i++ {
		f.add(liveTestDamage("boss", 101))
	}
	key, before := f.index.current.key, f.sequence
	f.add(liveTestDamage("boss", 101))
	f.add(liveTestDamage("boss", 101))
	response, rows := f.read("session=latest&knownSession=" + key + "&afterSequence=" + strconv.FormatUint(before, 10))
	if response.Header().Get("X-Dilmeter-Delta") != "true" || len(rows) > 258 {
		t.Fatalf("incremental read was not bounded: %d rows", len(rows))
	}
	newHits := 0
	for _, row := range rows {
		if row["Sequence"].(float64) > float64(before) {
			newHits++
		}
	}
	if newHits != 2 {
		t.Fatalf("same-time same-damage hits not preserved: %d", newHits)
	}
	_, empty := f.read("session=latest&knownSession=" + key + "&afterSequence=" + strconv.FormatUint(f.sequence+1, 10))
	if len(empty) != 0 {
		t.Fatal("writer lag should not replay already applied records")
	}
	f.add(&event.EventEntityDisappear{EventBase: event.EventBase{EventId: 2, Id: "boss", At: 102}})
	f.add(liveTestDamage("second", 103))
	response, _ = f.read("session=latest&knownSession=" + key + "&afterSequence=" + strconv.FormatUint(before, 10))
	if response.Header().Get("X-Dilmeter-Delta") != "false" {
		t.Fatal("new session attempted to append to old statistics")
	}
	for _, query := range []string{"session=latest&afterSequence=-1", "session=latest&afterSequence=bad"} {
		response, _ = f.read(query)
		if response.Code != 400 {
			t.Fatalf("invalid cursor accepted: %s", query)
		}
	}
}

func TestLiveBattleCatalogDoesNotReadLogs(t *testing.T) {
	f := newLiveBattleFixture(t)
	f.add(liveTestAppear("boss", "首领", 7603, 100))
	f.add(liveTestDamage("boss", 101))
	previous := currentLiveBattles.Swap(f.index)
	t.Cleanup(func() { currentLiveBattles.Store(previous) })
	f.file.Close()
	if err := os.Rename(f.index.path, f.index.path+".hidden"); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	handleLiveBattles(w, httptest.NewRequest("GET", "/api/live_battles", nil))
	if w.Code != 200 || !strings.Contains(w.Body.String(), "首领") {
		t.Fatalf("catalog requires damage log reads: %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	handleLiveBattles(w, httptest.NewRequest("POST", "/api/live_battles", nil))
	if w.Code != 405 {
		t.Fatal("unexpected method accepted")
	}
}
