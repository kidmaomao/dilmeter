const DESKTOP_SERVER_URL = "http://127.0.0.1:8030/";

/**
 * WebView2 versions differ in whether the WebSocket constructor accepts a
 * relative URL. Always hand it an absolute ws:// or wss:// URL instead.
 */
export function resolveWebSocketUrl(input: string, pageHref?: string): string {
    const raw = String(input || "/ws").trim() || "/ws";
    if (/^wss?:\/\//i.test(raw)) return raw;

    const browserHref = typeof window !== "undefined" ? window.location.href : "";
    const candidateBase = String(pageHref || browserHref || "").trim();
    const base = /^https?:\/\//i.test(candidateBase) ? candidateBase : DESKTOP_SERVER_URL;

    try {
        const resolved = new URL(raw, base);
        resolved.protocol = resolved.protocol === "https:" ? "wss:" : "ws:";
        return resolved.toString();
    } catch {
        return new URL("/ws", DESKTOP_SERVER_URL).toString().replace(/^http:/, "ws:");
    }
}
