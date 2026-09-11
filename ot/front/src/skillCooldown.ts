import type { EffectTimerOverlayItem } from "@/effectTimer";

export type SkillCooldownSoundMode = "default" | "none" | "custom";
export type SkillCooldownOwnerMode = "auto" | "player" | "pet";

export interface SkillCooldownRule {
    skillId: number;
    enabled: boolean;
    /** Track cooldown state for SkillBarOverlay without showing the legacy ready reminder. */
    barOnly?: boolean;
    /** Full lockout after a cumulative-cooldown skill reaches its limit. */
    cooldownSeconds: number;
    /** Amount added to the decaying pool by each Spiral Burst / Charging Strike use. */
    shortCooldownSeconds: number;
    /** Pool limit that starts the full cooldown when reached. */
    cumulativeCooldownSeconds: number;
    alwaysVisible: boolean;
    /** Auto uses the action source owner; explicit modes handle ambiguous skills. */
    ownerMode: SkillCooldownOwnerMode;
    soundMode: SkillCooldownSoundMode;
    customSoundId: string;
    customSoundName: string;
    /** Toah Spirit (27012) reminder threshold; ignored by ordinary skills. */
    progressThresholdPercent: number;
    /** Screen coordinate of the skill icon's top-left corner. */
    x: number;
    y: number;
}

export interface AimReminderSettings {
    enabled: boolean;
    /** Keep the independent prompt visible while Magnum Shot is idle. */
    alwaysVisible: boolean;
    /** Weapon's listed base range before the range identification bonus. */
    weaponRange: number;
    /** Range identification level; each level adds 70 to the effective range. */
    rangeIdentificationLevel: number;
    /** Weapon + skill aim correction. Magnum Shot starts from this system aim rate. */
    calibrationPercent: number;
    /** Erg aim-speed percentage already present before temporary skill buffs. */
    ergSpeedPercent: number;
    /** User correction applied to the calculated no-temporary-buff best-shot time. */
    fineTuneSeconds: number;
    /** Overall icon, text and progress-bar scale. */
    scalePercent: number;
    /** Screen coordinate of the independent horizontal aim reminder. */
    x: number;
    y: number;
}

export interface SkillCooldownSettings {
    iconSize: number;
    coordinateVersion: number;
    aimReminder: AimReminderSettings;
    rules: Record<number, SkillCooldownRule>;
}

export interface SkillCooldownRuntime {
    usedAtMs: number;
    readyAtMs: number;
    generation: number;
    petSkill?: boolean;
    /** Legacy field kept for message compatibility; cumulative CD has no separate refreshed window. */
    shortReadyAtMs?: number;
    /** Time when the currently accumulated cooldown debt naturally returns to zero. */
    accumulatedReadyAtMs?: number;
    /** Current, decaying cooldown debt for cumulative-cooldown skills. */
    accumulatedCooldownSeconds?: number;
    cooldownPhase?: "idle" | "accumulating" | "full";
}

export interface SkillCooldownActionObservation {
    At: number;
    AtMs?: number;
    IsFallback?: boolean;
    CombatActionId?: number;
}

export interface SkillCooldownAdjustmentObservation {
    At: number;
    AtMs?: number;
    Reset?: boolean;
    ReduceMs?: number;
}

export interface SkillCooldownOverlayItem extends SkillCooldownRuntime {
    skillId: number;
    name: string;
    iconUrl: string;
    alwaysVisible: boolean;
    barOnly?: boolean;
    x: number;
    y: number;
    /** Present only for the Toah Spirit progress reminder. */
    progressPercent?: number;
    progressThresholdPercent?: number;
    progressObserved?: boolean;
    /** Configured debt limit, supplied with cumulative-cooldown overlay items. */
    cumulativeCooldownSeconds?: number;
}

export interface AimReminderOverlayItem {
    active: boolean;
    alwaysVisible: boolean;
    startedAtMs: number;
    readyAtMs: number;
    speedMultiplier: number;
    buffNames: string[];
    calibrationPercent: number;
    targetId: string;
    x: number;
    y: number;
    iconUrl: string;
    scalePercent: number;
    generation: number;
}

export interface SkillCooldownOverlayMessage {
    type: "skill-cooldown-state";
    atMs: number;
    items: SkillCooldownOverlayItem[];
    settings: Pick<SkillCooldownSettings, "iconSize"> & { overlayEnabled?: boolean; dpiPercent?: number; opacity?: number };
    aimReminder?: AimReminderOverlayItem;
    targetHealth?: TargetHealthBarOverlayItem;
    effectTimers?: EffectTimerOverlayItem[];
    mechanics?: BossMechanicOverlayItem[];
    stackAlerts?: BuffStackAlertOverlayItem[];
}

export interface TargetHealthBarOverlayItem {
    entityId: string;
    name: string;
    currentHealth: number;
    maximumHealth: number;
    x: number;
    y: number;
    scalePercent: number;
    opacityPercent: number;
    phaseLabel: string;
    previewExpiresAtMs?: number;
    generation?: number;
}

export interface BossMechanicOverlayItem {
    key: string;
    name: string;
    icon: string;
    startedAtMs: number;
    endsAtMs: number;
    generation: number;
    x: number;
    y: number;
    scalePercent: number;
}

export interface BuffStackAlertOverlayItem {
    ccId: number;
    name: string;
    stack: number;
    startedAtMs: number;
    endsAtMs: number;
    generation: number;
    x: number;
    y: number;
    scalePercent: number;
}

