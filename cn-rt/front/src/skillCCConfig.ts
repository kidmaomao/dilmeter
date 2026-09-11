// ======================================================================
//  職業技能 CC 分析 — 設定模型 & localStorage 持久化
// ======================================================================

/**
 * 單一技能 CC 規則
 *  - skillIds 為空 → 對全部（非被動）命中生效
 *  - selfCCIds / targetCCIds 均使用 AND 邏輯：全部都必須同時存在
 */
export type SkillCCRule = {
    ruleId: string;
    label: string;
    /** 空陣列 = 全部非被動技能；否則只計這些 SkillId 的命中 */
    skillIds: number[];
    /** 自身 Conditions（Buff on self）中必須全部存在的 CCId */
    selfCCIds: number[];
    /** 目標 TargetConditions（Debuff on target）中必須全部存在的 CCId */
    targetCCIds: number[];
    /**
     * 是否在結果表格中顯示為獨立欄位；undefined 視為 true（向下相容）
     */
    showInTable?: boolean;
};

/** 單一職業設定 */
export type JobCCConfig = {
    id: string;
    name: string;
    /** 59xxx 職業專主技能 ID，用於自動偵測職業 */
    detectionSkillIds: number[];
    rules: SkillCCRule[];
};

// ── 預設職業設定（從 dashboard.vue 遷移，改為 AND 邏輯） ─────────────

export const DEFAULT_JOB_CC_CONFIGS: JobCCConfig[] = [
    {
        id: "archer",
        name: "流星射手",
        detectionSkillIds: [59060, 59061, 59064],
        rules: [
            {
                ruleId: "archer-all",
                label: "全部技能",
                skillIds: [21002, 59060, 59061],
                selfCCIds: [887],
                targetCCIds: [],
                showInTable: true,
            },
            {
                ruleId: "archer-1776577558547",
                label: "魔法陣",
                skillIds: [21002, 59061],
                selfCCIds: [],
                targetCCIds: [],
                showInTable: true,
            },
        ],
    },
    {
        id: "magicSword",
        name: "元素骑士",
        detectionSkillIds: [59023, 59024, 59025, 59026, 59028],
        rules: [
            {
                ruleId: "magicSword-all",
                label: "憤衝覆蓋率",
                skillIds: [59024, 59025, 59026, 59028, 20019, 20002],
                selfCCIds: [],
                targetCCIds: [323],
                showInTable: true,
            },
            {
                ruleId: "magicSword-1776577799470",
                label: "魔法陣",
                skillIds: [20002, 59026, 20019, 59028],
                selfCCIds: [10015, 10020],
                targetCCIds: [],
                showInTable: true,
            },
            {
                ruleId: "magicSword-1776577856080",
                label: "憤衝",
                skillIds: [20018],
                selfCCIds: [889, 10019],
                targetCCIds: [],
                showInTable: true,
            },
        ],
    },
    {
        id: "darkMage",
        name: "黑魔导士",
        detectionSkillIds: [59040, 59041, 59042],
        rules: [
            {
                ruleId: "darkMage-all",
                label: "魔力穿刺",
                skillIds: [30202, 30102, 30452, 59042, 59040, 59041],
                selfCCIds: [938],
                targetCCIds: [],
                showInTable: true,
            },
            {
                ruleId: "darkMage-1776578048363",
                label: "雷鏈",
                skillIds: [59040, 30452],
                selfCCIds: [],
                targetCCIds: [948],
                showInTable: true,
            },
        ],
    },
    {
        id: "lancer",
        name: "爆裂骑士枪",
        detectionSkillIds: [59100, 59101, 59102, 59103, 59104, 59105, 59106],
        rules: [
            {
                ruleId: "lancer-all",
                label: "騎乘效果",
                skillIds: [59101, 59102, 59103, 59104, 59105, 59106, 20017, 59100],
                selfCCIds: [975],
                targetCCIds: [],
                showInTable: true,
            },
            {
                ruleId: "lancer-1776578133219",
                label: "憤衝",
                skillIds: [20017, 59100, 59101, 59102, 59103, 59104, 59105, 59106],
                selfCCIds: [],
                targetCCIds: [323],
                showInTable: true,
            },
            {
                ruleId: "lancer-1776578268456",
                label: "魔法陣",
                skillIds: [20017, 59101, 59105],
                selfCCIds: [10093],
                targetCCIds: [],
                showInTable: true,
            },
        ],
    },
    {
        id: "shieldWarrior",
        name: "圣盾骑士",
        detectionSkillIds: [59080, 59081, 59082, 59083, 59084, 59085, 59086],
        rules: [
            {
                ruleId: "shieldWarrior-all",
                label: "全部技能",
                skillIds: [59080, 59081, 59082, 59083, 59084, 59085, 59086],
                selfCCIds: [],
                targetCCIds: [323],
                showInTable: true,
            },
        ],
    },
    {
        id: "bard",
        name: "圣光颂唱者",
        detectionSkillIds: [59004],
        rules: [],
    },
    {
        id: "gun",
        name: "枪炮师",
        detectionSkillIds: [59120, 59121, 59122, 59123, 59124],
        rules: [
            {
                ruleId: "gun-all",
                label: "全部技能",
                skillIds: [59120, 59121, 59122, 59123, 54303, 54305, 54306, 54307],
                selfCCIds: [1123],
                targetCCIds: [1122],
                showInTable: true,
            },
        ],
    },
    {
        id: "alchemist",
        name: "禁术炼金师",
        detectionSkillIds: [59143, 59144, 59145],
        rules: [
            {
                ruleId: "alchemist-all",
                label: "全部技能",
                skillIds: [35014, 35004, 59143, 59144, 59145],
                selfCCIds: [1146],
                targetCCIds: [1147],
                showInTable: true,
            },
        ],
    },
];

// ── 深複製工具 ───────────────────────────────────────────────────────

export function deepCloneConfigs(configs: JobCCConfig[]): JobCCConfig[] {
    return configs.map((c) => ({
        ...c,
        detectionSkillIds: [...c.detectionSkillIds],
        rules: c.rules.map((r) => ({
            ...r,
            skillIds: [...r.skillIds],
            selfCCIds: [...r.selfCCIds],
            targetCCIds: [...r.targetCCIds],
        })),
    }));
}

// ── localStorage 持久化 ───────────────────────────────────────────────

const LS_KEY = "skill-cc-config-v1";

export function loadJobCCConfigs(): JobCCConfig[] {
    try {
        const raw = localStorage.getItem(LS_KEY);
        if (raw) {
            const parsed = JSON.parse(raw) as JobCCConfig[];
            if (Array.isArray(parsed) && parsed.length > 0) return parsed;
        }
    } catch {
        /* ignore */
    }
    return deepCloneConfigs(DEFAULT_JOB_CC_CONFIGS);
}

export function saveJobCCConfigs(configs: JobCCConfig[]): void {
    localStorage.setItem(LS_KEY, JSON.stringify(configs));
}

export function resetToDefaultConfigs(): JobCCConfig[] {
    return deepCloneConfigs(DEFAULT_JOB_CC_CONFIGS);
}
