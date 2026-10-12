import type { EntityCondition, EntityConditionState } from "./eventActor";
import type { eventSkillAction } from "./protocols";
import { resolveBuffExpiresAt } from "./buffAlert";

export const LANCE_MOUNT_SKILLS = [20017, 59105, 59106];
// User-confirmed gains, before the optional +50% Arcana link reward.
export const LANCE_HEAT_GAINS: Readonly<Record<number, number>> = {
    59101: 2, 59102: 5, 59103: 3, 59104: 15, 59105: 15,
};
export const LANCE_ANNIHILATION = 59106;
export const lanceActionTime = (action: eventSkillAction) => action.AtMs > 0 ? action.AtMs / 1000 : action.At;

export function lanceConditionActive(condition: EntityCondition | undefined, at: number): boolean {
    if (!condition || condition.At > at) return false;
    return (resolveBuffExpiresAt(condition) ?? Infinity) > at;
}

// Freeze the state when the release arrives. Looking up the final snapshot
// of that second would incorrectly grant a Buff applied after the release.
export function captureLanceMount(action: eventSkillAction, actor: { conditionMap: Record<number, EntityCondition> } | undefined): eventSkillAction {
    if (!LANCE_MOUNT_SKILLS.includes(action.SkillId) || action.IsFallback
        || action.LanceMounted !== undefined || !actor) return action;
    return { ...action, LanceMounted: lanceConditionActive(actor.conditionMap[975], lanceActionTime(action)) };
}

export function lanceMountedAt(action: eventSkillAction, history: readonly EntityConditionState[]): boolean | null {
    if (typeof action.LanceMounted === "boolean") return action.LanceMounted;
    const at = lanceActionTime(action);
    const before = history.filter((state) => state.At < Math.floor(at)).at(-1);
    const sameSecond = history.filter((state) => state.At === Math.floor(at));
    const candidates = [...(before ? [before] : []), ...sameSecond];
    if (!candidates.length) return null;
    const statuses = candidates.map((state) => lanceConditionActive(state.List.find((condition) => condition.CCId === 975), at));
    // Legacy saved records have only second timestamps, so a transition in
    // the release second cannot establish which state preceded the skill.
    return statuses.every((value) => value === statuses[0]) ? statuses[0] : null;
}

export type LanceHeatSample = { action: eventSkillAction; basePoints: number | null };
export function lanceHeatSamples(actions: readonly eventSkillAction[], playerId: string, end: number,
    resetTimes: readonly number[] = []): LanceHeatSample[] {
    const result: LanceHeatSample[] = [], seen = new Set<string>();
    const resets = [...resetTimes].sort((a, b) => a - b);
    let resetIndex = 0, points: number | null = null;
    const own = actions.filter((action) => action.Id === playerId && !action.MechanicSignal
        && (!([59105, 59106].includes(action.SkillId)) || (action.SubSkillId ?? 0) <= 1)
        && (!action.SourceId || action.SourceId === playerId) && lanceActionTime(action) < end + 1)
        .sort((a, b) => lanceActionTime(a) - lanceActionTime(b) || (a.Sequence ?? 0) - (b.Sequence ?? 0));
    const releaseKey = (action: eventSkillAction) => `${action.SkillId}:${action.CombatActionId || lanceActionTime(action)}`;
    const confirmed = new Set(own.filter((action) => !action.IsFallback).map(releaseKey));
    for (const action of own) {
        const at = lanceActionTime(action);
        while (resetIndex < resets.length && resets[resetIndex] <= at) { points = null; resetIndex++; }
        if (action.IsFallback) {
            // A hit with no corresponding release cannot establish the gain or
            // consumption. Invalidate this cycle instead of silently undercounting.
            if ((action.SkillId === LANCE_ANNIHILATION || LANCE_HEAT_GAINS[action.SkillId] !== undefined)
                && !confirmed.has(releaseKey(action))) points = null;
            continue;
        }
        const key = `${action.SkillId}:${at}:${action.CombatActionId || 0}`;
        if (seen.has(key)) continue;
        seen.add(key);
        if (action.SkillId === LANCE_ANNIHILATION) {
            result.push({ action, basePoints: points });
            points = 0;
        } else if (points !== null && LANCE_HEAT_GAINS[action.SkillId] !== undefined) {
            points += LANCE_HEAT_GAINS[action.SkillId];
        }
    }
    return result;
}
