<template>
    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.actor.id">
        <template v-if="v.totalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <div class="d-flex align-center" style="width: 100%; gap: 4px;">
                        <span class="font-weight-medium">{{ prettyEntityName(v.actor) }}</span>
                        <v-btn v-if="v.actor.conditionHistory.length > 0"
                            icon="mdi-chart-timeline" size="x-small" variant="text"
                            @click.stop="showConditionChart(v.actor)" />
                        <v-spacer />
                        <span style="min-width: 48px; text-align: center; font-size: 0.85em; opacity: 0.7;">{{ formatDuration(v.damages.length >= 2 ? v.damages[v.damages.length - 1].At - v.damages[0].At : 0) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(v.totalDamage) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(v.damages.length >= 2 && v.damages[v.damages.length - 1].At > v.damages[0].At ? v.totalDamage / (v.damages[v.damages.length - 1].At - v.damages[0].At) : 0) }}</span>
                        <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * v.totalDamage / allApplyDamage).toFixed(1) }}%</span>
                    </div>
                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <v-sheet
                        v-for="[targetId, damageToTarget] in Object.entries(v.groupedTotalDamages).sort(([, av], [, bv]) => bv - av)"
                        v-bind:key="targetId" width="100%" class="mb-2"
                        style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer;"
                        @click.stop="showEntityDetailDamageList(v.actor.id, targetId)">
                        <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${Math.round(100 * damageToTarget / v.totalDamage)}%`, background: getMabiNameColor(prettyEntityName(entityMap[targetId]?.actor) || targetId), opacity: 0.4 }" />
                        <div class="d-flex align-center pa-1" style="position: relative; gap: 4px;">
                            <span class="font-weight-medium">{{ prettyEntityName(entityMap[targetId]?.actor) || targetId }}</span>
                            <v-spacer />
                            <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(damageToTarget) }}</span>
                            <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(arrayDps(damageToTarget, v.groupedDamages[targetId])) }}</span>
                            <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * damageToTarget / v.totalDamage).toFixed(1) }}%</span>
                        </div>
                    </v-sheet>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <v-sheet width="100%" class="mb-2"
                style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer;"
                @click.stop="showEntityAllDamageList(v.actor.id)">
                <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${Math.round(100 * v.totalDamage / allApplyDamage)}%`, background: getMabiNameColor(prettyEntityName(v.actor)!), opacity: 0.4 }" />
                <div class="d-flex align-center pa-1" style="position: relative; gap: 4px;">
                    <span class="font-weight-medium">{{ prettyEntityName(v.actor) }}</span>
                    <v-spacer />
                    <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(v.totalDamage) }}</span>
                    <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(arrayDps(v.totalDamage, v.damages)) }}</span>
                    <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * v.totalDamage / allApplyDamage).toFixed(1) }}%</span>
                </div>
            </v-sheet>
        </template>

    </v-expansion-panels>


</template>

<script lang="ts">
import { defineComponent, inject, computed, onUnmounted, type Ref } from "vue";

import { getMabiNameColor, prettyEntityName, humanReadableNumber, formatDuration } from '@/lib/util';
import { getDisplayName } from "@/store";
import type { EntityDamage, EntityActor } from '@/eventActor';
import { GroupedDamageCollector } from '@/actionCollector';
import { useDialogStack } from '@/lib/useDialogStack';
import { filterByTimeRange, computeGroupedStats } from '@/lib/timeRangeFilter';

import ConditionChart from '@/components/subComponents/conditionChart.vue';
import DamageList from '@/components/subComponents/damageList.vue';

