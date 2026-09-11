import type { ActorManager, EntityActor, EntityDamage } from "@/eventActor";
import type { CustomConditionConfig, ConditionFilter } from "@/summaryConfig";

// ===== 常數 =====

/** 無敵狀態的 condition ID（CCID 277=無敵, 494=無敵2） */
export const INVINCIBLE_CCIDS = new Set([277, 494]);

/** 橋接相鄰無敵區間的最大間隔秒數（補足封包遺失造成的斷點） */
export const INVINC_BRIDGE_GAP = 5;

/** 個人有效輸出時間判定：連續傷害間隔超過此秒數則視為中斷 */
export const ACTIVE_GAP = 5;

/**
 * 不計入暴擊率統計的技能（被動觸發傷害，本身不具備暴擊判定）
 * 58100=爆破, 58101=閃焰, 58104=籠罩, 58009=連續攻擊
 */
export const NO_CRIT_SKILL_IDS = new Set([58100, 58101, 58104, 58009]);

/** 傷害間隔超過此秒數視為中斷（輸出空窗） */
export const INACT_GAP = 3;
/** 活躍段命中次數低於此值視為無效（過場/僵直等） */
export const INACT_MIN_HITS = 3;

// ===== 職業判定 =====

type JobConfig = { name: string; skillIds: number[] };

function skillIdRange(start: number, end: number): number[] {
    return Array.from({ length: end - start + 1 }, (_, index) => start + index);
}

const JOB_CONFIGS: Record<string, JobConfig> = {
    harmonicSaint: {
        name: "圣光颂唱者",
        skillIds: skillIdRange(59000, 59019),
    },
    elementalKnight: {
        name: "元素骑士",
        skillIds: skillIdRange(59020, 59039),
    },
    darkMage: {
        name: "黑魔导士",
        skillIds: skillIdRange(59040, 59059),
    },
    alchemicStinger: {
        name: "流星射手",
        skillIds: skillIdRange(59060, 59079),
    },
    sacredGuardian: {
        name: "圣盾骑士",
        skillIds: skillIdRange(59080, 59099),
    },
    blastLancer: {
        name: "爆裂骑士枪",
        skillIds: skillIdRange(59100, 59119),
    },
    sacredGunner: {
        name: "枪炮师",
        skillIds: skillIdRange(59120, 59139),
    },
    forbiddenAlchemist: {
        name: "禁术炼金师",
        skillIds: skillIdRange(59140, 59159),
    },
    melodyMaestro: {
        name: "旋律操纵师",
        skillIds: skillIdRange(59160, 59179),
    },
    furiousFighter: {
        name: "狂怒斗士",
        skillIds: skillIdRange(59180, 59199),
    },
};

/** 依使用過的技能 ID 推測職業，回傳顯示名稱；無法判斷時回傳 null */
export function detectJob(usedSkillIds: Set<number>): string | null {
    let bestName: string | null = null;
    let bestScore = 0;
    for (const config of Object.values(JOB_CONFIGS)) {
        const score = config.skillIds.filter((id) => usedSkillIds.has(id)).length;
        if (score > bestScore) {
            bestScore = score;
            bestName = config.name;
        }
    }
    return bestScore > 0 ? bestName : null;
}

/** 武器相關 PocketType（待確認完整對應關係） */
export const WEAPON_POCKET_TYPES = new Set([10, 11, 12, 13, 14]);

export type WeaponInfo = { pocketType: number; itemId: number };

// ===== 基本型別 =====

export type TimeInterval = { start: number; end: number };

/**
 * Returns the selected Boss and only entities whose explicit owner chain
 * resolves to that Boss. Race/group membership is intentionally ignored:
 * unrelated monsters can share a race/group with a Boss or one of its parts.
 */
export function collectBossTargetEntityIds(
    bossEntityId: string,
    actorManager: ActorManager,
): Set<string> {
    const targetIds = new Set<string>([bossEntityId]);
    let changed = true;

    while (changed) {
        changed = false;
        for (const entity of Object.values(actorManager.entityMap) as EntityActor[]) {
            if (
                entity.isPC ||
                targetIds.has(entity.id) ||
                !entity.ownerId ||
                !targetIds.has(entity.ownerId)
            ) {
                continue;
            }
            targetIds.add(entity.id);
            changed = true;
        }
    }

    return targetIds;
}

/** 戰鬥 Session 基本資訊 */
export type BossFightSession = {
    bossEntityId: string;
    /** 第一次受到傷害的時間 */
    startAt: number;
    /** 最後一次受到傷害的時間 */
    endAt: number;
    /** 總戰鬥時間（秒） */
    totalDuration: number;
    /** 無敵區間列表 */
    invincibleIntervals: TimeInterval[];
    /** 有效輸出時間 = totalDuration - 無敵期間合計 */
    effectiveDuration: number;
    /** 無法輸出時間 = 無敵期間合計 */
    inactiveDuration: number;
};

/** 單一技能統計 */
export type SkillStat = {
    skillId: number;
    /** 使用次數：每條有效傷害記錄計一次，不做時間合併 */
    useCount: number;
    /** 每分鐘使用次數（基於個人有效輸出時間） */
    usesPerMinute: number;
    totalDamage: number;
    critHits: number;
    totalHits: number;
    /** 每次使用的平均命中段數（totalHits / useCount） */
    hitsPerUse: number;
    /** 每次命中的平均傷害（totalDamage / totalHits） */
    damagePerUse: number;
    critRate: number;
    /** true = 此技能不具備暴擊判定，個別暴擊率顯示為 — */
    noCritRate: boolean;
    /** 單次命中最大暴擊傷害；無暴擊紀錄時為 null */
    maxCritDamage: number | null;
    /** 單次命中最小暴擊傷害；無暴擊紀錄時為 null */
    minCritDamage: number | null;
    /** 單次命中最大非暴擊傷害；無非暴擊紀錄時為 null */
    maxNonCritDamage: number | null;
    /** 單次命中最小非暴擊傷害；無非暴擊紀錄時為 null */
    minNonCritDamage: number | null;
    /** 被動技能的觸發來源分布（依傷害降序）；主動技能為空陣列 */
    triggerSources: TriggerSource[];
};

