// Vue-free 版本的 eventActor.ts + 最小化的 DamageCollectorManager
// 用於 Web Worker 內部，無任何 Vue / DOM 依賴

import bounds from "binary-search-bounds";
import * as protocols from "../protocols";
import { isMarionetteDamageSkill } from "../ownedDamagePolicy";
import {
    allocateEffectiveDamage,
    CURRENT_HEALTH_STAT_ID,
    MAXIMUM_HEALTH_STAT_ID,
    type EntityHealthLoss,
} from "../effectiveDamage";
import type {
    SnapshotDamage,
    SnapshotCondition,
    SnapshotConditionState,
} from "./workerProtocol";

// ── 型別（與 eventActor.ts 相同，複製以避免引入 Vue 依賴鏈） ──────────────

export type EntityBody = {
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
};

export type EntityCondition = SnapshotCondition;
export type EntityConditionState = SnapshotConditionState;

function sameCondition(left: EntityCondition | undefined, right: EntityCondition): boolean {
    return Boolean(left)
        && left!.At === right.At
        && left!.CCId === right.CCId
        && left!.DisableAt === right.DisableAt
        && left!.DisableAtMs === right.DisableAtMs
        && left!.AttackerId === right.AttackerId
        && left!.Metadata === right.Metadata
        && left!.DurationMs === right.DurationMs;
}

export type EntityDamage = SnapshotDamage;

export type EntityItem = {
    PocketType: number;
    ItemId: number;
    Color1: string;
    Color2: string;
    Color3: string;
    Color5: string;
    Color6: string;
    Color7: string;
};

// ── PureDamageCollectorManager ──────────────────────────────────────────────

export class PureDamageCollectorManager {
    private _damages: EntityDamage[] = [];
    public get damages(): EntityDamage[] {
        return this._damages;
    }

    public onDamage(p: EntityDamage): void {
        this._damages.push(p);
    }
}

// ── PureActorManager ────────────────────────────────────────────────────────

export class PureActorManager {
    constructor(private _damageCollector: PureDamageCollectorManager) {}

    public entityMap: Record<string, PureEntityActor> = {};
    public groupMap: Record<string, PureGroupActor> = {};
    public damages: protocols.eventDamage[] = [];
    public skillActions: protocols.eventSkillAction[] = [];
    public effectiveDamages: protocols.eventDamage[] = [];
    public healthLosses: EntityHealthLoss[] = [];
    private pendingHealthDamages: Record<string, protocols.eventDamage[]> = {};
    private lastHealthMap: Record<string, number> = {};
    private pendingStatMap: Record<string, Record<number, number>> = {};
    public selectedTargetId = "";

    public static readonly pcRaceSet = new Set<number>([
        8001, 8002, 9001, 9002, 10001, 10002,
    ]);

