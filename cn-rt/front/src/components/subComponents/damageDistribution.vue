<template>
    <v-sheet class="d-flex flex-column" style="height: 100%;">
        <v-sheet class="d-flex align-center pa-2 flex-shrink-0 flex-wrap" style="gap: 8px;">
            <span>Count: {{ stats.count }}</span>
            <span>Critical: {{ stats.criticalCount }} ({{ stats.criticalRate }}%)</span>
            <span>Avg: {{ humanReadableNumber(stats.avg) }}</span>
            <span>Min: {{ humanReadableNumber(stats.min) }}</span>
            <span>Max: {{ humanReadableNumber(stats.max) }}</span>
            <span>DPS: {{ humanReadableNumber(stats.dps) }}</span>
            <v-btn v-if="isZoomed" size="x-small" variant="text" color="primary" prepend-icon="mdi-magnify-minus" @click="resetZoom">Reset Zoom</v-btn>
        </v-sheet>
        <div ref="chartDom" style="flex: 1; min-height: 0;"></div>
    </v-sheet>
</template>

<script lang="ts">
import { defineComponent, ref, reactive, PropType, onMounted, onUnmounted } from 'vue';
import highcharts, { Options } from 'highcharts';
import type { EntityDamage } from '@/eventActor';
import { humanReadableNumber } from '@/lib/util';

export default defineComponent({
    props: {
        damages: {
            type: Array as PropType<EntityDamage[]>,
            required: true,
        },
    },
    setup(props) {
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart = undefined!;
        const isZoomed = ref(false);

        const damages = props.damages;

        const computeStats = (ds: EntityDamage[]) => {
            const count = ds.length;
            const criticalCount = ds.filter(d => d.IsCritical).length;
            const criticalRate = count > 0 ? (100 * criticalCount / count).toFixed(1) : '0';
            const totalDamage = ds.reduce((s, d) => s + d.Damage, 0);
            const avg = count > 0 ? Math.round(totalDamage / count) : 0;
            const min = count > 0 ? Math.round(Math.min(...ds.map(d => d.Damage))) : 0;
            const max = count > 0 ? Math.round(Math.max(...ds.map(d => d.Damage))) : 0;
            const times = ds.map(d => d.At);
            const duration = times.length >= 2 ? Math.max(...times) - Math.min(...times) : 0;
            const dps = duration > 0 ? Math.round(totalDamage / duration) : 0;
            return { count, criticalCount, criticalRate, totalDamage, avg, min, max, dps };
        };

        const stats = reactive({ count: 0, criticalCount: 0, criticalRate: '0', totalDamage: 0, avg: 0, min: 0, max: 0, dps: 0 });

        let showNormal = true;
        let showCritical = true;
        let zoomDamageMin: number | null = null;
        let zoomDamageMax: number | null = null;
        let currentBinRangeMin = 0;
        let currentBinSize = 0;

        const getDamagesInRange = () => {
            if (zoomDamageMin === null || zoomDamageMax === null) return damages;
            return damages.filter(d => d.Damage >= zoomDamageMin! && d.Damage <= zoomDamageMax!);
        };

        const getVisibleDamages = (ds: EntityDamage[]) => {
            if (showNormal && showCritical) return ds;
            if (showNormal) return ds.filter(d => !d.IsCritical);
            if (showCritical) return ds.filter(d => d.IsCritical);
            return [];
        };

        const updateStats = () => {
            const inRange = getDamagesInRange();
            const visible = getVisibleDamages(inRange);
            Object.assign(stats, computeStats(visible));
        };

        const buildChartOpt = (): Options => {
            const inRange = getDamagesInRange();
            const visible = getVisibleDamages(inRange);
            Object.assign(stats, computeStats(visible));

            if (inRange.length === 0) return { title: { text: '' }, series: [] };

            const rangeMin = Math.min(...inRange.map(d => d.Damage));
            const rangeMax = Math.max(...inRange.map(d => d.Damage));
            const range = rangeMax - rangeMin;
            const binCount = Math.max(1, Math.min(50, Math.ceil(Math.sqrt(inRange.length))));
            const binSize = range > 0 ? range / binCount : 1;

            currentBinRangeMin = rangeMin;
            currentBinSize = binSize;

            const critBins: number[] = new Array(binCount).fill(0);
            const normalBins: number[] = new Array(binCount).fill(0);

            for (const d of inRange) {
                let idx = range > 0 ? Math.floor((d.Damage - rangeMin) / binSize) : 0;
                if (idx >= binCount) idx = binCount - 1;
                if (d.IsCritical) {
                    critBins[idx]++;
                } else {
                    normalBins[idx]++;
                }
            }

            const categories = Array.from({ length: binCount }, (_, i) => {
                const lo = Math.round(rangeMin + i * binSize);
                const hi = Math.round(rangeMin + (i + 1) * binSize);
                return `${lo}-${hi}`;
            });

            const onLegendClick = function (this: highcharts.Series) {
                if (this.name === 'Normal') showNormal = this.visible;
                else if (this.name === 'Critical') showCritical = this.visible;
                updateStats();
            };

            const onSelection = function (this: highcharts.Chart, e: any): boolean | undefined {
                const xAxis = e.xAxis?.[0];
                if (xAxis) {
                    const idxMin = Math.max(0, Math.floor(xAxis.min));
                    const idxMax = Math.ceil(xAxis.max);
                    zoomDamageMin = currentBinRangeMin + idxMin * currentBinSize;
                    zoomDamageMax = currentBinRangeMin + (idxMax + 1) * currentBinSize;
                    isZoomed.value = true;
                    rebuildChart();
                }
                return false;
            };

            return {
                lang: { locale: 'en' },
                title: { text: '' },
                chart: {
                    type: 'column',
                    zooming: { type: 'x' },
                    events: {
                        selection: onSelection as any,
                    },
                },
                xAxis: {
                    categories,
                    title: { text: 'Damage' },
                    labels: { rotation: -45, style: { fontSize: '10px' } },
                },
                yAxis: {
                    title: { text: 'Count' },
                    stackLabels: { enabled: false },
                },
                plotOptions: {
                    column: {
                        stacking: 'normal',
                        pointPadding: 0,
                        groupPadding: 0,
                        borderWidth: 1,
                    },
                    series: {
                        events: {
                            legendItemClick: onLegendClick,
                        },
                    },
                },
                tooltip: { valueDecimals: 0 },
                series: [
                    { type: 'column', name: 'Normal', data: normalBins, color: '#4FC3F7', visible: showNormal },
                    { type: 'column', name: 'Critical', data: critBins, color: '#EF5350', visible: showCritical },
                ],
            };
        };

        const rebuildChart = () => {
            if (chart) chart.destroy();
            chart = highcharts.chart(chartDom.value, buildChartOpt());
        };

        const resetZoom = () => {
            zoomDamageMin = null;
            zoomDamageMax = null;
            isZoomed.value = false;
            rebuildChart();
        };

        onMounted(() => {
            chart = highcharts.chart(chartDom.value, buildChartOpt());
        });

        onUnmounted(() => {
            chart?.destroy();
        });

        return { chartDom, stats, isZoomed, resetZoom, humanReadableNumber };
    },
});
</script>
