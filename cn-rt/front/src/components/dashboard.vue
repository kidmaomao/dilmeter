<template>
    <!-- ID 混淆設定按鈕 -->
    <div>目前僅支援讀檔案</div>
    <div class="ma-2">
        <v-btn
            @click="openIdMappingDialog"
            color="primary"
            variant="outlined"
            prepend-icon="mdi-incognito"
        >
            ID 混淆設定
        </v-btn>
    </div>

    <!-- ID 混淆設定 Dialog -->
    <v-dialog v-model="idMappingDialog" max-width="800px" scrollable>
        <v-card>
            <v-card-title class="d-flex align-center">
                <v-icon class="mr-2">mdi-incognito</v-icon>
                ID 混淆設定
                <v-spacer></v-spacer>
                <v-btn icon variant="text" @click="idMappingDialog = false">
                    <v-icon>mdi-close</v-icon>
                </v-btn>
            </v-card-title>

            <v-divider></v-divider>

            <v-card-text style="max-height: 500px">
                <v-list density="compact" class="pa-0">
                    <v-list-item
                        v-for="player in tempPlayerMappings"
                        :key="player.id"
                        class="px-2 py-1"
                    >
                        <v-row dense align="center" no-gutters>
                            <v-col cols="5" class="pr-2">
                                <div
                                    class="text-body-2 text-truncate"
                                    :title="player.originalName"
                                >
                                    {{ player.originalName }}
                                </div>
                            </v-col>
                            <v-col cols="1" class="text-center">
                                <v-icon size="small">mdi-arrow-right</v-icon>
                            </v-col>
                            <v-col cols="6">
                                <v-text-field
                                    v-model="player.displayName"
                                    density="compact"
                                    variant="outlined"
                                    hide-details
                                    placeholder="留空表示不改"
                                    clearable
                                ></v-text-field>
                            </v-col>
                        </v-row>
                    </v-list-item>
                </v-list>
                <div
                    v-if="tempPlayerMappings.length === 0"
                    class="text-center text-caption text-disabled py-4"
                >
                    沒有玩家資料
                </div>
            </v-card-text>

            <v-divider></v-divider>

            <v-card-actions>
                <v-spacer></v-spacer>
                <v-btn variant="text" @click="clearIdMappings" color="error">
                    清除全部
                </v-btn>
                <v-btn variant="text" @click="idMappingDialog = false">
                    取消
                </v-btn>
                <v-btn
                    variant="elevated"
                    @click="applyIdMappings"
                    color="primary"
                >
                    套用
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>

    <!-- 原有的 Vuetify 展開面板部分 -->
    <v-select
        v-model="targetId"
        :items="targetIdList"
        :item-title="
            (vv) => {
                const name = vv[0]
                    ? getDisplayName(entityMapWithTargetData[vv[0]]?.actor)
                    : 'all';
                const time = vv[0] ? renderTime(vv[0]) : '';
                return time ? `${name} ${time}` : name;
            }
        "
        :item-value="(vv) => vv[0]"
        class="ma-2"
        variant="outlined"
        density="compact"
        hide-details
    >
    </v-select>

    <!-- ========== 統計分析區塊 ========== -->
    <div v-if="compareData.length > 0" class="statistics-section">
        <!-- 標題列 -->
        <div class="d-flex align-center my-4">
            <span class="text-h6 mr-4">統計分析</span>
            <v-divider />
        </div>

        <!-- Loading 狀態 -->
        <div v-if="isChartLoading" class="loading-container">
            <v-progress-circular indeterminate :size="40" color="primary" />
            <p class="mt-4" style="font-size: 16px; color: #909399">
                正在處理資料...
            </p>
        </div>

        <!-- 圖表內容 -->
        <div v-else-if="isChartDataReady">
            <!-- Row 1: 玩家總體統計 & 玩家輸出佔比圓餅圖 -->
            <v-row>
                <!-- Table 1: 玩家總體輸出統計 -->
                <v-col cols="7">
                    <v-card>
                        <v-card-title>玩家總體輸出統計</v-card-title>
                        <v-data-table
                            :headers="playerTableHeaders"
                            :items="playerStats"
                            density="compact"
                            :items-per-page="-1"
                            hide-default-footer
                            hover
                            @click:row="(_: any, { item }: { item: any }) => handlePlayerSelect(item)"
                            :row-props="({ item }: { item: any }) => selectedPlayer === item.name ? { class: 'selected-row' } : {}"
                        >
                            <template #item.job="{ item }">
                                <v-chip size="small" color="grey">{{ item.job }}</v-chip>
                            </template>
                            <template #item.totalDamage="{ item }">
                                {{ Math.floor(item.totalDamage).toLocaleString() }}
                            </template>
                            <template #item.percentage="{ item }">
                                {{ item.percentage }}%
                            </template>
                            <template #item.critRate="{ item }">
                                <span
                                    :class="{
                                        'text-success': parseFloat(item.critRate) >= 80,
                                    }"
                                >
                                    {{ item.critRate }}%
                                </span>
                            </template>
                            <template #item.avgDamage="{ item }">
                                {{ Math.floor(item.avgDamage).toLocaleString() }}
                            </template>
                        </v-data-table>
                    </v-card>
                </v-col>

                <!-- Pie Chart 1: 玩家輸出佔比 -->
                <v-col cols="5">
                    <v-card>
                        <v-card-title>輸出佔比</v-card-title>
                        <div
                            ref="playerPieChartRef"
                            style="width: 100%; height: 400px"
                        ></div>
                    </v-card>
                </v-col>
            </v-row>

            <!-- Row 2: 技能詳細統計 & 技能輸出佔比圓餅圖 -->
            <v-row class="mt-5">
                <!-- Table 2: 技能詳細統計 (含玩家切換 Tabs) -->
                <v-col cols="7">
                    <v-card>
                        <v-card-title class="d-flex align-center" style="gap: 8px">
                            技能詳細統計
                            <v-chip
                                v-if="selectedPlayer"
                                color="primary"
                                size="small"
                            >
                                {{ selectedPlayer }}
                            </v-chip>
                        </v-card-title>

                        <!-- 玩家切換 Tabs -->
                        <v-tabs
                            v-model="selectedPlayerTab"
                            density="compact"
                        >
                            <v-tab
                                v-for="player in playerStats"
                                :key="player.name"
                                :value="player.name"
                            >
                                {{ player.name }}
                            </v-tab>
                        </v-tabs>

                        <v-data-table
                            v-if="selectedPlayer"
                            :headers="skillTableHeaders"
                            :items="selectedSkillStats"
                            density="compact"
                            :items-per-page="-1"
                            hide-default-footer
                            height="500"
                            fixed-header
                        >
                            <template #item.name="{ item }">
                                <div class="d-flex align-center">
                                    <img
                                        :src="`https://cdn.jsdelivr.net/gh/eldisa/mabinogiImage@main/SkillImage/${item.id}.png`"
                                        width="32"
                                        height="32"
                                        class="mr-2"
                                        @error="
                                            (e: any) =>
                                                (e.target.style.display = 'none')
                                        "
                                    />
                                    <span>{{ item.name }}</span>
                                </div>
                            </template>
                            <template #item.critRate="{ item }">
                                <span
                                    v-if="item.canCrit === false"
                                    class="text-disabled"
                                    style="font-style: italic"
                                >
                                    {{ item.critRate }}
                                </span>
                                <span
                                    v-else
                                    :class="{
                                        'text-success':
                                            parseFloat(item.critRate) >= 80,
                                    }"
                                >
                                    {{ item.critRate }}%
                                </span>
                            </template>
                            <template #item.ccRate="{ item }">
                                <span
                                    :class="{
                                        'text-success':
                                            parseFloat(item.ccRate) >= 90,
                                    }"
                                >
                                    {{ item.ccRate }}%
                                </span>
                            </template>
                            <template #item.maxDamage="{ item }">
                                {{ Math.floor(item.maxDamage).toLocaleString() }}
                            </template>
                            <template #item.totalDamage="{ item }">
                                {{ Math.floor(item.totalDamage).toLocaleString() }}
                            </template>
                            <template #item.percentage="{ item }">
                                {{ item.percentage }}%
                            </template>
                        </v-data-table>
                        <div
                            v-else
                            class="text-center text-disabled pa-8"
                        >
                            請選擇玩家
                        </div>
                    </v-card>
                </v-col>

                <!-- Pie Chart 2: 技能輸出佔比 -->
                <v-col cols="5">
                    <v-card>
                        <v-card-title class="d-flex align-center" style="gap: 8px">
                            技能輸出佔比
                            <v-chip
                                v-if="selectedPlayer"
                                color="primary"
                                size="small"
                            >
                                {{ selectedPlayer }}
                            </v-chip>
                        </v-card-title>
                        <div
                            ref="skillPieChartRef"
                            style="width: 100%; height: 400px"
                        ></div>
                        <div
                            v-if="!selectedPlayer"
                            class="text-center text-disabled pa-8"
                        >
                            請選擇玩家
                        </div>
                    </v-card>
                </v-col>
            </v-row>
        </div>

        <!-- 無資料狀態 -->
        <div v-else class="text-center text-disabled pa-12">無統計資料</div>
    </div>
