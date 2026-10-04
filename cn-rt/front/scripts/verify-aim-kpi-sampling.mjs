import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import ts from 'typescript';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)),
    logLevel: 'error', server: { middlewareMode: true, hmr: false }, optimizeDeps: { noDiscovery: true, include: [] } });
let cooldown;
try { cooldown = await server.ssrLoadModule('/src/skillCooldown.ts'); }
finally { await server.close(); }

// Execute the real component handlers with controlled event inputs.
const source = await readFile(new URL('../src/components/GameDpsReport.vue', import.meta.url), 'utf8');
const handlers = source.slice(source.indexOf('function handleSkillState('), source.indexOf('function skillActionIsPet('));
const settings = cooldown.loadSkillCooldownSettings();
settings.aimReminder.enabled = false;
const manager = { localEntityId: 'self', kpiAimSamples: [] };
const ctx = vm.createContext({ ...cooldown, actorManager: { value: manager },
    skillCooldownSettings: { value: settings }, finalShotActive: { value: false },
    activeMagnumAimCycle: { value: null }, recentMagnumAimCycle: { value: null },
    lastMagnumAimActionKey: '', findLocalBuffCondition: () => undefined });
vm.runInContext(ts.transpileModule(handlers, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText, ctx);
const start = (atMs, extra = {}) => ctx.handleSkillState({ detail: { Id: 'self', Scope: 'aim', SkillId: 21002, Active: true, AtMs: atMs, TargetId: 'boss', ...extra } });
const shot = (atMs, extra = {}) => ctx.observeMagnumAimShot({ Id: 'self', SkillId: 21002, AtMs: atMs, CombatActionId: atMs, ...extra });
start(1000);
assert.ok(ctx.activeMagnumAimCycle.value, 'overlay disabled must not suppress KPI aim starts');
const readyAt = ctx.activeMagnumAimCycle.value.readyAtMs;
shot(readyAt);
assert.equal(manager.kpiAimSamples.length, 1);
assert.equal(manager.kpiAimSamples[0].rate, .85);
assert.equal(manager.kpiAimSamples[0].targetId, 'boss');
shot(readyAt);
assert.equal(manager.kpiAimSamples.length, 1, 'duplicate releases are ignored');
start(5000);
shot(5500, { IsFallback: true });
shot(5500, { Id: 'teammate' });
shot(5500, { SourceId: 'pet' });
assert.equal(manager.kpiAimSamples.length, 1, 'fallback, teammate and pet messages cannot create local aim samples');
ctx.handleSkillState({ detail: { Id: 'self', Scope: 'aim', SkillId: 21002, Active: false, AtMs: 5600 } });
shot(5650);
assert.equal(manager.kpiAimSamples.length, 2, 'release arriving after aim end retains the sample');
start(9000, { Id: 'teammate' });
assert.equal(ctx.activeMagnumAimCycle.value, null);
assert.equal(settings.aimReminder.enabled, false, 'sampling never enables the overlay');
settings.aimReminder.enabled = true;
start(10000); shot(11000);
assert.equal(manager.kpiAimSamples.length, 3, 'enabled reminder still samples');
assert.match(source, /const aimReminder = aimSettings.enabled &&/, 'overlay publishing remains opt-in');
console.log('Aim KPI sampling verified with reminders off/on, duplicate and foreign events, delayed releases, and unchanged overlay opt-in.');
