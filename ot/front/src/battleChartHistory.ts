/** Serializable history shared by live processing, worker import and battle files. */
export interface BattleVitalPoint { At: number; current?: number; maximum?: number; dead?: boolean }
export interface BattleInterval { from: number; to: number }
export function recordVital(history: BattleVitalPoint[], at: number, patch: Omit<BattleVitalPoint, 'At'>) {
    if (!Number.isFinite(at)) return;
    const previous = history.at(-1);
    const next = { ...previous, ...patch, At: at };
    if (previous && previous.current === next.current && previous.maximum === next.maximum && previous.dead === next.dead) return;
    history.push(next);
}
export function clipVitalHistory(history: readonly BattleVitalPoint[], start: number, end: number) {
    const before = history.filter(p => p.At < start).at(-1);
    return [...(before ? [{ ...before, At: start }] : []), ...history.filter(p => p.At >= start && p.At <= end).map(p => ({ ...p }))];
}
export function deadIntervals(history: readonly BattleVitalPoint[], end: number): BattleInterval[] {
    const result: BattleInterval[] = []; let from: number | undefined;
    for (const point of history) {
        if (point.dead && from === undefined) from = point.At;
        if (point.dead === false && from !== undefined) { result.push({ from, to: point.At }); from = undefined; }
    }
    if (from !== undefined) result.push({ from, to: end });
    return result;
}
// Explicit invulnerability conditions. A pause in damage alone is never proof of immunity.
const INVULNERABLE = new Set([116, 494, 776, 840]);
export function invulnerableIntervals(history: readonly { At: number; List: readonly { CCId: number; DisableAt?: number; DisableAtMs?: number }[] }[], end: number): BattleInterval[] {
    return history.flatMap((point, i) => point.List.filter(c => INVULNERABLE.has(c.CCId)).map(c => ({
        from: point.At, to: Math.min(history[i + 1]?.At ?? end, end,
            c.DisableAtMs ? c.DisableAtMs / 1000 : c.DisableAt || end),
    }))).filter(p => p.to > p.from);
}
export function healthPercentAt(history: readonly BattleVitalPoint[], at: number): number | undefined {
    let lo = 0, hi = history.length;
    while (lo < hi) { const mid = (lo + hi) >>> 1; if (history[mid].At <= at) lo = mid + 1; else hi = mid; }
    const p = history[lo - 1];
    return p && Number.isFinite(p.current) && Number.isFinite(p.maximum) && p.maximum! > 0
        ? Math.max(0, Math.min(100, p.current! / p.maximum! * 100)) : undefined;
}
