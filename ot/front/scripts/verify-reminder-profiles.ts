import { strict as assert } from "node:assert";

const values = new Map<string, string>();
Object.defineProperty(globalThis, "localStorage", {
    value: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, String(value)),
    },
});

const {
    loadReminderProfileStore,
    makeReminderProfile,
    saveReminderProfileStore,
} = await import("../src/reminderProfiles.ts");

const fallback = {
    buffRules: { 680: { ccId: 680, overlayEnabled: true } },
    debuffSettings: {
        overlayEnabled: true,
        firstRoundGraceSeconds: 10,
        iconSize: 30,
        volume: 80,
        forcedBossRaceIds: [7615],
        soundMode: "electronic",
        customSoundId: "",
        customSoundName: "",
        rules: { 504: { ccId: 504, enabled: true, warningSeconds: 8, order: 0, soundEnabled: true, soundMode: "voice", customSoundId: "", customSoundName: "" } },
    },
    aimReminder: {
        enabled: true,
        alwaysVisible: true,
        weaponRange: 2100,
        rangeIdentificationLevel: 12,
        calibrationPercent: 35,
        ergSpeedPercent: 220,
        fineTuneSeconds: -0.08,
        scalePercent: 115,
        x: 720,
        y: 340,
    },
    skillRules: { 54151: { skillId: 54151, enabled: true, cooldownSeconds: 12 } },
    bossMechanicSettings: {
        volume: 70,
        x: 940,
        y: 260,
        scalePercent: 125,
        rules: { "miel-spear": { key: "miel-spear", enabled: true, skillId: 52402 } },
    },
    effectTimerSettings: {
        rules: { boost: { key: "boost", enabled: true, sourceType: "condition", sourceId: 123, name: "伤害增益", durationSeconds: 12, alwaysVisible: false, targetMode: "self", orientation: "horizontal", scalePercent: 100, opacityPercent: 80, x: 700, y: 300 } },
    },
    playerBuffIds: [680, 1023],
    playerBuffFavoriteIds: [680],
    playerBuffUsesDefaults: false,
};
const initial = loadReminderProfileStore(fallback as never);
assert.equal(initial.profiles.length, 1);
assert.equal(initial.profiles[0].skillRules[54151].cooldownSeconds, 12);
assert.equal(initial.profiles[0].debuffSettings.rules[504].soundMode, "voice");
assert.deepEqual(initial.profiles[0].debuffSettings.forcedBossRaceIds, [7615]);
assert.equal(initial.profiles[0].bossMechanicSettings.scalePercent, 125);
assert.deepEqual(initial.profiles[0].aimReminder, fallback.aimReminder);
assert.equal(initial.profiles[0].effectTimerSettings.rules.boost.durationSeconds, 12);

const second = makeReminderProfile("人偶方案", {
    ...fallback,
    skillRules: { 59167: { skillId: 59167, enabled: true, cooldownSeconds: 20 } },
} as never);
initial.profiles.push(second);
initial.activeProfileId = second.id;
saveReminderProfileStore(initial);

const reloaded = loadReminderProfileStore(fallback as never);
assert.equal(reloaded.activeProfileId, second.id);
assert.equal(reloaded.profiles[1].name, "人偶方案");
assert.equal(reloaded.profiles[1].skillRules[59167].cooldownSeconds, 20);
assert.equal(reloaded.profiles[1].debuffSettings.rules[504].ccId, 504);
assert.deepEqual(reloaded.profiles[1].debuffSettings.forcedBossRaceIds, [7615]);
assert.equal(reloaded.profiles[1].bossMechanicSettings.x, 940);
assert.deepEqual(reloaded.profiles[1].aimReminder, fallback.aimReminder);
assert.equal(reloaded.profiles[1].effectTimerSettings.rules.boost.opacityPercent, 80);

console.log("multiple Buff, Debuff, aim reminder, skill cooldown, effect timer and Boss mechanic profiles verified");
