<template>
    <div v-if="modelValue" class="record-browser-backdrop" @click.self="close">
        <section class="record-browser-dialog" role="dialog" aria-modal="true" aria-labelledby="record-browser-title">
            <header class="record-browser-titlebar">
                <button
                    v-if="view === 'detail'"
                    type="button"
                    class="record-back-button"
                    aria-label="返回记录列表"
                    @click="returnToList"
                >
                    <v-icon icon="mdi-arrow-left" size="17" />
                </button>
                <div>
                    <v-icon :icon="view === 'list' ? 'mdi-history' : 'mdi-chart-box-outline'" size="18" />
                    <span id="record-browser-title">{{ view === "list" ? "查看记录" : "DPS 记录" }}</span>
                    <small v-if="view === 'detail' && selectedRecord">{{ selectedRecord.date }} {{ selectedRecord.timeRange }}</small>
                </div>
                <button type="button" aria-label="关闭查看记录" @click="close">
                    <v-icon icon="mdi-close-box-outline" size="19" />
                </button>
            </header>

            <template v-if="view === 'list'">
                <div class="record-list-toolbar">
                    <div>
                        <strong>历史战斗记录</strong>
                        <span>每次加载 10 条；收藏优先置顶，每位玩家显示优先 Boss 目标的最高 DPS。</span>
                    </div>
                    <div>
                        <button
                            type="button"
                            class="record-bulk-delete-button"
                            :disabled="selectedDeleteCount === 0 || Boolean(operationName)"
                            title="批量删除已勾选记录及其全部相关文件"
                            @click="requestBatchDelete"
                        >
                            <v-icon icon="mdi-delete-sweep-outline" size="14" />
                            批量删除<span v-if="selectedDeleteCount">（{{ selectedDeleteCount }}）</span>
                        </button>
                        <button type="button" class="record-button" :disabled="listLoading" @click="refreshRecords">
                            <v-icon icon="mdi-refresh" size="14" />刷新
                        </button>
                        <button type="button" class="record-button" :disabled="loadingRecord || busy" @click="openLocalFile">
                            <v-icon icon="mdi-folder-open-outline" size="14" />打开其他文件
                        </button>
                    </div>
                    <input
                        ref="localFileInput"
                        type="file"
                        accept=".json,.ndjson,.dilmetercn,.txt"
                        hidden
                        @change="loadLocalFile"
                    />
                </div>

                <div v-if="listMessage" class="record-action-message" role="status">
                    <v-icon icon="mdi-check-circle-outline" size="16" />
                    <span>{{ listMessage }}</span>
                </div>

                <div v-if="listLoading" class="record-list-state" role="status">
                    <v-progress-circular indeterminate :size="28" :width="3" />
                    <span>正在整理历史记录…</span>
                </div>
                <div v-else-if="listError" class="record-list-state error" role="alert">
                    <v-icon icon="mdi-alert-circle-outline" size="22" />
                    <span>{{ listError }}</span>
                    <button type="button" class="record-button" @click="refreshRecords">重试</button>
                </div>
                <div v-else-if="records.length === 0" class="record-list-state">
                    <v-icon icon="mdi-file-document-outline" size="28" />
                    <strong>还没有可查看的战斗记录</strong>
                    <span>开始监测后，logs 文件夹中的记录会显示在这里。</span>
                </div>
                <div v-else ref="recordTableWrap" class="record-table-wrap" @scroll="onRecordScroll">
                    <table class="record-table">
                        <colgroup>
                            <col class="record-col-select" />
                            <col class="record-col-index" />
                            <col class="record-col-date" />
                            <col class="record-col-time" />
                            <col />
                            <col class="record-col-action" />
                        </colgroup>
                        <thead>
                            <tr>
                                <th class="record-select-cell">
                                    <input
                                        type="checkbox"
                                        :checked="allLoadedDeletableSelected"
                                        :indeterminate="someLoadedDeletableSelected"
                                        :disabled="deletableLoadedRecords.length === 0 || Boolean(operationName)"
                                        aria-label="选择当前已加载的全部可删除记录"
                                        title="选择当前已加载的全部可删除记录"
                                        @change="toggleAllLoadedRecords"
                                    />
                                </th>
                                <th>序号</th>
                                <th>日期</th>
                                <th>时间范围</th>
                                <th>记录内容</th>
                                <th>操作</th>
                            </tr>
                        </thead>
                        <tbody>
                            <tr
                                v-for="(record, index) in records"
                                :key="record.name"
                                :class="{ favorite: record.favorite, selected: isRecordSelected(record) }"
                            >
                                <td class="record-select-cell">
                                    <input
                                        type="checkbox"
                                        :checked="isRecordSelected(record)"
                                        :disabled="record.active || Boolean(operationName)"
                                        :aria-label="record.active ? '本次运行记录不可删除' : `选择 ${record.date} ${record.timeRange}`"
                                        :title="record.active ? '本次运行的记录不能删除' : '勾选后可批量删除'"
                                        @change="toggleRecordSelection(record, $event)"
                                    />
                                </td>
                                <td class="record-index">{{ index + 1 }}</td>
                                <td>
                                    <strong>{{ record.date }}</strong>
                                    <span v-if="record.active" class="record-live-badge">本次运行</span>
                                    <span v-else-if="record.favorite" class="record-favorite-badge">已收藏</span>
                                </td>
                                <td class="record-time">{{ record.timeRange }}</td>
                                <td>
                                    <div class="record-summary">
                                        <div v-if="record.players?.length" class="record-dps-target">
                                            <v-icon icon="mdi-bullseye-arrow" size="13" />
                                            <span>DPS 统计目标</span>
                                            <strong>{{ recordDPSTargetName(record) }}</strong>
                                        </div>
                                        <div v-if="record.players?.length" class="record-player-dps-list">
                                            <span v-for="player in record.players || []" :key="player.name" :title="`${recordDPSTargetName(record)} · 总伤害 ${formatNumber(player.totalDamage)}`">
                                                <strong>{{ displayName(player.name) }}</strong>
                                                <em>最高 DPS {{ formatNumber(player.dps) }}</em>
                                            </span>
                                        </div>
                                        <strong v-else>{{ record.summary }}</strong>
                                        <span>{{ record.summary }} · {{ formatSize(record.size) }}</span>
                                    </div>
                                </td>
                                <td>
                                    <div class="record-actions">
                                        <button
                                            type="button"
                                            class="record-view-button"
                                            :disabled="loadingRecord || busy || record.damageCount === 0"
                                            :title="record.damageCount === 0 ? '这条记录没有 DPS 数据' : '查看这条记录的 DPS'"
                                            @click="viewRecord(record)"
                                        >
                                            {{ loadingName === record.name ? "读取中" : "查看" }}
                                        </button>
                                        <button
                                            type="button"
                                            class="record-favorite-button"
                                            :class="{ active: record.favorite }"
                                            :disabled="operationName === record.name"
                                            :title="record.favorite ? '取消收藏' : '收藏并置顶'"
                                            @click="toggleFavorite(record)"
                                        >
                                            <v-icon :icon="record.favorite ? 'mdi-star' : 'mdi-star-outline'" size="14" />
                                            {{ record.favorite ? "取消" : "收藏" }}
                                        </button>
                                        <button
                                            type="button"
                                            class="record-delete-button"
                                            :disabled="record.active || operationName === record.name"
                                            :title="record.active ? '本次运行的记录不能删除' : '删除该次记录的全部相关文件'"
                                            @click="requestDelete(record)"
                                        >
                                            <v-icon icon="mdi-delete-outline" size="14" />删除
                                        </button>
                                    </div>
                                </td>
                            </tr>
                        </tbody>
                    </table>
                    <div v-if="loadingMore" class="record-load-more" role="status">
                        <v-progress-circular indeterminate :size="20" :width="2" />正在加载下一批…
                    </div>
                    <button v-else-if="hasMore" type="button" class="record-load-more-button" @click="loadMoreRecords">
                        继续加载 10 条
                    </button>
                </div>

                <footer class="record-browser-footer">
                    <span>
                        已显示 {{ records.length }} / {{ totalRecords }} 条
                        <template v-if="selectedDeleteCount">
                            · 已选择 {{ selectedDeleteCount }} 条
                            <button type="button" class="record-clear-selection" :disabled="Boolean(operationName)" @click="clearRecordSelection">清除选择</button>
                        </template>
                    </span>
                    <span>{{ hasMore ? "向下滚动会继续加载" : "已加载全部记录" }}</span>
                </footer>
            </template>

            <template v-else>
                <div v-if="detailError" class="record-list-state error" role="alert">
                    <v-icon icon="mdi-alert-circle-outline" size="22" />
                    <span>{{ detailError }}</span>
                    <button type="button" class="record-button" @click="returnToList">返回列表</button>
                </div>
                <div v-else class="record-detail-body">
                    <div class="record-detail-filters">
                        <label>
                            <span>战斗目标</span>
                            <select v-model="selectedBossId" :disabled="targetOptions.length === 0">
                                <option v-if="targetOptions.length === 0" value="">没有可用目标</option>
                                <option v-for="target in targetOptions" :key="target.id" :value="target.id">{{ target.label }}</option>
                            </select>
                        </label>
                        <label>
                            <span>查看队员</span>
                            <select v-model="selectedPlayerId" :disabled="playerOptions.length === 0">
                                <option v-if="playerOptions.length === 0" value="">没有 DPS 数据</option>
                                <option v-for="player in playerOptions" :key="player.entityId" :value="player.entityId">
                                    {{ displayName(player.name) }}
                                </option>
                            </select>
                        </label>
                        <div class="record-detail-file">
                            <span>记录文件</span>
                            <strong :title="selectedRecord?.name">{{ selectedRecord?.name || "外部记录" }}</strong>
                        </div>
                    </div>

                    <div v-if="!detailSummary || !selectedPlayer" class="record-list-state">
                        <v-icon icon="mdi-chart-line-variant" size="28" />
                        <strong>这条记录没有可显示的 DPS</strong>
                        <span>可以返回列表选择其他有伤害数据的记录。</span>
                    </div>
                    <template v-else>
                        <div class="record-primary-metrics">
                            <article>
                                <span>每秒伤害 DPS</span>
                                <strong>{{ formatNumber(selectedPlayer.totalDPS) }}</strong>
                                <small>{{ displayName(selectedPlayer.name) }}</small>
                            </article>
                            <article>
                                <span>副本时间</span>
                                <strong>{{ formatDuration(detailSummary.session.totalDuration) }}</strong>
                                <small>{{ formatClock(detailSummary.session.startAt) }} - {{ formatClock(detailSummary.session.endAt) }}</small>
                            </article>
                        </div>

                        <section class="record-dps-ranking" aria-label="全部队员 DPS">
                            <header>
                                <div>
                                    <strong>全部队员</strong>
                                    <span>{{ playerOptions.length }} 名有伤害记录的角色</span>
                                </div>
                                <span>点击一行查看该角色</span>
                            </header>
                            <button
                                v-for="(player, index) in playerOptions"
                                :key="player.entityId"
                                type="button"
                                :class="{ active: player.entityId === selectedPlayerId }"
                                @click="selectedPlayerId = player.entityId"
                            >
                                <span class="record-rank-number">{{ index + 1 }}</span>
                                <span class="record-rank-name">{{ displayName(player.name) }}</span>
                                <span class="record-rank-performance">
                                    <span><small>DPS</small><strong>{{ formatNumber(player.totalDPS) }}</strong></span>
                                    <i>/</i>
                                    <span>
                                        <small>累计伤害</small>
                                        <strong>{{ formatCompact(player.totalDamage) }}</strong>
                                        <em>（{{ formatPercent(playerContribution(player)) }}）</em>
                                    </span>
                                </span>
                            </button>
                        </section>

                        <button type="button" class="record-detail-toggle" :class="{ active: detailsOpen }" @click="detailsOpen = !detailsOpen">
                            <v-icon :icon="detailsOpen ? 'mdi-chevron-up' : 'mdi-chart-donut'" size="16" />
                            {{ detailsOpen ? "收起技能详情" : "查看技能占比" }}
                            <span>{{ selectedSkills.length }} 个造成伤害的技能</span>
                        </button>

                        <section v-if="detailsOpen" class="record-skill-details" aria-label="技能伤害占比">
                            <header>
                                <strong>{{ displayName(selectedPlayer.name) }} · 技能占比</strong>
                                <span>按技能伤害量从高到低排列</span>
                            </header>
                            <div v-if="selectedSkills.length === 0" class="record-skill-empty">没有造成伤害的技能。</div>
                            <div v-else class="record-skill-list">
                                <article v-for="skill in selectedSkills" :key="skill.skillId">
                                    <img :src="skillIconUrl(skill.skillId)" :alt="`${skillName(skill.skillId)}图标`" @error="fallbackSkillIcon" />
                                    <div>
                                        <header>
                                            <strong>{{ skillName(skill.skillId) }}</strong>
                                            <div class="record-skill-head-values">
                                                <span>{{ formatPercent(skill.ratio) }}</span>
                                                <small>累计 {{ formatCompact(skill.totalDamage) }}</small>
                                            </div>
                                        </header>
                                        <div class="record-skill-bar"><i :style="{ width: `${Math.max(1.5, skill.ratio * 100)}%` }" /></div>
                                        <div class="record-skill-metrics">
                                            <span><small>最大单次</small><strong>{{ formatCompact(maxSkillHit(skill)) }}</strong></span>
                                            <span><small>技能使用</small><strong>{{ skill.useCount }} 次</strong></span>
                                            <span><small>暴击次数</small><strong>{{ critText(skill) }}</strong></span>
                                        </div>
                                    </div>
                                </article>
                            </div>
                        </section>
                    </template>
                </div>

                <footer class="record-browser-footer detail-footer">
                    <button type="button" class="record-button" @click="returnToList">
                        <v-icon icon="mdi-arrow-left" size="14" />返回记录列表
                    </button>
                    <span>默认只展示 DPS 与副本时间，需要时再展开技能占比。</span>
                </footer>
            </template>

            <div v-if="deleteCandidates.length" class="record-delete-confirm-backdrop">
                <section class="record-delete-confirm" role="alertdialog" aria-modal="true" aria-labelledby="record-delete-title">
                    <header>
                        <v-icon icon="mdi-alert-outline" size="22" />
                        <div>
                            <strong id="record-delete-title">{{ deleteCandidates.length > 1 ? `批量删除 ${deleteCandidates.length} 条历史记录？` : "删除这条历史记录？" }}</strong>
                            <span>删除后无法恢复。</span>
                        </div>
                    </header>
                    <p>{{ deleteCandidates.length > 1 ? `预计会同时删除这些记录对应的 ${deleteCandidateFiles.length} 个文件：` : `会同时删除该次运行对应的 ${deleteCandidateFiles.length} 个文件：` }}</p>
                    <ul>
                        <li v-for="file in deleteCandidateFiles" :key="file">{{ file }}</li>
                    </ul>
                    <footer>
                        <button type="button" class="record-button" :disabled="deleteOperationActive" @click="deleteCandidates = []">取消</button>
                        <button type="button" class="record-confirm-delete-button" :disabled="deleteOperationActive" @click="confirmDelete">
                            {{ deleteOperationActive ? `删除中（${deleteProgress}/${deleteCandidates.length}）…` : deleteCandidates.length > 1 ? `确认删除 ${deleteCandidates.length} 条` : "确认永久删除" }}
                        </button>
                    </footer>
                </section>
            </div>
        </section>
    </div>