</template>

<script setup lang="ts">
import {
    inject,
    ref,
    computed,
    onUnmounted,
    watch,
    onMounted,
    nextTick,
    type Ref,
} from "vue";
import highcharts from "highcharts";
import type { Options } from "highcharts";

// Highcharts 12.x 需要明確設定 locale，否則 Intl.NumberFormat("") 會拋出
// RangeError: Invalid language tag:
highcharts.setOptions({
    lang: { locale: navigator.language || "zh-TW" },
});
import {
    type EntityDamage,
    ActorManager,
    type BaseActor,
    GroupActor,
    type EntityActor,
} from "@/eventActor";
import {
    type DamageCollectorBase,
    type DualGroupedDamageCollector,
    type GroupedDamageCollector,
} from "@/actionCollector";

// ===== Types =====
type EntityExtended = {
    actor: EntityActor;
    dc: DualGroupedDamageCollector;
    totalDamage: number;
    damages: EntityDamage[];
    groupedTotalDamages: Record<string, number>;
    groupedDamages: Record<string, EntityDamage[]>;
    groupedMinDamages: Record<string, number>;
    groupedMaxDamages: Record<string, number>;
    groupedCount: Record<string, number>;
};

// ===== Inject =====
const region = inject<Ref<string>>("region")!;
const raceNameMap = inject<Ref<Record<number, string>>>("raceNameMap")!;
const skillNameMap = inject<Ref<Record<number, string>>>("skillNameMap")!;
const appEvent = inject<Ref<EventTarget>>("appEvent")!;
const actorManager = inject<Ref<ActorManager>>("actorManager")!;
const dcManager = inject<Ref<any>>("dcManager")!;