export const SKILL_COOLDOWN_STORAGE_KEY = "dilmeter-cn-skill-cooldown-v1";
export const SKILL_COOLDOWN_CHANNEL = "dilmeter-cn-skill-cooldown-overlay-v1";
export const SKILL_ACTION_EVENT = "dilmeter-skill-action";
export const SKILL_DAMAGE_EVENT = "dilmeter-skill-damage";
export const SKILL_STATE_EVENT = "dilmeter-skill-state";
export const SKILL_COOLDOWN_ADJUSTMENT_EVENT = "dilmeter-skill-cooldown-adjustment";
export const MAGNUM_SHOT_SKILL_ID = 21002;
export const FINAL_SHOT_SKILL_ID = 23030;
export const LATIKA_SECRET_SKILL_ID = 23110;
export const RAPID_AIM_SKILL_ID = 58006;
export const LATIKA_SECRET_CC_ID = 407;
export const RAPID_AIM_CC_ID = 477;
/** The distance-independent 2025 archery update uses the former 1000-range reference point. */
export const MAGNUM_AIM_REFERENCE_DISTANCE = 1000;
export const MAGNUM_AIM_RANGE_FORMULA_DISTANCE_FACTOR = 7;
export const MAGNUM_AIM_RANGE_FORMULA_OFFSET_SECONDS = 1;
export const DEFAULT_MAGNUM_WEAPON_RANGE = 2200;
export const DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL = 0;
export const MAGNUM_RANGE_PER_IDENTIFICATION_LEVEL = 70;
export const DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT = 40;
export const DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT = 200;
export const DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS = 0;
/** Legacy export retained for older imports; 2200 range / 40% correction / Erg 200% is about 0.188s. */
export const DEFAULT_MAGNUM_AIM_SECONDS = 0.188;
export const MAGNUM_BEST_SYSTEM_AIM_RATE = 0.7;
export const MAGNUM_BEST_SHOT_PROGRESS = 0.85;
export const TOAH_SPIRIT_SKILL_ID = 27012;
export const TOAH_SPIRIT_STAT_ID = 198;
export const DEFAULT_TOAH_PROGRESS_THRESHOLD = 95;
export const SPIRAL_BURST_SKILL_ID = 59145;
export const CHARGING_STRIKE_SKILL_ID = 59104;
export const CUMULATIVE_COOLDOWN_SKILL_IDS: ReadonlySet<number> = new Set([
    SPIRAL_BURST_SKILL_ID,
    CHARGING_STRIKE_SKILL_ID,
]);
export const DEFAULT_SHORT_COOLDOWN_SECONDS = 1;
export const DEFAULT_CUMULATIVE_COOLDOWN_SECONDS = 10;

/**
 * Built-in descriptions shown when a tracked skill can receive an observed
 * server cooldown change. The packet still names the exact target skill, so
 * runtime handling remains correct even if the character lacks one of these
 * talents or weapon effects.
 */
export const BUILTIN_SKILL_COOLDOWN_RULE_DESCRIPTIONS: Readonly<Record<number, string>> = Object.freeze({
    20018: "双手剑／双手斧／双手锤精灵武器：暴击时 30% 概率由 27049 重置愤怒冲击 CD。",
    24101: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24102: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24103: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24201: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24202: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24301: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    24302: "燃魂技能暴击时由 27069 减少剩余 CD 20%；拳套改造触发时也可由 27049 重置。",
    26002: "手里剑精灵武器：暴击时 30% 概率由 27049 重置手里剑风暴 CD。",
    27201: "星辰交汇：该技能暴击出现 SLST27201 时，剩余 CD 减少 3 秒。",
    27202: "星辰交汇：该技能暴击出现 SLST27202 时，剩余 CD 减少 2 秒。",
    27203: "星辰交汇：该技能暴击出现 SLST27203 时，剩余 CD 减少 2 秒。",
    27204: "星辰交汇：该技能暴击出现 SLST27204 时，剩余 CD 减少 3 秒。",
    54306: "双枪精灵武器：暴击时 20% 概率由 27049 重置冲锋射击 CD。",
    59180: "燃魂技能暴击时由 27069 减少自身与格斗连续技的剩余 CD 20%。",
    59181: "燃魂技能暴击时由 27069 减少自身与格斗连续技的剩余 CD 20%。",
    59182: "燃魂技能暴击时由 27069 减少自身与格斗连续技的剩余 CD 20%。",
});

export function builtinSkillCooldownRuleDescription(skillId: unknown): string {
    return BUILTIN_SKILL_COOLDOWN_RULE_DESCRIPTIONS[Number(skillId)] ?? "";
}

export interface MagnumAimBuffState {
    finalShot: boolean;
    latikaSecret: boolean;
    rapid: boolean;
}

export interface MagnumAimTiming {
    speedMultiplier: number;
    temporarySpeedMultiplier: number;
    effectiveRange: number;
    durationMs: number;
    fullDurationMs: number;
    calculatedBaseBestMs: number;
    adjustedBaseBestMs: number;
    buffNames: string[];
}

export interface MagnumAimSample {
    atMs: number;
    rate: number;
}

export interface MagnumAimSummary {
    count: number;
    averageRate: number;
}

