<template>
    <div class="team-chart-component">
        <section class="team-dps-ranking" aria-label="团队成员 DPS 排名">
            <header><strong>成员 DPS 排名</strong><span>按本场共同战斗时长计算</span></header>
            <ol>
                <li v-for="(player, index) in rankings" :key="player.entityId">
                    <span class="rank-number">{{ index + 1 }}</span>
                    <div class="rank-bar" :style="{ '--rank-color': player.color }">
                        <i :style="{ width: `${player.ratio * 100}%` }" />
                        <div class="rank-bar-copy">
                            <strong>{{ player.label }}</strong>
                            <span>DPS {{ formatCompact(player.dps) }}　·　累计 {{ formatCompact(player.total) }}</span>
                        </div>
                    </div>
                    <span class="rank-share">{{ (player.share * 100).toFixed(1) }}%</span>
                </li>
            </ol>
        </section>
        <div class="team-chart-granularity">
            <span>时间粒度</span>
            <button
                v-for="option in tickOptions"
                :key="option"
                type="button"
                :class="{ active: tickSeconds === option }"
                @click="tickSeconds = option"
            >{{ option }} 秒</button>
        </div>
        <div ref="chartElement" class="team-chart-canvas" />
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, type PropType } from "vue";
import Highcharts from "highcharts";
import type { Options, SeriesOptionsType } from "highcharts";
import type { EntityDamage } from "@/eventActor";

export type TeamChartPlayer = {
    entityId: string;
    label: string;
    damages: EntityDamage[];
};

const props = defineProps({
    players: {
        type: Array as PropType<TeamChartPlayer[]>,
        required: true,
    },
    startAt: { type: Number, required: true },
    endAt: { type: Number, required: true },
});

const chartElement = ref<HTMLElement>();
const tickOptions = [1, 2, 5, 10];
const tickSeconds = ref(1);
let chart: Highcharts.Chart | undefined;

const rankings = computed(() => {
    const duration = Math.max(1, props.endAt - props.startAt);
    const rows = props.players.map((player, index) => {
        const total = player.damages
            .filter((damage) => damage.Damage > 0 && damage.At >= props.startAt && damage.At <= props.endAt)
            .reduce((sum, damage) => sum + damage.Damage, 0);
        return { ...player, total, dps: total / duration, color: PALETTE[index % PALETTE.length] };
    }).sort((a, b) => b.dps - a.dps);
    const maxDps = Math.max(1, rows[0]?.dps ?? 1);
    const teamTotal = Math.max(1, rows.reduce((sum, row) => sum + row.total, 0));
    return rows.map((row) => ({ ...row, ratio: row.dps / maxDps, share: row.total / teamTotal }));
});

function buildSeries(): SeriesOptionsType[] {
    const startAt = props.startAt;
    const endAt = Math.max(startAt, props.endAt);
    const duration = Math.max(0, endAt - startAt);
    const step = tickSeconds.value;
    return props.players.map((player, index) => {
        const events = player.damages
            .filter((damage) => damage.Damage > 0 && damage.At >= startAt && damage.At <= endAt)
            .sort((a, b) => a.At - b.At);
        let cursor = 0;
        let cumulative = 0;
        let previousCumulative = 0;
        const data: Array<{ x: number; y: number; custom: { dps: number } }> = [];
        const points = Math.max(1, Math.ceil(duration / step));
        for (let point = 0; point <= points; point += 1) {
            const elapsed = Math.min(duration, point * step);
            const until = startAt + elapsed + 0.000_001;
            while (cursor < events.length && events[cursor].At <= until) {
                cumulative += events[cursor].Damage;
                cursor += 1;
            }
            const interval = point === 0 ? Math.max(step, 1) : Math.max(0.001, elapsed - (data[data.length - 1]?.x ?? 0));
            data.push({
                x: elapsed,
                y: cumulative,
                custom: { dps: Math.max(0, (cumulative - previousCumulative) / interval) },
            });
            previousCumulative = cumulative;
        }
        return {
            type: "area",
            name: player.label,
            data,
            color: PALETTE[index % PALETTE.length],
            lineColor: PALETTE[index % PALETTE.length],
            lineWidth: 1.5,
            fillOpacity: 0.68,
            marker: { enabled: false },
        } as SeriesOptionsType;
    });
}

