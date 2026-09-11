import type { EntityCondition } from "@/eventActor";

export type BuffDurationMode = "auto" | "manual";
export type BuffSoundMode = "none" | "electronic" | "voice" | "custom";

export interface BuffAlertRule {
    ccId: number;
    overlayEnabled: boolean;
    durationMode: BuffDurationMode;
    manualDurationSeconds: number;
    flashEnabled: boolean;
    flashThresholdSeconds: number;
    soundMode: BuffSoundMode;
    soundThresholdSeconds: number;
    customSoundId: string;
    customSoundName: string;
    /** Optional stack-threshold alert for verified stack-bearing conditions. */
    stackAlertEnabled: boolean;
    stackThreshold: number;
    stackScreenEnabled: boolean;
    /** Independent desktop position for the stack-threshold prompt. */
    stackX: number;
    stackY: number;
    stackSoundMode: BuffSoundMode;
    stackCustomSoundId: string;
    stackCustomSoundName: string;
}

export interface BuffOverlaySettings {
    locked: boolean;
    iconSize: number;
    volume: number;
    /** 0 selects the native overlay window's current monitor DPI; positive values are manual percentages. */
    dpiPercent: number;
    /** Master switch for both desktop overlay windows. Reset to on when the app starts. */
    overlayEnabled: boolean;
    /** Visual opacity shared by Buff and skill-cooldown overlays. */
    opacity: number;
    /** User-controlled whole-second correction applied to every Buff timer. */
    timeAdjustmentSeconds: number;
    rules: Record<number, BuffAlertRule>;
}

export interface BuffOverlayItem {
    ccId: number;
    name: string;
    iconUrl: string;
    appliedAt: number;
    expiresAt: number | null;
    flashEnabled: boolean;
    flashThresholdSeconds: number;
    active?: boolean;
    preview?: boolean;
    kind?: "buff" | "debuff";
    state?: "active" | "expiring" | "missing";
}

export type BuffOverlayMessage = {
    type: "buff-state";
    at: number;
    items: BuffOverlayItem[];
    settings?: Pick<BuffOverlaySettings, "locked" | "iconSize" | "dpiPercent" | "overlayEnabled" | "opacity">;
};

export const BUFF_ALERT_STORAGE_KEY = "dilmeter-cn-buff-alert-v1";
export const BUFF_OVERLAY_CHANNEL = "dilmeter-cn-buff-overlay-v1";
export const STACK_ONLY_BUFF_ALERT_CC_IDS = new Set([1080, 1098, 1153]);

const DOTNET_UNIX_EPOCH_MS = 62135596800000;
const CHINA_UTC_OFFSET_MS = 8 * 60 * 60 * 1000;
let conditionServerClockLeadMs = 10000;

export const DEFAULT_BUFF_ALERT_RULE: Omit<BuffAlertRule, "ccId"> = {
    overlayEnabled: false,
    durationMode: "auto",
    manualDurationSeconds: 60,
    flashEnabled: true,
    flashThresholdSeconds: 10,
    soundMode: "none",
    soundThresholdSeconds: 5,
    customSoundId: "",
    customSoundName: "",
    stackAlertEnabled: false,
    stackThreshold: 5,
    stackScreenEnabled: true,
    stackX: 850,
    stackY: 280,
    stackSoundMode: "none",
    stackCustomSoundId: "",
    stackCustomSoundName: "",
};

export function makeBuffAlertRule(ccId: number): BuffAlertRule {
    const rule = { ccId, ...DEFAULT_BUFF_ALERT_RULE };
    if (isStackOnlyBuffAlertCondition(ccId)) {
        rule.stackAlertEnabled = true;
        rule.stackScreenEnabled = true;
        rule.overlayEnabled = false;
        rule.flashEnabled = false;
        rule.soundMode = "none";
    }
    return rule;
}

export function isStackOnlyBuffAlertCondition(ccId: number): boolean {
    return STACK_ONLY_BUFF_ALERT_CC_IDS.has(Number(ccId));
}

export function loadBuffOverlaySettings(): BuffOverlaySettings {
    const fallback: BuffOverlaySettings = {
        locked: true,
        iconSize: 20,
        volume: 80,
        dpiPercent: 0,
        overlayEnabled: true,
        opacity: 100,
        timeAdjustmentSeconds: 0,
        rules: {},
    };
    try {
        const raw = localStorage.getItem(BUFF_ALERT_STORAGE_KEY);
        if (!raw) return fallback;
        const parsed = JSON.parse(raw) as Partial<BuffOverlaySettings>;
        const rules: Record<number, BuffAlertRule> = {};
        if (parsed.rules && typeof parsed.rules === "object") {
            for (const [rawId, rawRule] of Object.entries(parsed.rules)) {
                const ccId = Number(rawId);
                if (!Number.isInteger(ccId) || ccId < 0 || !rawRule || typeof rawRule !== "object") continue;
                const value = rawRule as Partial<BuffAlertRule>;
                rules[ccId] = sanitizeBuffAlertRule(ccId, value);
            }
        }
        return {
            locked: parsed.locked !== false,
            iconSize: clampNumber(parsed.iconSize, 16, 80, 20),
            volume: clampNumber(parsed.volume, 0, 100, 80),
            dpiPercent: sanitizeOverlayDpiPercent(parsed.dpiPercent),
            overlayEnabled: parsed.overlayEnabled !== false,
            opacity: clampNumber(parsed.opacity, 20, 100, 100),
            timeAdjustmentSeconds: clampNumber(parsed.timeAdjustmentSeconds, -3600, 3600, 0),
            rules,
        };
    } catch {
        return fallback;
    }
}