// ===== Data =====
const damageCollectorMap: Record<string, DamageCollectorBase> = {};
const targetId = ref("");

// ID 混淆功能
const idMappings = ref<Record<string, string>>({});
const idMappingDialog = ref(false);

interface PlayerMapping {
    id: string;
    originalName: string;
    displayName: string;
}
const tempPlayerMappings = ref<PlayerMapping[]>([]);

// 統計分析相關
const selectedPlayer = ref<string>("");
const selectedPlayerTab = ref<string>("");
const playerPieChartRef = ref<HTMLElement>();
const skillPieChartRef = ref<HTMLElement>();
let playerChart: ReturnType<typeof highcharts.chart> | null = null;
let skillChart: ReturnType<typeof highcharts.chart> | null = null;

const isChartDataReady = ref(false);
const isChartLoading = ref(false);

// ===== Table Headers =====
const playerTableHeaders = [
    { title: "玩家", key: "name", width: 150 },
    { title: "職業", key: "job", align: "center" as const, width: 80 },
    { title: "總傷害", key: "totalDamage", align: "end" as const },
    { title: "輸出佔比", key: "percentage", align: "end" as const, width: 100 },
    { title: "次數", key: "totalCount", align: "end" as const, width: 80 },
    { title: "爆擊率", key: "critRate", align: "end" as const, width: 100 },
    { title: "平均傷害", key: "avgDamage", align: "end" as const, width: 120 },
];

const skillTableHeaders = [
    { title: "技能", key: "name", width: 180 },
    { title: "次數", key: "count", align: "end" as const, width: 80 },
    { title: "爆擊率", key: "critRate", align: "end" as const, width: 90 },
    { title: "CC覆蓋率", key: "ccRate", align: "end" as const, width: 100 },
    { title: "最大傷害", key: "maxDamage", align: "end" as const, width: 120 },
    { title: "總傷害", key: "totalDamage", align: "end" as const, width: 140 },
    { title: "輸出佔比", key: "percentage", align: "end" as const, width: 80 },
];

// ===== Highcharts Pie Options =====
const buildPlayerPieOptions = (): Options => ({
    chart: { type: "pie", backgroundColor: "transparent" },
    title: { text: "" },
    credits: { enabled: false },
    tooltip: {
        formatter: function () {
            const pt = this as any;
            return `<b>${pt.point.name}</b><br/>${pt.percentage?.toFixed(1)}%<br/>傷害: ${Math.floor(pt.y ?? 0).toLocaleString()}`;
        },
    },
    legend: {
        enabled: true,
        align: "left",
        verticalAlign: "middle",
        layout: "vertical",
        itemStyle: { color: "#b0b0b0", fontWeight: "normal" },
        labelFormatter: function () {
            const pt = this as any;
            return `${pt.name}: ${pt.percentage?.toFixed(1)}%`;
        },
    },
    plotOptions: {
        pie: {
            allowPointSelect: true,
            cursor: "pointer",
            innerSize: "40%",
            center: ["65%", "50%"],
            dataLabels: {
                enabled: true,
                format: "{point.percentage:.1f}%",
                style: { color: "#e0e0e0", textOutline: "none" },
            },
            showInLegend: true,
        },
    },
    series: [
        {
            type: "pie",
            name: "輸出佔比",
            colorByPoint: true,
            data: playerStats.value.map((p) => ({
                name: p.name,
                y: p.totalDamage,
            })),
        } as any,
    ],
});

