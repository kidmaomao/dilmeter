export interface HealerSound { kind: string; soundId: string; name: string }
export interface HealerBuffRule {
	deathLoss: { enabled: boolean; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number };
 ccId: number; name: string; warningSeconds: number; overlayEnabled: boolean;
 flashEnabled: boolean; flashSeconds: number; durationMode: 'auto' | 'manual';
 manualDurationSeconds: number; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number;
}
export interface HealerSkillRule { skillId: number; name: string; cooldownSeconds: number; sound: HealerSound }
export interface HealerSkillSettings { enabled: boolean; soundEnabled: boolean; overlay: HealerPosition; rules: HealerSkillRule[] }
// Power's preparation/charging does not start a cooldown. It belongs in burst cast/effect reminders.
const cooldownSkills = (value: HealerSkillSettings | undefined, index = 0): HealerSkillSettings => ({ ...(value ?? makeHealerSkills(index)), rules: (value?.rules ?? []).filter(rule => rule.skillId !== 58014) });
export const makeHealerSkills = (index = 0): HealerSkillSettings => ({ enabled: false, soundEnabled: true, overlay: { enabled: true, x: 40, y: 240 + index * 72 }, rules: [] });
export interface HealerPosition { enabled: boolean; x: number; y: number }
export interface HealerMemberChoice {
 key: string; id: string; name: string; included: boolean; favorite: boolean; health: boolean;
 skillSettings: HealerSkillSettings;
 healthSettings: { repeatCount: number; repeatIntervalSeconds: number; threshold: number; soundEnabled: boolean; sound: HealerSound; overlay: HealerPosition };
 buffSettings: { enabled: boolean; soundEnabled: boolean; overlay: HealerPosition; rules: HealerBuffRule[] };
}
export type HealerMemberTemplate = Pick<HealerMemberChoice, 'health' | 'healthSettings' | 'buffSettings' | 'skillSettings'> & { id: string; name: string };
export interface HealerSettings {
 version: number; enabled: boolean; volume: number; iconSize: number; soundEnabled: boolean;
 text: { enabled: boolean; fontSize: number; x: number; y: number; width: number };
 opacityPercent: number; templates: HealerMemberTemplate[]; members: HealerMemberChoice[];
}
export interface HealerBuffState { state: string; remainingSeconds: number | null; lossReason?: 'death' }
export interface HealerMemberState {
 key: string; id: string; name: string; active: boolean; waitingReason?: string;
 skills: Record<number, HealerBuffState>;
 healthState: string; healthPercent: number | null; buffs: Record<number, HealerBuffState>;
}
export interface HealerMonitorState {
 settings: HealerSettings; members: HealerMemberState[]; alerts: { key: string; name: string; message: string }[];
 capturing: boolean; updatedAt: number;
}
export interface HealerCard { skillId?: number; key: string; memberKey: string; name: string; title: string; value: string; ccId: number; category: string; state: string; flash: boolean; x: number; y: number }
export interface HealerOverlaySection { kind: 'skill' | 'buff'; cards: HealerCard[] }
export interface HealerStatusLayout { width: number; height: number; nameWidth: number; cellWidth: number; cellHeight: number; columns: number; labelWidth: number; labelFontSize: number; sections: HealerOverlaySection[] }
export interface HealerOverlayGroup { key: string; name: string; kind: string; x: number; y: number; width: number; height: number; nameWidth: number; cellWidth: number; cards: HealerCard[]; cellHeight?: number; columns?: number; labelWidth?: number; labelFontSize?: number; sections?: HealerOverlaySection[] }
export interface HealerOverlayFrame { groups: HealerOverlayGroup[]; x: number; y: number; width: number; height: number; fontSize: number; iconSize: number; opacityPercent: number; preview: boolean; updatedAt: number }
export const makeHealerSound = (kind: string): HealerSound => ({ kind, soundId: '', name: '' });
export const makeHealerRule = (ccId: number, name: string, warning = 10): HealerBuffRule => ({
 ccId, name, warningSeconds: warning, overlayEnabled: true, flashEnabled: true,
 flashSeconds: warning, durationMode: 'auto', manualDurationSeconds: 60, repeatCount: 1, repeatIntervalSeconds: 5,
 sound: makeHealerSound(ccId === 680 || ccId === 192 ? 'healer-music' : 'healer-buff'),
 deathLoss: { enabled: true, sound: makeHealerSound('healer-death'), repeatCount: 1, repeatIntervalSeconds: 5 },
});
export const makeHealerMember = (member: Pick<HealerMemberState, 'id' | 'name'>, index: number): HealerMemberChoice => ({
 key: `player:${member.name}`, id: member.id, name: member.name, included: true, favorite: false, health: false,
 skillSettings: makeHealerSkills(index),
 healthSettings: { repeatCount: 1, repeatIntervalSeconds: 5, threshold: 25, soundEnabled: true, sound: makeHealerSound('healer-health'), overlay: { enabled: true, x: 600, y: 180 + index * 110 } },
 buffSettings: { enabled: false, soundEnabled: true, overlay: { enabled: true, x: 40, y: 160 + index * 72 }, rules: [] },
});
export const makeHealerSettings = (): HealerSettings => ({ version: 2, enabled: false, volume: 70, iconSize: 30, opacityPercent: 100, templates: [], soundEnabled: true, text: { enabled: true, fontSize: 16, x: 600, y: 180, width: 660 }, members: [] });
export const cloneHealerSettings = (value: HealerSettings): HealerSettings => JSON.parse(JSON.stringify({ ...value, opacityPercent: value.opacityPercent ?? 100, templates: (value.templates ?? []).map((member, index) => ({ ...member, skillSettings: cooldownSkills(member.skillSettings, index) })), members: value.members.map((member, index) => ({ ...member, skillSettings: cooldownSkills(member.skillSettings, index) })) }));
// Copy only reminder preferences so templates never replace the target identity or share mutable rules.
export function healerTemplateFromMember(member: HealerMemberChoice, id: string, name: string): HealerMemberTemplate {
 return JSON.parse(JSON.stringify({ id, name, health: member.health, healthSettings: member.healthSettings, buffSettings: member.buffSettings, skillSettings: cooldownSkills(member.skillSettings) }));
}
export function applyHealerTemplate(member: HealerMemberChoice, template: HealerMemberTemplate): void {
 const copy = JSON.parse(JSON.stringify(template)) as HealerMemberTemplate;
 member.health = copy.health;
 member.healthSettings = copy.healthSettings;
 member.buffSettings = copy.buffSettings;
 member.skillSettings = cooldownSkills(copy.skillSettings);
}

