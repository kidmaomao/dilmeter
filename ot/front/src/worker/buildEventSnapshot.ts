// Web Worker 入口點 - 無 Vue / DOM 依賴
import { PureActorManager, PureDamageCollectorManager } from "./pureEventActor";
import type {
    WorkerInMessage,
    WorkerOutMessage,
    WorkerSnapshot,
    SnapshotEntity,
    SnapshotGroup,
} from "./workerProtocol";

export function buildEventSnapshot(ndjson: string, onProgress: (message: WorkerOutMessage) => void = () => {}): WorkerSnapshot {
    const dcMgr = new PureDamageCollectorManager();
    const actorMgr = new PureActorManager(dcMgr);

    // Parse and apply each line in one pass. Keeping a second full `events`
    // array doubled peak memory for multi-hour sessions and made the snapshot
    // handoff noticeably more expensive after restoring a minimized window.
    let lastPos = 0;
    const total = ndjson.length;
    let lastReportedPct = -1;

    while (lastPos < total) {
        const nextPos = ndjson.indexOf("\n", lastPos);
        const endPos = nextPos < 0 ? total : nextPos;
        const line = ndjson.substring(lastPos, endPos).trim();
        lastPos = nextPos < 0 ? total : nextPos + 1;

        if (!line) continue;

        let event;
        try { event = JSON.parse(line); } catch { continue; }
        actorMgr.onEvent(event);

        // Reserve the last few percent for snapshot materialization and the
        // structured-clone handoff back to the main renderer.
        const pct = Math.floor((lastPos / Math.max(1, total)) * 96);
        if (pct > lastReportedPct) {
            lastReportedPct = pct;
            const msg: WorkerOutMessage = {
                type: "progress",
                pct,
                phase: pct < 48 ? "parse" : "process",
            };
            onProgress(msg);
        }
    }

    // ── Build snapshot ────────────────────────────────────────────────────
    const entities: Record<string, SnapshotEntity> = {};
    for (const [id, e] of Object.entries(actorMgr.entityMap)) {
        entities[id] = {
            id: e.id,
            raceId: e.raceId,
            name: e.name,
            guildName: e.guildName,
            ownerId: e.ownerId,
            finisherId: e.finisherId,
            body: { ...e.body },
            totalTakeDamage: e.totalTakeDamage,
            takeDamages: e.takeDamages,
            totalApplyDamage: e.totalApplyDamage,
            applyDamages: e.applyDamages,
            conditionMap: { ...e.conditionMap },
            conditionRefreshGuardAt: e.snapshotConditionRefreshGuard(),
            conditionHistory: e.conditionHistory,
            equipItemMap: { ...e.equipItemMap },
            statMap: { ...e.statMap },
            appearedAt: e.appearedAt,
            groupKey: PureActorManager.groupTargetKey({
                Id: id,
                RaceId: e.raceId,
            } as any),
        };
    }

    const groups: Record<string, SnapshotGroup> = {};
    for (const [key, g] of Object.entries(actorMgr.groupMap)) {
        groups[key] = {
            id: g.id,
            raceId: g.raceId,
            name: g.name,
            body: { ...g.body },
            totalTakeDamage: g.totalTakeDamage,
            takeDamages: g.takeDamages,
        };
    }

    return {
        localEntityId: actorMgr.localEntityId,
        localEntityReliable: actorMgr.localEntityReliable,
        selectedTargetId: actorMgr.selectedTargetId,
        activeEntityMap: actorMgr.activeEntityMap,
        resumeState: actorMgr.snapshotResumeState(),
        entities,
        groups,
        damages: actorMgr.damages,
        skillActions: actorMgr.skillActions,
        effectiveDamages: actorMgr.effectiveDamages,
        healthLosses: actorMgr.healthLosses,
        collectorDamages: dcMgr.damages,
    };
}
