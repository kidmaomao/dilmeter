export interface DebuffAlertRule {
    ccId: number;
    enabled: boolean;
    warningSeconds: number;
    flashEnabled: boolean;
    /** Stable user-defined display order. */
    order: number;
    /** Sound can be disabled without losing the selected sound. */
    soundEnabled: boolean;
    soundMode: DebuffSoundMode;
    customSoundId: string;
    customSoundName: string;
}

export type DebuffSoundMode = "none" | "electronic" | "voice" | "custom";

export interface DebuffAlertSettings {
    overlayEnabled: boolean;
    firstRoundGraceSeconds: number;
    iconSize: number;
    volume: number;
    /** Race IDs that may be selected as the sole Debuff target below the normal Boss HP threshold. */
    forcedBossRaceIds: number[];
    soundMode: DebuffSoundMode;
    customSoundId: string;
    customSoundName: string;
    rules: Record<number, DebuffAlertRule>;
}

export interface DebuffOverlayItem {
    ccId: number;
    name: string;
    iconUrl: string;
    appliedAt: number;
    expiresAt: number | null;
    warningSeconds: number;
    state: "expiring" | "missing";
}

export interface DebuffOverlayBoss {
    entityId: string;
    name: string;
    appearedAt: number;
}

export interface TargetHealthOverlayItem {
    entityId: string;
    name: string;
    currentHealth: number;
    maximumHealth: number;
}

export interface DebuffOverlayMessage {
    type: "debuff-state";
    at: number;
    boss: DebuffOverlayBoss | null;
    targetHealth: TargetHealthOverlayItem | null;
    items: DebuffOverlayItem[];
    settings: { iconSize: number; overlayEnabled: boolean; dpiPercent?: number; opacity?: number };
}

export const DEBUFF_OVERLAY_CHANNEL = "dilmeter-cn-debuff-overlay-v1";
export const DEBUFF_BOSS_MIN_HEALTH = 100_000_000;

export interface DebuffBossCandidate {
    entityId: string;
    raceId: number;
    estimatedHealth: number;
    lastDamageAt: number;
    active: boolean;
}

/**
 * Debuff reminders deliberately track exactly one active Boss.  The currently
 * selected eligible target wins; otherwise the largest, most recently damaged
 * eligible target is used.  Ordinary monsters never arm the Debuff overlay or
 * its sounds, even when the battle-target filter is disabled in the main UI.
 */
export function selectDebuffBossCandidate<T extends DebuffBossCandidate>(
    candidates: readonly T[],
    preferredEntityId: string,
    minimumHealth = DEBUFF_BOSS_MIN_HEALTH,
    forcedBossRaceIds: readonly number[] = [],
): T | null {
    const forcedRaceIds = new Set(sanitizeRaceIds(forcedBossRaceIds));
    const eligible = candidates
        .filter((candidate) => candidate.active && (
            candidate.estimatedHealth >= minimumHealth || forcedRaceIds.has(candidate.raceId)
        ))
        .sort((a, b) => b.estimatedHealth - a.estimatedHealth || b.lastDamageAt - a.lastDamageAt);
    return eligible.find((candidate) => candidate.entityId === preferredEntityId) ?? eligible[0] ?? null;
}

const DEBUFF_EQUIVALENCE_GROUPS = [
    [912, 913],
    [392, 504],
] as const;

/** Only the two verified CN-server replacement pairs are grouped. */
export function equivalentDebuffIds(ccId: number): number[] {
    const group = DEBUFF_EQUIVALENCE_GROUPS.find((ids) => (ids as readonly number[]).includes(ccId));
    return group ? [...group] : [ccId];
}

export function canonicalDebuffId(ccId: number): number {
    return equivalentDebuffIds(ccId)[0];
}

