export interface SkillBarSlot {
    id: string;
    skillId: number;
    skillName: string;
    keyCode: string;
    keyLabel: string;
    keySequence: string[][];
    cooldownSeconds: number;
}

export interface SkillBarSettings {
    enabled: boolean;
    inputEnabled: boolean;
    clickButton: "left" | "right";
    stopMovement: boolean;
    stopKeyCode: string;
    locked: boolean;
    x: number;
    y: number;
    columns: number;
    iconSize: number;
    gap: number;
    opacity: number;
    slots: SkillBarSlot[];
}

export const SKILL_BAR_STORAGE_KEY = "dilmeter-cn-skill-bar-v1";
export const SKILL_BAR_CHANNEL = "dilmeter-cn-skill-bar-overlay-v1";
export const SKILL_BAR_SETTINGS_EVENT = "dilmeter-skill-bar-settings";

export function makeSkillBarSlot(index = 0): SkillBarSlot {
    return {
        id: makeSlotId(index),
        skillId: 0,
        skillName: "",
        keyCode: "",
        keyLabel: "",
        keySequence: [],
        cooldownSeconds: 30,
    };
}

export function defaultSkillBarSettings(): SkillBarSettings {
    return {
        enabled: false,
        inputEnabled: false,
        clickButton: "left",
        stopMovement: false,
        stopKeyCode: "",
        locked: true,
        x: 600,
        y: 520,
        columns: 10,
        iconSize: 48,
        gap: 3,
        opacity: 100,
        slots: Array.from({ length: 10 }, (_, index) => makeSkillBarSlot(index)),
    };
}

export function loadSkillBarSettings(): SkillBarSettings {
    try {
        const raw = localStorage.getItem(SKILL_BAR_STORAGE_KEY);
        if (!raw) return defaultSkillBarSettings();
        return normalizeSkillBarSettings(JSON.parse(raw));
    } catch {
        return defaultSkillBarSettings();
    }
}

export function saveSkillBarSettings(settings: SkillBarSettings): SkillBarSettings {
    const normalized = normalizeSkillBarSettings(settings);
    localStorage.setItem(SKILL_BAR_STORAGE_KEY, JSON.stringify(normalized));
    window.dispatchEvent(new CustomEvent(SKILL_BAR_SETTINGS_EVENT, { detail: normalized }));
    return normalized;
}

export function normalizeSkillBarSettings(value: Partial<SkillBarSettings> | undefined): SkillBarSettings {
    const fallback = defaultSkillBarSettings();
	const rawStopKeyCode = typeof value?.stopKeyCode === "string" ? value.stopKeyCode.trim() : "";
	const stopKeyCode = isSupportedSkillBarStopCode(rawStopKeyCode) ? rawStopKeyCode : "";
    const rawSlots = Array.isArray(value?.slots) ? value.slots.slice(0, 48) : fallback.slots;
    const slots = rawSlots.map((raw, index) => {
        const slot = raw as Partial<SkillBarSlot>;
        const legacyKeyCode = isSupportedSkillBarCode(slot.keyCode) ? String(slot.keyCode) : "";
        const keySequence = normalizeSkillBarKeySequence(slot.keySequence);
        if (!keySequence.length && legacyKeyCode) keySequence.push([legacyKeyCode]);
        const keyCode = keySequence.length === 1 && keySequence[0].length === 1 ? keySequence[0][0] : "";
        return {
            id: sanitizeText(slot.id, 80) || makeSlotId(index),
            skillId: clampInteger(slot.skillId, 0, 65535, 0),
            skillName: sanitizeText(slot.skillName, 120),
            keyCode,
            keyLabel: skillBarSequenceLabel(keySequence),
            keySequence,
            cooldownSeconds: clampNumber(slot.cooldownSeconds, 0.1, 86400, 30, 1),
        };
    });
    while (slots.length < 1) slots.push(makeSkillBarSlot(slots.length));
    return {
        enabled: value?.enabled === true,
        inputEnabled: value?.inputEnabled === true,
        clickButton: value?.clickButton === "right" ? "right" : "left",
        // This is deliberately opt-in. The reference overlay uses the same
        // short key tap, but its fixed W default can activate a CN skill bind.
        stopMovement: value?.stopMovement === true && stopKeyCode !== "",
        stopKeyCode,
        locked: value?.locked !== false,
        x: clampInteger(value?.x, -32000, 32000, fallback.x),
        y: clampInteger(value?.y, -32000, 32000, fallback.y),
        columns: clampInteger(value?.columns, 1, 12, fallback.columns),
        iconSize: clampInteger(value?.iconSize, 32, 80, fallback.iconSize),
        gap: clampInteger(value?.gap, 0, 12, fallback.gap),
        opacity: clampInteger(value?.opacity, 25, 100, fallback.opacity),
        slots,
    };
}