export default defineComponent({
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');
        const timeRangeMin = inject('timeRangeMin');
        const timeRangeMax = inject('timeRangeMax');

        const damageCollectorMap: Record<string, GroupedDamageCollector> = {};

        onUnmounted(() => {
            for (const v of Object.values(damageCollectorMap)) {
                dcManager.value.removeDamageCollector(v);
            }
        });

        const getDC = (attackerId: string) => {
            const key = `${attackerId}`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key];
            }

            const dc = dcManager.value.getGroupedDamageCollector(v => v.Id == attackerId, v => v.TargetId);
            damageCollectorMap[key] = dc;

            return dc;
        }

        const dialogStack = useDialogStack();

        const showEntityDetailDamageList = (attackerId: string, targetId: string) => {
            const entity = filteredEntityMap.value[attackerId];
            if (!entity) {
                return;
            }

            const attackerName = prettyName(entity.actor)!;
            const targetName = prettyName(entityMap.value[targetId]?.actor) || targetId;
            dialogStack.open(DamageList, {
                attackerName,
                targetName,
                damages: entity.groupedDamages[targetId],
            }, `${attackerName} → ${targetName}`);
        }

        const showEntityAllDamageList = (attackerId: string) => {
            const entity = filteredEntityMap.value[attackerId];
            if (!entity) {
                return;
            }

            const attackerName = prettyName(entity.actor)!;
            dialogStack.open(DamageList, {
                attackerName,
                targetName: 'All',
                damages: entity.damages,
            }, `${attackerName} → All`);
        }

        const entityMap = computed(() => {
            const m: Record<string, { actor: EntityActor, dc: GroupedDamageCollector }> = {};

            for (const k in actorManager.value.entityMap) {
                const v = actorManager.value.entityMap[k];

                m[k] = {
                    actor: v,
                    dc: getDC(k),
                }
            }

            return m;
        });

        type EntityFiltered = {
            actor: EntityActor,
            dc: GroupedDamageCollector,
            totalDamage: number,
            damages: EntityDamage[],
            groupedTotalDamages: Record<string, number>,
            groupedDamages: Record<string, EntityDamage[]>,
        };

        const filteredEntityMap = computed(() => {
            const m: Record<string, EntityFiltered> = {};
            const minAt = timeRangeMin.value;
            const maxAt = timeRangeMax.value;
            const hasFilter = minAt !== null && maxAt !== null;

            for (const k in entityMap.value) {
                const v = entityMap.value[k];

                if (hasFilter) {
                    const damages = filterByTimeRange(v.dc.damages, minAt, maxAt);
                    const stats = computeGroupedStats(damages, d => d.TargetId);
                    m[k] = { ...v, totalDamage: stats.totalDamage, damages, groupedTotalDamages: stats.groupedTotalDamages, groupedDamages: stats.groupedDamages };
                } else {
                    m[k] = { ...v, totalDamage: v.dc.totalDamage, damages: v.dc.damages, groupedTotalDamages: v.dc.groupedTotalDamages, groupedDamages: v.dc.groupedDamages };
                }
            }
            return m;
        });

        const pcEntities = computed(() =>
            Object.values(filteredEntityMap.value).filter(v => v.actor.isPC).sort((a, b) => b.totalDamage - a.totalDamage));

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalDamage, 0));

        const arrayDps = (totalDamage: number, damages: EntityDamage[]) => {
            if (damages.length < 2) return 0;
            const duration = damages[damages.length - 1].At - damages[0].At;
            return duration > 0 ? totalDamage / duration : 0;
        };

        const prettyName = (entity?: EntityActor) => {
            const raw = prettyEntityName(entity, raceNameMap);
            return raw != null ? getDisplayName(raw) : raw;
        };

        const showConditionChart = (actor: EntityActor) => {
            const name = prettyName(actor) || actor.id;
            dialogStack.open(ConditionChart, {
                conditionHistory: actor.conditionHistory,
            }, `CC - ${name}`);
        };

        return {
            isLoading,
            region,

            pcEntities,
            allApplyDamage,

            skillNameMap,
            entityMap,

            showEntityDetailDamageList,
            showEntityAllDamageList,
            showConditionChart,
            arrayDps,
            getMabiNameColor,
            humanReadableNumber,
            formatDuration,
            prettyEntityName: prettyName,
        }
    }
});

</script>