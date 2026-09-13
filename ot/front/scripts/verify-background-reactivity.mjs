import { strict as assert } from "node:assert";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const frontRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({
    root: frontRoot,
    configFile: false,
    appType: "custom",
    logLevel: "error",
    server: { middlewareMode: true },
    optimizeDeps: { noDiscovery: true, include: [] },
    resolve: {
        alias: {
            "@": path.resolve(frontRoot, "src"),
        },
    },
});

const nativeSetTimeout = globalThis.setTimeout;
const nativeClearTimeout = globalThis.clearTimeout;
const suspendedTimers = new Map();
let nextTimerId = 1;

try {
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const { PureActorManager, PureDamageCollectorManager } = await server.ssrLoadModule("/src/worker/pureEventActor.ts");

    // Simulate WebView/browser background throttling: callbacks are accepted but
    // never run until the native heartbeat explicitly flushes the pending state.
    globalThis.setTimeout = ((callback, delay) => {
        const id = nextTimerId++;
        suspendedTimers.set(id, { callback, delay });
        return id;
    });
    globalThis.clearTimeout = ((id) => {
        suspendedTimers.delete(Number(id));
    });

    const damageManager = new DamageCollectorManager();
    const collector = damageManager.getFilteredDamageCollector(() => true);
    let collectorTriggers = 0;
    collector.setUpdateCallback(() => {}, () => { collectorTriggers += 1; });
    damageManager.onDamage({
        Id: "local",
        At: 100,
        TargetId: "boss",
        SkillId: 35024,
        Damage: 42,
        IsCritical: false,
        IsDelayed: false,
        Conditions: [],
        TargetConditions: [],
        PetId: "",
    });

    assert.equal(collector.totalDamage, 42, "damage data is recorded before a render flush");
    assert.equal(collectorTriggers, 0, "a suspended browser timer cannot update the view by itself");
    assert.equal(suspendedTimers.size, 1, "the collector has one pending throttled update");
    damageManager.flushPendingUpdates();
    assert.equal(collectorTriggers, 1, "collector flush immediately triggers its reactive subscriber");
    assert.equal(suspendedTimers.size, 0, "collector flush cancels its stale browser timer");
    damageManager.flushPendingUpdates();
    assert.equal(collectorTriggers, 1, "an idle flush is side-effect free");

    const actorManager = new ActorManager(damageManager);
    actorManager.onEntityAppear({
        EventId: 1,
        At: 101,
        Id: "local",
        Name: "Local Player",
        RaceId: 10001,
        Height: 1,
        Weight: 1,
        Upper: 1,
        Lower: 1,
        GuildName: "",
        OwnerId: "",
    });
    actorManager.flushPendingUpdates();
    const actor = actorManager.entityMap.local;
    let actorTriggers = 0;
    actor.setUpdateCallback(() => {}, () => { actorTriggers += 1; });
    actor.onUpdateBody({
        EventId: 9,
        At: 102,
        Id: "local",
        Height: 1.2,
        Weight: 0.9,
        Upper: 1.1,
        Lower: 0.8,
    });

    assert.equal(actorTriggers, 0, "a suspended actor timer cannot update the view by itself");
    assert.equal(actor.body.Height, 1.2, "actor data is recorded before a render flush");
    actorManager.flushPendingUpdates();
    assert.equal(actorTriggers, 1, "actor flush immediately triggers its reactive subscriber");
    actorManager.flushPendingUpdates();
    assert.equal(actorTriggers, 1, "an idle actor flush is side-effect free");

    const bossAppear = {
        EventId: 1,
        At: 103,
        Id: "boss",
        Name: "Boss",
        RaceId: 7603,
        Height: 1,
        Weight: 1,
        Upper: 1,
        Lower: 1,
        GuildName: "",
        OwnerId: "",
    };
    const bossStats = {
        EventId: 17,
        At: 104,
        Id: "boss",
        Private: false,
        Stats: [
            { StatId: 28, Value: 1_500_000_000 },
            { StatId: 30, Value: 1_967_880_064 },
        ],
    };
    const channelReset = {
        EventId: 11,
        At: 105,
        Id: "local",
        Reliable: true,
        Reset: true,
    };

    actorManager.onEvent(bossAppear);
    actorManager.onEvent(bossStats);
    actorManager.onEvent(channelReset);
    assert.equal(actorManager.entityMap.boss.statMap[30], 1_967_880_064, "live reset preserves authoritative Boss maximum health");
    assert.equal(actorManager.entityMap.boss.statMap[28], undefined, "live reset drops stale current health");

    const pureActors = new PureActorManager(new PureDamageCollectorManager());
    pureActors.onEvent(bossAppear);
    pureActors.onEvent(bossStats);
    pureActors.onEvent(channelReset);
    assert.equal(pureActors.entityMap.boss.statMap[30], 1_967_880_064, "worker replay reset preserves authoritative Boss maximum health");
    assert.equal(pureActors.entityMap.boss.statMap[28], undefined, "worker replay reset drops stale current health");
} finally {
    globalThis.setTimeout = nativeSetTimeout;
    globalThis.clearTimeout = nativeClearTimeout;
    await server.close();
}