    public onEvent(event: protocols.eventBase): void {
        if (event.EventId === protocols.eventIdSkillAction) {
            this.skillActions.push(event as protocols.eventSkillAction);
            return;
        }
        if (event.EventId === protocols.eventIdLocalEntity) {
            const local = event as protocols.eventLocalEntity;
            if (local.Reset) {
                for (const entity of Object.values(this.entityMap)) {
                    entity.resetLiveConditions(local.At);
                    entity.resetLiveStats();
                }
                this.pendingStatMap = {};
                this.pendingHealthDamages = {};
                this.lastHealthMap = {};
                this.selectedTargetId = "";
            }
            return;
        }
        if (event.EventId === protocols.eventIdCombatTarget) {
            this.selectedTargetId = (event as protocols.eventCombatTarget).TargetId ?? "";
            return;
        }

        if (event.EventId === protocols.eventIdEntityAppear) {
            this.onEntityAppear(event as protocols.eventEntityAppear);
            return;
        }

        const entity = this.entityMap[event.Id];

        switch (event.EventId) {
            case protocols.eventIdEntityDisappear:
                if (event.Id === this.selectedTargetId) this.selectedTargetId = "";
                delete this.pendingHealthDamages[event.Id];
                delete this.lastHealthMap[event.Id];
                break;

            case protocols.eventIdDamage: {
                const event_ = event as protocols.eventDamage;
                this.damages.push(event_);
                (this.pendingHealthDamages[event_.TargetId] ??= []).push(event_);

                const targetEntity = this.entityMap[event_.TargetId];
                if (!targetEntity) {
                    this.onEntityAppear({
                        Id: event_.TargetId,
                        EventId: 1,
                        At: Date.now() / 1000,
                        Name: `unknown:${event_.TargetId}`,
                        RaceId: 10001,
                        Height: 1,
                        Weight: 1,
                        Upper: 1,
                        Lower: 1,
                        GuildName: "",
                        OwnerId: "",
                    });
                    // The dummy target now exists, so credit the source once.
                    this.applyDamageToOwner(event_);
                    // onEntityAppear replays the queued hit once.
                    break;
                }

                this.applyDamageToOwner(event_);
                targetEntity.onTakeDamage(event_);
                targetEntity.group.onTakeDamage(event_);
                break;
            }

            case protocols.eventIdCharacterConditionEnable:
                if (!entity) return;
                entity.onCharacterConditionEnable(
                    event as protocols.eventCharacterConditionEnable,
                );
                break;

            case protocols.eventIdCharacterConditionDisable:
                if (!entity) return;
                entity.onCharacterConditionDisable(
                    event as protocols.eventCharacterConditionDisable,
                );
                break;

            case protocols.eventIdFinish:
                if (!entity) return;
                entity.onFinish(event as protocols.eventFinish);
                delete this.pendingHealthDamages[event.Id];
                break;

            case protocols.eventIdEntityEquipItem:
                if (!entity) return;
                entity.onEquipItem(event as protocols.eventEntityEquipItem);
                break;

            case protocols.eventIdEntityUnequipItem:
                if (!entity) return;
                entity.onUnequipItem(event as protocols.eventEntityUnequipItem);
                break;

            case protocols.eventIdEntityUpdateBody:
                if (!entity) return;
                entity.onUpdateBody(event as protocols.eventEntityUpdateBody);
                break;
            case protocols.eventIdStatUpdate: {
                const update = event as protocols.eventStatUpdate;
                this.reconcileEffectiveDamage(update);
                if (!entity) {
                    const pending = (this.pendingStatMap[event.Id] ??= {});
                    for (const stat of update.Stats ?? []) pending[stat.StatId] = stat.Value;
                    return;
                }
                entity.onStatUpdate(update);
                break;
            }
        }
    }

    public onEntityAppear(event: protocols.eventEntityAppear): void {
        const { Id, RaceId, Name } = event;
        const groupKey = PureActorManager.groupTargetKey(event);

        if (!this.groupMap[groupKey]) {
            this.groupMap[groupKey] = new PureGroupActor(
                this,
                groupKey,
                RaceId,
                Name,
            );
        }
        const group = this.groupMap[groupKey];

        let entity = this.entityMap[Id];
        const isNewEntity = !entity;
        const dummyEntity = entity?.name.startsWith("unknown:");

        if (isNewEntity) {
            entity = new PureEntityActor(this, Id, RaceId, Name, group);
            this.entityMap[Id] = group.entityMap[Id] = entity;
        }

        entity.onEntityAppear(event);
        const pendingStats = this.pendingStatMap[Id];
        if (pendingStats) {
            entity.onStatUpdate({ EventId: 17, At: event.At, Id, Private: false, Stats: Object.entries(pendingStats).map(([statId, value]) => ({ StatId: Number(statId), Value: value })) });
            delete this.pendingStatMap[Id];
        }

        if (isNewEntity || dummyEntity) {
            for (const v of this.damages) {
                const source = this.entityMap[v.Id];
                if (v.Id === Id || source?.ownerId === Id) {
                    this.applyDamageToOwner(v);
                }
                if (v.TargetId === Id) {
                    entity.onTakeDamage(v);
                    entity.group.onTakeDamage(v);
                }
            }
        }
    }