/** 被動技能的觸發來源 */
export type TriggerSource = {
    skillId: number;
    damage: number;
    count: number;
};

/** 單一玩家 Summary */
export type PlayerSummary = {
    entityId: string;
    name: string;
    raceId: number;
    totalDamage: number;
    /** 全場 DPS = totalDamage / session.totalDuration */
    totalDPS: number;
    /** 有效 DPS = totalDamage / session.effectiveDuration */
    effectiveDPS: number;
    /** 個人有效輸出時間（5秒內有打才算） */
    individualEffectiveTime: number;
    /** 個人有效 DPS = totalDamage / individualEffectiveTime */
    individualEffectiveDPS: number;
    critRate: number;
    /** 被動技能傷害（閃焰/爆破/籠罩/連續攻擊）占個人總傷害比例 */
    passiveDamageRatio: number;
    /** 主動技能傷害占個人總傷害比例 */
    activeDamageRatio: number;
    skillStats: SkillStat[];
    /** 推測職業名稱，無法判斷時為 null */
    jobName: string | null;
    /** 武器裝備（PocketType 10=主手, 11=副手） */
    weapons: WeaponInfo[];
};

/** 技能使用紀錄（group 內的技能序列元素） */
export type SkillUse = {
    at: number;
    skillId: number;
    totalDamage: number;
    isCrit: boolean;
    /** 被動技能（NO_CRIT_SKILL_IDS）的觸發來源技能 ID */
    triggerSkillId?: number;
};

/** group 內單一玩家的資料 */
export type ConditionGroupPlayer = {
    entityId: string;
    name: string;
    totalDamage: number;
    /** 被動技能（NO_CRIT_SKILL_IDS）造成的傷害 */
    passiveDamage: number;
    /** 可計算暴擊的技能使用次數（排除被動技能） */
    critEligible: number;
    /** 其中暴擊次數 */
    critHits: number;
    skillSequence: SkillUse[];
};

/** 自訂條件的一段區間（group） */
export type ConditionGroup = {
    index: number;
    startAt: number;
    endAt: number;
    duration: number;
    /** 全隊合計傷害 */
    totalDamage: number;
    /** 全隊合計技能序列（跨玩家，依時間排序） */
    skillSequence: SkillUse[];
    /** 各玩家個別資料 */
    players: ConditionGroupPlayer[];
};

/** 自訂條件完整結果 */
export type CustomConditionResult = {
    configId: string;
    displayName: string;
    groups: ConditionGroup[];
    totalDamage: number;
    averageDamage: number;
};

/** Boss 使用技能的統計 */
export type BossSkillStat = {
    skillId: number;
    useCount: number;
    totalHits: number;
    totalDamage: number;
};

/** 整體 Summary 資料（傳給 UI） */
export type BossSummary = {
    session: BossFightSession;
    players: PlayerSummary[];
    customConditions: CustomConditionResult[];
    totalDamage: number;
    /** Actual selected Boss-body health removed (includes unattributed/system damage). */
    effectiveBossDamage: number;
    bossSkillStats: BossSkillStat[];
};

// ===== 無敵 / 條件區間工具 =====

/**
 * 從 entity 的 conditionHistory 建構指定 conditionId 的存在區間列表
 * （若 conditionIds 為 Set 則只要有任一個即視為進入）
 */
export function buildConditionIntervals(
    entity: EntityActor,
    conditionIds: number | Set<number>,
): TimeInterval[] {
    const ids =
        typeof conditionIds === "number"
            ? new Set([conditionIds])
            : conditionIds;

    const intervals: TimeInterval[] = [];
    let start: number | null = null;

    for (const state of entity.conditionHistory) {
        const has = state.List.some((c) => ids.has(c.CCId));
        if (has && start === null) {
            start = state.At;
        } else if (!has && start !== null) {
            intervals.push({ start, end: state.At });
            start = null;
        }
    }
    if (start !== null) {
        intervals.push({ start, end: Infinity });
    }
    return intervals;
}

/**
 * 橋接間隔 ≤ bridgeGap 秒的相鄰區間（補封包遺失斷點）
 */
export function bridgeIntervals(
    intervals: TimeInterval[],
    bridgeGap: number,
): TimeInterval[] {
    if (intervals.length < 2 || bridgeGap <= 0) return intervals;
    const merged = [{ ...intervals[0] }];
    for (let i = 1; i < intervals.length; i++) {
        const last = merged[merged.length - 1];
        const curr = intervals[i];
        if (curr.start - last.end <= bridgeGap) {
            last.end = Math.max(last.end, curr.end);
        } else {
            merged.push({ ...curr });
        }
    }
    return merged;
}

/** 判斷某時間點是否落在任一區間內 */
/** 合併重疊/相鄰的時間區間（需先排序過） */
function mergeIntervals(intervals: TimeInterval[]): TimeInterval[] {
    if (intervals.length === 0) return [];
    const sorted = [...intervals].sort((a, b) => a.start - b.start);
    const merged = [{ ...sorted[0] }];
    for (let i = 1; i < sorted.length; i++) {
        const last = merged[merged.length - 1];
        if (sorted[i].start <= last.end) {
            last.end = Math.max(last.end, sorted[i].end);
        } else {
            merged.push({ ...sorted[i] });
        }
    }
    return merged;
}

