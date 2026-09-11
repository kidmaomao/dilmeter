import { strict as assert } from "node:assert";
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

console.log("effect timer settings normalize skill/condition bars and visual bounds");


