import { strict as assert } from "node:assert";
import { normalizeEffectTimerSettings, endTransformationEffectTimers, type EffectTimerRuntime } from "../src/effectTimer.ts";

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
assert.equal(clamped.rules.x.sourceId, 0, "condition ID 0 remains selectable after saving");
assert.equal(normalizeEffectTimerSettings({ rules: { skill: { sourceType: "skill", sourceId: 0 } } }).rules.skill.sourceId, 1);
assert.equal(clamped.rules.x.durationSeconds, 0.1);
assert.equal(clamped.rules.x.scalePercent, 200);
assert.equal(clamped.rules.x.opacityPercent, 20);
assert.equal(clamped.rules.x.x, 32_000);
assert.equal(clamped.rules.x.y, -32_000);

console.log("effect timer settings normalize skill/condition bars and visual bounds");

for (const skillId of [40001, 41001, 42001, 43001, 44001]) {
    const rules = normalizeEffectTimerSettings({ rules: {
        transform: { sourceType: "skill", sourceId: skillId, targetMode: "self", alwaysVisible: true },
        ordinary: { sourceType: "skill", sourceId: 21002, targetMode: "self" },
        monster: { sourceType: "skill", sourceId: skillId, targetMode: "monster" },
    } }).rules;
    const runtime: Record<string, EffectTimerRuntime> = Object.fromEntries(Object.keys(rules).map(key =>
        [key, { startedAtMs: 100123, endsAtMs: 1900123, targetId: "self", generation: 1 }]));
    const list = Object.values(rules);
    assert.equal(endTransformationEffectTimers(list, runtime, "other", skillId, 102000), false);
    assert.equal(endTransformationEffectTimers(list, runtime, "self", skillId, 99999), false, "old cancellation cannot clear a new cast");
    assert.equal(endTransformationEffectTimers(list, runtime, "self", 21002, 102000), false);
    assert.equal(endTransformationEffectTimers(list, runtime, "self", skillId, 102000), true);
    assert.deepEqual(Object.keys(runtime).sort(), ["monster", "ordinary"], "only the matching local transformation ends");
    assert.equal(rules.transform.alwaysVisible, true, "empty-bar preference is preserved");
    assert.equal(endTransformationEffectTimers(list, runtime, "self", skillId, 102000), false, "duplicate cancellation is harmless");
    runtime.transform = { startedAtMs: 103123, endsAtMs: 1903123, targetId: "self", generation: 2 };
    assert.equal(endTransformationEffectTimers(list, runtime, "self", 0), true, "death/session reset clears transformations without a skill id");
}
console.log("Transformation duration cancellation verified for human, elf and giant; unrelated timers and empty-bar settings preserved.");

