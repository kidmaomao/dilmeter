import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const values = new Map();
globalThis.localStorage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)) };
const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)), logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] } });
try {
 const { normalizeBurstSettings, loadBurstSettings, saveBurstSettings } = await server.ssrLoadModule('/src/burstReminder.ts');
 const defaults = loadBurstSettings();
 assert.equal(defaults.enabled, false);
 assert.equal(defaults.rules[59005].ccId, 803); assert.equal(defaults.rules[58014].ccId, 516);
 assert.equal(defaults.rules[59005].castSeconds, 2); assert.equal(defaults.rules[58014].castSeconds, 5);
 assert.equal(defaults.rules[58014].cooldownSeconds, 0);
 assert.equal(normalizeBurstSettings({ rules: { 58014: { cooldownSeconds: 360, ready: { enabled: true } } } }).rules[58014].ready.enabled, false, 'legacy power cooldown must stay removed');
 const settings = normalizeBurstSettings({ enabled: true, includeSelf: false, includeTeammates: true, volume: 0, rules: { 59005: { ccId: 99, castSeconds: NaN, cooldownSeconds: 12.5, orientation: 'vertical', cast: { enabled: true, x: -120, y: 320, scalePercent: 175, soundEnabled: false }, ready: { enabled: false }, effect: { scalePercent: 999 } } } });
 assert.equal(settings.rules[59005].ccId, 803, 'source IDs stay authoritative');
 assert.equal(settings.rules[59005].castSeconds, 2);
 assert.equal(settings.rules[59005].effect.scalePercent, 175);
 assert.equal(settings.rules[59005].effect.x, -120); assert.equal(settings.rules[59005].ready.y, 320);
 assert.equal(settings.volume, 0); assert.equal(settings.includeSelf, false);
 saveBurstSettings(settings);
 assert.deepEqual(loadBurstSettings(), settings, 'independent phase controls survive saving/reloading');
 const clone = normalizeBurstSettings(settings); clone.rules[59005].cast.x = 900;
 assert.equal(settings.rules[59005].cast.x, -120, 'profiles/previews cannot share mutable phase settings');
 console.log('Burst settings: legacy defaults, sources, phase controls, clamping, mute, orientation and persistence verified');
} finally { await server.close(); }