/**
 * 從傷害序列推算輸出空窗（間隔 > gapThreshold 或命中不足 minHits 的段）
 */
function buildDamageGapIntervals(
    damages: readonly { At: number }[],
    gapThreshold: number,
    minHits: number,
): TimeInterval[] {
    if (damages.length < minHits) return [];

    const sorted = [...damages].sort((a, b) => a.At - b.At);
    const inactive: TimeInterval[] = [];

    let segStart = sorted[0].At;
    let segEnd   = sorted[0].At;
    let segHits  = 1;

    const flushSeg = (start: number, end: number, hits: number) => {
        if (hits < minHits) inactive.push({ start, end });
    };

    for (let i = 1; i < sorted.length; i++) {
        const gap = sorted[i].At - sorted[i - 1].At;
        if (gap <= gapThreshold) {
            segEnd = sorted[i].At;
            segHits++;
        } else {
            flushSeg(segStart, segEnd, segHits);
            // 間隔本身視為空窗
            inactive.push({ start: segEnd, end: sorted[i].At });
            segStart = sorted[i].At;
            segEnd   = sorted[i].At;
            segHits  = 1;
        }
    }
    flushSeg(segStart, segEnd, segHits);

    return mergeIntervals(inactive);
}

export function isInAnyInterval(
    at: number,
    intervals: TimeInterval[],
): boolean {
    return intervals.some((iv) => at >= iv.start && at <= iv.end);
}

/** 計算區間列表在 [capStart, capEnd] 範圍內的總秒數 */
export function sumIntervalDuration(
    intervals: TimeInterval[],
    capStart: number,
    capEnd: number,
): number {
    return intervals.reduce((sum, iv) => {
        const start = Math.max(iv.start, capStart);
        const end   = Math.min(iv.end === Infinity ? capEnd : iv.end, capEnd);
        return sum + Math.max(0, end - start);
    }, 0);
}

// ===== 條件篩選評估 =====

/**
 * 遍歷 filter tree，收集所有 duringCondition_* 葉節點的 conditionId
 * 用來預先計算需要的區間
 */
function collectDuringConditionIds(filter: ConditionFilter): {
    targetIds: Set<number>;
    attackerIds: Set<number>;
} {
    const targetIds = new Set<number>();
    const attackerIds = new Set<number>();

    function walk(f: ConditionFilter) {
        if (f.type === "leaf") {
            if (f.mode === "duringCondition_target")
                targetIds.add(f.conditionId);
            if (f.mode === "duringCondition_attacker")
                attackerIds.add(f.conditionId);
        } else {
            f.rules.forEach(walk);
        }
    }
    walk(filter);
    return { targetIds, attackerIds };
}

/**
 * 評估一筆傷害紀錄是否符合 filter
 * @param targetIntervalMap  key = conditionId, value = 目標 entity 的條件區間
 * @param attackerIntervalMap key = conditionId, value = 攻擊者 entity 的條件區間
 */
function evaluateFilter(
    damage: EntityDamage,
    filter: ConditionFilter,
    targetIntervalMap: Map<number, TimeInterval[]>,
    attackerIntervalMap: Map<number, TimeInterval[]>,
): boolean {
    if (filter.type === "leaf") {
        switch (filter.mode) {
            case "onHit_targetHas":
                return damage.TargetConditions.some(
                    (c) => c.CCId === filter.conditionId,
                );
            case "onHit_attackerHas":
                return damage.Conditions.some(
                    (c) => c.CCId === filter.conditionId,
                );
            case "duringCondition_target": {
                const ivs = targetIntervalMap.get(filter.conditionId) ?? [];
                return isInAnyInterval(damage.At, ivs);
            }
            case "duringCondition_attacker": {
                const ivs = attackerIntervalMap.get(filter.conditionId) ?? [];
                return isInAnyInterval(damage.At, ivs);
            }
        }
    }
    if (filter.type === "and") {
        return filter.rules.every((r) =>
            evaluateFilter(damage, r, targetIntervalMap, attackerIntervalMap),
        );
    }
    // "or"
    return filter.rules.some((r) =>
        evaluateFilter(damage, r, targetIntervalMap, attackerIntervalMap),
    );
}

/**
 * 從 filter tree 找出第一個 duringCondition_* 葉節點
 * 用來決定 eachEntry 分組依據
 */
function findPrimaryDuringLeaf(filter: ConditionFilter): {
    mode: "duringCondition_target" | "duringCondition_attacker";
    conditionId: number;
} | null {
    if (filter.type === "leaf") {
        if (
            filter.mode === "duringCondition_target" ||
            filter.mode === "duringCondition_attacker"
        ) {
            return { mode: filter.mode, conditionId: filter.conditionId };
        }
        return null;
    }
    for (const rule of filter.rules) {
        const found = findPrimaryDuringLeaf(rule);
        if (found) return found;
    }
    return null;
}

// ===== 技能序列建構 =====

/**
 * 被動技能相對於觸發技能單次命中傷害的預期比例範圍
 * 閃焰：40~52%，爆破：45~58.5%，連續攻擊：200~230%（單一 packet）
 */
const PASSIVE_DAMAGE_RATIO_RANGES = new Map<number, [number, number]>([
    [58101, [0.40, 0.52]],   // 閃焰
    [58100, [0.45, 0.585]],  // 爆破
    [58009, [2.00, 2.30]],   // 連續攻擊（單次 packet）
]);

/**
 * 有比例範圍定義的被動技能使用較長的追溯視窗（秒）
 * 比例驗證本身可防止誤判，因此可安全延長以涵蓋觸發間隔較大的情況
 */
const TRIGGER_WINDOW_RATIO = 3.0;

