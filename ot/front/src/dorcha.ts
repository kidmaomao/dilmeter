import type { BuffStackAlertOverlayItem, SkillCooldownRule } from "./skillCooldown";

export const DORCHA_MASTERY_SKILL_ID = 27000;
export const DORCHA_STAT_ID = 196;
export const DORCHA_MAXIMUM = 15;
export const DEFAULT_DORCHA_THRESHOLD = 3;

export function normalizeDorchaThreshold(value: unknown): number {
    const n = Number(value);
    return Number.isFinite(n) && n >= 1 && n <= 15 ? Math.round(n) : DEFAULT_DORCHA_THRESHOLD;
}

export function formatDorchaQuantity(value: number): string {
    // Truncate display precision so 2.999 cannot look like 3 while warning.
    return String(Math.floor(Math.min(15, Math.max(0, value)) * 100) / 100);
}

export interface DorchaQuantityRuntime {
    observed: boolean;
    quantity: number;
    below: boolean;
    threshold: number;
    triggeredAtMs: number;
    generation: number;
}

export function initialDorchaQuantityRuntime(): DorchaQuantityRuntime {
    return { observed: false, quantity: 0, below: false, threshold: 3, triggeredAtMs: 0, generation: 0 };
}

export function applyDorchaQuantityObservation(
    previous: DorchaQuantityRuntime, raw: unknown, rawThreshold: unknown, atMs: number,
): DorchaQuantityRuntime {
    const threshold = normalizeDorchaThreshold(rawThreshold);
    if (typeof raw !== "number" || !Number.isFinite(raw) || raw < 0 || raw > 15) {
        return { ...initialDorchaQuantityRuntime(), threshold, generation: previous.generation };
    }
    const below = raw < threshold;
    const crossed = below && (!previous.observed || !previous.below);
    const changedSetting = previous.observed && threshold !== previous.threshold;
    return {
        observed: true, quantity: raw, below, threshold,
        triggeredAtMs: crossed && !changedSetting ? atMs : previous.triggeredAtMs,
        generation: previous.generation + (crossed && !changedSetting ? 1 : 0),
    };
}

export function dorchaQuantityOverlayItem(
    state: DorchaQuantityRuntime, rule: SkillCooldownRule | undefined,
): BuffStackAlertOverlayItem | undefined {
    if (!rule?.enabled || rule.barOnly || (!rule.alwaysVisible && (!state.observed || !state.below))) return;
    return {
        ccId: 0, skillId: DORCHA_MASTERY_SKILL_ID,
        name: state.observed && state.below ? "多尔卡不足" : "多尔卡数量",
        stack: 0, quantityText: state.observed ? formatDorchaQuantity(state.quantity) : "--", quantityUnit: "/ 15",
        persistent: true, startedAtMs: state.triggeredAtMs, endsAtMs: 0, generation: state.generation,
        x: rule.x, y: rule.y, scalePercent: 100,
    };
}
