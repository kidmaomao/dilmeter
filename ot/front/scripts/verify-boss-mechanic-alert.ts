import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import {
    advanceMielOrbRuntime,
    createMielOrbRuntime,
    isMielOrbVisible,
    normalizeBossMechanicAlertSettings,
    observeMielOrbContact,
    observeMielOrbLateConfirmation,
    resolveMielOrbHealthPhase,
} from "../src/bossMechanicAlert.ts";

const migrated = normalizeBossMechanicAlertSettings({
    volume: 65,
    x: 900,
    y: 300,
    scalePercent: 125,
    rules: {
        "miel-orb": {
            key: "miel-orb",
            name: "legacy",
            bossRaceIds: [7603],
            trigger: "orb-spawn",
            skillId: 52402,
            triggerRaceIds: [7604, 7605, 7606, 7607],
            enabled: true,
            countdownSeconds: 12,
            showCountdown: true,
            soundMode: "voice",
        },
        "miel-laser": {
            key: "miel-laser",
            name: "legacy",
            bossRaceIds: [7603],
            trigger: "skill-action",
            skillId: 52401,
            triggerRaceIds: [7604, 7605, 7606, 7607, 7608],
            enabled: true,
            countdownSeconds: 5,
            showCountdown: true,
            soundMode: "electronic",
        },
    },
});

assert.deepEqual(migrated.rules["miel-orb"].bossRaceIds, [7603, 7615]);
assert(migrated.rules["miel-orb"].triggerRaceIds.includes(7616));
assert.deepEqual(migrated.rules["miel-laser"].bossRaceIds, [7603, 7615]);
assert(migrated.rules["miel-laser"].triggerRaceIds.includes(7620));
assert.equal(migrated.rules["miel-orb"].enabled, true);
assert.equal(migrated.rules["miel-orb"].countdownSeconds, 12);
assert.equal(migrated.rules["miel-orb"].soundMode, "dedicated", "legacy generic sounds migrate to the dedicated timeline");
assert.deepEqual(migrated.rules["miel-orb"].cancelConditionIds, []);
assert.equal(migrated.rules["miel-orb"].x, 900);
assert.equal(migrated.rules["miel-orb"].y, 300);
assert.equal(migrated.rules["miel-laser"].x, 1040, "legacy shared coordinates must be separated during migration");
assert.equal(migrated.rules["miel-laser"].name, "神剑（射线）");
assert.equal(migrated.rules["miel-spear"], undefined, "removed spear reminder must not survive an old profile");

const oldDefaultCountdown = normalizeBossMechanicAlertSettings({
    rules: { "miel-orb": { countdownSeconds: 15 } },
});
assert.equal(oldDefaultCountdown.rules["miel-orb"].countdownSeconds, 20, "the historical 15-second default migrates to 20 seconds");
assert.equal(normalizeBossMechanicAlertSettings(null).rules["miel-orb"].soundMode, "dedicated");
assert.equal(normalizeBossMechanicAlertSettings(null).rules["miel-laser"].soundMode, "dedicated");
assert.deepEqual(normalizeBossMechanicAlertSettings(null).mielShardHealthPhases,
    { normal80: false, normal60: true, normal40: false, regret80: false },
    "existing profiles keep the historical Miel 60% bar enabled");
assert.equal(normalizeBossMechanicAlertSettings({ miel60HealthBarEnabled: false }).mielShardHealthPhases.normal60, false,
    "an explicitly disabled legacy Miel 60% health bar must stay disabled");
const customHealthBar = normalizeBossMechanicAlertSettings({
    miel60HealthBarX: 640,
    miel60HealthBarY: 180,
    miel60HealthBarScalePercent: 135,
});
assert.deepEqual(
    [customHealthBar.mielShardHealthBarX, customHealthBar.mielShardHealthBarY, customHealthBar.mielShardHealthBarScalePercent],
    [640, 180, 135],
    "legacy health-bar position and size migrate to the comfort-shard mechanism",
);
const allPhases = normalizeBossMechanicAlertSettings({
    mielShardHealthPhases: { normal80: true, normal60: true, normal40: true, regret80: true },
    mielShardHealthBarOpacityPercent: 55,
});
assert.deepEqual(allPhases.mielShardHealthPhases, { normal80: true, normal60: true, normal40: true, regret80: true });
assert.equal(allPhases.mielShardHealthBarOpacityPercent, 55);

