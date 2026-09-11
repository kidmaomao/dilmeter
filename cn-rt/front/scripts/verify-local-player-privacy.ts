import { strict as assert } from "node:assert";
import { selectMainReportPlayer } from "../src/localPlayerSelection.ts";

const players = [
    { entityId: "self", name: "本人" },
    { entityId: "party-1", name: "队员" },
];

assert.equal(selectMainReportPlayer(players, "self", "party-1")?.entityId, "self", "local identity must win");
assert.equal(selectMainReportPlayer(players, "", "party-1")?.entityId, "party-1", "imported records keep their explicit selection");
assert.equal(selectMainReportPlayer(players, "")?.entityId, undefined, "live main UI must never fall back to the first teammate");

console.log("Main report local-player privacy verified");
