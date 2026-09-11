import { strict as assert } from "node:assert";
import { resolveWebSocketUrl } from "../src/lib/webSocketUrl.ts";

assert.equal(resolveWebSocketUrl("/ws", "http://127.0.0.1:8030/"), "ws://127.0.0.1:8030/ws");
assert.equal(resolveWebSocketUrl("/ws", "https://example.test/app"), "wss://example.test/ws");
assert.equal(resolveWebSocketUrl("ws://localhost:9000/ws", "http://ignored.test/"), "ws://localhost:9000/ws");
assert.equal(
    resolveWebSocketUrl("/ws", "file:///C:/DilmeterCN/index.html"),
    "ws://127.0.0.1:8030/ws",
    "a WebView without an HTTP origin must fall back to the local desktop server",
);

console.log("socket URL verification passed");