</template>

<script setup lang="ts">
import { computed, inject, nextTick, ref, shallowRef, watch, type Ref } from "vue";
import type { ActorManager, EntityActor } from "@/eventActor";
import { buildBossSummary, type BossSummary, type PlayerSummary, type SkillStat } from "@/summaryCollector";
import { getDisplayName, resourceNameVersion } from "@/store";
import { normalizeSkillDisplayName } from "@/skillDisplay";

type BattleRecordPlayerDPS = {
    name: string;
    dps: number;
    totalDamage: number;
};

type BattleRecordListItem = {
    name: string;
    date: string;
    timeRange: string;
    startedAt: number;
    endedAt: number;
    size: number;
    eventCount: number;
    damageCount: number;
    skillCount: number;
    attackerCount: number;
    targetCount: number;
    totalDamage: number;
    summary: string;
    active: boolean;
    favorite: boolean;
    relatedFiles: string[];
    players: BattleRecordPlayerDPS[];
    dpsTargetName?: string;
    dpsTargetRaceId?: number;
};

type SelectedSkill = SkillStat & { ratio: number };

const props = withDefaults(defineProps<{
    modelValue: boolean;
    busy?: boolean;
    loadRecord: (text: string, sourceName: string) => Promise<void>;
}>(), {
    busy: false,
});

