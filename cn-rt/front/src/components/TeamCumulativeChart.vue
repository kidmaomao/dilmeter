<template>
    <div class="team-chart-component" :aria-label="mode === 'dps' ? '团队实时 DPS 曲线' : '团队累计伤害曲线'">
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
        <div v-if="mode === 'dps'" class="team-chart-granularity">
            <span>DPS 口径</span>
            <button type="button" :class="{ active: dpsMode === 'rolling' }" @click="dpsMode = 'rolling'">近期 DPS</button>
            <button type="button" :class="{ active: dpsMode === 'cumulative' }" @click="dpsMode = 'cumulative'">累计 DPS</button>
            <template v-if="dpsMode === 'rolling'">
                <span>统计窗口</span>
                <button v-for="option in tickOptions" :key="option" type="button" :class="{ active: windowSeconds === option }" @click="windowSeconds = option">最近 {{ option }} 秒</button>
            </template>
        </div>
        <div class="team-chart-granularity">
            <span>横坐标</span>
            <button type="button" :class="{ active: axis === 'time' }" @click="axis = 'time'">战斗时间</button>
            <button type="button" :disabled="!hasHealth" :class="{ active: axis === 'hp' }" @click="axis = 'hp'">Boss 血量 %</button>
            <small v-if="!hasHealth">本记录缺少 Boss 血量历史</small>
            <span v-if="mode !== 'dps' || dpsMode === 'cumulative'">采样间隔</span>
            <template v-if="mode !== 'dps' || dpsMode === 'cumulative'">
            <button
                v-for="option in tickOptions"
                :key="option"
                type="button"
                :class="{ active: tickSeconds === option }"
                :aria-pressed="tickSeconds === option"
                @click="tickSeconds = option"
            >{{ option }} 秒</button>
            </template>
        </div>
        <div v-if="mode === 'dps'" class="team-dps-summary">
            <span>全团峰值 <strong>{{ formatCompact(dpsTimeline.peak.y) }}/秒</strong></span>
            <span v-if="dpsTimeline.peak.y > 0">{{ formatElapsed(dpsTimeline.peak.custom.from) }}–{{ formatElapsed(dpsTimeline.peak.custom.to) }}</span>
            <small>{{ dpsMode === 'rolling' ? `最近 ${windowSeconds} 秒伤害 ÷ 窗口时长（开场不足时按实际秒数）` : '累计伤害 ÷ 已经过时间' }}；虚线表示已记录的 Boss 无敌或成员倒地</small>
        </div>
        <div ref="chartElement" class="team-chart-canvas" />
        <section v-if="mode === 'dps' && dpsMode === 'rolling'" class="peak-analysis" aria-label="DPS 峰值分析">
            <div class="team-chart-granularity">
                <label><input v-model="showPeaks" type="checkbox" />峰值标记</label>
                <select v-if="players.length > 1" v-model="peakPlayerId" aria-label="峰值分析成员">
                    <option value="team-total">全团</option>
                    <option v-for="player in players" :key="player.entityId" :value="player.entityId">{{ player.label }}</option>
                </select>
                <small>点击峰值查看技能贡献与同期状态</small>
            </div>
            <template v-if="showPeaks">
                <div class="peak-choices">
                    <button v-for="(peak, index) in peaks" :key="peak.id" type="button" :class="{ active: activePeak?.id === peak.id }" :aria-pressed="activePeak?.id === peak.id" @click="selectedPeakId = peak.id">
                        {{ index + 1 }} · {{ formatElapsed(peak.point.custom.elapsed) }} {{ peak.title }}
                    </button>
                </div>
                <article v-if="activePeak" class="peak-detail">
                    <header>
                        <strong>{{ peakFocusLabel }} · {{ formatElapsed(activePeak.point.custom.elapsed) }} 峰值 {{ formatCompact(activePeak.point.y) }}/秒</strong>
                        <span v-if="activePeak.point.custom.hp !== undefined">Boss {{ activePeak.point.custom.hp.toFixed(1) }}%</span>
                        <span>窗口 {{ formatElapsed(activePeak.point.custom.from) }}–{{ formatElapsed(activePeak.point.custom.to) }}</span>
                        <span v-if="activePeak.previousDps !== undefined">前一窗口 {{ formatCompact(activePeak.previousDps) }}/秒</span>
                    </header>
                    <div class="peak-columns">
                        <div>
                            <h4>主要伤害技能 <small>按本窗口伤害排序</small></h4>
                            <ol class="peak-contributions">
                                <li v-for="row in activePeak.contributions.slice(0, 3)" :key="`${row.actorId}:${row.skillId}`">
                                    <div><strong>{{ row.name }}</strong><small v-if="peakPlayerId === 'team-total'">{{ row.actorLabel }}</small><b>{{ (row.share * 100).toFixed(1) }}%</b></div>
                                    <div><span>{{ formatCompact(row.damage) }} · {{ row.hits }} 次伤害</span><span v-if="row.increase !== undefined">较前窗 {{ row.increase >= 0 ? '+' : '−' }}{{ formatCompact(Math.abs(row.increase)) }}</span></div>
                                    <i :style="{ width: `${row.share * 100}%` }" />
                                </li>
                            </ol>
                            <small v-if="activePeak.contributions.length > 3">其余 {{ activePeak.contributions.length - 3 }} 项贡献 {{ ((1 - activePeak.contributions.slice(0, 3).reduce((sum, row) => sum + row.share, 0)) * 100).toFixed(1) }}%</small>
                        </div>
                        <div>
                            <h4>同期增益／减益 <small>记录到生效或施放</small></h4>
                            <ul class="peak-effects">
                                <li v-for="effect in activePeak.effects.slice(0, 4)" :key="effect.key"><strong>{{ effect.name }}</strong><span>{{ effect.source }} · {{ effect.evidence === 'condition' ? `作用于 ${effect.target}` : '已施放，覆盖未确认' }}</span></li>
                            </ul>
                            <details v-if="activePeak.effects.length > 4"><summary>查看其余 {{ activePeak.effects.length - 4 }} 个状态</summary><ul class="peak-effects"><li v-for="effect in activePeak.effects.slice(4)" :key="effect.key"><strong>{{ effect.name }}</strong><span>{{ effect.source }} · {{ effect.evidence === 'condition' ? `作用于 ${effect.target}` : '已施放，覆盖未确认' }}</span></li></ul></details>
                            <small v-if="!activePeak.effects.length">该窗口没有记录到可识别的相关状态或施放。</small>
                        </div>
                    </div>
                    <p>伤害占比按窗口统计；同期状态用于回顾配合，不代表已量化该效果带来的增伤。</p>
                </article>
                <small v-else>有近期伤害峰值后显示分析。</small>
            </template>
        </section>
    </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch, type PropType } from "vue";
