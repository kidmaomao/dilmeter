package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
)

// nativeReminderDragSnapshot is polled by the settings page. A sequence is
// published on every mouse move so X/Y inputs follow the native window live.
type nativeReminderDragSnapshot struct {
	Sequence int64  `json:"sequence"`
	Kind     string `json:"kind"`
	ID       string `json:"id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Dragging bool   `json:"dragging"`
	Locked   bool   `json:"locked"`
	pending  bool
}

type nativeReminderPositionUpdate struct {
	Kind string
	ID   string
	X    int
	Y    int
}

var nativeReminderDragState = struct {
	sync.RWMutex
	value nativeReminderDragSnapshot
}{}
var nativeReminderDragSequence atomic.Int64

func publishNativeReminderDrag(kind, id string, x, y int, dragging, pending bool) {
	nativeReminderDragState.Lock()
	nativeReminderDragState.value = nativeReminderDragSnapshot{
		Sequence: nativeReminderDragSequence.Add(1), Kind: kind, ID: id,
		X: x, Y: y, Dragging: dragging, Locked: nativeReminderOverlaysAreLocked(), pending: pending,
	}
	nativeReminderDragState.Unlock()
}

func currentNativeReminderDrag() nativeReminderDragSnapshot {
	nativeReminderDragState.RLock()
	value := nativeReminderDragState.value
	nativeReminderDragState.RUnlock()
	value.Locked = nativeReminderOverlaysAreLocked()
	return value
}

func markNativeReminderDragCommitted(kind, id string) {
	nativeReminderDragState.Lock()
	if nativeReminderDragState.value.Kind == kind && nativeReminderDragState.value.ID == id {
		nativeReminderDragState.value.pending = false
	}
	nativeReminderDragState.Unlock()
}

func applyNativeReminderDragOverride(message *nativeSkillOverlayMessage) {
	if message == nil {
		return
	}
	snapshot := currentNativeReminderDrag()
	if !snapshot.Dragging && !snapshot.pending {
		return
	}
	switch snapshot.Kind {
	case "skill":
		id, err := strconv.ParseUint(snapshot.ID, 10, 16)
		if err != nil {
			return
		}
		for index := range message.Items {
			if message.Items[index].SkillID == uint16(id) {
				message.Items[index].X, message.Items[index].Y = snapshot.X, snapshot.Y
			}
		}
	case "aim":
		if message.AimReminder != nil {
			message.AimReminder.X, message.AimReminder.Y = snapshot.X, snapshot.Y
		}
	case "mechanic":
		for index := range message.Mechanics {
			if message.Mechanics[index].Key == snapshot.ID {
				message.Mechanics[index].X, message.Mechanics[index].Y = snapshot.X, snapshot.Y
			}
		}
	case "target-health":
		if message.TargetHealth != nil {
			message.TargetHealth.X, message.TargetHealth.Y = snapshot.X, snapshot.Y
		}
	case "effect":
		for index := range message.EffectTimers {
			if message.EffectTimers[index].Key == snapshot.ID {
				message.EffectTimers[index].X, message.EffectTimers[index].Y = snapshot.X, snapshot.Y
			}
		}
	case "stack":
		id, err := strconv.ParseUint(snapshot.ID, 10, 32)
		if err != nil {
			return
		}
		for index := range message.StackAlerts {
			if message.StackAlerts[index].CCID == uint32(id) {
				message.StackAlerts[index].X, message.StackAlerts[index].Y = snapshot.X, snapshot.Y
			}
		}
	}
}

func handleNativeReminderDragState(w http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(currentNativeReminderDrag())
}

func applyNativeReminderPosition(settings *nativeReminderSettings, update nativeReminderPositionUpdate) bool {
	if settings == nil {
		return false
	}
	x := max(-32000, min(32000, update.X))
	y := max(-32000, min(32000, update.Y))
	switch update.Kind {
	case "skill":
		id, err := strconv.ParseUint(update.ID, 10, 16)
		if err != nil {
			return false
		}
		rule, ok := settings.SkillCooldowns.Rules[uint16(id)]
		if !ok {
			return false
		}
		rule.X, rule.Y = x, y
		settings.SkillCooldowns.Rules[uint16(id)] = rule
	case "aim":
		settings.SkillCooldowns.AimReminder.X, settings.SkillCooldowns.AimReminder.Y = x, y
	case "mechanic":
		rule, ok := settings.BossMechanics.Rules[update.ID]
		if !ok {
			return false
		}
		rule.X, rule.Y = x, y
		settings.BossMechanics.Rules[update.ID] = rule
	case "target-health":
		settings.BossMechanics.MielShardHealthBarX, settings.BossMechanics.MielShardHealthBarY = x, y
	case "effect":
		rule, ok := settings.EffectTimers.Rules[update.ID]
		if !ok {
			return false
		}
		rule.X, rule.Y = x, y
		settings.EffectTimers.Rules[update.ID] = rule
	case "stack":
		id, err := strconv.ParseUint(update.ID, 10, 32)
		if err != nil {
			return false
		}
		rule, ok := settings.Buff.Rules[uint32(id)]
		if !ok {
			return false
		}
		rule.StackX, rule.StackY = x, y
		settings.Buff.Rules[uint32(id)] = rule
	default:
		return false
	}
	return true
}

func queueNativeReminderPositionUpdate(update nativeReminderPositionUpdate) {
	nativeReminderRuntimeHolder.RLock()
	runtime := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.RUnlock()
	if runtime == nil {
		return
	}
	select {
	case runtime.positionCh <- update:
	case <-runtime.ctx.Done():
	}
}
