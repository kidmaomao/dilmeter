export interface HealerSound { kind: string; soundId: string; name: string }
export interface HealerBuffRule {
	deathLoss: { enabled: boolean; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number };
 ccId: number; name: string; warningSeconds: number; overlayEnabled: boolean;
 flashEnabled: boolean; flashSeconds: number; durationMode: 'auto' | 'manual';
 manualDurationSeconds: number; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number;
}
export interface HealerSkillRule { skillId: number; name: string; cooldownSeconds: number; sound: HealerSound; fromArcana?: boolean }
export interface HealerSkillSettings { enabled: boolean; soundEnabled: boolean; overlay: HealerPosition; rules: HealerSkillRule[] }
// Power's preparation/charging does not start a cooldown. It belongs in burst cast/effect reminders.
const cooldownSkills = (value: HealerSkillSettings | undefined, index = 0): HealerSkillSettings => ({ ...(value ?? makeHealerSkills(index)), rules: (value?.rules ?? []).filter(rule => rule.skillId !== 58014) });
export const makeHealerSkills = (index = 0): HealerSkillSettings => ({ enabled: false, soundEnabled: true, overlay: { enabled: true, x: 40, y: 240 + index * 72 }, rules: [] });
export interface HealerPosition { enabled: boolean; x: number; y: number }
export type HealerResourceSettings = { repeatCount: number; repeatIntervalSeconds: number; threshold: number; soundEnabled: boolean; sound: HealerSound; overlay: HealerPosition };
export const makeHealerMana = (index = 0): HealerResourceSettings => ({ repeatCount: 1, repeatIntervalSeconds: 5, threshold: 25, soundEnabled: true, sound: makeHealerSound("electronic"), overlay: { enabled: true, x: 850, y: 180 + index * 110 } });
export interface HealerMemberChoice {
 self?: boolean; appliedArcana?: number;
 mana?: boolean; manaSettings?: HealerResourceSettings;
 key: string; id: string; name: string; included: boolean; favorite: boolean; health: boolean;
 skillSettings: HealerSkillSettings;
 healthSettings: { repeatCount: number; repeatIntervalSeconds: number; threshold: number; soundEnabled: boolean; sound: HealerSound; overlay: HealerPosition };
 buffSettings: { enabled: boolean; soundEnabled: boolean; overlay: HealerPosition; rules: HealerBuffRule[] };
}
export type HealerMemberTemplate = Pick<HealerMemberChoice, 'health' | 'healthSettings' | 'buffSettings' | 'skillSettings'> & { id: string; name: string };
export interface HealerSettings {
 autoAddInDungeon: boolean; autoAddTemplateId: string;
 version: number; enabled: boolean; volume: number; iconSize: number; soundEnabled: boolean;
 text: { enabled: boolean; fontSize: number; scalePercent: number; x: number; y: number; width: number };
 opacityPercent: number; templates: HealerMemberTemplate[]; members: HealerMemberChoice[];
}
export interface HealerBuffState { state: string; remainingSeconds: number | null; lossReason?: 'death' }
export interface HealerMemberState {
 key: string; id: string; name: string; active: boolean; waitingReason?: string;
 automatic?: boolean; arcana?: string; partySlot?: number; curtainAmount?: number;
 skills: Record<number, HealerBuffState>;
 manaState?: string; manaPercent?: number | null; manaCurrent?: number; manaMaximum?: number; healthState: string; healthPercent: number | null; buffs: Record<number, HealerBuffState>;
}
export interface HealerMonitorState {
 settings: HealerSettings; members: HealerMemberState[]; alerts: { key: string; name: string; message: string }[];
 capturing: boolean; inDungeon: boolean; updatedAt: number;
 dungeonSession: number; temporaryMembers?: HealerMemberChoice[];
}
export interface HealerCard { skillId?: number; key: string; memberKey: string; name: string; title: string; value: string; ccId: number; category: string; state: string; flash: boolean; x: number; y: number }
export function healerStatusBadge(card: Pick<HealerCard, 'state' | 'value'>): string {
 if (card.state === 'unknown' || card.value === '未观测') return '?';
 if (card.value === '死亡丢失') return '死亡';
 if (card.state === 'missing') return '!';
 if (!/^\d+s$/.test(card.value)) return card.value;
 const seconds = Number.parseInt(card.value, 10);
 if (seconds < 1000) return String(seconds);
 const minutes = Math.floor(seconds / 60);
 return minutes < 1000 ? minutes + 'm' : Math.floor(minutes / 60) + 'h';
}

export interface HealerTeamRow {
 key: string; name: string; partySlot?: number; skills?: HealerCard[]; buffs?: HealerCard[];
 health?: HealerCard; mana?: HealerCard; curtain?: string;
}
export interface HealerOverlayGroup {
 key: string; kind: 'team'; x: number; y: number; width: number; height: number;
 nameWidth?: number; cellWidth?: number; cellHeight?: number; columns?: number; rows?: HealerTeamRow[];
}
export interface HealerOverlayFrame { groups: HealerOverlayGroup[]; x: number; y: number; width: number; height: number; fontSize: number; iconSize: number; scalePercent: number; dpiPercent: number; opacityPercent: number; preview: boolean; updatedAt: number }
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
export const makeHealerSelf = (): HealerMemberChoice => ({ ...makeHealerMember({ id: '', name: '自身角色' }, 0), self: true, key: 'self', favorite: true, mana: true, manaSettings: makeHealerMana() });
export const healerTemplateSummary = (template: HealerMemberTemplate): string => `${template.health ? `血量 ≤${template.healthSettings.threshold}%` : '血量关闭'} · ${template.buffSettings.rules.length} 个 Buff · ${template.skillSettings.rules.length} 个技能`;
export const makeHealerSettings = (): HealerSettings => ({ version: 4, enabled: false, autoAddInDungeon: false, autoAddTemplateId: '', volume: 70, iconSize: 28, opacityPercent: 100, templates: [], soundEnabled: true, text: { enabled: true, fontSize: 16, scalePercent: 100, x: 600, y: 180, width: 660 }, members: [] });
export const cloneHealerSettings = (value: HealerSettings): HealerSettings => JSON.parse(JSON.stringify({
 ...value, autoAddInDungeon: value.autoAddInDungeon ?? false, autoAddTemplateId: value.autoAddTemplateId ?? '',
 opacityPercent: value.opacityPercent ?? 100,
 text: { ...value.text, scalePercent: value.text.scalePercent ?? 100 },
 templates: (value.templates ?? []).map(({ id, name, health, healthSettings, buffSettings, skillSettings }, index) => ({
  id, name, health, healthSettings, buffSettings, skillSettings: cooldownSkills(skillSettings, index),
 })),
 members: value.members.map(({ mana, manaSettings, ...member }, index) => ({
  ...member, skillSettings: cooldownSkills(member.skillSettings, index),
  ...(member.self ? { mana: mana ?? false, manaSettings: manaSettings ?? makeHealerMana(index) } : {}),
 })),
}));
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
