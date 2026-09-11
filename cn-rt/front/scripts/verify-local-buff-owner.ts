import { strict as assert } from "node:assert";
import {
    findLocalBuffCondition,
    type LocalBuffManager,
} from "../src/buffOwnership.ts";

type Condition = { At: number; label: string };
const condition = (label: string, at: number): Condition => ({ label, At: at });
const local = condition("local", 100);
const other = condition("other player", 200);
const pet = condition("pet", 300);

const manager: LocalBuffManager<Condition> = {
    localEntityId: "player-local",
    localEntityReliable: true,
    entityMap: {
        "player-local": { isPC: true, conditionMap: { 1225: local } },
        "player-other": { isPC: true, conditionMap: { 1225: other } },
        "pet-other": { isPC: false, conditionMap: { 1225: pet } },
    },
    pendingConditionMap: {},
};

assert.equal(findLocalBuffCondition(manager, 1225)?.condition, local,
    "the local target must win even when another player has a newer copy");

manager.localEntityReliable = false;
assert.equal(findLocalBuffCondition(manager, 1225)?.condition, local,
    "a known PC actor must not depend on pet-assisted reliability confirmation");
manager.localEntityReliable = true;

delete manager.entityMap["player-local"].conditionMap[1225];
assert.equal(findLocalBuffCondition(manager, 1225), null,
    "another player or pet must not trigger the local reminder");

manager.localEntityId = "pet-other";
assert.equal(findLocalBuffCondition(manager, 1225), null,
    "a pet target must not be treated as the local player");

manager.localEntityId = "pending-player";
manager.pendingConditionMap = {
    "pending-player": { 1225: local },
    "pending-other": { 1225: other },
};
assert.equal(findLocalBuffCondition(manager, 1225)?.condition, local,
    "a confirmed mid-map local id may use only its own pending condition");

manager.localEntityReliable = false;
assert.equal(findLocalBuffCondition(manager, 1225)?.condition, local,
    "a provisional exact player id must work when CN omits the player's EntityAppear packet");

manager.entityMap["pending-player"] = { isPC: false, conditionMap: { 1225: pet } };
assert.equal(findLocalBuffCondition(manager, 1225), null,
    "a provisional id must be rejected as soon as it is identified as a pet");

delete manager.entityMap["pending-player"];
manager.localEntityId = "";
assert.equal(findLocalBuffCondition(manager, 1225), null,
    "no Buff may be selected before a private-stat target id is known");

console.log("local-player Buff ownership filtering verified");
