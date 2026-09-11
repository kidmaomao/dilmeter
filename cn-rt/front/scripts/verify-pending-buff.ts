import { strict as assert } from "node:assert";
import path from "node:path";
import { Buffer } from "node:buffer";
import { build } from "esbuild";
import { makeBuffAlertRule, resolveBuffExpiresAt } from "../src/buffAlert.ts";
import type {
    eventCharacterConditionDisable,
    eventCharacterConditionEnable,
    eventEntityDisappear,
    eventEntityAppear,
} from "../src/protocols.ts";

// eventActor uses Vite's @ alias. Bundle it in memory so this verification can
// execute the production class directly without adding a test-only runtime.
const frontRoot = path.resolve(import.meta.dirname, "..");
const bundle = await build({
    entryPoints: [path.join(frontRoot, "src/eventActor.ts")],
    alias: { "@": path.join(frontRoot, "src") },
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node24",
    write: false,
    logLevel: "silent",
});
const moduleUrl = `data:text/javascript;base64,${Buffer.from(bundle.outputFiles[0].text).toString("base64")}`;
const { ActorManager } = await import(moduleUrl) as typeof import("../src/eventActor.ts");
const pureBundle = await build({
    entryPoints: [path.join(frontRoot, "src/worker/pureEventActor.ts")],
    alias: { "@": path.join(frontRoot, "src") },
    bundle: true,
    format: "esm",
    platform: "node",
    target: "node24",
    write: false,
    logLevel: "silent",
});
const pureModuleUrl = `data:text/javascript;base64,${Buffer.from(pureBundle.outputFiles[0].text).toString("base64")}`;
const { PureActorManager, PureDamageCollectorManager } = await import(pureModuleUrl) as typeof import("../src/worker/pureEventActor.ts");

const actorId = "4503599631566088";
const enable = (ccId: number, at: number): eventCharacterConditionEnable => ({
    EventId: 4,
    At: at,
    Id: actorId,
    CCId: ccId,
    DisableAt: at + 60,
    AttackerId: actorId,
    Metadata: "",
    DurationMs: 60000,
});
const disable = (ccId: number, at: number): eventCharacterConditionDisable => ({
    EventId: 5,
    At: at,
    Id: actorId,
    CCId: ccId,
});
const appear: eventEntityAppear = {
    EventId: 1,
    At: 99,
    Id: actorId,
    Name: "测试角色",
    RaceId: 10001,
    Height: 1,
    Weight: 1,
    Upper: 1,
    Lower: 1,
    GuildName: "",
    OwnerId: "",
};
const newManager = () => new ActorManager({ onDamage() {} } as any);

// A map transition temporarily removes the local player from the visible
// entity set. It must not end a Buff that remains active on the server.
const mapTransition = newManager();
mapTransition.onEvent(appear);
mapTransition.onEvent({ EventId: 11, At: 100, Id: actorId, Reliable: true, Reset: false });
mapTransition.onEvent(enable(511, 101));
mapTransition.onEvent({ EventId: 2, At: 120, Id: actorId } as eventEntityDisappear);
assert.equal(mapTransition.entityMap[actorId]?.conditionMap[511]?.At, 101,
    "local-player map disappearance must preserve an active Buff");
mapTransition.onEvent({ ...appear, At: 123 });
assert.equal(mapTransition.entityMap[actorId]?.conditionMap[511]?.At, 101,
    "local-player Buff must remain continuous after the new map snapshot starts");
mapTransition.onEvent(disable(511, 130));
assert.equal(mapTransition.entityMap[actorId]?.conditionMap[511], undefined,
    "an explicit CC disable after changing maps must still end the Buff");

// Visibility loss for non-local entities must keep its old cleanup semantics.
const otherTransition = newManager();
otherTransition.onEvent(appear);
otherTransition.onEvent(enable(511, 201));
otherTransition.onEvent({ EventId: 2, At: 220, Id: actorId } as eventEntityDisappear);
assert.equal(otherTransition.entityMap[actorId]?.conditionMap[511], undefined,
    "non-local entity disappearance must still close live conditions");

// Starting mid-map still preserves a pending condition and its parsed duration.
const pendingRecognition = newManager();
pendingRecognition.onEvent(enable(1023, 100));
assert.equal(pendingRecognition.pendingConditionMap[actorId]?.[1023]?.DurationMs, 60000);
pendingRecognition.onEvent(appear);
assert.equal(pendingRecognition.pendingConditionMap[actorId], undefined);
assert.equal(pendingRecognition.entityMap[actorId]?.conditionMap[1023]?.DurationMs, 60000);

// A first-generation condition can legitimately disappear immediately. It
// must not be mistaken for the delayed remove packet of an earlier generation.
const firstPending = newManager();
firstPending.onEvent(enable(511, 200));
firstPending.onEvent(disable(511, 200));
assert.equal(firstPending.pendingConditionMap[actorId], undefined,
    "same-second disable after the first pending enable must clear CC 511");