const buildSkillPieOptions = (): Options => ({
    chart: { type: "pie", backgroundColor: "transparent" },
    title: { text: "" },
    credits: { enabled: false },
    tooltip: {
        formatter: function () {
            const pt = this as any;
            return `<b>${pt.point.name}</b><br/>${pt.percentage?.toFixed(1)}%<br/>傷害: ${Math.floor(pt.y ?? 0).toLocaleString()}`;
        },
    },
    legend: {
        enabled: true,
        align: "left",
        verticalAlign: "middle",
        layout: "vertical",
        itemStyle: { color: "#b0b0b0", fontWeight: "normal" },
        labelFormatter: function () {
            const pt = this as any;
            return `${pt.name}: ${pt.percentage?.toFixed(1)}%`;
        },
    },
    plotOptions: {
        pie: {
            allowPointSelect: true,
            cursor: "pointer",
            innerSize: "40%",
            center: ["65%", "50%"],
            dataLabels: {
                enabled: true,
                format: "{point.percentage:.1f}%",
                style: { color: "#e0e0e0", textOutline: "none" },
            },
            showInLegend: true,
        },
    },
    series: [
        {
            type: "pie",
            name: "技能輸出",
            colorByPoint: true,
            data: selectedSkillStats.value.map((s) => ({
                name: s.name,
                y: s.totalDamage,
            })),
        } as any,
    ],
});

// ===== Functions =====
const getDisplayName = (actor: EntityActor | BaseActor | undefined): string => {
    if (!actor) return "Unknown";
    const originalName = prettyEntityName(actor);
    if (!originalName) return "Unknown";
    const mappedName = idMappings.value[originalName];
    if (mappedName) return mappedName;
    return originalName;
};

const openIdMappingDialog = () => {
    initTempPlayerMappings();
    idMappingDialog.value = true;
};

const initTempPlayerMappings = () => {
    tempPlayerMappings.value = pcEntities.value.map((entity) => {
        const originalName = prettyEntityName(entity.actor) || "Unknown";
        return {
            id: entity.actor.id,
            originalName,
            displayName: idMappings.value[originalName] || "",
        };
    });
};

const applyIdMappings = () => {
    const newMappings: Record<string, string> = {};
    tempPlayerMappings.value.forEach((player) => {
        if (player.displayName && player.displayName.trim()) {
            newMappings[player.originalName] = player.displayName.trim();
        }
    });
    idMappings.value = newMappings;
    idMappingDialog.value = false;
    console.log("✅ ID 混淆已套用:", idMappings.value);

    nextTick(() => {
        if (playerStats.value.length > 0) {
            const firstPlayerName = playerStats.value[0].name;
            selectedPlayer.value = firstPlayerName;
            selectedPlayerTab.value = firstPlayerName;
            updatePlayerPieChart();
            updateSkillPieChart();
        }
    });
};

const clearIdMappings = () => {
    tempPlayerMappings.value.forEach((player) => {
        player.displayName = "";
    });
    idMappings.value = {};
    console.log("🗑️ ID 混淆已清除");

    nextTick(() => {
        if (playerStats.value.length > 0) {
            const firstPlayerName = playerStats.value[0].name;
            selectedPlayer.value = firstPlayerName;
            selectedPlayerTab.value = firstPlayerName;
            updatePlayerPieChart();
            updateSkillPieChart();
        }
    });
};

const getDC = (attackerId: string): DualGroupedDamageCollector => {
    const key = `${attackerId}`;
    if (damageCollectorMap[key]) {
        return damageCollectorMap[key] as DualGroupedDamageCollector;
    }
    const dc = dcManager.value.getDualGroupedDamageCollector(
        (v: any) => v.Id == attackerId,
        (v: any) => v.TargetId,
        (v: any) => `${v.SkillId}`,
    );
    damageCollectorMap[key] = dc;
    return dc;
};

const getTargetDC = (): GroupedDamageCollector => {
    const key = `target`;
    if (damageCollectorMap[key]) {
        return damageCollectorMap[key] as GroupedDamageCollector;
    }
    const dc = dcManager.value.getGroupedDamageCollector(
        () => true,
        (v: any) => v.TargetId,
    );
    damageCollectorMap[key] = dc;
    return dc;
};

