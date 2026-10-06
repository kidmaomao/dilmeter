import type { eventBase, eventCharacterConditionEnable } from "./protocols";
import { resolveBuffExpiresAt } from "./buffAlert";

// A performance is evidence of the caster's value, separate from the
// recipient's currently active Buff. Switching songs must not erase it.
export const eventIdMusicPerformance = 23;
export type MusicPerformance = Omit<eventCharacterConditionEnable, "EventId"> & { EndedAt?: number };
export const musicConditionIds = [680, 192, 193];

export function recordMusicPerformance(event: eventBase, performances: MusicPerformance[]) {
    if (event.EventId !== 4 && event.EventId !== eventIdMusicPerformance) return;
    const condition = event as eventCharacterConditionEnable;
    if (!musicConditionIds.includes(condition.CCId) || !condition.AttackerId || condition.AttackerId === "0") return;
    const { EventId: _, Sequence: __, ...performance } = condition;
    performances.push(performance);
}

export function endMusicPerformances(performances: MusicPerformance[], at: number, actorId?: string) {
    for (const performance of performances) {
        if (performance.At > at || (actorId && performance.Id !== actorId && performance.AttackerId !== actorId)) continue;
        performance.EndedAt = Math.min(performance.EndedAt ?? Infinity, at);
    }
}

export function musicPerformanceExpiry(performance: MusicPerformance): number | null {
    const expiry = resolveBuffExpiresAt(performance);
    return performance.EndedAt === undefined ? expiry : Math.min(expiry ?? Infinity, performance.EndedAt);
}

export function musicPerformanceInSession(performance: MusicPerformance, start: number, end: number): boolean {
    if (performance.At > end) return false;
    if (performance.At >= start) return true;
    const expiry = musicPerformanceExpiry(performance);
    // Only carry a preparatory performance while its recorded duration still
    // reaches this fight; unknown-duration and expired older casts stay out.
    return expiry !== null && expiry > start;
}