/** Summarizes completed Magnum Shot aim samples inside one combat session. */
export function summarizeMagnumAimSamples(
    samples: readonly MagnumAimSample[],
    sessionStartAtSeconds: unknown,
    sessionEndAtSeconds: unknown,
): MagnumAimSummary | undefined {
    const startAt = Number(sessionStartAtSeconds);
    const endAt = Number(sessionEndAtSeconds);
    if (!Number.isFinite(startAt) || !Number.isFinite(endAt) || endAt < startAt) return undefined;
    const startAtMs = startAt * 1000;
    // Session timestamps are whole seconds while aim samples retain milliseconds.
    const endBeforeMs = (endAt + 1) * 1000;
    let count = 0;
    let total = 0;
    for (const sample of samples) {
        const atMs = Number(sample?.atMs);
        const rate = Number(sample?.rate);
        if (!Number.isFinite(atMs) || !Number.isFinite(rate) || atMs < startAtMs || atMs >= endBeforeMs) continue;
        count += 1;
        total += Math.min(1, Math.max(0, rate));
    }
    return count > 0 ? { count, averageRate: total / count } : undefined;
}

/**
 * Converts elapsed time back to the game's displayed aim rate.
 * System aim grows by the square-root timing rule.  Above system 70%, the
 * client displays system + half of the remaining percentage, so 70% is shown
 * as 85% and remains the best-shot marker.
 */
export function calculateMagnumAimDisplayProgress(
    startedAtMs: unknown,
    bestAtMs: unknown,
    nowMs: unknown,
    calibrationPercent: unknown = DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
): number {
    const started = Number(startedAtMs);
    const best = Number(bestAtMs);
    const now = Number(nowMs);
    if (!Number.isFinite(started) || !Number.isFinite(best) || !Number.isFinite(now) || best <= started) return 0;
    const calibration = clampNumber(
        calibrationPercent,
        20,
        40,
        DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
    ) / 100;
    const bestSpan = Math.max(0.0001, MAGNUM_BEST_SYSTEM_AIM_RATE - calibration);
    const elapsedRatio = Math.max(0, now - started) / Math.max(1, best - started);
    const systemAim = Math.min(1, calibration + Math.sqrt(elapsedRatio) * bestSpan);
    if (systemAim < MAGNUM_BEST_SYSTEM_AIM_RATE) return systemAim;
    return Math.min(1, systemAim + (1 - systemAim) / 2);
}

/** Calculates system-70% / displayed-85% and full-aim times from game rules. */
export function calculateMagnumAimTiming(
    settings: Pick<
        AimReminderSettings,
        "weaponRange" | "rangeIdentificationLevel" | "calibrationPercent" | "ergSpeedPercent" | "fineTuneSeconds"
    >,
    buffs: Partial<MagnumAimBuffState> = {},
): MagnumAimTiming {
    const finalShot = buffs.finalShot === true;
    const latikaSecret = buffs.latikaSecret === true;
    const rapid = buffs.rapid === true;
    let temporarySpeedMultiplier = 1;
    let temporaryBuffNames: string[] = [];
    if (finalShot && latikaSecret) {
        temporarySpeedMultiplier = 9.6;
        temporaryBuffNames = ["无影箭", "拉蒂卡秘术"];
    } else if (latikaSecret) {
        temporarySpeedMultiplier = 4;
        temporaryBuffNames = ["拉蒂卡秘术"];
    } else if (finalShot) {
        temporarySpeedMultiplier = 2.4;
        temporaryBuffNames = ["无影箭"];
    } else if (rapid) {
        temporarySpeedMultiplier = 2;
        temporaryBuffNames = ["疾速"];
    }
    const weaponRange = clampNumber(
        settings?.weaponRange,
        100,
        10000,
        DEFAULT_MAGNUM_WEAPON_RANGE,
    );
    const rangeIdentificationLevel = Math.round(clampNumber(
        settings?.rangeIdentificationLevel,
        0,
        20,
        DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
    ));
    const calibrationPercent = clampNumber(
        settings?.calibrationPercent,
        20,
        40,
        DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
    );
    const ergSpeedPercent = clampNumber(
        settings?.ergSpeedPercent,
        100,
        1000,
        DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
    );
    const fineTuneSeconds = clampNumber(
        settings?.fineTuneSeconds,
        -10,
        10,
        DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
        2,
    );
    const calibration = calibrationPercent / 100;
    const ergSpeedMultiplier = ergSpeedPercent / 100;
    const effectiveRange = weaponRange + rangeIdentificationLevel * MAGNUM_RANGE_PER_IDENTIFICATION_LEVEL;
    const rangeBasedFullAimSeconds = MAGNUM_AIM_RANGE_FORMULA_DISTANCE_FACTOR
        * MAGNUM_AIM_REFERENCE_DISTANCE / effectiveRange
        + MAGNUM_AIM_RANGE_FORMULA_OFFSET_SECONDS;
    const bestSystemSpan = MAGNUM_BEST_SYSTEM_AIM_RATE - calibration;
    const calculatedBaseBestSeconds = rangeBasedFullAimSeconds * bestSystemSpan ** 2 / ergSpeedMultiplier;
    const adjustedBaseBestSeconds = Math.max(0.01, calculatedBaseBestSeconds + fineTuneSeconds);
    const bestDurationMs = Math.max(1, Math.round(
        adjustedBaseBestSeconds * 1000 / temporarySpeedMultiplier + 1e-9,
    ));
    const fullDurationRatio = ((1 - calibration) / Math.max(0.0001, bestSystemSpan)) ** 2;
    return {
        speedMultiplier: ergSpeedMultiplier * temporarySpeedMultiplier,
        temporarySpeedMultiplier,
        effectiveRange,
        durationMs: bestDurationMs,
        fullDurationMs: Math.max(bestDurationMs, Math.round(bestDurationMs * fullDurationRatio)),
        calculatedBaseBestMs: Math.round(calculatedBaseBestSeconds * 1000),
        adjustedBaseBestMs: Math.round(adjustedBaseBestSeconds * 1000),
        buffNames: [`弓尔格 ${Math.round(ergSpeedPercent)}%`, ...temporaryBuffNames],
    };
}