    private applyDamageToOwner(event: protocols.eventDamage): void {
        const attacker = this.entityMap[event.Id];
        if (!attacker) return;

        if (attacker.ownerId && !isMarionetteDamageSkill(event.SkillId)) {
            return;
        }

        const creditedEntity = attacker.ownerId
            ? this.entityMap[attacker.ownerId]
            : attacker;
        if (!creditedEntity) return;

        creditedEntity.onApplyDamage(event);
        creditedEntity.group.onApplyDamage(event);
    }

    public onEntityDamage(event: EntityDamage): void {
        this._damageCollector.onDamage(event);
    }

    private reconcileEffectiveDamage(update: protocols.eventStatUpdate): void {
        const health = update.Stats?.find(
            (stat) => stat.StatId === CURRENT_HEALTH_STAT_ID,
        )?.Value;
        if (!Number.isFinite(health)) return;
        const previous = this.lastHealthMap[update.Id];
        if (Number.isFinite(previous)) {
            const actualLoss = previous - Math.max(0, health!);
            if (actualLoss > 0) {
                const pending = this.pendingHealthDamages[update.Id] ?? [];
                this.healthLosses.push({
                    Id: update.Id,
                    At: pending.at(-1)?.At ?? update.At,
                    Damage: actualLoss,
                });
            }
            for (const allocation of allocateEffectiveDamage(
                this.pendingHealthDamages[update.Id] ?? [],
                previous,
                health!,
            )) {
                const attacker = this.entityMap[allocation.event.Id];
                const creditedId = attacker?.ownerId
                    && isMarionetteDamageSkill(allocation.event.SkillId)
                    ? attacker.ownerId
                    : allocation.event.Id;
                this.effectiveDamages.push({
                    ...allocation.event,
                    Id: creditedId,
                    Damage: allocation.damage,
                });
            }
        }
        this.pendingHealthDamages[update.Id] = [];
        this.lastHealthMap[update.Id] = health!;
    }

    public static groupTargetKey(event: protocols.eventEntityAppear): string {
        if (PureActorManager.pcRaceSet.has(event.RaceId)) {
            return event.Id;
        }
        return `${event.RaceId}`;
    }
}

// ── PureBaseActor ────────────────────────────────────────────────────────────

abstract class PureBaseActor {
    protected _id: string;
    protected _raceId: number;
    protected _name: string;
    protected _isPC: boolean;
    protected _body: EntityBody = { Height: 1, Weight: 1, Upper: 1, Lower: 1 };
    protected _totalTakeDamage = 0;
    protected _takeDamages: EntityDamage[] = [];
    protected _totalApplyDamage = 0;
    protected _applyDamages: EntityDamage[] = [];

    constructor(
        protected mgr: PureActorManager,
        id: string,
        raceId: number,
        name: string,
    ) {
        this._id = id;
        this._raceId = raceId;
        this._name = name;
        this._isPC = PureActorManager.pcRaceSet.has(raceId);
    }

    public get id() {
        return this._id;
    }
    public get raceId() {
        return this._raceId;
    }
    public get name() {
        return this._name;
    }
    public get body() {
        return this._body;
    }
    public get totalTakeDamage() {
        return this._totalTakeDamage;
    }
    public get takeDamages() {
        return this._takeDamages;
    }
    public get totalApplyDamage() {
        return this._totalApplyDamage;
    }
    public get applyDamages() {
        return this._applyDamages;
    }
    public get isPC() {
        return this._isPC;
    }

    public onEntityAppear(_event: protocols.eventEntityAppear): void {}
    public onTakeDamage(_event: protocols.eventDamage): void {}
    public onApplyDamage(_event: protocols.eventDamage): void {}
    public onCharacterConditionEnable(
        _event: protocols.eventCharacterConditionEnable,
    ): void {}
    public onCharacterConditionDisable(
        _event: protocols.eventCharacterConditionDisable,
    ): void {}
    public onFinish(_event: protocols.eventFinish): void {}
    public onEquipItem(_event: protocols.eventEntityEquipItem): void {}
    public onUnequipItem(_event: protocols.eventEntityUnequipItem): void {}

    public onUpdateBody(event: protocols.eventEntityUpdateBody): void {
        this._body.Height = event.Height;
        this._body.Weight = event.Weight;
        this._body.Upper = event.Upper;
        this._body.Lower = event.Lower;
    }
}