const appSource = await readFile(new URL("../src/App.vue", import.meta.url), "utf8");
const socketSource = await readFile(new URL("../src/lib/socketClient.ts", import.meta.url), "utf8");
const reportSource = await readFile(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
assert.match(
    appSource,
    /window\.addEventListener\("dilmeter-native-tick",\s*nativeReactiveTickListener\)/,
    "the main view must subscribe to the host's unthrottled native tick",
);
assert.match(
    appSource,
    /nativeReactiveTickListener\s*=\s*\(\(\)\s*=>\s*\{[\s\S]*?flushPendingUiUpdates\(\)/,
    "each native tick must flush pending actor and damage updates",
);
assert.match(
    appSource,
    /window\.addEventListener\("dilmeter-host-window-state",\s*hostWindowStateListener\)/,
    "the main view must subscribe to native minimize/restore state",
);
assert.match(
    appSource,
    /hostWindowStateListener\s*=\s*\(\(event:[\s\S]*?recoverAfterBackground\(/,
    "restoring the native window must run the authoritative recovery path",
);
assert.match(
    appSource,
    /socket\.ensureConnected\(\)/,
    "background recovery must repair a disconnected live event stream",
);
assert.match(
    appSource,
    /uiDormantNeedsReload\s*=\s*true[\s\S]*?socket\.suspend\(\)/,
    "a hidden DPS renderer must stop consuming reconstructable live events",
);
assert.doesNotMatch(
    appSource,
    /awayMs\s*>=\s*60_000/,
    "elapsed background time alone must not force a competing full-log reload",
);
assert.match(
    socketSource,
    /existingState\s*===\s*WebSocket\.OPEN\s*\|\|\s*existingState\s*===\s*WebSocket\.CONNECTING/,
    "connection recovery must not create duplicate sockets while one is connecting",
);
assert.match(
    socketSource,
    /public\s+suspend\(\)[\s\S]*?this\.suspended\s*=\s*true/,
    "the display socket must expose an explicit dormant mode",
);
assert.match(
    reportSource,
    /skillCooldowns:\s*\{[\s\S]*?rules:\s*skillRules/,
    "skill cooldown rules must be synchronized to the native reminder runtime",
);
assert.match(
    reportSource,
    /bossMechanics:\s*\{[\s\S]*?\.\.\.bossMechanicSettings\.value/,
    "Boss mechanic rules must be synchronized to the native reminder runtime",
);
assert.match(
    reportSource,
    /if\s*\(isStandalone\.value\)\s*\{\s*for\s*\(const rule of readyRules\) void playSkillReadySound\(rule\)/,
    "desktop skill-ready sounds must not depend on the renderer timer",
);
assert.match(
    reportSource,
    /function publishSkillCooldownOverlayState\(forceDesktopPreview = false\)[\s\S]*?if\s*\(!isStandalone\.value && !forceDesktopPreview\)\s*return/,
    "desktop overlays must allow an explicit one-shot user preview",
);
assert.match(
    reportSource,
    /function triggerBossMechanic\([\s\S]*?publishSkillCooldownOverlayState\(force\)/,
    "the Boss mechanic test button must publish its forced preview",
);
assert.match(
    reportSource,
    /function previewSkillCooldown\([\s\S]*?publishSkillCooldownOverlayState\(true\)[\s\S]*?publishSkillCooldownOverlayState\(true\)/,
    "both Toah and ordinary skill previews must publish on desktop",
);

console.log("background-throttled actor and damage reactivity verified");
