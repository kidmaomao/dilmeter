// Worker 訊息協議 - 無 Vue、無外部依賴

export type SnapshotBody = {
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
};

export type SnapshotCondition = {
    Id: string;
    At: number;
    CCId: number;
    DisableAt: number;
    DisableAtMs?: number;
    AttackerId: string;
    Metadata?: string;
    DurationMs?: number;
};

export type SnapshotConditionState = {
    At: number;
    List: SnapshotCondition[];
};

export type SnapshotItem = {
    PocketType: number;
    ItemId: number;
    Color1: string;
    Color2: string;
    Color3: string;
    Color5: string;
    Color6: string;
    Color7: string;
};

export type SnapshotDamage = {
    Id: string;
    At: number;
    AtMs?: number;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    IsDelayed: boolean;
    Conditions: SnapshotCondition[];
    TargetConditions: SnapshotCondition[];
    PetId: string;
};

export type SnapshotHealthLoss = {
    Id: string;
    At: number;
    Damage: number;
};

export type SnapshotEntity = {
    id: string;
    raceId: number;
    name: string;
    guildName: string;
    ownerId: string;
    finisherId: string;
    body: SnapshotBody;
    totalTakeDamage: number;
    takeDamages: SnapshotDamage[];
    totalApplyDamage: number;
    applyDamages: SnapshotDamage[];
    conditionMap: Record<number, SnapshotCondition>;
    conditionHistory: SnapshotConditionState[];
    equipItemMap: Record<number, SnapshotItem>;
    statMap: Record<number, number>;
    appearedAt?: number;
    groupKey: string;
};

export type SnapshotGroup = {
    id: string;
    raceId: number;
    name: string;
    body: SnapshotBody;
    totalTakeDamage: number;
    takeDamages: SnapshotDamage[];
};

export type WorkerSnapshot = {
    entities: Record<string, SnapshotEntity>;
    groups: Record<string, SnapshotGroup>;
    damages: any[]; // protocols.eventDamage[]
    /** Server-confirmed skill actions; absent in legacy records. */
    skillActions?: any[]; // protocols.eventSkillAction[]
    /** Health-bar reconciled damage; absent in legacy records. */
    effectiveDamages?: any[]; // protocols.eventDamage[]
    /** Authoritative body-health decreases; absent in legacy records. */
    healthLosses?: SnapshotHealthLoss[];
    collectorDamages: SnapshotDamage[]; // dcManager._damages
};

export type WorkerInMessage = {
    type: "process";
    ndjson: string;
};

export type WorkerOutMessage =
    | { type: "progress"; pct: number; phase: "parse" | "process" }
    | { type: "done"; snapshot: WorkerSnapshot }
    | { type: "error"; message: string };