const prettyEntityName = (entity?: BaseActor): string | undefined => {
    if (!entity) return undefined;
    if (ActorManager.pcRaceSet.has(entity.raceId)) return entity.name;
    const raceName =
        raceNameMap.value[entity.raceId] || `unknownRace:${entity.raceId}`;
    if (entity instanceof GroupActor) return raceName;
    if (entity.name[0] >= "0" && entity.name[0] <= "9") {
        return `${raceName} (${entity.name.slice(-4)})`;
    }
    return "[寵物] " + entity.name;
};

const updateSkillData = (
    skillMap: Map<number, any>,
    skillId: number,
    damage: number,
    isCrit: boolean,
    isCC: boolean,
) => {
    let detail = skillMap.get(skillId);
    if (!detail) {
        const config = skillConfig[skillId];
        detail = {
            id: skillId,
            count: 0,
            critCount: 0,
            minDamage: Number.MAX_SAFE_INTEGER,
            maxDamage: 0,
            totalDamage: 0,
            ccCoverCount: 0,
            canCrit: config?.canCrit !== false,
        };
        skillMap.set(skillId, detail);
    }
    detail.count++;
    detail.totalDamage += damage;
    if (isCrit) detail.critCount++;
    if (isCC) detail.ccCoverCount++;
    if (damage > detail.maxDamage) detail.maxDamage = damage;
    if (damage < detail.minDamage) detail.minDamage = damage;
};

const clearTarget = () => {
    targetId.value = "";
};

const renderTime = (actorId: string) => {
    if (!actorId || actorId === "") {
        const allDamages = Object.values(entityMap.value)
            .filter((v) => v.actor.isPC)
            .flatMap((v) => v.dc.damages)
            .sort((a, b) => a.At - b.At);
        if (allDamages.length < 2) return "";
        const totalSeconds =
            allDamages[allDamages.length - 1].At - allDamages[0].At;
        const minutes = Math.floor(totalSeconds / 60).toString().padStart(2, "0");
        const seconds = Math.floor(totalSeconds % 60).toString().padStart(2, "0");
        const startTime = new Date(
            allDamages[0].At * 1000 - (allDamages[0].At % 60) * 1000,
        ).toLocaleDateString("zh-TW", {
            hour: "2-digit",
            minute: "2-digit",
            second: "2-digit",
        });
        return `${startTime} ${minutes} 分 ${seconds} 秒`;
    }
    const entity = entityMap.value[actorId];
    if (!entity || !entity.dc) return "";
    const damages = entity.dc.damages;
    if (damages.length < 2) return "";
    const totalSeconds = damages[damages.length - 1].At - damages[0].At;
    const minutes = Math.floor(totalSeconds / 60).toString().padStart(2, "0");
    const seconds = Math.floor(totalSeconds % 60).toString().padStart(2, "0");
    const startTime = new Date(
        damages[0].At * 1000 - (damages[0].At % 60) * 1000,
    ).toLocaleDateString("zh-TW", {
        hour: "2-digit",
        minute: "2-digit",
        second: "2-digit",
    });
    return `${startTime} ${minutes} 分 ${seconds} 秒`;
};

// ===== Chart Functions =====
const initializeCharts = () => {
    if (!compareData.value.length || !playerStats.value.length) return;

    selectedPlayer.value = playerStats.value[0].name || "";
    selectedPlayerTab.value = playerStats.value[0].name || "";

    if (playerPieChartRef.value) initPlayerPieChart();
    if (skillPieChartRef.value) initSkillPieChart();
};

const handlePlayerSelect = (row: any) => {
    if (row) {
        selectedPlayer.value = row.name;
        selectedPlayerTab.value = row.name;
    }
};

const handleTabChange = (tabName: string | number) => {
    selectedPlayer.value = tabName as string;
};

const initPlayerPieChart = () => {
    if (!playerPieChartRef.value) return;
    if (playerChart) { playerChart.destroy(); playerChart = null; }
    playerChart = highcharts.chart(playerPieChartRef.value, buildPlayerPieOptions());
};

const updatePlayerPieChart = () => {
    if (!playerPieChartRef.value) return;
    const data = playerStats.value.map((p) => ({ name: p.name, y: p.totalDamage }));
    if (playerChart && playerChart.series?.[0]) {
        playerChart.series[0].setData(data, true);
    } else {
        if (playerChart) { playerChart.destroy(); playerChart = null; }
        playerChart = highcharts.chart(playerPieChartRef.value, buildPlayerPieOptions());
    }
};

const initSkillPieChart = () => {
    if (!skillPieChartRef.value) return;
    if (skillChart) { skillChart.destroy(); skillChart = null; }
    skillChart = highcharts.chart(skillPieChartRef.value, buildSkillPieOptions());
};

const updateSkillPieChart = () => {
    if (!skillPieChartRef.value) return;
    const data = selectedSkillStats.value.map((s) => ({ name: s.name, y: s.totalDamage }));
    if (skillChart && skillChart.series?.[0]) {
        skillChart.series[0].setData(data, true);
    } else {
        if (skillChart) { skillChart.destroy(); skillChart = null; }
        skillChart = highcharts.chart(skillPieChartRef.value, buildSkillPieOptions());
    }
};

