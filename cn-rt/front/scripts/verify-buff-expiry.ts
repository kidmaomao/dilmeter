import { strict as assert } from "node:assert";
import {
    isStackOnlyBuffAlertCondition,
    makeBuffAlertRule,
    parseConditionStack,
    resolveBuffExpiresAt,
    sanitizeBuffAlertRule,
} from "../src/buffAlert.ts";

const capturedAt = 1_800_000_000.123;
const condition = {
    Id: "player",
    At: Math.floor(capturedAt),
    CCId: 680,
    // The legacy whole-second value remains upward rounded for compatibility.
    DisableAt: 1_800_000_301,
    // New captures preserve the exact 300-second expiry.
    DisableAtMs: 1_800_000_300_123,
    AttackerId: "player",
    Metadata: "DUR:4:300000",
    DurationMs: 300_000,
};

const preciseExpiry = resolveBuffExpiresAt(condition);
assert.equal(preciseExpiry, 1_800_000_300.123);
assert.equal(Math.ceil((preciseExpiry ?? 0) - capturedAt), 300, "300-second Buff must start at 300");

const legacyExpiry = resolveBuffExpiresAt({ ...condition, DisableAtMs: undefined });
assert.equal(legacyExpiry, condition.DisableAt, "old logs still use the compatible whole-second expiry");

assert.equal(resolveBuffExpiresAt(condition, undefined, 2), preciseExpiry + 2, "+2 second global adjustment is exact");
assert.equal(resolveBuffExpiresAt(condition, undefined, -3), preciseExpiry - 3, "-3 second global adjustment is exact");
assert.equal(parseConditionStack("MCSTCT:2:1;MCSTMAX:2:5;"), 1);
assert.equal(parseConditionStack("MCSTCT:2:9;SBT:8:63921906330951;"), 9);
assert.equal(parseConditionStack('{"MCSTCT":7}'), 7);
assert.equal(parseConditionStack("SBT:8:63921906330951;"), null);

for (const ccId of [1080, 1098, 1153]) {
    assert.equal(isStackOnlyBuffAlertCondition(ccId), true);
    const rule = makeBuffAlertRule(ccId);
    assert.equal(rule.stackAlertEnabled, true, "verified stack conditions default to stack monitoring");
    assert.equal(rule.stackScreenEnabled, true, "verified stack conditions default to a screen flash");
    assert.equal(rule.overlayEnabled, false, "stack-only mechanics do not use an expiry icon");
    assert.deepEqual([rule.stackX, rule.stackY], [850, 280], "stack prompts have independent default coordinates");

    const migrated = sanitizeBuffAlertRule(ccId, {
        ...rule,
        overlayEnabled: true,
        flashEnabled: true,
        soundMode: "voice",
    });
    assert.equal(migrated.overlayEnabled, false, "old duration-overlay settings are removed from stack-only mechanics");
    assert.equal(migrated.flashEnabled, false, "old expiry flashing is removed from stack-only mechanics");
    assert.equal(migrated.soundMode, "none", "old expiry sounds are removed from stack-only mechanics");
    const legacyRule = sanitizeBuffAlertRule(ccId, { overlayEnabled: true, stackAlertEnabled: false });
    assert.equal(legacyRule.stackAlertEnabled, true, "an old expiry-icon rule migrates to stack monitoring once");
}

console.log("Buff expiry, stack metadata and stack-only alert rules verified");
