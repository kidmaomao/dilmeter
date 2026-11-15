package main

import (
	"fmt"
	"sort"
	"strings"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

func (s healerSettings) automaticTemplate() *healerMemberTemplate {
	for i := range s.Templates {
		if s.Templates[i].ID == s.AutoAddTemplateID {
			return &s.Templates[i]
		}
	}
	return nil
}

func (h *healerMonitor) partyMember(id string) event.PartyMember {
	for _, member := range h.party.Members {
		if member.ID == id {
			return member
		}
	}
	return event.PartyMember{}
}

func (h *healerMonitor) automaticMemberName(id string, runtime *nativeReminderRuntime) string {
	if !h.settings.AutoAddInDungeon || id == runtime.localID || len(h.settings.Members) >= 32 {
		return ""
	}
	var name string
	if len(h.party.Members) > 0 {
		name = h.partyMember(id).Name
	} else if h.inDungeon && h.regionID != 0 {
		entity, observed := runtime.entities[id], h.observed[id]
		if entity != nil && entity.Known && entity.OwnerID == "" && battleRecordPCRace(entity.RaceID) &&
			observed != nil && observed.visible && observed.regionID == h.regionID {
			name = entity.Name
		}
	}
	if name == "" {
		return ""
	}
	for _, member := range h.settings.Members {
		if member.ID == id || !member.Self && member.Name == name {
			return ""
		}
	}
	return name
}

type healerInstanceMember struct {
	name     string
	override *healerMemberSelection
}

func (h *healerMonitor) resetDungeonMembers() {
	clear(h.instanceMembers)
	h.dungeonSession++
}

func (h *healerMonitor) automaticMember(id, name string, template *healerMemberTemplate) healerMemberSelection {
	if saved := h.instanceMembers[id]; saved.name == name && saved.override != nil {
		return h.withArcanaPreset(*saved.override)
	}
	member := healerMemberSelection{
		automatic: true, ID: id, Name: name, Included: true,
		Health: template.Health, HealthSettings: template.HealthSettings,
		BuffSettings: template.BuffSettings, SkillSettings: template.SkillSettings,
	}
	return h.withArcanaPreset(member)
}

func (h *healerMonitor) memberSkillSettings(id string, runtime *nativeReminderRuntime) *healerMemberSkillSettings {
	for _, member := range h.settings.Members {
		if member.ID == id {
			return h.withArcanaPreset(member).SkillSettings
		}
	}
	if name := h.automaticMemberName(id, runtime); name != "" {
		if template := h.settings.automaticTemplate(); template != nil {
			return h.automaticMember(id, name, template).SkillSettings
		}
	}
	return nil
}

// Caller holds h.mu. Keep the instance-only selections separate from the
// saved configuration so disconnects, map changes and template edits take effect.
func (h *healerMonitor) selectedMembers(runtime *nativeReminderRuntime, includeAutomatic bool) []healerMemberSelection {
	h.resolveMembers(runtime)
	members := h.settings.Members
	template := h.settings.automaticTemplate()
	if !includeAutomatic || !h.settings.AutoAddInDungeon || template == nil || len(members) >= 32 {
		return members
	}
	var automatic []healerMemberSelection
	add := func(id string) {
		if name := h.automaticMemberName(id, runtime); name != "" {
			automatic = append(automatic, h.automaticMember(id, name, template))
		}
	}
	if len(h.party.Members) > 0 {
		for _, member := range h.party.Members {
			add(member.ID)
		}
	} else if h.inDungeon {
		for id := range runtime.entities {
			add(id)
		}
		sort.Slice(automatic, func(i, j int) bool {
			if automatic[i].Name == automatic[j].Name {
				return automatic[i].ID < automatic[j].ID
			}
			return automatic[i].Name < automatic[j].Name
		})
	}
	return append(append([]healerMemberSelection(nil), members...), automatic[:min(len(automatic), 32-len(members))]...)
}

func healerMemberKey(member healerMemberSelection) string {
	if member.Self {
		return "self"
	}
	if member.automatic {
		return "instance:" + member.ID
	}
	if member.Name != "" {
		return "player:" + member.Name
	}
	return "entity:" + member.ID
}

func normalizeHealerMember(member healerMemberSelection, settings healerSettings, index int) healerMemberSelection {
	member.Name = strings.TrimSpace(member.Name)
	if member.Self {
		member.Name = "自身角色"
		member.Favorite = true
		member.Included = true
		member.ManaSettings = normalizeHealerMana(member.ManaSettings, index)
	} else {
		member.Mana, member.ManaSettings = false, nil
	}
	member.Name = string([]rune(member.Name)[:min(len([]rune(member.Name)), 100)])
	member.Key = healerMemberKey(member)
	member.AppliedArcana &= healerArcanaMask
	if member.HealthSettings == nil {
		member.HealthSettings = &healerHealthSettings{Threshold: settings.LowHealthPercent, SoundEnabled: settings.SoundEnabled, Sound: settings.HealthSound,
			Overlay: healerPosition{Enabled: settings.Text.Enabled, X: settings.Text.X, Y: settings.Text.Y + index*100}}
	}
	health := *member.HealthSettings
	health.Threshold = clampNativeReminderInt(health.Threshold, 5, 95, 40)
	health.RepeatCount = clampNativeReminderInt(health.RepeatCount, 1, 10, 1)
	health.RepeatIntervalSeconds = clampNativeReminderInt(health.RepeatIntervalSeconds, 2, 300, 5)
	health.Sound = normalizeHealerSound(health.Sound, "healer-health")
	health.Overlay = normalizeHealerPosition(health.Overlay)
	member.HealthSettings = &health
	if member.BuffSettings == nil {
		rules := []healerBuffRule{}
		if member.Overture {
			rules = append(rules, healerMusicRule(settings.OvertureRule, 680, "战争序曲", settings))
		}
		if member.Vivace {
			rules = append(rules, healerMusicRule(settings.VivaceRule, 192, "活跃进行曲", settings))
		}
		for _, rule := range settings.Buffs {
			for _, id := range member.Buffs {
				if id == rule.CCID {
					rules = append(rules, rule)
					break
				}
			}
		}
		member.BuffSettings = &healerMemberBuffSettings{Enabled: len(rules) > 0, SoundEnabled: settings.SoundEnabled, Rules: rules,
			Overlay: healerPosition{Enabled: settings.Text.Enabled, X: 40, Y: 160 + index*72}}
	}
	buffs := *member.BuffSettings
	buffs.Overlay = normalizeHealerPosition(buffs.Overlay)
	buffs.Rules = []healerBuffRule{}
	seen := map[uint32]bool{}
	for _, rule := range member.BuffSettings.Rules {
		if seen[rule.CCID] {
			continue
		}
		seen[rule.CCID] = true
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			rule.Name = fmt.Sprintf("Buff %d", rule.CCID)
		}
		rule.Name = string([]rune(rule.Name)[:min(len([]rune(rule.Name)), 48)])
		fallback := settings.BuffSound
		if rule.CCID == 680 || rule.CCID == 192 {
			fallback = settings.MusicSound
		}
		buffs.Rules = append(buffs.Rules, normalizeHealerBuffRule(rule, fallback))
		if len(buffs.Rules) == 16 {
			break
		}
	}
	member.BuffSettings = &buffs
	member.SkillSettings = normalizeHealerSkills(member.SkillSettings, index)
	member.SkillSettings.Overlay.X, member.SkillSettings.Overlay.Y = buffs.Overlay.X, buffs.Overlay.Y
	return member
}

