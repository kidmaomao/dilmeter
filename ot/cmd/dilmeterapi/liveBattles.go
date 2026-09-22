package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Keep an uninterrupted fight intact. Idle periods and connection changes
// start a new window; archived windows retain only metadata and disk offsets.
const liveBattleIdleSeconds = 90

type liveBattleTarget struct {
	closed        bool
	SessionKey    string  `json:"sessionKey"`
	EntityID      string  `json:"entityId"`
	Name          string  `json:"name"`
	RaceID        uint32  `json:"raceId"`
	StartedAt     int64   `json:"startedAt"`
	EndedAt       int64   `json:"endedAt"`
	TotalDamage   float64 `json:"totalDamage"`
	MaximumHealth float64 `json:"maximumHealth"`
}

type liveBattleAnchor struct {
	sequence uint64
	offset   int64
}
type liveBattleWindow struct {
	key                             string
	start, end, seedStart, seedSize int64
	startedAt, lastDamageAt         int64
	anchors                         []liveBattleAnchor
	eventCount                      int
	targets                         map[string]*liveBattleTarget
}

type liveBattleActorState struct {
	appear          *event.EventEntityAppear
	finish          *event.EventFinish
	body            *event.EventEntityUpdateBody
	stats           map[uint32]float64
	conditions      map[uint32]*event.EventCharacterConditionEnable
	refreshPrevious map[uint32]*event.EventCharacterConditionEnable
	items           map[uint32]*event.EventEntityEquipItem
}

type liveBattleIndex struct {
	mu                 sync.RWMutex
	path               string
	generation         string
	checkpoints        *os.File
	offset, seedOffset int64
	sequence           uint64
	nextWindow         uint64
	current            *liveBattleWindow
	windows            map[string]*liveBattleWindow
	actors             map[string]*liveBattleActorState
	local              *event.EventLocalEntity
	target             *event.EventCombatTarget
}

var currentLiveBattles atomic.Pointer[liveBattleIndex]

