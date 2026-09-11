<template>
    <v-sheet class="d-flex flex-column" style="height: 100%;">
        <v-sheet class="d-flex align-center pa-2 flex-shrink-0" style="gap: 12px;">
            <span>Total: {{ humanReadableNumber(totalDamage || 0) }}</span>
            <span>Duration: {{ durationText }}</span>
            <v-spacer />
            <v-checkbox v-if="mode === 'dps'" v-model="dithering" label="Dithering" density="compact" hide-details />
            <span>Tick:</span>
            <v-select v-model="tickSize" :items="tickOptions" item-title="label" item-value="value"
                variant="outlined" density="compact" hide-details style="max-width: 150px;" />
        </v-sheet>
        <div ref="chartDom" style="flex: 1; min-height: 0;"></div>
    </v-sheet>
</template>

<script lang="ts">
import { defineComponent, ref, PropType, onMounted, onUnmounted, watch } from 'vue';
import highcharts, { Options, SeriesAreaOptions } from 'highcharts';
import type { EntityDamage } from '@/eventActor';
import { humanReadableNumber } from '@/lib/util';
import { setTimeRange, clearTimeRange } from '@/store';

type ChartEntity = { name: string, damages: EntityDamage[] };

export default defineComponent({
    props: {
        entities: {
            type: Array as PropType<ChartEntity[]>,
            required: true,
        },
        mode: {
            type: String as PropType<'cumulative' | 'dps' | 'count'>,
            default: 'cumulative',
        },
    },
    setup(props) {
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart = undefined!;

        const TICK_STORAGE_KEY = `damageChart.tickSize.${props.mode}`;
        const savedTick = Number(localStorage.getItem(TICK_STORAGE_KEY));
        const tickSize = ref(savedTick || 300);
        const tickOptions = [
            { label: '5s', value: 5 },
            { label: '15s', value: 15 },
            { label: '30s', value: 30 },
            { label: '1min', value: 60 },
            { label: '2min', value: 120 },
            { label: '5min', value: 300 },
            { label: '10min', value: 600 },
            { label: '30min', value: 1800 },
        ];

        const DITHER_STORAGE_KEY = 'damageChart.dithering';
        const dithering = ref(localStorage.getItem(DITHER_STORAGE_KEY) === 'true');

        const buildChartOpt = (): Options => {
            const seriesList = props.mode === 'cumulative'
                ? getCumulativeSeries(props.entities, tickSize.value)
                : props.mode === 'count'
                    ? getCountSeries(props.entities, tickSize.value)
                    : getDpsSeries(props.entities, tickSize.value);

            const n = props.entities.length;
            const gap = 3;
            const panelHeight = (100 - gap * (n - 1)) / n;

            return {
                ...baseChartOpt,
                yAxis: props.entities.map((entity, i) => ({
                    title: { text: entity.name, style: { fontSize: '11px' } },
                    top: `${(panelHeight + gap) * i}%`,
                    height: `${panelHeight}%`,
                    offset: 0,
                })),
                series: seriesList.map((s, i) => ({ ...s, yAxis: i })),
            };
        };

        const bucketDamages = (damages: EntityDamage[], binSize: number) => {
            const bins = new Map<number, number>();
            for (const d of damages) {
                const bt = d.At - (d.At % binSize);
                bins.set(bt, (bins.get(bt) || 0) + d.Damage);
            }
            return bins;
        };

        const bucketCounts = (damages: EntityDamage[], binSize: number) => {
            const bins = new Map<number, number>();
            for (const d of damages) {
                const bt = d.At - (d.At % binSize);
                bins.set(bt, (bins.get(bt) || 0) + 1);
            }
            return bins;
        };

        const getCumulativeSeries = (entities: ChartEntity[], tick: number) => {
            const series = entities.map((v): SeriesAreaOptions & { data: [number, number][] } => {
                const bins = bucketDamages(v.damages, tick);
                if (bins.size === 0) return { type: 'area', name: v.name, data: [] };

                const binTimes = [...bins.keys()].sort((a, b) => a - b);
                const data: [number, number][] = [];
                let cumSum = 0;
                for (let t = binTimes[0]; t <= binTimes[binTimes.length - 1]; t += tick) {
                    cumSum += bins.get(t) || 0;
                    data.push([t * 1000, cumSum]);
                }
                return { type: 'area', name: v.name, data };
            });

            alignSeries(series, true);
            return series;
        };

        const getDpsSeries = (entities: ChartEntity[], tick: number) => {
            if (!dithering.value) {
                const series = entities.map((v): SeriesAreaOptions & { data: [number, number][] } => {
                    const bins = bucketDamages(v.damages, tick);
                    if (bins.size === 0) return { type: 'area', name: v.name, data: [] };

                    const binTimes = [...bins.keys()].sort((a, b) => a - b);
                    const data: [number, number][] = [];
                    for (let t = binTimes[0]; t <= binTimes[binTimes.length - 1]; t += tick) {
                        data.push([t * 1000, bins.get(t) || 0]);
                    }
                    return { type: 'area', name: v.name, data };
                });
                alignSeries(series, false);
                return series;
            }

            // Dithering: 2-tick wide window, stepped by 1 tick, offset by halfTick
            // e.g. tick=5s → centers at 2.5, 7.5, 12.5... windows (−2.5,7.5], (2.5,12.5], (7.5,17.5]
            const halfTick = tick / 2;
            const series = entities.map((v): SeriesAreaOptions & { data: [number, number][] } => {
                const bins = new Map<number, number>();
                for (const d of v.damages) {
                    const bt = d.At - (d.At % halfTick);
                    bins.set(bt, (bins.get(bt) || 0) + d.Damage);
                }
                if (bins.size === 0) return { type: 'area', name: v.name, data: [] };

                const binTimes = [...bins.keys()].sort((a, b) => a - b);
                const minTime = binTimes[0];
                const maxTime = binTimes[binTimes.length - 1];
                const firstCenter = (minTime - (minTime % tick)) + halfTick;

                const data: [number, number][] = [];
                for (let center = firstCenter; center - tick <= maxTime; center += tick) {
                    let sum = 0;
                    // sum 4 halfTick bins covering [center-tick, center+tick)
                    for (let bt = center - tick; bt < center + tick; bt += halfTick) {
                        sum += bins.get(bt) || 0;
                    }
                    data.push([center * 1000, sum]);
                }
                return { type: 'area', name: v.name, data };
            });
            alignSeries(series, false);
            return series;
        };

        const getCountSeries = (entities: ChartEntity[], tick: number) => {
            const series = entities.map((v): SeriesAreaOptions & { data: [number, number][] } => {
                const bins = bucketCounts(v.damages, tick);
                if (bins.size === 0) return { type: 'area', name: v.name, data: [] };

                const binTimes = [...bins.keys()].sort((a, b) => a - b);
                const data: [number, number][] = [];
                for (let t = binTimes[0]; t <= binTimes[binTimes.length - 1]; t += tick) {
                    data.push([t * 1000, bins.get(t) || 0]);
                }
                return { type: 'area', name: v.name, data };
            });
            alignSeries(series, false);
            return series;
        };

        const alignSeries = (series: (SeriesAreaOptions & { data: [number, number][] })[], fillPrevious: boolean) => {
            for (let i = 0; i < (series[0]?.data.length || 0); i++) {
                const tickAt = Math.min(...series.map(v => v.data[i]?.[0] || Infinity));

                for (const s of series) {
                    if (s.data[i]?.[0] != tickAt) {
                        s.data.splice(i, 0, [tickAt, fillPrevious ? (s.data[i - 1]?.[1] || 0) : 0]);
                    }
                }
            }
        };

        const rebuildChart = () => {
            chart?.destroy();
            chart = highcharts.chart(chartDom.value, buildChartOpt());
        };

        onMounted(() => {
            chart = highcharts.chart(chartDom.value, buildChartOpt());
        });

        watch(tickSize, (v) => {
            localStorage.setItem(TICK_STORAGE_KEY, String(v));
            rebuildChart();
        });

        watch(dithering, (v) => {
            localStorage.setItem(DITHER_STORAGE_KEY, String(v));
            rebuildChart();
        });

        onUnmounted(() => {
            chart?.destroy();
        });

        const totalDamage = props.entities.reduce((sum, e) => sum + e.damages.reduce((s, d) => s + d.Damage, 0), 0);

        const allTimes = props.entities.flatMap(e => e.damages.map(d => d.At));
        const durationSec = allTimes.length >= 2 ? Math.max(...allTimes) - Math.min(...allTimes) : 0;
        const durationMin = Math.floor(durationSec / 60);
        const durationRemSec = Math.floor(durationSec % 60);
        const durationText = `${String(durationMin).padStart(2, '0')}:${String(durationRemSec).padStart(2, '0')}`;

        return { chartDom, tickSize, tickOptions, dithering, totalDamage, durationText, humanReadableNumber };
    },
});

const baseChartOpt: Options = {
    lang: { locale: 'en' },
    title: { text: '' },
    chart: {
        zooming: { type: 'x' },
        events: {
            selection: function (e): undefined {
                if ((e as any).resetSelection) {
                    clearTimeRange();
                    return;
                }
                const xAxis = (e as any).xAxis?.[0];
                if (xAxis) {
                    setTimeRange(xAxis.min / 1000, xAxis.max / 1000);
                }
            },
        },
    },
    xAxis: { type: 'datetime' },
    tooltip: { valueDecimals: 0 },
};
</script>