/**
 * 不須顯示觸發來源的被動技能（反彈傷害等，來源非主動技能）
 * 58104=籠罩（反彈）
 */
const NO_TRIGGER_SOURCE_IDS = new Set([58104]);

/**
 * 在排序後的傷害序列中，往前找索引 i 的被動技能命中的觸發來源
 * 優先選擇傷害比例在預期範圍內的主動技能；若無符合者，退回時間最近的主動技能
 * 有比例範圍的技能使用較長視窗（TRIGGER_WINDOW_RATIO）
 */
function findTriggerSource(sorted: EntityDamage[], i: number): EntityDamage | undefined {
    const d = sorted[i];
    if (NO_TRIGGER_SOURCE_IDS.has(d.SkillId)) return undefined;
    const ratioRange = PASSIVE_DAMAGE_RATIO_RANGES.get(d.SkillId);
    const window = ratioRange ? TRIGGER_WINDOW_RATIO : TRIGGER_WINDOW;
    let fallback: EntityDamage | undefined;

    for (let j = i - 1; j >= 0; j--) {
        const prev = sorted[j];
        if (d.At - prev.At > window) break;
        if (NO_CRIT_SKILL_IDS.has(prev.SkillId)) continue;

        if (ratioRange) {
            const ratio = prev.Damage > 0 ? d.Damage / prev.Damage : -1;
            if (ratio >= ratioRange[0] && ratio <= ratioRange[1]) {
                return prev;            // 比例完全符合，優先採用
            }
            if (!fallback) fallback = prev;  // 備選：時間最近的主動技能
        } else {
            return prev;               // 無比例限制，直接取最近的
        }
    }
    return fallback;
}

/**
 * 將傷害列表整理成技能使用序列（每筆傷害獨立一列，被動技能標示觸發來源）
 */
function buildSkillSequence(damages: EntityDamage[]): SkillUse[] {
    if (damages.length === 0) return [];

    const sorted = [...damages].sort((a, b) => a.At - b.At);

    return sorted.map((d, i) => {
        let triggerSkillId: number | undefined;
        if (NO_CRIT_SKILL_IDS.has(d.SkillId)) {
            triggerSkillId = findTriggerSource(sorted, i)?.SkillId;
        }
        return {
            at: d.At,
            skillId: d.SkillId,
            totalDamage: d.Damage,
            isCrit: d.IsCritical,
            triggerSkillId,
        };
    });
}

// ===== 個人有效輸出時間 =====

/**
 * 計算個人有效輸出時間：
 * 連續傷害事件間隔 ≤ ACTIVE_GAP 秒視為同一段 active period
 */
function buildIndividualEffectiveTime(damages: EntityDamage[]): number {
    if (damages.length === 0) return 0;

    const times = damages.map((d) => d.At).sort((a, b) => a - b);

    let totalActive = 0;
    let segStart = times[0];
    let segEnd = times[0];

    for (let i = 1; i < times.length; i++) {
        if (times[i] - segEnd <= ACTIVE_GAP) {
            segEnd = times[i];
        } else {
            totalActive += segEnd - segStart;
            segStart = times[i];
            segEnd = times[i];
        }
    }
    // 最後一段（單一時間點算 0，避免除以 0）
    totalActive += segEnd - segStart;

    return totalActive;
}

/**
 * 計算有效輸出的時間段陣列（供甘特圖使用）
 * 邏輯與 buildIndividualEffectiveTime 相同，但回傳段落陣列
 */
export function buildEffectiveSegments(damages: EntityDamage[]): { start: number; end: number }[] {
    if (damages.length === 0) return [];
    const times = damages.map((d) => d.At).sort((a, b) => a - b);
    const segs: { start: number; end: number }[] = [];
    let segStart = times[0], segEnd = times[0];
    for (let i = 1; i < times.length; i++) {
        if (times[i] - segEnd <= ACTIVE_GAP) {
            segEnd = times[i];
        } else {
            segs.push({ start: segStart, end: segEnd });
            segStart = times[i];
            segEnd = times[i];
        }
    }
    segs.push({ start: segStart, end: segEnd });
    return segs;
}

// ===== 技能統計 =====

/** 被動 proc 往前追溯的最大時間視窗（秒） */
const TRIGGER_WINDOW = 1.5;