export function isCumulativeCooldownSkill(skillId: unknown): boolean {
    return CUMULATIVE_COOLDOWN_SKILL_IDS.has(Number(skillId));
}

export function toahSpiritDisplayPercent(rawProgress: unknown): number {
    const numericProgress = Number(rawProgress);
    if (!Number.isFinite(numericProgress)) return 0;
    return Math.floor(Math.min(100, Math.max(0, numericProgress)));
}

export function shouldShowToahSpiritProgressOverlay(
    item: Pick<SkillCooldownOverlayItem, "alwaysVisible" | "progressObserved" | "progressPercent" | "progressThresholdPercent">,
): boolean {
    if (item.alwaysVisible) return true;
    if (!item.progressObserved) return false;
    const progress = Math.min(100, Math.max(0, Number(item.progressPercent) || 0));
    const threshold = Math.min(100, Math.max(1, Number(item.progressThresholdPercent) || DEFAULT_TOAH_PROGRESS_THRESHOLD));
    return progress >= threshold && progress < 100;
}

export interface ToahSpiritProgressRuntime {
    observed: boolean;
    progressPercent: number;
    thresholdPercent: number;
    thresholdReached: boolean;
    fullChargeReached: boolean;
    triggeredAtMs: number;
    generation: number;
    fullChargeGeneration: number;
}

export function initialToahSpiritProgressRuntime(): ToahSpiritProgressRuntime {
    return {
        observed: false,
        progressPercent: 0,
        thresholdPercent: DEFAULT_TOAH_PROGRESS_THRESHOLD,
        thresholdReached: false,
        fullChargeReached: false,
        triggeredAtMs: 0,
        generation: 0,
        fullChargeGeneration: 0,
    };
}

/**
 * Stat 198 is the server-published Toah Spirit gauge (0–100). A reminder is
 * generated only on a below-to-above threshold crossing, then re-armed after
 * the gauge falls below the configured percentage or the live session resets.
 */
export function applyToahSpiritProgressObservation(
    previous: ToahSpiritProgressRuntime | undefined,
    rawProgress: unknown,
    rawThreshold: unknown,
    atMs = Date.now(),
): ToahSpiritProgressRuntime {
    const prior = previous ?? initialToahSpiritProgressRuntime();
    const numericProgress = Number(rawProgress);
    if (!Number.isFinite(numericProgress)) {
        return {
            observed: false,
            progressPercent: 0,
            thresholdPercent: clampNumber(rawThreshold, 1, 100, DEFAULT_TOAH_PROGRESS_THRESHOLD),
            thresholdReached: false,
            fullChargeReached: false,
            triggeredAtMs: prior.triggeredAtMs,
            generation: prior.generation,
            fullChargeGeneration: prior.fullChargeGeneration,
        };
    }

    const progressPercent = clampNumber(numericProgress, 0, 100, 0, 1);
    const threshold = clampNumber(rawThreshold, 1, 100, DEFAULT_TOAH_PROGRESS_THRESHOLD);
    // The server can publish tiny floating-point corrections. Only a genuine
    // spend/reset (more than five points down) re-arms alerts, matching the
    // observed game helper behaviour and preventing threshold chatter.
    const spentOrReset = prior.observed && prior.progressPercent - progressPercent > 5;
    const thresholdChanged = prior.thresholdPercent !== threshold;
    const wasThresholdReached = spentOrReset || thresholdChanged ? false : prior.thresholdReached;
    // Full-charge cooldown refresh is a separate edge signal: any reading
    // below 100 re-arms it, while repeated 100 values remain deduplicated.
    const wasFullChargeReached = progressPercent < 100 ? false : prior.fullChargeReached;
    const crossedThreshold = progressPercent >= threshold && !wasThresholdReached;
    const crossedFullCharge = progressPercent >= 100 && !wasFullChargeReached;
    return {
        observed: true,
        progressPercent,
        thresholdPercent: threshold,
        thresholdReached: wasThresholdReached || crossedThreshold,
        fullChargeReached: wasFullChargeReached || crossedFullCharge,
        triggeredAtMs: crossedThreshold ? atMs : prior.triggeredAtMs,
        generation: prior.generation + (crossedThreshold ? 1 : 0),
        fullChargeGeneration: prior.fullChargeGeneration + (crossedFullCharge ? 1 : 0),
    };
}

/**
 * Active techniques do not consistently send the ordinary skill-execute
 * packet.  Their self-applied condition is the server-confirmed use signal.
 */
export const TECHNIQUE_SKILL_BY_CONDITION: Readonly<Record<number, number>> = Object.freeze({
    479: 58000,
    478: 58001,
    487: 58005,
    477: 58006,
    476: 58007,
    521: 58010,
    522: 58013,
    520: 58014,
    555: 58016,
});

/** Active techniques that can be confirmed from a self-applied condition. */
export const ACTIVE_TECHNIQUE_SKILL_IDS: ReadonlySet<number> = new Set(
    Object.values(TECHNIQUE_SKILL_BY_CONDITION),
);

