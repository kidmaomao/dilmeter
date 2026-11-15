package main

import (
	"slices"
	"strconv"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

type healerArcanaPreset struct {
	name   string
	vivace bool
	skill  healerSkillRule
}

// The ranges match the combat report's Arcana detection (59000–59199).
var healerArcanaPresets = [...]healerArcanaPreset{
	{name: "圣光颂唱者", skill: healerSkillRule{SkillID: 59005, Name: "崩坏波动", CooldownSeconds: 180}},
	{name: "元素骑士"},
	{name: "黑魔导士", vivace: true, skill: healerSkillRule{SkillID: 59047, Name: "烬火引燃", CooldownSeconds: 36}},
	{name: "流星射手", skill: healerSkillRule{SkillID: 59064, Name: "元素连击", CooldownSeconds: 36}},
	{name: "圣盾骑士"},
	{name: "爆裂骑士枪", skill: healerSkillRule{SkillID: 59106, Name: "湮灭", CooldownSeconds: 30}},
	{name: "枪炮师", skill: healerSkillRule{SkillID: 59123, Name: "致命狙击", CooldownSeconds: 25}},
	{name: "禁术炼金术师", vivace: true, skill: healerSkillRule{SkillID: 59143, Name: "召唤梦魇", CooldownSeconds: 60}},
	{name: "旋律操纵师", skill: healerSkillRule{SkillID: 59163, Name: "终幕绝响", CooldownSeconds: 45}},
	{name: "狂怒斗士", skill: healerSkillRule{SkillID: 59187, Name: "烈火拳三击", CooldownSeconds: 30}},
}

const healerArcanaMask = (1 << len(healerArcanaPresets)) - 1

type healerArcanaObservation struct {
	name string
	job  int
	atMs int64
}

func healerArcanaForSkill(skill uint16) int {
	if skill < 59000 || int(skill) >= 59000+20*len(healerArcanaPresets) {
		return 0
	}
	return int(skill-59000)/20 + 1
}

// An Arcana action identifies its caster. Conditions on the recipient and
// skills produced by a summon don't identify the recipient or owner's Arcana.
func (h *healerMonitor) observeArcana(current event.IEvent, runtime *nativeReminderRuntime) {
	var actor string
	var skill uint16
	var atMs int64
	switch v := current.(type) {
	case *event.EventSkillAction:
		actor, skill, atMs = v.SourceId, v.SkillId, nativeEventAtMs(v.At, v.AtMs)
		if actor == "" {
			actor = v.Id
		}
	case *event.EventDamage:
		if !v.IsDelayed {
			actor, skill, atMs = v.Id, v.SkillId, nativeEventAtMs(v.At, v.AtMs)
		}
	case *event.EventSkillState:
		if v.Active && (v.Scope == "burst-release" || v.Scope == "obs-execute" || v.Scope == "obs-public-release") {
			actor, skill, atMs = v.Id, v.SkillId, nativeEventAtMs(v.At, v.AtMs)
		}
	case *event.EventCharacterConditionEnable:
		if !v.Snapshot && v.CCId == 803 {
			actor, skill, atMs = v.AttackerId, 59005, nativeEventAtMs(v.At, v.AtMs)
			if _, source, valid := parseConditionSkillMetadata(v.Metadata); valid && source != 0 {
				actor = strconv.FormatUint(source, 10)
			}
		}
	}
	job := healerArcanaForSkill(skill)
	if job == 0 || actor == runtime.localID {
		return
	}
	entity, observed := runtime.entities[actor], h.observed[actor]
	if entity == nil || !entity.Known || entity.Name == "" || entity.OwnerID != "" || !battleRecordPCRace(entity.RaceID) ||
		observed == nil || !observed.visible {
		return
	}
	if h.arcana == nil {
		h.arcana = make(map[string]healerArcanaObservation)
	}
	if previous := h.arcana[actor]; previous.name != entity.Name || atMs >= previous.atMs {
		h.arcana[actor] = healerArcanaObservation{name: entity.Name, job: job, atMs: atMs}
	}
}

func (h *healerMonitor) memberArcana(id, name string) int {
	observed := h.arcana[id]
	if observed.name == name {
		return observed.job
	}
	return 0
}

// Apply each class's defaults once, preserving existing rules and later edits,
// including deliberate removal of a suggested rule.
func (h *healerMonitor) withArcanaPreset(member healerMemberSelection) healerMemberSelection {
	job := h.memberArcana(member.ID, member.Name)
	if job == 0 || member.Self {
		return member
	}
	flag := uint16(1 << (job - 1))
	if member.AppliedArcana&flag != 0 {
		return member
	}
	member.AppliedArcana |= flag
	preset := healerArcanaPresets[job-1]
	if member.BuffSettings == nil || member.SkillSettings == nil {
		member = normalizeHealerMember(member, h.settings, 0)
	}

	music := healerMusicRule(h.settings.OvertureRule, 680, "战争序曲", h.settings)
	if preset.vivace {
		music = healerMusicRule(h.settings.VivaceRule, 192, "活跃进行曲", h.settings)
	}
	buffs := *member.BuffSettings
	if len(buffs.Rules) < 16 && !slices.ContainsFunc(buffs.Rules, func(rule healerBuffRule) bool { return rule.CCID == music.CCID }) {
		buffs.Rules = append(slices.Clone(buffs.Rules), music)
		buffs.Enabled = true
		member.BuffSettings = &buffs
	}
	skills := *member.SkillSettings
	if preset.skill.SkillID != 0 && len(skills.Rules) < 16 && !slices.ContainsFunc(skills.Rules, func(rule healerSkillRule) bool { return rule.SkillID == preset.skill.SkillID }) {
		rule := preset.skill
		rule.FromArcana = true
		rule.Sound = healerSound{Kind: "skill-ready"}
		skills.Rules = append(slices.Clone(skills.Rules), rule)
		skills.Enabled = true
		member.SkillSettings = &skills
	}
	return member
}