const emit = defineEmits<{
    (event: "update:modelValue", value: boolean): void;
}>();

const actorManager = inject<Ref<ActorManager>>("actorManager")!;
const raceNameMap = inject<Ref<Record<number, string>>>("raceNameMap")!;
const skillNameMap = inject<Ref<Record<number, string>>>("skillNameMap")!;
const view = ref<"list" | "detail">("list");
const records = ref<BattleRecordListItem[]>([]);
const listLoading = ref(false);
const loadingMore = ref(false);
const loadingRecord = ref(false);
const loadingName = ref("");
const operationName = ref("");
const listError = ref("");
const listMessage = ref("");
const totalRecords = ref(0);
const hasMore = ref(false);
const detailError = ref("");
const selectedRecord = ref<BattleRecordListItem | null>(null);
const selectedRecordNames = ref<string[]>([]);
const deleteCandidates = ref<BattleRecordListItem[]>([]);
const deleteProgress = ref(0);
const selectedBossId = ref("");
const selectedPlayerId = ref("");
const detailSummary = shallowRef<BossSummary | null>(null);
const detailsOpen = ref(false);
const localFileInput = ref<HTMLInputElement | null>(null);
const recordTableWrap = ref<HTMLElement | null>(null);
const recordListScrollTop = ref(0);
const deleteScrollTop = ref(0);
const dataRevision = ref(0);
const isDesignPreview = import.meta.env.DEV && new URLSearchParams(window.location.search).has("preview");
const RECORD_PAGE_SIZE = 10;

watch(() => props.modelValue, (open) => {
    if (!open) return;
    view.value = "list";
    recordListScrollTop.value = 0;
    selectedRecordNames.value = [];
    deleteCandidates.value = [];
    deleteProgress.value = 0;
    detailsOpen.value = false;
    void refreshRecords();
});

const selectedRecordNameSet = computed(() => new Set(selectedRecordNames.value));
const deletableLoadedRecords = computed(() => records.value.filter((record) => !record.active));
const selectedDeleteCount = computed(() => selectedRecordNames.value.length);
const allLoadedDeletableSelected = computed(() => (
    deletableLoadedRecords.value.length > 0
    && deletableLoadedRecords.value.every((record) => selectedRecordNameSet.value.has(record.name))
));
const someLoadedDeletableSelected = computed(() => (
    !allLoadedDeletableSelected.value
    && deletableLoadedRecords.value.some((record) => selectedRecordNameSet.value.has(record.name))
));
const deleteCandidateFiles = computed(() => Array.from(new Set(
    deleteCandidates.value.flatMap((record) => record.relatedFiles),
)));
const deleteOperationActive = computed(() => operationName.value === "__batch_delete__" || (
    deleteCandidates.value.length === 1 && operationName.value === deleteCandidates.value[0]?.name
));

const targetOptions = computed(() => {
    dataRevision.value;
    resourceNameVersion.value;
    return (Object.values(actorManager.value.entityMap) as EntityActor[])
        .filter((entity) => !entity.isPC && entity.totalTakeDamage > 0)
        .map((entity) => {
            const maximumHealth = Number(entity.statMap?.[30]);
            const hasTrueHealth = Number.isFinite(maximumHealth) && maximumHealth > 0;
            return {
                id: entity.id,
                damage: entity.totalTakeDamage,
                health: hasTrueHealth ? maximumHealth : 0,
                label: `${cleanName(raceNameMap.value[entity.raceId]) || cleanName(entity.name) || `目标 ${entity.raceId}`} · ${hasTrueHealth ? `真实血量 ${formatCompact(maximumHealth)}` : `血量未知（已承伤 ${formatCompact(entity.totalTakeDamage)}）`}`,
            };
        })
        .sort((left, right) => right.damage - left.damage);
});

const playerOptions = computed<PlayerSummary[]>(() => detailSummary.value?.players ?? []);
const selectedPlayer = computed(() => playerOptions.value.find((player) => player.entityId === selectedPlayerId.value));
const selectedBossDamageBase = computed(() => {
    const entity = actorManager.value.entityMap[selectedBossId.value] as EntityActor | undefined;
    const maximumHealth = Number(entity?.statMap?.[30]);
    if (Number.isFinite(maximumHealth) && maximumHealth > 0) return maximumHealth;
    return detailSummary.value?.effectiveBossDamage || detailSummary.value?.totalDamage || 0;
});
const selectedSkills = computed<SelectedSkill[]>(() => {
    const player = selectedPlayer.value;
    if (!player || player.totalDamage <= 0) return [];
    return player.skillStats
        .filter((skill) => skill.totalDamage > 0)
        .map((skill) => ({ ...skill, ratio: skill.totalDamage / player.totalDamage }))
        .sort((left, right) => right.totalDamage - left.totalDamage);
});

watch(selectedBossId, () => refreshDetailSummary());
watch(playerOptions, (players) => {
    if (players.some((player) => player.entityId === selectedPlayerId.value)) return;
    selectedPlayerId.value = players[0]?.entityId ?? "";
}, { immediate: true });

async function refreshRecords() {
    listLoading.value = true;
    listError.value = "";
    listMessage.value = "";
    records.value = [];
    selectedRecordNames.value = [];
    totalRecords.value = 0;
    hasMore.value = false;
    try {
        await fetchRecordPage(0, true);
        if (isDesignPreview && records.value.length === 0) {
            const preview = previewRecordItems();
            records.value = preview.slice(0, RECORD_PAGE_SIZE);
            totalRecords.value = preview.length;
            hasMore.value = records.value.length < preview.length;
        }
    } catch (error) {
        if (isDesignPreview) {
            const preview = previewRecordItems();
            records.value = preview.slice(0, RECORD_PAGE_SIZE);
            totalRecords.value = preview.length;
            hasMore.value = records.value.length < preview.length;
            listError.value = "";
        } else {
            listError.value = `无法读取历史记录：${error}`;
        }
    } finally {
        listLoading.value = false;
    }
}

async function fetchRecordPage(offset: number, reset = false) {
    const response = await fetch(`/api/battle_records?offset=${offset}&limit=${RECORD_PAGE_SIZE}`, { cache: "no-store" });
    if (!response.ok) throw new Error(await responseMessage(response));
    const payload = await response.json() as {
        records?: BattleRecordListItem[];
        total?: number;
        hasMore?: boolean;
    };
    const page = Array.isArray(payload.records) ? payload.records.map(normalizeRecordItem) : [];
    records.value = reset ? page : [...records.value, ...page];
    totalRecords.value = Number.isFinite(payload.total) ? Number(payload.total) : records.value.length;
    hasMore.value = Boolean(payload.hasMore);
}

