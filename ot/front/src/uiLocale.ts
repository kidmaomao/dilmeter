import { ref, watch } from "vue";
import { Converter } from "opencc-js";

export type UiLocale = "zh-CN" | "zh-TW";
export type ResourceRegion = "cn" | "tw";
const LOCALE_KEY = "dilmeter-ui-locale-v1";
const REGION_KEY = "dilmeter-resource-region-v1";

function stored(key: string): string | null {
    try { return localStorage.getItem(key); } catch { return null; }
}

export const uiLocale = ref<UiLocale>(stored(LOCALE_KEY) === "zh-TW" ? "zh-TW" : "zh-CN");
export const resourceRegion = ref<ResourceRegion>(stored(REGION_KEY) === "tw" ? "tw" : "cn");

// Only call this at UI presentation boundaries. Packet names, IDs, editable
// user text and persisted reminder keys must keep their original values.
const traditional = Converter({ from: "cn", to: "tw" });
const simplified = Converter({ from: "tw", to: "cn" });
const translations = new Map<string, string>();
export function uiText(value: unknown): string {
    const text = value == null ? "" : String(value);
    if (uiLocale.value !== "zh-TW" || !/[\u3400-\u9fff]/.test(text)) return text;
    let result = translations.get(text);
    if (result === undefined) {
        result = traditional(text);
        if (translations.size >= 4000) translations.clear();
        translations.set(text, result);
    }
    return result;
}

// Search accepts either script, without rewriting the official TW names.
export function normalizeNameSearch(text: string): string {
    return simplified(text).toLowerCase();
}

// Vuetify renders option/header titles itself. Translate only presentation
// fields, retaining the original item values used by selection and sorting.
export function uiItems<T>(items: readonly T[] | undefined): T[] {
    return (items || []).map(item => {
        if (!item || typeof item !== "object") return item;
        const result = { ...item } as Record<string, unknown>;
        for (const field of ["title", "label", "text"]) {
            if (typeof result[field] === "string") result[field] = uiText(result[field]);
        }
        return result as T;
    });
}

export function setUiLocale(value: UiLocale): void {
    uiLocale.value = value === "zh-TW" ? "zh-TW" : "zh-CN";
    try { localStorage.setItem(LOCALE_KEY, uiLocale.value); } catch { /* session only */ }
}

export function saveResourceRegion(value: ResourceRegion): void {
    resourceRegion.value = value;
    try { localStorage.setItem(REGION_KEY, value); } catch { /* session only */ }
}

if (typeof window !== "undefined") {
    watch(uiLocale, value => { document.documentElement.lang = value; }, { immediate: true });
    window.addEventListener("storage", event => {
        if (event.key === LOCALE_KEY) uiLocale.value = event.newValue === "zh-TW" ? "zh-TW" : "zh-CN";
    });
}