function buildSkillStats(damages: EntityDamage[], individualEffectiveTime: number): SkillStat[] {
    const map = new Map<
        number,
        { totalDamage: number; critHits: number; totalHits: number;
          maxCrit: number; minCrit: number; maxNonCrit: number; minNonCrit: number; }
    >();

    for (const d of damages) {
        let s = map.get(d.SkillId);
        if (!s) {
            s = { totalDamage: 0, critHits: 0, totalHits: 0,
                  maxCrit: 0, minCrit: Infinity, maxNonCrit: 0, minNonCrit: Infinity };
            map.set(d.SkillId, s);
        }
        s.totalDamage += d.Damage;
        s.totalHits++;
        if (d.IsCritical) {
            s.critHits++;
            if (d.Damage > s.maxCrit) s.maxCrit = d.Damage;
            if (d.Damage < s.minCrit) s.minCrit = d.Damage;
        } else {
            if (d.Damage > s.maxNonCrit) s.maxNonCrit = d.Damage;
            if (d.Damage < s.minNonCrit) s.minNonCrit = d.Damage;
        }
    }

    // ── 觸發來源推斷 ────────────────────────────────────────────
    // sorted by time for backward-search
    const sorted = [...damages].sort((a, b) => a.At - b.At);
    // passiveSkillId → triggerSkillId → { damage, count }
    const triggerMap = new Map<number, Map<number, { damage: number; count: number }>>();

    for (let i = 0; i < sorted.length; i++) {
        const d = sorted[i];
        if (!NO_CRIT_SKILL_IDS.has(d.SkillId)) continue;

        const trigger = findTriggerSource(sorted, i);
        if (!trigger) continue;

        if (!triggerMap.has(d.SkillId)) triggerMap.set(d.SkillId, new Map());
        const src = triggerMap.get(d.SkillId)!;
        const cur = src.get(trigger.SkillId) ?? { damage: 0, count: 0 };
        src.set(trigger.SkillId, { damage: cur.damage + d.Damage, count: cur.count + 1 });
    }

    const effectiveMinutes = individualEffectiveTime / 60;

    return [...map.entries()].map(([skillId, s]) => {
        const useCount = s.totalHits;
        const noCrit = NO_CRIT_SKILL_IDS.has(skillId);

        // 整理觸發來源（依傷害降序）
        const triggerSources: TriggerSource[] = noCrit
            ? [...(triggerMap.get(skillId) ?? [])].map(([tid, v]) => ({
                skillId: tid, damage: v.damage, count: v.count,
              })).sort((a, b) => b.damage - a.damage)
            : [];

        return {
            skillId,
            useCount,
            usesPerMinute: effectiveMinutes > 0 ? s.totalHits / effectiveMinutes : 0,
            totalDamage: s.totalDamage,
            critHits: s.critHits,
            totalHits: s.totalHits,
            hitsPerUse: useCount > 0 ? s.totalHits / useCount : 0,
            damagePerUse: s.totalHits > 0 ? s.totalDamage / s.totalHits : 0,
            critRate: (!noCrit && s.totalHits > 0) ? s.critHits / s.totalHits : 0,
            noCritRate: noCrit,
            maxCritDamage:    s.critHits > 0                     ? s.maxCrit    : null,
            minCritDamage:    s.critHits > 0                     ? s.minCrit    : null,
            maxNonCritDamage: (s.totalHits - s.critHits) > 0     ? s.maxNonCrit : null,
            minNonCritDamage: (s.totalHits - s.critHits) > 0     ? s.minNonCrit : null,
            triggerSources,
        };
    });
}

// ===== 主要計算函式 =====

/**
 * 建構戰鬥 Session 基本資訊
 * @param bossEntity 目標 Boss entity
 */
export function buildBossFightSession(
    bossEntity: EntityActor,
    actorManager?: ActorManager,
    prepared?: {
        effectiveDamages?: readonly Pick<EntityDamage, "At" | "Damage" | "TargetId">[];
        healthLosses?: readonly { Id: string; At: number; Damage: number }[];
    },
): BossFightSession | null {
    // A report target is one health bar. Owned fragments/deployables are
    // independent health pools and must never inflate this Boss's DPS.
    const effectiveFallback = (prepared?.effectiveDamages ?? actorManager?.effectiveDamages ?? []).filter(
        (damage) => damage.Damage > 0 && damage.TargetId === bossEntity.id,
    ) ?? [];
    const rawDamages = bossEntity.takeDamages.filter(
        (damage) => damage.Damage > 0 && damage.TargetId === bossEntity.id,
    );
    // DPS measures the damage players produced. Hits during a phase transition
    // or immediately after the finishing blow can be valid output even when
    // Stat28 does not move, so the raw damage timeline is authoritative here.
    // Reconciled damage remains available for actual Boss-health accounting.
    const damages = rawDamages.length > 0
        ? rawDamages
        : effectiveFallback;
    const healthLosses = (prepared?.healthLosses ?? actorManager?.healthLosses ?? []).filter(
        (loss) => loss.Id === bossEntity.id && loss.Damage > 0,
    );
    if (damages.length === 0 && healthLosses.length === 0) return null;

    // Fall back to health-loss timestamps only for legacy records that contain
    // no usable raw/reconciled damage packets.
    const timelineEvents = damages.length > 0 ? damages : healthLosses;
    const startAt = timelineEvents.reduce((m, d) => Math.min(m, d.At), Infinity);
    const endAt = timelineEvents.reduce((m, d) => Math.max(m, d.At), -Infinity);
    const totalDuration = endAt - startAt;

    // 無敵條件（CCID 277/494）
    const rawInvincIntervals = buildConditionIntervals(bossEntity, INVINCIBLE_CCIDS);
    const condInvincIntervals = bridgeIntervals(rawInvincIntervals, INVINC_BRIDGE_GAP);

    // 傷害間隔推算空窗（間隔 > 3s 或命中 < 3 的段）
    const damageGapIntervals = buildDamageGapIntervals(damages, INACT_GAP, INACT_MIN_HITS);

    // 聯集：兩種條件合併
    const invincibleIntervals = mergeIntervals([...condInvincIntervals, ...damageGapIntervals]);

    const inactiveDuration = sumIntervalDuration(invincibleIntervals, startAt, endAt);
    const effectiveDuration = Math.max(0, totalDuration - inactiveDuration);

    return {
        bossEntityId: bossEntity.id,
        startAt,
        endAt,
        totalDuration,
        invincibleIntervals,
        effectiveDuration,
        inactiveDuration,
    };
}

/**
 * 建構單一玩家的 Summary
 * @param playerEntity 玩家 entity
 * @param bossEntityId Boss 的 entity ID
 * @param session      戰鬥 session 資訊
 */
