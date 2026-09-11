import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const cacheDir = fs.mkdtempSync(path.join(os.tmpdir(), "dilmetercn-vite-verify-"));
const server = await createServer({
    configFile: false,
    root,
    cacheDir,
    logLevel: "error",
    server: { middlewareMode: true },
    resolve: { alias: { "@": path.join(root, "src") } },
    optimizeDeps: { noDiscovery: true, include: [] },
});

try {
    const {
        BATTLE_RECORD_FORMAT,
        BATTLE_RECORD_VERSION,
        battleRecordFilename,
        createBattleRecord,
        parseBattleRecord,
    } = await server.ssrLoadModule("/src/battleRecord.ts");
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const {
        PureActorManager,
        PureDamageCollectorManager,
    } = await server.ssrLoadModule("/src/worker/pureEventActor.ts");
    const { buildBossSummary, detectJob } = await server.ssrLoadModule("/src/summaryCollector.ts");
    const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
    const {
        normalizeSkillDisplayName,
    } = await server.ssrLoadModule("/src/skillDisplay.ts");
    const {
        isMarionetteDamageSkill,
    } = await server.ssrLoadModule("/src/ownedDamagePolicy.ts");

    const emptyBody = { Height: 1, Weight: 1, Upper: 1, Lower: 1 };
    const damage = (id, targetId, at, amount, skillId = 10001) => ({
        Id: id,
        At: at,
        TargetId: targetId,
        SkillId: skillId,
        Damage: amount,
        IsCritical: false,
        IsDelayed: false,
        Conditions: [],
        TargetConditions: [],
        PetId: "",
    });
    const group = (id, raceId, name) => ({
        id,
        raceId,
        name,
        body: emptyBody,
        takeDamages: [],
    });
    const entity = ({ id, raceId, name, isPC, group: entityGroup, ownerId = "", takeDamages = [], applyDamages = [] }) => ({
        id,
        raceId,
        name,
        isPC,
        group: entityGroup,
        guildName: "",
        ownerId,
        finisherId: "",
        body: emptyBody,
        takeDamages,
        applyDamages,
        conditionHistory: [],
        equipItemMap: {},
    });
    const appear = (id, raceId, name, ownerId = "") => ({
        EventId: 1,
        At: 1,
        Id: id,
        RaceId: raceId,
        Name: name,
        Height: 1,
        Weight: 1,
        Upper: 1,
        Lower: 1,
        GuildName: "",
        OwnerId: ownerId,
    });
    const rawHit = (id, targetId, at, amount, skillId) => ({
        EventId: 3,
        Id: id,
        TargetId: targetId,
        At: at,
        Damage: amount,
        SkillId: skillId,
        IsCritical: false,
        IsDelayed: false,
    });
    const health = (id, at, value) => ({
        EventId: 17,
        Id: id,
        At: at,
        Private: false,
        Stats: [{ StatId: 28, Value: value }],
    });

    const bossGroup = group("7600", 7600, "7600");
    const p1Group = group("player-1", 10001, "玩家一");
    const p2Group = group("player-2", 10001, "玩家二");
    const unrelatedGroup = group("player-x", 10001, "场外玩家");
    const p1Hit = damage("player-1", "boss-1", 100, 100, 20001);
    const p2Hit = damage("player-2", "boss-1", 101, 200, 20002);
    const outsideHit = damage("player-1", "other-boss", 99, 999, 20003);
    const boss = entity({
        id: "boss-1",
        raceId: 7600,
        name: "测试首领",
        isPC: false,
        group: bossGroup,
        takeDamages: [p1Hit, p2Hit],
    });
    const p1 = entity({
        id: "player-1",
        raceId: 10001,
        name: "玩家一",
        isPC: true,
        group: p1Group,
        applyDamages: [outsideHit, p1Hit],
    });
    const p2 = entity({
        id: "player-2",
        raceId: 10001,
        name: "玩家二",
        isPC: true,
        group: p2Group,
        applyDamages: [p2Hit],
    });
    const unrelated = entity({
        id: "player-x",
        raceId: 10001,
        name: "场外玩家",
        isPC: true,
        group: unrelatedGroup,
        applyDamages: [outsideHit],
    });

    const actorManager = {
        entityMap: { [boss.id]: boss, [p1.id]: p1, [p2.id]: p2, [unrelated.id]: unrelated },
        damages: [outsideHit, p1Hit, p2Hit],
        effectiveDamages: [p1Hit, p2Hit],
    };
    const dcManager = { damages: [outsideHit, p1Hit, p2Hit] };

    const record = createBattleRecord(actorManager, dcManager, boss.id, p2.id, { bossName: "测试首领" });
    assert.match(battleRecordFilename(record), /\.json$/);
    assert.equal(record.format, BATTLE_RECORD_FORMAT);
    assert.equal(record.version, BATTLE_RECORD_VERSION);
    assert.equal(record.battle.totalDamage, 300);
    assert.equal(record.battle.playerCount, 2);
    assert.equal(record.selection.playerEntityId, p2.id);
    assert.deepEqual(Object.keys(record.snapshot.entities).sort(), [boss.id, p1.id, p2.id].sort());
    assert.equal(record.snapshot.entities[p1.id].totalApplyDamage, 100);
    assert.equal(record.snapshot.collectorDamages.length, 2);

    const roundTrip = parseBattleRecord(JSON.stringify(record));
    assert.equal(roundTrip?.battle.totalDamage, 300);
    assert.equal(roundTrip?.selection.bossEntityId, boss.id);

    const hydratedDamageManager = new DamageCollectorManager();
    const hydratedActorManager = new ActorManager(hydratedDamageManager);
    hydrateFromSnapshot(roundTrip.snapshot, hydratedActorManager, hydratedDamageManager);
    const hydratedSummary = buildBossSummary(boss.id, hydratedActorManager, []);
    assert.equal(hydratedSummary?.totalDamage, 300);
    assert.equal(hydratedSummary?.players.length, 2);
    assert.equal(hydratedSummary?.players.find((player) => player.entityId === p1.id)?.totalDamage, 100);
    assert.equal(hydratedSummary?.players.find((player) => player.entityId === p2.id)?.totalDamage, 200);

    // Contribution uses authoritative Boss-body health loss as its denominator.
    // Pet/system damage removes real health, but remains excluded from player DPS.
    const denominatorBossGroup = group("7603-denominator", 7603, "7603-denominator");
    const denominatorPlayerGroup = group("denominator-player", 10001, "denominator-player");
    const denominatorPetGroup = group("denominator-pet", 990103, "denominator-pet");
    const denominatorPlayerHit = damage("denominator-player", "denominator-boss", 300, 100, 59040);
    const denominatorPetHit = damage("denominator-pet", "denominator-boss", 301, 50, 10001);
    const denominatorBoss = entity({
        id: "denominator-boss", raceId: 7603, name: "denominator-boss",
        isPC: false, group: denominatorBossGroup,
        takeDamages: [denominatorPlayerHit, denominatorPetHit],
    });
    const denominatorPlayer = entity({
        id: "denominator-player", raceId: 10001, name: "denominator-player",
        isPC: true, group: denominatorPlayerGroup,
        applyDamages: [denominatorPlayerHit],
    });
    const denominatorPet = entity({
        id: "denominator-pet", raceId: 990103, name: "denominator-pet",
        isPC: false, group: denominatorPetGroup, ownerId: denominatorPlayer.id,
        applyDamages: [denominatorPetHit],
    });
    const denominatorActors = {
        entityMap: {
            [denominatorBoss.id]: denominatorBoss,
            [denominatorPlayer.id]: denominatorPlayer,
            [denominatorPet.id]: denominatorPet,
        },
        damages: [denominatorPlayerHit, denominatorPetHit],
        effectiveDamages: [denominatorPlayerHit, denominatorPetHit],
        healthLosses: [
            { Id: denominatorBoss.id, At: 300, Damage: 100 },
            { Id: denominatorBoss.id, At: 301, Damage: 50 },
        ],
    };
    const denominatorSummary = buildBossSummary(denominatorBoss.id, denominatorActors, []);
    assert.equal(denominatorSummary?.totalDamage, 100);
    assert.equal(denominatorSummary?.effectiveBossDamage, 150);

    const denominatorRecord = createBattleRecord(
        denominatorActors,
        { damages: [denominatorPlayerHit] },
        denominatorBoss.id,
        denominatorPlayer.id,
        { bossName: denominatorBoss.name },
    );
    assert.equal(denominatorRecord.battle.totalDamage, 150);
    assert.equal(denominatorRecord.snapshot.healthLosses.length, 2);
    assert.equal(
        denominatorRecord.snapshot.healthLosses.reduce((sum, loss) => sum + loss.Damage, 0),
        150,
    );

    const denominatorRoundTrip = parseBattleRecord(JSON.stringify(denominatorRecord));
    const denominatorHydratedDC = new DamageCollectorManager();
    const denominatorHydratedActors = new ActorManager(denominatorHydratedDC);
    hydrateFromSnapshot(denominatorRoundTrip.snapshot, denominatorHydratedActors, denominatorHydratedDC);
    const denominatorHydratedSummary = buildBossSummary(
        denominatorBoss.id,
        denominatorHydratedActors,
        [],
    );
    assert.equal(denominatorHydratedSummary?.totalDamage, 100);
    assert.equal(denominatorHydratedSummary?.effectiveBossDamage, 150);

    // Boss health accounting excludes overkill, but player damage/DPS keeps
    // the full confirmed packet value. This also recovers raw values when an
    // older imported record persisted only the clipped effective allocation.
    const overkillBossGroup = group("overkill-boss", 7603, "overkill-boss");
    const overkillPlayerGroup = group("overkill-player", 10001, "overkill-player");
    const overkillHit = damage("overkill-player", "overkill-boss", 320, 100, 59040);
    const clippedOverkillHit = { ...overkillHit, Damage: 30 };
    const overkillBoss = entity({
        id: "overkill-boss", raceId: 7603, name: "overkill-boss",
        isPC: false, group: overkillBossGroup, takeDamages: [overkillHit],
    });
    const overkillPlayer = entity({
        id: "overkill-player", raceId: 10001, name: "overkill-player",
        isPC: true, group: overkillPlayerGroup, applyDamages: [overkillHit],
    });
    const overkillActors = {
        entityMap: {
            [overkillBoss.id]: overkillBoss,
            [overkillPlayer.id]: overkillPlayer,
        },
        damages: [overkillHit],
        effectiveDamages: [clippedOverkillHit],
        healthLosses: [{ Id: overkillBoss.id, At: 320, Damage: 30 }],
    };
    const overkillSummary = buildBossSummary(overkillBoss.id, overkillActors, []);
    assert.equal(overkillSummary?.totalDamage, 100);
    assert.equal(overkillSummary?.players[0].totalDamage, 100);
    assert.equal(overkillSummary?.players[0].skillStats[0].maxNonCritDamage, 100);
    assert.equal(overkillSummary?.effectiveBossDamage, 30);

    // Mixed old/new state: another target may already have reconciled Stat28
    // events while the selected legacy target has raw packets only. Effective
    // data from the other target must not suppress the selected target's raw
    // session, player summary, or battle-record export.
    const mixedSelectedGroup = group("mixed-selected", 7600, "mixed-selected");
    const mixedOtherGroup = group("mixed-other", 7603, "mixed-other");
    const mixedPlayerGroup = group("mixed-player", 10001, "mixed-player");
    const mixedSelectedHit = damage("mixed-player", "mixed-selected", 350, 125, 59040);
    const mixedOtherHit = damage("mixed-player", "mixed-other", 351, 999, 59041);
    const mixedSelectedBoss = entity({
        id: "mixed-selected", raceId: 7600, name: "mixed-selected",
        isPC: false, group: mixedSelectedGroup, takeDamages: [mixedSelectedHit],
    });
    const mixedOtherBoss = entity({
        id: "mixed-other", raceId: 7603, name: "mixed-other",
        isPC: false, group: mixedOtherGroup, takeDamages: [mixedOtherHit],
    });
    const mixedPlayer = entity({
        id: "mixed-player", raceId: 10001, name: "mixed-player",
        isPC: true, group: mixedPlayerGroup,
        applyDamages: [mixedSelectedHit, mixedOtherHit],
    });
    const mixedActors = {
        entityMap: {
            [mixedSelectedBoss.id]: mixedSelectedBoss,
            [mixedOtherBoss.id]: mixedOtherBoss,
            [mixedPlayer.id]: mixedPlayer,
        },
        damages: [mixedSelectedHit, mixedOtherHit],
        effectiveDamages: [mixedOtherHit],
        healthLosses: [{ Id: mixedOtherBoss.id, At: 351, Damage: 999 }],
    };
    const mixedSummary = buildBossSummary(mixedSelectedBoss.id, mixedActors, []);
    assert.equal(mixedSummary?.totalDamage, 125);
    assert.equal(mixedSummary?.effectiveBossDamage, 125);
    assert.equal(mixedSummary?.players.length, 1);
    assert.equal(mixedSummary?.players[0].entityId, mixedPlayer.id);
    assert.equal(mixedSummary?.players[0].totalDamage, 125);

    const mixedRecord = createBattleRecord(
        mixedActors,
        { damages: [mixedSelectedHit, mixedOtherHit] },
        mixedSelectedBoss.id,
        mixedPlayer.id,
        { bossName: mixedSelectedBoss.name },
    );
    assert.equal(mixedRecord.battle.totalDamage, 125);
    assert.equal(mixedRecord.battle.playerCount, 1);
    assert.equal(mixedRecord.snapshot.effectiveDamages.length, 0);
    assert.equal(mixedRecord.snapshot.collectorDamages.length, 1);
    assert.equal(mixedRecord.snapshot.collectorDamages[0].TargetId, mixedSelectedBoss.id);

    const mixedRoundTrip = parseBattleRecord(JSON.stringify(mixedRecord));
    const mixedHydratedDC = new DamageCollectorManager();
    const mixedHydratedActors = new ActorManager(mixedHydratedDC);
    hydrateFromSnapshot(mixedRoundTrip.snapshot, mixedHydratedActors, mixedHydratedDC);
    const mixedHydratedSummary = buildBossSummary(
        mixedSelectedBoss.id,
        mixedHydratedActors,
        [],
    );
    assert.equal(mixedHydratedSummary?.totalDamage, 125);
    assert.equal(mixedHydratedSummary?.effectiveBossDamage, 125);
    assert.equal(mixedHydratedSummary?.players[0].totalDamage, 125);

    // The selected Boss is one health bar. Even fragments whose explicit owner
    // chain ends at that Boss have independent health pools and must not inflate
    // the report, battle record, or its round-trip summary.
    const partBossGroup = group("7603", 7603, "7603");
    const nestedPartGroup = group("7608", 7608, "7608");
    const otherBossGroup = group("7604", 7604, "7604");
    const partPlayerGroup = group("part-player", 10001, "part-player");
    const partDirectHit = damage("part-player", "part-boss", 200, 100, 59040);
    const partHit = damage("part-player", "part-direct", 201, 200, 59041);
    const nestedPartHit = damage("part-player", "part-nested", 205, 300, 59044);
    const sameRaceStrangerHit = damage("part-player", "part-stranger", 202, 400, 59042);
    const otherBossPartHit = damage("part-player", "other-boss-part", 203, 500, 59043);
    const partBoss = entity({
        id: "part-boss",
        raceId: 7603,
        name: "part-boss",
        isPC: false,
        group: partBossGroup,
        takeDamages: [partDirectHit],
    });
    const directPart = entity({
        id: "part-direct",
        raceId: 7608,
        name: "direct-part",
        isPC: false,
        group: nestedPartGroup,
        ownerId: partBoss.id,
        takeDamages: [partHit],
    });
    const nestedPart = entity({
        id: "part-nested",
        raceId: 7608,
        name: "nested-part",
        isPC: false,
        group: nestedPartGroup,
        ownerId: directPart.id,
        takeDamages: [nestedPartHit],
    });
    const sameRaceStranger = entity({
        id: "part-stranger",
        raceId: 7608,
        name: "same-race-stranger",
        isPC: false,
        group: nestedPartGroup,
        takeDamages: [sameRaceStrangerHit],
    });
    const otherBoss = entity({
        id: "other-part-boss",
        raceId: 7604,
        name: "other-boss",
        isPC: false,
        group: otherBossGroup,
    });
    const otherBossPart = entity({
        id: "other-boss-part",
        raceId: 7608,
        name: "other-boss-part",
        isPC: false,
        group: nestedPartGroup,
        ownerId: otherBoss.id,
        takeDamages: [otherBossPartHit],
    });
    const partPlayer = entity({
        id: "part-player",
        raceId: 10001,
        name: "part-player",
        isPC: true,
        group: partPlayerGroup,
        applyDamages: [partDirectHit, partHit, nestedPartHit, sameRaceStrangerHit, otherBossPartHit],
    });
    const partActorManager = {
        entityMap: {
            [partBoss.id]: partBoss,
            [directPart.id]: directPart,
            [nestedPart.id]: nestedPart,
            [sameRaceStranger.id]: sameRaceStranger,
            [otherBoss.id]: otherBoss,
            [otherBossPart.id]: otherBossPart,
            [partPlayer.id]: partPlayer,
        },
        damages: [partDirectHit, partHit, nestedPartHit, sameRaceStrangerHit, otherBossPartHit],
        effectiveDamages: [partDirectHit],
    };
    const partDCManager = {
        damages: [partDirectHit, partHit, nestedPartHit, sameRaceStrangerHit, otherBossPartHit],
    };
    const partSummary = buildBossSummary(partBoss.id, partActorManager, []);
    assert.equal(partSummary?.totalDamage, 100);
    assert.equal(partSummary?.session.endAt, 200);
    assert.equal(partSummary?.players[0].skillStats.length, 1);
    assert.equal(partSummary?.players[0].skillStats[0].skillId, 59040);

    const partRecord = createBattleRecord(
        partActorManager,
        partDCManager,
        partBoss.id,
        partPlayer.id,
        { bossName: "part-boss" },
    );
    assert.equal(partRecord.battle.totalDamage, 100);
    assert.deepEqual(
        Object.keys(partRecord.snapshot.entities).sort(),
        [partBoss.id, partPlayer.id].sort(),
    );
    assert.equal(partRecord.snapshot.collectorDamages.length, 1);
    assert.equal(partRecord.snapshot.effectiveDamages.length, 1);
    const partRoundTrip = parseBattleRecord(JSON.stringify(partRecord));
    const hydratedPartDC = new DamageCollectorManager();
    const hydratedPartActors = new ActorManager(hydratedPartDC);
    hydrateFromSnapshot(partRoundTrip.snapshot, hydratedPartActors, hydratedPartDC);
    const hydratedPartSummary = buildBossSummary(partBoss.id, hydratedPartActors, []);
    assert.equal(hydratedPartSummary?.totalDamage, 100);
    assert.equal(hydratedPartSummary?.players[0].skillStats.length, 1);
    assert.equal(hydratedPartSummary?.players[0].skillStats[0].skillId, 59040);

    // Damage keeps millisecond precision, while StatUpdate is second-granular.
    // The health loss at 40.000 still belongs to the hit/session at 39.999.
    const boundaryDC = new DamageCollectorManager();
    const boundaryActors = new ActorManager(boundaryDC);
    boundaryActors.onEvent(appear("boundary-player", 10001, "boundary-player"));
    boundaryActors.onEvent(appear("boundary-boss", 7603, "boundary-boss"));
    boundaryActors.onEvent(health("boundary-boss", 39, 1_000));
    const boundaryHit = rawHit("boundary-player", "boundary-boss", 39.999, 100, 59040);
    boundaryHit.AtMs = 39_999;
    boundaryActors.onEvent(boundaryHit);
    boundaryActors.onEvent(health("boundary-boss", 40, 900));
    assert.deepEqual(boundaryActors.healthLosses, [
        { Id: "boundary-boss", At: 39.999, Damage: 100 },
    ]);
    const boundarySummary = buildBossSummary("boundary-boss", boundaryActors, []);
    assert.equal(boundarySummary?.session.endAt, 39.999);
    assert.equal(boundarySummary?.totalDamage, 100);
    assert.equal(boundarySummary?.effectiveBossDamage, 100);

    // A follow-up packet can move Stat28 below zero after the first finishing
    // hit. Both raw packets belong to DPS, while Boss health loss stops at zero.
    const negativeHealthDC = new DamageCollectorManager();
    const negativeHealthActors = new ActorManager(negativeHealthDC);
    negativeHealthActors.onEvent(appear("negative-player", 10001, "negative-player"));
    negativeHealthActors.onEvent(appear("negative-boss", 7603, "negative-boss"));
    negativeHealthActors.onEvent(health("negative-boss", 50, 30));
    negativeHealthActors.onEvent(rawHit("negative-player", "negative-boss", 51, 100, 59040));
    negativeHealthActors.onEvent(health("negative-boss", 51, 0));
    negativeHealthActors.onEvent(rawHit("negative-player", "negative-boss", 52, 20, 59041));
    negativeHealthActors.onEvent(health("negative-boss", 52, -20));
    const negativeHealthSummary = buildBossSummary("negative-boss", negativeHealthActors, []);
    assert.equal(negativeHealthSummary?.totalDamage, 120);
    assert.equal(negativeHealthSummary?.effectiveBossDamage, 30);
    assert.equal(negativeHealthSummary?.session.endAt, 52);

    // Live path: puppet damage is stored on and credited to its player owner.
    const liveDC = new DamageCollectorManager();
    const liveActors = new ActorManager(liveDC);
    liveActors.onEvent(appear("owner-live", 10001, "人偶师"));
    liveActors.onEvent(appear("boss-live", 7600, "测试首领"));
    liveActors.onEvent(health("boss-live", 9, 1_000));
    liveActors.onEvent(appear("puppet-live", 990102, "小丑人偶", "owner-live"));
    liveActors.onEvent(rawHit("puppet-live", "boss-live", 10, 400, 54151));
    liveActors.onEvent(health("boss-live", 10, 600));
    assert.equal(
        liveActors.healthLosses.reduce((sum, loss) => sum + loss.Damage, 0),
        400,
    );
    assert.equal(liveActors.entityMap["owner-live"].applyDamages.length, 1);
    assert.equal(liveActors.entityMap["owner-live"].applyDamages[0].Id, "owner-live");
    assert.equal(liveActors.entityMap["owner-live"].applyDamages[0].PetId, "puppet-live");
    assert.equal(liveActors.entityMap["puppet-live"].applyDamages.length, 0);
    assert.equal(liveDC.damages.length, 1);
    const liveSummary = buildBossSummary("boss-live", liveActors, []);
    assert.equal(liveSummary?.players[0].totalDamage, 400);
    assert.equal(liveSummary?.players[0].skillStats[0].skillId, 54151);

    // Normal pet attacks and skills remain visible on the boss's incoming
    // damage log, but are not credited to the owner as player DPS.
    liveActors.onEvent(appear("pet-live", 990103, "test-pet", "owner-live"));
    liveActors.onEvent(rawHit("pet-live", "boss-live", 11, 250, 10001));
    assert.equal(liveActors.entityMap["boss-live"].takeDamages.length, 2);
    assert.equal(liveActors.entityMap["owner-live"].applyDamages.length, 1);
    assert.equal(liveDC.damages.length, 1);
    assert.equal(buildBossSummary("boss-live", liveActors, [])?.players[0].totalDamage, 400);

    // An OwnerId alone is not proof that passive damage belongs to the player.
    // Keep these unverified proxy hits excluded just like ordinary pet damage.
    const passiveDC = new DamageCollectorManager();
    const passiveActors = new ActorManager(passiveDC);
    passiveActors.onEvent(appear("owner-passive", 10001, "被动测试玩家"));
    passiveActors.onEvent(appear("boss-passive", 7600, "被动测试首领"));
    passiveActors.onEvent(health("boss-passive", 19, 2_000));
    passiveActors.onEvent(appear("proxy-passive", 990104, "玩家被动代理", "owner-passive"));
    passiveActors.onEvent(rawHit("proxy-passive", "boss-passive", 20, 100, 58009));
    passiveActors.onEvent(rawHit("proxy-passive", "boss-passive", 21, 200, 58100));
    passiveActors.onEvent(rawHit("proxy-passive", "boss-passive", 22, 300, 58101));
    passiveActors.onEvent(rawHit("proxy-passive", "boss-passive", 23, 999, 10001));
    passiveActors.onEvent(health("boss-passive", 23, 401));
    const passiveSummary = buildBossSummary("boss-passive", passiveActors, []);
    assert.equal(passiveSummary?.players.length, 0);
    assert.equal(passiveDC.damages.length, 0);

    // Direct player packets for all three passive skills remain valid.
    const directPassiveDC = new DamageCollectorManager();
    const directPassiveActors = new ActorManager(directPassiveDC);
    directPassiveActors.onEvent(appear("direct-passive-player", 10001, "直接被动玩家"));
    directPassiveActors.onEvent(appear("direct-passive-boss", 7600, "直接被动首领"));
    directPassiveActors.onEvent(health("direct-passive-boss", 23, 100));
    for (const [index, skillId] of [58009, 58100, 58101].entries()) {
        directPassiveActors.onEvent(rawHit(
            "direct-passive-player",
            "direct-passive-boss",
            24 + index,
            10 * (index + 1),
            skillId,
        ));
    }
    directPassiveActors.onEvent(health("direct-passive-boss", 27, 40));
    assert.deepEqual(
        buildBossSummary("direct-passive-boss", directPassiveActors, [])
            ?.players[0].skillStats
            .map((skill) => [skill.skillId, skill.totalDamage])
            .sort((a, b) => a[0] - b[0]),
        [[58009, 10], [58100, 20], [58101, 30]],
    );

    // A hit received before the target's appear packet is replayed exactly once.
    const unknownTargetDC = new DamageCollectorManager();
    const unknownTargetActors = new ActorManager(unknownTargetDC);
    unknownTargetActors.onEvent(appear("owner-unknown-target", 10001, "测试玩家"));
    unknownTargetActors.onEvent(rawHit("owner-unknown-target", "boss-unknown-target", 15, 12345, 59040));
    assert.equal(unknownTargetActors.entityMap["boss-unknown-target"].takeDamages.length, 1);
    assert.equal(unknownTargetActors.entityMap["boss-unknown-target"].totalTakeDamage, 12345);
    assert.equal(unknownTargetActors.entityMap["owner-unknown-target"].applyDamages.length, 1);
    assert.equal(unknownTargetActors.entityMap["owner-unknown-target"].totalApplyDamage, 12345);
    assert.equal(unknownTargetActors.damages.length, 1);
    unknownTargetActors.onEvent(
        appear("boss-unknown-target", 7600, "测试首领"),
    );
    assert.equal(unknownTargetActors.entityMap["boss-unknown-target"].takeDamages.length, 1);
    assert.equal(unknownTargetActors.entityMap["boss-unknown-target"].totalTakeDamage, 12345);
    assert.equal(unknownTargetActors.entityMap["owner-unknown-target"].applyDamages.length, 1);

    // Late owner path: the raw hit waits for the owner entity and is replayed once.
    const lateDC = new DamageCollectorManager();
    const lateActors = new ActorManager(lateDC);
    lateActors.onEvent(appear("boss-late", 7600, "测试首领"));
    lateActors.onEvent(appear("puppet-late", 990102, "小丑人偶", "owner-late"));
    lateActors.onEvent(rawHit("puppet-late", "boss-late", 20, 500, 59167));
    assert.equal(lateDC.damages.length, 0);
    lateActors.onEvent(appear("owner-late", 10001, "后到的人偶师"));
    assert.equal(lateActors.entityMap["owner-late"].applyDamages.length, 1);
    assert.equal(lateDC.damages.length, 1);

    // Worker import path follows the same owner attribution rules.
    const pureDC = new PureDamageCollectorManager();
    const pureActors = new PureActorManager(pureDC);
    pureActors.onEvent(appear("owner-worker", 10001, "导入人偶师"));
    pureActors.onEvent(appear("boss-worker", 7600, "测试首领"));
    pureActors.onEvent(health("boss-worker", 29, 1_000));
    pureActors.onEvent(appear("puppet-worker", 990202, "巨像人偶", "owner-worker"));
    pureActors.onEvent(rawHit("puppet-worker", "boss-worker", 30, 600, 59168));
    pureActors.onEvent(health("boss-worker", 30, 400));
    assert.equal(
        pureActors.healthLosses.reduce((sum, loss) => sum + loss.Damage, 0),
        600,
    );
    assert.equal(pureActors.entityMap["owner-worker"].applyDamages.length, 1);
    assert.equal(pureActors.entityMap["puppet-worker"].applyDamages.length, 0);
    assert.equal(pureDC.damages.length, 1);

    const purePassiveDC = new PureDamageCollectorManager();
    const purePassiveActors = new PureActorManager(purePassiveDC);
    purePassiveActors.onEvent(appear("owner-worker-passive", 10001, "导入被动玩家"));
    purePassiveActors.onEvent(appear("boss-worker-passive", 7600, "导入被动首领"));
    purePassiveActors.onEvent(appear("proxy-worker-passive", 990204, "导入被动代理", "owner-worker-passive"));
    purePassiveActors.onEvent(rawHit("proxy-worker-passive", "boss-worker-passive", 40, 100, 58100));
    assert.equal(purePassiveActors.entityMap["owner-worker-passive"].applyDamages.length, 0);
    assert.equal(purePassiveDC.damages.length, 0);
    pureActors.onEvent(appear("pet-worker", 990203, "worker-pet", "owner-worker"));
    pureActors.onEvent(rawHit("pet-worker", "boss-worker", 31, 300, 20001));
    assert.equal(pureActors.entityMap["boss-worker"].takeDamages.length, 2);
    assert.equal(pureActors.entityMap["owner-worker"].applyDamages.length, 1);
    assert.equal(pureDC.damages.length, 1);

    const pureUnknownDC = new PureDamageCollectorManager();
    const pureUnknownActors = new PureActorManager(pureUnknownDC);
    pureUnknownActors.onEvent(appear("owner-pure-unknown", 10001, "导入玩家"));
    pureUnknownActors.onEvent(rawHit("owner-pure-unknown", "boss-pure-unknown", 35, 12345, 59040));
    assert.equal(pureUnknownActors.entityMap["boss-pure-unknown"].takeDamages.length, 1);
    assert.equal(pureUnknownActors.entityMap["boss-pure-unknown"].totalTakeDamage, 12345);
    assert.equal(pureUnknownActors.entityMap["owner-pure-unknown"].applyDamages.length, 1);
    assert.equal(pureUnknownActors.entityMap["owner-pure-unknown"].totalApplyDamage, 12345);
    assert.equal(pureUnknownActors.damages.length, 1);
    pureUnknownActors.onEvent(
        appear("boss-pure-unknown", 7600, "测试首领"),
    );
    assert.equal(pureUnknownActors.entityMap["boss-pure-unknown"].takeDamages.length, 1);
    assert.equal(pureUnknownActors.entityMap["boss-pure-unknown"].totalTakeDamage, 12345);
    assert.equal(pureUnknownActors.entityMap["owner-pure-unknown"].applyDamages.length, 1);

    // Older battle records can recover owner-remapped hits from collectorDamages.
    const legacySnapshot = structuredClone(roundTrip.snapshot);
    delete legacySnapshot.effectiveDamages;
    legacySnapshot.collectorDamages.push({
        ...damage("player-1", "boss-1", 100.5, 400, 54151),
        PetId: "legacy-puppet",
    });
    const legacyDC = new DamageCollectorManager();
    const legacyActors = new ActorManager(legacyDC);
    hydrateFromSnapshot(legacySnapshot, legacyActors, legacyDC);
    const legacySummary = buildBossSummary("boss-1", legacyActors, []);
    assert.equal(
        legacySummary?.players.find((player) => player.entityId === "player-1")?.totalDamage,
        500,
    );
    legacySnapshot.collectorDamages.push({
        ...damage("player-1", "boss-1", 100.6, 700, 10001),
        PetId: "legacy-pet",
    });
    const legacyPetFilteredDC = new DamageCollectorManager();
    const legacyPetFilteredActors = new ActorManager(legacyPetFilteredDC);
    hydrateFromSnapshot(
        legacySnapshot,
        legacyPetFilteredActors,
        legacyPetFilteredDC,
    );
    assert.equal(
        buildBossSummary("boss-1", legacyPetFilteredActors, [])?.players.find(
            (player) => player.entityId === "player-1",
        )?.totalDamage,
        500,
    );
    assert.equal(legacyPetFilteredDC.damages.length, 3);

    // Imported records may contain historical owner-remapped proxy hits. Do not
    // revive unverified passive-source attribution during hydration.
    const legacyPassiveSnapshot = structuredClone(roundTrip.snapshot);
    delete legacyPassiveSnapshot.effectiveDamages;
    for (const [index, skillId] of [58009, 58100, 58101].entries()) {
        legacyPassiveSnapshot.collectorDamages.push({
            ...damage("player-1", "boss-1", 100.7 + index / 10, 100 * (index + 1), skillId),
            PetId: "legacy-passive-proxy",
        });
    }
    const legacyPassiveDC = new DamageCollectorManager();
    const legacyPassiveActors = new ActorManager(legacyPassiveDC);
    hydrateFromSnapshot(legacyPassiveSnapshot, legacyPassiveActors, legacyPassiveDC);
    const legacyPassivePlayer = buildBossSummary("boss-1", legacyPassiveActors, [])?.players.find(
        (player) => player.entityId === "player-1",
    );
    assert.equal(legacyPassivePlayer?.totalDamage, 100);
    assert.equal(
        legacyPassivePlayer?.skillStats.some((skill) => [58009, 58100, 58101].includes(skill.skillId)),
        false,
    );

    for (const skillId of [54101, 54106, 54151, 54156, 59167, 59169]) {
        assert.equal(isMarionetteDamageSkill(skillId), true);
    }
    for (const skillId of [54100, 54107, 54150, 54157, 59166, 59170, 10001]) {
        assert.equal(isMarionetteDamageSkill(skillId), false);
    }
    assert.equal(normalizeSkillDisplayName(54151, "第2幕: 怒气上涌 AI"), "第2幕: 怒气上涌");
    assert.equal(normalizeSkillDisplayName(59167, "重拍坠音 AI"), "重拍坠音");
    assert.equal(normalizeSkillDisplayName(59168, "猎踪踏影 AI"), "猎踪踏影");
    assert.equal(normalizeSkillDisplayName(59169, "终幕绝响 AI"), "终幕绝响");

    // Optional generic overkill-capture replay. The most heavily damaged
    // health-bar target must retain more raw player damage than health removed.
    const overkillLogPath = process.env.DILMETER_OVERKILL_NDJSON;
    if (overkillLogPath && fs.existsSync(overkillLogPath)) {
        const replayDC = new PureDamageCollectorManager();
        const replayActors = new PureActorManager(replayDC);
        const lines = readline.createInterface({
            input: fs.createReadStream(overkillLogPath, { encoding: "utf8" }),
            crlfDelay: Infinity,
        });
        for await (const line of lines) {
            const trimmed = line.trim();
            if (trimmed) replayActors.onEvent(JSON.parse(trimmed));
        }

        const result = Object.values(replayActors.entityMap)
            .filter((candidate) => !candidate.isPC && candidate.statMap[30] > 0)
            .map((candidate) => ({
                candidate,
                summary: buildBossSummary(candidate.id, replayActors, []),
            }))
            .filter((entry) => entry.summary)
            .sort((a, b) => b.summary.totalDamage - a.summary.totalDamage)[0];
        assert.ok(result, "overkill capture must contain a reportable target");
        assert.ok(result.summary.totalDamage > result.summary.effectiveBossDamage);
        process.stdout.write(`Overkill capture replay: ${JSON.stringify({
            entityId: result.candidate.id,
            raceId: result.candidate.raceId,
            maximumHealth: result.candidate.statMap[30],
            playerDamage: result.summary.totalDamage,
            bossHealthLoss: result.summary.effectiveBossDamage,
            startAt: result.summary.session.startAt,
            endAt: result.summary.session.endAt,
        })}\n`);
    }

    // Optional real-log replay. CI without the external capture skips it. This
    // protects the concrete 7603 encounter that exposed immune/overkill and
    // fragment-health inflation.
    const realLogPath = process.env.DILMETER_REAL_NDJSON
        || "C:/Users/admin/Desktop/PowerToys.MouseWithoutBorders/ScreenCaptures/packet_log_2026-08-13_22-12-28.ndjson";
    if (fs.existsSync(realLogPath)) {
        const replayDC = new PureDamageCollectorManager();
        const replayActors = new PureActorManager(replayDC);
        const lines = readline.createInterface({
            input: fs.createReadStream(realLogPath, { encoding: "utf8" }),
            crlfDelay: Infinity,
        });
        for await (const line of lines) {
            const trimmed = line.trim();
            if (trimmed) replayActors.onEvent(JSON.parse(trimmed));
        }

        const realBoss = Object.values(replayActors.entityMap).find(
            (candidate) => candidate.raceId === 7603
                && candidate.statMap[30] === 1_967_880_064,
        );
        assert.ok(realBoss, "real 7603 Boss with authoritative Stat30 must exist");
        const realSummary = buildBossSummary(realBoss.id, replayActors, []);
        assert.equal(realSummary?.effectiveBossDamage, 1_967_880_064);
        assert.ok(realSummary.totalDamage > 0);

        const fragmentIds = new Set(
            Object.values(replayActors.entityMap)
                .filter((candidate) => candidate.ownerId === realBoss.id)
                .map((candidate) => candidate.id),
        );
        const fragmentDamage = replayActors.effectiveDamages
            .filter((event) => fragmentIds.has(event.TargetId))
            .reduce((sum, event) => sum + event.Damage, 0);
        assert.ok(fragmentDamage > 0, "capture must contain effective fragment damage");
        assert.notEqual(
            realSummary.effectiveBossDamage,
            1_967_880_064 + fragmentDamage,
            "fragment health pools must stay outside the Boss denominator",
        );
    }

    const jobCases = [
        [59004, "圣光颂唱者"],
        [59023, "元素骑士"],
        [59040, "黑魔导士"],
        [59064, "流星射手"],
        [59083, "圣盾骑士"],
        [59106, "爆裂骑士枪"],
        [59124, "枪炮师"],
        [59145, "禁术炼金师"],
        [59167, "旋律操纵师"],
        [59187, "狂怒斗士"],
    ];
    for (const [skillId, expectedJob] of jobCases) {
        assert.equal(detectJob(new Set([skillId])), expectedJob);
    }

    assert.equal(parseBattleRecord('{"EventId":1}\n'), null);
    assert.throws(
        () => parseBattleRecord(JSON.stringify({ ...record, version: 999 })),
        /不支持的战斗记录版本/,
    );

    process.stdout.write("Battle record round-trip verified.\n");
} finally {
    await server.close();
    fs.rmSync(cacheDir, { recursive: true, force: true });
}
