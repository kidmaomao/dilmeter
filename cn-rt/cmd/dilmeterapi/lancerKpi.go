package main

import (
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

// Called while the publisher is locked, in packet order. A nil snapshot means
// the caster was not observable; absence on a known caster means inactive.
func (t *eventPublisher) captureLanceMount(action *event.EventSkillAction) {
	if action.LanceMounted != nil || action.IsFallback || action.MechanicSignal != "" ||
		(action.SourceId != "" && action.SourceId != action.Id) {
		return
	}
	if action.SkillId != 20017 && action.SkillId != 59105 && action.SkillId != 59106 {
		return
	}
	id, err := strconv.ParseUint(action.Id, 10, 64)
	if err != nil {
		return
	}
	actor := t.entityCache[id]
	if actor == nil || actor.EntityInfo == nil || !actor.IsUser() || actor.disappearAt != 0 {
		return
	}
	actor.Lock()
	defer actor.Unlock()
	at := action.AtMs
	if at == 0 {
		at = action.At * 1000
	}
	active := false
	if condition := actor.characterConditionMap[975]; condition != nil {
		expires := condition.DisableAtMs
		if expires == 0 {
			expires = condition.DisableAt * 1000
		}
		active = expires <= 0 || expires > at
	}
	action.LanceMounted = &active
}
