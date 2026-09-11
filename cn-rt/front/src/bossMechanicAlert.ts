export type BossMechanicTrigger = "orb-spawn" | "skill-action";
export type BossMechanicSoundMode = "none" | "dedicated" | "custom";

export interface BossMechanicAlertRule {
    key: string;
    name: string;
    bossRaceIds: number[];
    trigger: BossMechanicTrigger;
    skillId: number;
    triggerRaceIds: number[];
    /** Conditions emitted by a mechanism entity when it is consumed early. */
    cancelConditionIds: number[];
    enabled: boolean;
    countdownSeconds: number;
    showCountdown: boolean;
    soundMode: BossMechanicSoundMode;
    customSoundId: string;
    customSoundName: string;
    /** Screen coordinate of this mechanic card's top-left corner. */
    x: number;
    y: number;
}

export interface BossMechanicAlertSettings {
    volume: number;
    x: number;
    y: number;
    scalePercent: number;
    /** Independently selectable phase bars for Miel's comfort-shard mechanic. */
    mielShardHealthPhases: MielShardHealthPhaseSettings;
    mielShardHealthBarX: number;
    mielShardHealthBarY: number;
    mielShardHealthBarScalePercent: number;
    mielShardHealthBarOpacityPercent: number;
    /** Legacy fields are read only while migrating settings saved by older builds. */
    miel60HealthBarEnabled?: boolean;
    miel60HealthBarX?: number;
    miel60HealthBarY?: number;
    miel60HealthBarScalePercent?: number;
    rules: Record<string, BossMechanicAlertRule>;
}

export interface MielShardHealthPhaseSettings {
    normal80: boolean;
    normal60: boolean;
    normal40: boolean;
    regret80: boolean;
}

export type MielShardHealthPhase = keyof MielShardHealthPhaseSettings;

export type MielOrbHealthPhase = "high" | "low" | "unknown";

/**
 * Pure red-orb resolution state.  Contact feedback can only provisionally
 * hide the warning at the Boss-variant checkpoint; the state remains alive so a
 * later server confirmation can make the warning visible again.
 */
export interface MielOrbRuntime {
    startedAtMs: number;
    endsAtMs: number;
    bossRaceId: number;
    healthPhase: MielOrbHealthPhase;
    requiredContacts: number;
    checkpointAtMs: number;
    contactCount: number;
    /** Contacts received no later than the checkpoint. */
    checkpointContactCount: number;
    lastContactAtMs: number | null;
    checkpointEvaluatedAtMs: number | null;
    provisionallyHidden: boolean;
    lateConfirmedAtMs: number | null;
}

export const MIEL_ORB_CONTACT_GROUP_MS = 120;
export const MIEL_ORB_HP_SPLIT_PERCENT = 60;
export const MIEL_ORB_LATE_CONFIRM_START_MS = 12_000;
export const MIEL_ORB_LATE_CONFIRM_END_MS = 14_750;

const MIEL_ORB_7603_REQUIRED_CONTACTS = 8;
const MIEL_ORB_7603_CHECKPOINT_MS = 8_000;
const MIEL_ORB_7615_REQUIRED_CONTACTS = 10;
const MIEL_ORB_7615_CHECKPOINT_MS = 11_000;

export const BOSS_MECHANIC_EVENT = "dilmeter-boss-mechanic-event";
const STORAGE_KEY = "dilmeter-cn-boss-mechanic-alert-v1";

const DEFAULT_RULES: Record<string, BossMechanicAlertRule> = {
    "miel-orb": {
        key: "miel-orb",
        name: "缠绕的神迹（环绕球）",
        bossRaceIds: [7603, 7615],
        trigger: "orb-spawn",
        skillId: 52402,
        triggerRaceIds: [7604, 7605, 7606, 7607, 7616, 7617, 7618, 7619],
        // CC 494 and the Boss's 52407 feedback show contact activity, but do
        // not authoritatively prove destruction.  The reference helper uses a
        // Boss-variant checkpoint, followed by a late-alive confirmation;
        // ordinary condition/entity disappearance must never cancel it.
        cancelConditionIds: [],
        enabled: false,
        countdownSeconds: 20,
        showCountdown: true,
        soundMode: "dedicated",
        customSoundId: "",
        customSoundName: "",
        x: defaultOverlayX(),
        y: defaultOverlayY(),
    },
    "miel-laser": {
        key: "miel-laser",
        name: "神剑（射线）",
        bossRaceIds: [7603, 7615],
        trigger: "skill-action",
        skillId: 52401,
        // The rotating beam is cast by the Boss-owned "安乐碎片" entities,
        // not necessarily by the 7603/7615 Boss entity itself.
        triggerRaceIds: [7604, 7605, 7606, 7607, 7608, 7616, 7617, 7618, 7619, 7620],
        cancelConditionIds: [],
        enabled: false,
        countdownSeconds: 5,
        showCountdown: true,
        soundMode: "dedicated",
        customSoundId: "",
        customSoundName: "",
        x: defaultOverlayX() + 140,
        y: defaultOverlayY(),
    },
};