func newLiveBattleIndex(path string) (*liveBattleIndex, error) {
	checkpoints, err := os.OpenFile(path+".checkpoints", os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	return &liveBattleIndex{path: path, checkpoints: checkpoints, generation: strconv.FormatInt(time.Now().UnixNano(), 36),
		windows: map[string]*liveBattleWindow{}, actors: map[string]*liveBattleActorState{}}, nil
}

func (s *liveBattleIndex) actor(id string) *liveBattleActorState {
	a := s.actors[id]
	if a == nil {
		a = &liveBattleActorState{stats: map[uint32]float64{}, conditions: map[uint32]*event.EventCharacterConditionEnable{}, refreshPrevious: map[uint32]*event.EventCharacterConditionEnable{}, items: map[uint32]*event.EventEntityEquipItem{}}
		s.actors[id] = a
	}
	return a
}

func (s *liveBattleIndex) checkpoint(at int64) ([]byte, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	write := func(e event.IEvent) error { return enc.Encode(e) }
	if s.local != nil {
		local := *s.local
		local.Reset = false
		if err := write(&local); err != nil {
			return nil, err
		}
	}
	ids := make([]string, 0, len(s.actors))
	for id := range s.actors {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	// All identities precede state, including owners of pets and marionettes.
	for _, id := range ids {
		if a := s.actors[id]; a.appear != nil {
			if err := write(a.appear); err != nil {
				return nil, err
			}
		}
	}
	for _, id := range ids {
		a := s.actors[id]
		if a.body != nil {
			if err := write(a.body); err != nil {
				return nil, err
			}
		}
		stats := make([]event.EventStatUpdateEntry, 0, len(a.stats))
		for statID, value := range a.stats {
			stats = append(stats, event.EventStatUpdateEntry{StatId: statID, Value: value})
		}
		if len(stats) > 0 {
			if err := write(&event.EventStatUpdate{EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: at, Id: id}, Stats: stats}); err != nil {
				return nil, err
			}
		}
		for _, item := range a.items {
			if err := write(item); err != nil {
				return nil, err
			}
		}
		if a.finish != nil {
			if err := write(a.finish); err != nil {
				return nil, err
			}
		}
		for ccID, cc := range a.conditions {
			if cc.DisableAt > 0 && cc.DisableAt <= at {
				delete(a.conditions, ccID)
				delete(a.refreshPrevious, ccID)
				continue
			}
			// Preserve a pending refresh/remove guard at an immediate encounter
			// boundary, using ordinary events understood by older log readers.
			if previous := a.refreshPrevious[ccID]; previous != nil && at-cc.At <= 1 {
				if err := write(previous); err != nil {
					return nil, err
				}
			}
			if err := write(cc); err != nil {
				return nil, err
			}
		}
	}
	if s.target != nil {
		if err := write(s.target); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}

// append writes and indexes each event under one lock. Readers see a complete
// EOF and sequence boundary, never a half-written line or a speculative offset.
func (s *liveBattleIndex) append(file *os.File, e event.IEvent, raw []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	base, ok := e.(interface{ GetEventBase() *event.EventBase })
	if !ok {
		return fmt.Errorf("event has no base: %T", e)
	}
	b := base.GetEventBase()
	reset := false
	if local, ok := e.(*event.EventLocalEntity); ok {
		reset = local.Reset
	}
	rotate := s.current == nil || reset
	if s.current != nil && b.At-max(s.current.startedAt, s.current.lastDamageAt) > liveBattleIdleSeconds && !s.hasActiveBoss(s.current) {
		rotate = true
	}
	if damage, ok := e.(*event.EventDamage); ok && damage.Damage > 0 && s.current != nil && len(s.current.targets) > 0 {
		// Finishing one encounter and immediately engaging the next must not
		// require an artificial 90-second pause between them.
		active := false
		for _, target := range s.current.targets {
			if !target.closed && !battleRecordPCRace(target.RaceID) {
				active = true
				break
			}
		}
		// Finish/disappear can precede the final multihit or delayed damage.
		// Those packets still belong to the target already in this window.
		// Incoming damage to a player is not evidence of a new encounter either.
		target := s.actors[damage.TargetId]
		playerTarget := target != nil && target.appear != nil && battleRecordPCRace(target.appear.RaceId)
		if !active && s.current.targets[damage.TargetId] == nil && !playerTarget {
			rotate = true
		}
	}
	if rotate {
		seed, err := s.checkpoint(b.At)
		if err != nil {
			return err
		}
		if _, err = s.checkpoints.Write(seed); err != nil {
			return err
		}
		if s.current != nil {
			s.current.anchors = nil // only the live window supports incremental resume
			if len(s.current.targets) == 0 {
				delete(s.windows, s.current.key)
			}
		}
		s.nextWindow++
		w := &liveBattleWindow{key: s.generation + "-" + strconv.FormatUint(s.nextWindow, 10), start: s.offset, end: s.offset,
			seedStart: s.seedOffset, seedSize: int64(len(seed)), startedAt: b.At, targets: map[string]*liveBattleTarget{}}
		s.seedOffset += int64(len(seed))
		s.current = w
		s.windows[w.key] = w
	}
	w := s.current
	if b.Sequence > 0 && w.eventCount%256 == 0 {
		w.anchors = append(w.anchors, liveBattleAnchor{b.Sequence, s.offset})
	}
	n, err := file.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	s.offset += int64(n)
	w.end = s.offset
	w.eventCount++
	s.sequence = max(s.sequence, b.Sequence)
	if target := w.targets[b.Id]; target != nil {
		switch e.GetEventId() {
		case event.EventIdFinish, event.EventIdEntityDisappear:
			target.closed = true
		case event.EventIdEntityAppear:
			target.closed = false
		}
	}
	s.observe(e)
	if damage, ok := e.(*event.EventDamage); ok && damage.Damage > 0 {
		w.lastDamageAt = max(w.lastDamageAt, damage.At)
		t := w.targets[damage.TargetId]
		if t == nil {
			t = &liveBattleTarget{SessionKey: w.key, EntityID: damage.TargetId, StartedAt: damage.At}
			w.targets[damage.TargetId] = t
		}
		t.EndedAt = max(t.EndedAt, damage.At)
		t.TotalDamage += float64(damage.Damage)
		s.updateTarget(t)
	}
	// Late appearance/health packets fill in previously unknown targets.
	if t := w.targets[b.Id]; t != nil {
		s.updateTarget(t)
	}
	return nil
}

func (s *liveBattleIndex) updateTarget(t *liveBattleTarget) {
	if a := s.actors[t.EntityID]; a != nil {
		if a.appear != nil {
			t.Name, t.RaceID = a.appear.Name, a.appear.RaceId
		}
		t.MaximumHealth = max(t.MaximumHealth, a.stats[30])
	}
}

func (s *liveBattleIndex) hasActiveBoss(w *liveBattleWindow) bool {
	for id, target := range w.targets {
		if target.closed {
			continue
		}
		if target.MaximumHealth < battleRecordBossHealthFloor && target.TotalDamage < battleRecordBossHealthFloor {
			continue
		}
		if a := s.actors[id]; a != nil && a.finish == nil && (a.stats[28] > 0 || a.stats[28] == 0 && !hasLiveHealth(a.stats)) {
			return true
		}
	}
	return false
}

func hasLiveHealth(stats map[uint32]float64) bool { _, ok := stats[28]; return ok }

func (s *liveBattleIndex) observe(e event.IEvent) {
	switch v := e.(type) {
	case *event.EventLocalEntity:
		if v.Reset {
			clear(s.actors)
			s.target = nil
		}
		s.local = v
	case *event.EventEntityAppear:
		s.actor(v.Id).appear = v
		s.actor(v.Id).finish = nil
	case *event.EventFinish:
		s.actor(v.Id).finish = v
	case *event.EventEntityDisappear:
		if s.local == nil || v.Id != s.local.Id {
			delete(s.actors, v.Id)
		}
	case *event.EventEntityUpdateBody:
		s.actor(v.Id).body = v
	case *event.EventStatUpdate:
		a := s.actor(v.Id)
		for _, stat := range v.Stats {
			a.stats[stat.StatId] = stat.Value
		}
	case *event.EventCharacterConditionEnable:
		a := s.actor(v.Id)
		previous := a.conditions[v.CCId]
		if previous != nil && previous.At == v.At && previous.DisableAt == v.DisableAt && previous.DisableAtMs == v.DisableAtMs && previous.AttackerId == v.AttackerId && previous.Metadata == v.Metadata && previous.DurationMs == v.DurationMs {
			return
		}
		if previous != nil {
			a.refreshPrevious[v.CCId] = previous
		} else {
			delete(a.refreshPrevious, v.CCId)
		}
		a.conditions[v.CCId] = v
	case *event.EventCharacterConditionDisable:
		if a := s.actors[v.Id]; a != nil {
			active, guarded := a.conditions[v.CCId], a.refreshPrevious[v.CCId] != nil
			delete(a.refreshPrevious, v.CCId)
			if guarded && active != nil && v.At >= active.At && v.At-active.At <= 1 {
				return
			}
			delete(a.conditions, v.CCId)
		}
	case *event.EventEntityEquipItem:
		s.actor(v.Id).items[v.PocketType] = v
	case *event.EventEntityUnequipItem:
		if a := s.actors[v.Id]; a != nil {
			delete(a.items, v.PocketType)
		}
	case *event.EventCombatTarget:
		s.target = v
	}
}

func handleLiveBattles(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", 405)
		return
	}
	s := currentLiveBattles.Load()
	if s == nil {
		http.Error(w, "战斗记录尚未就绪", http.StatusServiceUnavailable)
		return
	}
	if key := r.URL.Query().Get("session"); key != "" {
		s.serveWindow(w, r, key)
		return
	}
	s.mu.RLock()
	items := []liveBattleTarget{}
	current := ""
	if s.current != nil {
		current = s.current.key
	}
	for _, window := range s.windows {
		for _, target := range window.targets {
			if !battleRecordPCRace(target.RaceID) {
				items = append(items, *target)
			}
		}
	}
	sequence := s.sequence
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].StartedAt != items[j].StartedAt {
			return items[i].StartedAt > items[j].StartedAt
		}
		return items[i].SessionKey+":"+items[i].EntityID > items[j].SessionKey+":"+items[j].EntityID
	})
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]any{"currentSessionKey": current, "sequence": sequence, "targets": items})
}