export function buildPlayerSummary(
    playerEntity: EntityActor,
    bossEntityId: string,
    session: BossFightSession,
    bossTargetEntityIds: ReadonlySet<string> = new Set([bossEntityId]),
): PlayerSummary {
    // 取出此玩家對此 Boss 的所有原始傷害（含寵物，已在 onApplyDamage
    // 重映射到玩家 ID）。機制轉場與擊殺溢傷也是玩家實際輸出能力的一部分。
    const damages = playerEntity.applyDamages.filter(
        (d) =>
            bossTargetEntityIds.has(d.TargetId) &&
            d.Damage > 0 &&
            d.At >= session.startAt &&
            d.At <= session.endAt,
    );

    const totalDamage = damages.reduce((s, d) => s + d.Damage, 0);
    const passiveDamage = damages
        .filter((d) => NO_CRIT_SKILL_IDS.has(d.SkillId))
        .reduce((s, d) => s + d.Damage, 0);
    // 暴擊率排除不具暴擊判定的技能
    const critEligible = damages.filter((d) => !NO_CRIT_SKILL_IDS.has(d.SkillId));
    const totalHits = critEligible.length;
    const critHits = critEligible.filter((d) => d.IsCritical).length;

    const individualEffectiveTime = buildIndividualEffectiveTime(damages);

    const usedSkillIds = new Set(damages.map((d) => d.SkillId));
    const jobName = detectJob(usedSkillIds);

    const weapons: WeaponInfo[] = Object.values(playerEntity.equipItemMap)
        .filter((item) => WEAPON_POCKET_TYPES.has(item.PocketType))
        .sort((a, b) => a.PocketType - b.PocketType)
        .map((item) => ({ pocketType: item.PocketType, itemId: item.ItemId }));

    return {
        entityId: playerEntity.id,
        name: playerEntity.name,
        raceId: playerEntity.raceId,
        totalDamage,
        totalDPS: session.totalDuration > 0 ? totalDamage / session.totalDuration : 0,
        effectiveDPS: session.effectiveDuration > 0 ? totalDamage / session.effectiveDuration : 0,
        individualEffectiveTime,
        individualEffectiveDPS:
            individualEffectiveTime > 0
                ? totalDamage / individualEffectiveTime
                : 0,
        critRate: totalHits > 0 ? critHits / totalHits : 0,
        passiveDamageRatio: totalDamage > 0 ? passiveDamage / totalDamage : 0,
        activeDamageRatio: totalDamage > 0 ? (totalDamage - passiveDamage) / totalDamage : 0,
        skillStats: buildSkillStats(damages, individualEffectiveTime),
        jobName,
        weapons,
    };
}

/**
 * 建構自訂條件的結果（group 列表）
 * @param config        自訂條件設定
 * @param playerEntity  攻擊者（攻擊者的 conditionHistory 用於 onHit_attackerHas / duringCondition_attacker）
 * @param bossEntity    目標 Boss（目標的 conditionHistory 用於 duringCondition_target）
 * @param session       戰鬥 session 資訊
 */
export function buildCustomConditionResult(
    config: CustomConditionConfig,
    playerEntity: EntityActor,
    bossEntity: EntityActor,
    session: BossFightSession,
    bossTargetEntityIds: ReadonlySet<string> = new Set([bossEntity.id]),
): CustomConditionResult {
    const filter = config.filter;

    // 預先計算 duringCondition 需要的區間
    const { targetIds, attackerIds } = collectDuringConditionIds(filter);

    const targetIntervalMap = new Map<number, TimeInterval[]>();
    for (const id of targetIds) {
        targetIntervalMap.set(id, buildConditionIntervals(bossEntity, id));
    }

    const attackerIntervalMap = new Map<number, TimeInterval[]>();
    for (const id of attackerIds) {
        attackerIntervalMap.set(
            id,
            buildConditionIntervals(playerEntity, id),
        );
    }

    // 篩選符合條件的傷害列表
    const allDamages = playerEntity.applyDamages.filter(
        (d) =>
            bossTargetEntityIds.has(d.TargetId) &&
            d.Damage > 0 &&
            d.At >= session.startAt &&
            d.At <= session.endAt,
    );

    const matchedDamages = allDamages.filter((d) =>
        evaluateFilter(d, filter, targetIntervalMap, attackerIntervalMap),
    );

    // ── 分組 ──────────────────────────────────────────────
    let groups: ConditionGroup[];

    if (config.groupBy === "eachEntry") {
        // 找出可作為分組依據的 duringCondition leaf
        const primaryLeaf = findPrimaryDuringLeaf(filter);

        if (primaryLeaf) {
            // 以 condition 的進出區間分組
            const pivotMap =
                primaryLeaf.mode === "duringCondition_target"
                    ? targetIntervalMap
                    : attackerIntervalMap;
            const intervals = pivotMap.get(primaryLeaf.conditionId) ?? [];

            groups = intervals.map((iv, i) => {
                const ivEnd =
                    iv.end === Infinity ? session.endAt : iv.end;
                const groupDamages = matchedDamages.filter(
                    (d) => d.At >= iv.start && d.At <= ivEnd,
                );
                const totalDamage = groupDamages.reduce(
                    (s, d) => s + d.Damage,
                    0,
                );
                return {
                    index: i + 1,
                    startAt: iv.start,
                    endAt: ivEnd,
                    duration: ivEnd - iv.start,
                    totalDamage,
                    skillSequence: buildSkillSequence(groupDamages),
                    players: [],
                };
            });
        } else {
            // 無 duringCondition leaf：以連續命中叢集分組
            groups = buildHitClusters(matchedDamages, session);
        }
    } else {
        // groupBy === "none"
        const totalDamage = matchedDamages.reduce((s, d) => s + d.Damage, 0);
        groups =
            matchedDamages.length > 0
                ? [
                      {
                          index: 1,
                          startAt: matchedDamages[0].At,
                          endAt: matchedDamages[matchedDamages.length - 1].At,
                          duration:
                              matchedDamages[matchedDamages.length - 1].At -
                              matchedDamages[0].At,
                          totalDamage,
                          skillSequence: buildSkillSequence(matchedDamages),
                          players: [],
                      },
                  ]
                : [];
    }

    const totalDamage = groups.reduce((s, g) => s + g.totalDamage, 0);

    return {
        configId: config.id,
        displayName: config.displayName,
        groups,
        totalDamage,
        averageDamage: groups.length > 0 ? totalDamage / groups.length : 0,
    };
}

