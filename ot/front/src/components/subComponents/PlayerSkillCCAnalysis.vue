<template>
    <v-card variant="outlined">
        <!-- ── 標題列 ── -->
        <v-card-title class="text-subtitle-1 py-2 px-3 d-flex align-center">
            玩家技能 CC 分析
            <v-spacer />
            <v-btn
                v-if="!collapsed"
                icon
                size="x-small"
                variant="text"
                class="mr-1"
                @click="settingsOpen = true"
            >
                <v-icon size="small" class="text-disabled">mdi-cog-outline</v-icon>
            </v-btn>
            <v-btn
                icon
                size="x-small"
                variant="text"
                @click="collapsed = !collapsed"
            >
                <v-icon size="small" class="text-disabled">
                    {{ collapsed ? 'mdi-chevron-down' : 'mdi-chevron-up' }}
                </v-icon>
            </v-btn>
        </v-card-title>

        <!-- ── 主體 ── -->
        <v-expand-transition>
            <div v-if="!collapsed">
                <v-divider />
                <v-card-text class="pa-3">

                    <!-- 顯示次數切換 -->
                    <div class="d-flex justify-end mb-2">
                        <v-btn-toggle
                            v-model="showCounts"
                            density="compact"
                            variant="outlined"
                            mandatory
                        >
                            <v-btn :value="false" size="x-small">僅 %</v-btn>
                            <v-btn :value="true"  size="x-small">顯示次數</v-btn>
                        </v-btn-toggle>
                    </div>

                    <!-- 無資料 -->
                    <div
                        v-if="playerTableData.length === 0"
                        class="text-center text-disabled text-body-2 py-4"
                    >
                        無資料
                    </div>

                    <!-- 各玩家區塊 -->
                    <template v-else>
                        <div
                            v-for="(ptd, pi) in playerTableData"
                            :key="ptd.entityId"
                            :class="{ 'mt-5': pi > 0 }"
                        >
                            <!-- 玩家名稱 + 職業 chip -->
                            <div class="d-flex align-center mb-1">
                                <span class="text-body-2 font-weight-bold">{{ ptd.displayName }}</span>
                                <v-chip
                                    size="x-small"
                                    class="ml-2"
                                    :color="ptd.jobConfig ? 'primary' : undefined"
                                    variant="tonal"
                                >
                                    {{ ptd.jobConfig?.name ?? '未偵測' }}
                                </v-chip>
                            </div>

                            <!-- 無職業 -->
                            <div v-if="!ptd.jobConfig" class="text-caption text-disabled">
                                無法偵測到職業（未使用職業專主技能）
                            </div>

                            <!-- 無可顯示的 rule -->
                            <div v-else-if="ptd.visibleRules.length === 0" class="text-caption text-disabled">
                                此職業無可顯示的規則
                                <v-btn size="x-small" variant="text" class="ml-1" @click="settingsOpen = true">
                                    前往設定
                                </v-btn>
                            </div>

                            <!-- 技能 × rule 矩陣表格 -->
                            <div v-else class="cc-table-wrap">
                                <table class="cc-matrix-table">
                                    <thead>
                                        <tr>
                                            <th class="col-skill">技能</th>
                                            <th
                                                v-for="rule in ptd.visibleRules"
                                                :key="rule.ruleId"
                                                class="col-rule"
                                            >
                                                {{ rule.label }}
                                            </th>
                                        </tr>
                                    </thead>
                                    <tbody>
                                        <!-- 技能列（按總命中數降序） -->
                                        <tr
                                            v-for="row in ptd.skillRows"
                                            :key="row.skillId"
                                        >
                                            <td class="col-skill skill-name">{{ row.skillName }}</td>
                                            <td
                                                v-for="(cell, ci) in row.cells"
                                                :key="ci"
                                                class="col-rule cell-data"
                                            >
                                                <template v-if="cell">
                                                    <span
                                                        class="cell-pct"
                                                        :style="{ color: pctColor(cell.pct) }"
                                                    >
                                                        {{ cell.pct != null ? cell.pct.toFixed(1) + '%' : '—' }}
                                                    </span>
                                                    <span v-if="showCounts" class="cell-count">
                                                        ({{ cell.withCC }}/{{ cell.total }})
                                                    </span>
                                                </template>
                                                <span v-else class="text-disabled">—</span>
                                            </td>
                                        </tr>

                                        <!-- 合計列 -->
                                        <tr class="overall-row">
                                            <td class="col-skill skill-name font-weight-bold">合計</td>
                                            <td
                                                v-for="(cell, ci) in ptd.overallCells"
                                                :key="ci"
                                                class="col-rule cell-data"
                                            >
                                                <template v-if="cell">
                                                    <span
                                                        class="cell-pct font-weight-bold"
                                                        :style="{ color: pctColor(cell.pct) }"
                                                    >
                                                        {{ cell.pct != null ? cell.pct.toFixed(1) + '%' : '—' }}
                                                    </span>
                                                    <span v-if="showCounts" class="cell-count">
                                                        ({{ cell.withCC }}/{{ cell.total }})
                                                    </span>
                                                </template>
                                                <span v-else class="text-disabled">—</span>
                                            </td>
                                        </tr>
                                    </tbody>
                                </table>
                            </div>
                        </div>
                    </template>

                </v-card-text>
            </div>
        </v-expand-transition>

        <!-- ════════════════════════════════════════
             設定 Dialog
        ════════════════════════════════════════ -->
        <v-dialog v-model="settingsOpen" max-width="720" scrollable>
            <v-card>
                <v-card-title class="text-subtitle-1 d-flex align-center py-3 px-4">
                    規則設定
                    <v-spacer />
                    <v-btn size="x-small" variant="outlined" @click="onResetDefaults">
                        重置為預設
                    </v-btn>
                </v-card-title>

                <!-- 職業 Tabs -->
                <v-tabs v-model="settingsJobTab" density="compact" show-arrows>
                    <v-tab
                        v-for="cfg in configs"
                        :key="cfg.id"
                        :value="cfg.id"
                        size="small"
                    >
                        {{ cfg.name }}
                    </v-tab>
                </v-tabs>

                <v-divider />

                <v-card-text style="min-height: 340px; max-height: 60vh;">
                    <v-window v-model="settingsJobTab">
                        <v-window-item
                            v-for="cfg in configs"
                            :key="cfg.id"
                            :value="cfg.id"
                        >
                            <div class="text-caption text-disabled mb-3 mt-1">
                                偵測技能：{{ cfg.detectionSkillIds.join(', ') }}
                            </div>

                            <div
                                v-if="cfg.rules.length === 0"
                                class="text-body-2 text-disabled text-center py-4"
                            >
                                此職業未設定任何規則
                            </div>

                            <!-- 規則卡片 -->
                            <div
                                v-for="rule in cfg.rules"
                                :key="rule.ruleId"
                                class="rule-card pa-3 mb-3 rounded"
                            >
                                <!-- 名稱 + 顯示欄開關 + 刪除 -->
                                <div class="d-flex align-center mb-3" style="gap: 8px;">
                                    <v-text-field
                                        v-if="editData[rule.ruleId]"
                                        v-model="editData[rule.ruleId].label"
                                        label="規則名稱"
                                        density="compact"
                                        hide-details
                                        variant="outlined"
                                        class="flex-grow-1"
                                    />
                                    <v-tooltip text="顯示為結果表格的欄位" location="top">
                                        <template #activator="{ props: tip }">
                                            <v-btn
                                                v-if="editData[rule.ruleId]"
                                                v-bind="tip"
                                                :color="editData[rule.ruleId].showInTable ? 'primary' : undefined"
                                                :variant="editData[rule.ruleId].showInTable ? 'tonal' : 'outlined'"
                                                size="small"
                                                @click="editData[rule.ruleId].showInTable = !editData[rule.ruleId].showInTable"
                                            >
                                                <v-icon size="small">mdi-table-column</v-icon>
                                                <span class="ml-1 text-caption">
                                                    {{ editData[rule.ruleId].showInTable ? '顯示' : '隱藏' }}
                                                </span>
                                            </v-btn>
                                        </template>
                                    </v-tooltip>
                                    <v-btn
                                        icon
                                        size="small"
                                        variant="text"
                                        color="error"
                                        @click="deleteRule(cfg.id, rule.ruleId)"
                                    >
                                        <v-icon size="small">mdi-delete-outline</v-icon>
                                    </v-btn>
                                </div>

                                <template v-if="editData[rule.ruleId]">
                                    <!-- 技能多選 -->
                                    <v-autocomplete
                                        v-model="editData[rule.ruleId].skillIds"
                                        :items="skillSelectItems"
                                        item-title="title"
                                        item-value="value"
                                        multiple
                                        chips
                                        closable-chips
                                        clearable
                                        label="技能（空 = 全部非被動技能）"
                                        density="compact"
                                        hide-details
                                        variant="outlined"
                                        class="mb-3"
                                    />
                                    <!-- CC 多選 -->
                                    <v-row dense>
                                        <v-col cols="6">
                                            <v-autocomplete
                                                v-model="editData[rule.ruleId].selfCCIds"
                                                :items="ccSelectItems"
                                                item-title="title"
                                                item-value="value"
                                                multiple
                                                chips
                                                closable-chips
                                                clearable
                                                label="自身 Buff CC"
                                                density="compact"
                                                hide-details
                                                variant="outlined"
                                            />
                                        </v-col>
                                        <v-col cols="6">
                                            <v-autocomplete
                                                v-model="editData[rule.ruleId].targetCCIds"
                                                :items="ccSelectItems"
                                                item-title="title"
                                                item-value="value"
                                                multiple
                                                chips
                                                closable-chips
                                                clearable
                                                label="目標 Debuff CC"
                                                density="compact"
                                                hide-details
                                                variant="outlined"
                                            />
                                        </v-col>
                                    </v-row>
                                </template>
                            </div>

                            <v-btn
                                size="small"
                                variant="outlined"
                                prepend-icon="mdi-plus"
                                @click="addRule(cfg.id)"
                            >
                                新增規則
                            </v-btn>
                        </v-window-item>
                    </v-window>
                </v-card-text>

                <v-divider />
                <v-card-actions class="px-4 py-2">
                    <span class="text-caption text-disabled">AND 邏輯：自身與目標的 CC 必須全部同時存在</span>
                    <v-spacer />
                    <v-btn variant="text" @click="settingsOpen = false">取消</v-btn>
                    <v-btn color="primary" variant="tonal" @click="saveSettings">儲存</v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>
    </v-card>