export function loadBossMechanicAlertSettings(): BossMechanicAlertSettings {
    try {
        const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "null") as Partial<BossMechanicAlertSettings> | null;
        return normalizeBossMechanicAlertSettings(parsed);
    } catch {
        return normalizeBossMechanicAlertSettings(null);
    }
}

export function normalizeBossMechanicAlertSettings(value: unknown): BossMechanicAlertSettings {
    const fallback: BossMechanicAlertSettings = {
        volume: 80,
        x: defaultOverlayX(),
        y: defaultOverlayY(),
        scalePercent: 100,
        mielShardHealthPhases: { normal80: false, normal60: true, normal40: false, regret80: false },
        mielShardHealthBarX: defaultOverlayX() - 100,
        mielShardHealthBarY: defaultOverlayY(),
        mielShardHealthBarScalePercent: 100,
        mielShardHealthBarOpacityPercent: 100,
        rules: cloneRules(DEFAULT_RULES),
    };
    if (!value || typeof value !== "object") return fallback;
    const parsed = value as Partial<BossMechanicAlertSettings>;
    const legacyX = clampInteger(parsed.x, -32000, 32000, fallback.x);
    const legacyY = clampInteger(parsed.y, -32000, 32000, fallback.y);
    const rules = cloneRules(DEFAULT_RULES);
    for (const [key, raw] of Object.entries(parsed.rules ?? {})) {
        if (!raw || typeof raw !== "object") continue;
        const base = rules[key];
        if (!base) continue;
        const storedRule = raw as Partial<BossMechanicAlertRule>;
        const storedCountdown = Number(storedRule.countdownSeconds);
        const countdownFallback = key === "miel-orb" && storedCountdown === 15
            ? 20
            : base.countdownSeconds;
        // Mechanic identity, Boss/fragment races and skill mapping always come
        // from the current build. Profiles preserve only user preferences so
        // an old profile cannot roll back newly supported Boss variants.
        rules[key] = {
            ...base,
            enabled: storedRule.enabled === true,
            countdownSeconds: key === "miel-orb" && storedCountdown === 15
                ? 20
                : clamp(storedRule.countdownSeconds, 1, 120, countdownFallback),
            showCountdown: storedRule.showCountdown !== false,
            soundMode: sanitizeSoundMode(storedRule.soundMode),
            customSoundId: sanitizeText(storedRule.customSoundId, 128),
            customSoundName: sanitizeText(storedRule.customSoundName, 180),
            x: clampInteger(storedRule.x, -32000, 32000, key === "miel-laser" ? legacyX + 140 : legacyX),
            y: clampInteger(storedRule.y, -32000, 32000, legacyY),
        };
    }
    const storedPhases = parsed.mielShardHealthPhases;
    const legacy60Enabled = parsed.miel60HealthBarEnabled !== false;
    return {
        volume: clamp(parsed.volume, 0, 100, 80),
        x: legacyX,
        y: legacyY,
        scalePercent: clampInteger(parsed.scalePercent, 50, 200, 100),
        mielShardHealthPhases: storedPhases && typeof storedPhases === "object"
            ? {
                normal80: storedPhases.normal80 === true,
                normal60: storedPhases.normal60 === true,
                normal40: storedPhases.normal40 === true,
                regret80: storedPhases.regret80 === true,
            }
            : { normal80: false, normal60: legacy60Enabled, normal40: false, regret80: false },
        mielShardHealthBarX: clampInteger(parsed.mielShardHealthBarX ?? parsed.miel60HealthBarX, -32000, 32000, fallback.mielShardHealthBarX),
        mielShardHealthBarY: clampInteger(parsed.mielShardHealthBarY ?? parsed.miel60HealthBarY, -32000, 32000, fallback.mielShardHealthBarY),
        mielShardHealthBarScalePercent: clampInteger(parsed.mielShardHealthBarScalePercent ?? parsed.miel60HealthBarScalePercent, 50, 200, 100),
        mielShardHealthBarOpacityPercent: clampInteger(parsed.mielShardHealthBarOpacityPercent, 20, 100, 100),
        rules,
    };
}

