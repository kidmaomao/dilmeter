import assert from "node:assert/strict";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import readline from "node:readline";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const [packetPath, bossId = "4767482419275653", playerId = "4503599631566088"] = process.argv.slice(2);
if (!packetPath) {
    throw new Error("Usage: node scripts/verify-packet-dps.mjs <packet.ndjson> [bossId] [playerId]");
}

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const cacheDir = fs.mkdtempSync(path.join(os.tmpdir(), "dilmetercn-packet-verify-"));
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
    const { buildBossSummary } = await server.ssrLoadModule("/src/summaryCollector.ts");
    const {
        PureActorManager,
        PureDamageCollectorManager,
    } = await server.ssrLoadModule("/src/worker/pureEventActor.ts");

    const damageManager = new PureDamageCollectorManager();
    const actorManager = new PureActorManager(damageManager);
    const healthUpdates = [];
    let packetSequence = 0;
    const lines = readline.createInterface({
        input: fs.createReadStream(packetPath, { encoding: "utf8" }),
        crlfDelay: Infinity,
    });

    for await (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed) continue;
        const event = JSON.parse(trimmed);
        event.__PacketSequence = packetSequence++;
        if (event.EventId === 17 && event.Id === bossId) {
            const health = event.Stats?.find((stat) => stat.StatId === 28)?.Value;
            if (Number.isFinite(health)) {
                healthUpdates.push({
                    sequence: event.__PacketSequence,
                    at: event.At,
                    health,
                });
            }
        }
        actorManager.onEvent(event);
    }

    const summary = buildBossSummary(bossId, actorManager, []);
    assert.ok(summary, `Boss ${bossId} must produce a DPS summary`);
    const player = summary.players.find((candidate) => candidate.entityId === playerId);
    assert.ok(player, `Player ${playerId} must be present in the Boss summary`);

    const rawHits = actorManager.entityMap[bossId].takeDamages.filter((damage) => (
        damage.Id === playerId
        && damage.Damage > 0
        && damage.At >= summary.session.startAt
        && damage.At <= summary.session.endAt
    ));
    const confirmedHits = actorManager.effectiveDamages.filter((damage) => (
        damage.Id === playerId
        && damage.TargetId === bossId
        && damage.Damage > 0
        && damage.At >= summary.session.startAt
        && damage.At <= summary.session.endAt
    ));
    const damageIdentity = (damage) => [
        damage.Id,
        damage.AtMs ?? damage.At,
        damage.TargetId,
        damage.SkillId,
        damage.IsCritical ? 1 : 0,
        damage.IsDelayed ? 1 : 0,
    ].join("|");
    const remainingConfirmedHits = new Map();
    for (const damage of confirmedHits) {
        const key = damageIdentity(damage);
        remainingConfirmedHits.set(key, (remainingConfirmedHits.get(key) ?? 0) + 1);
    }
    const previouslyCountedHits = rawHits.filter((damage) => {
        const key = damageIdentity(damage);
        const remaining = remainingConfirmedHits.get(key) ?? 0;
        if (remaining <= 0) return false;
        if (remaining === 1) remainingConfirmedHits.delete(key);
        else remainingConfirmedHits.set(key, remaining - 1);
        return true;
    });
    const actualHealthRemoved = confirmedHits.reduce((total, damage) => total + damage.Damage, 0);
    const previouslyCountedDamage = previouslyCountedHits.reduce(
        (total, damage) => total + damage.Damage,
        0,
    );
    const previouslyCountedSet = new Set(previouslyCountedHits);
    const excludedHits = rawHits.filter((damage) => !previouslyCountedSet.has(damage));
    const maximumHealth = actorManager.entityMap[bossId].statMap[30];
    const excludedHitDetails = excludedHits.map((damage) => {
        const sequence = damage.__PacketSequence;
        let previousHealth;
        let nextHealth;
        for (const update of healthUpdates) {
            if (update.sequence < sequence) previousHealth = update;
            else if (update.sequence > sequence) {
                nextHealth = update;
                break;
            }
        }
        return {
            at: damage.At,
            atMs: damage.AtMs,
            fightSecond: damage.At - summary.session.startAt,
            skillId: damage.SkillId,
            damage: damage.Damage,
            isCritical: damage.IsCritical,
            bossHealthBefore: previousHealth?.health,
            bossHealthBeforePercent: Number.isFinite(maximumHealth) && previousHealth
                ? previousHealth.health / maximumHealth * 100
                : undefined,
            nextBossHealth: nextHealth?.health,
            nextHealthChange: previousHealth && nextHealth
                ? nextHealth.health - previousHealth.health
                : undefined,
            secondsToNextHealthUpdate: nextHealth ? nextHealth.at - damage.At : undefined,
        };
    });

    const expectedDamage = 1_098_218_786.4207003;
    const expectedDuration = 545;
    const expectedDps = expectedDamage / expectedDuration;
    assert.ok(Math.abs(player.totalDamage - expectedDamage) < 0.01);
    assert.equal(summary.session.totalDuration, expectedDuration);
    assert.ok(Math.abs(player.totalDPS - expectedDps) < 0.0001);
    assert.ok(Math.abs(previouslyCountedDamage - 1_082_826_553.7907002) < 0.01);

    process.stdout.write(`${JSON.stringify({
        packetPath,
        boss: {
            entityId: bossId,
            name: actorManager.entityMap[bossId].name,
            raceId: actorManager.entityMap[bossId].raceId,
        },
        player: {
            entityId: playerId,
            name: player.name,
        },
        durationSeconds: summary.session.totalDuration,
        rawDamage: player.totalDamage,
        dps: player.totalDPS,
        rawHits: rawHits.length,
        previousHealthConfirmedRawDamage: previouslyCountedDamage,
        previousDps: previouslyCountedDamage / summary.session.totalDuration,
        previousHealthConfirmedHits: previouslyCountedHits.length,
        newlyCountedMechanismOrFinishingDamage: player.totalDamage - previouslyCountedDamage,
        actualHealthRemoved,
        maximumHealth,
        excludedHitDetails,
    }, null, 2)}\n`);
} finally {
    await server.close();
    fs.rmSync(cacheDir, { recursive: true, force: true });
}
