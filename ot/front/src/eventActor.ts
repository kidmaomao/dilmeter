import { CustomReactive, IUpdateCallback } from "@/lib/util";
import { shallowReactive } from "vue";
import bounds from "binary-search-bounds";

import * as protocols from "@/protocols";
import { DamageCollectorManager } from "@/actionCollector";
import { isMarionetteDamageSkill } from "@/ownedDamagePolicy";
import {
    allocateEffectiveDamage,
    CURRENT_HEALTH_STAT_ID,
    MAXIMUM_HEALTH_STAT_ID,
    type EntityHealthLoss,
} from "@/effectiveDamage";

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

// TODO: take cc, apply cc 구분하기

export class ActorManager {
    constructor(private _damageCollector: DamageCollectorManager) {}
    private readonly pendingReactiveActors = new Set<BaseActor>();
    /** Monotonic data revision used by throttled aggregate views. */
    public eventVersion = 0;

    public entityMap: Record<string, EntityActor> = shallowReactive({});
    public groupMap: Record<string, GroupActor> = shallowReactive({});
    /** Tracks only the current server snapshot; historical actors remain in entityMap. */
    public activeEntityMap: Record<string, boolean> = shallowReactive({});
    // Condition packets can arrive before the corresponding entity packet
    // (most often when Dilmeter starts after the player has entered a map).
    // Keep them instead of dropping them so Buff reminders work immediately.
    public pendingConditionMap: Record<string, Record<number, EntityCondition>> = shallowReactive({});
    public pendingStatMap: Record<string, Record<number, number>> = shallowReactive({});
    // A refresh can be followed by the previous generation's remove packet.
    // Arm this guard only when the same CC was already active, and consume it
    // at most once. A condition's first enable must never suppress a quick,
    // legitimate disable (Condition Support effects can do exactly that).
    private pendingConditionRefreshGuardAt: Record<string, Record<number, number>> = {};
    public damages: protocols.eventDamage[] = [];
    /** Server-confirmed skill executions used by battle skill timelines. */
    public skillActions: protocols.eventSkillAction[] = [];
    /** Health-bar reconciled damage; raw packets remain in damages. */
    public effectiveDamages: protocols.eventDamage[] = [];
    /** Authoritative health-bar decreases, including pet/system damage. */
    public healthLosses: EntityHealthLoss[] = [];
    private pendingHealthDamages: Record<string, protocols.eventDamage[]> = {};
    private lastHealthMap: Record<string, number> = {};
    public localEntityId = "";
    public localEntityReliable = false;
    /** Entity explicitly selected by the local player; empty means no target. */
    public selectedTargetId = "";

    public static pcRaceSet = new Set<number>([
        8001, 8002, 9001, 9002, 10001, 10002,
    ]);