import Highcharts from "highcharts";
import type { Options, SeriesOptionsType } from "highcharts";
import type { EntityDamage } from "@/eventActor";
import { buildTeamDpsTimeline, splitTeamSegments, type TeamDpsPoint, type TeamDpsMode } from "@/teamDps";
import type { BattleVitalPoint, BattleInterval } from "@/battleChartHistory";
import { explainDpsPeak, selectDpsPeaks, type PeakAction, type PeakActor } from "@/teamDpsPeaks";

export type TeamChartPlayer = {
    entityId: string;
    label: string;
    damages: EntityDamage[];
    deadIntervals?: BattleInterval[];
};

const props = defineProps({
    players: {
        type: Array as PropType<TeamChartPlayer[]>,
        required: true,
    },
    startAt: { type: Number, required: true },
    endAt: { type: Number, required: true },
    health: { type: Array as PropType<BattleVitalPoint[]>, default: () => [] },
    invulnerable: { type: Array as PropType<BattleInterval[]>, default: () => [] },
    mode: { type: String as PropType<"damage" | "dps">, default: "damage" },
    actions: { type: Array as PropType<PeakAction[]>, default: () => [] },
    actors: { type: Object as PropType<Record<string, PeakActor>>, default: () => ({}) },
    skillName: { type: Function as PropType<(id: number) => string> },
    conditionName: { type: Function as PropType<(id: number) => string> },
    focusPlayerId: { type: String, default: '' },
});