/**
 * The complete Technique category from the skill resource. Toah Spirit does
 * not reset any of these cooldowns. Keep the IDs explicit: 58100/58101 are
 * Stardust damage skills and must not be caught by a broad 58xxx rule.
 */
export const TECHNIQUE_SKILL_IDS: ReadonlySet<number> = new Set([
    58000, 58001, 58002, 58003, 58004,
    58005, 58006, 58007, 58008, 58009,
    58010, 58011, 58012, 58013, 58014,
    58015, 58016, 58017, 58018,
]);

export function shouldRefreshSkillCooldownFromToah(skillId: unknown, petSkill = false): boolean {
    const normalizedSkillId = Number(skillId);
    if (!Number.isInteger(normalizedSkillId) || normalizedSkillId <= 0) return false;
    return !petSkill && normalizedSkillId !== TOAH_SPIRIT_SKILL_ID
        && !TECHNIQUE_SKILL_IDS.has(normalizedSkillId);
}

export interface TechniqueConditionObservation {
    EventId: number;
    At: number;
    Id: string;
    CCId: number;
    DisableAt?: number;
    DisableAtMs?: number;
    DurationMs?: number;
}

/**
 * Browser-side safety net for active-technique cooldowns.  New desktop hosts
 * already publish EventSkillAction, but accepting the same authoritative CC
 * here keeps reminders working with an older/partially updated native host.
 * Expired channel snapshots are rejected so switching channel cannot start a
 * cooldown for a technique that was used before the new connection.
 */
export function techniqueCooldownActionFromCondition(
    condition: TechniqueConditionObservation,
    localEntityId: string,
): ({
    EventId: 10;
    At: number;
    AtMs: number;
    Id: string;
    SkillId: number;
    SubSkillId: number;
    CombatActionId: number;
    SourceId: string;
    IsFallback: false;
    IsLocal: true;
} | null) {
    const skillId = TECHNIQUE_SKILL_BY_CONDITION[Number(condition.CCId)] ?? 0;
    if (!skillId || !localEntityId || condition.Id !== localEntityId) return null;

    const eventAtMs = Number(condition.At) * 1000;
    const expiresAtMs = Number(condition.DisableAtMs) > 0
        ? Number(condition.DisableAtMs)
        : Number(condition.DisableAt) * 1000;
    if (expiresAtMs > 0 && expiresAtMs <= eventAtMs) return null;

    const durationMs = Number(condition.DurationMs);
    const derivedAtMs = expiresAtMs > 0 && durationMs > 0
        ? expiresAtMs - durationMs
        : eventAtMs;
    const atMs = Number.isFinite(derivedAtMs) && derivedAtMs > 0 ? derivedAtMs : eventAtMs;
    return {
        EventId: 10,
        At: Math.floor(atMs / 1000),
        AtMs: atMs,
        Id: localEntityId,
        SkillId: skillId,
        SubSkillId: 0,
        CombatActionId: 0,
        SourceId: condition.Id,
        IsFallback: false,
        IsLocal: true,
    };
}

/**
 * Damage is a safe cooldown fallback only when it came from the local player,
 * or from the local player's marionette. Owned pets deliberately remain
 * excluded so summoning a pet cannot start an unrelated cooldown timer.
 */
export function isLocalCooldownDamage(
    skillId: number,
    sourceId: string,
    localEntityId: string,
    sourceOwnerId = "",
): boolean {
    if (!localEntityId) return false;
    if (sourceId === localEntityId) return true;
    if (sourceOwnerId !== localEntityId) return false;
    return (
        (skillId >= 54101 && skillId <= 54106)
        || (skillId >= 54151 && skillId <= 54156)
        || (skillId >= 59167 && skillId <= 59169)
    );
}

export function makeSkillCooldownRule(skillId: number, x = 600, y = 180): SkillCooldownRule {
    return {
        skillId,
        enabled: true,
        barOnly: false,
        cooldownSeconds: 30,
        shortCooldownSeconds: DEFAULT_SHORT_COOLDOWN_SECONDS,
        cumulativeCooldownSeconds: DEFAULT_CUMULATIVE_COOLDOWN_SECONDS,
        alwaysVisible: false,
        ownerMode: "auto",
        soundMode: "default",
        customSoundId: "",
        customSoundName: "",
        progressThresholdPercent: DEFAULT_TOAH_PROGRESS_THRESHOLD,
        x: clampNumber(x, -32000, 32000, 600),
        y: clampNumber(y, -32000, 32000, 180),
    };
}

export function makeAimReminderSettings(x = 600, y = 180): AimReminderSettings {
    return {
        enabled: true,
        alwaysVisible: false,
        weaponRange: DEFAULT_MAGNUM_WEAPON_RANGE,
        rangeIdentificationLevel: DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
        calibrationPercent: DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
        ergSpeedPercent: DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
        fineTuneSeconds: DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
        scalePercent: 100,
        x: clampNumber(x, -32000, 32000, 600),
        y: clampNumber(y, -32000, 32000, 180),
    };
}

