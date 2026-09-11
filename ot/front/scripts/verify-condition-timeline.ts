import { strict as assert } from "node:assert";
import { buildConditionTimelineScale } from "../src/conditionTimeline.ts";

const wide = buildConditionTimelineScale(20 * 60 + 22, 1_333);
assert.equal(wide.majorStep, 120, "wide 20:22 timeline should label every two minutes");
assert.equal(wide.minorStep, 30, "wide 20:22 timeline should retain 30-second guide marks");
assert.equal(wide.labelTicks[0]?.seconds, 0, "timeline must keep its origin label");
assert.equal(wide.labelTicks.at(-1)?.seconds, 1_222, "timeline must keep its exact end label");

const narrow = buildConditionTimelineScale(20 * 60 + 22, 600);
assert.equal(narrow.majorStep, 180, "narrow 20:22 timeline should reduce label density");
assert.equal(narrow.minorStep, 60, "narrow timeline should retain minute guide marks");

for (let index = 1; index < narrow.labelTicks.length; index += 1) {
    const gapPct = narrow.labelTicks[index].pct - narrow.labelTicks[index - 1].pct;
    assert.ok(gapPct * 6 >= 54, "adjacent narrow labels should keep at least 54px apart");
}

const empty = buildConditionTimelineScale(0, 900);
assert.deepEqual(empty.labelTicks.map((tick) => tick.seconds), [0]);
assert.equal(empty.gridTicks.length, 0);

console.log("adaptive Buff timeline scale verified");
