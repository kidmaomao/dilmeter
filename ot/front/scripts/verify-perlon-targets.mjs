import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)), logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true } });
const store = new Map();
globalThis.localStorage = { getItem: key => store.get(key) ?? null, setItem: (key, value) => store.set(key, value) };
try {
    const { normalizeBossMechanicAlertSettings: normalize, saveBossMechanicAlertSettings: save, loadBossMechanicAlertSettings: load } = await server.ssrLoadModule('/src/bossMechanicAlert.ts');
    for (const enabled of [false, true]) {
        const rule = normalize({ rules: { 'perlon-tornado': { enabled } } }).rules['perlon-tornado'];
        assert.deepEqual(rule.tornadoMonitoring, { self: enabled, teammates: enabled });
        assert.equal(rule.enabled, enabled);
    }
    assert.deepEqual(normalize(null).rules['perlon-tornado'].tornadoMonitoring, { self: false, teammates: false });
    for (const self of [false, true]) for (const teammates of [false, true]) {
        const settings = normalize({ rules: { 'perlon-tornado': { enabled: true, tornadoMonitoring: { self, teammates } }, 'perlon-resonance': { enabled: true } } });
        save(settings);
        const restored = load();
        assert.deepEqual(restored.rules['perlon-tornado'].tornadoMonitoring, { self, teammates });
        assert.equal(restored.rules['perlon-tornado'].enabled, self || teammates);
        assert.equal(restored.rules['perlon-resonance'].enabled, true);
        const profileRoundTrip = normalize(JSON.parse(JSON.stringify(restored)));
        assert.deepEqual(profileRoundTrip.rules['perlon-tornado'].tornadoMonitoring, { self, teammates });
    }
    console.log('PASS Perlon target defaults, legacy migration, four combinations, save/reload and profile round-trip.');
} finally { await server.close(); }