const chartElement = ref<HTMLElement>();
const tickOptions = [1, 2, 5, 10];
const tickSeconds = ref(1);
const dpsMode = ref<TeamDpsMode>('rolling');
const windowSeconds = ref(5);
const axis = ref<"time" | "hp">("time");
const showPeaks = ref(true);
const peakPlayerId = ref(props.focusPlayerId || props.players[0]?.entityId || 'team-total');
const selectedPeakId = ref('');
watch(() => props.focusPlayerId, id => { if (id) peakPlayerId.value = id; });
watch(() => props.players, players => {
    if (peakPlayerId.value !== 'team-total' && !players.some(p => p.entityId === peakPlayerId.value)) peakPlayerId.value = players[0]?.entityId || 'team-total';
});
const hasHealth = computed(() => props.health.some(p => Number.isFinite(p.current) && Number.isFinite(p.maximum) && p.maximum! > 0));
watch(hasHealth, value => { if (!value) axis.value = "time"; });
let chart: Highcharts.Chart | undefined;
const dpsTimeline = computed(() => buildTeamDpsTimeline(props.players, props.startAt, props.endAt, dpsMode.value === 'rolling' && props.mode === 'dps' ? 1 : tickSeconds.value, {
    health: props.health, invulnerable: props.invulnerable, dpsMode: dpsMode.value, windowSeconds: windowSeconds.value,
}));
const peakFocusLabel = computed(() => props.players.find(p => p.entityId === peakPlayerId.value)?.label || '全团');
const peaks = computed(() => {
    if (!showPeaks.value || props.mode !== 'dps' || dpsMode.value !== 'rolling') return [];
    const all = peakPlayerId.value === 'team-total';
    const points = all ? dpsTimeline.value.total : dpsTimeline.value.members.find(p => p.entityId === peakPlayerId.value)?.points ?? [];
    const players = all ? props.players : props.players.filter(p => p.entityId === peakPlayerId.value);
    return selectDpsPeaks(points, windowSeconds.value).map(point => explainDpsPeak(point, players, props.startAt, props));
});
const activePeak = computed(() => peaks.value.find(p => p.id === selectedPeakId.value) ?? peaks.value.reduce<(typeof peaks.value)[number] | undefined>((best, p) => !best || p.point.y > best.point.y ? p : best, undefined));

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

