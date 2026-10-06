export interface BurstDisplay { enabled: boolean; x: number; y: number; scalePercent: number; soundEnabled: boolean }
export interface BurstRule {
 skillId: number; ccId: number; name: string; cooldownSeconds: number; castSeconds: number;
 cooldownAlwaysVisible: boolean; cooldownAlertEnabled: boolean; cooldownLeadSeconds: number;
 /** Legacy preference, read only during migration. */ cooldownMode?: 'always' | 'lead';
 ready: BurstDisplay; cast: BurstDisplay; effect: BurstDisplay; orientation: 'horizontal' | 'vertical';
}
export interface BurstSettings { enabled: boolean; includeSelf: boolean; includeTeammates: boolean; volume: number; rules: Record<number, BurstRule> }
export const BURST_STORAGE_KEY = 'dilmeter-burst-reminders-v1';
const number = (value: unknown, min: number, max: number, fallback: number) => value === undefined || value === null || !Number.isFinite(Number(value)) ? fallback : Math.max(min, Math.min(max, Number(value)));
export function normalizeBurstSettings(value: unknown): BurstSettings {
 const source = (value && typeof value === 'object' ? value : {}) as Partial<BurstSettings>;
 const rules: Record<number, BurstRule> = {};
 [59005, 58014].forEach((skillId, index) => {
  const collapse = skillId === 59005;
  const raw = source.rules?.[skillId];
  const display = (kind: 'ready' | 'cast' | 'effect', y: number): BurstDisplay => ({ enabled: raw?.[kind]?.enabled ?? (kind !== 'ready' || collapse), x: Math.round(number(raw?.[kind]?.x, -32000, 32000, 600)), y: Math.round(number(raw?.[kind]?.y, -32000, 32000, y + index * 120)), scalePercent: Math.round(number(raw?.[kind]?.scalePercent, 50, 200, 100)), soundEnabled: raw?.[kind]?.soundEnabled ?? kind !== 'effect' });
  const ready = display('ready', 260);
  if (!collapse) ready.enabled = false;
  rules[skillId] = { skillId, ccId: collapse ? 803 : 516, name: collapse ? '崩坏波动' : '万钧之力', cooldownSeconds: collapse ? number(raw?.cooldownSeconds, .1, 86400, 60) : 0, castSeconds: number(raw?.castSeconds, .1, 60, collapse ? 2 : 5), cooldownAlwaysVisible: raw?.cooldownAlwaysVisible ?? raw?.cooldownMode === 'always', cooldownAlertEnabled: raw?.cooldownAlertEnabled !== false, cooldownLeadSeconds: number(raw?.cooldownLeadSeconds, 0, 86400, 0), ready, cast: display('cast', 350), effect: display('effect', 450), orientation: raw?.orientation === 'vertical' ? 'vertical' : 'horizontal' };
  const rule = rules[skillId];
  for (const phase of ['ready', 'effect'] as const) Object.assign(rule[phase], { x: rule.cast.x, y: rule.cast.y, scalePercent: rule.cast.scalePercent });
 });
 return { enabled: source.enabled === true, includeSelf: source.includeSelf !== false, includeTeammates: source.includeTeammates !== false, volume: Math.round(number(source.volume, 0, 100, 80)), rules };
}
export function loadBurstSettings(): BurstSettings { try { return normalizeBurstSettings(JSON.parse(localStorage.getItem(BURST_STORAGE_KEY) || 'null')); } catch { return normalizeBurstSettings(null); } }
export function saveBurstSettings(settings: BurstSettings): void { localStorage.setItem(BURST_STORAGE_KEY, JSON.stringify(normalizeBurstSettings(settings))); }