</template>

<script lang="ts">
import {
    defineComponent,
    inject,
    computed,
    ref,
    watch,
    type Ref,
    type PropType,
} from "vue";
import { ActorManager, EntityActor, type EntityDamage } from "@/eventActor";
import { type BossSummary } from "@/summaryCollector";
import { getDisplayName, condNameMap, skillNameMap } from "@/store";
import {
    type SkillCCRule,
    type JobCCConfig,
    loadJobCCConfigs,
    saveJobCCConfigs,
    resetToDefaultConfigs,
} from "@/skillCCConfig";

// 被動技能（不計入統計）
const PASSIVE_SKILL_IDS = new Set([58100, 58101, 58104, 58009]);

// ── 工具 ───────────────────────────────────────────────────────

function checkCC(damage: EntityDamage, rule: SkillCCRule): boolean {
    if (rule.selfCCIds.length === 0 && rule.targetCCIds.length === 0) return true;
    const selfSet   = new Set(damage.Conditions.map((c) => c.CCId));
    const targetSet = new Set(damage.TargetConditions.map((c) => c.CCId));
    return (
        rule.selfCCIds.every((id) => selfSet.has(id)) &&
        rule.targetCCIds.every((id) => targetSet.has(id))
    );
}

function pctColor(pct: number | null): string {
    if (pct === null) return "";
    if (pct >= 90) return "#4CAF50";
    if (pct >= 70) return "#FF9800";
    return "#F44336";
}