export const healerLabelWidth = (text: string, fontSize: number) => Math.ceil([...text].reduce((units, char) => units + (char.codePointAt(0)! < 128 ? 6 : 10), 0) * fontSize / 10) + 4;

// Mirror layoutHealerStatusGroup in Go for settings previews and live window bounds.
export function healerStatusLayout(name: string, cards: HealerCard[], fontSize: number, iconSize: number): HealerStatusLayout {
 const nameWidth = Math.min(Math.max(120, fontSize * 8), Math.max(healerLabelWidth('队友', Math.min(12, fontSize)), healerLabelWidth(name, fontSize)));
 const labelFontSize = Math.max(10, Math.min(16, Math.floor(fontSize * 3 / 4)));
 const labelWidth = healerLabelWidth('Buff', labelFontSize);
 const cellHeight = iconSize + 2 + Math.ceil(fontSize * 6 / 5);
 const sections: HealerOverlaySection[] = (['skill', 'buff'] as const).map(kind => ({ kind, cards: cards.filter(card => (card.category === 'skill') === (kind === 'skill')) })).filter(section => section.cards.length);
 const columns = Math.max(1, ...sections.map(section => Math.min(4, section.cards.length)));
 const cellWidth = Math.max(iconSize, ...cards.map(card => healerLabelWidth(card.value.replace(/s$/, ''), fontSize)), ...sections.map(section => healerLabelWidth(section.kind === 'skill' ? '未观测' : '00000', fontSize)));
 const contentHeight = sections.reduce((height, section, index) => {
  const rows = Math.ceil(section.cards.length / columns);
  return height + rows * cellHeight + (rows - 1) * 6 + (index ? 7 : 0);
 }, 0);
 return { nameWidth, labelFontSize, labelWidth, cellWidth, cellHeight, columns, sections, width: 18 + nameWidth + 6 + labelWidth + 4 + columns * cellWidth + (columns - 1) * 4, height: 14 + Math.max(cellHeight, contentHeight) };
}

// Both settings previews mirror the single live Buff/skill strip.
export function healerCombinedPreview(member: HealerMemberChoice, fontSize: number, iconSize: number): HealerOverlayGroup {
 const base = { memberKey: member.key, name: member.name, flash: false, x: 0, y: 0 };
 const cards: HealerCard[] = [
  ...member.buffSettings.rules.filter(rule => rule.overlayEnabled).map((rule, index) => ({ ...base, key: `buff:${rule.ccId}`, title: rule.name, value: ['1380', '24', '生效'][index % 3], ccId: rule.ccId, category: 'buff', state: 'active' })),
  ...member.skillSettings.rules.map((rule, index) => ({ ...base, key: `skill:${rule.skillId}`, title: rule.name, value: ['就绪', '24', '未观测'][index % 3], ccId: 0, skillId: rule.skillId, category: 'skill', state: index % 3 === 2 ? 'unknown' : 'ready' })),
 ];
 return { key: 'buff', kind: 'buff', name: member.name, cards, x: 0, y: 0, ...healerStatusLayout(member.name, cards, fontSize, iconSize) };
}
