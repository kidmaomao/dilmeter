import { healthPercentAt, type BattleInterval, type BattleVitalPoint } from './battleChartHistory';
export interface TeamDpsPlayer {
    entityId: string; label: string;
    damages: readonly { At: number; Damage: number }[];
    deadIntervals?: readonly BattleInterval[];
}
export interface TeamDpsPoint {
    x: number; y: number;
    custom: { from: number; to: number; damage: number; elapsed: number; hp?: number; dashed: boolean; reason: string };
}
export type TeamDpsMode = 'cumulative' | 'rolling';
export function buildTeamDpsTimeline(players: readonly TeamDpsPlayer[], startAt: number, endAt: number, requestedStep: number,
    boss: { health?: readonly BattleVitalPoint[]; invulnerable?: readonly BattleInterval[]; dpsMode?: TeamDpsMode; windowSeconds?: number } = {}) {
    const duration = Number.isFinite(startAt) && Number.isFinite(endAt) ? Math.max(0, endAt - startAt) : 0;
    const stepSeconds = Math.max(Number.isFinite(requestedStep) && requestedStep >= 1 ? requestedStep : 1, Math.ceil(duration / 12_000));
    const times = new Set<number>([0, duration]);
    for (let t = stepSeconds; t < duration; t += stepSeconds) times.add(t);
    for (const interval of [...(boss.invulnerable ?? []), ...players.flatMap(p => [...(p.deadIntervals ?? [])])])
        for (const t of [interval.from - startAt, interval.to - startAt]) if (t > 0 && t < duration) times.add(t);
    const elapsed = [...times].sort((a, b) => a - b);
    const rolling = boss.dpsMode === 'rolling';
    const windowSeconds = Math.max(1, Number.isFinite(boss.windowSeconds) ? boss.windowSeconds! : 5);
    const fromAt = (t: number) => rolling ? Math.max(0, t - windowSeconds) : 0;
    const active = (intervals: readonly BattleInterval[], at: number) => intervals.some(i => i.from <= at && at < i.to);
    const members = players.map(player => {
        const events = player.damages.filter(e => Number.isFinite(e.At) && Number.isFinite(e.Damage) && e.Damage > 0 && e.At >= startAt && e.At <= endAt).slice().sort((a,b) => a.At-b.At);
        let cursor = 0, expired = 0, damage = 0, windowDamage = 0;
        const points = elapsed.map(t => {
            while (cursor < events.length && events[cursor].At <= startAt + t) {
                const value = events[cursor++].Damage;
                damage += value; windowDamage += value;
            }
            if (rolling && t > windowSeconds) {
                while (expired < cursor && events[expired].At <= startAt + t - windowSeconds) windowDamage -= events[expired++].Damage;
            }
            const dead = active(player.deadIntervals ?? [], startAt + t);
            const immune = active(boss.invulnerable ?? [], startAt + t);
            return { x: t, y: t > 0 ? (rolling ? Math.max(0, windowDamage) / Math.min(t, windowSeconds) : damage / t) : 0, custom: { from: fromAt(t), to: t, elapsed: t, damage,
                hp: healthPercentAt(boss.health ?? [], startAt + t), dashed: dead || immune,
                reason: [immune ? 'Boss 无敌' : '', dead ? '倒地' : ''].filter(Boolean).join('、') } };
        });
        return { entityId: player.entityId, label: player.label, points };
    });
    const total: TeamDpsPoint[] = elapsed.map((t, i) => {
        const damage = members.reduce((sum, p) => sum + p.points[i].custom.damage, 0);
        const immune = active(boss.invulnerable ?? [], startAt + t);
        return { x: t, y: members.reduce((sum, p) => sum + p.points[i].y, 0), custom: { from: fromAt(t), to: t, elapsed: t, damage,
            hp: healthPercentAt(boss.health ?? [], startAt + t), dashed: immune, reason: immune ? 'Boss 无敌' : '' } };
    });
    const peak = total.reduce((best, p) => p.y > best.y ? p : best, total[0]);
    return { members, total, peak, stepSeconds };
}
/** Split outgoing segments, duplicating transition vertices so there are no gaps. */
export function splitTeamSegments(points: readonly TeamDpsPoint[], dashed: boolean, axis: 'time' | 'hp', damageMode = false) {
    const data: (TeamDpsPoint | null)[] = [];
    const convert = (p: TeamDpsPoint) => ({ ...p, x: axis === 'hp' ? p.custom.hp! : p.x, y: damageMode ? p.custom.damage : p.y });
    for (let i = 0; i + 1 < points.length; i++) {
        const a = points[i], b = points[i+1];
        if (a.custom.dashed !== dashed || (axis === 'hp' && (a.custom.hp === undefined || b.custom.hp === undefined))) {
            if (data.at(-1) !== null) data.push(null); continue;
        }
        if (!data.length || data.at(-1) === null) data.push(convert(a));
        data.push(convert(b));
    }
    return data;
}
