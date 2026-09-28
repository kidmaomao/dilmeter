import type { TeamDpsPoint } from './teamDps';

export interface PeakCondition {
    CCId: number; At?: number; AttackerId?: string; DisableAt?: number; DisableAtMs?: number;
}
export interface PeakDamage {
    At: number; Damage: number; SkillId?: number; Conditions?: readonly PeakCondition[]; TargetConditions?: readonly PeakCondition[];
}
export interface PeakPlayer { entityId: string; label: string; damages: readonly PeakDamage[] }
export interface PeakAction { Id: string; At: number; AtMs?: number; SkillId: number; IsFallback?: boolean }
export interface PeakActor { label: string; ownerId?: string; isLocal?: boolean; isTeamMember?: boolean }
export interface PeakContext {
    actions?: readonly PeakAction[];
    actors?: Readonly<Record<string, PeakActor>>;
    skillName?: (id: number) => string;
    conditionName?: (id: number) => string;
}
export interface PeakContribution {
    actorId: string; actorLabel: string; skillId: number; name: string; damage: number; hits: number; share: number; increase?: number;
}
export interface PeakEffect {
    key: string; name: string; source: string; sourceId: string; target: string;
    evidence: 'condition' | 'action'; at: number; newlyApplied: boolean;
}
export interface DpsPeak {
    id: string; point: TeamDpsPoint; title: string; contributions: PeakContribution[];
    effects: PeakEffect[]; damage: number; previousDps?: number;
}

// Names and mappings are from the bundled CN resource and the existing
// server-confirmed technique-condition mappings. A condition is context, never
// a claimed damage multiplier. In particular 520 is the state of Force of Nature.
const SUPPORT_SKILLS: Readonly<Record<number, string>> = {
    58005: '时间扭曲', 58014: '万钧之力', 59005: '崩坏波动', 53101: '战争序曲',
};
const EFFECTS: Readonly<Record<number, { name: string; skillId?: number; transientOnly?: boolean }>> = {
    487: { name: '时间扭曲', skillId: 58005 },
    520: { name: '万钧之力（强健的状态）', skillId: 58014 },
    803: { name: '崩坏波动', skillId: 59005 },
    680: { name: '战争序曲', skillId: 53101 },
    321: { name: '无限连击' }, 322: { name: '无限连击最大化' },
    500: { name: '连击发动几率增加' }, 508: { name: '连击发动几率增加' }, 1087: { name: '连击伤害增加' },
    63: { name: '攻击力增加', transientOnly: true }, 388: { name: '最大伤害增加', transientOnly: true },
    392: { name: '闪电风暴' }, 426: { name: '死亡锁定' }, 504: { name: '防御削弱' },
    598: { name: '卡丁车水炸弹' }, 913: { name: '喵咪的奔袭' }, 1004: { name: '锐利之眼' },
    1093: { name: '最大保护减少' }, 1094: { name: '最大魔法保护减少' }, 1138: { name: '三叶草印记' },
    1164: { name: '防御／保护减少' }, 1165: { name: '魔法防御／魔法保护减少' }, 1166: { name: '增加受到的伤害' },
    1225: { name: '超燃咚咚' },
};

function sourceLabel(id: string, actors: PeakContext['actors']): string {
    const actor = actors?.[id];
    if (!id || id === '0' || !actor) return '来源未记录';
    if (actor.isLocal) return '自己';
    if (actor.ownerId) {
        const owner = actors?.[actor.ownerId];
        if (owner?.isLocal) return '自己的召唤物';
        if (owner?.isTeamMember) return `队友 ${owner.label} 的召唤物`;
    }
    return actor.isTeamMember ? `队友 ${actor.label}` : `其他角色 ${actor.label}`;
}
function conditionActive(c: PeakCondition, at: number) {
    const expires = c.DisableAtMs ? c.DisableAtMs / 1000 : c.DisableAt;
    return (!c.At || c.At <= at) && (!expires || expires > at);
}
function inWindow(at: number, from: number, to: number, includesStart: boolean) {
    return at <= to && (includesStart ? at >= from : at > from);
}
function prominentEffect(effect: PeakEffect) {
    return /^(487|520|803):/.test(effect.key) || /^action:.*:(58005|58014|59005)$/.test(effect.key);
}

