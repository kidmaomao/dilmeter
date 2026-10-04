import type { EntityCondition, EntityConditionState } from "./eventActor";
import type { eventArcanaSignal, eventSkillAction, eventSkillCooldown, eventStatUpdate } from "./protocols";
import type { BossFightSession, TimeInterval } from "./summaryCollector";
import { holyEnergyState } from "./skillEnergy";
import { parseConditionStack, resolveBuffExpiresAt } from "./buffAlert";
import { estimateFighterIdle } from "./fighterEnergyBounds";

export type KpiAimSample = { entityId: string; atMs: number; rate: number; targetId?: string };
export type KpiStatus = "measured" | "estimated" | "no-samples" | "missing-data" | "pending";
export type ArcanaKpiRow = {
    id: string; label: string; value: number | null; unit: string;
    status: KpiStatus; detail: string; samples?: number; upperValue?: number;
};
export type ArcanaKpiReport = {
    jobName: string | null; activeSeconds: number; excludedSeconds: number;
    rows: ArcanaKpiRow[];
};
type Actor = { id: string; conditionHistory: readonly EntityConditionState[] };
export type ArcanaKpiInput = {
    player: Actor; boss: Actor; session: BossFightSession; jobName: string | null;
    localEntityId?: string;
    actions: readonly eventSkillAction[];
    skillCooldowns?: readonly eventSkillCooldown[];
    statUpdates?: readonly eventStatUpdate[];
    arcanaSignals?: readonly eventArcanaSignal[];
    aimSamples?: readonly KpiAimSample[];
    dorchaMinimum?: number;
};
type ConditionSlice = TimeInterval & { condition: EntityCondition };

// Every metric shares this clock. Damage gaps are not evidence of immunity:
// removing them would hide the waiting/idle behaviour these KPIs measure.
export function kpiActiveIntervals(boss: Actor, session: BossFightSession): TimeInterval[] {
    const immune = conditionSlices(boss, [277, 494], session.startAt, session.endAt);
    return subtractIntervals([{ start: session.startAt, end: session.endAt }], immune);
}

export function subtractIntervals(base: readonly TimeInterval[], excluded: readonly TimeInterval[]): TimeInterval[] {
    const cuts = [...excluded].sort((a, b) => a.start - b.start);
    return base.flatMap(({ start, end }) => {
        const result: TimeInterval[] = [];
        let cursor = start;
        for (const cut of cuts) {
            if (cut.end <= cursor || cut.start >= end) continue;
            if (cut.start > cursor) result.push({ start: cursor, end: Math.min(end, cut.start) });
            cursor = Math.max(cursor, cut.end);
            if (cursor >= end) break;
        }
        if (cursor < end) result.push({ start: cursor, end });
        return result;
    });
}

function duration(intervals: readonly TimeInterval[]) {
    return intervals.reduce((total, interval) => total + Math.max(0, interval.end - interval.start), 0);
}
function overlap(interval: TimeInterval, windows: readonly TimeInterval[]) {
    return windows.reduce((sum, window) => sum + Math.max(0,
        Math.min(interval.end, window.end) - Math.max(interval.start, window.start)), 0);
}
function contains(at: number, windows: readonly TimeInterval[], finalEnd: number) {
    return windows.some((window) => at >= window.start
        && (at < window.end || (window.end === finalEnd && at === finalEnd)));
}
function conditionSlices(actor: Actor, ids: readonly number[], start: number, end: number): ConditionSlice[] {
    const slices: ConditionSlice[] = [];
    const history = actor.conditionHistory;
    for (let i = 0; i < history.length; i++) {
        for (const condition of history[i].List) {
            if (!ids.includes(condition.CCId)) continue;
            const left = Math.max(start, history[i].At, condition.At);
            const right = Math.min(end, history[i + 1]?.At ?? end, resolveBuffExpiresAt(condition) ?? end);
            if (right > left) slices.push({ start: left, end: right, condition });
        }
    }
    return slices;
}
function actionTime(action: eventSkillAction) { return action.AtMs > 0 ? action.AtMs / 1000 : action.At; }
// The CN capture and the user's single-target enhancement confirmation
// establish 7.5 as the fully narrowed Hydro range parameter. Smaller,
// unsupported values are unknown rather than automatically "fully charged".
export function hydroChargeIsFull(rangeParameter: number | undefined): boolean | null {
    if (!Number.isFinite(rangeParameter) || rangeParameter! < 7.5 - 0.0001 || rangeParameter! > 180) return null;
    return Math.abs(rangeParameter! - 7.5) <= 0.0001;
}
function completeCastSamples(signals: readonly eventArcanaSignal[], windows: readonly TimeInterval[], end: number,
    minimum: number, maximum: number): eventArcanaSignal[] {
    const samples = new Map<number, eventArcanaSignal>();
    for (const signal of signals) {
        const first = Math.floor(signal.FirstHitAtMs! / 1000);
        if (!signal.Complete || !Number.isFinite(signal.CastAtMs) || signal.CastAtMs! <= 0
            || !Number.isFinite(signal.FirstHitAtMs) || signal.FirstHitAtMs! < signal.CastAtMs!
            || !Number.isFinite(signal.AtMs) || signal.AtMs < signal.FirstHitAtMs!
            || signal.At !== Math.floor(signal.AtMs / 1000)
            || !Number.isInteger(signal.Count) || signal.Count < minimum || signal.Count > maximum
            || !contains(first, windows, end) || !contains(signal.At, windows, end)
            || overlap({ start: first, end: signal.At }, windows) < signal.At - first - 0.001) continue;
        samples.set(signal.CastAtMs!, signal);
    }
    return [...samples.values()];
}
export function kpiMetadataNumber(metadata: string | undefined, key: string): number | null {
    const escaped = key.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
    const match = metadata?.match(new RegExp(`(?:^|;)${escaped}:(?:f|[1248]):([^;]+)(?:;|$)`));
    if (!match) return null;
    const value = Number(match[1]);
    return Number.isFinite(value) ? value : null;
}
function canonicalActions(input: ArcanaKpiInput) {
    const seen = new Set<string>();
    return input.actions.filter((action) => {
        // A damage-derived fallback cannot reveal missed casts or multi-hit counts.
        if (action.Id !== input.player.id || action.IsFallback || action.MechanicSignal
            || (action.SourceId && action.SourceId !== input.player.id)) return false;
        const at = actionTime(action);
        if (at < input.session.startAt || at >= input.session.endAt + 1) return false;
        const key = `${action.SkillId}:${at}:${action.CombatActionId || 0}`;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
    }).sort((a, b) => actionTime(a) - actionTime(b));
}
function row(id: string, label: string, value: number | null, unit: string, detail: string,
    status: KpiStatus = value === null ? "no-samples" : "measured", samples?: number): ArcanaKpiRow {
    return { id, label, value, unit, status, detail, samples };
}
function pending(id: string, label: string, unit: string, detail: string): ArcanaKpiRow {
    return row(id, label, null, unit, detail, "pending");
}
function missing(id: string, label: string, unit: string, detail: string): ArcanaKpiRow {
    return row(id, label, null, unit, detail, "missing-data");
}

