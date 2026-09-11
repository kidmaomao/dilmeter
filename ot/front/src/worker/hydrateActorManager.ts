// 主線程：將 Worker snapshot 還原到 reactive actorManager / dcManager
import { CustomReactive } from "@/lib/util";
import { ActorManager, EntityActor, GroupActor } from "@/eventActor";
import { DamageCollectorManager } from "@/actionCollector";
import { isIncludedPlayerDamage } from "@/ownedDamagePolicy";
import type { WorkerSnapshot } from "./workerProtocol";

export function hydrateFromSnapshot(
    snapshot: WorkerSnapshot,
    actorMgr: ActorManager,
    dcMgr: DamageCollectorManager,
): void {
    const includedCollectorDamages = snapshot.collectorDamages.filter(
        isIncludedPlayerDamage,
    );

    // 1. 暫停 Vue reactivity，避免每次賦值都觸發 re-render
    actorMgr.pauseReactivity();

    try {
        // 2. 清除舊的 map（保留 shallowReactive 物件本身）
        for (const k in actorMgr.entityMap) delete actorMgr.entityMap[k];
        for (const k in actorMgr.groupMap) delete actorMgr.groupMap[k];
        actorMgr.damages.length = 0;
        actorMgr.skillActions.length = 0;
        actorMgr.effectiveDamages.length = 0;
        actorMgr.healthLosses.length = 0;

        // 3. 建立 GroupActor 實例
        for (const [key, sg] of Object.entries(snapshot.groups)) {
            const group = CustomReactive(
                new GroupActor(actorMgr, key, sg.raceId, sg.name),
            );
            group.hydrateFrom(sg);
            actorMgr.groupMap[key] = group;
        }

        // 4. 建立 EntityActor 實例
        for (const [id, se] of Object.entries(snapshot.entities)) {
            const group = actorMgr.groupMap[se.groupKey];
            if (!group) {
                console.warn(
                    `hydrateFromSnapshot: group not found for entity ${id}, groupKey=${se.groupKey}`,
                );
                continue;
            }

            const entity = CustomReactive(
                new EntityActor(actorMgr, id, se.raceId, se.name, group),
            );
            const applyDamages = se.applyDamages.filter(isIncludedPlayerDamage);
            entity.hydrateFrom({
                ...se,
                applyDamages,
                totalApplyDamage: applyDamages.reduce(
                    (sum, damage) => sum + damage.Damage,
                    0,
                ),
            });
            actorMgr.entityMap[id] = entity;
            // group.entityMap 的 getter 回傳的是 _entityMap 的參照，直接賦值即可
            group.entityMap[id] = entity;
        }

        // 5. 還原 raw damage log
        actorMgr.damages.push(...snapshot.damages);
        actorMgr.skillActions.push(...(snapshot.skillActions ?? []));
        actorMgr.effectiveDamages.push(...(snapshot.effectiveDamages ?? []));
        actorMgr.healthLosses.push(...(snapshot.healthLosses ?? []));

        // Older .dilmetercn records stored owner-remapped puppet/pet hits in
        // collectorDamages but omitted them from the player's applyDamages.
        // Recover the missing multiplicity without collapsing legitimate
        // identical multi-hits.
        const existingOwnerDamageCounts = new Map<string, number>();
        for (const entity of Object.values(actorMgr.entityMap)) {
            if (!entity.isPC) continue;
            for (const damage of entity.applyDamages) {
                const key = damageIdentity(damage);
                existingOwnerDamageCounts.set(
                    key,
                    (existingOwnerDamageCounts.get(key) || 0) + 1,
                );
            }
        }
        for (const damage of includedCollectorDamages) {
            if (!damage.PetId) continue;
            const owner = actorMgr.entityMap[damage.Id];
            if (!owner?.isPC) continue;

            const key = damageIdentity(damage);
            const existing = existingOwnerDamageCounts.get(key) || 0;
            if (existing > 0) {
                existingOwnerDamageCounts.set(key, existing - 1);
                continue;
            }
            owner.hydrateAdditionalApplyDamage(damage);
        }
    } finally {
        // 6. 恢復 reactivity，觸發一次性 re-render
        actorMgr.eventVersion += 1;
        actorMgr.resumeReactivity();
    }

    // 7. 重播 dcManager damages（通知所有已訂閱的 collector）
    //    在 resumeReactivity 之後執行，確保 entities 已就位
    dcMgr.clear();
    for (const d of includedCollectorDamages) {
        dcMgr.onDamage(d as any);
    }
}

function damageIdentity(damage: {
    Id: string;
    PetId: string;
    TargetId: string;
    SkillId: number;
    At: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
}): string {
    return [
        damage.Id,
        damage.PetId,
        damage.TargetId,
        damage.SkillId,
        damage.At,
        damage.Damage,
        damage.IsCritical ? 1 : 0,
        damage.IsDelayed ? 1 : 0,
    ].join("|");
}