export function equivalentDebuffDisplayName(ccId: number): string | undefined {
    const canonical = canonicalDebuffId(ccId);
    if (canonical === 912) return "喵咪的奔袭";
    if (canonical === 392) return "雷霆咆哮 / 闪电风暴";
    return undefined;
}

export type DebuffAlertDecision = "hidden" | "expiring" | "missing";

export const DEBUFF_SOUND_MERGE_COOLDOWN_MS = 1_800;

export interface DebuffSoundCandidate {
    state: "expiring" | "missing";
    hasBeenApplied: boolean;
    soundEnabled: boolean;
    soundMode: DebuffSoundMode;
    customSoundId: string;
}

/** Initial missing icons are visual-only; expiry or a previously applied Debuff may sound. */
export function isDebuffSoundCandidate(candidate: DebuffSoundCandidate): boolean {
    if (!candidate.soundEnabled || candidate.soundMode === "none") return false;
    if (candidate.soundMode === "custom" && !candidate.customSoundId) return false;
    return candidate.state === "expiring" || candidate.hasBeenApplied;
}

/** A batch deliberately produces at most one sound and never queues behind an active sound. */
export function selectDebuffSoundCandidate<T extends DebuffSoundCandidate>(
    candidates: readonly T[],
    nowMs: number,
    blockedUntilMs: number,
    playing: boolean,
): T | null {
    if (playing || nowMs < blockedUntilMs) return null;
    return candidates.find(isDebuffSoundCandidate) ?? null;
}

/**
 * Decide whether an armed Boss Debuff needs attention.
 *
 * Missing Debuffs stay visible. When a trustworthy future expiry exists, the
 * icon can return shortly before expiry; otherwise the enable/disable stream
 * remains authoritative and no duration is guessed.
 */
export function evaluateDebuffAlert(
    active: boolean,
    expiresAt: number | null,
    now: number,
    warningSeconds: number,
): DebuffAlertDecision {
    if (!active) return "missing";
    if (expiresAt !== null && Number.isFinite(expiresAt) && expiresAt > now) {
        return expiresAt - now <= Math.max(1, warningSeconds) ? "expiring" : "hidden";
    }
    return "hidden";
}

const STORAGE_KEY = "dilmeter-cn-debuff-alert-v1";
export const DEFAULT_DEBUFF_ALERT_IDS = [1164, 1165, 1166, 504, 598, 913, 1093, 1094, 1138] as const;

export function makeDebuffAlertRule(ccId: number): DebuffAlertRule {
    return {
        ccId,
        enabled: true,
        warningSeconds: 20,
        flashEnabled: true,
        order: 0,
        soundEnabled: false,
        soundMode: "electronic",
        customSoundId: "",
        customSoundName: "",
    };
}