const individuallyPositioned = normalizeBossMechanicAlertSettings({
    x: 900,
    y: 300,
    rules: {
        "miel-orb": { x: 420, y: 260 },
        "miel-laser": { x: 860, y: 510 },
    },
});
assert.deepEqual(
    [individuallyPositioned.rules["miel-orb"].x, individuallyPositioned.rules["miel-orb"].y],
    [420, 260],
);
assert.deepEqual(
    [individuallyPositioned.rules["miel-laser"].x, individuallyPositioned.rules["miel-laser"].y],
    [860, 510],
);

const overlaySource = readFileSync(new URL("../src/components/SkillCooldownOverlay.vue", import.meta.url), "utf8");
const reportSource = readFileSync(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
assert.match(
    overlaySource,
    /remaining <= 6\.5 && remaining > 1\.5/,
    "the orb warning must begin at the final 6.5 seconds",
);
assert.doesNotMatch(
    overlaySource,
    /remaining <= 16\.5/,
    "the obsolete 16.5-second orb warning threshold must not return",
);
const warningAnimation = /@keyframes boss-mechanic-warning\s*\{[\s\S]*?\n\}/.exec(overlaySource)?.[0] ?? "";
assert.match(
    warningAnimation,
    /scale\(var\(--boss-mechanic-scale\)\)/,
    "the warning animation must preserve the configured mechanic size",
);
assert.match(
    warningAnimation,
    /scale\(var\(--boss-mechanic-warning-scale\)\)/,
    "the warning pulse must derive from the configured mechanic size",
);
assert.doesNotMatch(
    warningAnimation,
    /transform:\s*scale\(1(?:\.045)?\)/,
    "the warning animation must not reset a custom-sized card to 100%",
);
assert.match(
    overlaySource,
    /const PARTICLE_MARGIN = 96;/,
    "the native overlay bounds must reserve room for a 200% mechanic glow",
);
assert.match(reportSource, /observeActiveMielOrbContact\(damage, damageAtMs\)/,
    "52407 damage feedback must feed the live orb state machine");
assert.match(reportSource, /advanceMielOrbRuntime\(activeOrbRuntime\.mielOrb, atMs\)/,
    "the scheduled 8\/11-second checkpoint must be evaluated by the overlay clock");
assert.match(reportSource, /action\.MechanicSignal === "miel-orb-late-confirm"/,
    "the dedicated 0x6d66 late-alive signal must reach the orb state machine");
assert.match(reportSource, /class="boss-mechanic-rule boss-mechanic-health-rule"[\s\S]{0,800}安乐碎片机制/,
    "comfort-shard phase switches must have their own Boss-mechanic card");
assert.match(reportSource, /mielShardHealthPhases\.normal80[\s\S]{0,1000}normal60[\s\S]{0,1000}normal40[\s\S]{0,1000}regret80/,
    "normal 80/60/40 and Regret 80 must be independently selectable");
assert.match(reportSource, /mielShardHealthBarX[\s\S]{0,600}mielShardHealthBarY[\s\S]{0,600}mielShardHealthBarScalePercent[\s\S]{0,600}mielShardHealthBarOpacityPercent/,
    "the shared health-bar controls must expose position, scale, and opacity");
assert.match(reportSource, /@click="previewMielShardHealthBar"/,
    "the independent health-bar card must expose an in-app preview button");
assert.match(overlaySource, /v-if="visibleTargetHealth"[\s\S]{0,1000}target-health-bar/,
    "the movable skill overlay must render the Miel target health preview and live state");
const mechanicPayloadSource = reportSource.slice(
    reportSource.indexOf("const mechanics = configuredBossMechanicRules"),
    reportSource.indexOf("const stackAlerts =", reportSource.indexOf("const mechanics = configuredBossMechanicRules")),
);
assert.doesNotMatch(mechanicPayloadSource, /\.\.\.runtime/,
    "the BroadcastChannel payload must not include the nested reactive orb runtime");
assert.match(mechanicPayloadSource, /startedAtMs: runtime\.startedAtMs[\s\S]*endsAtMs: runtime\.endsAtMs[\s\S]*generation: runtime\.generation/,
    "the overlay receives a plain primitive mechanic lifecycle payload");
assert.match(reportSource, /skillOverlayChannel\.postMessage\(JSON\.parse\(JSON\.stringify\(message\)\)/,
    "BroadcastChannel clone failures retain a plain-object fallback");
assert.doesNotMatch(reportSource, /watch\(selectedBossId,[\s\S]{0,600}bossMechanicRuntime\.value = \{\}/,
    "ordinary target selection changes must not erase a live orb countdown");
assert.match(reportSource, /`boss-\$\{mechanicKey\}`/,
    "each Lei Neien mechanic must address its own private built-in sound");
assert.match(reportSource, /\(isStandalone\.value \|\| force\)[\s\S]{0,220}playBossMechanicSound/,
    "desktop live Boss events must leave sound ownership to the native runtime while preserving previews");

function readPcm16Wave(filename: string, expectedSeconds: number) {
    const data = readFileSync(new URL(`../public/audio/${filename}`, import.meta.url));
    assert.equal(data.subarray(0, 4).toString(), "RIFF");
    assert.equal(data.subarray(8, 12).toString(), "WAVE");
    assert.equal(data.readUInt16LE(22), 1, `${filename} must be mono`);
    assert.equal(data.readUInt32LE(24), 44_100, `${filename} must use 44.1 kHz`);
    assert.equal(data.readUInt16LE(34), 16, `${filename} must use PCM16`);
    assert.equal((data.length - 44) / 2 / 44_100, expectedSeconds, `${filename} duration`);
    return (start: number, end: number) => {
        let peak = 0;
        const first = 44 + Math.floor(start * 44_100) * 2;
        const last = Math.min(data.length, 44 + Math.ceil(end * 44_100) * 2);
        for (let offset = first; offset + 1 < last; offset += 2) {
            peak = Math.max(peak, Math.abs(data.readInt16LE(offset)));
        }
        return peak;
    };
}

const orbPeak = readPcm16Wave("boss-miel-orb.wav", 20);
assert.equal(orbPeak(0, 9.95), 0, "orb audio stays silent until the ten-second Xiaoxiao cue");
assert(orbPeak(10, 11.4) > 2_000, "orb audio speaks 注意球 at ten seconds");
assert(orbPeak(13.5, 13.9) > 5_000, "orb audio has a distinct 13.5-second cue");
assert(orbPeak(14, 18.45) > 2_000, "orb audio restarts and accelerates after the first cue");
assert(orbPeak(18.5, 18.9) > 5_000, "orb audio has a distinct 18.5-second cue");

const laserPeak = readPcm16Wave("boss-miel-laser.wav", 5);
assert.equal(laserPeak(0, 3.49), 0, "Divine Sword stays silent until 3.5 seconds");
assert(laserPeak(3.5, 4.19) > 2_000, "Divine Sword accelerates from 3.5 seconds");
assert(laserPeak(4.2, 4.6) > 5_000, "Divine Sword emphasizes the 0.8-second-remaining point");

const start = 1_700_000_000_000;
assert.equal(resolveMielOrbHealthPhase(60, 100), "high", "60% belongs to the high-health phase");
assert.equal(resolveMielOrbHealthPhase(59.99, 100), "low");
assert.equal(resolveMielOrbHealthPhase(undefined, undefined), "unknown");

function addContacts(runtime: ReturnType<typeof createMielOrbRuntime>, offsets: number[]) {
    return offsets.reduce((state, offset) => observeMielOrbContact(state, start + offset), runtime);
}

// The 7615 full-health capture produced ten feedback ticks by +6.161s.  It
// remains visible until its +11s checkpoint instead of disappearing on hit #8.
let high = createMielOrbRuntime(start, 7615, 3_449_779_200, 3_449_779_200);
high = addContacts(high, [0, 503, 1_022, 1_525, 2_030, 4_116, 4_640, 5_161, 5_662, 6_161]);
assert.equal(high.healthPhase, "high");
assert.equal(high.contactCount, 10);
assert.equal(high.requiredContacts, 10, "7615 always uses its ten-contact rule, even at full health");
assert.equal(advanceMielOrbRuntime(high, start + 10_999).provisionallyHidden, false);
high = advanceMielOrbRuntime(high, start + 11_000);
assert.equal(high.provisionallyHidden, true, "a cleared 7615 orb hides only at the +11s checkpoint");
assert.equal(isMielOrbVisible(high, start + 11_000), false);

let insufficientHigh = createMielOrbRuntime(start, 7603, 80, 100);
insufficientHigh = addContacts(insufficientHigh, [0, 500, 1_000, 1_500, 2_000, 2_500, 3_000]);
insufficientHigh = advanceMielOrbRuntime(insufficientHigh, start + 8_000);
assert.equal(insufficientHigh.provisionallyHidden, false);
insufficientHigh = observeMielOrbContact(insufficientHigh, start + 8_500);
assert.equal(insufficientHigh.contactCount, 8);
assert.equal(
    advanceMielOrbRuntime(insufficientHigh, start + 8_500).provisionallyHidden,
    true,
    "reaching the required contact count after the checkpoint hides immediately",
);

let low = createMielOrbRuntime(start, 7603, 59, 100);
low = addContacts(low, [0, 500, 1_000, 1_500, 2_000, 2_500, 3_000, 3_500, 4_000, 4_500]);
assert.equal(low.requiredContacts, 8, "7603 keeps its eight-contact rule below 60% HP");
low = advanceMielOrbRuntime(low, start + 8_000);
assert.equal(low.provisionallyHidden, true, "a cleared 7603 orb hides at the +8s checkpoint");

let delayed7615 = createMielOrbRuntime(start, 7615, 100, 100);
delayed7615 = addContacts(delayed7615, [0, 500, 1_000, 1_500, 2_000, 2_500, 3_000, 3_500, 4_000]);
delayed7615 = advanceMielOrbRuntime(delayed7615, start + 11_000);
assert.equal(delayed7615.provisionallyHidden, false);
delayed7615 = observeMielOrbContact(delayed7615, start + 11_500);
assert.equal(delayed7615.provisionallyHidden, true, "7615 hides when its tenth contact arrives after the checkpoint");

const unknown = createMielOrbRuntime(start, 0);
assert.equal(unknown.healthPhase, "unknown");
assert.equal(unknown.requiredContacts, 10, "an unknown Boss variant takes the conservative ten-contact path");
assert.equal(unknown.checkpointAtMs, start + 11_000);

let grouped = createMielOrbRuntime(start, 7603, 100, 100);
grouped = addContacts(grouped, [0, 119, 120, 239, 240]);
assert.equal(grouped.contactCount, 3, "feedback less than 120ms apart belongs to one contact");

let resolved = createMielOrbRuntime(start, 7603, 100, 100);
resolved = addContacts(resolved, [0, 500, 1_000, 1_500, 2_000, 2_500, 3_000, 3_500]);
resolved = advanceMielOrbRuntime(resolved, start + 8_000);
assert.equal(resolved.provisionallyHidden, true);
assert.strictEqual(observeMielOrbLateConfirmation(resolved, start + 11_999), resolved);
assert.strictEqual(
    observeMielOrbLateConfirmation(resolved, start + 12_000),
    resolved,
    "a late confirmation cannot resurrect a contact-cleared orb at the 6.5-second warning threshold",
);
assert.equal(isMielOrbVisible(resolved, start + 13_500), false);
assert.equal(isMielOrbVisible(resolved, start + 19_999), false);
assert.equal(isMielOrbVisible(resolved, start + 20_000), false, "natural expiry still ends the lifecycle");

let unresolved = createMielOrbRuntime(start, 7603, 100, 100);
unresolved = advanceMielOrbRuntime(unresolved, start + 8_000);
unresolved = observeMielOrbLateConfirmation(unresolved, start + 12_000);
assert.equal(unresolved.provisionallyHidden, false);
assert.equal(unresolved.lateConfirmedAtMs, start + 12_000, "an unresolved orb still records its valid late confirmation");
assert.strictEqual(observeMielOrbLateConfirmation(unresolved, start + 14_751), unresolved);
assert.equal(isMielOrbVisible(unresolved, start + 19_999), true);

console.log("Boss mechanic profile migration verification passed");