async function loadMoreRecords() {
    if (loadingMore.value || listLoading.value || !hasMore.value) return;
    loadingMore.value = true;
    listError.value = "";
    try {
        if (isDesignPreview && records.value.some((record) => record.name.startsWith("__preview__"))) {
            const preview = previewRecordItems();
            const next = preview.slice(records.value.length, records.value.length + RECORD_PAGE_SIZE);
            records.value.push(...next);
            totalRecords.value = preview.length;
            hasMore.value = records.value.length < preview.length;
        } else {
            await fetchRecordPage(records.value.length);
        }
    } catch (error) {
        listError.value = `无法继续加载：${error}`;
    } finally {
        loadingMore.value = false;
    }
}

function onRecordScroll(event: Event) {
    const target = event.currentTarget as HTMLElement;
    if (target.scrollHeight - target.scrollTop - target.clientHeight <= 90) {
        void loadMoreRecords();
    }
}

async function viewRecord(record: BattleRecordListItem) {
    recordListScrollTop.value = recordTableWrap.value?.scrollTop ?? 0;
    loadingRecord.value = true;
    loadingName.value = record.name;
    detailError.value = "";
    detailsOpen.value = false;
    try {
        if (isDesignPreview && record.name.startsWith("__preview__")) {
            await props.loadRecord(previewRecordNdjson(), record.name.replace("__preview__", ""));
        } else {
            const response = await fetch(`/api/battle_records?name=${encodeURIComponent(record.name)}`, { cache: "no-store" });
            if (!response.ok) throw new Error(await responseMessage(response));
            await props.loadRecord(await response.text(), record.name);
        }
        selectedRecord.value = isDesignPreview && record.name.startsWith("__preview__")
            ? { ...record, name: record.name.replace("__preview__", "") }
            : record;
        await prepareDetail();
    } catch (error) {
        listError.value = `无法打开这条记录：${error}`;
    } finally {
        loadingRecord.value = false;
        loadingName.value = "";
    }
}

async function toggleFavorite(record: BattleRecordListItem) {
    if (operationName.value) return;
    const nextFavorite = !record.favorite;
    operationName.value = record.name;
    listError.value = "";
    listMessage.value = "";
    try {
        if (isDesignPreview && record.name.startsWith("__preview__")) {
            record.favorite = nextFavorite;
            records.value.sort((left, right) => Number(right.favorite) - Number(left.favorite) || right.startedAt - left.startedAt);
        } else {
            const response = await fetch(`/api/battle_records?name=${encodeURIComponent(record.name)}`, {
                method: "PATCH",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ favorite: nextFavorite }),
            });
            if (!response.ok) throw new Error(await responseMessage(response));
            await refreshRecords();
        }
        listMessage.value = nextFavorite ? "已收藏并置顶这条记录。" : "已取消收藏。";
    } catch (error) {
        listError.value = `无法更新收藏：${error}`;
    } finally {
        operationName.value = "";
    }
}

function isRecordSelected(record: BattleRecordListItem) {
    return selectedRecordNameSet.value.has(record.name);
}

function toggleRecordSelection(record: BattleRecordListItem, event: Event) {
    if (record.active || operationName.value) return;
    const checked = (event.currentTarget as HTMLInputElement).checked;
    const selected = new Set(selectedRecordNames.value);
    if (checked) selected.add(record.name);
    else selected.delete(record.name);
    selectedRecordNames.value = Array.from(selected);
}

function toggleAllLoadedRecords(event: Event) {
    if (operationName.value) return;
    const checked = (event.currentTarget as HTMLInputElement).checked;
    const loadedNames = new Set(deletableLoadedRecords.value.map((record) => record.name));
    const selected = new Set(selectedRecordNames.value);
    for (const name of loadedNames) {
        if (checked) selected.add(name);
        else selected.delete(name);
    }
    selectedRecordNames.value = Array.from(selected);
}

function clearRecordSelection() {
    if (operationName.value) return;
    selectedRecordNames.value = [];
}

function requestDelete(record: BattleRecordListItem) {
    if (record.active || operationName.value) return;
    deleteScrollTop.value = recordTableWrap.value?.scrollTop ?? 0;
    deleteCandidates.value = [record];
    deleteProgress.value = 0;
}

function requestBatchDelete() {
    if (operationName.value || selectedDeleteCount.value === 0) return;
    const selected = selectedRecordNameSet.value;
    const candidates = records.value.filter((record) => !record.active && selected.has(record.name));
    if (candidates.length === 0) {
        selectedRecordNames.value = [];
        return;
    }
    deleteScrollTop.value = recordTableWrap.value?.scrollTop ?? 0;
    deleteCandidates.value = candidates;
    deleteProgress.value = 0;
}

async function confirmDelete() {
    const candidates = deleteCandidates.value.filter((record) => !record.active);
    if (candidates.length === 0 || operationName.value) return;
    operationName.value = candidates.length > 1 ? "__batch_delete__" : candidates[0].name;
    deleteProgress.value = 0;
    listError.value = "";
    listMessage.value = "";

    const deletedNames = new Set<string>();
    const deletedFiles = new Set<string>();
    const failures: string[] = [];
    try {
        for (const record of candidates) {
            try {
                let recordDeletedFiles = record.relatedFiles;
                if (!(isDesignPreview && record.name.startsWith("__preview__"))) {
                    const response = await fetch(`/api/battle_records?name=${encodeURIComponent(record.name)}`, { method: "DELETE" });
                    if (!response.ok) throw new Error(await responseMessage(response));
                    const payload = await response.json() as { deleted?: string[] };
                    if (Array.isArray(payload.deleted)) recordDeletedFiles = payload.deleted;
                }
                deletedNames.add(record.name);
                for (const file of recordDeletedFiles) deletedFiles.add(file);
            } catch (error) {
                failures.push(`${record.date} ${record.timeRange}：${error}`);
            } finally {
                deleteProgress.value += 1;
            }
        }

        if (deletedNames.size > 0) {
            records.value = records.value.filter((record) => !deletedNames.has(record.name));
            totalRecords.value = Math.max(0, totalRecords.value - deletedNames.size);
            selectedRecordNames.value = selectedRecordNames.value.filter((name) => !deletedNames.has(name));
        }
        hasMore.value = records.value.length < totalRecords.value;
        deleteCandidates.value = [];

        if (failures.length > 0) {
            listError.value = `已删除 ${deletedNames.size} 条记录，但有 ${failures.length} 条失败：${failures.join("；")}`;
        } else if (deletedNames.size > 1) {
            listMessage.value = `已批量删除 ${deletedNames.size} 条记录，共 ${deletedFiles.size} 个相关文件。`;
        } else {
            listMessage.value = `已永久删除 ${deletedFiles.size} 个相关文件。`;
        }

        await nextTick();
        if (recordTableWrap.value) {
            const maximumScroll = Math.max(0, recordTableWrap.value.scrollHeight - recordTableWrap.value.clientHeight);
            recordTableWrap.value.scrollTop = Math.min(deleteScrollTop.value, maximumScroll);
        }
    } finally {
        operationName.value = "";
        deleteProgress.value = 0;
    }
}

function openLocalFile() {
    if (!localFileInput.value) return;
    recordListScrollTop.value = recordTableWrap.value?.scrollTop ?? 0;
    localFileInput.value.value = "";
    localFileInput.value.click();
}

async function loadLocalFile(event: Event) {
    const input = event.currentTarget as HTMLInputElement;
    const file = input.files?.[0];
    if (!file) return;
    loadingRecord.value = true;
    loadingName.value = file.name;
    listError.value = "";
    try {
        await props.loadRecord(await file.text(), file.name);
        const date = new Date(file.lastModified || Date.now());
        selectedRecord.value = {
            name: file.name,
            date: date.toLocaleDateString("zh-CN"),
            timeRange: "外部文件",
            startedAt: Math.floor(date.getTime() / 1000),
            endedAt: Math.floor(date.getTime() / 1000),
            size: file.size,
            eventCount: 0,
            damageCount: 1,
            skillCount: 0,
            attackerCount: 0,
            targetCount: 0,
            totalDamage: 0,
            summary: "外部记录",
            active: false,
            favorite: false,
            relatedFiles: [file.name],
            players: [],
        };
        await prepareDetail();
    } catch (error) {
        listError.value = `无法打开这个文件：${error}`;
    } finally {
        loadingRecord.value = false;
        loadingName.value = "";
    }
}