export function skillBarBounds(settings: SkillBarSettings) {
    const columns = Math.max(1, Math.min(settings.columns, settings.slots.length));
    const rows = Math.max(1, Math.ceil(settings.slots.length / columns));
    const padding = settings.locked ? 3 : 6;
    return {
        x: settings.x,
        y: settings.y,
        width: padding * 2 + columns * settings.iconSize + Math.max(0, columns - 1) * settings.gap,
        height: padding * 2 + rows * settings.iconSize + Math.max(0, rows - 1) * settings.gap,
    };
}

export function skillBarHasContent(settings: SkillBarSettings) {
    return settings.slots.some((slot) => slot.skillId > 0);
}

export function nativeSkillBarPayload(settings: SkillBarSettings) {
    const normalized = normalizeSkillBarSettings(settings);
    return {
        active: normalized.enabled && skillBarHasContent(normalized),
        locked: normalized.locked,
        inputEnabled: normalized.inputEnabled,
        clickButton: normalized.clickButton,
        stopMovement: normalized.stopMovement,
        stopKeyCode: normalized.stopKeyCode,
        ...skillBarBounds(normalized),
        columns: normalized.columns,
        iconSize: normalized.iconSize,
        gap: normalized.gap,
        opacity: normalized.opacity,
        slots: normalized.slots.map((slot) => ({
            skillId: slot.skillId,
            keyCode: slot.keyCode,
            keyLabel: slot.keyLabel,
            keySequence: slot.keySequence,
        })),
        sequence: Math.round((performance.timeOrigin + performance.now()) * 1000),
    };
}

export async function syncNativeSkillBarSettings(settings: SkillBarSettings) {
    const response = await fetch("/api/skill_bar", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(nativeSkillBarPayload(settings)),
    });
    if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
    return response.json() as Promise<{
        x: number;
        y: number;
        width: number;
        height: number;
    }>;
}

export function isSupportedSkillBarCode(value: unknown): boolean {
    const code = typeof value === "string" ? value : "";
    if (/^Key[A-Z]$/.test(code) || /^Digit[0-9]$/.test(code) || /^F(?:[1-9]|1[0-9]|2[0-4])$/.test(code)) return true;
    if (/^Numpad[0-9]$/.test(code)) return true;
    return new Set([
        "Escape", "Tab", "CapsLock", "Space", "Enter", "Backspace",
        "ShiftLeft", "ShiftRight", "ControlLeft", "ControlRight", "AltLeft", "AltRight",
        "Insert", "Delete", "Home", "End", "PageUp", "PageDown",
        "ArrowLeft", "ArrowUp", "ArrowRight", "ArrowDown",
        "Minus", "Equal", "BracketLeft", "BracketRight", "Backslash", "Semicolon",
        "Quote", "Backquote", "Comma", "Period", "Slash",
        "NumpadAdd", "NumpadSubtract", "NumpadMultiply", "NumpadDivide", "NumpadDecimal", "NumpadEnter",
    ]).has(code);
}

export function isSupportedSkillBarStopCode(value: unknown): boolean {
    const code = typeof value === "string" ? value.trim() : "";
    if ([
        "ShiftLeft", "ShiftRight", "ControlLeft", "ControlRight", "AltLeft", "AltRight", "CapsLock",
    ].includes(code)) return false;
    return isSupportedSkillBarCode(code);
}

