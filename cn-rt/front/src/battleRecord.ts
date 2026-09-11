import type { DamageCollectorManager } from "@/actionCollector";
import type {
    ActorManager,
    EntityActor,
    EntityCondition,
    EntityConditionState,
    EntityDamage,
    GroupActor,
} from "@/eventActor";
import {
    buildBossFightSession,
    buildBossSummary,
} from "@/summaryCollector";
import type {
    SnapshotCondition,
    SnapshotConditionState,
    SnapshotDamage,
    SnapshotEntity,
    SnapshotGroup,
    WorkerSnapshot,
} from "@/worker/workerProtocol";

export const BATTLE_RECORD_FORMAT = "dilmetercn-battle-record";
export const BATTLE_RECORD_VERSION = 1;
export const BATTLE_RECORD_LOADED_EVENT = "battle-record-loaded";

export type BattleRecordSelection = {
    bossEntityId: string;
    playerEntityId: string;
};

export type DilmeterBattleRecord = {
    format: typeof BATTLE_RECORD_FORMAT;
    version: typeof BATTLE_RECORD_VERSION;
    app: "DilmeterCN";
    savedAt: string;
    battle: {
        bossEntityId: string;
        bossRaceId: number;
        bossName: string;
        startAt: number;
        endAt: number;
        totalDamage: number;
        playerCount: number;
    };
    selection: BattleRecordSelection;
    snapshot: WorkerSnapshot;
};

type BattleRecordLabels = {
    bossName: string;
};

export function createBattleRecord(
    actorManager: ActorManager,
    dcManager: DamageCollectorManager,
    bossEntityId: string,
    playerEntityId: string,
    labels: BattleRecordLabels,
): DilmeterBattleRecord {
    const boss = actorManager.entityMap[bossEntityId] as EntityActor | undefined;
    if (!boss || boss.isPC) throw new Error("没有可保存的首领场次");

    const bossTargetEntityIds = new Set([bossEntityId]);
    const session = buildBossFightSession(boss, actorManager);
    if (!session) throw new Error("当前目标没有有效伤害数据");

    const inSession = (damage: { At: number }) =>
        damage.At >= session.startAt && damage.At <= session.endAt;
    const hitsBoss = (damage: { At: number; TargetId: string }) =>
        inSession(damage) && bossTargetEntityIds.has(damage.TargetId);

    const entities: Record<string, SnapshotEntity> = {};
    const groups: Record<string, SnapshotGroup> = {};

    const targetTakeDamages: SnapshotDamage[] = [];
    for (const targetId of bossTargetEntityIds) {
        const target = actorManager.entityMap[targetId] as EntityActor | undefined;
        if (!target) continue;
        const takeDamages = target.takeDamages.filter(hitsBoss).map(cloneDamage);
        const applyDamages = target.applyDamages.filter(inSession).map(cloneDamage);
        targetTakeDamages.push(...takeDamages);
        entities[target.id] = snapshotEntity(
            target,
            takeDamages,
            applyDamages,
            target.group.id,
            session.startAt,
            session.endAt,
        );
        const groupTakeDamages = target.group.takeDamages
            .filter(hitsBoss)
            .map(cloneDamage);
        groups[target.group.id] = snapshotGroup(target.group, groupTakeDamages);
    }

    const playerIds: string[] = [];
    const reconciledDamages = actorManager.effectiveDamages ?? [];
    const selectedEffectiveDamages = reconciledDamages.filter(hitsBoss);
    for (const entity of Object.values(actorManager.entityMap) as EntityActor[]) {
        if (!entity.isPC) continue;
        const applyDamages = entity.applyDamages.filter(hitsBoss).map(cloneDamage);
        const hasEffectiveDamage = selectedEffectiveDamages.length > 0
            ? selectedEffectiveDamages.some((damage) =>
                damage.Id === entity.id && hitsBoss(damage) && damage.Damage > 0,
            )
            : applyDamages.length > 0;
        if (!hasEffectiveDamage) continue;

        playerIds.push(entity.id);
        const takeDamages = entity.takeDamages.filter(inSession).map(cloneDamage);
        entities[entity.id] = snapshotEntity(
            entity,
            takeDamages,
            applyDamages,
            entity.group.id,
            session.startAt,
            session.endAt,
        );
        groups[entity.group.id] = snapshotGroup(entity.group, takeDamages);
    }

    if (playerIds.length === 0) throw new Error("本场没有可保存的玩家伤害数据");
    const selectedPlayerId = playerIds.includes(playerEntityId) ? playerEntityId : playerIds[0];

    const snapshot: WorkerSnapshot = {
        entities,
        groups,
        damages: actorManager.damages
            .filter(hitsBoss)
            .map((damage) => ({ ...damage })),
        skillActions: (actorManager.skillActions ?? [])
            .filter((action) => action.At >= session.startAt && action.At <= session.endAt)
            .map((action) => ({ ...action })),
        effectiveDamages: selectedEffectiveDamages
            .map((damage) => ({ ...damage })),
        healthLosses: (actorManager.healthLosses ?? [])
            .filter((loss) => loss.Id === bossEntityId && inSession(loss))
            .map((loss) => ({ ...loss })),
        collectorDamages: dcManager.damages
            .filter(hitsBoss)
            .map(cloneDamage),
    };

    return {
        format: BATTLE_RECORD_FORMAT,
        version: BATTLE_RECORD_VERSION,
        app: "DilmeterCN",
        savedAt: new Date().toISOString(),
        battle: {
            bossEntityId,
            bossRaceId: boss.raceId,
            bossName: labels.bossName,
            startAt: session.startAt,
            endAt: session.endAt,
            totalDamage: buildBossSummary(bossEntityId, actorManager, [])?.effectiveBossDamage
                ?? sumDamage(snapshot.effectiveDamages as SnapshotDamage[]),
            playerCount: playerIds.length,
        },
        selection: {
            bossEntityId,
            playerEntityId: selectedPlayerId,
        },
        snapshot,
    };
}

