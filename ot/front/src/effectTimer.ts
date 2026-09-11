export type EffectTimerSourceType = "skill" | "condition";
export type EffectTimerTargetMode = "self" | "monster";
export type EffectTimerOrientation = "horizontal" | "vertical";

export interface EffectTimerRule {
    key: string;
    enabled: boolean;
    sourceType: EffectTimerSourceType;
    sourceId: number;
    name: string;
    durationSeconds: number;
    alwaysVisible: boolean;
    targetMode: EffectTimerTargetMode;
    orientation: EffectTimerOrientation;
    scalePercent: number;
    opacityPercent: number;
    x: number;
    y: number;
}

export interface EffectTimerSettings {
    rules: Record<string, EffectTimerRule>;
}

export interface EffectTimerRuntime {
    startedAtMs: number;
    endsAtMs: number;
    generation: number;
    targetId: string;
}

export interface EffectTimerOverlayItem extends EffectTimerRuntime, EffectTimerRule {
    iconUrl: string;
}

export const EFFECT_TIMER_CONDITION_EVENT = "dilmeter-effect-timer-condition";
export const EFFECT_TIMER_STORAGE_KEY = "dilmeter-cn-effect-timer-v1";

export function loadEffectTimerSettings(): EffectTimerSettings {
    try {
        return normalizeEffectTimerSettings(JSON.parse(localStorage.getItem(EFFECT_TIMER_STORAGE_KEY) || "null"));
    } catch {
        return { rules: {} };
    }
}

export function saveEffectTimerSettings(settings: EffectTimerSettings): void {
    const normalized = normalizeEffectTimerSettings(settings);
    settings.rules = normalized.rules;
    localStorage.setItem(EFFECT_TIMER_STORAGE_KEY, JSON.stringify(normalized));
}

export function normalizeEffectTimerSettings(value: unknown): EffectTimerSettings {
    const source = value && typeof value === "object" && "rules" in value
        ? (value as { rules?: unknown }).rules
        : undefined;
    const rules: Record<string, EffectTimerRule> = {};
    if (!source || typeof source !== "object") return { rules };
    for (const [rawKey, rawRule] of Object.entries(source as Record<string, unknown>)) {
        if (!rawRule || typeof rawRule !== "object") continue;
        const rule = rawRule as Partial<EffectTimerRule>;
        const key = sanitizeText(rule.key || rawKey, 80);
        if (!key) continue;
        rules[key] = {
            key,
            enabled: rule.enabled !== false,
            sourceType: rule.sourceType === "skill" ? "skill" : "condition",
            sourceId: clampInteger(rule.sourceId, 1, 4_294_967_295, 1),
            name: sanitizeText(rule.name, 80) || "未命名效果",
            durationSeconds: clampNumber(rule.durationSeconds, 0.1, 86_400, 10),
            alwaysVisible: rule.alwaysVisible === true,
            targetMode: rule.targetMode === "monster" ? "monster" : "self",
            orientation: rule.orientation === "vertical" ? "vertical" : "horizontal",
            scalePercent: clampInteger(rule.scalePercent, 50, 200, 100),
            opacityPercent: clampInteger(rule.opacityPercent, 20, 100, 100),
            x: clampInteger(rule.x, -32_000, 32_000, 600),
            y: clampInteger(rule.y, -32_000, 32_000, 260),
        };
    }
    return { rules };
}

export function makeEffectTimerRule(index = 0): EffectTimerRule {
    const now = Date.now();
    return {
        key: `effect-${now}-${Math.random().toString(36).slice(2, 8)}`,
        enabled: true,
        sourceType: "condition",
        sourceId: 1,
        name: "未命名效果",
        durationSeconds: 10,
        alwaysVisible: false,
        targetMode: "self",
        orientation: "horizontal",
        scalePercent: 100,
        opacityPercent: 100,
        x: 600 + (index % 3) * 320,
        y: 260 + Math.floor(index / 3) * 90,
    };
}

function sanitizeText(value: unknown, maxLength: number): string {
    return typeof value === "string"
        ? value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength)
        : "";
}

function clampInteger(value: unknown, min: number, max: number, fallback: number): number {
    return Math.round(clampNumber(value, min, max, fallback));
}

function clampNumber(value: unknown, min: number, max: number, fallback: number): number {
    const numeric = Number(value);
    return Number.isFinite(numeric) ? Math.min(max, Math.max(min, numeric)) : fallback;
}
