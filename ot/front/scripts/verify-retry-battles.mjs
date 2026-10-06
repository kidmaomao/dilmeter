import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false, ws: false },
    optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
try {
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const { buildEventSnapshot } = await server.ssrLoadModule("/src/worker/buildEventSnapshot.ts");
    const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
    const { buildBossSummary } = await server.ssrLoadModule("/src/summaryCollector.ts");
    const { createBattleRecord, parseBattleRecord } = await server.ssrLoadModule("/src/battleRecord.ts");
    const appear = (Id, RaceId, At) => ({ EventId: 1, Id, RaceId, At,
        Name: RaceId === 193810 ? "同一个首领" : Id, OwnerId: "", GuildName: "",
        Height: 1, Weight: 1, Upper: 1, Lower: 1 });
    const hp = (Id, At, Value) => ({ EventId: 17, Id, At, Private: false,
        Stats: [{ StatId: 28, Value }, { StatId: 30, Value: 1_800_000_000 }] });
    const hit = (TargetId, At, Damage) => ({ EventId: 3, Id: "player", TargetId, At,
        SkillId: 10001, Damage, IsCritical: false, IsDelayed: false });
    // Taiwan's observed retry replaces the Boss entity inside the same map and
    // connection. No local reset, kill, room-clear or new player appearance occurs.
    const events = [appear("player", 10001, 100), appear("attempt-1", 193810, 100),
        hp("attempt-1", 100, 1_800_000_000), hit("attempt-1", 101, 1_000_000_000),
        hp("attempt-1", 101, 800_000_000), hit("attempt-1", 110, 600_000_000),
        hp("attempt-1", 110, 200_000_000), hp("attempt-1", 111, 400_000_000),
        { EventId: 2, Id: "attempt-1", At: 111 }, appear("attempt-2", 193810, 165),
        hp("attempt-2", 165, 1_800_000_000), hit("attempt-2", 166, 900_000_000),
        hp("attempt-2", 166, 900_000_000), hit("attempt-2", 176, 900_000_000),
        hp("attempt-2", 176, 0)];

    for (const mode of ["live", "worker"]) {
        const collector = new DamageCollectorManager();
        const actors = new ActorManager(collector);
        if (mode === "live") events.forEach(e => actors.onEvent(e));
        else hydrateFromSnapshot(buildEventSnapshot(events.map(e => JSON.stringify(e)).join("\n")), actors, collector);
        assert.equal(actors.entityMap["attempt-1"].group, actors.entityMap["attempt-2"].group,
            "the test must exercise two entities of the same Boss group");
        for (const [id, damage, start, end] of [["attempt-1", 1_600_000_000, 101, 110], ["attempt-2", 1_800_000_000, 166, 176]]) {
            const summary = buildBossSummary(id, actors, []);
            assert.equal(summary.totalDamage, damage, `${mode}: retry damage must stay separate`);
            assert.deepEqual([summary.session.startAt, summary.session.endAt], [start, end]);
            assert.equal(summary.players[0].totalDamage, damage);
            const record = parseBattleRecord(JSON.stringify(createBattleRecord(actors, collector, id, "player", { bossName: "同一个首领" })));
            const restoredCollector = new DamageCollectorManager();
            const restored = new ActorManager(restoredCollector);
            hydrateFromSnapshot(record.snapshot, restored, restoredCollector);
            assert.equal(buildBossSummary(id, restored, []).totalDamage, damage,
                `${mode}: saved attempt must not import another retry's damage`);
        }
    }
    console.log("Retry with a replacement Boss on the same connection: live, worker and saved reports remain separate.");
} finally { await server.close(); }