    public onEvent(event: protocols.eventBase) {
        this.eventVersion += 1;
        if (event.EventId === protocols.eventIdSkillAction) {
            this.skillActions.push(event as protocols.eventSkillAction);
            return;
        }
        if (event.EventId === protocols.eventIdLocalEntity) {
            const local = event as protocols.eventLocalEntity;
            if (local.Reset) {
                this.resetLiveSession(local.At);
            }
            this.localEntityId = local.Id;
            this.localEntityReliable = local.Reliable;
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
                this.activeEntityMap[event.Id] = false;
                if (event.Id === this.selectedTargetId) this.selectedTargetId = "";
                // The local player temporarily disappears while changing maps.
                // Their server-side Buffs can continue across that boundary, so
                // closing every live condition here creates a false interruption
                // in both the overlay countdown and coverage history.  Explicit
                // CC disable packets remain authoritative and still clear each
                // Buff normally. Other entities are visibility-scoped and retain
                // the original cleanup behaviour.
                if (event.Id !== this.localEntityId) {
                    entity?.resetLiveConditions(event.At);
                    delete this.pendingConditionMap[event.Id];
                    delete this.pendingConditionRefreshGuardAt[event.Id];
                }
                delete this.pendingStatMap[event.Id];
                delete this.pendingHealthDamages[event.Id];
                delete this.lastHealthMap[event.Id];
                break;

            case protocols.eventIdDamage:
                {
                    const event_ = event as protocols.eventDamage;
                    this.damages.push(event_);
                    (this.pendingHealthDamages[event_.TargetId] ??= []).push(event_);

                    const targetEntity = this.entityMap[event_.TargetId];
                    if (!targetEntity) {
                        // 유저 정보가 오기전에 대미지가 먼저오는 경우
                        // @TODO: 추후에 local storage를 사용해 캐싱하는 식으로 변경
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
                        // The dummy target now exists, so the source-side
                        // damage can be credited exactly once.
                        this.applyDamageToOwner(event_);
                        // onEntityAppear replays the raw damage that was just
                        // queued. Processing it again here would count the
                        // first hit against an unknown target twice.
                        break;
                    }

                    this.applyDamageToOwner(event_);
                    targetEntity.onTakeDamage(event_);
                    targetEntity.group.onTakeDamage(event_);
                }
                break;

            case protocols.eventIdCharacterConditionEnable:
                if (!entity) {
                    this.storePendingCondition(
                        event as protocols.eventCharacterConditionEnable,
                    );
                    return;
                }

                entity.onCharacterConditionEnable(
                    event as protocols.eventCharacterConditionEnable,
                );
                break;

            case protocols.eventIdCharacterConditionDisable:
                if (!entity) {
                    this.removePendingCondition(
                        event as protocols.eventCharacterConditionDisable,
                    );
                    return;
                }

                entity.onCharacterConditionDisable(
                    event as protocols.eventCharacterConditionDisable,
                );
                break;

            case protocols.eventIdFinish:
                if (!entity) {
                    return;
                }

                this.activeEntityMap[event.Id] = false;
                entity.onFinish(event as protocols.eventFinish);
                delete this.pendingHealthDamages[event.Id];
                break;

            case protocols.eventIdEntityEquipItem:
                if (!entity) {
                    return;
                }

                entity.onEquipItem(event as protocols.eventEntityEquipItem);
                break;

            case protocols.eventIdEntityUnequipItem:
                if (!entity) {
                    return;
                }

                entity.onUnequipItem(event as protocols.eventEntityUnequipItem);
                break;

            case protocols.eventIdEntityUpdateBody:
                if (!entity) {
                    return;
                }

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

    /** Rebuilds reconciled damage after loading a legacy battle snapshot. */
    public replaceEffectiveDamages(events: readonly protocols.eventDamage[]) {
        this.effectiveDamages.length = 0;
        this.effectiveDamages.push(...events);
    }

    public replaceHealthLosses(events: readonly EntityHealthLoss[]) {
        this.healthLosses.length = 0;
        this.healthLosses.push(...events);
    }

    public onEntityAppear(event: protocols.eventEntityAppear) {
        const { Id, RaceId, Name } = event;
        this.activeEntityMap[Id] = true;

        const groupKey = ActorManager.groupTargetKey(event);
        const group = (this.groupMap[groupKey] ??= CustomReactive(
            new GroupActor(this, groupKey, RaceId, Name),
        ));

        let entity = this.entityMap[Id];
        const isNewEntity = !entity;
        const dummyEntity = entity?.name.startsWith("unknown:");

        if (isNewEntity) {
            entity = CustomReactive(
                new EntityActor(this, Id, RaceId, Name, group),
            );
            this.entityMap[Id] = group.entityMap[Id] = entity;
        }

        // OwnerId must be known before replaying earlier puppet/pet damage.
        entity.onEntityAppear(event);

        const pendingStats = this.pendingStatMap[Id];
        if (pendingStats) {
            entity.onStatUpdate({
                EventId: protocols.eventIdStatUpdate,
                At: event.At,
                Id,
                Private: false,
                Stats: Object.entries(pendingStats).map(([statId, value]) => ({ StatId: Number(statId), Value: value })),
            });
            delete this.pendingStatMap[Id];
        }

        const pendingConditions = this.pendingConditionMap[Id];
        if (pendingConditions) {
            const refreshGuards = this.pendingConditionRefreshGuardAt[Id];
            for (const condition of Object.values(pendingConditions).sort((a, b) => a.At - b.At)) {
                entity.onCharacterConditionEnable({
                    EventId: protocols.eventIdCharacterConditionEnable,
                    ...condition,
                });
                if (refreshGuards?.[condition.CCId] === condition.At) {
                    entity.armConditionRefreshRemoveGuard(condition.CCId, condition.At);
                }
            }
            delete this.pendingConditionMap[Id];
            delete this.pendingConditionRefreshGuardAt[Id];
        }

        if (isNewEntity || dummyEntity) {
            // entity appear를 받은 뒤에 api가 켜진 경우
            for (const v of this.damages) {
                const source = this.entityMap[v.Id];
                if (v.Id == Id || source?.ownerId == Id) {
                    this.applyDamageToOwner(v);
                }
                if (v.TargetId == Id) {
                    entity.onTakeDamage(v);
                    entity.group.onTakeDamage(v);
                }
            }
        }
    }

    /**
     * Credits supported marionette damage to its player owner. Normal pet and
     * other owned-source damage are intentionally excluded. If the owner entity
     * has not appeared yet, the raw event remains in damages and is replayed
     * when the owner appears.
     */
    private applyDamageToOwner(event: protocols.eventDamage) {
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

    private storePendingCondition(event: protocols.eventCharacterConditionEnable) {
        const current = this.pendingConditionMap[event.Id] ?? {};
        if (current[event.CCId]) {
            const guards = (this.pendingConditionRefreshGuardAt[event.Id] ??= {});
            guards[event.CCId] = event.At;
        } else {
            this.clearPendingConditionRefreshGuard(event.Id, event.CCId);
        }
        this.pendingConditionMap[event.Id] = {
            ...current,
            [event.CCId]: {
                Id: event.Id,
                At: event.At,
                CCId: event.CCId,
                DisableAt: event.DisableAt,
                DisableAtMs: event.DisableAtMs ?? 0,
                AttackerId: event.AttackerId,
                Metadata: event.Metadata ?? "",
                DurationMs: event.DurationMs ?? 0,
            },
        };
    }

    private removePendingCondition(event: protocols.eventCharacterConditionDisable) {
        const current = this.pendingConditionMap[event.Id];
        const active = current?.[event.CCId];
        if (!active) return;
        const refreshAt = this.pendingConditionRefreshGuardAt[event.Id]?.[event.CCId];
        if (
            refreshAt !== undefined &&
            refreshAt === active.At &&
            event.At >= refreshAt &&
            event.At - refreshAt <= 1.25
        ) {
            this.clearPendingConditionRefreshGuard(event.Id, event.CCId);
            return;
        }
        this.clearPendingConditionRefreshGuard(event.Id, event.CCId);
        const next = { ...current };
        delete next[event.CCId];
        if (Object.keys(next).length) {
            this.pendingConditionMap[event.Id] = next;
        } else {
            delete this.pendingConditionMap[event.Id];
        }
    }

    private clearPendingConditionRefreshGuard(entityId: string, ccId: number) {
        const guards = this.pendingConditionRefreshGuardAt[entityId];
        if (!guards) return;
        delete guards[ccId];
        if (!Object.keys(guards).length) delete this.pendingConditionRefreshGuardAt[entityId];
    }

    public onEntityDamage(event: EntityDamage) {
        this._damageCollector.onDamage(event);
    }

    public forceUpdateAll(): void {
        for (const k in this.entityMap) {
            this.entityMap[k].forceUpdate();
        }
        for (const k in this.groupMap) {
            this.groupMap[k].forceUpdate();
        }
    }

    /**
     * Complete reactive updates whose short browser timer has been throttled.
     * Unlike forceUpdateAll this is cheap while idle because actors without a
     * pending update do not trigger Vue rendering.
     */
    public flushPendingUpdates(): void {
        for (const actor of [...this.pendingReactiveActors]) actor.flushPendingUpdate();
    }

    public markPendingUpdate(actor: BaseActor): void {
        this.pendingReactiveActors.add(actor);
    }

    public clearPendingUpdate(actor: BaseActor): void {
        this.pendingReactiveActors.delete(actor);
    }

    public pauseReactivity(): void {
        BaseActor.pause();
    }

    public resumeReactivity(): void {
        BaseActor.resume();
        this.forceUpdateAll();
    }

    /** Release the previous window's pending work before replacing its actors. */
    public prepareSnapshot(): void {
        for (const actor of this.pendingReactiveActors) actor.flushPendingUpdate();
        this.pendingReactiveActors.clear();
        for (const id of Object.keys(this.activeEntityMap)) delete this.activeEntityMap[id];
        for (const id of Object.keys(this.pendingConditionMap)) delete this.pendingConditionMap[id];
        for (const id of Object.keys(this.pendingStatMap)) delete this.pendingStatMap[id];
        this.pendingConditionRefreshGuardAt = {};
        this.pendingHealthDamages = {};
        this.lastHealthMap = {};
    }

    public restoreSnapshotResumeState(state: {
        pendingHealthDamages: Record<string, protocols.eventDamage[]>;
        lastHealthMap: Record<string, number>;
        pendingStatMap: Record<string, Record<number, number>>;
    }): void {
        this.pendingHealthDamages = state.pendingHealthDamages;
        this.lastHealthMap = state.lastHealthMap;
        Object.assign(this.pendingStatMap, state.pendingStatMap);
    }

    public clear() {
		this.eventVersion += 1;
        this.selectedTargetId = "";
        // object instance를 새로 만들면 귀찮아짐
        this.damages.length = 0;
        this.skillActions.length = 0;
        this.effectiveDamages.length = 0;
        this.healthLosses.length = 0;
        this.pendingHealthDamages = {};
        this.lastHealthMap = {};

        for (const k in this.entityMap) {
            const v = this.entityMap[k];

            v.clear();
        }

        for (const k in this.groupMap) {
            const v = this.groupMap[k];

            v.clear();
        }
    }

    /**
     * A channel change replaces the server's live entity snapshot. Close all
     * currently active condition ranges and discard pending identity state,
     * but intentionally retain damage and completed condition history.
     */
    private resetLiveSession(at: number) {
        for (const entity of Object.values(this.entityMap)) {
            entity.resetLiveConditions(at);
            this.activeEntityMap[entity.id] = false;
        }
        for (const id of Object.keys(this.pendingConditionMap)) {
            delete this.pendingConditionMap[id];
        }
        for (const id of Object.keys(this.pendingConditionRefreshGuardAt)) {
            delete this.pendingConditionRefreshGuardAt[id];
        }
        for (const id of Object.keys(this.pendingStatMap)) delete this.pendingStatMap[id];
        for (const entity of Object.values(this.entityMap)) entity.resetLiveStats();
        this.pendingHealthDamages = {};
        this.lastHealthMap = {};
        this.localEntityId = "";
        this.localEntityReliable = false;
        this.selectedTargetId = "";
    }

    private reconcileEffectiveDamage(update: protocols.eventStatUpdate) {
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
                    // Attribute the health change to its latest causal packet.
                    // StatUpdate only has whole-second time and may arrive in
                    // the next second after a finishing hit.
                    At: pending.at(-1)?.At ?? update.At,
                    Damage: actualLoss,
                });
            }
            const pending = this.pendingHealthDamages[update.Id] ?? [];
            for (const allocation of allocateEffectiveDamage(
                pending,
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

    private static groupTargetKey(event: protocols.eventEntityAppear): string {
        // pc 일 경우 group안에 entity가 여러개 생기지 않도록
        if (this.pcRaceSet.has(event.RaceId)) {
            return event.Id;
        }

        return `${event.RaceId}`;
    }
}

interface IEventActor {
    /** damage 쪽 수치들만 reset */
    clear(): void;
}

export abstract class BaseActor implements IEventActor, IUpdateCallback {
    protected vueUpdateTrack?: () => void;
    private vueUpdateTrigger?: () => void;
    private vueUpdateTimeout = 0;
    private static vueUpdateTick = 33;
    private static _paused = false;

    public static pause(): void {
        BaseActor._paused = true;
    }
    public static resume(): void {
        BaseActor._paused = false;
    }

    protected constructor(
        protected mgr: ActorManager,
        private _id: string,
        protected _raceId: number,
        protected _name: string,
    ) {
        this._isPC = ActorManager.pcRaceSet.has(_raceId);
    }

    public get id() {
        this.vueUpdateTrack?.();
        return this._id;
    }

    public get raceId() {
        this.vueUpdateTrack?.();
        return this._raceId;
    }

    public get name() {
        this.vueUpdateTrack?.();
        return this._name;
    }

    protected _body: EntityBody = {
        Height: 1,
        Weight: 1,
        Upper: 1,
        Lower: 1,
    };
    public get body() {
        this.vueUpdateTrack?.();
        return this._body;
    }

    /** 받은 대미지 */
    public get totalTakeDamage() {
        this.vueUpdateTrack?.();
        return this._totalTakeDamage;
    }
    protected _totalTakeDamage = 0;

    protected _takeDamages: EntityDamage[] = [];
    public get takeDamages() {
        this.vueUpdateTrack?.();
        return this._takeDamages;
    }

    /** 준 대미지 */
    public get totalApplyDamage() {
        this.vueUpdateTrack?.();
        return this._totalApplyDamage;
    }
    protected _totalApplyDamage = 0;

    protected _applyDamages: EntityDamage[] = [];
    public get applyDamages() {
        this.vueUpdateTrack?.();
        return this._applyDamages;
    }

    private _isPC = false;
    public get isPC() {
        this.vueUpdateTrack?.();
        return this._isPC;
    }

    public onEntityAppear(event: protocols.eventEntityAppear): void {
        // nothing
        event;
    }

    public onTakeDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onApplyDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onCharacterConditionEnable(
        event: protocols.eventCharacterConditionEnable,
    ): void {
        // nothing
        event;
    }

    public onCharacterConditionDisable(
        event: protocols.eventCharacterConditionDisable,
    ): void {
        // nothing
        event;
    }

    public onFinish(event: protocols.eventFinish): void {
        // nothing
        event;
    }

    public onEquipItem(event: protocols.eventEntityEquipItem): void {
        // nothing
        event;
    }

    public onUnequipItem(event: protocols.eventEntityUnequipItem): void {
        // nothing
        event;
    }

    public onUpdateBody(event: protocols.eventEntityUpdateBody): void {
        this._body.Height = event.Height;
        this._body.Weight = event.Weight;
        this._body.Upper = event.Upper;
        this._body.Lower = event.Lower;

        this.vueUpdateRequest();
    }

    public clear() {
        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;

        this._totalApplyDamage = 0;
        this._applyDamages.length = 0;

        this.vueUpdateRequest();
    }

    public setUpdateCallback(track: () => void, trigger: () => void): void {
        this.vueUpdateTrack = track;
        this.vueUpdateTrigger = trigger;
    }

    private vueUpdate(): void {
        this.vueUpdateTimeout = 0;
        this.mgr.clearPendingUpdate(this);
        this.vueUpdateTrigger?.();
    }

    protected vueUpdateRequest(): void {
        if (BaseActor._paused) {
            return;
        }

        if (this.vueUpdateTimeout) {
            return;
        }

        this.vueUpdateTimeout = setTimeout(
            () => this.vueUpdate(),
            BaseActor.vueUpdateTick,
        );
        this.mgr.markPendingUpdate(this);
    }

    public forceUpdate(): void {
        if (this.vueUpdateTimeout) {
            clearTimeout(this.vueUpdateTimeout);
            this.vueUpdateTimeout = 0;
        }
        this.mgr.clearPendingUpdate(this);
        this.vueUpdateTrigger?.();
    }

    public flushPendingUpdate(): void {
        if (!this.vueUpdateTimeout) return;
        clearTimeout(this.vueUpdateTimeout);
        this.vueUpdate();
    }
}

export class EntityActor extends BaseActor {
    public constructor(
        mgr: ActorManager,
        id: string,
        raceId: number,
        name: string,
        private _group: GroupActor,
    ) {
        super(mgr, id, raceId, name);
    }

    protected _guildName = "";
    public get guildName() {
        this.vueUpdateTrack?.();
        return this._guildName;
    }

    protected _appearedAt = 0;
    public get appearedAt() {
        this.vueUpdateTrack?.();
        return this._appearedAt;
    }

    protected _ownerId = "";
    public get ownerId() {
        this.vueUpdateTrack?.();
        return this._ownerId;
    }

    public get group() {
        this.vueUpdateTrack?.();
        return this._group;
    }

    protected _conditionMap: Record<number, EntityCondition> = {};
    private _conditionRefreshGuardAt: Record<number, number> = {};
    public get conditionMap() {
        this.vueUpdateTrack?.();
        return this._conditionMap;
    }

    protected _conditionHistory: EntityConditionState[] = [];
    public get conditionHistory() {
        this.vueUpdateTrack?.();
        return this._conditionHistory;
    }

    protected _statMap: Record<number, number> = {};
    public get statMap() {
        this.vueUpdateTrack?.();
        return this._statMap;
    }

    protected _finisherId = "";
    public get finisherId() {
        this.vueUpdateTrack?.();
        return this._finisherId;
    }

    protected _equipItemMap: Record<number, EntityItem> = {};
    public get equipItemMap() {
        this.vueUpdateTrack?.();
        return this._equipItemMap;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this.vueUpdateRequest();

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

        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        this.vueUpdateRequest();

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
        this.vueUpdateRequest();

        const attackerId = event.Id;
        const attacker = this.mgr.entityMap[attackerId];

        const targetId = event.TargetId;
        const target = this.mgr.entityMap[targetId];
        if (!target || !(target instanceof EntityActor)) {
            // 몹 정보가 없으면 무시
            return;
        }

        const damage: EntityDamage = {
            ...event,

            Conditions: this.getConditionState(event.At),
            TargetConditions: target.getConditionState(event.At),
            PetId: "",
        };

        // apply의 경우 pet 대신 pc가 공격한 것 처럼 처리
        if (attacker?.ownerId) {
            damage.PetId = attackerId;
            damage.Id = attacker.ownerId;
        }

        this._totalApplyDamage += event.Damage;
        this._applyDamages.push(damage);

        // apply에서만 호출
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

        this.vueUpdateRequest();
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
            this._conditionHistory.push({
                At: event.At,
                List: current,
            });
        }
    }

    public armConditionRefreshRemoveGuard(ccId: number, refreshedAt: number): void {
        if (this._conditionMap[ccId]?.At === refreshedAt) {
            this._conditionRefreshGuardAt[ccId] = refreshedAt;
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

        this.vueUpdateRequest();

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
            this._conditionHistory.push({
                At: event.At,
                List: current,
            });
        }
    }

    public override onFinish(event: protocols.eventFinish): void {
        this.vueUpdateRequest();

        this._finisherId = event.AttackerId;
    }

    public override onEquipItem(event: protocols.eventEntityEquipItem): void {
        this.vueUpdateRequest();

        this._equipItemMap[event.PocketType] = event;
    }

    public override onUnequipItem(
        event: protocols.eventEntityUnequipItem,
    ): void {
        this.vueUpdateRequest();

        delete this._equipItemMap[event.PocketType];
    }

    public onStatUpdate(event: protocols.eventStatUpdate): void {
        let changed = false;
        for (const stat of event.Stats ?? []) {
            if (!Number.isFinite(stat.StatId) || !Number.isFinite(stat.Value) || this._statMap[stat.StatId] === stat.Value) continue;
            this._statMap[stat.StatId] = stat.Value;
            changed = true;
        }
        if (changed) this.vueUpdateRequest();
    }

    public resetLiveStats(): void {
        const maximumHealth = this._statMap[MAXIMUM_HEALTH_STAT_ID];
        this._statMap = Number.isFinite(maximumHealth) && maximumHealth > 0
            ? { [MAXIMUM_HEALTH_STAT_ID]: maximumHealth }
            : {};
        this.vueUpdateRequest();
    }

    public getConditionState(at: number): EntityCondition[] {
        const idx = bounds.le<{ At: number }>(
            this._conditionHistory,
            { At: at },
            (a, b) => a.At - b.At,
        );
        if (idx < 0) {
            return [];
        }

        return this._conditionHistory[idx].List;
    }

    public resetLiveConditions(at: number): void {
        const hadActiveConditions = Object.keys(this._conditionMap).length > 0;
        for (const ccId of Object.keys(this._conditionMap)) {
            delete this._conditionMap[Number(ccId)];
        }
        for (const ccId of Object.keys(this._conditionRefreshGuardAt)) {
            delete this._conditionRefreshGuardAt[Number(ccId)];
        }
        if (hadActiveConditions) {
            this._conditionHistory.push({ At: at, List: [] });
        }
        this.vueUpdateRequest();
    }

    public override clear() {
        super.clear();
    }

    /** 從 Worker snapshot 直接還原內部狀態，繞過事件重播 */
    public hydrateFrom(s: {
        name: string;
        raceId: number;
        guildName: string;
        ownerId: string;
        finisherId: string;
        body: { Height: number; Weight: number; Upper: number; Lower: number };
        totalTakeDamage: number;
        takeDamages: EntityDamage[];
        totalApplyDamage: number;
        applyDamages: EntityDamage[];
        conditionMap: Record<number, EntityCondition>;
        conditionHistory: EntityConditionState[];
        conditionRefreshGuardAt?: Record<number, number>;
        equipItemMap: Record<number, EntityItem>;
        statMap?: Record<number, number>;
        appearedAt?: number;
    }): void {
        this._name = s.name;
        this._raceId = s.raceId;
        this._body = { ...s.body };
        this._totalTakeDamage = s.totalTakeDamage;
        this._takeDamages = s.takeDamages;
        this._totalApplyDamage = s.totalApplyDamage;
        this._applyDamages = s.applyDamages;
        this._guildName = s.guildName;
        this._ownerId = s.ownerId;
        this._finisherId = s.finisherId;
        this._conditionMap = s.conditionMap as Record<number, EntityCondition>;
        this._conditionHistory = s.conditionHistory as EntityConditionState[];
        this._conditionRefreshGuardAt = s.conditionRefreshGuardAt ?? {};
        this._equipItemMap = s.equipItemMap as Record<number, EntityItem>;
        this._statMap = { ...(s.statMap ?? {}) };
        this._appearedAt = Number(s.appearedAt) || 0;
    }

    /** Adds one recovered owner-attributed hit from an older battle record. */
    public hydrateAdditionalApplyDamage(damage: EntityDamage): void {
        this._applyDamages.push(damage);
        this._totalApplyDamage += damage.Damage;
    }
}

// TODO: GroupActor에 Group 조건 추가하는 식으로 바꾸는게 좋을듯
export class GroupActor extends BaseActor {
    public constructor(
        mgr: ActorManager,
        id: string,
        raceId: number,
        name: string,
    ) {
        const groupName = ActorManager.pcRaceSet.has(raceId)
            ? name
            : `${raceId}`;

        super(mgr, id, raceId, groupName);
    }

    private _entityMap: Record<string, EntityActor> = {};
    public get entityMap() {
        this.vueUpdateTrack?.();
        return this._entityMap;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this.vueUpdateRequest();

        this._name = ActorManager.pcRaceSet.has(event.RaceId)
            ? event.Name
            : `${event.RaceId}`;
        this._raceId = event.RaceId;

        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        const target = this._entityMap[event.Id];
        if (!target) {
            // ?
            return;
        }

        this._takeDamages = this._takeDamages.filter((v) => v.Id !== event.Id);
        this._totalTakeDamage = this._takeDamages.reduce(
            (acc, v) => acc + v.Damage,
            0,
        );
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        this.vueUpdateRequest();
        // console.log('group take damage', this.id, event);

        const attacker = this.mgr.entityMap[event.Id];
        const target = this._entityMap[event.TargetId];
        if (!target) {
            // ?
            return;
        }

        const damage: EntityDamage = {
            ...event,

            Conditions: attacker?.getConditionState(event.At) ?? [],
            TargetConditions: target.getConditionState(event.At),
            PetId: "",
        };

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);
    }

    public override clear(): void {
        super.clear();
    }

    /** 從 Worker snapshot 直接還原內部狀態 */
    public hydrateFrom(s: {
        name: string;
        raceId: number;
        body: { Height: number; Weight: number; Upper: number; Lower: number };
        totalTakeDamage: number;
        takeDamages: EntityDamage[];
    }): void {
        this._name = s.name;
        this._raceId = s.raceId;
        this._body = { ...s.body };
        this._totalTakeDamage = s.totalTakeDamage;
        this._takeDamages = s.takeDamages;
    }
}

export type EntityDamage = {
    Id: string;
    At: number;
    AtMs?: number;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
    Conditions: EntityCondition[];
    TargetConditions: EntityCondition[];
    PetId: string; // 펫 공격일 경우
};

export type EntityCondition = {
    Id: string;
    At: number;
    CCId: number;
    DisableAt: number;
    DisableAtMs?: number;
    AttackerId: string;
    Metadata?: string;
    DurationMs?: number;
};

export type EntityConditionState = {
    At: number;
    List: EntityCondition[];
};

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

export type EntityBody = {
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
};