func (s *liveBattleIndex) serveWindow(w http.ResponseWriter, r *http.Request, key string) {
	// The socket and writer are independent consumers. Never replace data the
	// browser already applied with a snapshot whose writer is still behind it.
	minimum, _ := strconv.ParseUint(r.URL.Query().Get("minimumSequence"), 10, 64)
	if key == "latest" && minimum > 0 && strings.HasPrefix(r.URL.Query().Get("knownSession"), s.generation+"-") {
		deadline := time.NewTimer(2 * time.Second)
		tick := time.NewTicker(10 * time.Millisecond)
		defer deadline.Stop()
		defer tick.Stop()
		for {
			s.mu.RLock()
			ready := s.sequence >= minimum
			s.mu.RUnlock()
			if ready {
				break
			}
			select {
			case <-r.Context().Done():
				return
			case <-deadline.C:
				http.Error(w, "日志仍在同步，请稍后重试", http.StatusServiceUnavailable)
				return
			case <-tick.C:
			}
		}
	}
	after, err := strconv.ParseUint(r.URL.Query().Get("afterSequence"), 10, 64)
	if r.URL.Query().Has("afterSequence") && err != nil {
		http.Error(w, "invalid sequence", 400)
		return
	}
	s.mu.RLock()
	window := s.windows[key]
	if key == "latest" {
		window = s.current
	}
	if window == nil {
		s.mu.RUnlock()
		http.Error(w, "场次不存在", 404)
		return
	}
	start, end, seedStart, seedSize := window.start, window.end, window.seedStart, window.seedSize
	sequence, sessionKey := s.sequence, window.key
	delta := key == "latest" && r.URL.Query().Get("knownSession") == sessionKey && r.URL.Query().Has("afterSequence")
	if delta {
		seedSize = 0
		if after >= sequence {
			start = end
		} else {
			index := sort.Search(len(window.anchors), func(i int) bool { return window.anchors[i].sequence > after }) - 1
			if index >= 0 {
				start = window.anchors[index].offset
			}
		}
	}
	s.mu.RUnlock()
	file, err := os.Open(s.path)
	if err != nil {
		http.Error(w, "无法读取场次记录", 500)
		return
	}
	defer file.Close()
	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("X-Dilmeter-Session", sessionKey)
	w.Header().Set("X-Dilmeter-Sequence", strconv.FormatUint(sequence, 10))
	w.Header().Set("X-Dilmeter-Delta", strconv.FormatBool(delta))
	// Readers intentionally use captured section bounds, never the growing EOF.
	// For deltas the browser discards the <= afterSequence prefix (at most one
	// sparse block), preserving genuinely identical multihits by sequence.
	if seedSize > 0 {
		if _, err = io.Copy(w, io.NewSectionReader(s.checkpoints, seedStart, seedSize)); err != nil {
			return
		}
	}
	_, _ = io.Copy(w, io.NewSectionReader(file, start, end-start))
}
