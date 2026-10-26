import assert from 'node:assert/strict';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const server = await createServer({ configFile: false, root, logLevel: 'error',
    server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true },
    resolve: { alias: { '@': path.join(root, 'src') } } });
globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.window = { dispatchEvent() {} };
try {
    const { recordFrenzySignal, frenzyDurations } = await server.ssrLoadModule('/src/frenzyKpi.ts');
    const { buildArcanaKpi } = await server.ssrLoadModule('/src/arcanaKpi.ts');
    const { PureActorManager, PureDamageCollectorManager } = await server.ssrLoadModule('/src/worker/pureEventActor.ts');
    const { ActorManager } = await server.ssrLoadModule('/src/eventActor.ts');
    const { DamageCollectorManager } = await server.ssrLoadModule('/src/actionCollector.ts');
    const enable = (at, extra = {}) => ({ EventId: 4, Id: 'player', At: Math.floor(at), AtMs: at * 1000,
        CCId: 230, DisableAt: at + 38, DisableAtMs: (at + 38) * 1000, ...extra });
    const disable = (at, extra = {}) => ({ EventId: 5, Id: 'player', At: Math.floor(at), AtMs: at * 1000, CCId: 230, ...extra });
    const events = [enable(1.5), enable(3), disable(11.5), enable(12), disable(32),
        enable(35, { Id: 'other' }), disable(45, { Id: 'other' }),
        enable(50, { Snapshot: true }), disable(55), enable(60),
        { EventId: 2, Id: 'player', At: 65 }, disable(70), enable(80)];
    const manager = new PureActorManager(new PureDamageCollectorManager());
    manager.localEntityId = 'player';
    events.forEach(event => manager.onEvent(event));
    const result = frenzyDurations(manager.arcanaSignals, 'player', 0, 90);
    assert.deepEqual(result.samples.map(s => s.seconds), [10, 20]);
    assert.deepEqual(frenzyDurations(manager.arcanaSignals, 'player', 5, 90).samples.map(s => s.seconds), [20]);
    const live = new ActorManager(new DamageCollectorManager()); live.localEntityId = 'player';
    events.forEach(event => live.onEvent(event));
    assert.deepEqual(live.arcanaSignals, manager.arcanaSignals, 'live and worker preserve identical evidence');
    assert.deepEqual(frenzyDurations(JSON.parse(JSON.stringify(manager.arcanaSignals)), 'player', 0, 90), result);
    const report = buildArcanaKpi({ player: { id: 'player', conditionHistory: [] },
        boss: { id: 'boss', conditionHistory: [{ At: 0, List: [{ CCId: 277, At: 0, DisableAt: 90 }] }] },
        session: { startAt: 0, endAt: 89 }, jobName: '枪炮师', actions: [], arcanaSignals: manager.arcanaSignals });
    const row = report.rows.find(row => row.id === 'frenzy-duration');
    assert.equal(row.value, 15); assert.equal(row.samples, 2); assert.equal(row.status, 'measured');
    const legacy = []; [enable(1.5), disable(11.9, { AtMs: undefined })].forEach(e => recordFrenzySignal(e, legacy));
    assert.deepEqual(frenzyDurations(legacy, 'player', 0, 90).samples, [{ seconds: 9.5, estimated: true }]);
    const stale = []; [enable(1), disable(60)].forEach(e => recordFrenzySignal(e, stale));
    assert.equal(frenzyDurations(stale, 'player', 0, 90).samples.length, 0);
    const dead = []; [enable(1), { EventId: 6, Id: 'player', At: 6 }].forEach(e => recordFrenzySignal(e, dead));
    assert.equal(frenzyDurations(dead, 'player', 0, 90).samples[0].seconds, 5);
    if (process.env.DILMETER_FRENZY_LOG) {
        const replay = new PureActorManager(new PureDamageCollectorManager());
        for (const line of fs.readFileSync(process.env.DILMETER_FRENZY_LOG, 'utf8').trim().split('\n')) replay.onEvent(JSON.parse(line));
        const ids = new Set(replay.arcanaSignals.filter(s => s.Signal === 'frenzy-state' && s.Count === 1).map(s => s.Id));
        let total = 0;
        for (const id of ids) {
            const samples = frenzyDurations(replay.arcanaSignals, id, 0, Number.MAX_SAFE_INTEGER).samples;
            total += samples.length;
            console.log('Capture Frenzy:', samples.length, 'complete samples;', samples.map(s => s.seconds.toFixed(3)).join(', '));
        }
        assert.ok(total > 10, 'capture must produce complete, non-fabricated durations');
    }
    console.log('Frenzy duration: refresh, early end, snapshots, reset, death, unfinished casts, worker and replay verified');
} finally { await server.close(); }
