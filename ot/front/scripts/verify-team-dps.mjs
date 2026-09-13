import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] } });
try {
    const { buildTeamDpsTimeline: build } = await server.ssrLoadModule("/src/teamDps.ts");
    const player = (id, damages) => ({ entityId: id, label: id, damages: damages.map(([At, Damage]) => ({ At, Damage })) });
    const players = [
        player("a", [[102.5, 100], [100, 100], [100.99, 200], [101, 500], [102, 400]]),
        player("b", [[100.5, 50], [101.5, 50], [99, 10000], [103, 10000], [101, -2], [NaN, 1], [101, Infinity]]),
    ];
    const original = structuredClone(players);
    const timeline = build(players, 100, 102.5, 1);
    assert.deepEqual(timeline.members[0].points.map(p => p.y), [0, 800, 400, 200]);
    assert.deepEqual(timeline.total.map(p => p.y), [0, 850, 450, 200]);
    assert.deepEqual(timeline.peak, { x: 1, y: 850, custom: { from: 0, to: 1, damage: 850 } });
    assert.deepEqual(players, original, "chart calculations must not reorder or mutate combat records");
    for (const step of [1, 2, 5, 10]) {
        const result = build(players, 100, 102.5, step);
        const integratedDamage = result.total.reduce((sum, p) => sum + p.y * (p.custom.to - p.custom.from), 0);
        assert.equal(integratedDamage, 1400, "changing resolution must preserve all damage, including first and final hits");
        result.total.forEach((p, i) => assert.equal(p.y, result.members.reduce((sum, m) => sum + m.points[i].y, 0)));
    }
    assert.deepEqual(build([player("a", [[0, 10], [1, 20], [2, 30]])], 0, 2, 1).total.map(p => p.y), [0, 30, 30]);
    const live = [player("a", [[0, 10], [1, 20], [2, 30]])];
    const before = build(live, 0, 2, 1).total;
    const after = build([...live, player("new", [[3, 40]])], 0, 3, 1).total;
    assert.deepEqual(after.slice(0, before.length), before, "live extension must not move completed-interval hits or manufacture a final peak");
    assert.deepEqual(build([player("burst", [[0, 120]])], 0, 5, 1).total.map(p => p.y), [0, 120, 0, 0, 0, 0], "idle phases must drop to zero, not keep the cumulative average");
    assert.deepEqual(build([player("short", [[0, 100], [12.5, 25]])], 0, 12.5, 10).total.map(p => p.y), [0, 10, 10], "partial intervals use their actual duration");
    for (const [start, end] of [[1, 1], [2, 1], [NaN, 1], [0, Infinity]]) {
        const result = build(players, start, end, 1);
        assert.deepEqual(result.total.map(p => p.y), [0], "zero/invalid duration cannot create infinite DPS");
    }
    assert.equal(build([], 0, 5, 1).peak.y, 0);
    const long = build([], 0, 60000, 1);
    assert.equal(long.stepSeconds, 5);
    assert.equal(long.total.length, 12001, "long recordings have bounded chart point counts");
    console.log("Team DPS verified: phase damage, idle periods, first/final hits, partial intervals, totals and bounded long recordings.");
} finally {
    await server.close();
}
