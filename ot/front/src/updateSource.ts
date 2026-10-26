export type UpdateSourcePreference = "auto" | "github" | "gitee";
const storageKey = "dilmeter-update-source";

export function loadUpdateSource(): UpdateSourcePreference {
    try {
        const value = localStorage.getItem(storageKey);
        return value === "github" || value === "gitee" ? value : "auto";
    } catch { return "auto"; }
}

export function saveUpdateSource(value: UpdateSourcePreference): void {
    try { localStorage.setItem(storageKey, value); } catch { /* The selection still applies to this session. */ }
}

export function updateRequestURL(value: UpdateSourcePreference): string {
    return `/api/update?source=${encodeURIComponent(value)}`;
}

export function updateSourceLabel(value?: string): string {
    return value === "github" ? "GitHub" : value === "gitee" ? "Gitee" : value === "mirror" ? "下载站" : "";
}
