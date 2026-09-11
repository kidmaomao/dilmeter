import { strict as assert } from "node:assert";
import { readFile } from "node:fs/promises";
import { normalizeEffectTimerSettings } from "../src/effectTimer.ts";

const settings = normalizeEffectTimerSettings({
    rules: {
        boost: {
            key: "boost",
            enabled: true,
            sourceType: "skill",
            sourceId: 21002,
            name: "测试增益",
            durationSeconds: 7.5,
            alwaysVisible: true,
            targetMode: "monster",
            orientation: "vertical",
            scalePercent: 135,
            opacityPercent: 65,
            x: 810,
            y: 240,
        },
    },
});

assert.deepEqual(settings.rules.boost, {
    key: "boost",
    enabled: true,
    sourceType: "skill",
    sourceId: 21002,
    name: "测试增益",
    durationSeconds: 7.5,
    alwaysVisible: true,
    targetMode: "monster",
    orientation: "vertical",
    scalePercent: 135,
    opacityPercent: 65,
    x: 810,
    y: 240,
});

const clamped = normalizeEffectTimerSettings({ rules: { x: {
    sourceId: 0, durationSeconds: 0, scalePercent: 900, opacityPercent: 0, x: 99_999, y: -99_999,
} } });
assert.equal(clamped.rules.x.sourceId, 1);
assert.equal(clamped.rules.x.durationSeconds, 0.1);
assert.equal(clamped.rules.x.scalePercent, 200);
assert.equal(clamped.rules.x.opacityPercent, 20);
assert.equal(clamped.rules.x.x, 32_000);
assert.equal(clamped.rules.x.y, -32_000);

const overlaySource = await readFile(new URL("../src/components/SkillCooldownOverlay.vue", import.meta.url), "utf8");
const appSource = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
assert.match(overlaySource, /v-if="item\.orientation !== 'vertical'"/, "vertical timers omit the effect name");
assert.match(overlaySource, /effect-timer-track[\s\S]*?effect-timer-remaining/, "remaining time is centered inside the progress track");
assert.match(overlaySource, /Math\.pow\(progress, \.85\)[\s\S]*?linear-gradient/, "timer colour moves from cool hues to red near expiry");
assert.match(overlaySource, /\.effect-timer-card\s*\{[\s\S]*?background:\s*none;[\s\S]*?box-shadow:\s*none;/, "effect timers have no redundant outer card");
assert.match(overlaySource, /effect-vertical[\s\S]*?width:\s*32px;[\s\S]*?height:\s*120px;/, "vertical timers keep a thick readable track");
assert.match(appSource, /Always merge the bundled CN snapshot[\s\S]*?await ensureCnResourceNames\(\)/, "bundled CN names always override an empty or stale cache");

console.log("effect timer settings normalize skill/condition bars and visual bounds");