// ===== Computed =====
const targetDC = ref(getTargetDC());

const entityMap = computed(() => {
    const m: Record<
        string,
        { actor: EntityActor; dc: DualGroupedDamageCollector }
    > = {};
    for (const k in actorManager.value.entityMap) {
        const v = actorManager.value.entityMap[k];
        m[k] = { actor: v, dc: getDC(k) };
    }
    return m;
});

const entityMapWithTargetData = computed(() => {
    const m: Record<string, EntityExtended> = {};
    for (const k in entityMap.value) {
        const v = entityMap.value[k];
        const totalDamage = targetId.value
            ? v.dc.groupedTotalDamages[targetId.value] || 0
            : v.dc.totalDamage;
        const damages = targetId.value
            ? v.dc.groupedDamages[targetId.value] || []
            : v.dc.damages;
        const groupedTotalDamages = targetId.value
            ? v.dc.dualGroupedTotalDamages[targetId.value]
            : v.dc.grouped2TotalDamages;
        const groupedDamages = targetId.value
            ? v.dc.dualGroupedDamages[targetId.value]
            : v.dc.grouped2Damages;
        const groupedMinDamages = targetId.value
            ? v.dc.dualGroupedMinDamages[targetId.value]
            : v.dc.grouped2MinDamages;
        const groupedMaxDamages = targetId.value
            ? v.dc.dualGroupedMaxDamages[targetId.value]
            : v.dc.grouped2MaxDamages;
        const groupedCount = targetId.value
            ? v.dc.dualGroupedCount[targetId.value]
            : v.dc.grouped2Count;
        m[k] = {
            ...v,
            totalDamage,
            damages,
            groupedTotalDamages,
            groupedDamages,
            groupedMinDamages,
            groupedMaxDamages,
            groupedCount,
        };
    }
    return m;
});

const pcEntities = computed(() =>
    Object.values(entityMapWithTargetData.value)
        .filter((v) => v.actor.isPC && v.totalDamage > 10000)
        .sort((a, b) => b.totalDamage - a.totalDamage),
);

// 技能配置（標記不會暴擊的技能）
const skillConfig: Record<number, { name: string; canCrit: boolean }> = {
    58009: { name: "被動技能1", canCrit: false },
    58100: { name: "被動技能2", canCrit: false },
    58101: { name: "被動技能3", canCrit: false },
};

const commonSkillIds = [58009, 58100, 58101];

interface JobConfig {
    name: string;
    skillIds: number[];
    conditionId?: number;
    targetConditionId?: number;
}

const jobConfigs: Record<string, JobConfig> = {
    archer: {
        name: "流星射手",
        skillIds: [21002, 59060, 59061, 59064, ...commonSkillIds],
        conditionId: 887,
    },
    magicSword: {
        name: "元素骑士",
        skillIds: [20002, 20018, 20019, 59023, 59024, 59025, 59026, 59028, ...commonSkillIds],
        targetConditionId: 323,
    },
    darkMage: {
        name: "黑魔导士",
        skillIds: [30102, 30202, 30205, 30452, 59040, 59041, 59042, ...commonSkillIds],
        targetConditionId: 948,
        conditionId: 938,
    },
    lancer: {
        name: "爆裂骑士枪",
        skillIds: [20017, 59100, 59101, 59102, 59103, 59104, 59105, 59106, ...commonSkillIds],
        conditionId: 975,
        targetConditionId: 323,
    },
    shieldWarrior: {
        name: "圣盾骑士",
        skillIds: [59080, 59081, 59082, 59083, 59084, 59085, 59086, ...commonSkillIds],
        targetConditionId: 323,
    },
    bard: {
        name: "圣光颂唱者",
        skillIds: [59004, ...commonSkillIds],
    },
    gun: {
        name: "枪炮师",
        skillIds: [35127, 54302, 54303, 54304, 54305, 54306, 54307, 59120, 59121, 59122, 59123, 59124, ...commonSkillIds],
        targetConditionId: 1122,
        conditionId: 1123,
    },
    alchemist: {
        name: "禁术炼金师",
        skillIds: [35002, 35004, 35007, 35008, 35009, 35014, 35024, 35123, 59143, 59144, 59145, ...commonSkillIds],
        targetConditionId: 1147,
        conditionId: 1146,
    },
};