// ── PureEntityActor ──────────────────────────────────────────────────────────

export class PureEntityActor extends PureBaseActor {
    private _guildName = "";
    private _ownerId = "";
    private _finisherId = "";
    private _group: PureGroupActor;
    private _conditionMap: Record<number, EntityCondition> = {};
    private _conditionRefreshGuardAt: Record<number, number> = {};
    private _conditionHistory: EntityConditionState[] = [];
    private _equipItemMap: Record<number, EntityItem> = {};
    private _statMap: Record<number, number> = {};
    private _appearedAt = 0;

    public constructor(
        mgr: PureActorManager,
        id: string,
        raceId: number,
        name: string,
        group: PureGroupActor,
    ) {
        super(mgr, id, raceId, name);
        this._group = group;
    }

    public get guildName() {
        return this._guildName;
    }
    public get ownerId() {
        return this._ownerId;
    }
    public get finisherId() {
        return this._finisherId;
    }
    public get group() {
        return this._group;
    }
    public get conditionMap() {
        return this._conditionMap;
    }
    public get conditionHistory() {
        return this._conditionHistory;
    }
    public get equipItemMap() {
        return this._equipItemMap;
    }
    public get statMap() { return this._statMap; }
    public get appearedAt() { return this._appearedAt; }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this._appearedAt = event.At;
        this._name = event.Name;
        this._raceId = event.RaceId;
        this._finisherId = "";
        this._guildName = event.GuildName;
        this._ownerId = event.OwnerId;
        this._body.Height = event.Height;
        this._body.Weight = event.Weight;
        this._body.Upper = event.Upper;
        this._body.Lower = event.Lower;

        if (PureActorManager.pcRaceSet.has(event.RaceId)) {
            return;
        }

        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        const attacker = this.mgr.entityMap[event.Id];