function buildOptions(): Options {
    const rootStyle = getComputedStyle(document.documentElement);
    const themeText = rootStyle.getPropertyValue("--ui-theme-text").trim() || "#f3f3f3";
    const themeMuted = rootStyle.getPropertyValue("--ui-theme-muted").trim() || "#c9c9c9";
    const themeBorder = rootStyle.getPropertyValue("--ui-theme-border").trim() || "#686868";
    const themeSurface = rootStyle.getPropertyValue("--ui-theme-surface").trim() || "#171917";
    const themeAccent = rootStyle.getPropertyValue("--ui-color-accent").trim() || "#8ee000";
    return {
        chart: {
            type: "area",
            animation: false,
            backgroundColor: "transparent",
            spacing: [14, 18, 10, 10],
            zooming: { type: "x" },
        },
        title: { text: undefined },
        credits: { enabled: false },
        xAxis: {
            min: 0,
            max: Math.max(1, props.endAt - props.startAt),
            title: { text: "战斗时间", style: { color: themeMuted, fontSize: "11px" } },
            labels: {
                style: { color: themeMuted, fontSize: "10px" },
                formatter() { return formatElapsed(Number(this.value)); },
            },
            lineColor: themeBorder,
            tickColor: themeBorder,
            gridLineWidth: 1,
            gridLineColor: themeBorder,
        },
        yAxis: {
            min: 0,
            title: { text: "全团累计伤害", style: { color: themeMuted, fontSize: "11px" } },
            labels: {
                style: { color: themeMuted, fontSize: "10px" },
                formatter() { return formatCompact(Number(this.value)); },
            },
            gridLineColor: themeBorder,
        },
        legend: {
            enabled: true,
            itemStyle: { color: themeText, fontSize: "11px", fontWeight: "600" },
            itemHoverStyle: { color: themeAccent },
            itemHiddenStyle: { color: themeMuted },
            symbolRadius: 0,
        },
        tooltip: {
            shared: true,
            useHTML: true,
            backgroundColor: themeSurface,
            borderColor: themeBorder,
            style: { color: themeText, fontSize: "11px" },
            formatter() {
                const points = this.points ?? [];
                const rows = points.slice().reverse().map((point) => {
                    const custom = point.options.custom as { dps?: number } | undefined;
                    return `<span style="color:${point.color}">■</span> ${escapeHtml(point.series.name)}：<b>${formatCompact(point.y ?? 0)}</b> <span style="color:${themeAccent}">(${formatCompact(custom?.dps ?? 0)}/秒)</span>`;
                }).join("<br>");
                const teamTotal = points.reduce((sum, point) => sum + Number(point.y ?? 0), 0);
                return `<b>${formatElapsed(Number(this.x))}</b><br>${rows}<br><span style="color:${themeText}">全团：<b>${formatCompact(teamTotal)}</b></span>`;
            },
        },
        plotOptions: {
            area: {
                stacking: "normal",
                threshold: 0,
                animation: false,
                states: { inactive: { opacity: 0.28 } },
            },
            series: { turboThreshold: 0 },
        },
        series: buildSeries(),
    };
}

function renderChart() {
    if (!chartElement.value) return;
    chart?.destroy();
    chart = Highcharts.chart(chartElement.value, buildOptions());
}

watch([tickSeconds, () => props.players, () => props.startAt, () => props.endAt], renderChart, { deep: true });
const handleUiColorThemeChanged = () => renderChart();
onMounted(() => {
    renderChart();
    window.addEventListener("dilmeter-ui-color-theme-changed", handleUiColorThemeChanged);
});
onUnmounted(() => {
    window.removeEventListener("dilmeter-ui-color-theme-changed", handleUiColorThemeChanged);
    chart?.destroy();
});

