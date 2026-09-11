import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
const values = new Map<string, string>();
Object.defineProperty(globalThis, "localStorage", {
    value: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, String(value)),
    },
});
Object.defineProperty(globalThis, "window", { value: new EventTarget() });

const {
    canonicalDebuffId,
    equivalentDebuffDisplayName,
    equivalentDebuffIds,
    evaluateDebuffAlert,
    isDebuffSoundCandidate,
    loadDebuffAlertSettings,
    makeDebuffAlertRule,
    saveDebuffAlertSettings,
    selectDebuffSoundCandidate,
    selectDebuffBossCandidate,
} = await import("../src/debuffAlert.ts");

const now = 1_800_000_000;
assert.equal(evaluateDebuffAlert(false, null, now, 8), "missing", "an absent armed Debuff must stay visible");
assert.equal(evaluateDebuffAlert(true, null, now, 8), "hidden", "unknown duration must not invent a countdown");
assert.equal(evaluateDebuffAlert(true, now + 30, now, 8), "hidden", "healthy Debuff remains quiet");
assert.equal(evaluateDebuffAlert(true, now + 8, now, 8), "expiring", "an expiring CC returns as a flashing reminder");
assert.equal(evaluateDebuffAlert(true, now + 0.01, now, 8), "expiring", "a trustworthy imminent expiry must warn");
assert.equal(
    evaluateDebuffAlert(true, now, now, 8),
    "hidden",
    "an active condition wins over a stale calculated expiry and must not stay falsely lit",
);
assert.deepEqual(equivalentDebuffIds(912), [912, 913]);
assert.deepEqual(equivalentDebuffIds(913), [912, 913]);
assert.deepEqual(equivalentDebuffIds(392), [392, 504]);
assert.deepEqual(equivalentDebuffIds(504), [392, 504]);
assert.deepEqual(equivalentDebuffIds(1144), [1144]);
assert.equal(canonicalDebuffId(913), 912);
assert.equal(equivalentDebuffDisplayName(504), "雷霆咆哮 / 闪电风暴");

const initialMissingSound = {
    state: "missing" as const,
    hasBeenApplied: false,
    soundEnabled: true,
    soundMode: "electronic" as const,
    customSoundId: "",
};
assert.equal(isDebuffSoundCandidate(initialMissingSound), false, "the first missing Debuff is visual-only");
assert.equal(
    isDebuffSoundCandidate({ ...initialMissingSound, hasBeenApplied: true }),
    true,
    "a Debuff that was applied and then lost may sound",
);
assert.equal(
    isDebuffSoundCandidate({ ...initialMissingSound, state: "expiring" }),
    true,
    "an applied Debuff entering its warning countdown may sound",
);
const simultaneousSounds = [
    { ...initialMissingSound, state: "expiring" as const, ccId: 504 },
    { ...initialMissingSound, hasBeenApplied: true, ccId: 913 },
];
assert.equal(selectDebuffSoundCandidate(simultaneousSounds, 10_000, 0, false)?.ccId, 504, "one batch chooses only its first sound");
assert.equal(selectDebuffSoundCandidate(simultaneousSounds, 10_000, 11_000, false), null, "the global cooldown drops new sounds");
assert.equal(selectDebuffSoundCandidate(simultaneousSounds, 10_000, 0, true), null, "an active sound never queues another sound");

const eligibleBosses = [
    { entityId: "small-add", raceId: 7001, estimatedHealth: 99_999_999, lastDamageAt: 30, active: true },
    { entityId: "boss-a", raceId: 7002, estimatedHealth: 300_000_000, lastDamageAt: 20, active: true },
    { entityId: "boss-b", raceId: 7003, estimatedHealth: 200_000_000, lastDamageAt: 40, active: true },
    { entityId: "old-boss", raceId: 7004, estimatedHealth: 900_000_000, lastDamageAt: 50, active: false },
];
assert.equal(
    selectDebuffBossCandidate(eligibleBosses, "small-add")?.entityId,
    "boss-a",
    "ordinary monsters and inactive bosses must not arm Debuff reminders",
);
assert.equal(
    selectDebuffBossCandidate(eligibleBosses, "boss-b")?.entityId,
    "boss-b",
    "the selected eligible Boss must be the sole Debuff target",
);
assert.equal(
    selectDebuffBossCandidate([eligibleBosses[0]], "small-add"),
    null,
    "no Debuff target exists below the 100 million health threshold",
);
assert.equal(
    selectDebuffBossCandidate([eligibleBosses[0]], "small-add", 100_000_000, [7001])?.entityId,
    "small-add",
    "a user-maintained Race ID must bypass only the Boss health threshold",
);
assert.equal(
    selectDebuffBossCandidate([{ ...eligibleBosses[0], active: false }], "small-add", 100_000_000, [7001]),
    null,
    "a forced Race ID must not revive an inactive target",
);

const settings = loadDebuffAlertSettings();
assert.deepEqual(
    Object.values(settings.rules).sort((a, b) => a.order - b.order).map((rule) => rule.ccId),
    [1164, 1165, 1166, 504, 598, 913, 1093, 1094, 1138],
    "fresh installations must start with the requested Boss Debuff reminders",
);
assert(Object.values(settings.rules).every((rule) => rule.warningSeconds === 20));
assert(Object.values(settings.rules).every((rule) => rule.soundEnabled === false));
settings.overlayEnabled = false;
settings.forcedBossRaceIds = [7615, 7603, 7615, -1, 0];
settings.soundMode = "custom";
settings.customSoundId = "sound-id";
settings.customSoundName = "boss-warning.wav";
const firstRule = makeDebuffAlertRule(913);
assert.equal(firstRule.soundEnabled, false, "new Debuff reminders must default to no sound");
firstRule.order = 8;
firstRule.soundEnabled = false;
firstRule.soundMode = "voice";
const secondRule = makeDebuffAlertRule(504);
secondRule.order = 2;
secondRule.soundMode = "custom";
secondRule.customSoundId = "rule-sound-id";
secondRule.customSoundName = "rule-warning.wav";
settings.rules = { 913: firstRule, 504: secondRule };
saveDebuffAlertSettings(settings);
const restored = loadDebuffAlertSettings();
assert.equal(restored.overlayEnabled, false, "Debuff visibility must persist independently from Buff visibility");
assert.deepEqual(restored.forcedBossRaceIds, [7615, 7603], "custom Debuff Boss Race IDs must be sanitized and persist");
assert.equal(restored.soundMode, "custom", "Debuff sound mode must persist");
assert.equal(restored.customSoundId, "sound-id", "Debuff custom sound must persist");
assert.equal(restored.rules[913].soundEnabled, false, "each Debuff must persist its own sound toggle");
assert.equal(restored.rules[504].customSoundId, "rule-sound-id", "each Debuff must persist its own selected sound");
assert.deepEqual(
    Object.values(restored.rules).sort((a, b) => a.order - b.order).map((rule) => rule.ccId),
    [504, 913],
    "Debuff icon order must persist",
);

const overlaySource = readFileSync(new URL("../src/components/DebuffOverlay.vue", import.meta.url), "utf8");
assert.match(
    overlaySource,
    /v-if="activeBoss && visibleItems\.length"/,
    "the Boss title and Debuff icons must hide together when no reminder remains",
);
assert.doesNotMatch(
    overlaySource,
    /activeBoss\.name[^\n]*出现/,
    "the overlay title must display only the Boss name",
);

console.log("Boss Debuff reminder state machine verified");