// ── 型別 ───────────────────────────────────────────────────────

type CellData = { withCC: number; total: number; pct: number | null };

type SkillRowData = {
    skillId: number;
    skillName: string;
    totalHits: number;
    cells: (CellData | null)[];   // 對應 visibleRules，null = 不在 scope
};

type PlayerTableData = {
    entityId: string;
    displayName: string;
    jobConfig: JobCCConfig | null;
    visibleRules: SkillCCRule[];
    skillRows: SkillRowData[];
    overallCells: (CellData | null)[];
};

type EditEntry = {
    label: string;
    skillIds: number[];
    selfCCIds: number[];
    targetCCIds: number[];
    showInTable: boolean;
};

export default defineComponent({
    name: "PlayerSkillCCAnalysis",
    props: {
        summary:       { type: Object as PropType<BossSummary>, default: null },
        bossEntityKey: { type: String, default: "" },
    },
    setup(props) {
        const actorManager = inject("actorManager") as Ref<ActorManager>;

        const collapsed      = ref(false);
        const showCounts     = ref(false);
        const settingsOpen   = ref(false);
        const settingsJobTab = ref("archer");

        // ── 設定狀態 ─────────────────────────────────────────────
        const configs  = ref<JobCCConfig[]>(loadJobCCConfigs());
        const editData = ref<Record<string, EditEntry>>({});

        const ccSelectItems = computed(() =>
            Object.entries(condNameMap.value)
                .map(([id, name]) => ({ title: `${name}（${id}）`, value: Number(id) }))
                .sort((a, b) => a.value - b.value),
        );
        const skillSelectItems = computed(() =>
            Object.entries(skillNameMap.value)
                .map(([id, name]) => ({ title: `${name}（${id}）`, value: Number(id) }))
                .sort((a, b) => a.value - b.value),
        );

        // ── 編輯狀態 ─────────────────────────────────────────────

        function buildEditData() {
            const map: Record<string, EditEntry> = {};
            for (const cfg of configs.value) {
                for (const rule of cfg.rules) {
                    map[rule.ruleId] = {
                        label:       rule.label,
                        skillIds:    [...rule.skillIds],
                        selfCCIds:   [...rule.selfCCIds],
                        targetCCIds: [...rule.targetCCIds],
                        showInTable: rule.showInTable !== false,
                    };
                }
            }
            editData.value = map;
        }

        watch(settingsOpen, (open) => { if (open) buildEditData(); });

        function addRule(jobId: string) {
            const job = configs.value.find((c) => c.id === jobId);
            if (!job) return;
            const ruleId = `${jobId}-${Date.now()}`;
            job.rules.push({
                ruleId, label: "新規則",
                skillIds: [], selfCCIds: [], targetCCIds: [],
                showInTable: true,
            });
            editData.value = {
                ...editData.value,
                [ruleId]: { label: "新規則", skillIds: [], selfCCIds: [], targetCCIds: [], showInTable: true },
            };
        }

        function deleteRule(jobId: string, ruleId: string) {
            const job = configs.value.find((c) => c.id === jobId);
            if (!job) return;
            job.rules = job.rules.filter((r) => r.ruleId !== ruleId);
            const next = { ...editData.value };
            delete next[ruleId];
            editData.value = next;
        }

        function applyEditData() {
            for (const cfg of configs.value) {
                for (const rule of cfg.rules) {
                    const e = editData.value[rule.ruleId];
                    if (!e) continue;
                    rule.label       = e.label.trim() || rule.label;
                    rule.skillIds    = [...e.skillIds];
                    rule.selfCCIds   = [...e.selfCCIds];
                    rule.targetCCIds = [...e.targetCCIds];
                    rule.showInTable = e.showInTable;
                }
            }
        }

        function saveSettings() {
            applyEditData();
            saveJobCCConfigs(configs.value);
            settingsOpen.value = false;
        }

        function onResetDefaults() {
            configs.value = resetToDefaultConfigs();
            saveJobCCConfigs(configs.value);
            buildEditData();
        }

        // ── 職業偵測 ─────────────────────────────────────────────

        function detectJobConfig(usedSkillIds: Set<number>): JobCCConfig | null {
            for (const cfg of configs.value) {
                if (cfg.detectionSkillIds.some((id) => usedSkillIds.has(id))) return cfg;
            }
            let best: JobCCConfig | null = null;
            let bestScore = 0;
            for (const cfg of configs.value) {
                const score = cfg.detectionSkillIds.filter((id) => usedSkillIds.has(id)).length;
                if (score > bestScore) { bestScore = score; best = cfg; }
            }
            return bestScore > 0 ? best : null;
        }

        // ── 計算單格資料 ─────────────────────────────────────────

        function makeCell(hits: EntityDamage[], rule: SkillCCRule): CellData | null {
            if (hits.length === 0) return null;
            const withCC = hits.filter((d) => checkCC(d, rule)).length;
            const total  = hits.length;
            return { withCC, total, pct: (withCC / total) * 100 };
        }

        // ── 主計算：矩陣表格資料 ─────────────────────────────────

        const playerTableData = computed((): PlayerTableData[] => {
            if (!props.summary || !props.bossEntityKey) return [];
            const sess = props.summary.session;

            return props.summary.players
                .map((p) => {
                    const entity = actorManager.value.entityMap[p.entityId] as EntityActor | undefined;
                    if (!entity) return null;

                    const hits = entity.applyDamages.filter(
                        (d) =>
                            d.TargetId === props.bossEntityKey &&
                            d.Damage > 0 &&
                            d.At >= sess.startAt &&
                            d.At <= sess.endAt,
                    );
                    if (hits.length === 0) return null;

                    const usedSkillIds = new Set(hits.map((d) => d.SkillId));
                    const jobConfig    = detectJobConfig(usedSkillIds);

                    // showInTable !== false 的規則才顯示為欄
                    const visibleRules = (jobConfig?.rules ?? []).filter(
                        (r) => r.showInTable !== false,
                    );

                    // 按技能分組（非被動）
                    const skillHitsMap = new Map<number, EntityDamage[]>();
                    for (const hit of hits) {
                        if (PASSIVE_SKILL_IDS.has(hit.SkillId)) continue;
                        if (!skillHitsMap.has(hit.SkillId)) skillHitsMap.set(hit.SkillId, []);
                        skillHitsMap.get(hit.SkillId)!.push(hit);
                    }

                    // 建立技能列
                    const skillRows: SkillRowData[] = [];
                    for (const [skillId, skillHits] of skillHitsMap) {
                        const cells: (CellData | null)[] = visibleRules.map((rule) => {
                            const inScope =
                                rule.skillIds.length === 0 ||
                                rule.skillIds.includes(skillId);
                            return inScope ? makeCell(skillHits, rule) : null;
                        });

                        // 至少有一欄有資料才顯示
                        if (cells.some((c) => c !== null)) {
                            skillRows.push({
                                skillId,
                                skillName: skillNameMap.value[skillId] ?? String(skillId),
                                totalHits: skillHits.length,
                                cells,
                            });
                        }
                    }

                    // 按總命中數降序排列
                    skillRows.sort((a, b) => b.totalHits - a.totalHits);

                    // 合計列：每個 rule 聚合所有在 scope 的命中
                    const overallCells: (CellData | null)[] = visibleRules.map((rule) => {
                        const ruleHits =
                            rule.skillIds.length > 0
                                ? hits.filter((d) => rule.skillIds.includes(d.SkillId))
                                : hits.filter((d) => !PASSIVE_SKILL_IDS.has(d.SkillId));
                        return makeCell(ruleHits, rule);
                    });

                    return {
                        entityId:     p.entityId,
                        displayName:  getDisplayName(p.name),
                        jobConfig,
                        visibleRules,
                        skillRows,
                        overallCells,
                    };
                })
                .filter((v): v is PlayerTableData => v !== null);
        });

        return {
            collapsed,
            showCounts,
            settingsOpen,
            settingsJobTab,
            configs,
            editData,
            ccSelectItems,
            skillSelectItems,
            playerTableData,
            addRule,
            deleteRule,
            saveSettings,
            onResetDefaults,
            pctColor,
        };
    },
});
</script>