function buildSeries(themeText: string): SeriesOptionsType[] {
    if (props.mode === "dps" || axis.value === "hp") {
        const rows = [...(props.mode === "dps" && props.players.length > 1 ? [{ entityId: 'team-total', label: '全团 DPS', points: dpsTimeline.value.total }] : []), ...dpsTimeline.value.members];
        const series = rows.flatMap((player) => {
            const total = player.entityId === 'team-total';
            const index = props.players.findIndex(p => p.entityId === player.entityId);
            const id = `player-${player.entityId}`;
            return [false, true].map(dashed => ({
                id: dashed ? `${id}-dashed` : id, linkedTo: dashed ? id : undefined,
                type: axis.value === 'hp' ? 'scatter' : 'line', name: player.label,
                data: splitTeamSegments(player.points, dashed, axis.value, props.mode === 'damage'),
                color: total ? themeText : PALETTE[index % PALETTE.length],
                lineWidth: total ? 3 : 1.5, zIndex: total ? 3 : 1,
                dashStyle: dashed ? 'Dash' : 'Solid', showInLegend: !dashed,
                marker: { enabled: false }, connectNulls: false,
            }));
        }) as SeriesOptionsType[];
        if (peaks.value.length) series.push({
            id: 'dps-peak-markers', type: 'scatter', name: '峰值', linkedTo: `player-${peakPlayerId.value}`,
            color: '#ffcf70', lineWidth: 0, zIndex: 7, showInLegend: false,
            tooltip: { pointFormat: '{point.y}', followPointer: false },
            marker: { enabled: true, symbol: 'diamond', radius: 5, lineWidth: 1, lineColor: '#171917' },
            data: peaks.value.flatMap((peak, index) => axis.value === 'hp' && peak.point.custom.hp === undefined ? [] : [{
                x: axis.value === 'hp' ? peak.point.custom.hp! : peak.point.x, y: peak.point.y,
                custom: { ...peak.point.custom, peakId: peak.id, peakLabel: `${index + 1} ${peak.title}` },
            }]),
            dataLabels: { enabled: true, allowOverlap: false, crop: true, overflow: 'justify', y: -8,
                style: { color: '#ffcf70', fontSize: '10px', fontWeight: '500', textOutline: '2px #171917' },
                formatter() { return escapeHtml(String(this.options.custom?.peakLabel ?? '')); } },
            point: { events: { click() { selectedPeakId.value = String(this.options.custom?.peakId ?? ''); } } },
        });
        return series;
    }
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
            id: `player-${player.entityId}`,
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
            type: props.mode === "dps" ? "line" : "area",
            animation: false,
            backgroundColor: "transparent",
            spacing: [14, 18, 10, 10],
            zooming: { type: "x" },
        },
        title: { text: undefined },
        credits: { enabled: false },
        xAxis: {
            min: 0,
            max: axis.value === "hp" ? 100 : Math.max(1, props.endAt - props.startAt),
            reversed: axis.value === "hp",
            title: { text: axis.value === "hp" ? "Boss 剩余血量" : "战斗时间", style: { color: themeMuted, fontSize: "11px" } },
            labels: {
                style: { color: themeMuted, fontSize: "10px" },
                formatter() { return axis.value === "hp" ? `${this.value}%` : formatElapsed(Number(this.value)); },
            },
            lineColor: themeBorder,
            tickColor: themeBorder,
            gridLineWidth: 1,
            gridLineColor: themeBorder,
        },
        yAxis: {
            min: 0,
            title: { text: props.mode === "dps" ? (dpsMode.value === 'rolling' ? `最近 ${windowSeconds.value} 秒 DPS` : '累计 DPS（累计伤害 / 已经过秒数）') : "全团累计伤害", style: { color: themeMuted, fontSize: "11px" } },
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
            shared: axis.value === "time",
            outside: true,
            useHTML: true,
            backgroundColor: themeSurface,
            borderColor: themeBorder,
            style: { color: themeText, fontSize: "11px", zIndex: 10030 },
            formatter() {
                const peakId = this.options.custom?.peakId;
                if (peakId) {
                    const peak = peaks.value.find(p => p.id === peakId);
                    return peak ? `<b>${formatElapsed(peak.point.custom.elapsed)} · ${escapeHtml(peak.title)}</b><br>${formatCompact(peak.point.y)}/秒<br>点击查看伤害贡献与同期状态` : false;
                }
                const points = [...new Map((this.points ?? [this]).filter(p => !p.options.custom?.peakId).map(p => [p.series.name, p])).values()];
                if (props.mode === "dps" || axis.value === "hp") {
                    const interval = points[0]?.options.custom as TeamDpsPoint["custom"] | undefined;
                    const rows = points.map((point) => {
                        const detail = point.options.custom as TeamDpsPoint['custom'];
                        return `<span style="color:${point.color}">●</span> ${escapeHtml(point.series.name)}：<b>${formatCompact(point.y ?? 0)}${props.mode === 'dps' ? '/秒' : ''}</b>${detail.reason ? ` · ${detail.reason}` : ''}`;
                    }).join("<br>");
                    const window = props.mode === 'dps' && dpsMode.value === 'rolling' && interval
                        ? `<br>统计窗口 ${formatElapsed(interval.from)}–${formatElapsed(interval.to)}` : '';
                    return `<b>${formatElapsed(interval?.elapsed ?? 0)}${interval?.hp !== undefined ? ` · Boss ${interval.hp.toFixed(1)}%` : ''}</b>${window}<br>${rows}`;
                }
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
            series: { turboThreshold: 0, animation: false },
        },
        series: buildSeries(themeText),
    };
}

function renderChart() {
    if (!chartElement.value) return;
    const options = buildOptions();
    if (chart) {
        const series = options.series ?? [];
        // HP values can repeat or rise after healing. Highcharts' keyed point
        // updates match by X and can append unmatched vertices out of time order.
        // Replace the data in chronological order, preserving legend visibility.
        chart.update({ ...options, series: series.map(item => {
            const metadata = { ...item } as Highcharts.SeriesLineOptions;
            delete metadata.data;
            return metadata;
        }) }, false, true, false);
        for (const item of series) {
            const current = chart.series.find(s => s.options.id === item.id);
            current?.setData((item as Highcharts.SeriesLineOptions).data ?? [], false, false, false);
        }
        chart.redraw(false);
    } else chart = Highcharts.chart(chartElement.value, options);
}

