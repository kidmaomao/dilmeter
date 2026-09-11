<template>
    <v-sheet class="d-flex flex-column" style="height: 100%;">
        <v-sheet class="d-flex align-center pa-2 flex-shrink-0 flex-wrap" style="gap: 12px;">
            <span>CC:</span>
            <v-select v-model="selectedCCId" :items="ccOptions" item-title="label" item-value="value"
                variant="outlined" density="compact" hide-details style="max-width: 300px;">
                <template v-slot:item="{ props: itemProps, item }">
                    <v-list-item v-bind="itemProps">
                        <template v-slot:prepend>
                            <img width="16" height="16" :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`" />
                        </template>
                    </v-list-item>
                </template>
                <template v-slot:selection="{ item }">
                    <div class="d-flex align-center" style="gap: 4px;">
                        <img width="16" height="16" :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`" />
                        <span>{{ item.raw.label }}</span>
                    </div>
                </template>
            </v-select>
            <v-btn v-if="selectedCCId != null" icon="mdi-close" size="x-small" variant="text"
                @click="hideSelectedCC" title="이 CC 숨기기" />
            <span>Total: {{ totalDurationText }}</span>
            <span>ON: {{ onDurationText }} ({{ onPercent }}%)</span>
        </v-sheet>
        <v-sheet v-if="attackerStats.length > 0" class="pa-2 flex-shrink-0" style="max-height: 200px; overflow-y: auto;">
            <v-sheet v-for="a in attackerStats" :key="a.name" width="100%" class="mb-1"
                style="position: relative; overflow: hidden; border-radius: 4px; font-size: 0.85em;">
                <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${a.onPercent}%`, background: getMabiNameColor(a.name), opacity: 0.4 }" />
                <div class="d-flex align-center pa-1" style="position: relative; gap: 8px;">
                    <span class="font-weight-medium">{{ a.name }}</span>
                    <v-spacer />
                    <span>ON: {{ a.onText }} ({{ a.onPercent }}%)</span>
                    <span>Total: {{ a.totalText }}</span>
                </div>
            </v-sheet>
        </v-sheet>
        <div ref="chartDom" style="flex: 1; min-height: 0; width: 100%;"></div>
    </v-sheet>
</template>

<script lang="ts">
import { defineComponent, ref, PropType, watch, computed, inject, onMounted, onUnmounted, Ref } from 'vue';
import highcharts, { Options, SeriesAreaOptions } from 'highcharts';
import type { EntityConditionState } from '@/eventActor';
import { getMabiNameColor, prettyEntityName } from '@/lib/util';
import { setTimeRange, clearTimeRange, addHiddenCC } from '@/store';

function formatDuration(seconds: number): string {
    const m = Math.floor(seconds / 60);
    const s = Math.floor(seconds % 60);
    return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

export default defineComponent({
    props: {
        conditionHistory: {
            type: Array as PropType<EntityConditionState[]>,
            required: true,
        },
        _dialogTitle: {
            type: Object as PropType<Ref<string>>,
        },
        startTime: {
            type: Number as PropType<number | null>,
            default: null,
        },
        endTime: {
            type: Number as PropType<number | null>,
            default: null,
        },
        attackers: {
            type: Array as PropType<{ id: string, name: string, startTime: number, endTime: number }[]>,
            default: () => [],
        },
    },
    setup(props) {
        const region = inject('region');
        const condNameMap = inject('condNameMap') as Ref<Record<number, string>>;
        const raceNameMap = inject('raceNameMap') as Ref<Record<number, string>>;
        const hiddenCCIds = inject('hiddenCCIds') as Ref<Set<number>>;
        const timeRangeMin = inject('timeRangeMin') as Ref<number | null>;
        const timeRangeMax = inject('timeRangeMax') as Ref<number | null>;
        const actorManager = inject('actorManager') as Ref<any>;

        const resolveAttackerId = (id: string): string => {
            const actor = actorManager.value.entityMap[id];
            return actor?.ownerId || id;
        };

        // bound conditionHistory by startTime/endTime props
        const boundedHistory = computed(() => {
            const s = props.startTime;
            const e = props.endTime;
            if (s === null || e === null) return props.conditionHistory;
            return props.conditionHistory.filter(st => st.At >= s && st.At <= e);
        });

        // collect all unique CCIds that ever appeared, excluding hidden
        const ccOptions = computed(() => {
            const ids = new Set<number>();
            for (const state of boundedHistory.value) {
                for (const cond of state.List) {
                    ids.add(cond.CCId);
                }
            }
            return [...ids]
                .filter(id => !hiddenCCIds.value.has(id))
                .sort((a, b) => a - b)
                .map(id => ({
                    value: id,
                    label: condNameMap.value[id] ?? `CC ${id}`,
                }));
        });

        const selectedCCId = ref<number | null>(ccOptions.value.length > 0 ? ccOptions.value[0].value : null);

        const hideSelectedCC = () => {
            if (selectedCCId.value == null) return;
            addHiddenCC(selectedCCId.value);
            // select next available CC
            selectedCCId.value = ccOptions.value.length > 0 ? ccOptions.value[0].value : null;
        };

        // filter by global time range (for stats only)
        const filteredHistory = computed(() => {
            const minAt = timeRangeMin.value;
            const maxAt = timeRangeMax.value;
            if (minAt === null || maxAt === null) return boundedHistory.value;
            return boundedHistory.value.filter(s => s.At >= minAt && s.At <= maxAt);
        });

        // compute total duration & on duration for selected CC
        const timeStats = computed(() => {
            const history = filteredHistory.value;
            if (history.length < 2 || selectedCCId.value == null) {
                return { totalSec: 0, onSec: 0 };
            }

            const ccId = selectedCCId.value;
            const totalSec = history[history.length - 1].At - history[0].At;
            let onSec = 0;

            for (let i = 0; i < history.length - 1; i++) {
                const active = history[i].List.some(c => c.CCId === ccId);
                if (active) {
                    onSec += history[i + 1].At - history[i].At;
                }
            }

            return { totalSec, onSec };
        });

        const totalDurationText = computed(() => formatDuration(timeStats.value.totalSec));
        const onDurationText = computed(() => formatDuration(timeStats.value.onSec));
        const onPercent = computed(() => {
            const { totalSec, onSec } = timeStats.value;
            if (totalSec <= 0) return '0.0';
            return (100 * onSec / totalSec).toFixed(1);
        });

        // per-attacker CC stats
        const attackerStats = computed(() => {
            if (props.attackers.length === 0 || selectedCCId.value == null) return [];
            const ccId = selectedCCId.value;
            const history = props.conditionHistory;

            return props.attackers.map(a => {
                const filtered = history.filter(s => s.At >= a.startTime && s.At <= a.endTime);
                const totalSec = filtered.length >= 2 ? filtered[filtered.length - 1].At - filtered[0].At : 0;
                let onSec = 0;
                for (let i = 0; i < filtered.length - 1; i++) {
                    if (filtered[i].List.some(c => c.CCId === ccId && resolveAttackerId(c.AttackerId) === a.id)) {
                        onSec += filtered[i + 1].At - filtered[i].At;
                    }
                }
                const pct = totalSec > 0 ? (100 * onSec / totalSec).toFixed(1) : '0.0';
                return {
                    name: a.name,
                    totalText: formatDuration(totalSec),
                    onText: formatDuration(onSec),
                    onPercent: pct,
                };
            });
        });

        // update popup title when CC selection changes
        const updateTitle = () => {
            if (!props._dialogTitle) return;
            const baseTitle = props._dialogTitle.value.split(' - ').slice(-1)[0];
            const ccId = selectedCCId.value;
            const ccName = ccId != null ? (condNameMap.value[ccId] ?? `CC ${ccId}`) : '';
            props._dialogTitle.value = ccName ? `${ccName} - ${baseTitle}` : baseTitle;
        };

        onMounted(() => {
            updateTitle();
        });

        watch(selectedCCId, () => {
            updateTitle();
        });

        // --- chart ---
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart | undefined;

        const buildSeriesList = (): SeriesAreaOptions[] => {
            if (selectedCCId.value == null || boundedHistory.value.length === 0) return [];

            const ccId = selectedCCId.value;
            const history = boundedHistory.value;

            // collect unique attacker IDs for the selected CC (resolved via owner)
            const attackerIds = new Set<string>();
            for (const state of history) {
                for (const cond of state.List) {
                    if (cond.CCId === ccId) {
                        attackerIds.add(resolveAttackerId(cond.AttackerId));
                    }
                }
            }

            if (attackerIds.size === 0) {
                // fallback: no AttackerId info, show single on/off series
                const data: [number, number][] = [];
                for (const state of history) {
                    const active = state.List.some(c => c.CCId === ccId) ? 1 : 0;
                    data.push([state.At * 1000, active]);
                }
                return [{ type: 'area', name: 'ON', data, step: 'left' }];
            }

            // per-attacker series
            const sorted = [...attackerIds];
            return sorted.map(attackerId => {
                const data: [number, number][] = [];
                for (const state of history) {
                    const active = state.List.some(c => c.CCId === ccId && resolveAttackerId(c.AttackerId) === attackerId) ? 1 : 0;
                    data.push([state.At * 1000, active]);
                }
                const actor = actorManager.value.entityMap[attackerId];
                const name = prettyEntityName(actor, raceNameMap) || 'unknown';
                return { type: 'area' as const, name, data, step: 'left' as const };
            });
        };

        const buildChartOpt = (): Options => {
            return {
                ...baseChartOpt,
                yAxis: {
                    title: { text: '' },
                    min: 0,
                    max: 1,
                    tickInterval: 1,
                    labels: {
                        formatter() { return this.value === 1 ? 'ON' : 'OFF'; },
                    },
                },
                plotOptions: {
                    area: {
                        stacking: 'normal',
                    },
                },
                series: buildSeriesList(),
            };
        };

        const rebuildChart = () => {
            chart?.destroy();
            if (chartDom.value) {
                chart = highcharts.chart(chartDom.value, buildChartOpt());
            }
        };

        onMounted(() => {
            chart = highcharts.chart(chartDom.value, buildChartOpt());
        });

        watch(selectedCCId, rebuildChart);

        onUnmounted(() => {
            chart?.destroy();
        });

        return {
            selectedCCId, ccOptions, region,
            totalDurationText, onDurationText, onPercent,
            attackerStats,
            getMabiNameColor,
            hideSelectedCC,
            chartDom,
        };
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
    tooltip: {
        formatter() {
            const time = new Date(this.x as number).toLocaleTimeString();
            const state = this.y === 1 ? 'ON' : 'OFF';
            return `${time}<br/><b>${this.series.name}</b>: ${state}`;
        },
    },
};
</script>