const detectPlayerJob = (damages: any[]): JobConfig | null => {
    if (!damages || damages.length === 0) return null;
    const usedSkillIds = new Set(damages.map((d) => d.SkillId));
    for (const [, config] of Object.entries(jobConfigs)) {
        const specialSkills = config.skillIds.filter((id) => id >= 59000);
        if (specialSkills.some((skillId) => usedSkillIds.has(skillId))) {
            return config;
        }
    }
    let maxMatchCount = 0;
    let bestMatch: JobConfig | null = null;
    for (const [, config] of Object.entries(jobConfigs)) {
        const matchCount = config.skillIds.filter((skillId) =>
            usedSkillIds.has(skillId),
        ).length;
        if (matchCount > maxMatchCount) {
            maxMatchCount = matchCount;
            bestMatch = config;
        }
    }
    return bestMatch;
};

const compareData = computed(() => {
    return pcEntities.value.map((v) => {
        const { actor, totalDamage, damages } = v;
        const detectedJob = detectPlayerJob(damages);
        const jobConfig = detectedJob || jobConfigs.archer;
        const targetSkillIdArray = jobConfig.skillIds;
        const targetCCId = jobConfig.conditionId;
        const targetConditionId = jobConfig.targetConditionId;
        const skillMap = new Map();

        damages.forEach((detail) => {
            const { SkillId, IsCritical, Conditions, TargetConditions, Damage } = detail;
            if (!targetSkillIdArray.includes(SkillId)) return;

            let ccCovering = false;
            if (targetCCId && Conditions) {
                ccCovering = Conditions.some((c) => c.CCId == targetCCId);
            }
            if (!ccCovering && targetConditionId && TargetConditions) {
                ccCovering = TargetConditions.some((c) => c.CCId == targetConditionId);
            }

            updateSkillData(skillMap, SkillId, Damage, IsCritical, !!ccCovering);
        });

        return {
            pilotName: {
                id: getDisplayName(actor),
                nickname: getDisplayName(actor),
            },
            totalDamage,
            skillMap,
            detectedJob: detectedJob?.name || "未知",
        };
    });
});

const targetIdList = computed(() => {
    const entries = Object.entries(targetDC.value.groupedTotalDamages).filter(
        ([, damage]) => damage > 100000000,
    );
    const sorted = entries.sort(([idA], [idB]) => {
        const entityA = entityMap.value[idA];
        const entityB = entityMap.value[idB];
        if (!entityA?.dc?.damages?.length) return 1;
        if (!entityB?.dc?.damages?.length) return -1;
        const timeA = entityA.dc.damages[0]?.At || 0;
        const timeB = entityB.dc.damages[0]?.At || 0;
        return timeA - timeB;
    });
    sorted.unshift(["", targetDC.value.totalDamage]);
    return sorted;
});

const playerStats = computed(() => {
    const totalAllDamage = compareData.value.reduce((sum, p) => {
        const playerTotal = Array.from(p.skillMap.values()).reduce(
            (s: number, skill: any) => s + skill.totalDamage,
            0,
        );
        return sum + playerTotal;
    }, 0);

    return compareData.value
        .map((player) => {
            const skillsArray = Array.from(player.skillMap.values()) as any[];
            const totalDamage = skillsArray.reduce((sum, skill) => sum + skill.totalDamage, 0);
            const totalCount = skillsArray.reduce((sum, skill) => sum + skill.count, 0);
            const totalCrit = skillsArray.reduce((sum, skill) => sum + skill.critCount, 0);
            const critableSkills = skillsArray.filter((skill) => skill.canCrit !== false);
            const critableCount = critableSkills.reduce((sum, skill) => sum + skill.count, 0);
            const critableCritCount = critableSkills.reduce((sum, skill) => sum + skill.critCount, 0);

            return {
                name: player.pilotName.nickname,
                job: player.detectedJob || "未知",
                totalDamage,
                totalCount,
                critCount: totalCrit,
                critRate:
                    critableCount > 0
                        ? ((critableCritCount / critableCount) * 100).toFixed(1)
                        : "0.0",
                avgDamage: totalCount > 0 ? Math.round(totalDamage / totalCount) : 0,
                percentage:
                    totalAllDamage > 0
                        ? ((totalDamage / totalAllDamage) * 100).toFixed(1)
                        : "0.0",
            };
        })
        .sort((a, b) => b.totalDamage - a.totalDamage);
});

