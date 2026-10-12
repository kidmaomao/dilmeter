import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false }, optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.window = { dispatchEvent() {}, addEventListener() {} };
globalThis.document = { documentElement: { lang: '' }, createElement: () => ({}) };
try {
    const { buildArcanaKpi } = await server.ssrLoadModule("/src/arcanaKpi.ts");
    const { captureLanceMount, lanceMountedAt, lanceHeatSamples } = await server.ssrLoadModule("/src/lancerKpi.ts");
    const { buildEventSnapshot } = await server.ssrLoadModule("/src/worker/buildEventSnapshot.ts");
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
    const { createBattleRecord, parseBattleRecord } = await server.ssrLoadModule("/src/battleRecord.ts");
    const action = (skill, at, extra = {}) => ({ EventId: 10, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: skill, SubSkillId: 0, CombatActionId: skill + at * 1000, SourceId: "player", IsFallback: false, ...extra });
    const condition = { Id: "player", CCId: 975, At: 3, DisableAt: 10, DisableAtMs: 5500, Metadata: "" };
    const actor = (id, conditionHistory = [{ At: 0, List: [] }]) => ({ id, conditionHistory });
    const input = { player: actor("player"), boss: actor("boss", [{ At: 0, List: [] },
        { At: 10, List: [{ CCId: 277, At: 10, DisableAt: 20 }] }, { At: 20, List: [] }]),
        session: { startAt: 0, endAt: 30 }, actions: [], jobName: "爆裂骑士枪", lanceHeatGainMultiplier: 1.5 };
    const report = (actions, extra = {}) => buildArcanaKpi({ ...input, actions, ...extra });
    const row = (result, id) => result.rows.find((r) => r.id === id);
    const releases = [action(20017, 2, { LanceMounted: true }), action(59105, 4, { LanceMounted: true }),
        action(59106, 6, { LanceMounted: false }), action(59106, 30.8, { LanceMounted: true })];
    const mount = row(report([...releases, releases[0], action(20017, 15, { LanceMounted: false }),
        action(20017, 5, { Id: "other" }), action(20017, 5, { SourceId: "pet" }),
        action(59105, 4.4, { SubSkillId: 2 }), action(59105, 5, { IsFallback: true })]), "lance-mounted");
    assert.equal(mount.value, 75); assert.equal(mount.samples, 4);
    assert.equal(row(report(releases.map((a) => ({ ...a, LanceMounted: true }))), "lance-mounted").value, 100);
    assert.equal(row(report(releases.map((a) => ({ ...a, LanceMounted: false }))), "lance-mounted").value, 0);
    assert.equal(row(report([]), "lance-mounted").value, null);
    assert.equal(row(report([action(59105, 2, { IsFallback: true })]), "lance-mounted").status, "missing-data");
    assert.equal(row(report([action(20017, 2)], { player: actor("player", []) }), "lance-mounted").status, "missing-data");
    assert.equal(captureLanceMount(action(20017, 5.1), { conditionMap: { 975: condition } }).LanceMounted, true);
    assert.equal(captureLanceMount(action(20017, 5.6), { conditionMap: { 975: condition } }).LanceMounted, false);
    assert.equal(captureLanceMount(releases[0], { conditionMap: {} }).LanceMounted, true);
    assert.equal(lanceMountedAt(action(20017, 5.1), [{ At: 4, List: [] }, { At: 5, List: [condition] }]), null,
        "legacy same-second transition must not guess release order");
    const gains = [action(59106, -2), action(59101, -1), action(59102, 1), action(59103, 2),
        action(59104, 3), action(59105, 4), action(59106, 5)];
    assert.equal(row(report(gains), "heat").value, 60);
    assert.equal(row(report(gains, { lanceHeatGainMultiplier: 1 }), "heat").value, 40);
    assert.equal(row(report(gains, { lanceHeatGainMultiplier: undefined }), "heat").status, "pending");
    assert.equal(row(report([action(59106, 1), action(59102, 2), action(59106, 3)]), "heat").value, 7.5);
    assert.equal(row(report([action(59106, 1), action(59106, 2)]), "heat").value, 0);
    assert.equal(row(report([action(59106, 1)]), "heat").value, null);
    assert.deepEqual(lanceHeatSamples([action(59106, 1), action(59105, 2), action(59106, 2000)], "player", 2000)
        .map((s) => s.basePoints), [null, 15], "no time decay or invented cap");
    assert.equal(row(report([action(59106, 1), action(59102, 12), action(59106, 21)]), "heat").value, 7.5,
        "immune-period resource gains still affect the next valid sample");
    assert.equal(row(report([action(59106, 1), action(59102, 2), action(59106, 12), action(59106, 21)]), "heat").value, 0,
        "immune-period Annihilation clears heat but is not itself a KPI sample");
    assert.deepEqual(lanceHeatSamples(gains, "player", 30, [3.5]).map((s) => s.basePoints), [null, null]);
    assert.equal(row(report([...gains, action(59102, 3.5, { IsFallback: true })]), "heat").value, null,
        "an unmatched fallback inside a cycle invalidates its resource total");
    assert.equal(row(report([...gains, { ...gains[2], IsFallback: true }]), "heat").value, 60,
        "a matching fallback copy does not add a gain or invalidate its confirmed release");
    assert.equal(row(report([...gains, action(59105, 4.5, { SubSkillId: 2 }), gains[4],
        action(59102, 4.1, { Id: "other" }), action(59102, 4.2, { SourceId: "pet" })]), "heat").value, 60);
    const appearance = (id, race) => ({ EventId: 1, Id: id, At: 0, Name: id, RaceId: race, Height: 1, Weight: 1, Upper: 1, Lower: 1, OwnerId: "", GuildName: "" });
    const damage = (at) => ({ EventId: 3, Id: "player", At: at, TargetId: "boss", Damage: 100, SkillId: 59106 });
    const events = [appearance("player", 9001), appearance("boss", 7601), action(59106, 1), action(59102, 2),
        { EventId: 4, ...condition }, action(20017, 4), { EventId: 5, Id: "player", At: 4, CCId: 975 },
        action(59106, 4.5), damage(4), damage(6)];
    const collector = new DamageCollectorManager(), manager = new ActorManager(collector);
    for (const event of events) manager.onEvent(event);
    const snapshot = buildEventSnapshot(events.map(JSON.stringify).join("\n"));
    assert.deepEqual(snapshot.skillActions, manager.skillActions, "live and worker preserve identical ordered release states");
    assert.equal(snapshot.skillActions.find((a) => a.SkillId === 20017).LanceMounted, true);
    assert.equal(snapshot.skillActions.at(-1).LanceMounted, false);
    const record = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    assert.equal(record.snapshot.skillActions[0].At, 1, "pre-fight clearing cast survives export");
    hydrateFromSnapshot(record.snapshot, manager, collector);
    const restored = buildArcanaKpi({ ...input, player: manager.entityMap.player, boss: manager.entityMap.boss,
        session: record.battle, actions: manager.skillActions, arcanaSignals: manager.arcanaSignals });
    assert.equal(row(restored, "heat").value, 7.5);
    assert.equal(row(restored, "lance-mounted").value, 50);
    console.log("Lancer KPI verified: mounted releases, heat gains, +50%, expiry, phase/owner dedup, immunity and record round-trip.");
} finally { await server.close(); }
