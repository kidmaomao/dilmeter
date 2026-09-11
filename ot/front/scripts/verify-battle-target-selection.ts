import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import {
    lockBattleTargetSelection,
    reconcileBattleTargetSelection,
    selectDamageBattleTarget,
} from "../src/battleTargetSelection.ts";

const targets = [
    { entityId: "chosen-boss", active: false },
    { entityId: "dot-target", active: true },
];

const lockedChoice = lockBattleTargetSelection("chosen-boss");
assert.deepEqual(
    reconcileBattleTargetSelection(lockedChoice, targets),
    lockedChoice,
    "an inactive report target remains selected while another target receives damage",
);
assert.deepEqual(
    selectDamageBattleTarget(lockedChoice, "dot-target", false),
    lockedChoice,
    "continued damage never replaces a visible user selection",
);

const afterLockedTargetDisappears = reconcileBattleTargetSelection(lockedChoice, [targets[1]]);
assert.deepEqual(
    afterLockedTargetDisappears,
    { selectedId: "dot-target", manuallyLockedId: "" },
    "removing a manually selected target returns the report to automatic mode",
);
assert.deepEqual(
    selectDamageBattleTarget(afterLockedTargetDisappears, "chosen-boss", false),
    { selectedId: "chosen-boss", manuallyLockedId: "" },
    "an automatically selected inactive target may follow the next active fight",
);

for (const manuallySelectedId of ["chosen-boss", "dot-target"]) {
    const relockedChoice = lockBattleTargetSelection(manuallySelectedId);
    assert.deepEqual(
        selectDamageBattleTarget(relockedChoice, manuallySelectedId === "chosen-boss" ? "dot-target" : "chosen-boss", false),
        relockedChoice,
        "a new manual selection restores the lock after an automatic switch",
    );
}

assert.deepEqual(
    selectDamageBattleTarget({ selectedId: "", manuallyLockedId: "" }, "dot-target", false),
    { selectedId: "dot-target", manuallyLockedId: "" },
);
assert.deepEqual(
    reconcileBattleTargetSelection({ selectedId: "missing", manuallyLockedId: "" }, targets),
    { selectedId: "dot-target", manuallyLockedId: "" },
);
assert.deepEqual(
    reconcileBattleTargetSelection({ selectedId: "", manuallyLockedId: "" }, []),
    { selectedId: "", manuallyLockedId: "" },
);

const reportSource = readFileSync(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
assert.match(reportSource, /@change="handleBattleTargetSelected"/);
assert.match(reportSource, /manuallyLockedBossId\.value = next\.manuallyLockedId/);
assert.match(reportSource, /selectDamageBattleTarget\(/);

console.log("stable battle target selection verified");
