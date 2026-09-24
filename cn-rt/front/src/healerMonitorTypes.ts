export interface HealerSound { kind: string; soundId: string; name: string }
export interface HealerBuffRule {
	deathLoss: { enabled: boolean; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number };
 ccId: number; name: string; warningSeconds: number; overlayEnabled: boolean;
 flashEnabled: boolean; flashSeconds: number; durationMode: 'auto' | 'manual';
 manualDurationSeconds: number; sound: HealerSound; repeatCount: number; repeatIntervalSeconds: number;
}
export interface HealerPosition { enabled: boolean; x: number; y: number }
export interface HealerMemberChoice {
 key: string; id: string; name: string; included: boolean; favorite: boolean; health: boolean;
 healthSettings: { repeatCount: number; repeatIntervalSeconds: number; threshold: number; soundEnabled: boolean; sound: HealerSound; overlay: HealerPosition };
 buffSettings: { enabled: boolean; soundEnabled: boolean; overlay: HealerPosition; rules: HealerBuffRule[] };
}
export type HealerMemberTemplate = Pick<HealerMemberChoice, 'health' | 'healthSettings' | 'buffSettings'> & { id: string; name: string };
export interface HealerSettings {
 version: number; enabled: boolean; volume: number; iconSize: number; soundEnabled: boolean;
 text: { enabled: boolean; fontSize: number; x: number; y: number; width: number };
 opacityPercent: number; templates: HealerMemberTemplate[]; members: HealerMemberChoice[];
}
export interface HealerBuffState { state: string; remainingSeconds: number | null; lossReason?: 'death' }
export interface HealerMemberState {
 key: string; id: string; name: string; active: boolean; waitingReason?: string;
 healthState: string; healthPercent: number | null; buffs: Record<number, HealerBuffState>;
}
export interface HealerMonitorState {
 settings: HealerSettings; members: HealerMemberState[]; alerts: { key: string; name: string; message: string }[];
 capturing: boolean; updatedAt: number;
}
export interface HealerCard { key: string; memberKey: string; name: string; title: string; value: string; ccId: number; category: string; state: string; flash: boolean; x: number; y: number }
export interface HealerOverlayGroup { key: string; name: string; kind: string; x: number; y: number; width: number; height: number; nameWidth: number; cellWidth: number; cards: HealerCard[] }
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
 healthSettings: { repeatCount: 1, repeatIntervalSeconds: 5, threshold: 25, soundEnabled: true, sound: makeHealerSound('healer-health'), overlay: { enabled: true, x: 600, y: 180 + index * 110 } },
 buffSettings: { enabled: false, soundEnabled: true, overlay: { enabled: true, x: 40, y: 160 + index * 72 }, rules: [] },
});
export const makeHealerSettings = (): HealerSettings => ({ version: 2, enabled: false, volume: 70, iconSize: 30, opacityPercent: 100, templates: [], soundEnabled: true, text: { enabled: true, fontSize: 16, x: 600, y: 180, width: 660 }, members: [] });
export const cloneHealerSettings = (value: HealerSettings): HealerSettings => JSON.parse(JSON.stringify({ ...value, opacityPercent: value.opacityPercent ?? 100, templates: value.templates ?? [] }));
// Copy only reminder preferences so templates never replace the target identity or share mutable rules.
export function healerTemplateFromMember(member: HealerMemberChoice, id: string, name: string): HealerMemberTemplate {
 return JSON.parse(JSON.stringify({ id, name, health: member.health, healthSettings: member.healthSettings, buffSettings: member.buffSettings }));
}
export function applyHealerTemplate(member: HealerMemberChoice, template: HealerMemberTemplate): void {
 const copy = JSON.parse(JSON.stringify(template)) as HealerMemberTemplate;
 member.health = copy.health;
 member.healthSettings = copy.healthSettings;
 member.buffSettings = copy.buffSettings;
}

export const healerLabelWidth = (text: string, fontSize: number) => Math.ceil([...text].reduce((units, char) => units + (char.codePointAt(0)! < 128 ? 6 : 10), 0) * fontSize / 10) + 4;