        const damage: EntityDamage = {
            ...event,
            Conditions: attacker?.getConditionState(event.At) ?? [],
            TargetConditions: this.getConditionState(event.At),
            PetId: "",
        };

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);
    }

    public override onApplyDamage(event: protocols.eventDamage): void {
        const attackerId = event.Id;
        const attacker = this.mgr.entityMap[attackerId];

        const targetId = event.TargetId;
        const target = this.mgr.entityMap[targetId];
        if (!target || !(target instanceof PureEntityActor)) {
            return;
        }

        const damage: EntityDamage = {
            ...event,
            Conditions: this.getConditionState(event.At),
            TargetConditions: target.getConditionState(event.At),
            PetId: "",
        };

        if (attacker?.ownerId) {
            damage.PetId = attackerId;
            damage.Id = attacker.ownerId;
        }

        this._totalApplyDamage += event.Damage;
        this._applyDamages.push(damage);

        this.mgr.onEntityDamage(damage);
    }

    public override onCharacterConditionEnable(
        event: protocols.eventCharacterConditionEnable,
    ): void {
        const next: EntityCondition = {
            Id: event.Id,
            At: event.At,
            CCId: event.CCId,
            DisableAt: event.DisableAt,
            DisableAtMs: event.DisableAtMs ?? 0,
            AttackerId: event.AttackerId,
            Metadata: event.Metadata ?? "",
            DurationMs: event.DurationMs ?? 0,
        };
        const existing = this._conditionMap[event.CCId];
        if (sameCondition(existing, next)) return;
        const wasActive = Boolean(existing);
        this._conditionMap[event.CCId] = next;
        if (wasActive) {
            this._conditionRefreshGuardAt[event.CCId] = event.At;
        } else {
            delete this._conditionRefreshGuardAt[event.CCId];
        }

        const prev = this._conditionHistory.length
            ? this._conditionHistory[this._conditionHistory.length - 1].List
            : [];
        const current = Object.values(this._conditionMap).sort(
            (a, b) => a.CCId - b.CCId,
        );

        const needUpdate =
            prev.length !== current.length ||
            !prev.every((v, i) =>
                v.CCId === current[i].CCId &&
                v.At === current[i].At &&
                v.DisableAt === current[i].DisableAt &&
                v.DisableAtMs === current[i].DisableAtMs &&
                v.DurationMs === current[i].DurationMs,
            );
        if (needUpdate) {
            this._conditionHistory.push({ At: event.At, List: current });
        }
    }

    public override onCharacterConditionDisable(
        event: protocols.eventCharacterConditionDisable,
    ): void {
        const active = this._conditionMap[event.CCId];
        if (!active) return;
        const refreshAt = this._conditionRefreshGuardAt[event.CCId];
        if (
            active &&
            refreshAt !== undefined &&
            refreshAt === active.At &&
            event.At >= refreshAt &&
            event.At - refreshAt <= 1.25
        ) {
            delete this._conditionRefreshGuardAt[event.CCId];
            return;
        }
        delete this._conditionRefreshGuardAt[event.CCId];
        delete this._conditionMap[event.CCId];

        const prev = this._conditionHistory.length
            ? this._conditionHistory[this._conditionHistory.length - 1].List
            : [];
        const current = Object.values(this._conditionMap).sort(
            (a, b) => a.CCId - b.CCId,
        );

        const needUpdate =
            prev.length !== current.length ||
            !prev.every((v, i) => v.CCId === current[i].CCId);
        if (needUpdate) {
            this._conditionHistory.push({ At: event.At, List: current });
        }
    }

    public override onFinish(event: protocols.eventFinish): void {
        this._finisherId = event.AttackerId;
    }

    public override onEquipItem(event: protocols.eventEntityEquipItem): void {
        this._equipItemMap[event.PocketType] = event;
    }

    public override onUnequipItem(
        event: protocols.eventEntityUnequipItem,
    ): void {
        delete this._equipItemMap[event.PocketType];
    }

    public onStatUpdate(event: protocols.eventStatUpdate): void {
        for (const stat of event.Stats ?? []) {
            if (Number.isFinite(stat.StatId) && Number.isFinite(stat.Value)) this._statMap[stat.StatId] = stat.Value;
        }
    }

    public resetLiveStats(): void {
        const maximumHealth = this._statMap[MAXIMUM_HEALTH_STAT_ID];
        this._statMap = Number.isFinite(maximumHealth) && maximumHealth > 0
            ? { [MAXIMUM_HEALTH_STAT_ID]: maximumHealth }
            : {};
    }

    public getConditionState(at: number): EntityCondition[] {
        const idx = bounds.le<{ At: number }>(
            this._conditionHistory,
            { At: at },
            (a, b) => a.At - b.At,
        );
        if (idx < 0) return [];
        return this._conditionHistory[idx].List;
    }

    public resetLiveConditions(at: number): void {
        const hadActiveConditions = Object.keys(this._conditionMap).length > 0;
        this._conditionMap = {};
        this._conditionRefreshGuardAt = {};
        if (hadActiveConditions) {
            this._conditionHistory.push({ At: at, List: [] });
        }
    }
}

// ── PureGroupActor ───────────────────────────────────────────────────────────

export class PureGroupActor extends PureBaseActor {
    public entityMap: Record<string, PureEntityActor> = {};

    public constructor(
        mgr: PureActorManager,
        id: string,
        raceId: number,
        name: string,
    ) {
        const groupName = PureActorManager.pcRaceSet.has(raceId)
            ? name
            : `${raceId}`;
        super(mgr, id, raceId, groupName);
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this._name = PureActorManager.pcRaceSet.has(event.RaceId)
            ? event.Name
            : `${event.RaceId}`;
        this._raceId = event.RaceId;

        if (PureActorManager.pcRaceSet.has(event.RaceId)) {
            return;
        }

        const target = this.entityMap[event.Id];
        if (!target) return;

        this._takeDamages = this._takeDamages.filter((v) => v.Id !== event.Id);
        this._totalTakeDamage = this._takeDamages.reduce(
            (acc, v) => acc + v.Damage,
            0,
        );
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        const attacker = this.mgr.entityMap[event.Id];
        const target = this.entityMap[event.TargetId];
        if (!target) return;

        const damage: EntityDamage = {
            ...event,
            Conditions: attacker?.getConditionState(event.At) ?? [],
            TargetConditions: target.getConditionState(event.At),
            PetId: "",
        };

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);
    }
}