function formatElapsed(seconds: number): string {
    const value = Math.max(0, Math.round(seconds));
    const minutes = Math.floor(value / 60);
    return `${String(minutes).padStart(2, "0")}:${String(value % 60).padStart(2, "0")}`;
}

function formatCompact(value: number): string {
    if (!Number.isFinite(value) || value <= 0) return "0";
    if (value >= 100_000_000) return `${(value / 100_000_000).toFixed(2)}亿`;
    if (value >= 10_000) return `${(value / 10_000).toFixed(value >= 1_000_000 ? 0 : 1)}万`;
    return Math.round(value).toLocaleString("zh-CN");
}

function escapeHtml(value: string): string {
    return value.replace(/[&<>"']/g, (character) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        "\"": "&quot;",
        "'": "&#39;",
    })[character] ?? character);
}

const PALETTE = ["#8ee000", "#31a9ff", "#ffb43c", "#cf72ff", "#ff5b65", "#38d0b2", "#e8df5b", "#6d7dff", "#f489c7", "#82b46b"];
</script>

<style scoped>
.team-chart-component { min-height: 480px; }
.team-dps-ranking { margin: 0 0 10px; border: 1px solid #4a4a4a; background: linear-gradient(#202020, #161616); }
.team-dps-ranking header { display: flex; align-items: baseline; gap: 8px; padding: 7px 9px; border-bottom: 1px solid #484848; }
.team-dps-ranking header strong { color: #fff; font-size: 12px; }
.team-dps-ranking header span { color: #969696; font-size: 10px; }
.team-dps-ranking ol { display: grid; gap: 4px; margin: 0; padding: 7px; list-style: none; }
.team-dps-ranking li { display: grid; grid-template-columns: 28px minmax(260px, 1fr) 58px; align-items: stretch; gap: 6px; min-height: 39px; color: #f2f2f2; font-size: 11px; }
.rank-number { display: grid; place-items: center; color: #f3f3f3; background: #292929; border: 1px solid #4c4c4c; font-size: 13px; font-weight: 800; }
.rank-bar { position: relative; min-width: 0; overflow: hidden; background: #292929; border: 1px solid #505050; box-shadow: inset 0 0 0 1px rgba(0, 0, 0, .45); }
.rank-bar i { position: absolute; inset: 0 auto 0 0; display: block; min-width: 3px; background: var(--rank-color); opacity: .72; }
.rank-bar-copy { position: relative; z-index: 1; display: flex; align-items: center; justify-content: space-between; gap: 12px; height: 100%; padding: 0 11px; text-shadow: 0 1px 2px #000, 1px 0 1px #000; }
.rank-bar-copy strong { overflow: hidden; color: #fff; font-size: 12px; text-overflow: ellipsis; white-space: nowrap; }
.rank-bar-copy span { flex: 0 0 auto; color: #f7f7f7; font-size: 11px; font-weight: 700; white-space: nowrap; }
.rank-share { display: grid; place-items: center; color: #f2f2f2; background: #252525; border: 1px solid #4b4b4b; font-weight: 800; text-align: center; }
.team-chart-granularity { display: flex; align-items: center; gap: 5px; margin: 0 0 8px; color: #bdbdbd; font-size: 11px; }
.team-chart-granularity span { margin-right: 4px; }
.team-chart-granularity button { height: 24px; padding: 0 10px; color: #d8d8d8; background: linear-gradient(#393939, #202020); border: 1px solid #5e5e5e; cursor: pointer; }
.team-chart-granularity button.active { color: #fff; border-color: #9bd74f; box-shadow: inset 0 0 0 1px #435f23; }
.team-chart-canvas { height: 350px; border: 1px solid #444; background: linear-gradient(#181818, #101010); }
</style>