export function loadSkillCooldownSettings(): SkillCooldownSettings {
    const fallback: SkillCooldownSettings = {
        iconSize: 48,
        coordinateVersion: 2,
        aimReminder: makeAimReminderSettings(),
        rules: {},
    };
    try {
        const raw = localStorage.getItem(SKILL_COOLDOWN_STORAGE_KEY);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw) as Partial<SkillCooldownSettings>;
        const rules: Record<number, SkillCooldownRule> = {};
        if (parsed.rules && typeof parsed.rules === "object") {
            let fallbackIndex = 0;
            for (const [rawId, rawRule] of Object.entries(parsed.rules)) {
                const skillId = Number(rawId);
                if (!Number.isInteger(skillId) || skillId <= 0 || !rawRule || typeof rawRule !== "object") continue;
                const value = rawRule as Partial<SkillCooldownRule>;
                const fallbackX = 600 + (fallbackIndex % 4) * 80;
                const fallbackY = 180 + Math.floor(fallbackIndex / 4) * 96;
                rules[skillId] = {
                    skillId,
                    enabled: value.enabled !== false,
                    barOnly: value.barOnly === true,
                    cooldownSeconds: clampNumber(value.cooldownSeconds, 0.1, 86400, 30, 1),
                    shortCooldownSeconds: clampNumber(
                        value.shortCooldownSeconds,
                        0.1,
                        86400,
                        DEFAULT_SHORT_COOLDOWN_SECONDS,
                        1,
                    ),
                    cumulativeCooldownSeconds: clampNumber(
                        value.cumulativeCooldownSeconds,
                        0.1,
                        86400,
                        DEFAULT_CUMULATIVE_COOLDOWN_SECONDS,
                        1,
                    ),
                    alwaysVisible: Boolean(value.alwaysVisible),
                    ownerMode: value.ownerMode === "player" || value.ownerMode === "pet" ? value.ownerMode : "auto",
                    soundMode: value.soundMode === "none" || value.soundMode === "custom" ? value.soundMode : "default",
                    customSoundId: sanitizeText(value.customSoundId, 128),
                    customSoundName: sanitizeText(value.customSoundName, 180),
                    progressThresholdPercent: clampNumber(
                        value.progressThresholdPercent,
                        1,
                        100,
                        DEFAULT_TOAH_PROGRESS_THRESHOLD,
                    ),
                    x: clampNumber(value.x, -32000, 32000, fallbackX),
                    y: clampNumber(value.y, -32000, 32000, fallbackY),
                };
                fallbackIndex += 1;
            }
        }
        const legacyRule = parsed.rules && typeof parsed.rules === "object"
            ? (parsed.rules as Record<number, unknown>)[MAGNUM_SHOT_SKILL_ID] as {
                aimAssistEnabled?: unknown;
                aimBaseSeconds?: unknown;
                x?: unknown;
                y?: unknown;
            } | undefined
            : undefined;
        const aimValue = parsed.aimReminder && typeof parsed.aimReminder === "object"
            ? parsed.aimReminder as Partial<AimReminderSettings>
            : undefined;
        const aimReminder = aimValue
            ? {
                enabled: aimValue.enabled !== false,
                alwaysVisible: Boolean(aimValue.alwaysVisible),
                weaponRange: clampNumber(
                    aimValue.weaponRange,
                    100,
                    10000,
                    DEFAULT_MAGNUM_WEAPON_RANGE,
                ),
                rangeIdentificationLevel: Math.round(clampNumber(
                    aimValue.rangeIdentificationLevel,
                    0,
                    20,
                    DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
                )),
                calibrationPercent: clampNumber(
                    aimValue.calibrationPercent,
                    20,
                    40,
                    DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
                ),
                ergSpeedPercent: clampNumber(
                    aimValue.ergSpeedPercent,
                    100,
                    1000,
                    DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
                ),
                fineTuneSeconds: clampNumber(
                    aimValue.fineTuneSeconds,
                    -10,
                    10,
                    DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
                    2,
                ),
                scalePercent: clampNumber(aimValue.scalePercent, 50, 200, 100),
                x: clampNumber(aimValue.x, -32000, 32000, 600),
                y: clampNumber(aimValue.y, -32000, 32000, 180),
            }
            : {
                enabled: legacyRule?.aimAssistEnabled === undefined || Boolean(legacyRule.aimAssistEnabled),
                alwaysVisible: false,
                weaponRange: DEFAULT_MAGNUM_WEAPON_RANGE,
                rangeIdentificationLevel: DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
                calibrationPercent: DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
                ergSpeedPercent: DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
                fineTuneSeconds: DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
                scalePercent: 100,
                x: clampNumber(legacyRule?.x, -32000, 32000, 600),
                y: clampNumber(legacyRule?.y, -32000, 32000, 180),
            };
        return {
            iconSize: clampNumber(parsed.iconSize, 24, 96, 48),
            coordinateVersion: Number(parsed.coordinateVersion) >= 2 ? 2 : 1,
            aimReminder,
            rules,
        };
    } catch {
        return fallback;
    }
}

