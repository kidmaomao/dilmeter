import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import vm from "node:vm";
import { createServer } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true }, optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
try {
    const { LiveBattleCursor, applyLiveDelta } = await server.ssrLoadModule("/src/liveBattles.ts");
    const { buildEventSnapshot } = await server.ssrLoadModule("/src/worker/buildEventSnapshot.ts");
    const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const ndjson = (events) => events.map((e) => JSON.stringify(e)).join("\n") + "\n";
    const appear = (Id, RaceId) => ({ EventId: 1, At: 100, Id, RaceId, Name: Id, GuildName: "", OwnerId: "", Height: 1, Weight: 1, Upper: 1, Lower: 1 });
    const hit = (Sequence) => ({ EventId: 3, At: 101, Id: "player", TargetId: "boss", SkillId: 35024, Damage: 100, IsCritical: false, IsDelayed: false, Sequence });
    const actors = new ActorManager(new DamageCollectorManager());
    const collector = new DamageCollectorManager();
    const events = [
        { EventId: 11, At: 100, Id: "player", Reliable: true },
        appear("player", 10001), appear("boss", 7603),
        { EventId: 17, Id: "boss", At: 100, Stats: [{ StatId: 28, Value: 1000 }, { StatId: 30, Value: 1000 }] },
        hit(10),
    ];
    const snapshot = buildEventSnapshot(ndjson(events));
    hydrateFromSnapshot(snapshot, actors, collector);
    assert.equal(actors.localEntityId, "player", "recent restore preserves local-player identity");
    assert.equal(actors.activeEntityMap.boss, true);
    actors.onEvent({ EventId: 17, Id: "boss", At: 101, Stats: [{ StatId: 28, Value: 900 }] });
    assert.equal(actors.effectiveDamages.at(-1).Damage, 100, "a snapshot ending between damage and HP update preserves the pending hit");

    const buff = (At) => ({ EventId: 4, Id: "player", At, CCId: 123, DisableAt: At + 60, AttackerId: "" });
    const buffSnapshot = buildEventSnapshot(ndjson([appear("player", 10001), buff(99), buff(100)]));
    hydrateFromSnapshot(buffSnapshot, actors, collector);
    actors.onEvent({ EventId: 5, Id: "player", At: 101, CCId: 123 });
    assert.ok(actors.entityMap.player.conditionMap[123], "a pending Buff refresh guard survives a snapshot handoff");
    actors.onEvent({ EventId: 5, Id: "player", At: 102, CCId: 123 });
    assert.equal(actors.entityMap.player.conditionMap[123], undefined, "a subsequent real remove still applies");

    const cursor = new LiveBattleCursor();
    cursor.sessionKey = "generation-1";
    cursor.sequence = 10;
    const applied = [];
    let yields = 0;
    const delta = [appear("stale-bootstrap", 7603), hit(10), hit(11), hit(12), ...Array.from({ length: 1024 }, (_, i) => ({ EventId: 10, At: 102, Id: "player", SkillId: 35024, Sequence: i + 13 }))];
    await applyLiveDelta(ndjson(delta), (rows) => {
        for (const row of rows) if (cursor.accept(row)) applied.push(row);
    }, async () => { yields++; });
    assert.deepEqual(applied.filter((e) => e.EventId === 3).map((e) => e.Sequence), [11, 12], "overlap is excluded without collapsing genuine identical multihits");
    assert.ok(yields >= 2, "large catch-up batches yield to UI interaction");
    assert.equal(cursor.accept(hit(12)), false, "buffered socket overlap is ignored");
    assert.equal(applied.some((e) => e.Id === "stale-bootstrap"), false, "a sparse delta does not resurrect stale bootstrap actors");
    const beforeMessage = cursor.sequence;
    cursor.accept({ EventId: -1, Id: "", At: 103, Sequence: beforeMessage + 10 });
    assert.equal(cursor.sequence, beforeMessage, "unpersisted messages cannot advance the disk-resume watermark");
    const query = new URL(cursor.requestUrl(true), "http://localhost").searchParams;
    assert.equal(query.get("afterSequence"), String(cursor.sequence));
    assert.equal(query.get("knownSession"), cursor.sessionKey);

    // Restore a new window into an existing manager: no historical actor or
    // pending timer may survive and add work to the live report.
    const next = buildEventSnapshot(ndjson([appear("player", 10001), appear("new", 7615)]));
    hydrateFromSnapshot(next, actors, collector);
    assert.equal(actors.entityMap.boss, undefined);
    assert.equal(actors.activeEntityMap.boss, undefined);
    assert.equal(actors.damages.length, 0);
    assert.equal(collector.damages.length, 0);
    const large = buildEventSnapshot(ndjson([appear("player", 10001), appear("boss", 7603), hit(1)]));
    large.damages = Array.from({ length: 200_000 }, () => hit(1));
    hydrateFromSnapshot(large, actors, collector);
    assert.equal(actors.damages.length, 200_000, "large selected records do not exceed the JS argument limit");
    actors.prepareSnapshot();

    // Execute the actual App recovery handler with healthy and suspended
    // connections. Ordinary focus/timer callbacks must not rebuild a report.
    const source = await readFile(path.join(root, "src/App.vue"), "utf8");
    const recovery = source.slice(source.indexOf("        const recoverAfterBackground ="), source.indexOf("        const notePageBackground ="));
    let reloads = 0;
    const context = { flushPendingUiUpdates() {}, isStandalone: false, isRecordReplay: { value: false },
        document: { hidden: false }, uiDormantNeedsReload: false,
        socket: { isSuspended: false, isOpen: true, ensureConnected() {} },
        needsLiveRecovery: (suspended, connected) => suspended || !connected,
        resumeDormantUi() { reloads++; }, refreshBattleCatalog() {} };
    vm.createContext(context);
    vm.runInContext(recovery + "\nfor (let i = 0; i < 100; i++) recoverAfterBackground();", context);
    assert.equal(reloads, 0, "healthy focus changes never start a full reload");
    context.socket.isSuspended = true;
    vm.runInContext("recoverAfterBackground();", context);
    assert.equal(reloads, 1, "a real suspension triggers incremental recovery");
    console.log("live windows: snapshot continuity, exact-once multihits, bounded catch-up, old-state release, and healthy-focus recovery verified");
} finally { await server.close(); }