export function saveBuffOverlaySettings(settings: BuffOverlaySettings) {
    settings.locked = settings.locked !== false;
    settings.iconSize = clampNumber(settings.iconSize, 16, 80, 20);
    settings.volume = clampNumber(settings.volume, 0, 100, 80);
    settings.dpiPercent = sanitizeOverlayDpiPercent(settings.dpiPercent);
    settings.overlayEnabled = settings.overlayEnabled !== false;
    settings.opacity = clampNumber(settings.opacity, 20, 100, 100);
    settings.timeAdjustmentSeconds = clampNumber(settings.timeAdjustmentSeconds, -3600, 3600, 0);
    for (const [rawId, rule] of Object.entries(settings.rules)) {
        const ccId = Number(rawId);
        if (!Number.isInteger(ccId) || ccId < 0) continue;
        settings.rules[ccId] = sanitizeBuffAlertRule(ccId, rule);
    }
    localStorage.setItem(BUFF_ALERT_STORAGE_KEY, JSON.stringify(settings));
    window.dispatchEvent(new CustomEvent("dilmeter-buff-alert-settings", { detail: settings }));
}

/** Resolve preview scale; the desktop host resolves auto mode from the native window DPI. */
export function resolveOverlayDpiPercent(configuredPercent: unknown, devicePixelRatio?: number): number {
    const configured = sanitizeOverlayDpiPercent(configuredPercent);
    if (configured > 0) return configured;
    const ratio = Number(devicePixelRatio ?? globalThis.devicePixelRatio ?? 1);
    if (!Number.isFinite(ratio) || ratio <= 0) return 100;
    return clampNumber(ratio * 100, 50, 500, 100);
}

function sanitizeOverlayDpiPercent(value: unknown): number {
    const number = Number(value);
    if (!Number.isFinite(number) || number <= 0) return 0;
    return clampNumber(number, 50, 500, 100);
}

export function sanitizeBuffAlertRule(ccId: number, value: Partial<BuffAlertRule>): BuffAlertRule {
    const stackOnly = isStackOnlyBuffAlertCondition(ccId);
    return {
        ccId,
        overlayEnabled: stackOnly ? false : Boolean(value.overlayEnabled),
        durationMode: value.durationMode === "manual" ? "manual" : "auto",
        manualDurationSeconds: clampNumber(value.manualDurationSeconds, 1, 86400, 60),
        flashEnabled: stackOnly ? false : value.flashEnabled !== false,
        flashThresholdSeconds: clampNumber(value.flashThresholdSeconds, 1, 3600, 10),
        soundMode: !stackOnly && (value.soundMode === "electronic" || value.soundMode === "voice" || value.soundMode === "custom")
            ? value.soundMode
            : "none",
        soundThresholdSeconds: clampNumber(value.soundThresholdSeconds, 1, 3600, 5),
        customSoundId: sanitizeSoundText(value.customSoundId, 128),
        customSoundName: sanitizeSoundText(value.customSoundName, 180),
        stackAlertEnabled: stackOnly
            ? value.stackAlertEnabled === true || Boolean(value.overlayEnabled)
            : Boolean(value.stackAlertEnabled),
        stackThreshold: clampNumber(value.stackThreshold, 1, 99, 5),
        stackScreenEnabled: value.stackScreenEnabled !== false,
        stackX: clampNumber(value.stackX, -32000, 32000, 850),
        stackY: clampNumber(value.stackY, -32000, 32000, 280),
        stackSoundMode: value.stackSoundMode === "electronic" || value.stackSoundMode === "voice" || value.stackSoundMode === "custom"
            ? value.stackSoundMode
            : "none",
        stackCustomSoundId: sanitizeSoundText(value.stackCustomSoundId, 128),
        stackCustomSoundName: sanitizeSoundText(value.stackCustomSoundName, 180),
    };
}

/** Extract the server-provided stack count (for example MCSTCT:2:7). */
export function parseConditionStack(metadata: string | undefined): number | null {
    if (!metadata) return null;
    const patterns = [
        /(?:^|[^A-Z0-9_])MCSTCT\s*:\s*\d+\s*:\s*(-?\d+)/i,
        /["']MCSTCT["']\s*:\s*(-?\d+)/i,
        /<MCSTCT>\s*(-?\d+)\s*<\/MCSTCT>/i,
    ];
    for (const pattern of patterns) {
        const value = Number(pattern.exec(metadata)?.[1]);
        if (Number.isInteger(value) && value >= 0 && value <= 999) return value;
    }
    return null;
}

function sanitizeSoundText(value: unknown, maxLength: number): string {
    if (typeof value !== "string") return "";
    return value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength);
}