export function saveSkillCooldownSettings(settings: SkillCooldownSettings) {
    settings.iconSize = clampNumber(settings.iconSize, 24, 96, 48);
    settings.coordinateVersion = 2;
    settings.aimReminder = {
        enabled: settings.aimReminder?.enabled !== false,
        alwaysVisible: Boolean(settings.aimReminder?.alwaysVisible),
        weaponRange: clampNumber(
            settings.aimReminder?.weaponRange,
            100,
            10000,
            DEFAULT_MAGNUM_WEAPON_RANGE,
        ),
        rangeIdentificationLevel: Math.round(clampNumber(
            settings.aimReminder?.rangeIdentificationLevel,
            0,
            20,
            DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
        )),
        calibrationPercent: clampNumber(
            settings.aimReminder?.calibrationPercent,
            20,
            40,
            DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
        ),
        ergSpeedPercent: clampNumber(
            settings.aimReminder?.ergSpeedPercent,
            100,
            1000,
            DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
        ),
        fineTuneSeconds: clampNumber(
            settings.aimReminder?.fineTuneSeconds,
            -10,
            10,
            DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
            2,
        ),
        scalePercent: clampNumber(settings.aimReminder?.scalePercent, 50, 200, 100),
        x: clampNumber(settings.aimReminder?.x, -32000, 32000, 600),
        y: clampNumber(settings.aimReminder?.y, -32000, 32000, 180),
    };
    for (const rule of Object.values(settings.rules)) {
        rule.barOnly = rule.barOnly === true;
        rule.cooldownSeconds = clampNumber(rule.cooldownSeconds, 0.1, 86400, 30, 1);
        rule.shortCooldownSeconds = clampNumber(
            rule.shortCooldownSeconds,
            0.1,
            86400,
            DEFAULT_SHORT_COOLDOWN_SECONDS,
            1,
        );
        rule.cumulativeCooldownSeconds = clampNumber(
            rule.cumulativeCooldownSeconds,
            0.1,
            86400,
            DEFAULT_CUMULATIVE_COOLDOWN_SECONDS,
            1,
        );
        rule.soundMode = rule.soundMode === "none" || rule.soundMode === "custom" ? rule.soundMode : "default";
        rule.ownerMode = rule.ownerMode === "player" || rule.ownerMode === "pet" ? rule.ownerMode : "auto";
        rule.customSoundId = sanitizeText(rule.customSoundId, 128);
        rule.customSoundName = sanitizeText(rule.customSoundName, 180);
        rule.progressThresholdPercent = clampNumber(
            rule.progressThresholdPercent,
            1,
            100,
            DEFAULT_TOAH_PROGRESS_THRESHOLD,
        );
        rule.x = clampNumber(rule.x, -32000, 32000, 600);
        rule.y = clampNumber(rule.y, -32000, 32000, 180);
    }
    localStorage.setItem(SKILL_COOLDOWN_STORAGE_KEY, JSON.stringify(settings));
    window.dispatchEvent(new CustomEvent("dilmeter-skill-cooldown-settings", { detail: settings }));
}

/**
 * Applies one server skill observation to a cooldown timer. Owned-entity and
 * damage-phase observations are fallback signals: the first one can start an
 * idle timer, while later hits cannot keep pushing the same cooldown forward.
 */
export function applySkillCooldownObservation(
    rule: Pick<SkillCooldownRule, "cooldownSeconds"> & Partial<Pick<SkillCooldownRule,
        "skillId" | "shortCooldownSeconds" | "cumulativeCooldownSeconds">>,
    previous: SkillCooldownRuntime | undefined,
    action: SkillCooldownActionObservation,
    nowMs = Date.now(),
): SkillCooldownRuntime {
    const observedAtMs = Number(action.AtMs) > 0 ? Number(action.AtMs) : Number(action.At) * 1000;
    const usedAtMs = Number.isFinite(observedAtMs) && observedAtMs > 0 ? observedAtMs : nowMs;
    // The native and browser CC paths can intentionally report the same
    // authoritative technique use.  Treat near-identical timestamps as one
    // observation before applying the weaker damage-fallback rule.
    const cumulative = isCumulativeCooldownSkill(rule.skillId);
    // These two skills can legitimately be pressed repeatedly while their
    // cooldown debt is accumulating, so only collapse truly adjacent packets.
    const duplicateWindowMs = cumulative ? 120 : 1500;
    if (previous && Math.abs(previous.usedAtMs - usedAtMs) <= duplicateWindowMs) {
        return previous;
    }
    if (action.IsFallback === true && previous && previous.readyAtMs > usedAtMs) {
        return previous;
    }
    if (cumulative) {
        const shortCooldownSeconds = clampNumber(
            rule.shortCooldownSeconds,
            0.1,
            86400,
            DEFAULT_SHORT_COOLDOWN_SECONDS,
            1,
        );
        const cumulativeCooldownSeconds = clampNumber(
            rule.cumulativeCooldownSeconds,
            0.1,
            86400,
            DEFAULT_CUMULATIVE_COOLDOWN_SECONDS,
            1,
        );
        // A full lockout rejects impossible early re-use packets. Once the
        // lockout has elapsed, the next confirmed use starts a fresh cycle.
        if (previous?.cooldownPhase === "full" && previous.readyAtMs > usedAtMs) return previous;

        const previousDebtReadyAtMs = Number(previous?.accumulatedReadyAtMs) || 0;
        const currentAccumulated = previous?.cooldownPhase === "accumulating"
            && previousDebtReadyAtMs > usedAtMs
            ? Math.max(0, (previousDebtReadyAtMs - usedAtMs) / 1000)
            : 0;
        // Accumulated CD is one decaying time pool. Every accepted use adds
        // one complete short-CD amount to the time still left in that pool:
        // 3s on the first use; after 1s, 2+3=5s; after another 1s, 4+3=7s.
        const accumulatedCooldownSeconds = Math.min(cumulativeCooldownSeconds, Math.round(
            (currentAccumulated + shortCooldownSeconds) * 10,
        ) / 10);
        const entersFullCooldown = accumulatedCooldownSeconds >= cumulativeCooldownSeconds;
        const accumulatedReadyAtMs = entersFullCooldown || accumulatedCooldownSeconds <= 0
            ? 0
            : usedAtMs + Math.round(accumulatedCooldownSeconds * 1000);
        return {
            usedAtMs,
            readyAtMs: entersFullCooldown
                ? usedAtMs + Math.max(100, Number(rule.cooldownSeconds) * 1000)
                : usedAtMs,
            shortReadyAtMs: 0,
            accumulatedReadyAtMs,
            accumulatedCooldownSeconds,
            cooldownPhase: entersFullCooldown ? "full" : "accumulating",
            generation: (previous?.generation ?? 0) + 1,
        };
    }
    return {
        usedAtMs,
        readyAtMs: usedAtMs + Math.max(100, Number(rule.cooldownSeconds) * 1000),
        cooldownPhase: "full",
        generation: (previous?.generation ?? 0) + 1,
    };
}