export function parseBattleRecord(text: string): DilmeterBattleRecord | null {
    let value: unknown;
    try {
        value = JSON.parse(text);
    } catch {
        return null;
    }

    if (!isObject(value) || value.format !== BATTLE_RECORD_FORMAT) return null;
    if (value.version !== BATTLE_RECORD_VERSION) {
        throw new Error(`不支持的战斗记录版本：${String(value.version)}`);
    }
    if (
        value.app !== "DilmeterCN"
        || !isObject(value.battle)
        || !isObject(value.selection)
        || typeof value.selection.bossEntityId !== "string"
        || typeof value.selection.playerEntityId !== "string"
        || !isWorkerSnapshot(value.snapshot)
    ) {
        throw new Error("战斗记录文件不完整或已经损坏");
    }

    return value as DilmeterBattleRecord;
}

export function battleRecordFilename(record: DilmeterBattleRecord): string {
    const at = new Date(record.battle.startAt * 1000);
    const stamp = Number.isNaN(at.getTime())
        ? "unknown-time"
        : `${at.getFullYear()}-${pad(at.getMonth() + 1)}-${pad(at.getDate())}_${pad(at.getHours())}-${pad(at.getMinutes())}-${pad(at.getSeconds())}`;
    return safeFilename(`DilmeterCN_${record.battle.bossName}_${stamp}.json`);
}

function snapshotEntity(
    entity: EntityActor,
    takeDamages: SnapshotDamage[],
    applyDamages: SnapshotDamage[],
    groupKey: string,
    startAt: number,
    endAt: number,
): SnapshotEntity {
    const conditionHistory = clipConditionHistory(entity.conditionHistory, startAt, endAt);
    const finalConditions = conditionHistory.at(-1)?.List ?? [];
    const conditionMap = Object.fromEntries(
        finalConditions.map((condition) => [condition.CCId, cloneCondition(condition)]),
    );

    return {
        id: entity.id,
        raceId: entity.raceId,
        name: entity.name,
        guildName: entity.guildName,
        ownerId: entity.ownerId,
        finisherId: entity.finisherId,
        body: { ...entity.body },
        totalTakeDamage: sumDamage(takeDamages),
        takeDamages,
        totalApplyDamage: sumDamage(applyDamages),
        applyDamages,
        conditionMap,
        conditionHistory,
        equipItemMap: Object.fromEntries(
            Object.entries(entity.equipItemMap).map(([key, item]) => [key, { ...item }]),
        ),
        statMap: { ...entity.statMap },
        appearedAt: entity.appearedAt,
        groupKey,
    };
}

function snapshotGroup(group: GroupActor, takeDamages: SnapshotDamage[]): SnapshotGroup {
    return {
        id: group.id,
        raceId: group.raceId,
        name: group.name,
        body: { ...group.body },
        totalTakeDamage: sumDamage(takeDamages),
        takeDamages,
    };
}

function clipConditionHistory(
    history: EntityConditionState[],
    startAt: number,
    endAt: number,
): SnapshotConditionState[] {
    const result: SnapshotConditionState[] = [];
    const stateAtStart = [...history].reverse().find((state) => state.At <= startAt);
    if (stateAtStart) {
        result.push({
            At: startAt,
            List: stateAtStart.List.map(cloneCondition),
        });
    }

    for (const state of history) {
        if (state.At <= startAt || state.At > endAt) continue;
        result.push({
            At: state.At,
            List: state.List.map(cloneCondition),
        });
    }
    return result;
}

function cloneCondition(condition: EntityCondition | SnapshotCondition): SnapshotCondition {
    return { ...condition };
}

function cloneDamage(damage: EntityDamage | SnapshotDamage): SnapshotDamage {
    return {
        ...damage,
        Conditions: damage.Conditions.map(cloneCondition),
        TargetConditions: damage.TargetConditions.map(cloneCondition),
    };
}

function sumDamage(damages: SnapshotDamage[]): number {
    return damages.reduce((sum, damage) => sum + damage.Damage, 0);
}

function isWorkerSnapshot(value: unknown): value is WorkerSnapshot {
    return isObject(value)
        && isObject(value.entities)
        && isObject(value.groups)
        && Array.isArray(value.damages)
        && Array.isArray(value.collectorDamages);
}

function isObject(value: unknown): value is Record<string, any> {
    return typeof value === "object" && value !== null && !Array.isArray(value);
}

function safeFilename(value: string): string {
    return value.replace(/[\\/:*?"<>|]/g, "_");
}

function pad(value: number): string {
    return String(value).padStart(2, "0");
}
