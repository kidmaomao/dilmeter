export const DARK_ENERGY_SKILL_ID = 59047;
export const HOLY_ENERGY_SKILL_ID = 59085;
export function isEnergySkill(id: number) { return id === DARK_ENERGY_SKILL_ID || id === HOLY_ENERGY_SKILL_ID; }
export interface SkillEnergyState { Percent: number; Active: boolean }
export function holyEnergyState(condition?: { Metadata?: string; DisableAt?: number; DisableAtMs?: number }, atMs = Date.now()): SkillEnergyState | undefined {
    if (!condition) return undefined;
    const expiry = condition.DisableAtMs || (condition.DisableAt || 0) * 1000;
    if (expiry > 0 && expiry <= atMs) return undefined;
    const match = condition.Metadata?.match(/(?:^|;)MCNPGV:f:([^;]+);/);
    const percent = match ? Number(match[1]) : condition.Metadata?.includes('MCNPGIPT:f:') ? 0 : NaN;
    return Number.isFinite(percent) && percent >= 0 && percent <= 100 ? { Percent: percent, Active: true } : undefined;
}
export function energyReminderReady(id: number, state: SkillEnergyState | undefined, threshold: number, cooldown: { usedAtMs: number; readyAtMs: number } | undefined, now: number) {
    return Boolean(state?.Active && state.Percent >= threshold && (id !== DARK_ENERGY_SKILL_ID || (cooldown && cooldown.usedAtMs > 0 && cooldown.readyAtMs <= now)));
}