/**
 * 當 filter 無 duringCondition leaf 時，以連續命中叢集（ACTIVE_GAP）切分 group
 */
function buildHitClusters(
    damages: EntityDamage[],
    session: BossFightSession,
): ConditionGroup[] {
    if (damages.length === 0) return [];

    const sorted = [...damages].sort((a, b) => a.At - b.At);
    const clusters: EntityDamage[][] = [];
    let cluster: EntityDamage[] = [sorted[0]];

    for (let i = 1; i < sorted.length; i++) {
        if (sorted[i].At - sorted[i - 1].At <= ACTIVE_GAP) {
            cluster.push(sorted[i]);
        } else {
            clusters.push(cluster);
            cluster = [sorted[i]];
        }
    }
    clusters.push(cluster);

    return clusters.map((c, i) => {
        const totalDamage = c.reduce((s, d) => s + d.Damage, 0);
        return {
            index: i + 1,
            startAt: c[0].At,
            endAt: c[c.length - 1].At,
            duration: c[c.length - 1].At - c[0].At,
            totalDamage,
            skillSequence: buildSkillSequence(c),
            players: [],
        };
    });
}

// ===== 入口函式 =====

/**
 * 計算指定 Boss entity 的完整 Summary
 * @param bossEntityId   Boss 的 entity ID
 * @param actorManager   ActorManager 實例
 * @param customConfigs  自訂條件設定列表
 */
export function buildBossSummary(
    bossEntityId: string,
    actorManager: ActorManager,
    customConfigs: CustomConditionConfig[],
    timeRange?: { startAt: number; endAt: number },
): BossSummary | null {
    const bossEntity = actorManager.entityMap[bossEntityId] as EntityActor | undefined;
    if (!bossEntity) return null;

    const bossTargetEntityIds = new Set([bossEntityId]);
    const effectiveEvents = actorManager.effectiveDamages ?? [];
    const healthLosses = actorManager.healthLosses ?? [];
    // Select the target's timelines once. Previously the global effective-
    // damage and health-loss histories were scanned again inside the session
    // builder and then once more for every player on every reactive update.
    const bossEffectiveEvents = effectiveEvents.filter(
        (damage) => damage.TargetId === bossEntityId && damage.Damage > 0,
    );
    const bossHealthLosses = healthLosses.filter(
        (loss) => loss.Id === bossEntityId && loss.Damage > 0,
    );
    const baseSession = buildBossFightSession(bossEntity, actorManager, {
        effectiveDamages: bossEffectiveEvents,
        healthLosses: bossHealthLosses,
    });
    if (!baseSession) return null;

    // 若有時間段選取，將 session 裁切到該範圍
    let session = baseSession;
    if (timeRange) {
        const clippedStart = Math.max(timeRange.startAt, baseSession.startAt);
        const clippedEnd   = Math.min(timeRange.endAt,   baseSession.endAt);
        if (clippedStart >= clippedEnd) return null;
        const totalDur = clippedEnd - clippedStart;
        const inactDur = sumIntervalDuration(baseSession.invincibleIntervals, clippedStart, clippedEnd);
        session = {
            ...baseSession,
            startAt:           clippedStart,
            endAt:             clippedEnd,
            totalDuration:     totalDur,
            inactiveDuration:  inactDur,
            effectiveDuration: Math.max(0, totalDur - inactDur),
        };
    }

    const selectedEffectiveEvents = timeRange
        ? bossEffectiveEvents.filter(
            (damage) => damage.At >= session.startAt && damage.At <= session.endAt,
        )
        : bossEffectiveEvents;
    // 找出在 session 期間內有對此 Boss 造成傷害的玩家
    const players: PlayerSummary[] = [];
    for (const entityId in actorManager.entityMap) {
        const entity = actorManager.entityMap[entityId] as EntityActor;
        if (!entity.isPC) continue;

        const hasHit = entity.applyDamages.some(
            (d) => d.TargetId === bossEntityId
                && d.Damage > 0 && d.At >= session.startAt && d.At <= session.endAt,
        );
        if (!hasHit) continue;

        players.push(
            buildPlayerSummary(
                entity,
                bossEntityId,
                session,
                bossTargetEntityIds,
            ),
        );
    }

    // 依總傷害降序排列
    players.sort((a, b) => b.totalDamage - a.totalDamage);

    // 計算自訂條件結果（對每個玩家）
    const customConditions: CustomConditionResult[] = customConfigs.map(
        (config) => {
            const perPlayer = players.map((p) => {
                const playerEntity = actorManager.entityMap[p.entityId] as EntityActor;
                return {
                    player: { entityId: p.entityId, name: p.name },
                    result: buildCustomConditionResult(
                        config,
                        playerEntity,
                        bossEntity,
                        session,
                        bossTargetEntityIds,
                    ),
                };
            });

            // 合併所有玩家的 groups，保留 per-player 明細
            const mergedGroups = mergePlayerConditionResults(perPlayer);

            const totalDamage = mergedGroups.reduce(
                (s, g) => s + g.totalDamage,
                0,
            );

            return {
                configId: config.id,
                displayName: config.displayName,
                groups: mergedGroups,
                totalDamage,
                averageDamage:
                    mergedGroups.length > 0
                        ? totalDamage / mergedGroups.length
                        : 0,
            };
        },
    );

    const totalDamage = players.reduce((s, p) => s + p.totalDamage, 0);
    const authoritativeHealthLoss = bossHealthLosses
        .filter((loss) => loss.At >= session.startAt && loss.At <= session.endAt)
        .reduce((sum, loss) => sum + loss.Damage, 0);
    const attributedEffectiveDamage = selectedEffectiveEvents
        .reduce((sum, damage) => sum + damage.Damage, 0);
    const effectiveBossDamage = authoritativeHealthLoss
        || attributedEffectiveDamage
        || totalDamage;

    // ── Boss 技能使用統計 ──────────────────────────────────────
    const bossSkillStats = buildBossSkillStats(bossEntity, session);

    return {
        session,
        players,
        customConditions,
        totalDamage,
        effectiveBossDamage,
        bossSkillStats,
    };
}