function fighterIdle(signals: readonly eventArcanaSignal[], playerId: string, windows: readonly TimeInterval[], end: number) {
    const own = signals.filter((signal) => signal.Id === playerId && Number.isFinite(signal.AtMs));
    const starts = own.filter((signal) => signal.Signal === "fighter-spend-start" && [59185, 59186, 59187].includes(signal.SkillId));
    const resets = own.filter((signal) => signal.Signal === "fighter-energy-reset").map((signal) => signal.AtMs).sort((a, b) => a - b);
    const finishes = new Map(own.filter((signal) => signal.Signal === "fighter-spend-end" && signal.AtMs >= (signal.CastAtMs ?? Infinity))
        .map((signal) => [`${signal.SkillId}:${signal.CastAtMs}`, signal.AtMs]));
    const busy = starts.map((start) => {
        const finish = finishes.get(`${start.SkillId}:${start.CastAtMs}`);
        const reset = resets.find((at) => at >= start.AtMs);
        return { start: start.AtMs / 1000, end: Math.min(end, (finish ?? end * 1000) / 1000, (reset ?? end * 1000) / 1000) };
    });
    const eligible = subtractIntervals(windows, busy);
    const energy = own.filter((signal) => signal.Signal.startsWith("fighter-energy-")).sort((a, b) => a.AtMs - b.AtMs);
    let state: { at: number; value: number; rate: number; maximum: number } | undefined;
    let observed = 0, idle = 0, baselines = 0, deltas = 0;
    const advance = (at: number) => {
        if (!state || at <= state.at) return;
        const until = Math.min(at, end), from = state.at;
        if (until > from) {
            observed += overlap({ start: from, end: until }, eligible);
            // Each unit is 100 points. Of the three spender skills, two
            // cost 100 and one costs 200, so 100 makes at least one usable.
            const ready = state.value >= 100 ? from : state.rate > 0 ? from + (100 - state.value) / state.rate : Infinity;
            if (ready < until) idle += overlap({ start: Math.max(from, ready), end: until }, eligible);
        }
        state.value = Math.min(state.maximum, state.value + Math.max(0, at - state.at) * state.rate);
        state.at = at;
    };
    for (const signal of energy) {
        const at = signal.AtMs / 1000;
        if (at > end) break;
        advance(at);
        if (signal.Signal === "fighter-energy-reset") { state = undefined; continue; }
        if (signal.Signal === "fighter-energy-bounds") { state = undefined; continue; }
        if (signal.Signal === "fighter-energy-baseline") {
            const value = signal.Value ?? 0, rate = signal.Rate ?? 0, maximum = signal.Maximum ?? 0;
            state = Number.isFinite(value) && Number.isFinite(rate) && Number.isFinite(maximum) && value >= 0
                && maximum >= 100 && value <= maximum && rate >= 0 ? { at, value, rate, maximum } : undefined;
            if (state) baselines++;
        } else if (state && signal.Signal === "fighter-energy-delta") {
            const delta = signal.Value ?? 0;
            if (!Number.isFinite(delta)) { state = undefined; continue; }
            // A decrease exceeding the known resource invalidates its
            // history; do not silently turn missing gains into zero energy.
            const next = state.value + delta;
            state = next >= -0.1 ? { ...state, value: Math.max(0, Math.min(state.maximum, next)) } : undefined;
        }
        if (signal.Signal === "fighter-energy-delta") deltas++;
    }
    advance(end);
    return { value: Math.abs(observed - duration(eligible)) <= 0.001 && energy.length ? idle : null,
        observedSeconds: observed, requiredSeconds: duration(eligible), baselines, deltas,
        bounds: estimateFighterIdle(energy, eligible, end) };
}

// User-approved CN test reference, estimated from the 24 fps normal-combo
// video (2026-10-03 10:12:47): approximately frames 45 -> 67, or 0.92 s.
// This is a visual reference, not a server timing constant or an in-fight sample.
const fighterNormalVideoBaselineSeconds = 0.92;