export function saveBossMechanicAlertSettings(settings: BossMechanicAlertSettings): void {
    settings.volume = clamp(settings.volume, 0, 100, 80);
    settings.x = clampInteger(settings.x, -32000, 32000, defaultOverlayX());
    settings.y = clampInteger(settings.y, -32000, 32000, defaultOverlayY());
    settings.scalePercent = clampInteger(settings.scalePercent, 50, 200, 100);
    settings.mielShardHealthPhases = {
        normal80: settings.mielShardHealthPhases?.normal80 === true,
        normal60: settings.mielShardHealthPhases?.normal60 === true,
        normal40: settings.mielShardHealthPhases?.normal40 === true,
        regret80: settings.mielShardHealthPhases?.regret80 === true,
    };
    settings.mielShardHealthBarX = clampInteger(settings.mielShardHealthBarX, -32000, 32000, defaultOverlayX() - 100);
    settings.mielShardHealthBarY = clampInteger(settings.mielShardHealthBarY, -32000, 32000, defaultOverlayY());
    settings.mielShardHealthBarScalePercent = clampInteger(settings.mielShardHealthBarScalePercent, 50, 200, 100);
    settings.mielShardHealthBarOpacityPercent = clampInteger(settings.mielShardHealthBarOpacityPercent, 20, 100, 100);
    delete settings.miel60HealthBarEnabled;
    delete settings.miel60HealthBarX;
    delete settings.miel60HealthBarY;
    delete settings.miel60HealthBarScalePercent;
    for (const rule of Object.values(settings.rules)) {
        rule.countdownSeconds = clamp(rule.countdownSeconds, 1, 120, 5);
        rule.soundMode = sanitizeSoundMode(rule.soundMode);
        rule.customSoundId = sanitizeText(rule.customSoundId, 128);
        rule.customSoundName = sanitizeText(rule.customSoundName, 180);
        rule.x = clampInteger(rule.x, -32000, 32000, defaultOverlayX());
        rule.y = clampInteger(rule.y, -32000, 32000, defaultOverlayY());
    }
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}

export function resolveMielOrbHealthPhase(
    currentHealth: unknown,
    maximumHealth: unknown,
): MielOrbHealthPhase {
    const current = Number(currentHealth);
    const maximum = Number(maximumHealth);
    if (!Number.isFinite(current) || !Number.isFinite(maximum) || maximum <= 0 || current < 0) {
        return "unknown";
    }
    return current / maximum >= MIEL_ORB_HP_SPLIT_PERCENT / 100 ? "high" : "low";
}

export function createMielOrbRuntime(
    startedAtMs: number,
    bossRaceId: number,
    currentHealth?: unknown,
    maximumHealth?: unknown,
    countdownSeconds = 20,
): MielOrbRuntime {
    const start = finiteTimestamp(startedAtMs);
    const phase = resolveMielOrbHealthPhase(currentHealth, maximumHealth);
    const raceId = Number.isInteger(Number(bossRaceId)) ? Number(bossRaceId) : 0;
    // These two variants have different clear requirements even at the same
    // health percentage. Unknown variants use the conservative 10/+11 path.
    const eightContactVariant = raceId === 7603;
    const checkpointDelay = eightContactVariant ? MIEL_ORB_7603_CHECKPOINT_MS : MIEL_ORB_7615_CHECKPOINT_MS;
    const durationMs = Math.max(1_000, finiteNumber(countdownSeconds, 20) * 1000);
    return {
        startedAtMs: start,
        endsAtMs: start + durationMs,
        bossRaceId: raceId,
        healthPhase: phase,
        requiredContacts: eightContactVariant ? MIEL_ORB_7603_REQUIRED_CONTACTS : MIEL_ORB_7615_REQUIRED_CONTACTS,
        checkpointAtMs: start + checkpointDelay,
        contactCount: 0,
        checkpointContactCount: 0,
        lastContactAtMs: null,
        checkpointEvaluatedAtMs: null,
        provisionallyHidden: false,
        lateConfirmedAtMs: null,
    };
}

