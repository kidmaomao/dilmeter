import assert from "node:assert/strict";
import { readFileSync } from "node:fs";

import {
    allocateEffectiveDamage,
    float32HealthTolerance,
    selectConfirmedRawDamages,
} from "../src/effectiveDamage.ts";

type TestDamage = {
    Id: string;
    TargetId: string;
    At: number;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
};

function hit(id: string, damage: number, at = 1): TestDamage {
    return {
        Id: id,
        TargetId: "boss",
        At: at,
        SkillId: 10001,
        Damage: damage,
        IsCritical: false,
        IsDelayed: false,
    };
}

function byPlayer(
    allocations: ReturnType<typeof allocateEffectiveDamage<TestDamage>>,
): Record<string, number> {
    const result: Record<string, number> = {};
    for (const allocation of allocations) {
        result[allocation.event.Id] = (result[allocation.event.Id] ?? 0) + allocation.damage;
    }
    return result;
}

// Health did not move: every pending hit was immune and must stay out of DPS.
assert.deepEqual(allocateEffectiveDamage([hit("player", 200)], 1_000, 1_000), []);

// At a phase threshold the older hit can be immune while the newest hit is
// effective. Allocation therefore consumes the authoritative decrease from
// newest to oldest, rather than scaling the whole pending batch.
assert.deepEqual(
    byPlayer(allocateEffectiveDamage([
        hit("immune-first", 100, 1),
        hit("effective-last", 40, 2),
    ], 800, 760)),
    { "effective-last": 40 },
);

// Two players in one Stat28 update follow packet order. Aggregate health loss
// remains exact, and the most recent player receives the partial attribution.
const multiplayer = allocateEffectiveDamage([
    hit("player-a", 100, 1),
    hit("player-b", 100, 2),
], 200, 150);
assert.deepEqual(byPlayer(multiplayer), { "player-b": 50 });
assert.equal(multiplayer.reduce((sum, value) => sum + value.damage, 0), 50);

// Health accounting clips a finishing packet to the remaining Boss health,
// while DPS keeps the confirmed packet's full raw value (including overkill).
const finishingHit = hit("finisher", 100);
const finishingAllocation = allocateEffectiveDamage([finishingHit], 30, 0);
assert.deepEqual(byPlayer(finishingAllocation), { finisher: 30 });
assert.deepEqual(
    selectConfirmedRawDamages(
        [finishingHit],
        finishingAllocation.map(({ event, damage }) => ({ ...event, Damage: damage })),
    ).map((damage) => damage.Damage),
    [100],
);

// The server can report a follow-up packet as a negative Stat28 value after
// the first hit reached zero. It is overkill for DPS, not additional Boss HP.
assert.deepEqual(
    byPlayer(allocateEffectiveDamage([hit("late-finisher", 20)], 0, -20)),
    { "late-finisher": 20 },
);

// Matching consumes occurrence counts, so one confirmed hit cannot revive an
// otherwise identical immune packet.
assert.equal(
    selectConfirmedRawDamages(
        [hit("player", 200), hit("player", 200)],
        [{ ...hit("player", 200), Damage: 50 }],
    ).length,
    1,
);

// Stat28 is float32. Around a two-billion-health Boss one ULP is 128, so a
// 100-point packet may legitimately expose a 128-point health-bar decrease.
const bossHealth = 1_967_880_064;
assert.equal(float32HealthTolerance(bossHealth), 256);
const float32Allocation = allocateEffectiveDamage(
    [hit("player", 100)],
    bossHealth,
    bossHealth - 128,
);
assert.deepEqual(byPlayer(float32Allocation), { player: 128 });
assert.equal(float32Allocation[0].event.Damage, 100, "raw packet must remain immutable");

const reportSource = readFileSync(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
assert.match(reportSource, /trueMaximumHealth\s*>\s*0\s*\?\s*fmtBossHealth\(trueMaximumHealth\)\s*:\s*"血量未知"/);
assert.match(reportSource, /function fmtBossHealth[\s\S]*?100_000_000\)\.toFixed\(2\)/);
assert.match(reportSource, /const bossDamage = selectedBossMaximumHealth\(\) \|\| summary\.value\?\.effectiveBossDamage \|\| 0/);
assert.match(reportSource, /溢伤和被动伤害也会计入 DPS/);

process.stdout.write("Effective damage reconciliation and real Boss-health display verified.\n");