const firstKnown = newManager();
firstKnown.onEvent(appear);
firstKnown.onEvent(enable(511, 210));
firstKnown.onEvent(disable(511, 211.25));
assert.equal(firstKnown.entityMap[actorId]?.conditionMap[511], undefined,
    "disable within 1.25 seconds after the first known enable must clear CC 511");

// Only a real refresh arms the guard. It protects the new generation from one
// immediately following old-generation remove, then is consumed.
const refreshedPending = newManager();
refreshedPending.onEvent(enable(511, 300));
refreshedPending.onEvent(enable(511, 301));
refreshedPending.onEvent(disable(511, 301.5));
assert.equal(refreshedPending.pendingConditionMap[actorId]?.[511]?.At, 301,
    "one old-generation pending remove must not cancel a refreshed condition");
refreshedPending.onEvent(disable(511, 301.75));
assert.equal(refreshedPending.pendingConditionMap[actorId], undefined,
    "the pending refresh guard must be consumed after one remove");

const refreshedKnown = newManager();
refreshedKnown.onEvent(appear);
refreshedKnown.onEvent(enable(511, 400));
refreshedKnown.onEvent(enable(511, 401));
refreshedKnown.onEvent(disable(511, 401.5));
assert.equal(refreshedKnown.entityMap[actorId]?.conditionMap[511]?.At, 401,
    "one old-generation known-actor remove must not cancel a refreshed condition");
refreshedKnown.onEvent(disable(511, 401.75));
assert.equal(refreshedKnown.entityMap[actorId]?.conditionMap[511], undefined,
    "the known-actor refresh guard must be consumed after one remove");

// Preserve the one-use guard if EntityAppear arrives between a pending refresh
// and the previous generation's delayed remove.
const promotedRefresh = newManager();
promotedRefresh.onEvent(enable(511, 500));
promotedRefresh.onEvent(enable(511, 501));
promotedRefresh.onEvent({ ...appear, At: 501 });
promotedRefresh.onEvent(disable(511, 501.5));
assert.equal(promotedRefresh.entityMap[actorId]?.conditionMap[511]?.At, 501,
    "pending refresh protection must survive promotion to a known actor");
promotedRefresh.onEvent(disable(511, 501.75));
assert.equal(promotedRefresh.entityMap[actorId]?.conditionMap[511], undefined,
    "promoted refresh protection must still be one-use");

// The protection is only for an immediately following remove.
const lateRemove = newManager();
lateRemove.onEvent(appear);
lateRemove.onEvent(enable(511, 600));
lateRemove.onEvent(enable(511, 601));
lateRemove.onEvent(disable(511, 603));
assert.equal(lateRemove.entityMap[actorId]?.conditionMap[511], undefined,
    "a later legitimate remove must clear a refreshed condition");

const automaticRule = makeBuffAlertRule(680);
assert.equal(resolveBuffExpiresAt({
    Id: actorId,
    At: 100,
    CCId: 680,
    DisableAt: 0,
    AttackerId: "",
    Metadata: "",
    DurationMs: 0,
}, automaticRule), null, "automatic mode must not use a fallback duration");

const sharpMetadata = "SSAD:f:10;SSCRI:f:10;SSRT:f:20;MCAGT:8:63921310753919;SDUR:4:300000;SBT:8:63921311053919;";
assert.equal(resolveBuffExpiresAt({
    Id: actorId,
    At: 1785685161,
    CCId: 511,
    DisableAt: 1785685453,
    AttackerId: "",
    Metadata: sharpMetadata,
    DurationMs: 4,
}, makeBuffAlertRule(511)), 1785685453,
"CC511 must prefer its real DisableAt over the misparsed legacy 4 ms duration");

assert.equal(resolveBuffExpiresAt({
    Id: actorId,
    At: 100,
    CCId: 511,
    DisableAt: 0,
    AttackerId: "",
    Metadata: sharpMetadata,
    DurationMs: 4,
}, makeBuffAlertRule(511)), 400,
"typed SDUR:4:300000 metadata must recover a five-minute duration in old logs");

for (const ccId of [511, 512, 513, 514, 515]) {
    const pure = new PureActorManager(new PureDamageCollectorManager());
    pure.onEvent(appear);
    pure.onEvent(enable(ccId, 700));
    assert.equal(pure.entityMap[actorId]?.conditionMap[ccId]?.At, 700,
        `pure worker must retain the first enable for CC${ccId}`);
    pure.onEvent(disable(ccId, 700.5));
    assert.equal(pure.entityMap[actorId]?.conditionMap[ccId], undefined,
        `pure worker must not suppress a first quick disable for CC${ccId}`);

    pure.onEvent(enable(ccId, 710));
    pure.onEvent(enable(ccId, 711));
    pure.onEvent(disable(ccId, 711.5));
    assert.equal(pure.entityMap[actorId]?.conditionMap[ccId]?.At, 711,
        `pure worker must consume one stale refresh remove for CC${ccId}`);
    pure.onEvent(disable(ccId, 711.75));
    assert.equal(pure.entityMap[actorId]?.conditionMap[ccId], undefined,
        `pure worker refresh guard must be one-use for CC${ccId}`);
}

console.log("pending and known-actor Buff refresh protection verified");
