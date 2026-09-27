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
    assert.deepEqual(timeline.members[0].points.map(p => p.y), [0, 800, 600, 520]);
    assert.deepEqual(timeline.total.map(p => p.y), [0, 850, 650, 560]);
    assert.equal(timeline.peak.y, 850);
    assert.deepEqual(players, original);
    for (const step of [1, 2, 5, 10]) {
        const result = build(players, 100, 102.5, step);
        assert.equal(result.total.at(-1).y, 560, "resolution must preserve cumulative DPS");
        assert.equal(result.total.at(-1).custom.damage, 1400);
        result.total.forEach((p, i) => assert.equal(p.y, result.members.reduce((sum, m) => sum + m.points[i].y, 0)));
    }
    assert.deepEqual(build([player("burst", [[0, 120]])], 0, 5, 1).total.map(p => p.y), [0, 120, 60, 40, 30, 24]);
    const burst = [player('burst', [[0,100],[1,100],[2,100],[3,1100],[4,100],[5,100],[6,100],[7,100],[8,100]])];
    const rolling = build(burst, 0, 10, 1, { dpsMode: 'rolling', windowSeconds: 5 });
    assert.equal(rolling.total[1].y, 200, 'opening window uses elapsed time');
    assert.equal(rolling.total[5].y, 320, 'opening hit is included in first full window');
    assert.equal(rolling.total[6].y, 300, 'left boundary is excluded once the window starts moving');
    assert.equal(rolling.total[8].y, 100, 'burst leaves the window after five seconds');
    assert.equal(rolling.total[10].y, 60, 'pauses lower recent DPS instead of retaining cumulative average');
    assert.deepEqual([rolling.total[8].custom.from, rolling.total[8].custom.to], [3,8]);
    assert.equal(rolling.total.at(-1).custom.damage, 1900, 'recent DPS keeps cumulative damage available');
    const rollingTeam = build([...burst, player('other', [[3,500]])], 0, 10, 1, { dpsMode: 'rolling', windowSeconds: 5 });
    rollingTeam.total.forEach((p,i) => assert.equal(p.y, rollingTeam.members.reduce((s,m)=>s+m.points[i].y,0)));
    const { splitTeamSegments } = await server.ssrLoadModule("/src/teamDps.ts");
    const { deadIntervals, invulnerableIntervals, clipVitalHistory, healthPercentAt } = await server.ssrLoadModule("/src/battleChartHistory.ts");
    const hp = [{ At: 0, current: 1000, maximum: 1000 }, { At: 2, current: 800, maximum: 1000 }, { At: 4, current: 900, maximum: 1000 }];
    const died = deadIntervals([{ At: 2, dead: true }], 6);
    const revived = deadIntervals([{ At: 2, dead: true }, { At: 4, dead: false }], 6);
    assert.deepEqual(revived, [{ from: 2, to: 4 }]);
    const immune = invulnerableIntervals([{ At: 3, List: [{ CCId: 494, DisableAtMs: 4500 }] }, { At: 5, List: [] }], 6);
    assert.deepEqual(immune, [{ from: 3, to: 4.5 }]);
    const death = build([{ ...player('dead', [[0,120]]), deadIntervals: died }], 0, 6, 5, { health: hp, invulnerable: immune });
    assert.deepEqual(death.members[0].points.map(p => p.x), [0, 2, 3, 4.5, 5, 6]);
    assert.equal(death.members[0].points.at(-1).y, 20, 'dead player must decay to the team end');
    const dashed = splitTeamSegments(death.members[0].points, true, 'time');
    assert.equal(dashed.filter(Boolean)[0].x, 2);
    assert.equal(dashed.at(-1).x, 6);
    const hpSeries = splitTeamSegments(death.members[0].points, true, 'hp').filter(Boolean);
    assert.deepEqual(hpSeries.map(p => p.x), [80,80,90,90,90], 'healing and flat HP retain chronological vertices');
    assert.equal(healthPercentAt([], 5), undefined, 'old records do not invent HP');
    assert.deepEqual(clipVitalHistory(hp, 3, 5), [{ ...hp[1], At: 3 }, hp[2]]);
    const healed = build([{ ...player('revived', [[0,120],[5,60]]), deadIntervals: revived }], 0, 6, 1);
    assert.equal(healed.members[0].points[4].custom.dashed, false);
    for (const [start, end] of [[1, 1], [2, 1], [NaN, 1], [0, Infinity]]) assert.deepEqual(build(players,start,end,1).total.map(p=>p.y),[0]);
    assert.equal(build([], 0, 60000, 1).total.length, 12001);
    console.log("Team cumulative DPS verified: death/revival, immunity boundaries, health plateaus/healing, legacy records and sampling.");
} finally {
    await server.close();
}