/**
 * Applies an authoritative reset or reduction only to a timer that is already
 * cooling. Signals for unselected/idle skills never create synthetic state.
 */
export function applySkillCooldownAdjustment(
    previous: SkillCooldownRuntime | undefined,
    adjustment: SkillCooldownAdjustmentObservation,
    nowMs = Date.now(),
): SkillCooldownRuntime | undefined {
    if (!previous) return previous;
    const observedAtMs = Number(adjustment.AtMs) > 0
        ? Number(adjustment.AtMs)
        : Number(adjustment.At) * 1000;
    const atMs = Number.isFinite(observedAtMs) && observedAtMs > 0 ? observedAtMs : nowMs;
    if (previous.readyAtMs <= atMs && previous.cooldownPhase !== "accumulating") return previous;

    const reset = adjustment.Reset === true;
    const reduceMs = Math.min(86_400_000, Math.max(0, Math.round(Number(adjustment.ReduceMs) || 0)));
    if (!reset && reduceMs <= 0) return previous;

    if (previous.cooldownPhase === "accumulating") {
        const accumulatedReadyAtMs = Number(previous.accumulatedReadyAtMs) || 0;
        if (accumulatedReadyAtMs <= atMs) return previous;
        const nextAccumulatedReadyAtMs = reset
            ? atMs
            : Math.max(atMs, accumulatedReadyAtMs - reduceMs);
        if (nextAccumulatedReadyAtMs <= atMs) {
            return {
                ...previous,
                readyAtMs: 0,
                shortReadyAtMs: 0,
                accumulatedReadyAtMs: 0,
                accumulatedCooldownSeconds: 0,
                cooldownPhase: "idle",
            };
        }
        return {
            ...previous,
            accumulatedReadyAtMs: nextAccumulatedReadyAtMs,
            accumulatedCooldownSeconds: Math.ceil((nextAccumulatedReadyAtMs - atMs) / 100) / 10,
        };
    }

    const readyAtMs = reset ? atMs : Math.max(atMs, previous.readyAtMs - reduceMs);
    return {
        ...previous,
        readyAtMs,
        cooldownPhase: readyAtMs <= atMs ? "idle" : previous.cooldownPhase,
        shortReadyAtMs: readyAtMs <= atMs ? 0 : previous.shortReadyAtMs,
        accumulatedReadyAtMs: readyAtMs <= atMs ? 0 : previous.accumulatedReadyAtMs,
        accumulatedCooldownSeconds: readyAtMs <= atMs ? 0 : previous.accumulatedCooldownSeconds,
    };
}

/**
 * Counts the accumulated CD pool back down to zero.
 */
export function settleSkillCooldownRuntime(
    rule: Pick<SkillCooldownRule, "skillId">,
    runtime: SkillCooldownRuntime | undefined,
    atMs = Date.now(),
): SkillCooldownRuntime | undefined {
    if (!runtime || !isCumulativeCooldownSkill(rule.skillId)) return runtime;
    if (runtime.cooldownPhase !== "accumulating") return runtime;
    const accumulated = Math.max(0, Number(runtime.accumulatedCooldownSeconds) || 0);
    const accumulatedReadyAtMs = Number(runtime.accumulatedReadyAtMs) || 0;
    if (accumulated > 0 && accumulatedReadyAtMs > 0 && atMs < accumulatedReadyAtMs) {
        const currentAccumulated = Math.ceil(Math.max(0, (accumulatedReadyAtMs - atMs) / 1000) * 10) / 10;
        if (currentAccumulated === accumulated) return runtime;
        return { ...runtime, accumulatedCooldownSeconds: currentAccumulated };
    }
    const resetAtMs = accumulatedReadyAtMs > 0
        ? accumulatedReadyAtMs
        : Number(runtime.shortReadyAtMs);
    if (resetAtMs <= 0 || atMs < resetAtMs) return runtime;
    return {
        ...runtime,
        readyAtMs: 0,
        shortReadyAtMs: 0,
        accumulatedReadyAtMs: 0,
        accumulatedCooldownSeconds: 0,
        cooldownPhase: "idle",
    };
}

function sanitizeText(value: unknown, maxLength: number): string {
    if (typeof value !== "string") return "";
    return value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength);
}

function clampNumber(value: unknown, min: number, max: number, fallback: number, decimals = 0): number {
    const number = Number(value);
    if (!Number.isFinite(number)) return fallback;
    const factor = 10 ** decimals;
    return Math.min(max, Math.max(min, Math.round(number * factor) / factor));
}