<style scoped>
/* ── 矩陣表格 ── */
.cc-table-wrap {
    overflow-x: auto;
    width: 100%;
}

.cc-matrix-table {
    width: 100%;
    border-collapse: collapse;
    font-size: 0.8rem;
    white-space: nowrap;
}

.cc-matrix-table th,
.cc-matrix-table td {
    padding: 4px 10px;
    border-bottom: 1px solid rgba(128, 128, 128, 0.12);
}

.cc-matrix-table th {
    font-size: 0.72rem;
    font-weight: 600;
    text-align: center;
    background: rgba(128, 128, 128, 0.06);
}

.col-skill {
    text-align: left;
    min-width: 100px;
}

.col-rule {
    text-align: center;
    min-width: 90px;
}

.skill-name {
    font-size: 0.78rem;
}

.cell-data {
    font-variant-numeric: tabular-nums;
}

.cell-pct {
    font-weight: 500;
}

.cell-count {
    font-size: 0.68rem;
    opacity: 0.6;
    margin-left: 2px;
}

.overall-row td {
    border-top: 2px solid rgba(128, 128, 128, 0.25);
    background: rgba(128, 128, 128, 0.04);
}

/* ── 規則卡片（設定 dialog） ── */
.rule-card {
    border: 1px solid rgba(128, 128, 128, 0.2);
    background: rgba(128, 128, 128, 0.04);
}
</style>
