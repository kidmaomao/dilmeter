<template>
    <div>
        <!-- 控制列 -->
        <div class="d-flex align-center px-1 mb-1" style="gap: 8px">
            <span class="text-caption text-disabled">時間粒度</span>
            <v-select
                v-model="tickSize"
                :items="tickOptions"
                item-title="label"
                item-value="value"
                variant="outlined"
                density="compact"
                hide-details
                style="max-width: 100px"
            />
        </div>

        <!-- 圖表 -->
        <div ref="chartDom" style="height: 220px" />

        <!-- 玩家 toggle -->
        <div class="d-flex flex-wrap mt-1 px-1" style="gap: 0 4px">
            <v-checkbox
                v-for="(e, idx) in entities"
                :key="e.entityId"
                v-model="enabledIds"
                :value="e.entityId"
                density="compact"
                hide-details
                class="flex-0-0"
                :color="palette[idx % palette.length]"
            >
                <template #label>
                    <span
                        class="mr-1"
                        style="display: inline-block; width: 10px; height: 10px; border-radius: 2px; flex-shrink: 0"
                        :style="{ background: palette[idx % palette.length] }"
                    />
                    <span class="text-body-2">{{ e.displayName }}</span>
                </template>
            </v-checkbox>
        </div>
    </div>
</template>

<script lang="ts">
import { defineComponent, ref, watch, onMounted, onUnmounted, computed, type PropType } from "vue";
import highcharts from "highcharts";
import type { Options, SeriesOptionsType } from "highcharts";
import type { EntityDamage } from "@/eventActor";

export type DpsChartEntity = {
    entityId: string;
    displayName: string;
    damages: EntityDamage[];
};

export default defineComponent({
    name: "SummaryDpsChart",
    props: {
        entities: {
            type: Array as PropType<DpsChartEntity[]>,
            required: true,
        },
        /** 戰鬥開始/結束時間（秒），用於對齊 X 軸 */
        startAt: { type: Number, required: true },
        endAt:   { type: Number, required: true },
    },
    setup(props) {
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart | undefined;

        const TICK_KEY = "summaryDpsChart.tickSize";
        const tickSize = ref(Number(localStorage.getItem(TICK_KEY)) || 15);
        const tickOptions = [
            { label: "5s",   value: 5   },
            { label: "10s",  value: 10  },
            { label: "15s",  value: 15  },
            { label: "30s",  value: 30  },
            { label: "1min", value: 60  },
            { label: "2min", value: 120 },
        ];

        // 預設全部開啟
        const enabledIds = ref<string[]>([]);
        watch(
            () => props.entities,
            (es) => {
                // 新增玩家時預設勾選；移除的玩家也同步清掉
                const ids = es.map((e) => e.entityId);
                enabledIds.value = ids.filter(
                    (id) => enabledIds.value.includes(id) || !enabledIds.value.length,
                );
                if (!enabledIds.value.length) enabledIds.value = [...ids];
            },
            { immediate: true },
        );

        // ── 資料計算 ────────────────────────────────────────────────

        const buildSeries = (): SeriesOptionsType[] => {
            const tick  = tickSize.value;
            const start = props.startAt;
            const end   = props.endAt;

            return props.entities
                .map((e, idx): SeriesOptionsType | null => {
                    if (!enabledIds.value.includes(e.entityId)) return null;

                    const bins = new Map<number, number>();
                    for (const d of e.damages) {
                        if (d.At < start || d.At > end) continue;
                        const bt = Math.floor(d.At / tick) * tick;
                        bins.set(bt, (bins.get(bt) ?? 0) + d.Damage);
                    }

                    const data: [number, number][] = [];
                    const firstBin = Math.floor(start / tick) * tick;
                    const lastBin  = Math.floor(end   / tick) * tick;
                    for (let t = firstBin; t <= lastBin; t += tick) {
                        data.push([t * 1000, Math.round((bins.get(t) ?? 0) / tick)]);
                    }

                    const color = PALETTE[idx % PALETTE.length];
                    return {
                        type: "area",
                        name: e.displayName,
                        data,
                        color,
                        fillColor: {
                            linearGradient: { x1: 0, y1: 0, x2: 0, y2: 1 },
                            stops: [
                                [0, highcharts.color(color).setOpacity(0.3).get() as string],
                                [1, highcharts.color(color).setOpacity(0.02).get() as string],
                            ],
                        },
                        lineWidth: 1.5,
                        marker: { enabled: false },
                        threshold: null,
                    };
                })
                .filter((s): s is SeriesOptionsType => s !== null);
        };

        const buildOptions = (): Options => ({
            title:   { text: "" },
            credits: { enabled: false },
            time:    { timezone: Intl.DateTimeFormat().resolvedOptions().timeZone },
            chart: {
                animation: false,
                zooming: { type: "x" },
                margin: [8, 12, 36, 56],
                backgroundColor: "transparent",
            },
            xAxis: {
                type: "datetime",
                labels: { style: { fontSize: "10px" } },
                lineColor: "rgba(128,128,128,0.3)",
                tickColor: "rgba(128,128,128,0.3)",
            },
            yAxis: {
                title: { text: "DPS", style: { fontSize: "10px" } },
                labels: {
                    style: { fontSize: "10px" },
                    formatter() { return fmtDps(this.value as number); },
                },
                gridLineColor: "rgba(128,128,128,0.15)",
                min: 0,
            },
            legend: {
                enabled: false,  // 用 checkbox 取代
            },
            tooltip: {
                shared: true,
                formatter() {
                    const pts = (this.points ?? []);
                    const time = new Date(this.x as number).toLocaleTimeString("zh-TW", { hour12: false });
                    const rows = pts.map(
                        (p) => `<span style="color:${p.color}">●</span> ${p.series.name}: <b>${fmtDps(p.y ?? 0)}</b>`,
                    ).join("<br>");
                    return `<span style="font-size:10px">${time}</span><br>${rows}`;
                },
            },
            plotOptions: {
                area: { stacking: undefined },
            },
            series: buildSeries(),
        });

        const rebuildChart = () => {
            if (!chartDom.value) return;
            chart?.destroy();
            chart = highcharts.chart(chartDom.value, buildOptions());
        };

        onMounted(rebuildChart);

        watch([tickSize, enabledIds, () => props.entities, () => props.startAt, () => props.endAt],
            rebuildChart,
            { deep: true },
        );

        watch(tickSize, (v) => localStorage.setItem(TICK_KEY, String(v)));

        onUnmounted(() => chart?.destroy());

        return { chartDom, tickSize, tickOptions, enabledIds, palette: PALETTE };
    },
});

const PALETTE = [
    "#5470c6","#91cc75","#fac858","#ee6666",
    "#73c0de","#3ba272","#fc8452","#9a60b4","#ea7ccc",
];

function fmtDps(v: number): string {
    if (!isFinite(v) || v <= 0) return "0";
    if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(2)}M/s`;
    if (v >= 1_000)     return `${(v / 1_000).toFixed(1)}k/s`;
    return `${Math.round(v)}/s`;
}
</script>