export function observeMielOrbContact(runtime: MielOrbRuntime, atMs: number): MielOrbRuntime {
    const observedAt = Number(atMs);
    if (!Number.isFinite(observedAt) || observedAt < runtime.startedAtMs || observedAt > runtime.endsAtMs) {
        return runtime;
    }
    if (runtime.lastContactAtMs !== null) {
        const elapsed = observedAt - runtime.lastContactAtMs;
        if (elapsed < MIEL_ORB_CONTACT_GROUP_MS) return runtime;
    }
    const contactCount = runtime.contactCount + 1;
    const crossedRequiredContacts = runtime.contactCount < runtime.requiredContacts
        && contactCount >= runtime.requiredContacts;
    return {
        ...runtime,
        contactCount,
        checkpointContactCount: runtime.checkpointContactCount + (observedAt <= runtime.checkpointAtMs ? 1 : 0),
        lastContactAtMs: observedAt,
        // The reference helper checks once at the scheduled checkpoint, but
        // also clears immediately if the required contact is reached later.
        provisionallyHidden: runtime.provisionallyHidden
            || (runtime.checkpointEvaluatedAtMs !== null && crossedRequiredContacts),
    };
}

/** Evaluate the scheduled Boss-variant checkpoint. */
export function advanceMielOrbRuntime(runtime: MielOrbRuntime, atMs: number): MielOrbRuntime {
    const observedAt = Number(atMs);
    if (!Number.isFinite(observedAt)
        || observedAt < runtime.checkpointAtMs
        || runtime.checkpointEvaluatedAtMs !== null) {
        return runtime;
    }
    return {
        ...runtime,
        checkpointEvaluatedAtMs: observedAt,
        provisionallyHidden: runtime.checkpointContactCount >= runtime.requiredContacts,
    };
}

/**
 * Record a 0x6d66 late confirmation only while the orb is unresolved. Once the
 * Boss-specific contact checkpoint has confirmed a clear, that decision is
 * terminal for this generation: a delayed/duplicate confirmation must not
 * resurrect the card at the 6.5-second warning threshold.
 */
export function observeMielOrbLateConfirmation(runtime: MielOrbRuntime, atMs: number): MielOrbRuntime {
    const observedAt = Number(atMs);
    if (!Number.isFinite(observedAt)) return runtime;
    const elapsed = observedAt - runtime.startedAtMs;
    if (elapsed < MIEL_ORB_LATE_CONFIRM_START_MS || elapsed > MIEL_ORB_LATE_CONFIRM_END_MS) {
        return runtime;
    }
    if (runtime.provisionallyHidden) return runtime;
    return {
        ...runtime,
        lateConfirmedAtMs: observedAt,
    };
}

export function isMielOrbVisible(runtime: MielOrbRuntime, atMs: number): boolean {
    const observedAt = Number(atMs);
    return Number.isFinite(observedAt)
        && observedAt >= runtime.startedAtMs
        && observedAt < runtime.endsAtMs
        && !runtime.provisionallyHidden;
}

function defaultOverlayX(): number {
    if (typeof window === "undefined" || !window.screen) return 850;
    return Math.round(window.screen.width / 2 - 110);
}

function defaultOverlayY(): number {
    if (typeof window === "undefined" || !window.screen) return 280;
    return Math.round(window.screen.height * 0.27);
}

function sanitizeSoundMode(value: unknown): BossMechanicSoundMode {
    // Generic Boss sounds from older profiles migrate to the new mechanic-
    // specific timeline.  These two long cues are never exposed to Buff,
    // Debuff or skill-CD sound pickers.
    if (value === "dedicated" || value === "custom") return value;
    if (value === "electronic" || value === "voice") return "dedicated";
    return "none";
}

function sanitizeText(value: unknown, maxLength: number): string {
	if (typeof value !== "string") return "";
	return value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength);
}

function clamp(value: unknown, min: number, max: number, fallback: number): number {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(max, Math.max(min, Math.round(number * 10) / 10)) : fallback;
}

function clampInteger(value: unknown, min: number, max: number, fallback: number): number {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(max, Math.max(min, Math.round(number))) : fallback;
}

function finiteTimestamp(value: unknown): number {
    const number = Number(value);
    return Number.isFinite(number) && number >= 0 ? number : 0;
}

function finiteNumber(value: unknown, fallback: number): number {
    const number = Number(value);
    return Number.isFinite(number) && number > 0 ? number : fallback;
}

function cloneRules(value: Record<string, BossMechanicAlertRule>): Record<string, BossMechanicAlertRule> {
    return JSON.parse(JSON.stringify(value)) as Record<string, BossMechanicAlertRule>;
}