watch([axis, dpsMode, windowSeconds, () => props.health, () => props.invulnerable, tickSeconds, () => props.mode, () => props.players, () => props.startAt, () => props.endAt], renderChart, { deep: true });
watch(peaks, renderChart);
watch([axis, () => props.startAt], () => chart?.xAxis[0]?.setExtremes(undefined, undefined));
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
    const precise = Math.max(0, Math.round(seconds * 10) / 10);
    const value = Math.floor(precise);
    const minutes = Math.floor(value / 60);
    const decimal = precise % 1 > 0 ? (precise % 1).toFixed(1).slice(1) : "";
    return `${String(minutes).padStart(2, "0")}:${String(value % 60).padStart(2, "0")}${decimal}`;
}

function formatCompact(value: number): string {
    if (!Number.isFinite(value) || value <= 0) return "0";
    if (value >= 100_000_000) return `${(value / 100_000_000).toFixed(2)}亿`;
    if (value >= 10_000) return `${(value / 10_000).toFixed(1)}万`;
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
.team-chart-granularity { display: flex; flex-wrap: wrap; align-items: center; gap: 5px; margin: 0 0 8px; color: #bdbdbd; font-size: 11px; }
.team-chart-granularity span { margin-right: 4px; }
.team-chart-granularity button { height: 24px; padding: 0 10px; color: #d8d8d8; background: linear-gradient(#393939, #202020); border: 1px solid #5e5e5e; cursor: pointer; }
.team-chart-granularity button.active { color: #fff; border-color: #9bd74f; box-shadow: inset 0 0 0 1px #435f23; }
.team-dps-summary { display: flex; align-items: baseline; flex-wrap: wrap; gap: 6px 14px; margin: 0 0 8px; color: var(--ui-theme-muted, #bdbdbd); font-size: 11px; }
.team-dps-summary strong { color: var(--ui-theme-text, #fff); font-size: 14px; }
.team-dps-summary small { margin-left: auto; font-size: 10px; }
.team-chart-canvas { height: 350px; border: 1px solid #444; background: linear-gradient(#181818, #101010); }
.peak-analysis { margin-top: 12px; color: var(--ui-theme-text, #eee); font-size: 12px; }
.peak-analysis label { display: flex; align-items: center; gap: 4px; }
.peak-analysis select { padding: 3px 6px; color: var(--ui-theme-text, #eee); background: var(--ui-theme-surface, #171917); border: 1px solid var(--ui-theme-border, #555); }
.peak-choices { display: flex; flex-wrap: wrap; gap: 6px; margin: 8px 0; }
.peak-choices button { padding: 6px 9px; color: var(--ui-theme-muted, #bdbdbd); background: var(--ui-theme-surface, #171917); border: 1px solid var(--ui-theme-border, #555); cursor: pointer; }
.peak-choices button.active { color: #ffcf70; border-color: #aa8746; }
.peak-detail { padding: 14px; border: 1px solid var(--ui-theme-border, #555); background: var(--ui-theme-surface, #171917); }
.peak-detail header { display: flex; flex-wrap: wrap; gap: 8px 16px; align-items: baseline; }
.peak-detail header strong { font-size: 14px; }
.peak-detail header span, .peak-analysis small, .peak-detail p { color: var(--ui-theme-muted, #aaa); }
.peak-columns { display: grid; grid-template-columns: 1fr 1fr; gap: 24px; margin-top: 16px; }
.peak-columns h4 { margin: 0 0 10px; font-size: 12px; }.peak-columns h4 small { margin-left: 5px; font-weight: 400; }
.peak-contributions, .peak-effects { list-style: none; margin: 0; padding: 0; }
.peak-contributions li { position: relative; margin: 10px 0; padding-bottom: 7px; border-bottom: 3px solid var(--ui-theme-border, #444); }
.peak-contributions li div { display: flex; gap: 8px; margin-bottom: 4px; }.peak-contributions b, .peak-contributions li div span:last-child { margin-left: auto; }
.peak-contributions li div span { color: var(--ui-theme-muted, #aaa); font-size: 11px; }
.peak-contributions i { position: absolute; bottom: -3px; left: 0; height: 3px; background: #ffcf70; }
.peak-effects li { display: flex; flex-direction: column; gap: 3px; margin: 8px 0; }.peak-effects span { color: var(--ui-theme-muted, #aaa); font-size: 11px; }
.peak-detail summary { cursor: pointer; color: #ffcf70; font-size: 11px; }.peak-detail p { margin: 14px 0 0; font-size: 10px; }
@media (max-width: 700px) { .peak-columns { grid-template-columns: 1fr; } }
</style>
