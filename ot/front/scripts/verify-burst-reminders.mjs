import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const values = new Map();
globalThis.localStorage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, String(value)) };
const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)), logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] } });
try {
 const { normalizeBurstSettings, loadBurstSettings, saveBurstSettings } = await server.ssrLoadModule('/src/burstReminder.ts');
 const { createBurstPreview, burstPopupDimensions } = await server.ssrLoadModule('/src/burstReminderView.ts');
 const defaults = loadBurstSettings();
 for (const id of [59005,58014]) for (const phase of ['cast','effect','ready','cooldown','alert']) {
  const preview=createBurstPreview(defaults.rules[id],phase,100000);
  assert.equal(preview.endsAtMs,108000); assert.equal(preview.previewExpiresAtMs,108000);
 }
 const mixed=createBurstPreview(defaults.rules[59005],'alert',100000);
 assert.equal(mixed.readyActors[0].actorName,'队友 A'); assert.equal(mixed.actorName,'队友 B'); assert.equal(mixed.nextReadySoon,true);
 mixed.readyActors=Array.from({length:8},(_,i)=>({actorId:String(i),actorName:'队友'+i}));
 assert.ok(burstPopupDimensions(mixed).height>=80+16*9,'all ready rows fit without clipping');
 assert.equal(defaults.enabled, false);
 assert.equal(defaults.rules[59005].ccId, 803); assert.equal(defaults.rules[58014].ccId, 516);
 assert.equal(defaults.rules[59005].castSeconds, 2); assert.equal(defaults.rules[58014].castSeconds, 5);
 assert.equal(defaults.rules[58014].cooldownSeconds, 0);
 assert.equal(normalizeBurstSettings({ rules: { 58014: { cooldownSeconds: 360, ready: { enabled: true } } } }).rules[58014].ready.enabled, false, 'legacy power cooldown must stay removed');
 assert.equal(defaults.rules[59005].cooldownAlwaysVisible, false);
 assert.equal(defaults.rules[59005].cooldownAlertEnabled, true);
 assert.equal(defaults.rules[59005].cooldownLeadSeconds, 0, 'legacy settings only show readiness');
 for (const [value, expected] of [[-5, 0], [NaN, 0], [Infinity, 0], [999999, 86400], [2.5, 2.5]]) {
  const rule = normalizeBurstSettings({ rules: { 59005: { cooldownMode: 'bad', cooldownLeadSeconds: value } } }).rules[59005];
  assert.equal(rule.cooldownAlwaysVisible, false); assert.equal(rule.cooldownAlertEnabled, true); assert.equal(rule.cooldownLeadSeconds, expected);
 }
 const settings = normalizeBurstSettings({ enabled: true, includeSelf: false, includeTeammates: true, volume: 0, rules: { 59005: { cooldownMode: 'always', cooldownLeadSeconds: 2.5, ccId: 99, castSeconds: NaN, cooldownSeconds: 12.5, orientation: 'vertical', cast: { enabled: true, x: -120, y: 320, scalePercent: 175, soundEnabled: false }, ready: { enabled: false }, effect: { scalePercent: 999 } } } });
 assert.equal(settings.rules[59005].ccId, 803, 'source IDs stay authoritative');
 assert.equal(settings.rules[59005].castSeconds, 2);
 assert.equal(settings.rules[59005].effect.scalePercent, 175);
 assert.equal(settings.rules[59005].effect.x, -120); assert.equal(settings.rules[59005].ready.y, 320);
 assert.equal(settings.volume, 0); assert.equal(settings.includeSelf, false);
 assert.equal(settings.rules[59005].cooldownAlwaysVisible, true, 'migrate legacy always mode');
 assert.equal(settings.rules[59005].cooldownAlertEnabled, true, 'always-visible does not disable alerts');
 for (const always of [false, true]) for (const alert of [false, true]) {
  const value = normalizeBurstSettings({ rules: { 59005: { cooldownMode: 'always', cooldownAlwaysVisible: always, cooldownAlertEnabled: alert, cooldownLeadSeconds: 12.5 } } });
  saveBurstSettings(value); const restored = loadBurstSettings().rules[59005];
  assert.equal(restored.cooldownAlwaysVisible, always); assert.equal(restored.cooldownAlertEnabled, alert); assert.equal(restored.cooldownLeadSeconds, 12.5);
 }
 saveBurstSettings(settings);
 assert.deepEqual(loadBurstSettings(), settings, 'independent phase controls survive saving/reloading');
 const clone = normalizeBurstSettings(settings); clone.rules[59005].cast.x = 900;
 assert.equal(settings.rules[59005].cast.x, -120, 'profiles/previews cannot share mutable phase settings');
 console.log('Burst settings: legacy defaults, sources, phase controls, clamping, mute, orientation and persistence verified');
} finally { await server.close(); }
