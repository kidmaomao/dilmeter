import assert from "node:assert/strict";

import { buildBossSummary } from "../src/summaryCollector.ts";

type TestDamage = {
    Id: string;
    TargetId: string;
    At: number;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
    Conditions: never[];
    TargetConditions: never[];
    PetId: string;
};

function hit(at: number, damage: number, skillId: number): TestDamage {
    return {
        Id: "player",
        TargetId: "boss",
        At: at,
        SkillId: skillId,
        Damage: damage,
        IsCritical: false,
        IsDelayed: false,
        Conditions: [],
        TargetConditions: [],
        PetId: "",
    };
}

// The first and last raw hits are phase/finishing hits without a matching HP
// decrease. They must still extend the fight and contribute to player DPS.
const rawDamages = [
    hit(5, 50, 35014),
    hit(10, 100, 59145),
    hit(30, 300, 59145),
    hit(35, 50, 35014),
];
const boss = {
    id: "boss",
    name: "Boss",
    raceId: 7615,
    isPC: false,
    ownerId: "",
    takeDamages: rawDamages,
    applyDamages: [],
    conditionHistory: [],
    equipItemMap: {},
};
const player = {
    id: "player",
    name: "Player",
    raceId: 9001,
    isPC: true,
    ownerId: "",
    takeDamages: [],
    applyDamages: rawDamages,
    conditionHistory: [],
    equipItemMap: {},
};
const actorManager = {
    entityMap: { boss, player },
    effectiveDamages: [
        { ...rawDamages[1], Damage: 100 },
        { ...rawDamages[2], Damage: 50 },
    ],
    healthLosses: [
        { Id: "boss", At: 10, Damage: 100 },
        { Id: "boss", At: 30, Damage: 50 },
    ],
};

const summary = buildBossSummary("boss", actorManager as never, []);
assert.ok(summary);
assert.equal(summary.session.startAt, 5);
assert.equal(summary.session.endAt, 35);
assert.equal(summary.session.totalDuration, 30);
assert.equal(summary.players.length, 1);
assert.equal(summary.players[0].totalDamage, 500);
assert.equal(summary.players[0].totalDPS, 500 / 30);
assert.equal(summary.players[0].skillStats.reduce((sum, skill) => sum + skill.totalHits, 0), 4);
assert.equal(summary.effectiveBossDamage, 150, "HP accounting must remain reconciled");

process.stdout.write("Raw phase and finishing damage DPS verified.\n");