const selectedSkillStats = computed(() => {
    const playerName = selectedPlayerTab.value || selectedPlayer.value;
    if (!playerName) return [];
    const player = compareData.value.find((p) => p.pilotName.nickname === playerName);
    if (!player) return [];
    const skillsArray = Array.from(player.skillMap.values()) as any[];
    const totalDamage = skillsArray.reduce((sum, skill) => sum + skill.totalDamage, 0);

    return skillsArray
        .map((skill) => ({
            id: skill.id,
            name: skillNameMap.value[skill.id] || `未知技能:${skill.id}`,
            count: skill.count,
            critCount: skill.critCount,
            critRate:
                skill.canCrit === false
                    ? "-"
                    : skill.count > 0
                      ? ((skill.critCount / skill.count) * 100).toFixed(1)
                      : "0.0",
            ccRate:
                skill.count > 0
                    ? ((skill.ccCoverCount / skill.count) * 100).toFixed(1)
                    : "0.0",
            minDamage:
                skill.minDamage === Number.MAX_SAFE_INTEGER ? 0 : skill.minDamage,
            maxDamage: skill.maxDamage,
            totalDamage: skill.totalDamage,
            avgDamage: skill.count > 0 ? Math.round(skill.totalDamage / skill.count) : 0,
            percentage:
                totalDamage > 0
                    ? ((skill.totalDamage / totalDamage) * 100).toFixed(1)
                    : "0.0",
            canCrit: skill.canCrit,
        }))
        .sort((a, b) => b.totalDamage - a.totalDamage);
});

// ===== Watchers =====
watch(
    () => compareData.value,
    (newVal, oldVal) => {
        if (oldVal === undefined || oldVal.length === 0) {
            isChartLoading.value = true;
        }
        nextTick(() => {
            isChartLoading.value = false;
            isChartDataReady.value = newVal.length > 0;
        });
    },
    { immediate: true },
);

// flush: 'post' 確保 Vue 已完成 DOM 更新（v-else-if 區塊已渲染）才初始化圖表
watch(isChartDataReady, (ready) => {
    if (ready && !isChartLoading.value) {
        initializeCharts();
    }
}, { flush: "post" });

watch(selectedPlayerTab, (newVal) => {
    selectedPlayer.value = newVal;
    if (newVal) {
        updateSkillPieChart();
    }
}, { flush: "post" });

// ===== Lifecycle =====
onMounted(() => {
    appEvent.value.addEventListener("clear", clearTarget);
    window.addEventListener("resize", () => {
        playerChart?.reflow();
        skillChart?.reflow();
    });
});

onUnmounted(() => {
    appEvent.value.removeEventListener("clear", clearTarget);
    for (const v of Object.values(damageCollectorMap)) {
        dcManager.value.removeDamageCollector(v);
    }
    if (playerChart) { playerChart.destroy(); playerChart = null; }
    if (skillChart) { skillChart.destroy(); skillChart = null; }
});
</script>

<style scoped>
/* ===== Dark Mode 主題配色 ===== */
:deep(.v-select) {
    background-color: #1e1e1e;
    color: #e0e0e0;
}

:deep(.v-select .v-field) {
    background-color: #2d2d2d;
    color: #e0e0e0;
}

:deep(.v-select .v-field__input) {
    color: #e0e0e0;
}

:deep(.v-expansion-panel) {
    background-color: #2d2d2d !important;
    color: #e0e0e0 !important;
}

:deep(.v-expansion-panel-title) {
    background-color: #2d2d2d !important;
    color: #e0e0e0 !important;
}

:deep(.v-expansion-panel-text) {
    background-color: #1e1e1e !important;
    color: #e0e0e0 !important;
}

:deep(.v-sheet) {
    background-color: transparent !important;
    color: #e0e0e0 !important;
}

:deep(.v-dialog .v-card) {
    background-color: #2d2d2d !important;
    color: #e0e0e0 !important;
}

/* ===== 統計分析區塊 ===== */
.statistics-section {
    margin-top: 40px;
    padding: 20px;
    background: #1e1e1e;
    border-radius: 8px;
}

.loading-container {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    min-height: 400px;
    padding: 40px;
    background-color: #2d2d2d;
}

/* 選中行高亮 */
:deep(.selected-row td) {
    background-color: #404040 !important;
}

.text-success {
    color: #67c23a;
    font-weight: bold;
}

/* Vuetify 部分的樣式 */
.row {
    display: flex;
    padding: 10px;
    border-bottom: 1px solid #404040;
    position: relative;
    cursor: pointer;
    background-color: #2d2d2d;
}

.row:hover {
    background-color: #3a3a3a;
}

.bg-bar {
    position: absolute;
    top: 0;
    left: 0;
    bottom: 0;
    background-color: rgba(64, 158, 255, 0.3);
    z-index: 0;
    transition: width 0.5s;
}

.content {
    display: flex;
    width: 100%;
    position: relative;
    z-index: 1;
    justify-content: space-between;
    color: #e0e0e0;
}

.detail-row {
    background-color: #1e1e1e;
    padding: 10px 20px;
}

.detail-row table {
    width: 100%;
    font-size: 0.9em;
    color: #b0b0b0;
}

.detail-row td {
    padding: 5px;
    color: #b0b0b0;
}
</style>
