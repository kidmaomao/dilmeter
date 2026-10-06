import fs from "node:fs";
import assert from "node:assert/strict";
import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const source = process.argv[2], destination = process.argv[3];
if (!source) throw new Error("Usage: node scripts/inspect-arcana-kpi.mjs capture.ndjson [report.json]");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false }, optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.window = { dispatchEvent() {} };
try {
    const { PureActorManager, PureDamageCollectorManager } = await server.ssrLoadModule("/src/worker/pureEventActor.ts");
    const { buildBossSummary } = await server.ssrLoadModule("/src/summaryCollector.ts");
    const { buildArcanaKpi } = await server.ssrLoadModule("/src/arcanaKpi.ts");
    const collector = new PureDamageCollectorManager();
    const manager = new PureActorManager(collector);
    let eventCount = 0;
    for await (const line of readline.createInterface({ input: fs.createReadStream(source), crlfDelay: Infinity })) {
        let event; try { event = JSON.parse(line); } catch { continue; }
        manager.onEvent(event); eventCount++;
    }
    const minimumDamage = Number(process.env.DILMETER_KPI_MIN_DAMAGE ?? 100_000_000);
    const selectedBossId = process.env.DILMETER_KPI_BOSS_ID;
    const bosses = Object.values(manager.entityMap).filter((actor) => !actor.isPC && actor.totalTakeDamage >= minimumDamage
        && (!selectedBossId || actor.id === selectedBossId))
        .sort((a, b) => b.totalTakeDamage - a.totalTakeDamage).slice(0, 8);
    const reports = [];
    for (const boss of bosses) {
        const summary = buildBossSummary(boss.id, manager, []);
        if (!summary) continue;
        for (const player of summary.players) {
            if (!player.jobName) continue;
            const report = buildArcanaKpi({ player: manager.entityMap[player.entityId], boss,
                session: summary.session, jobName: player.jobName, actions: manager.skillActions, localEntityId: manager.localEntityId,
                statUpdates: manager.statUpdates, skillCooldowns: manager.skillCooldowns, arcanaSignals: manager.arcanaSignals, musicPerformances: manager.musicPerformances, dorchaMinimum: .5 });
            reports.push({ bossEntityId: boss.id, bossRaceId: boss.raceId, playerEntityId: player.entityId,
                totalDamage: player.totalDamage, ...report });
        }
    }
    const result = { eventCount, localEntityId: manager.localEntityId, reports };
    if (destination) fs.writeFileSync(destination, JSON.stringify(result, null, 2));
    if (process.argv[4] && bosses[0]) {
        const { createBattleRecord } = await server.ssrLoadModule("/src/battleRecord.ts");
        const record = createBattleRecord(manager, collector, bosses[0].id, manager.localEntityId, { bossName: "抓包参考首领" });
        fs.mkdirSync(path.dirname(process.argv[4]), { recursive: true });
        fs.writeFileSync(process.argv[4], JSON.stringify(record));
        const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
        const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
        const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
        const replayCollector = new DamageCollectorManager(), replayManager = new ActorManager(replayCollector);
        hydrateFromSnapshot(JSON.parse(fs.readFileSync(process.argv[4], "utf8")).snapshot, replayManager, replayCollector);
        const replaySummary = buildBossSummary(bosses[0].id, replayManager, []);
        for (const original of reports.filter((report) => report.bossEntityId === bosses[0].id)) {
            const replay = buildArcanaKpi({ player: replayManager.entityMap[original.playerEntityId],
                boss: replayManager.entityMap[original.bossEntityId], session: replaySummary.session,
                jobName: original.jobName, actions: replayManager.skillActions, statUpdates: replayManager.statUpdates, localEntityId: replayManager.localEntityId,
                arcanaSignals: replayManager.arcanaSignals, musicPerformances: replayManager.musicPerformances, skillCooldowns: replayManager.skillCooldowns, dorchaMinimum: .5 });
            assert.deepEqual(replay.rows.map(({ id, value, upperValue, status, samples }) => ({ id, value, upperValue, status, samples })),
                original.rows.map(({ id, value, upperValue, status, samples }) => ({ id, value, upperValue, status, samples })), "KPI values and uncertainty bounds survive the exported capture record");
        }
    }
    console.log(JSON.stringify({ eventCount, reports: reports.slice(0, 2).map((report) => ({ bossRaceId: report.bossRaceId,
        jobName: report.jobName, activeSeconds: report.activeSeconds, excludedSeconds: report.excludedSeconds,
        values: report.rows.map((row) => ({ id: row.id, value: row.value, upperValue: row.upperValue, status: row.status })) })) }, null, 2));
} finally { await server.close(); }
