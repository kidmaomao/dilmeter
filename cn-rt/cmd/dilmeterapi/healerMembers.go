package main

import (
	"fmt"
	"strings"
)

func healerMemberKey(member healerMemberSelection) string {
	if member.Name != "" {
		return "player:" + member.Name
	}
	return "entity:" + member.ID
}

func normalizeHealerMember(member healerMemberSelection, settings healerSettings, index int) healerMemberSelection {
	member.Name = strings.TrimSpace(member.Name)
	member.Name = string([]rune(member.Name)[:min(len([]rune(member.Name)), 100)])
	member.Key = healerMemberKey(member)
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
		member := normalizeHealerMember(healerMemberSelection{Name: template.Name, Health: template.Health, HealthSettings: template.HealthSettings, BuffSettings: template.BuffSettings}, settings, index)
		template.HealthSettings, template.BuffSettings = member.HealthSettings, member.BuffSettings
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
		if member.Name == "" {
			entity := runtime.entities[member.ID]
			if entity != nil && entity.Known && entity.OwnerID == "" && battleRecordPCRace(entity.RaceID) && member.ID != runtime.localID && h.observed[member.ID] != nil {
				member.Name = entity.Name
			}
		}
		if member.Name != "" {
			matches := []string{}
			for id, entity := range runtime.entities {
				observed := h.observed[id]
				if id != runtime.localID && entity.Known && entity.OwnerID == "" && battleRecordPCRace(entity.RaceID) && entity.Name == member.Name && observed != nil && observed.visible {
					matches = append(matches, id)
				}
			}
			if len(matches) == 1 {
				member.ID = matches[0]
			} else {
				member.ID = ""
			}
		}
		member.Key = healerMemberKey(*member)
	}
}
