import { strict as assert } from "node:assert";
import {
    buildPositiveDamageTimeRange,
    formatBattleTargetTimeRange,
    formatLocalClockTime,
} from "../src/battleTargetTime.ts";

const startAt = new Date(2026, 6, 23, 17, 0, 0).getTime() / 1000;
const endAt = new Date(2026, 6, 23, 17, 20, 0).getTime() / 1000;

const range = buildPositiveDamageTimeRange([
    { At: endAt, Damage: 10 },
    { At: startAt - 60, Damage: 0 },
    { At: startAt, Damage: 1 },
    { At: startAt - 120, Damage: -50 },
]);

assert.deepEqual(range, { startAt, endAt }, "only positive hits define the battle range");
assert.equal(formatLocalClockTime(startAt), "17:00:00");
assert.equal(formatBattleTargetTimeRange(range), "17:00:00~17:20:00");
assert.equal(formatBattleTargetTimeRange(null), "");
assert.equal(buildPositiveDamageTimeRange([]), null);

console.log("battle target local time range verified");