/** Boss 在 session 期間內使用技能的統計（依 applyDamages 計算） */
function buildBossSkillStats(
    bossEntity: EntityActor,
    session: BossFightSession,
): BossSkillStat[] {
    const damages = bossEntity.applyDamages.filter(
        (d) => d.Damage > 0 && d.At >= session.startAt && d.At <= session.endAt,
    );
    if (damages.length === 0) return [];

    const map = new Map<number, { totalDamage: number; totalHits: number }>();
    for (const d of damages) {
        let s = map.get(d.SkillId);
        if (!s) {
            s = { totalDamage: 0, totalHits: 0 };
            map.set(d.SkillId, s);
        }
        s.totalDamage += d.Damage;
        s.totalHits++;
    }

    return [...map.entries()]
        .map(([skillId, s]) => ({
            skillId,
            useCount: s.totalHits,
            totalHits: s.totalHits,
            totalDamage: s.totalDamage,
        }))
        .sort((a, b) => b.useCount - a.useCount);
}

/** 從 skillSequence 計算被動傷害、暴擊統計 */
function calcSeqStats(seq: SkillUse[]): { passiveDamage: number; critEligible: number; critHits: number } {
    let passiveDamage = 0, critEligible = 0, critHits = 0;
    for (const u of seq) {
        if (NO_CRIT_SKILL_IDS.has(u.skillId)) {
            passiveDamage += u.totalDamage;
        } else {
            critEligible++;
            if (u.isCrit) critHits++;
        }
    }
    return { passiveDamage, critEligible, critHits };
}

/**
 * 將多個玩家的 CustomConditionResult 合併：
 * 相同 index 的 group 傷害疊加、技能序列合併
 */
function mergePlayerConditionResults(
    entries: { player: { entityId: string; name: string }; result: CustomConditionResult }[],
): ConditionGroup[] {
    // 展開所有玩家的 group，附帶玩家資訊，略過空 group
    const tagged: (Omit<ConditionGroup, "players"> & {
        playerEntityId: string;
        playerName: string;
    })[] = [];

    for (const { player, result } of entries) {
        for (const group of result.groups) {
            if (group.totalDamage <= 0) continue;
            tagged.push({
                ...group,
                skillSequence: [...group.skillSequence],
                playerEntityId: player.entityId,
                playerName: player.name,
            });
        }
    }

    if (tagged.length === 0) return [];

    // 依 startAt 排序
    tagged.sort((a, b) => a.startAt - b.startAt);

    // 以時間重疊合併（同一條件期間的不同玩家 group → 同一段）
    // 兩個 group 的 startAt 差距在 ACTIVE_GAP 秒內視為同一段
    const merged: ConditionGroup[] = [];

    for (const g of tagged) {
        const last = merged[merged.length - 1];
        const overlaps = last && g.startAt <= last.endAt + ACTIVE_GAP;

        if (overlaps) {
            last.endAt = Math.max(last.endAt, g.endAt);
            last.duration = last.endAt - last.startAt;
            last.totalDamage += g.totalDamage;
            last.skillSequence = [
                ...last.skillSequence,
                ...g.skillSequence,
            ].sort((a, b) => a.at - b.at);

            const ep = last.players.find((p) => p.entityId === g.playerEntityId);
            if (ep) {
                const stats = calcSeqStats(g.skillSequence);
                ep.totalDamage    += g.totalDamage;
                ep.passiveDamage  += stats.passiveDamage;
                ep.critEligible   += stats.critEligible;
                ep.critHits       += stats.critHits;
                ep.skillSequence   = [...ep.skillSequence, ...g.skillSequence].sort((a, b) => a.at - b.at);
            } else {
                const seq   = [...g.skillSequence];
                const stats = calcSeqStats(seq);
                last.players.push({
                    entityId:     g.playerEntityId,
                    name:         g.playerName,
                    totalDamage:  g.totalDamage,
                    passiveDamage:  stats.passiveDamage,
                    critEligible:   stats.critEligible,
                    critHits:       stats.critHits,
                    skillSequence:  seq,
                });
            }
        } else {
            const seq   = [...g.skillSequence];
            const stats = calcSeqStats(seq);
            merged.push({
                index:      merged.length + 1,
                startAt:    g.startAt,
                endAt:      g.endAt,
                duration:   g.endAt - g.startAt,
                totalDamage: g.totalDamage,
                skillSequence: seq,
                players: [{
                    entityId:     g.playerEntityId,
                    name:         g.playerName,
                    totalDamage:  g.totalDamage,
                    passiveDamage:  stats.passiveDamage,
                    critEligible:   stats.critEligible,
                    critHits:       stats.critHits,
                    skillSequence:  [...seq],
                }],
            });
        }
    }

    // 各 group 內玩家依傷害降序排列
    for (const g of merged) {
        g.players.sort((a, b) => b.totalDamage - a.totalDamage);
    }

    return merged;
}
