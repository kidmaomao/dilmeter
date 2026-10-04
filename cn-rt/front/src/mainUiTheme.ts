import { UI_COLOR_THEMES } from "./uiColorTheme";

export type MainUiTheme = "light" | "dark";
const STORAGE_KEY = "dilmeter.mainUiTheme.v2";

// Main-window preferences are separate from the existing overlay palettes.
export function loadMainUiTheme(): MainUiTheme {
    try {
        const saved = localStorage.getItem(STORAGE_KEY);
        if (saved === "light" || saved === "dark") return saved;
        const legacy = UI_COLOR_THEMES.find(theme => theme.id === localStorage.getItem("dilmeter.uiColorTheme.v1"));
        return legacy && !legacy.isLight ? "dark" : "light";
    } catch { return "light"; }
}

export function saveMainUiTheme(value: unknown): MainUiTheme {
    const theme = value === "dark" ? "dark" : "light";
    try { localStorage.setItem(STORAGE_KEY, theme); } catch { /* retain the live preference */ }
    return theme;
}

export function mainUiThemeStyle(value: unknown): Record<string, string> {
    const dark = value === "dark";
    const palette = dark
        ? { canvas: "#303a58", surface: "#293147", control: "#242d42", raised: "#39415b", text: "#f4f5ff", muted: "#c5cee2", border: "#535e7c", controlBorder: "#8292b0", accent: "#c2c8ff", accentText: "#d3d8ff", onAccent: "#252947", selected: "#495983", hover: "#444e71", danger: "#ffb6c4", disabledText: "#b4bfd5", disabled: "#3b465f" }
        : { canvas: "#eaf2ff", surface: "#f9fcff", control: "#ffffff", raised: "#eff5ff", text: "#24334b", muted: "#50627e", border: "#cbd9ed", controlBorder: "#8095b4", accent: "#565ac4", accentText: "#414ba4", onAccent: "#ffffff", selected: "#e4e8ff", hover: "#e5edfc", danger: "#ac3550", disabledText: "#71829a", disabled: "#e6edf7" };
    const rgb = (hex: string) => [1, 3, 5].map(offset => parseInt(hex.slice(offset, offset + 2), 16)).join(", ");
    return {
        ...Object.fromEntries(["canvas", "surface", "control", "raised", "text", "muted", "border"].map(role => [`--ui-theme-${role}`, palette[role as keyof typeof palette]])),
        "--ui-color-accent": palette.accent,
        "--ui-color-rgb": rgb(palette.accent),
        "--ui-theme-accent-text": palette.accentText,
        "--ui-theme-on-accent": palette.onAccent,
        // Legacy selected controls pair this token with on-accent. Soft selections
        // have their own token and always use accent-text instead.
        "--ui-theme-selected": palette.accent,
        "--ui-theme-selection-soft": palette.selected,
        "--ui-theme-control-border": palette.controlBorder,
        "--ui-theme-disabled-text": palette.disabledText,
        "--ui-theme-disabled": palette.disabled,
        "--ui-glass-surface": dark ? "rgba(29, 34, 54, .42)" : "rgba(255, 255, 255, .68)",
        "--ui-glass-inner": dark ? "rgba(57, 64, 93, .22)" : "rgba(255, 255, 255, .64)",
        "--ui-glass-border": dark ? "rgba(211, 220, 255, .23)" : "rgba(255, 255, 255, .88)",
        "--ui-glass-shadow": dark ? "inset 0 1px 0 rgba(255, 255, 255, .12), 0 12px 30px rgba(11, 15, 35, .16)" : "0 8px 28px rgba(65, 89, 151, .08)",
        "--ui-backdrop-image": dark ? "url('/ui-glass-backdrop-dark.png')" : "url('/ui-glass-backdrop.png')",
        "--ui-tint-sky": dark ? "rgba(137, 161, 226, .14)" : "#d7edff",
        "--ui-tint-lilac": dark ? "rgba(166, 153, 226, .16)" : "#e2ddff",
        "--ui-tint-mint": dark ? "rgba(139, 161, 214, .14)" : "#d9f3e6",
        "--ui-tint-peach": dark ? "rgba(173, 153, 199, .16)" : "#ffe4db",
        "--ui-tint-rose": dark ? "rgba(166, 153, 226, .16)" : "#f6e0f0",
        "--ui-theme-surface-hover": palette.hover,
        "--ui-theme-border-soft": palette.border,
        "--ui-theme-inset": palette.control,
        "--ui-theme-panel": palette.raised,
        "--ui-theme-input": palette.control,
        "--ui-theme-text-muted": palette.muted,
        "--ui-color-accent-soft": palette.accentText,
        "--ui-theme-hover": palette.hover,
        "--ui-theme-danger": palette.danger,
        "--ui-color-scheme": dark ? "dark" : "light",
        "--v-theme-background": rgb(palette.canvas),
        "--v-theme-surface": rgb(palette.surface),
        "--v-theme-on-background": rgb(palette.text),
        "--v-theme-on-surface": rgb(palette.text),
        "--v-theme-primary": rgb(palette.accent),
        "--v-theme-on-primary": rgb(palette.onAccent),
    };
}