export function buildArcanaKpi(input: ArcanaKpiInput): ArcanaKpiReport {
    const { player, boss, session } = input;
    const windows = kpiActiveIntervals(boss, session);
    const activeSeconds = duration(windows);
    const valid = (at: number) => contains(at, windows, session.endAt);
    const actions = canonicalActions(input);
    const teammate = Boolean(input.localEntityId && player.id !== input.localEntityId);
    const onlyFallback = (ids: number[]) => !actions.some((action) => ids.includes(action.SkillId))
        && input.actions.some((action) => action.Id === player.id && action.IsFallback && ids.includes(action.SkillId)
            && actionTime(action) >= session.startAt && actionTime(action) <= session.endAt);
    const casts = (ids: number[]) => actions.filter((action) => ids.includes(action.SkillId) && valid(actionTime(action)));
    const bossSlices = (ids: number[]) => conditionSlices(boss, ids, session.startAt, session.endAt);
    const selfSlices = (ids: number[]) => conditionSlices(player, ids, session.startAt, session.endAt);
    const maximum = (id: string, label: string, actor: Actor, ccId: number, key: string, unit: string,
        detail: string, ownSource = false, transform: (value: number) => number = (value) => value) => {
        let value: number | null = null, samples = 0, observed = 0;
        const seen = new Set<string>();
        // Evaluate snapshots as well as positive-duration slices so same-second
        // metadata changes do not erase a recorded peak.
        for (let i = 0; i < actor.conditionHistory.length; i++) {
            const state = actor.conditionHistory[i];
            if (state.At > session.endAt || (state.At < session.startAt && (actor.conditionHistory[i + 1]?.At ?? Infinity) <= session.startAt)) continue;
            if (!valid(Math.max(state.At, session.startAt))) continue;
            for (const condition of state.List) {
                if (condition.CCId !== ccId || (ownSource && condition.AttackerId !== player.id)) continue;
                const expiry = resolveBuffExpiresAt(condition);
                if (expiry !== null && expiry <= Math.max(state.At, session.startAt)) continue;
                const identity = `${condition.At}:${condition.AttackerId}:${condition.Metadata}`;
                if (seen.has(identity)) continue;
                seen.add(identity); observed++;
                const recorded = kpiMetadataNumber(condition.Metadata, key);
                if (recorded === null) continue;
                samples++; const current = transform(recorded);
                value = value === null ? current : Math.max(value, current);
            }
        }
        return row(id, label, value, unit, value === null && observed === 0
            ? `${detail} 当前统计区间未观察到符合归属条件的记录。` : detail,
            value === null ? (observed || !actor.conditionHistory.length ? "missing-data" : "no-samples") : "measured", samples);
    };
    const fury = () => {
        if (!boss.conditionHistory.length) return missing("fury", "愤怒狂暴覆盖率", "%", "缺少首领状态时间轴。");
        const slices = bossSlices([323]);
        const seconds = activeSeconds - duration(subtractIntervals(windows, slices));
        return row("fury", "愤怒狂暴覆盖率", activeSeconds > 0 ? seconds / activeSeconds * 100 : null, "%",
            `首领所受愤怒狂暴状态覆盖时间 / 可输出时间；含队友施加的同类状态。覆盖 ${seconds.toFixed(2)} 秒。`);
    };
    const between = (id: string, label: string, anchor: number, inserted: number) => {
        if (onlyFallback([anchor]) || onlyFallback([inserted])) return missing(id, label, "次/间隔",
            "已收到该角色的伤害，但缺少完整技能释放确认，不能以伤害段数计算循环；旧日志可重新导入配套原始抓包。");
        const boundaries = actions.filter((action) => action.SkillId === anchor);
        let samples = 0, count = 0;
        for (let i = 1; i < boundaries.length; i++) {
            const start = actionTime(boundaries[i - 1]), end = actionTime(boundaries[i]);
            // Require both releases to belong to normal output. Do not bridge a
            // mechanic phase into a rotation sample, or count partial first/last pairs.
            if (!valid(start) || !valid(end) || overlap({ start, end }, windows) < end - start - 0.001) continue;
            samples++;
            count += casts([inserted]).filter((action) => actionTime(action) > start && actionTime(action) < end).length;
        }
        return row(id, label, samples ? count / samples : null, "次/间隔",
            `仅统计两次完整释放之间的次数；跨无敌区间及首尾不完整间隔不计。共 ${count} 次 / ${samples} 个间隔。`,
            actions.length ? undefined : "missing-data", samples);
    };
    const withoutChain = (id: string, label: string, ids: number[]) => {
        if (!boss.conditionHistory.length || !actions.length || onlyFallback(ids)) return missing(id, label, "次", "需要首领状态时间轴和本角色技能释放记录；伤害段数不能替代释放次数。");
        const chain = bossSlices([948]);
        const count = casts(ids).filter((action) => !chain.some((slice) => actionTime(action) >= slice.start && actionTime(action) < slice.end)).length;
        return row(id, label, count, "次", "按本角色确认释放次数统计，释放时首领未处于闪电链状态；排除无敌期间。");
    };
    const cooldownWait = (id: string, label: string, skill: number, cooldownSeconds: number) => {
        const releases = actions.filter((action) => action.SkillId === skill);
        const seen = new Set<string>();
        const adjustments = (input.skillCooldowns ?? []).filter((event) => {
            if (event.Id !== player.id || event.SkillId !== skill) return false;
            const at = event.AtMs > 0 ? event.AtMs / 1000 : event.At;
            if (!Number.isFinite(at) || (!event.Reset && !(Number.isFinite(event.ReduceMs) && event.ReduceMs > 0))) return false;
            const key = event.Sequence ? `sequence:${event.Sequence}` : `${at}:${event.Reset}:${event.ReduceMs}`;
            if (seen.has(key)) return false;
            seen.add(key);
            return true;
        }).sort((a, b) => (a.AtMs > 0 ? a.AtMs / 1000 : a.At) - (b.AtMs > 0 ? b.AtMs / 1000 : b.At));
        let total = 0, samples = 0, unknown = 0;
        for (let i = 1; i < releases.length; i++) {
            const previous = releases[i - 1], next = releases[i];
            const start = actionTime(previous), end = actionTime(next);
            if (!valid(end)) continue;
            let ready = start + cooldownSeconds;
            for (const adjustment of adjustments) {
                const at = adjustment.AtMs > 0 ? adjustment.AtMs / 1000 : adjustment.At;
                // Sequence disambiguates a reset and a cast acknowledged in
                // the same millisecond. Without it, use strictly later signals.
                const afterStart = at > start || (at === start && previous.Sequence && adjustment.Sequence && adjustment.Sequence > previous.Sequence);
                const beforeEnd = at < end || (at === end && next.Sequence && adjustment.Sequence && adjustment.Sequence < next.Sequence);
                if (!afterStart || !beforeEnd) continue;
                // A reset/reduction after the skill was already ready cannot
                // erase waiting that has already accrued.
                ready = Math.min(ready, adjustment.Reset ? at : Math.max(at, ready - adjustment.ReduceMs / 1000));
            }
            if (end < ready - 0.001) { unknown++; continue; }
            // Cooldown elapses in wall time even during immunity. Only idle
            // time AFTER readiness is clipped to the boss's active windows.
            total += overlap({ start: ready, end }, windows);
            samples++;
        }
        return row(id, label, samples ? total / samples : null, "秒",
            `当前角色技能冷却 ${cooldownSeconds} 秒后，到该角色下一次释放的平均等待时间；已记录的冷却重置／缩减会修正可用时刻。`
                + `冷却在首领无敌期间照常经过，等待时间扣除无敌；共 ${samples} 个完整周期，累计等待 ${total.toFixed(2)} 秒。`
                + "开场首次释放前及末次释放后的不完整等待不计。"
                + (unknown ? `另有 ${unknown} 个周期短于已知冷却且缺少对应重置信号，不计入平均。` : "")
                + (teammate ? "队友冷却调整可能不公开，本项按已知冷却及收到的调整估算。" : ""),
            samples ? teammate ? "estimated" : "measured" : unknown || !actions.length ? "missing-data" : "no-samples", samples);
    };
    let rows: ArcanaKpiRow[] = [];
    switch (input.jobName) {
        case "元素骑士": {
            const slices = selfSlices([321, 322]);
            let seconds = 0, weighted = 0, unknownSeconds = 0;
            for (const slice of slices) {
                const weight = overlap(slice, windows);
                const stack = kpiMetadataNumber(slice.condition.Metadata, "MCRSTKCNT") ?? parseConditionStack(slice.condition.Metadata);
                if (stack === null) { unknownSeconds += weight; continue; }
                seconds += weight; weighted += stack * weight;
            }
            rows = [fury(), unknownSeconds > 0
                ? missing("combo", "无限连击平均层数", "层", `有 ${unknownSeconds.toFixed(2)} 秒状态缺少层数参数，无法计算完整平均值。`)
                : row("combo", "无限连击平均层数", seconds > 0 ? weighted / seconds : null, "层", "在无限连击状态生效期间按持续时间加权平均；无敌期间不计。"),
                between("rush", "雷霆重击间隔内突进斩次数", 59026, 59028)];
            break;
        }
        case "圣光颂唱者":
            rows = [maximum("sonic-protection", "音波洗礼最高保护／魔法保护减少（取整记录）", boss, 1176, "SBPD", "", "本人对当前首领同时造成的保护、魔法保护减少，合并显示。日志提供已取整的总减益值，无需再乘层数；未提供技能说明中的每层精确小数。", true),
                maximum("sonic-curtain", "自身生命帷幕最高减伤率", player, 999, "SBBDDR", "%", "显示本人生命帷幕的伤害减免百分比；帷幕吸收量与自身追加吸收量不作为减伤率。", false, (value) => value * 100),
                maximum("music-war", "战争序曲演奏最高值（最大伤害）", player, 680, "MCMBAMAX", "%", "本人演奏带来的最大伤害增益；仅统计当前场次可输出期间生效的记录。", true),
                maximum("music-active-magic", "活跃进行曲演奏最高值（魔法攻击力提升）", player, 192, "LSMA", "%", "本人演奏带来的魔法攻击力提升百分比；与吟唱速度分别取最高值，仅统计当前场次可输出期间生效的记录。", true),
                maximum("music-active", "活跃进行曲演奏最高值（魔法吟唱速度）", player, 192, "MFCP", "%", "本人演奏带来的魔法吟唱速度增幅；与魔法攻击力分别取最高值，仅统计当前场次可输出期间生效的记录。", true),
                maximum("music-march", "行进曲演奏最高值（角色移动速度）", player, 193, "SPDPC", "%", "本人演奏带来的角色移动速度增益；仅统计当前场次可输出期间生效的记录。", true, (value) => (value - 1) * 100)];
            break;
        case "黑魔导士": {
            const dragons = casts([59040]);
            const chainStates = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id && signal.SkillId === 59041
                && Number.isFinite(signal.AtMs) && signal.AtMs / 1000 <= session.endAt
                && (signal.Signal === "lightning-chain-reset" || (signal.Signal === "lightning-chain-state" && signal.Kind === 814
                    && signal.Complete && (signal.Count === 0 || signal.Count === 1))))
                .sort((a, b) => a.AtMs - b.AtMs || (a.Sequence ?? 0) - (b.Sequence ?? 0));
            const chain = chainStates.flatMap((state, i) => state.Signal === "lightning-chain-state" && state.Count === 1
                ? [{ start: state.AtMs / 1000, end: (chainStates[i + 1]?.AtMs ?? (session.endAt + 1) * 1000) / 1000 }] : []);
            const chained = dragons.filter((action) => chain.some((slice) => actionTime(action) >= slice.start && actionTime(action) < slice.end)).length;
            const legacyThunder = casts([30102]).some((action) => action.CombatActionId === 100);
            rows = [!chainStates.some((state) => state.Signal === "lightning-chain-state") || !actions.length
                ? missing("dragon-chain", "闪电链开启期间龙炎占比", "%", "需要当前角色闪电链开启／结束及龙炎释放记录；支持队友的公开启停消息，旧日志可重新导入配套原始抓包。")
                : row("dragon-chain", "闪电链开启期间龙炎占比", dragons.length ? chained / dragons.length * 100 : null, "%",
                    `当前角色闪电链实际开启期间释放的龙炎次数 / 该角色龙炎总释放次数；${chained} 次 / ${dragons.length} 次。按实际启停判断，持续时间与冷却时间分别处理；排除首领无敌期间。`, undefined, dragons.length),
                cooldownWait("seal-wait", "魔法封锁平均等待时间（冷却结束后）", 59045, 30),
                legacyThunder ? missing("thunder", "龙炎间隔内平均雷击次数", "次/间隔", "此记录含旧版漏计雷击的释放参数，无法恢复完整次数；请重新导入配套原始抓包或用新版重新采集。")
                    : between("thunder", "龙炎间隔内平均雷击次数", 59040, 30102)];
            const minimum = input.dorchaMinimum;
            if (!(minimum !== undefined && Number.isFinite(minimum) && minimum > 0)) {
                rows.splice(1, 0, pending("piercing", "多尔卡不足：魔法穿刺无效率", "%", "待确认多尔卡不足的点数标准；需同时记录多尔卡数量和魔法穿刺状态。"));
                break;
            }
            const updates = (input.statUpdates ?? []).filter((event) => event.Id === player.id)
                .flatMap((event) => event.Stats.filter((stat) => stat.StatId === 196).map((stat) => ({ at: event.At, value: stat.Value })))
                .sort((a, b) => a.at - b.at);
            const low: TimeInterval[] = [], known: TimeInterval[] = [];
            for (let i = 0; i < updates.length; i++) {
                const interval = { start: Math.max(session.startAt, updates[i].at), end: Math.min(session.endAt, updates[i + 1]?.at ?? session.endAt) };
                if (interval.end <= interval.start) continue;
                if (!Number.isFinite(updates[i].value) || updates[i].value < 0 || updates[i].value > 15) continue;
                known.push(interval);
                if (updates[i].value < minimum) low.push(interval);
            }
            const observedSeconds = activeSeconds - duration(subtractIntervals(windows, known));
            if (observedSeconds < activeSeconds - 0.001 || !player.conditionHistory.length) {
                rows.splice(1, 0, missing("piercing", "多尔卡不足：魔法穿刺无效率", "%", teammate ? "未收到该队友完整的多尔卡数量记录，无法由伤害反推；本人资源不会用于代算队友。" : "缺少完整多尔卡数量或状态时间轴；旧日志的最终数量不能代表整场。"));
                break;
            }
            const lowActive = windows.flatMap((window) => low.map((interval) => ({ start: Math.max(window.start, interval.start), end: Math.min(window.end, interval.end) })).filter((interval) => interval.end > interval.start));
            const piercingOff = duration(subtractIntervals(lowActive, selfSlices([938])));
            rows.splice(1, 0, row("piercing", "多尔卡不足：魔法穿刺无效率", activeSeconds ? piercingOff / activeSeconds * 100 : null, "%",
                `多尔卡少于 ${minimum} 点且魔法穿刺未生效的时间 / 可输出时间；累计 ${piercingOff.toFixed(2)} 秒。`));
            break;
        }
        case "流星射手": {
            const teammate = Boolean(input.localEntityId && input.localEntityId !== player.id);
            const samples = (teammate ? [] : input.aimSamples ?? []).filter((sample) => sample.entityId === player.id
                && (!sample.targetId || sample.targetId === boss.id)
                && valid(sample.atMs / 1000) && Number.isFinite(sample.rate) && sample.rate >= 0 && sample.rate <= 1);
            const rotation = between("magnum", "爆炎箭间隔内穿心箭次数", 59060, 21002);
            if (teammate) {
                rotation.detail += "队友按公开的爆炎箭独立释放消息、穿心箭成功动作统计，多段伤害不重复计数。";
                if (!actions.some((action) => action.SkillId === 59060)
                    && input.actions.some((action) => action.Id === player.id && action.SkillId === 59060 && action.IsFallback)) {
                    rotation.status = "missing-data";
                    rotation.detail += "当前记录只有伤害回退消息，需用新版重新抓包或回放原始抓包来补齐释放记录。";
                }
            }
            rows = [row("aim", "穿心平均瞄准率（估算）", samples.length ? samples.reduce((sum, sample) => sum + sample.rate, 0) / samples.length * 100 : null, "%",
                teammate ? "队友的瞄准起点、射程及个人修正参数无法完整获取，无法可靠计算本项；不会使用本人的瞄准参数代算队友。"
                    : "请先在「提醒设置 → 瞄准提醒」填写并保存射程、装备及修正参数；无需勾选「启用瞄准提醒」，也会统计后续战斗。此项为本人的估算瞄准率，并非服务器直接提供的真实值，已排除无敌期间。", samples.length ? "measured" : "missing-data", samples.length),
                rotation];
            const hydroCasts = new Map<number, boolean | null>();
            let observedHydro = false;
            for (const signal of input.arcanaSignals ?? []) {
                if (signal.Signal !== "hydro-charge-sample" || signal.SkillId !== 59061 || signal.Kind !== 825
                    || signal.Id !== player.id || signal.TargetId !== boss.id || !signal.Complete
                    || !Number.isFinite(signal.CastAtMs) || signal.CastAtMs! <= 0
                    || !Number.isFinite(signal.AtMs) || signal.AtMs < signal.CastAtMs!
                    || signal.At !== Math.floor(signal.AtMs / 1000)
                    || !Number.isFinite(signal.Value) || signal.Value! <= 0 || signal.Value! > 180) continue;
                observedHydro = true;
                const start = Math.floor(signal.CastAtMs! / 1000), end = signal.At;
                if (!valid(start) || !valid(end) || overlap({ start, end }, windows) < end - start - 0.001) continue;
                const full = hydroChargeIsFull(signal.Value);
                // Conflicting feedback for the same release is unknown, not a
                // second shot or a reason to pick the more favourable result.
                hydroCasts.set(signal.CastAtMs!, hydroCasts.has(signal.CastAtMs!) && hydroCasts.get(signal.CastAtMs!) !== full ? null : full);
            }
            const fullHydro = [...hydroCasts.values()].filter((full) => full === true).length;
            const partialHydro = [...hydroCasts.values()].filter((full) => full === false).length;
            const unknownHydro = hydroCasts.size - fullHydro - partialHydro;
            const hydroSamples = fullHydro + partialHydro;
            rows.push(row("hydro-full", "水流箭蓄满率（单体强化）", hydroSamples ? fullHydro / hydroSamples * 100 : null, "%",
                `释放范围收窄至已确认的最小范围、进入单体强化时记为蓄满；蓄满 ${fullHydro} 次，未蓄满 ${partialHydro} 次。`
                    + (unknownHydro ? `另有 ${unknownHydro} 次反馈无法判定，不计入分母。` : "")
                    + "仅计当前首领可输出期间的完整释放，取消、截断及跨无敌过程不计；缺少反馈不当作未蓄满。"
                    + (teammate ? "队友使用公开的蓄力起点和实际释放范围判定。" : ""),
                hydroSamples ? "measured" : hydroCasts.size || !observedHydro ? "missing-data" : "no-samples", hydroSamples));
            break;
        }
        case "圣盾骑士": {
            const history = player.conditionHistory;
            let from: number | undefined, total = 0, samples = 0;
            for (const state of history) {
                const condition = state.List.find((item) => item.CCId === 1179);
                const energy = holyEnergyState(condition, state.At * 1000);
                if (!energy || state.At < session.startAt || state.At > session.endAt) { from = undefined; continue; }
                if (energy.Percent === 0 && from === undefined) from = state.At;
                if (energy.Percent >= 100 && from !== undefined) {
                    total += overlap({ start: from, end: state.At }, windows); samples++; from = undefined;
                }
            }
            rows = [row("sacrifice", "大招平均等待时间（牺牲值 0→100）", samples ? total / samples : null, "秒",
                "仅统计观察到完整 0→100 的蓄能周期；开场已蓄能及未攒满周期不计，无敌期间暂停。", undefined, samples), fury()];
            break;
        }
        case "爆裂骑士枪":
            rows = [withoutChain("lance", "无闪电链时骑士枪冲刺次数", [20017]),
                withoutChain("blast", "无闪电链时爆裂冲刺次数", [59105]),
                withoutChain("annihilation", "无闪电链时湮灭次数", [59106]),
                missing("heat", "湮灭时平均过热值", "", "待确认骑士枪过热数值的记录字段。")];
            break;
        case "枪炮师": {
            const domainRecords = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.SkillId === 59121 && signal.Signal === "domain-sample" && signal.TargetId === boss.id);
            const domainSamples = new Map<number, number>();
            for (const signal of domainRecords) {
                if (!signal.Complete || !valid(signal.At) || !Number.isFinite(signal.AtMs)
                    || !Number.isFinite(signal.CastAtMs) || signal.CastAtMs! <= 0 || signal.AtMs < signal.CastAtMs!
                    || !Number.isInteger(signal.Count) || signal.Count < 0 || signal.Count > 32
                    || (signal.ObjectIds ?? []).length !== signal.Count || new Set(signal.ObjectIds).size !== signal.Count) continue;
                domainSamples.set(signal.CastAtMs!, signal.Count);
            }
            const domainTotal = [...domainSamples.values()].reduce((sum, count) => sum + count, 0);
            const counters = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.SkillId === 59123 && signal.Signal === "sniper-counter"
                && signal.TargetId === boss.id && Number.isFinite(signal.CastAtMs) && signal.CastAtMs! > 0);
            const groups = new Map<number, eventArcanaSignal[]>();
            for (const signal of counters) {
                const group = groups.get(signal.CastAtMs!) ?? [];
                group.push(signal); groups.set(signal.CastAtMs!, group);
            }
            let shots = 0, samples = 0;
            for (const group of groups.values()) {
                group.sort((a, b) => a.AtMs - b.AtMs);
                const terminalIndex = group.findIndex((signal) => signal.Complete && signal.Phase === 7);
                if (terminalIndex < 0 || group[0].Count !== 0) continue;
                const complete = group.slice(0, terminalIndex + 1), terminal = complete.at(-1)!;
                let previous = 0, firstShotAt: number | undefined, eligible = valid(terminal.At);
                for (const signal of complete) {
                    if (!Number.isInteger(signal.Count) || signal.Count < previous || signal.Count > 100
                        || !Number.isFinite(signal.AtMs) || signal.AtMs < signal.CastAtMs!) { eligible = false; break; }
                    if (signal.Count > previous) {
                        // Status/condition histories use seconds. Keep the packet's
                        // integer observation time so the final hit's subsecond
                        // counter remains inside its last-damage second.
                        firstShotAt ??= signal.At;
                        if (!valid(signal.At)) eligible = false;
                    }
                    previous = signal.Count;
                }
                if (!eligible || firstShotAt === undefined || overlap({ start: firstShotAt, end: terminal.At }, windows) < terminal.At - firstShotAt - 0.001) continue;
                shots += terminal.Count; samples++;
            }
            rows = [domainRecords.length ? row("domain-count", "重炮炮火时的领域平均覆盖个数", domainSamples.size ? domainTotal / domainSamples.size : null, "个/次",
                    `按本角色对当前首领完整重炮炮火反馈的关联领域数量取平均，共 ${domainTotal} 个 / ${domainSamples.size} 次；只统计可输出期间的释放。未记录领域列表的释放不补记为 0。`, undefined, domainSamples.size)
                    : missing("domain-count", "重炮炮火时的领域平均覆盖个数", "个/次", "需完整重炮炮火的关联领域列表及目标记录；旧日志缺少此信号，可用原始抓包重新解码。"),
                counters.length ? row("sniper", "致命狙击平均发数", samples ? shots / samples : null, "发/次",
                    `按本角色完整狙击发射记录统计，共 ${shots} 发 / ${samples} 次；队友使用公开起手及逐发反馈。需从零开始、完整结束且目标为当前首领，无敌期间及不完整释放不计。`, undefined, samples)
                    : missing("sniper", "致命狙击平均发数", "发/次", "需要狙击累计发数记录；旧日志只有伤害段数，无法补算，可用原始抓包重新解码。")];
            break;
        }
        case "禁术炼金师": {
            const feedback = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.SkillId === 59144 && signal.TargetId === boss.id && signal.Signal === "chemical-sample");
            const samples = completeCastSamples(feedback, windows, session.endAt, 1, 100);
            const total = samples.reduce((sum, signal) => sum + signal.Count - 1, 0);
            rows = [feedback.length ? row("chemical", "化学平均追加次数", samples.length ? total / samples.length : null, "次/释放",
                `每次化学狂欢扣除 1 次基础攻击，共追加 ${total} 次 / ${samples.length} 次释放；无敌期间及不完整释放不计。`, undefined, samples.length)
                : missing("chemical", "化学平均追加次数", "次/释放", "需要化学狂欢完整释放及攻击计数；每次扣除 1 次基础攻击。旧日志缺少计数时可用原始抓包重新解码。")];
            break;
        }
        case "旋律操纵师": {
            const feedback = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.SkillId === 54105 && signal.TargetId === boss.id && signal.Signal === "act7-sample");
            const samples = completeCastSamples(feedback, windows, session.endAt, 0, 1)
                .filter((signal) => signal.Phase === 3 && signal.ObjectIds?.length === 1);
            const empty = samples.reduce((sum, signal) => sum + signal.Count, 0);
            const interlude = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.SkillId === 59165 && signal.TargetId === boss.id && signal.Signal === "interlude-sample");
            const puppetSamples = completeCastSamples(interlude, windows, session.endAt, 1, 100);
            const puppets = puppetSamples.reduce((sum, signal) => sum + signal.Count, 0);
            const releases = actions.filter((action) => action.SkillId === 59165);
            // Fight boundaries are stored as inclusive whole seconds, while
            // executions retain milliseconds. Preserve the final second's
            // releases; integrate only up to each actual release timestamp.
            const intervalWindows = kpiActiveIntervals(boss, { ...session, endAt: session.endAt + 1 });
            const validRelease = (at: number) => contains(at, intervalWindows, session.endAt + 1);
            let intervalSeconds = 0, intervalSamples = 0;
            for (let i = 1; i < releases.length; i++) {
                const start = actionTime(releases[i - 1]), end = actionTime(releases[i]);
                // The requested "wait" is the full release-to-release gap.
                // Keep adjacency before filtering so an immune-period release
                // cannot be skipped to form a longer, artificial interval.
                if (end <= start || !validRelease(start) || !validRelease(end)) continue;
                intervalSeconds += overlap({ start, end }, intervalWindows);
                intervalSamples++;
            }
            rows = [feedback.length ? row("act7", "第7幕空车率", samples.length ? empty / samples.length * 100 : null, "%",
                `完全未命中任何目标视为空车，共 ${empty} / ${samples.length} 次；命中其他目标也不算空车。无敌期间及不完整释放不计。`, undefined, samples.length)
                : missing("act7", "第7幕空车率", "%", teammate ? "队友第7幕缺少可确认的选定目标与完整人偶执行过程，不能仅凭没有伤害判断空车；间奏斩可单独统计。" : "需要本人人偶的完整执行、攻击尝试及结束记录；旧日志仅凭没有首领伤害无法判断空车，可用原始抓包重新解码。"),
                row("interlude-wait", "间奏斩平均等待时间", intervalSamples ? intervalSeconds / intervalSamples : null, "秒",
                    `相邻两次当前角色间奏斩释放之间的时间取平均，扣除首领无敌时间；共 ${intervalSamples} 个完整间隔，累计 ${intervalSeconds.toFixed(2)} 秒。首尾不完整间隔及无敌期间的释放不计。`,
                    actions.length ? undefined : "missing-data", intervalSamples),
                interlude.length ? row("puppets", "间奏斩平均人偶数量", puppetSamples.length ? puppets / puppetSamples.length : null, "个/次",
                    `每次间奏斩参与的人偶数量取平均，共 ${puppets} 个 / ${puppetSamples.length} 次；多目标伤害不会重复计算人偶数量，仅统计本场完整释放。`, undefined, puppetSamples.length)
                    : missing("puppets", "间奏斩平均人偶数量", "个/次", "需要完整间奏斩的参与人偶反馈及首领命中记录；旧日志可用原始抓包重新解码。")];
            break;
        }
        case "狂怒斗士": {
            const idle = fighterIdle(input.arcanaSignals ?? [], player.id, windows, session.endAt);
            rows = [idle.value !== null ? row("energy-idle", "斗气能量空置时间", idle.value, "秒",
                "斗气至少 100 点（满足最低消耗），却没有使用疾风突刺、愤怒践踏或烈拳三击的时间；自然增长计入，使用技能期间及首领无敌期间不计。")
                : idle.bounds ? { ...row("energy-idle", "斗气能量空置时间（推算）", idle.bounds.lower, "秒",
                    "按自然增长 3 点/秒、上限 400 点，结合实际增减及技能使用情况推算；首领无敌和消耗技能使用期间不计。", "estimated"),
                    upperValue: idle.bounds.upper }
                : missing("energy-idle", "斗气能量空置时间", "秒", teammate && !idle.deltas && !idle.baselines
                    ? "未收到该队友的斗气资源记录，无法可靠统计空置时间；逆龙连段可使用公开技能消息单独统计。"
                    : !idle.baselines
                    ? `${idle.deltas ? `已捕获 ${idle.deltas} 条斗气增减，` : ""}缺少斗气初始化（初始值、自然增长及上限）。请在开始记录后切换出狂怒斗士再切回；若原始抓包也缺少初始化，重新导入无法补回。`
                    : `斗气记录未连续覆盖本场：可确认 ${idle.observedSeconds.toFixed(2)} / ${idle.requiredSeconds.toFixed(2)} 秒（已扣除无敌及消耗技能使用时间）。换线或能量记录中断后，需要重新捕获初始化。`)];
            const feedback = (input.arcanaSignals ?? []).filter((signal) => signal.Id === player.id
                && signal.Signal === "fighter-combo" && signal.SkillId === 24201 && signal.TargetId === boss.id);
            const samples = new Map<number, eventArcanaSignal>();
            for (const signal of feedback) {
                const start = Math.floor(signal.CastAtMs! / 1000);
                if (!signal.Complete || !Number.isFinite(signal.CastAtMs) || signal.CastAtMs! <= 0
                    || !Number.isFinite(signal.AtMs) || signal.AtMs < signal.CastAtMs!
                    || signal.At !== Math.floor(signal.AtMs / 1000)
                    || !Number.isFinite(signal.ReadyAtMs) || signal.ReadyAtMs! < signal.CastAtMs! || signal.AtMs < signal.ReadyAtMs!
                    || !valid(start) || !valid(signal.At) || overlap({ start, end: signal.At }, windows) < signal.At - start - 0.001
                    || (signal.Count !== 0 && signal.Count !== 1)) continue;
                if (signal.Count === 1 && (!Number.isFinite(signal.ReverseAtMs) || signal.ReverseAtMs! < signal.CastAtMs!
                    || !Number.isFinite(signal.ReverseEndAtMs) || signal.ReverseEndAtMs! < signal.ReverseAtMs!
                    || signal.ReadyAtMs! < signal.ReverseEndAtMs!)) continue;
                samples.set(signal.CastAtMs!, signal);
            }
            const normal = [...samples.values()].filter((signal) => signal.Count === 0);
            const reversed = [...samples.values()].filter((signal) => signal.Count === 1);
            const meanSeconds = (group: eventArcanaSignal[]) => group.reduce((sum, signal) => sum + (signal.AtMs - signal.CastAtMs!) / 1000, 0) / group.length;
            const normalMean = normal.length ? meanSeconds(normal) : null;
            const reversedMean = reversed.length ? meanSeconds(reversed) : null;
            const saved = reversedMean !== null ? fighterNormalVideoBaselineSeconds - reversedMean : null;
            rows.push(feedback.length ? row("reverse-saved", "逆龙节省后摇平均时间（估算）", saved, "秒", saved !== null
                ? `正常升龙→飞踢基准约 ${fighterNormalVideoBaselineSeconds.toFixed(2)} 秒−本场逆龙平均 ${reversedMean!.toFixed(3)} 秒，共 ${reversed.length} 次。${normalMean !== null ? `本场正常连段平均 ${normalMean.toFixed(3)} 秒（${normal.length} 次），仅供对照。` : ""}仅统计当前角色在首领可输出期间的完整连段；间隔包含操作和网络时间。${teammate ? "队友以公开升龙动作、逆龙启停及飞踢起手确认，飞踢伤害到达时间不作为起手。" : ""}`
                : `正常动作基准约 ${fighterNormalVideoBaselineSeconds.toFixed(2)} 秒；本场暂无完整逆龙连段，无法计算节省时间。`, saved !== null ? "estimated" : "no-samples", reversed.length)
                : missing("reverse-saved", "逆龙节省后摇平均时间（估算）", "秒", `正常动作基准约 ${fighterNormalVideoBaselineSeconds.toFixed(2)} 秒；需要本场升龙→逆龙打断→飞踢的完整记录。`));
            break;
        }
    }
    if (activeSeconds <= 0) rows = rows.map((item) => ({ ...item, value: null,
        status: "no-samples", detail: "本场没有可统计的输出时间。" }));
    return { jobName: input.jobName, activeSeconds, excludedSeconds: Math.max(0, session.totalDuration - activeSeconds), rows };
}