export function skillBarKeyLabel(code: string): string {
    if (/^Key[A-Z]$/.test(code)) return code.slice(3);
    if (/^Digit[0-9]$/.test(code)) return code.slice(5);
    if (/^F(?:[1-9]|1[0-9]|2[0-4])$/.test(code)) return code;
    if (/^Numpad[0-9]$/.test(code)) return `Num${code.slice(6)}`;
    const labels: Record<string, string> = {
        Escape: "Esc", Tab: "Tab", CapsLock: "Caps", Space: "Space", Enter: "Enter", Backspace: "Back",
        ShiftLeft: "LShift", ShiftRight: "RShift", ControlLeft: "LCtrl", ControlRight: "RCtrl",
        AltLeft: "LAlt", AltRight: "RAlt", Insert: "Ins", Delete: "Del", Home: "Home", End: "End",
        PageUp: "PgUp", PageDown: "PgDn", ArrowLeft: "←", ArrowUp: "↑", ArrowRight: "→", ArrowDown: "↓",
        Minus: "-", Equal: "=", BracketLeft: "[", BracketRight: "]", Backslash: "\\",
        Semicolon: ";", Quote: "'", Backquote: "`", Comma: ",", Period: ".", Slash: "/",
        NumpadAdd: "Num+", NumpadSubtract: "Num-", NumpadMultiply: "Num*", NumpadDivide: "Num/",
        NumpadDecimal: "Num.", NumpadEnter: "Num↵",
    };
    return labels[code] ?? code;
}

export function normalizeSkillBarKeySequence(value: unknown): string[][] {
    if (!Array.isArray(value)) return [];
    const sequence: string[][] = [];
    for (const rawChord of value.slice(0, 8)) {
        if (!Array.isArray(rawChord)) continue;
        const chord: string[] = [];
        for (const rawCode of rawChord.slice(0, 4)) {
            const code = typeof rawCode === "string" ? rawCode.trim() : "";
            if (isSupportedSkillBarCode(code) && !chord.includes(code)) chord.push(code);
        }
        if (chord.length) sequence.push(chord);
    }
    return sequence;
}

export function skillBarChordLabel(chord: string[]): string {
    const modifierOrder = new Map([
        ["ControlLeft", 0], ["ControlRight", 0],
        ["ShiftLeft", 1], ["ShiftRight", 1],
        ["AltLeft", 2], ["AltRight", 2],
    ]);
    const modifierLabels = new Map([
        ["ControlLeft", "Ctrl"], ["ControlRight", "Ctrl"],
        ["ShiftLeft", "Shift"], ["ShiftRight", "Shift"],
        ["AltLeft", "Alt"], ["AltRight", "Alt"],
    ]);
    return chord
        .map((code, index) => ({ code, index, order: modifierOrder.get(code) ?? 10 }))
        .sort((left, right) => left.order - right.order || left.index - right.index)
        .map(({ code }) => modifierLabels.get(code) ?? skillBarKeyLabel(code))
        .join("+");
}

export function skillBarSequenceLabel(sequence: string[][]): string {
    return normalizeSkillBarKeySequence(sequence).map(skillBarChordLabel).join(" → ");
}

function makeSlotId(index: number) {
    if (typeof crypto !== "undefined" && typeof crypto.randomUUID === "function") return crypto.randomUUID();
    return `slot-${Date.now().toString(36)}-${index}-${Math.random().toString(36).slice(2, 9)}`;
}

function sanitizeText(value: unknown, maxLength: number): string {
    if (typeof value !== "string") return "";
    return value.replace(/[\u0000-\u001f\u007f]/g, "").trim().slice(0, maxLength);
}

function clampInteger(value: unknown, min: number, max: number, fallback: number): number {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) return fallback;
    return Math.min(max, Math.max(min, Math.round(numeric)));
}

function clampNumber(value: unknown, min: number, max: number, fallback: number, decimals: number): number {
    const numeric = Number(value);
    if (!Number.isFinite(numeric)) return fallback;
    const factor = 10 ** decimals;
    return Math.min(max, Math.max(min, Math.round(numeric * factor) / factor));
}