async function prepareDetail() {
    await nextTick();
    dataRevision.value += 1;
    selectedBossId.value = targetOptions.value[0]?.id ?? "";
    refreshDetailSummary();
    selectedPlayerId.value = playerOptions.value[0]?.entityId ?? "";
    detailError.value = selectedBossId.value ? "" : "这条记录中没有可用于计算 DPS 的战斗目标。";
    view.value = "detail";
}

function refreshDetailSummary() {
    detailSummary.value = selectedBossId.value
        ? buildBossSummary(selectedBossId.value, actorManager.value, [])
        : null;
}

function close() {
    deleteCandidates.value = [];
    selectedRecordNames.value = [];
    emit("update:modelValue", false);
}

async function returnToList() {
    view.value = "list";
    await nextTick();
    if (!recordTableWrap.value) return;
    const maximumScroll = Math.max(0, recordTableWrap.value.scrollHeight - recordTableWrap.value.clientHeight);
    recordTableWrap.value.scrollTop = Math.min(recordListScrollTop.value, maximumScroll);
}

function normalizeRecordItem(record: BattleRecordListItem): BattleRecordListItem {
    return {
        ...record,
        favorite: Boolean(record.favorite),
        relatedFiles: Array.isArray(record.relatedFiles) && record.relatedFiles.length ? record.relatedFiles : [record.name],
        players: Array.isArray(record.players) ? record.players : [],
        dpsTargetName: cleanName(record.dpsTargetName),
        dpsTargetRaceId: Number(record.dpsTargetRaceId) || 0,
    };
}

const RECORD_BOSS_NAME_OVERRIDES: Record<number, string> = {
    7600: "枯木之佩塔克",
    7601: "枯木之佩塔克",
    7602: "布隆塔纳斯",
    7603: "雷内恩的米耶尔",
    7615: "雷内恩的米耶尔：悔恨",
};

function recordDPSTargetName(record: BattleRecordListItem) {
    const raceId = Number(record.dpsTargetRaceId) || 0;
    const rawName = cleanName(record.dpsTargetName);
    return RECORD_BOSS_NAME_OVERRIDES[raceId]
        || cleanName(raceNameMap.value[raceId])
        || getDisplayName(rawName)
        || rawName
        || "其他 Boss 级目标";
}

function displayName(name: string) {
    return getDisplayName(cleanName(name)) || "未命名角色";
}

function skillName(skillId: number) {
    const raw = cleanName(skillNameMap.value[skillId]);
    return normalizeSkillDisplayName(skillId, raw) || `技能 ${skillId}`;
}

function skillIconUrl(skillId: number) {
    return `/skill-icons/${skillId}.png`;
}

function fallbackSkillIcon(event: Event) {
    const image = event.currentTarget as HTMLImageElement;
    if (image.dataset.fallback === "1") return;
    image.dataset.fallback = "1";
    image.src = skillIconUrl(10001);
}

function cleanName(value: string | undefined) {
    return value?.replace(/\s+\d+$/, "").trim() ?? "";
}

function formatNumber(value: number) {
    return Math.round(value || 0).toLocaleString("zh-CN");
}

function formatCompact(value: number) {
    const absolute = Math.abs(value || 0);
    if (absolute >= 100_000_000) return `${(value / 100_000_000).toFixed(2)}亿`;
    if (absolute >= 10_000) return `${(value / 10_000).toFixed(1)}万`;
    return formatNumber(value);
}

