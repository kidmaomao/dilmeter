export interface UiColorThemeOption {
    id: string;
    name: string;
    accent: string;
    canvas: string;
    surface: string;
    raised: string;
    control: string;
    border: string;
    text: string;
    muted: string;
    onAccent: string;
    isLight: boolean;
}

const STORAGE_KEY = "dilmeter.uiColorTheme.v1";

/** Complete UI palettes based on Mabinogi's fourteen interface-colour presets. */
export const UI_COLOR_THEMES: readonly UiColorThemeOption[] = [
    { id: "transparent-sky", name: "透明天空", accent: "#96e2f7", canvas: "#173d4f", surface: "#245b70", raised: "#34778e", control: "#1a485d", border: "#64b8d5", text: "#eefcff", muted: "#b7dbe7", onAccent: "#102f3b", isLight: false },
    { id: "electronics", name: "电子产品", accent: "#65b8dd", canvas: "#050606", surface: "#171917", raised: "#292c29", control: "#101210", border: "#5f625c", text: "#f0f1ef", muted: "#a8aaa5", onAccent: "#102d38", isLight: false },
    { id: "doll-pink", name: "人偶粉红", accent: "#fdbdd9", canvas: "#6b3448", surface: "#9a526d", raised: "#91465f", control: "#75384f", border: "#eba0bc", text: "#fff5f8", muted: "#e9c4d1", onAccent: "#4a1d2e", isLight: false },
    { id: "transparent-gray", name: "透明灰色", accent: "#aeb9a8", canvas: "#20231f", surface: "#343833", raised: "#4a5048", control: "#252824", border: "#70786b", text: "#f2f4ef", muted: "#b8bdb4", onAccent: "#20251e", isLight: false },
    { id: "fireworks", name: "烟火开发", accent: "#e4af0f", canvas: "#56171a", surface: "#84292c", raised: "#a93d3c", control: "#63191d", border: "#d25d50", text: "#fff2d0", muted: "#e4b89c", onAccent: "#361000", isLight: false },
    { id: "milky-white", name: "乳白色", accent: "#f1a95e", canvas: "#d5cec3", surface: "#eee9df", raised: "#fffaf0", control: "#c8bbaa", border: "#9c7f64", text: "#493b31", muted: "#77685c", onAccent: "#3a2510", isLight: true },
    { id: "pink", name: "粉红", accent: "#ebaab7", canvas: "#dfc4c4", surface: "#f3dede", raised: "#fff1f1", control: "#d8b8b8", border: "#c67f88", text: "#6a3038", muted: "#8e5c62", onAccent: "#4b1e25", isLight: true },
    { id: "neon-green", name: "氖绿色", accent: "#4aedcb", canvas: "#071d1b", surface: "#0c3430", raised: "#12524b", control: "#082522", border: "#15988a", text: "#ebfffb", muted: "#91cbc3", onAccent: "#063027", isLight: false },
    { id: "neon-pink", name: "氖粉红色", accent: "#ed4adc", canvas: "#210719", surface: "#3a0e2c", raised: "#5f1648", control: "#2a0920", border: "#a62a7d", text: "#fff0fb", muted: "#d7a0c8", onAccent: "#3b0a31", isLight: false },
    { id: "natural-green", name: "自然绿", accent: "#d9f45a", canvas: "#74890c", surface: "#a4bb29", raised: "#c4d946", control: "#829811", border: "#d7e653", text: "#101500", muted: "#35400a", onAccent: "#2d3700", isLight: true },
    { id: "golden-apricot", name: "金杏", accent: "#f6936c", canvas: "#c5935f", surface: "#e0b477", raised: "#f1c995", control: "#b9824d", border: "#f4d09a", text: "#4d2b14", muted: "#765039", onAccent: "#4a1f11", isLight: true },
    { id: "urban-man", name: "都市男人", accent: "#77b9d4", canvas: "#a9bcc0", surface: "#d4e0df", raised: "#edf3f1", control: "#9eb4ba", border: "#73a5b6", text: "#30454b", muted: "#5b7177", onAccent: "#18333d", isLight: true },
    { id: "deep-yellow", name: "深黄色", accent: "#f68762", canvas: "#b9ad53", surface: "#d7cc71", raised: "#eee18e", control: "#c9875e", border: "#f0df81", text: "#513719", muted: "#745d36", onAccent: "#4a2116", isLight: true },
    { id: "sea-blue", name: "海蓝色", accent: "#4fdbdd", canvas: "#91cdd7", surface: "#c4edf0", raised: "#e1f7f7", control: "#61b7d5", border: "#3aa6ce", text: "#165369", muted: "#3f7180", onAccent: "#103b43", isLight: true },
] as const;

export const DEFAULT_UI_COLOR_THEME = "electronics";

export function normalizeUiColorThemeId(value: unknown): string {
    return typeof value === "string" && UI_COLOR_THEMES.some((theme) => theme.id === value)
        ? value
        : DEFAULT_UI_COLOR_THEME;
}

export function loadUiColorTheme(): string {
    try {
        return normalizeUiColorThemeId(localStorage.getItem(STORAGE_KEY));
    } catch {
        return DEFAULT_UI_COLOR_THEME;
    }
}

export function saveUiColorTheme(value: unknown): string {
    const theme = normalizeUiColorThemeId(value);
    try { localStorage.setItem(STORAGE_KEY, theme); } catch { /* keep the live selection */ }
    return theme;
}

function hexRgb(value: string): string {
    const hex = value.replace(/^#/, "");
    return [0, 2, 4].map((offset) => Number.parseInt(hex.slice(offset, offset + 2), 16)).join(", ");
}

export function uiColorThemeStyle(value: unknown): Record<string, string> {
    const id = normalizeUiColorThemeId(value);
    const theme = UI_COLOR_THEMES.find((item) => item.id === id) ?? UI_COLOR_THEMES[0];
    return {
        "--ui-theme-canvas": theme.canvas,
        "--ui-theme-surface": theme.surface,
        "--ui-theme-raised": theme.raised,
        "--ui-theme-control": theme.control,
        "--ui-theme-border": theme.border,
        "--ui-theme-text": theme.text,
        "--ui-theme-muted": theme.muted,
        "--ui-color-accent": theme.accent,
        "--ui-color-rgb": hexRgb(theme.accent),
        "--ui-theme-on-accent": theme.onAccent,
        "--ui-color-scheme": theme.isLight ? "light" : "dark",
        "--v-theme-background": hexRgb(theme.canvas),
        "--v-theme-surface": hexRgb(theme.surface),
        "--v-theme-on-background": hexRgb(theme.text),
        "--v-theme-on-surface": hexRgb(theme.text),
        "--v-theme-primary": hexRgb(theme.accent),
        "--v-theme-on-primary": hexRgb(theme.onAccent),
    };
}