export function resolveBuffExpiresAt(
    condition: EntityCondition,
    rule?: BuffAlertRule,
    timeAdjustmentSeconds = 0,
): number | null {
    const adjustment = clampNumber(timeAdjustmentSeconds, -3600, 3600, 0);
    if (rule?.durationMode === "manual") {
        return condition.At + clampNumber(rule.manualDurationSeconds, 1, 86400, 60) + adjustment;
    }

    // New captures retain the exact millisecond expiry. This prevents a
    // 300-second Buff from becoming 300.x seconds after the backend rounds an
    // integer DisableAt upward, which the UI would correctly display as 301.
    const disableAtMs = Number(condition.DisableAtMs);
    const preciseDeltaMs = disableAtMs - condition.At * 1000;
    if (Number.isFinite(disableAtMs) && preciseDeltaMs > 0 && preciseDeltaMs <= 7 * 24 * 60 * 60 * 1000) {
        return disableAtMs / 1000 + adjustment;
    }

    // DisableAt is the actual scheduled end time. Prefer it over a full Buff
    // duration because packets can be observed several seconds after MCAGT.
    const disableAt = Number(condition.DisableAt);
    const delta = disableAt - condition.At;
    if (Number.isFinite(disableAt) && delta > 0 && delta <= 7 * 24 * 60 * 60) {
        return disableAt + adjustment;
    }

    // Older builds misread typed values such as SDUR:4:300000 as 4 ms. Parse
    // the retained raw metadata first so imported logs repair themselves.
    const metadataDurationMs = parseDurationMs(condition.Metadata, condition.At);
    const reportedDurationMs = Number(condition.DurationMs);
    const durationMs = metadataDurationMs > 0
        ? metadataDurationMs
        : reportedDurationMs >= 100 ? reportedDurationMs : 0;
    if (durationMs > 0 && durationMs <= 7 * 24 * 60 * 60 * 1000) {
        return condition.At + durationMs / 1000 + adjustment;
    }

    return null;
}

export function parseDurationMs(metadata: string | undefined, observedAtSeconds?: number): number {
    if (!metadata) return 0;
    const patterns = [
        /(?:^|[^A-Z0-9_])DUR["']?\s*[=:]\s*\d+\s*:\s*(\d+)/i,
        /(?:^|[^A-Z0-9_])DUR["']?\s*[=:]\s*["']?(\d+)/i,
        /<DUR>\s*(\d+)\s*<\/DUR>/i,
        /(?:^|[^A-Z0-9_])SDUR["']?\s*[=:]\s*\d+\s*:\s*(\d+)/i,
        /(?:^|[^A-Z0-9_])SDUR["']?\s*[=:]\s*["']?(\d+)/i,
        /<SDUR>\s*(\d+)\s*<\/SDUR>/i,
    ];
    for (const pattern of patterns) {
        const value = Number(pattern.exec(metadata)?.[1] ?? 0);
        if (Number.isFinite(value) && value > 0) return value;
    }

    let appliedAt = Number(/(?:^|[^A-Z0-9_])MCAGT:8:(\d+)/i.exec(metadata)?.[1] ?? 0);
    let endAt = Number(/(?:^|[^A-Z0-9_])SBT:8:(\d+)/i.exec(metadata)?.[1] ?? 0);
    if (appliedAt > 1e15) appliedAt /= 10000;
    if (endAt > 1e15) endAt /= 10000;
    if (Number.isFinite(appliedAt) && Number.isFinite(endAt)) {
        if (appliedAt > 0 && Number.isFinite(observedAtSeconds)) {
            const lead = appliedAt - DOTNET_UNIX_EPOCH_MS - CHINA_UTC_OFFSET_MS
                - Number(observedAtSeconds) * 1000;
            if (lead >= -60000 && lead <= 60000) conditionServerClockLeadMs = lead;
        }
        let duration = endAt - appliedAt;
        const maxDurationMs = 7 * 24 * 60 * 60 * 1000;
        if (duration > maxDurationMs && duration % 10000 === 0) duration /= 10000;
        if (duration > 0 && duration <= maxDurationMs) return duration;
    }

    // Some CN Buffs (including astrology cards) only carry SBT. SBT is a
    // timezone-less .NET millisecond timestamp in China Standard Time (UTC+8).
    // Derive the remaining duration from the event capture time so imported
    // logs made by older builds can also recover their countdown.
    if (Number.isFinite(endAt) && endAt > 0 && Number.isFinite(observedAtSeconds)) {
        const endUnixMs = endAt - DOTNET_UNIX_EPOCH_MS - CHINA_UTC_OFFSET_MS;
        const duration = endUnixMs - Number(observedAtSeconds) * 1000 - conditionServerClockLeadMs;
        const maxDurationMs = 7 * 24 * 60 * 60 * 1000;
        if (duration > 0 && duration <= maxDurationMs) return duration;
    }
    return 0;
}

function clampNumber(value: unknown, min: number, max: number, fallback: number): number {
    const number = Number(value);
    if (!Number.isFinite(number)) return fallback;
    return Math.min(max, Math.max(min, Math.round(number)));
}
