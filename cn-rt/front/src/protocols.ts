export type eventId = number;

export const eventIdEntityAppear = 1;
export const eventIdEntityDisappear = 2;
export const eventIdDamage = 3;
export const eventIdCharacterConditionEnable = 4;
export const eventIdCharacterConditionDisable = 5;
export const eventIdFinish = 6;
export const eventIdEntityEquipItem = 7;
export const eventIdEntityUnequipItem = 8;
export const eventIdEntityUpdateBody = 9;
export const eventIdSkillAction = 10;
export const eventIdLocalEntity = 11;
export const eventIdStatUpdate = 17;
export const eventIdSkillState = 18;
export const eventIdCombatTarget = 19;
export const eventIdSkillCooldown = 20;

export const eventIdMessageBox = -1;

export type eventBase = {
    EventId: eventId;
    At: number;
    Id: string;
    Sequence?: number;
}

export type eventEntityAppear = eventBase & {
    EventId: 1;
    Name: string;
    RaceId: number;
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
    GuildName: string;
    OwnerId: string;
}

export type eventEntityDisappear = eventBase & {
    EventId: 2;
}

export type eventDamage = eventBase & {
    EventId: 3;
    /** Exact packet time for grouping repeated mechanism feedback packets. */
    AtMs?: number;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
}

export type eventCharacterConditionEnable = eventBase & {
    EventId: 4;
    CCId: number;
    DisableAt: number;
    /** Exact expiry for new captures; absent in older logs. */
    DisableAtMs?: number;
    AttackerId: string;
    Metadata?: string;
    DurationMs?: number;
}

export type eventCharacterConditionDisable = eventBase & {
    EventId: 5;
    CCId: number;
}

export type eventFinish = eventBase & {
    EventId: 6;
    AttackerId: string;
}

export type eventEntityEquipItem = eventBase & {
    EventId: 7;
    PocketType: number;
    ItemId: number;
    Color1: string;
    Color2: string;
    Color3: string;
    Color5: string;
    Color6: string;
    Color7: string;
}

export type eventEntityUnequipItem = eventBase & {
    EventId: 8;
    PocketType: number;
    ItemId: number;
}

export type eventEntityUpdateBody = eventBase & {
    EventId: 9;
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
}

export type eventSkillAction = eventBase & {
    EventId: 10;
    SkillId: number;
    SubSkillId: number;
    CombatActionId: number;
    AtMs: number;
    /** Dedicated derived mechanic signal; normal skill actions omit it. */
    MechanicSignal?: string;
    /** Entity that produced the action; owned marionettes/deployables differ from Id. */
    SourceId?: string;
    /** Damage-derived actions may start an idle timer but never restart a running one. */
    IsFallback?: boolean;
    /** False for monster actions used by Boss mechanic reminders. */
    IsLocal?: boolean;
}

export type eventLocalEntity = eventBase & {
    EventId: 11;
    Reliable: boolean;
    /** True when capture moved to a new game-server connection/channel. */
    Reset?: boolean;
}

export type eventStatUpdateEntry = {
    StatId: number;
    Value: number;
}

export type eventStatUpdate = eventBase & {
    EventId: 17;
    Private: boolean;
    Stats: eventStatUpdateEntry[];
}

export type eventSkillState = eventBase & {
    EventId: 18;
    AtMs: number;
    SkillId: number;
    Scope: "aim" | "active";
    Active: boolean;
    TargetId?: string;
}

export type eventCombatTarget = eventBase & {
    EventId: 19;
    AtMs: number;
    TargetId?: string;
}

export type eventSkillCooldown = eventBase & {
    EventId: 20;
    AtMs: number;
    SkillId: number;
    Reset: boolean;
    ReduceMs: number;
    Signal?: "27049" | "27069" | "SLST" | string;
}

export type eventMessageBox = eventBase & {
    EventId: -1;
    Message: string;
}