/** Inspect the exact same trailing window used by the plotted DPS point. */
export function explainDpsPeak(point: TeamDpsPoint, players: readonly PeakPlayer[], startAt: number, context: PeakContext = {}): DpsPeak {
    const from = startAt + point.custom.from, to = startAt + point.custom.to;
    const width = point.custom.to - point.custom.from;
    const previousFrom = Math.max(startAt, from - width);
    const contributions = new Map<string, PeakContribution>();
    const previous = new Map<string, number>();
    const effects = new Map<string, PeakEffect>();
    const actions = (context.actions ?? []).filter(a => !a.IsFallback && SUPPORT_SKILLS[a.SkillId]);
    let damage = 0, previousDamage = 0;
    for (const player of players) {
        for (const hit of player.damages) {
            if (!Number.isFinite(hit.Damage) || hit.Damage <= 0 || !Number.isFinite(hit.At)) continue;
            const skillId = hit.SkillId || 0, key = `${player.entityId}:${skillId}`;
            if (from > startAt && inWindow(hit.At, previousFrom, from, previousFrom === startAt)) {
                previous.set(key, (previous.get(key) ?? 0) + hit.Damage); previousDamage += hit.Damage;
            }
            if (!inWindow(hit.At, from, to, point.custom.from === 0)) continue;
            damage += hit.Damage;
            const row = contributions.get(key) ?? { actorId: player.entityId, actorLabel: player.label, skillId,
                name: context.skillName?.(skillId) || (skillId === 58009 ? '连击' : `技能 ${skillId}`), damage: 0, hits: 0, share: 0 };
            row.damage += hit.Damage; row.hits++; contributions.set(key, row);
            for (const [conditions, target] of [[hit.Conditions, player.label], [hit.TargetConditions, 'Boss']] as const) {
                for (const c of conditions ?? []) {
                    const definition = EFFECTS[c.CCId];
                    if (!definition || !conditionActive(c, hit.At)) continue;
                    const newlyApplied = Number.isFinite(c.At) && inWindow(c.At!, from, to, point.custom.from === 0);
                    if (definition.transientOnly && !newlyApplied) continue;
                    // A missing attacker is only filled from the same actor's
                    // confirmed self-use at this condition's application time.
                    const selfUse = target !== 'Boss' && c.At && definition.skillId
                        ? actions.find(a => a.Id === player.entityId && a.SkillId === definition.skillId && Math.abs(a.At - c.At!) <= 1) : undefined;
                    const sourceId = c.AttackerId && c.AttackerId !== '0' ? c.AttackerId : selfUse?.Id ?? '';
                    const effectKey = `${c.CCId}:${sourceId}:${target}`;
                    const old = effects.get(effectKey);
                    effects.set(effectKey, { key: effectKey,
                        name: context.conditionName?.(c.CCId) || definition.name,
                        source: sourceLabel(sourceId, context.actors), sourceId, target, evidence: 'condition',
                        at: Math.max(old?.at ?? 0, c.At ?? hit.At), newlyApplied: Boolean(old?.newlyApplied || newlyApplied) });
                }
            }
        }
    }
    for (const action of actions) {
        const at = action.AtMs && action.AtMs > 0 ? action.AtMs / 1000 : action.At;
        const actor = context.actors?.[action.Id];
        if (!actor || (!actor.isLocal && !actor.isTeamMember) || !inWindow(at, from, to, point.custom.from === 0)) continue;
        const covered = [...effects.values()].some(e => e.sourceId === action.Id &&
            Object.entries(EFFECTS).some(([ccId, definition]) => definition.skillId === action.SkillId && e.key.startsWith(`${ccId}:`)));
        if (covered) continue;
        const key = `action:${action.Id}:${action.SkillId}`;
        effects.set(key, { key, name: context.skillName?.(action.SkillId) || SUPPORT_SKILLS[action.SkillId],
            source: sourceLabel(action.Id, context.actors), sourceId: action.Id, target: '覆盖未确认', evidence: 'action', at, newlyApplied: true });
    }
    const rows = [...contributions.values()].sort((a,b) => b.damage-a.damage || a.skillId-b.skillId);
    for (const row of rows) {
        row.share = damage > 0 ? row.damage / damage : 0;
        if (from > startAt) row.increase = row.damage - (previous.get(`${row.actorId}:${row.skillId}`) ?? 0);
    }
    return { id: String(point.custom.elapsed), point, title: rows[0]?.name ?? '伤害峰值', contributions: rows, damage,
        previousDps: from > startAt ? previousDamage / (from - previousFrom) : undefined,
        effects: [...effects.values()].sort((a,b) => Number(prominentEffect(b))-Number(prominentEffect(a)) || Number(b.newlyApplied)-Number(a.newlyApplied) || b.at-a.at || a.name.localeCompare(b.name)) };
}

/** Keep the strongest separated local maxima so annotations remain readable. */
export function selectDpsPeaks(points: readonly TeamDpsPoint[], windowSeconds = 5, limit = 6): TeamDpsPoint[] {
    const candidates: TeamDpsPoint[] = [];
    const maximum = Math.max(0, ...points.map(p => p.y));
    for (let i = 0; i < points.length; i++) {
        if (!(points[i].y > 0)) continue;
        let end = i;
        while (end + 1 < points.length && points[end + 1].y === points[i].y) end++;
        if (points[i].y > (points[i-1]?.y ?? 0) && points[i].y > (points[end+1]?.y ?? 0) && points[i].y >= maximum * .25) candidates.push(points[end]);
        i = end;
    }
    const chosen: TeamDpsPoint[] = [], separation = Math.max(10, windowSeconds * 2);
    for (const point of candidates.sort((a,b) => b.y-a.y || a.x-b.x)) {
        if (chosen.every(p => Math.abs(p.custom.elapsed-point.custom.elapsed) >= separation)) chosen.push(point);
        if (chosen.length >= limit) break;
    }
    return chosen.sort((a,b) => a.custom.elapsed-b.custom.elapsed);
}