func normalizeHealerTemplates(templates []healerMemberTemplate, settings healerSettings) []healerMemberTemplate {
	result := make([]healerMemberTemplate, 0, min(len(templates), 32))
	seen := map[string]bool{}
	for index, template := range templates {
		template.ID = strings.TrimSpace(template.ID)
		if template.ID == "" {
			template.ID = fmt.Sprintf("template-%d", index+1)
		}
		if len(template.ID) > 128 || seen[template.ID] {
			continue
		}
		template.Name = strings.TrimSpace(template.Name)
		if template.Name == "" {
			template.Name = fmt.Sprintf("队友%d", index+1)
		}
		template.Name = string([]rune(template.Name)[:min(len([]rune(template.Name)), 48)])
		member := normalizeHealerMember(healerMemberSelection{Name: template.Name, Health: template.Health, HealthSettings: template.HealthSettings, BuffSettings: template.BuffSettings, SkillSettings: template.SkillSettings}, settings, index)
		template.HealthSettings, template.BuffSettings, template.SkillSettings = member.HealthSettings, member.BuffSettings, member.SkillSettings
		seen[template.ID] = true
		result = append(result, template)
		if len(result) == 32 {
			break
		}
	}
	return result
}

func normalizeHealerPosition(position healerPosition) healerPosition {
	position.X = clampNativeReminderInt(position.X, -32000, 32000, 40)
	position.Y = clampNativeReminderInt(position.Y, -32000, 32000, 160)
	return position
}

// Runtime IDs are deliberately omitted on disk. A newly observed, unique
// character name is required before a favorite can produce any alert.
func healerFavoriteMembers(members []healerMemberSelection) []healerMemberSelection {
	result := []healerMemberSelection{}
	for _, member := range members {
		if member.Self {
			member.Name = "自身角色"
			member.Key = "self"
			member.Favorite = true
			member.Included = true
		}
		if !member.Favorite || member.Name == "" {
			continue
		}
		member.ID = ""
		result = append(result, member)
	}
	return result
}

func (h *healerMonitor) resolveMembers(runtime *nativeReminderRuntime) {
	for index := range h.settings.Members {
		member := &h.settings.Members[index]
		if member.Self {
			member.ID = runtime.localID
			member.Key = "self"
			continue
		}
		if member.Name == "" {
			entity := runtime.entities[member.ID]
			if entity != nil && entity.Known && entity.OwnerID == "" && battleRecordPCRace(entity.RaceID) && member.ID != runtime.localID && h.observed[member.ID] != nil {
				member.Name = entity.Name
			}
		}
		if member.Name != "" {
			matches := []string{}
			for _, player := range h.party.Members {
				if player.ID != runtime.localID && player.Name == member.Name {
					matches = append(matches, player.ID)
				}
			}
			if len(matches) == 0 {
				for id, entity := range runtime.entities {
					observed := h.observed[id]
					if id != runtime.localID && entity.Known && entity.OwnerID == "" && battleRecordPCRace(entity.RaceID) && entity.Name == member.Name && observed != nil && observed.visible {
						matches = append(matches, id)
					}
				}
			}
			if len(matches) == 1 {
				member.ID = matches[0]
			} else {
				member.ID = ""
			}
		}
		member.Key = healerMemberKey(*member)
		*member = h.withArcanaPreset(*member)
	}
}