export function loadDebuffAlertSettings(): DebuffAlertSettings {
    const defaultRules = Object.fromEntries(DEFAULT_DEBUFF_ALERT_IDS.map((ccId, order) => {
        const rule = makeDebuffAlertRule(ccId);
        rule.order = order;
        return [ccId, rule];
    })) as Record<number, DebuffAlertRule>;
    const fallback: DebuffAlertSettings = {
        overlayEnabled: true,
        firstRoundGraceSeconds: 10,
        iconSize: 30,
        volume: 80,
        forcedBossRaceIds: [],
        soundMode: "electronic",
        customSoundId: "",
        customSoundName: "",
        rules: defaultRules,
    };
    try {
        const parsed = JSON.parse(localStorage.getItem(STORAGE_KEY) || "null") as Partial<DebuffAlertSettings> | null;
        if (!parsed) return fallback;
        const rules: Record<number, DebuffAlertRule> = {};
        const legacySoundMode = sanitizeSoundMode(parsed.soundMode);
        for (const [index, [rawId, rawRule]] of Object.entries(parsed.rules ?? {}).entries()) {
            const ccId = Number(rawId);
            if (!Number.isInteger(ccId) || ccId < 0 || !rawRule || typeof rawRule !== "object") continue;
            const value = rawRule as Partial<DebuffAlertRule>;
            const storedSoundMode = sanitizeSoundMode(value.soundMode ?? legacySoundMode);
            rules[ccId] = {
                ccId,
                enabled: value.enabled !== false,
                warningSeconds: clamp(value.warningSeconds, 1, 3600, 20),
                flashEnabled: value.flashEnabled !== false,
                order: clamp(value.order, 0, 10000, index),
                soundEnabled: typeof value.soundEnabled === "boolean" ? value.soundEnabled : storedSoundMode !== "none",
                soundMode: storedSoundMode === "none" ? "electronic" : storedSoundMode,
                customSoundId: sanitizeText(value.customSoundId ?? parsed.customSoundId, 128),
                customSoundName: sanitizeText(value.customSoundName ?? parsed.customSoundName, 180),
            };
        }
        return {
            overlayEnabled: parsed.overlayEnabled !== false,
            firstRoundGraceSeconds: clamp(parsed.firstRoundGraceSeconds, 0, 120, 10),
            iconSize: clamp(parsed.iconSize, 16, 80, 30),
            volume: clamp(parsed.volume, 0, 100, 80),
            forcedBossRaceIds: sanitizeRaceIds(parsed.forcedBossRaceIds),
            soundMode: sanitizeSoundMode(parsed.soundMode),
            customSoundId: sanitizeText(parsed.customSoundId, 128),
            customSoundName: sanitizeText(parsed.customSoundName, 180),
            rules,
        };
    } catch {
        return fallback;
    }
}

export function saveDebuffAlertSettings(settings: DebuffAlertSettings) {
    settings.overlayEnabled = settings.overlayEnabled !== false;
    settings.firstRoundGraceSeconds = clamp(settings.firstRoundGraceSeconds, 0, 120, 10);
    settings.iconSize = clamp(settings.iconSize, 16, 80, 30);
    settings.volume = clamp(settings.volume, 0, 100, 80);
    settings.forcedBossRaceIds = sanitizeRaceIds(settings.forcedBossRaceIds);
    settings.soundMode = sanitizeSoundMode(settings.soundMode);
    settings.customSoundId = sanitizeText(settings.customSoundId, 128);
    settings.customSoundName = sanitizeText(settings.customSoundName, 180);
    Object.values(settings.rules)
        .sort((a, b) => a.order - b.order || a.ccId - b.ccId)
        .forEach((rule, index) => {
            rule.warningSeconds = clamp(rule.warningSeconds, 1, 3600, 20);
            rule.flashEnabled = rule.flashEnabled !== false;
            rule.order = index;
            rule.soundEnabled = rule.soundEnabled !== false;
            const soundMode = sanitizeSoundMode(rule.soundMode);
            rule.soundMode = soundMode === "none" ? "electronic" : soundMode;
            rule.customSoundId = sanitizeText(rule.customSoundId, 128);
            rule.customSoundName = sanitizeText(rule.customSoundName, 180);
        });
    localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
    window.dispatchEvent(new CustomEvent("dilmeter-debuff-alert-settings", { detail: settings }));
}

function sanitizeSoundMode(value: unknown): DebuffSoundMode {
    return value === "none" || value === "voice" || value === "custom" ? value : "electronic";
}

function sanitizeText(value: unknown, maxLength: number): string {
    return typeof value === "string"
        ? value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength)
        : "";
}

function sanitizeRaceIds(value: unknown): number[] {
    if (!Array.isArray(value)) return [];
    return [...new Set(value
        .map(Number)
        .filter((id) => Number.isInteger(id) && id > 0 && id <= 0x7fffffff))]
        .slice(0, 100);
}

function clamp(value: unknown, min: number, max: number, fallback: number): number {
    const number = Number(value);
    return Number.isFinite(number) ? Math.min(max, Math.max(min, Math.round(number))) : fallback;
}