function formatDuration(seconds: number) {
    const total = Math.max(0, Math.round(seconds || 0));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const remainder = total % 60;
    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function formatClock(seconds: number) {
    return new Date(seconds * 1000).toLocaleTimeString("zh-CN", { hour12: false });
}

function formatPercent(value: number) {
    return `${((value || 0) * 100).toFixed(1)}%`;
}

function maxSkillHit(skill: SkillStat) {
    return Math.max(skill.maxCritDamage ?? 0, skill.maxNonCritDamage ?? 0);
}

function critText(skill: SkillStat) {
    return skill.noCritRate ? `${skill.critHits}（—）` : `${skill.critHits}（${formatPercent(skill.critRate)}）`;
}

function playerContribution(player: PlayerSummary) {
    const total = selectedBossDamageBase.value;
    return total > 0 ? player.totalDamage / total : 0;
}

function formatSize(value: number) {
    if (value >= 1024 * 1024 * 1024) return `${(value / (1024 * 1024 * 1024)).toFixed(1)} GB`;
    if (value >= 1024 * 1024) return `${(value / (1024 * 1024)).toFixed(1)} MB`;
    if (value >= 1024) return `${(value / 1024).toFixed(1)} KB`;
    return `${value} B`;
}

async function responseMessage(response: Response) {
    try {
        const payload = await response.json() as { error?: string };
        return payload.error || `HTTP ${response.status}`;
    } catch {
        return `HTTP ${response.status}`;
    }
}

function previewRecordItems(): BattleRecordListItem[] {
    const items: BattleRecordListItem[] = [
        {
            name: "__preview__packet_log_2026-08-29_20-14-03.ndjson",
            date: "2026-08-29",
            timeRange: "20:14:03 - 20:16:41",
            startedAt: 1_788_005_643,
            endedAt: 1_788_005_801,
            size: 2_846_120,
            eventCount: 1862,
            damageCount: 642,
            skillCount: 18,
            attackerCount: 4,
            targetCount: 1,
            totalDamage: 1_037_202_447,
            summary: "642 条伤害 · 18 个技能 · 4 名攻击者 / 1 个目标",
            dpsTargetName: "雷内恩的米耶尔",
            dpsTargetRaceId: 7603,
            active: true,
            favorite: false,
            relatedFiles: ["packet_log_2026-08-29_20-14-03.ndjson", "log_2026-08-29_20-14-03.txt", "packet_capture_1788005643.pcapng"],
            players: [
                { name: "风铃", dps: 1_037_870, totalDamage: 143_225_998 },
                { name: "星弦", dps: 852_652, totalDamage: 117_665_976 },
                { name: "青岚", dps: 657_000, totalDamage: 90_666_000 },
            ],
        },
        {
            name: "__preview__packet_log_2026-08-29_18-42-16.ndjson",
            date: "2026-08-29",
            timeRange: "18:42:16 - 18:45:02",
            startedAt: 1_787_995_336,
            endedAt: 1_787_995_502,
            size: 1_624_908,
            eventCount: 1208,
            damageCount: 401,
            skillCount: 15,
            attackerCount: 3,
            targetCount: 2,
            totalDamage: 684_502_310,
            summary: "401 条伤害 · 15 个技能 · 3 名攻击者 / 2 个目标",
            dpsTargetName: "雷内恩的米耶尔：悔恨",
            dpsTargetRaceId: 7615,
            active: false,
            favorite: true,
            relatedFiles: ["packet_log_2026-08-29_18-42-16.ndjson", "log_2026-08-29_18-42-16.txt", "packet_capture_1787995336.pcapng"],
            players: [
                { name: "流光", dps: 924_410, totalDamage: 153_452_060 },
                { name: "枫墨", dps: 711_825, totalDamage: 118_163_000 },
            ],
        },
        {
            name: "__preview__packet_log_2026-08-28_23-08-51.ndjson",
            date: "2026-08-28",
            timeRange: "23:08:51 - 23:09:44",
            startedAt: 1_787_924_931,
            endedAt: 1_787_924_984,
            size: 458_313,
            eventCount: 329,
            damageCount: 87,
            skillCount: 8,
            attackerCount: 2,
            targetCount: 1,
            totalDamage: 148_302_644,
            summary: "87 条伤害 · 8 个技能 · 2 名攻击者 / 1 个目标",
            dpsTargetName: "布隆塔纳斯",
            dpsTargetRaceId: 7602,
            active: false,
            favorite: false,
            relatedFiles: ["packet_log_2026-08-28_23-08-51.ndjson", "log_2026-08-28_23-08-51.txt"],
            players: [{ name: "山雨", dps: 688_230, totalDamage: 36_475_900 }],
        },
        {
            name: "__preview__packet_log_2026-08-28_21-55-04.ndjson",
            date: "2026-08-28",
            timeRange: "21:55:04 - 21:55:29",
            startedAt: 1_787_920_504,
            endedAt: 1_787_920_529,
            size: 82_092,
            eventCount: 74,
            damageCount: 0,
            skillCount: 0,
            attackerCount: 0,
            targetCount: 0,
            totalDamage: 0,
            summary: "无伤害记录 · 74 条状态或场景数据",
            active: false,
            favorite: false,
            relatedFiles: ["packet_log_2026-08-28_21-55-04.ndjson"],
            players: [],
        },
    ];

    for (let index = 4; index < 16; index += 1) {
        const day = 28 - Math.floor(index / 4);
        const hour = 20 - (index % 4);
        const stamp = `2026-08-${String(day).padStart(2, "0")}_${String(hour).padStart(2, "0")}-16-2${index % 10}`;
        const startedAt = 1_787_900_000 - index * 7_200;
        items.push({
            name: `__preview__packet_log_${stamp}.ndjson`,
            date: `2026-08-${String(day).padStart(2, "0")}`,
            timeRange: `${String(hour).padStart(2, "0")}:16:2${index % 10} - ${String(hour).padStart(2, "0")}:18:4${index % 10}`,
            startedAt,
            endedAt: startedAt + 140,
            size: 720_000 + index * 31_000,
            eventCount: 600 + index * 13,
            damageCount: 220 + index * 7,
            skillCount: 10 + index % 6,
            attackerCount: 3,
            targetCount: 1,
            totalDamage: 380_000_000 + index * 8_500_000,
            summary: `${220 + index * 7} 条伤害 · ${10 + index % 6} 个技能 · 3 名攻击者 / 1 个目标`,
            dpsTargetName: "枯木之佩塔克",
            dpsTargetRaceId: 7600,
            active: false,
            favorite: false,
            relatedFiles: [`packet_log_${stamp}.ndjson`, `log_${stamp}.txt`, `packet_capture_${startedAt}.pcapng`],
            players: [
                { name: "风铃", dps: 820_000 - index * 8_900, totalDamage: 114_800_000 },
                { name: "星弦", dps: 690_000 - index * 6_200, totalDamage: 96_600_000 },
            ],
        });
    }
    return items.sort((left, right) => Number(right.favorite) - Number(left.favorite) || right.startedAt - left.startedAt);
}

function previewRecordNdjson(): string {
    const start = 1_788_005_643;
    const events: Array<Record<string, unknown>> = [
        { EventId: 1, At: start, Id: "preview-boss", Name: "雷内恩的米耶尔", RaceId: 7603, Height: 1, Weight: 1, Upper: 1, Lower: 1, GuildName: "", OwnerId: "" },
        { EventId: 1, At: start, Id: "preview-player-1", Name: "风铃", RaceId: 10001, Height: 1, Weight: 1, Upper: 1, Lower: 1, GuildName: "", OwnerId: "" },
        { EventId: 1, At: start, Id: "preview-player-2", Name: "星弦", RaceId: 10002, Height: 1, Weight: 1, Upper: 1, Lower: 1, GuildName: "", OwnerId: "" },
        { EventId: 1, At: start, Id: "preview-player-3", Name: "青岚", RaceId: 9001, Height: 1, Weight: 1, Upper: 1, Lower: 1, GuildName: "", OwnerId: "" },
        { EventId: 17, At: start + 1, Id: "preview-boss", Private: false, Stats: [{ StatId: 28, Value: 1_506_000_000 }, { StatId: 30, Value: 2_005_000_000 }] },
        { EventId: 11, At: start, Id: "preview-player-1", Reliable: true, Reset: false },
    ];
    const players = [
        { id: "preview-player-1", skills: [59060, 59061, 59064, 58100], base: 8_600_000 },
        { id: "preview-player-2", skills: [59040, 59041, 59042], base: 6_900_000 },
        { id: "preview-player-3", skills: [59120, 59121, 58009], base: 5_400_000 },
    ];
    for (let playerIndex = 0; playerIndex < players.length; playerIndex += 1) {
        const player = players[playerIndex];
        for (let hit = 0; hit < 18; hit += 1) {
            events.push({
                EventId: 3,
                At: start + 2 + hit * 8 + playerIndex,
                Id: player.id,
                TargetId: "preview-boss",
                SkillId: player.skills[hit % player.skills.length],
                Damage: player.base - (hit % player.skills.length) * 720_000 + hit * 42_000,
                IsCritical: hit % 3 !== 0,
                IsDelayed: false,
            });
        }
    }
    return `${events.map((event) => JSON.stringify(event)).join("\n")}\n`;
}
</script>

<style scoped>
.record-browser-backdrop {
    position: fixed;
    z-index: 2600;
    inset: 0;
    display: grid;
    place-items: center;
    padding: 20px;
    background: rgba(0, 0, 0, .7);
    font-family: "Microsoft YaHei", sans-serif;
}

.record-browser-dialog {
    --record-danger: color-mix(in srgb, #ff5f56 78%, var(--ui-theme-text));
    position: relative;
    display: flex;
    flex-direction: column;
    width: min(1240px, calc(100vw - 40px));
    height: min(760px, calc(100vh - 40px));
    overflow: hidden;
    color: var(--ui-theme-text);
    background: var(--ui-theme-canvas);
    border: 1px solid var(--ui-theme-border);
    box-shadow: 0 18px 56px rgba(0, 0, 0, .72);
}

.record-browser-titlebar {
    display: flex;
    align-items: center;
    min-height: 42px;
    padding: 0 11px;
    background: linear-gradient(var(--ui-theme-raised), var(--ui-theme-surface));
    border-bottom: 1px solid var(--ui-theme-border);
}

.record-browser-titlebar > div {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 0;
    font-size: 14px;
    font-weight: 800;
}

.record-browser-titlebar > div :deep(.v-icon) { color: var(--ui-color-accent); }
.record-browser-titlebar small { overflow: hidden; margin-left: 6px; color: var(--ui-theme-muted); font-size: 10px; font-weight: 500; text-overflow: ellipsis; white-space: nowrap; }
.record-browser-titlebar > button { display: grid; flex: 0 0 28px; width: 28px; height: 28px; margin-left: auto; place-items: center; color: var(--ui-theme-text); background: transparent; border: 0; cursor: pointer; }
.record-browser-titlebar > .record-back-button { margin: 0 7px 0 0; border-right: 1px solid var(--ui-theme-border-soft); }
.record-browser-titlebar > button:hover { color: var(--ui-color-accent); background: color-mix(in srgb, var(--ui-theme-raised) 76%, var(--ui-color-accent)); }

.record-list-toolbar,
.record-detail-filters {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 11px 13px;
    background: var(--ui-theme-surface);
    border-bottom: 1px solid var(--ui-theme-border-soft);
}

.record-list-toolbar > div:first-child { display: grid; gap: 2px; }
.record-list-toolbar > div:first-child strong { font-size: 12px; }
.record-list-toolbar > div:first-child span { color: var(--ui-theme-muted); font-size: 10px; }
.record-list-toolbar > div:last-of-type { display: flex; gap: 6px; margin-left: auto; }
.record-action-message { display: flex; align-items: center; gap: 6px; min-height: 31px; padding: 5px 13px; color: var(--ui-theme-text); background: color-mix(in srgb, var(--ui-color-accent) 14%, var(--ui-theme-surface)); border-bottom: 1px solid var(--ui-theme-border-soft); font-size: 10px; }
.record-action-message :deep(.v-icon) { color: var(--ui-color-accent); }

.record-button,
.record-view-button,
.record-favorite-button,
.record-delete-button,
.record-bulk-delete-button,
.record-confirm-delete-button,
.record-detail-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    min-height: 29px;
    padding: 0 10px;
    color: var(--ui-theme-text);
    background: linear-gradient(var(--ui-theme-raised), var(--ui-theme-control));
    border: 1px solid var(--ui-theme-border);
    font: 700 10px "Microsoft YaHei", sans-serif;
    cursor: pointer;
}

.record-button:hover:not(:disabled),
.record-view-button:hover:not(:disabled),
.record-favorite-button:hover:not(:disabled),
.record-delete-button:hover:not(:disabled),
.record-bulk-delete-button:hover:not(:disabled),
.record-confirm-delete-button:hover:not(:disabled),
.record-detail-toggle:hover { background: linear-gradient(var(--ui-theme-surface-hover), var(--ui-theme-control)); border-color: var(--ui-color-accent); }
.record-button:disabled,
.record-view-button:disabled,
.record-favorite-button:disabled,
.record-delete-button:disabled,
.record-bulk-delete-button:disabled,
.record-confirm-delete-button:disabled { opacity: .46; cursor: not-allowed; }
.record-view-button { min-width: 58px; color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); border-color: var(--ui-color-accent); }
.record-favorite-button.active { color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); border-color: var(--ui-color-accent); }
.record-delete-button { color: var(--record-danger); }
.record-bulk-delete-button { color: #fff; background: color-mix(in srgb, var(--record-danger) 82%, var(--ui-theme-control)); border-color: var(--record-danger); }
.record-confirm-delete-button { color: #fff; background: var(--record-danger); border-color: var(--record-danger); }

.record-table-wrap { flex: 1; min-height: 0; overflow: auto; padding: 0 10px 10px; background: var(--ui-theme-canvas); }
.record-table { width: 100%; border-collapse: separate; border-spacing: 0; font-size: 11px; }
.record-table th { position: sticky; z-index: 1; top: 0; height: 34px; padding: 0 9px; color: var(--ui-theme-muted); background: var(--ui-theme-control); border-bottom: 1px solid var(--ui-theme-border); text-align: left; }
.record-table td { min-height: 68px; padding: 8px 9px; background: color-mix(in srgb, var(--ui-theme-surface) 88%, transparent); border-bottom: 1px solid var(--ui-theme-border-soft); vertical-align: middle; }
.record-table tbody tr:hover td { background: var(--ui-theme-surface-hover); }
.record-table tbody tr.favorite td { background: color-mix(in srgb, var(--ui-theme-selected) 12%, var(--ui-theme-surface)); }
.record-table tbody tr.favorite:hover td { background: color-mix(in srgb, var(--ui-theme-selected) 18%, var(--ui-theme-surface-hover)); }
.record-table tbody tr.selected td { background: color-mix(in srgb, var(--record-danger) 10%, var(--ui-theme-surface)); }
.record-table tbody tr.selected:hover td { background: color-mix(in srgb, var(--record-danger) 14%, var(--ui-theme-surface-hover)); }
.record-table th:first-child,
.record-table td:first-child { border-left: 1px solid var(--ui-theme-border-soft); text-align: center; }
.record-table th:last-child,
.record-table td:last-child { border-right: 1px solid var(--ui-theme-border-soft); text-align: center; }
.record-col-index { width: 54px; }
.record-col-select { width: 38px; }
.record-col-date { width: 132px; }
.record-col-time { width: 174px; }
.record-col-action { width: 218px; }
.record-index { color: var(--ui-theme-muted); font-weight: 800; }
.record-select-cell { width: 38px; padding-right: 4px !important; padding-left: 4px !important; text-align: center !important; }
.record-select-cell input { width: 15px; height: 15px; margin: 0; accent-color: var(--record-danger); cursor: pointer; vertical-align: middle; }
.record-select-cell input:disabled { opacity: .38; cursor: not-allowed; }
.record-table td > strong { font-size: 11px; }
.record-time { font-variant-numeric: tabular-nums; white-space: nowrap; }
.record-live-badge { display: inline-flex; margin-left: 6px; padding: 1px 5px; color: var(--ui-theme-on-accent); background: var(--ui-color-accent); border-radius: 2px; font-size: 8px; font-weight: 800; }
.record-favorite-badge { display: inline-flex; margin-left: 6px; padding: 1px 5px; color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); border: 1px solid var(--ui-color-accent); border-radius: 2px; font-size: 8px; font-weight: 800; }
.record-summary { display: grid; gap: 3px; }
.record-summary strong { font-size: 11px; }
.record-summary span { color: var(--ui-theme-muted); font-size: 9px; }
.record-dps-target { display: flex; align-items: center; gap: 5px; min-width: 0; }
.record-dps-target :deep(.v-icon) { flex: 0 0 auto; color: var(--ui-color-accent); }
.record-dps-target > span { flex: 0 0 auto; }
.record-dps-target > strong { overflow: hidden; color: var(--ui-theme-text); text-overflow: ellipsis; white-space: nowrap; }
.record-player-dps-list { display: flex; flex-wrap: wrap; gap: 4px; }
.record-player-dps-list > span { display: inline-flex; align-items: baseline; gap: 5px; padding: 3px 6px; color: var(--ui-theme-text); background: var(--ui-theme-raised); border: 1px solid var(--ui-theme-border-soft); white-space: nowrap; }
.record-player-dps-list strong { max-width: 90px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.record-player-dps-list em { color: var(--ui-color-accent); font-style: normal; font-weight: 800; font-variant-numeric: tabular-nums; }
.record-actions { display: flex; align-items: center; justify-content: center; gap: 4px; }
.record-actions button { min-width: 58px; padding: 0 7px; }
.record-load-more { display: flex; align-items: center; justify-content: center; gap: 7px; min-height: 44px; color: var(--ui-theme-muted); font-size: 10px; }
.record-load-more-button { display: block; min-width: 180px; height: 32px; margin: 10px auto 0; color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); font: 700 10px "Microsoft YaHei", sans-serif; cursor: pointer; }
.record-load-more-button:hover { color: var(--ui-color-accent); border-color: var(--ui-color-accent); }

.record-list-state { display: flex; flex: 1; min-height: 220px; flex-direction: column; align-items: center; justify-content: center; gap: 9px; padding: 30px; color: var(--ui-theme-muted); text-align: center; }
.record-list-state strong { color: var(--ui-theme-text); font-size: 13px; }
.record-list-state.error :deep(.v-icon) { color: #ff7d72; }

.record-browser-footer { display: flex; align-items: center; justify-content: space-between; min-height: 39px; gap: 12px; padding: 6px 12px; color: var(--ui-theme-muted); background: var(--ui-theme-surface); border-top: 1px solid var(--ui-theme-border); font-size: 9px; }
.record-clear-selection { margin-left: 3px; padding: 1px 4px; color: var(--record-danger); background: transparent; border: 0; font: 700 9px "Microsoft YaHei", sans-serif; text-decoration: underline; cursor: pointer; }
.record-clear-selection:disabled { opacity: .45; cursor: not-allowed; }

.record-detail-body { flex: 1; min-height: 0; overflow: auto; padding: 12px; background: var(--ui-theme-canvas); }
.record-detail-filters { padding: 9px 12px; }
.record-detail-filters label,
.record-detail-file { display: grid; gap: 4px; min-width: 0; }
.record-detail-filters label { flex: 0 1 290px; }
.record-detail-filters label > span,
.record-detail-file > span { color: var(--ui-theme-muted); font-size: 9px; font-weight: 700; }
.record-detail-filters select { width: 100%; height: 29px; padding: 0 25px 0 7px; overflow: hidden; color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); font: 700 10px "Microsoft YaHei", sans-serif; text-overflow: ellipsis; }
.record-detail-file { margin-left: auto; max-width: 260px; }
.record-detail-file strong { overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }

.record-primary-metrics { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
.record-primary-metrics article { display: grid; min-height: 112px; align-content: center; gap: 4px; padding: 15px 20px; background: var(--ui-theme-raised); border: 1px solid var(--ui-theme-border-soft); box-shadow: inset 3px 0 var(--ui-color-accent); }
.record-primary-metrics span { color: var(--ui-theme-muted); font-size: 10px; font-weight: 800; }
.record-primary-metrics strong { color: var(--ui-color-accent); font-size: clamp(25px, 3vw, 38px); line-height: 1.1; font-variant-numeric: tabular-nums; }
.record-primary-metrics small { color: var(--ui-theme-muted); font-size: 10px; }

.record-dps-ranking { margin-top: 10px; overflow: hidden; background: var(--ui-theme-surface); border: 1px solid var(--ui-theme-border-soft); }
.record-dps-ranking > header { display: flex; align-items: center; justify-content: space-between; min-height: 42px; padding: 7px 11px; background: linear-gradient(var(--ui-theme-raised), var(--ui-theme-surface)); border-bottom: 1px solid var(--ui-theme-border-soft); }
.record-dps-ranking > header > div { display: flex; align-items: baseline; gap: 7px; }
.record-dps-ranking > header strong { font-size: 11px; }
.record-dps-ranking > header span { color: var(--ui-theme-muted); font-size: 9px; }
.record-dps-ranking > button { display: grid; width: 100%; grid-template-columns: 44px minmax(0, 1fr) minmax(390px, 46%); align-items: center; min-height: 42px; padding: 0 11px; color: var(--ui-theme-text); background: var(--ui-theme-surface); border: 0; border-bottom: 1px solid var(--ui-theme-border-soft); text-align: left; cursor: pointer; }
.record-dps-ranking > button:last-child { border-bottom: 0; }
.record-dps-ranking > button:hover { background: var(--ui-theme-surface-hover); }
.record-dps-ranking > button.active { color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); box-shadow: inset 3px 0 var(--ui-color-accent); }
.record-rank-number { color: var(--ui-theme-muted); font-size: 11px; font-weight: 900; }
.record-rank-name { overflow: hidden; font-size: 11px; font-weight: 800; text-overflow: ellipsis; white-space: nowrap; }
.record-rank-performance { display: flex; align-items: baseline; justify-content: flex-end; gap: 8px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.record-rank-performance > span { display: inline-flex; align-items: baseline; gap: 5px; }
.record-rank-performance small { color: var(--ui-theme-muted); font-size: 8px; }
.record-rank-performance strong { font-size: 13px; font-weight: 900; }
.record-rank-performance i { color: var(--ui-theme-muted); font-size: 10px; font-style: normal; }
.record-rank-performance em { color: var(--ui-color-accent); font-size: 10px; font-style: normal; font-weight: 800; }
.record-dps-ranking > button.active .record-rank-performance small,
.record-dps-ranking > button.active .record-rank-performance i,
.record-dps-ranking > button.active .record-rank-performance em { color: var(--ui-theme-on-accent); }

.record-detail-toggle { width: 100%; margin-top: 10px; min-height: 38px; }
.record-detail-toggle.active { color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); border-color: var(--ui-color-accent); }
.record-detail-toggle span { margin-left: auto; color: inherit; opacity: .72; font-size: 9px; }
.record-skill-details { margin-top: 8px; padding: 10px; background: var(--ui-theme-surface); border: 1px solid var(--ui-theme-border-soft); }
.record-skill-details > header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 9px; }
.record-skill-details > header strong { font-size: 11px; }
.record-skill-details > header span { color: var(--ui-theme-muted); font-size: 9px; }
.record-skill-list { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 7px; }
.record-skill-list article { display: grid; grid-template-columns: 40px minmax(0, 1fr); gap: 8px; align-items: start; min-height: 80px; padding: 7px; background: var(--ui-theme-raised); border: 1px solid var(--ui-theme-border-soft); }
.record-skill-list img { width: 40px; height: 40px; object-fit: cover; border: 1px solid var(--ui-theme-border); }
.record-skill-list article > div { min-width: 0; }
.record-skill-list article header { display: flex; align-items: center; gap: 8px; }
.record-skill-list article header strong { overflow: hidden; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.record-skill-head-values { display: inline-flex; align-items: baseline; gap: 8px; margin-left: auto; white-space: nowrap; }
.record-skill-head-values span { color: var(--ui-color-accent); font-size: 10px; font-weight: 900; }
.record-skill-head-values small { color: var(--ui-theme-muted); font-size: 8px; font-weight: 700; }
.record-skill-bar { height: 6px; margin: 4px 0; overflow: hidden; background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); }
.record-skill-bar i { display: block; height: 100%; background: var(--ui-color-accent); }
.record-skill-list small { color: var(--ui-theme-muted); font-size: 8px; }
.record-skill-metrics { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 4px; margin-top: 6px; }
.record-skill-metrics > span { display: grid; gap: 1px; min-width: 0; padding: 3px 5px; background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); }
.record-skill-metrics small { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.record-skill-metrics strong { overflow: hidden; font-size: 9px; font-variant-numeric: tabular-nums; text-overflow: ellipsis; white-space: nowrap; }
.record-skill-empty { padding: 22px; color: var(--ui-theme-muted); text-align: center; font-size: 10px; }
.detail-footer { justify-content: flex-start; }
.detail-footer > span { margin-left: auto; }

.record-delete-confirm-backdrop { position: absolute; z-index: 3; inset: 42px 0 0; display: grid; place-items: center; padding: 20px; background: rgba(0, 0, 0, .68); }
.record-delete-confirm { width: min(520px, 100%); padding: 16px; color: var(--ui-theme-text); background: var(--ui-theme-raised); border: 1px solid var(--record-danger); box-shadow: 0 16px 44px rgba(0, 0, 0, .62); }
.record-delete-confirm > header { display: flex; align-items: flex-start; gap: 10px; }
.record-delete-confirm > header :deep(.v-icon) { color: var(--record-danger); }
.record-delete-confirm > header div { display: grid; gap: 2px; }
.record-delete-confirm > header strong { font-size: 14px; }
.record-delete-confirm > header span,
.record-delete-confirm p { color: var(--ui-theme-muted); font-size: 10px; }
.record-delete-confirm p { margin: 14px 0 7px; }
.record-delete-confirm ul { max-height: 150px; overflow: auto; margin: 0; padding: 8px 8px 8px 25px; background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); font: 9px Consolas, monospace; }
.record-delete-confirm li + li { margin-top: 4px; }
.record-delete-confirm footer { display: flex; justify-content: flex-end; gap: 7px; margin-top: 14px; }

@media (max-width: 760px) {
    .record-browser-backdrop { padding: 8px; }
    .record-browser-dialog { width: calc(100vw - 16px); height: calc(100vh - 16px); }
    .record-list-toolbar { align-items: flex-start; }
    .record-list-toolbar > div:last-of-type { flex-direction: column; }
    .record-col-time { width: 130px; }
    .record-col-date { width: 106px; }
    .record-detail-filters { align-items: stretch; flex-direction: column; }
    .record-detail-filters label { flex-basis: auto; }
    .record-detail-file { max-width: none; margin-left: 0; }
    .record-primary-metrics { grid-template-columns: 1fr; }
    .record-dps-ranking > button { grid-template-columns: 36px minmax(100px, 1fr); gap: 4px; padding: 7px 9px; }
    .record-rank-performance { grid-column: 2; justify-content: flex-start; flex-wrap: wrap; }
    .record-skill-list { grid-template-columns: 1fr; }
}
</style>
