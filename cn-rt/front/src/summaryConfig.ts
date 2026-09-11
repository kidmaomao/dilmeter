import { ref } from "vue";

// ===== 型別定義 =====

/**
 * 條件篩選模式
 *
 * onHit_targetHas      — 傷害發生時，目標身上有指定 condition
 * onHit_attackerHas    — 傷害發生時，攻擊者身上有指定 condition（吃 debuff 的職業用）
 * duringCondition_target   — 指定 condition 在目標身上存在的期間內所有傷害
 * duringCondition_attacker — 指定 condition 在攻擊者身上存在的期間內所有傷害
 */
export type ConditionFilterMode =
    | "onHit_targetHas"
    | "onHit_attackerHas"
    | "duringCondition_target"
    | "duringCondition_attacker";

/** 單一條件規則（葉節點） */
export type ConditionFilterLeaf = {
    type: "leaf";
    mode: ConditionFilterMode;
    conditionId: number;
};

/** 組合規則（AND / OR，可巢狀） */
export type ConditionFilterGroup = {
    type: "and" | "or";
    rules: ConditionFilter[];
};

export type ConditionFilter = ConditionFilterLeaf | ConditionFilterGroup;

/**
 * 分組方式
 *
 * eachEntry — 每次進出條件算一個 group（例如崩壞每段分開統計）
 * none      — 全部合計，不分組
 */
export type GroupByMode = "eachEntry" | "none";

/** 自訂條件設定 */
export type CustomConditionConfig = {
    /** 唯一識別碼 */
    id: string;
    /** 顯示名稱，例如「崩壞期間爆發」 */
    displayName: string;
    /** 條件篩選規則 */
    filter: ConditionFilter;
    /** 結果分組方式 */
    groupBy: GroupByMode;
};

// ===== 預設自訂條件 =====

const DEFAULT_CUSTOM_CONDITION_CONFIGS: CustomConditionConfig[] = [
    {
        id: "cc_1776579683788_bd55k",
        displayName: "崩",
        filter: { type: "leaf", mode: "onHit_targetHas", conditionId: 803 },
        groupBy: "eachEntry",
    },
];

// ===== localStorage 存取 =====

const STORAGE_KEY = "summaryConditionConfigs";

function loadFromStorage(): CustomConditionConfig[] {
    try {
        const raw = localStorage.getItem(STORAGE_KEY);
        if (raw) return JSON.parse(raw) as CustomConditionConfig[];
    } catch {
        /* ignore */
    }
    return DEFAULT_CUSTOM_CONDITION_CONFIGS.map((c) => ({ ...c }));
}

function saveToStorage(configs: CustomConditionConfig[]) {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(configs));
}

// ===== 響應式狀態 =====

export const customConditionConfigs = ref<CustomConditionConfig[]>(
    loadFromStorage(),
);

// ===== CRUD =====

/** 新增自訂條件（自動產生 id） */
export function addCustomCondition(
    config: Omit<CustomConditionConfig, "id">,
): CustomConditionConfig {
    const newConfig: CustomConditionConfig = {
        ...config,
        id: `cc_${Date.now()}_${Math.random().toString(36).slice(2, 7)}`,
    };
    customConditionConfigs.value = [...customConditionConfigs.value, newConfig];
    saveToStorage(customConditionConfigs.value);
    return newConfig;
}

/** 更新指定 id 的自訂條件 */
export function updateCustomCondition(
    id: string,
    updates: Partial<Omit<CustomConditionConfig, "id">>,
) {
    customConditionConfigs.value = customConditionConfigs.value.map((c) =>
        c.id === id ? { ...c, ...updates } : c,
    );
    saveToStorage(customConditionConfigs.value);
}

/** 刪除指定 id 的自訂條件 */
export function removeCustomCondition(id: string) {
    customConditionConfigs.value = customConditionConfigs.value.filter(
        (c) => c.id !== id,
    );
    saveToStorage(customConditionConfigs.value);
}

/** 調整順序（拖曳排序用） */
export function reorderCustomCondition(fromIndex: number, toIndex: number) {
    const list = [...customConditionConfigs.value];
    const [item] = list.splice(fromIndex, 1);
    list.splice(toIndex, 0, item);
    customConditionConfigs.value = list;
    saveToStorage(customConditionConfigs.value);
}

// ===== 匯出 / 匯入（分享用） =====

/** 將所有設定匯出為 JSON 字串 */
export function exportConfigs(): string {
    return JSON.stringify(customConditionConfigs.value, null, 2);
}

/** 從 JSON 字串匯入設定（覆蓋現有） */
export function importConfigs(json: string): boolean {
    try {
        const parsed = JSON.parse(json) as CustomConditionConfig[];
        if (!Array.isArray(parsed)) return false;
        customConditionConfigs.value = parsed;
        saveToStorage(parsed);
        return true;
    } catch {
        return false;
    }
}

// ===== 建立輔助函式 =====

/** 建立一個空的 leaf filter */
export function makeLeafFilter(
    mode: ConditionFilterMode = "onHit_targetHas",
    conditionId = 0,
): ConditionFilterLeaf {
    return { type: "leaf", mode, conditionId };
}

/** 建立一個 AND 組合 filter */
export function makeAndFilter(
    rules: ConditionFilter[] = [],
): ConditionFilterGroup {
    return { type: "and", rules };
}

/** 建立一個 OR 組合 filter */
export function makeOrFilter(
    rules: ConditionFilter[] = [],
): ConditionFilterGroup {
    return { type: "or", rules };
}

/** 建立一個預設空白的自訂條件（尚未加入 storage） */
export function makeEmptyConditionConfig(): Omit<CustomConditionConfig, "id"> {
    return {
        displayName: "",
        filter: makeLeafFilter(),
        groupBy: "eachEntry",
    };
}

// ===== 顯示輔助 =====

const MODE_LABEL: Record<ConditionFilterMode, string> = {
    onHit_targetHas: "傷害時目標有",
    onHit_attackerHas: "傷害時攻擊者有",
    duringCondition_target: "目標 condition 存在期間",
    duringCondition_attacker: "攻擊者 condition 存在期間",
};

export function getModeLabelTw(mode: ConditionFilterMode): string {
    return MODE_LABEL[mode];
}

/** 將 filter 轉為人類可讀的描述字串（用於 UI 預覽） */
export function describeFilter(
    filter: ConditionFilter,
    condNameMap: Record<number, string>,
): string {
    if (filter.type === "leaf") {
        const condName =
            condNameMap[filter.conditionId] ?? `#${filter.conditionId}`;
        return `${MODE_LABEL[filter.mode]}「${condName}」`;
    }

    const sep = filter.type === "and" ? " 且 " : " 或 ";
    const parts = filter.rules.map((r) => describeFilter(r, condNameMap));
    return parts.length === 1 ? parts[0] : `(${parts.join(sep)})`;
}
