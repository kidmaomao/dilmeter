<template>
    <v-expansion-panels multiple
        v-for="v in visibleGroups"
        v-bind:key="v.actor.id">
        <template v-if="v.totalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <div class="d-flex align-center" style="width: 100%; gap: 4px;">
                        <span class="font-weight-medium">{{ prettyEntityName(v.actor) }}</span>
                        <v-btn v-if="!v.actor.isPC" icon="mdi-close" size="x-small" variant="text"
                            @click.stop="hideGroup(v.actor)" />
                        <v-spacer />
                        <span style="min-width: 48px; text-align: center; font-size: 0.85em; opacity: 0.7;">{{ groupDuration(v.groupedDamages) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(v.totalDamage) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(groupDps(v.totalDamage, v.groupedDamages)) }}</span>
                    </div>
                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <template v-for="entity, entityk in v.entity" v-bind:key="entityk">
                        <template v-if="entity.totalDamage > 0">
                            <v-sheet class="d-flex align-center mb-1" style="gap: 4px;">
                                <span>{{ prettyEntityName(entity.actor) }}</span>
                                <span v-if="entity.actor.finisherId" style="font-size: 0.85em; opacity: 0.7;">
                                    Killed by {{ prettyEntityName(entityMap[entity.actor.finisherId]?.actor) || entity.actor.finisherId }}
                                </span>
                                <condition-image-list :conditions="Object.values(entity.actor.conditionMap)" />
                                <v-btn v-if="entity.actor.conditionHistory.length > 0"
                                    icon="mdi-chart-timeline" size="x-small" variant="text"
                                    @click.stop="showConditionChart(entity.actor)" />
                                <v-spacer />
                                <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(entity.totalDamage) }}</span>
                                <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(groupDps(entity.totalDamage, entity.groupedDamages)) }}</span>
                                <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * entity.totalDamage / v.totalDamage).toFixed(1) }}%</span>
                            </v-sheet>

                            <v-sheet
                                v-for="[attackerId, damageByAttacker] in Object.entries(entity.groupedTotalDamages).sort(([, av], [, bv]) => bv - av)"
                                v-bind:key="attackerId" width="100%" class="mb-2"
                                style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer;"
                                @click.stop="showEntityDetailDamageList(entity.actor.id, attackerId)">
                                <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${Math.round(100 * damageByAttacker / entity.totalDamage)}%`, background: getMabiNameColor(prettyEntityName(entityMap[attackerId]?.actor) || attackerId), opacity: 0.4 }" />
                                <div class="d-flex align-center pa-1" style="position: relative; gap: 4px;">
                                    <span class="font-weight-medium">{{ prettyEntityName(entityMap[attackerId]?.actor) || attackerId }}</span>
                                    <v-spacer />
                                    <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(damageByAttacker) }}</span>
                                    <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(arrayDps(damageByAttacker, entity.groupedDamages[attackerId])) }}</span>
                                    <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * damageByAttacker / entity.totalDamage).toFixed(1) }}%</span>
                                </div>
                            </v-sheet>
                        </template>
                    </template>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <v-sheet
                v-for="[attackerId, damageByAttacker] in Object.entries(v.groupedTotalDamages).sort(([, av], [, bv]) => bv - av)"
                v-bind:key="attackerId" width="100%" class="mb-2"
                style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer;"
                @click.stop="showEntityGroupDetailDamageList(v.actor.id, attackerId)">
                <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${Math.round(100 * damageByAttacker / v.totalDamage)}%`, background: getMabiNameColor(prettyEntityName(entityMap[attackerId]?.actor) || attackerId), opacity: 0.4 }" />
                <div class="d-flex align-center pa-1" style="position: relative; gap: 4px;">
                    <span class="font-weight-medium">{{ prettyEntityName(entityMap[attackerId]?.actor) || attackerId }}</span>
                    <v-spacer />
                    <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(damageByAttacker) }}</span>
                    <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(arrayDps(damageByAttacker, v.groupedDamages[attackerId])) }}</span>
                    <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * damageByAttacker / v.totalDamage).toFixed(1) }}%</span>
                </div>
            </v-sheet>
        </template>

    </v-expansion-panels>


</template>

<script lang="ts">
import { defineComponent, inject, computed, onUnmounted, type Ref } from "vue";

import { getMabiNameColor, prettyEntityName, humanReadableNumber, formatDuration } from '@/lib/util';
import type { EntityDamage, EntityActor } from '@/eventActor';
import { GroupActor } from '@/eventActor';
import { GroupedDamageCollector } from '@/actionCollector';
import { useDialogStack } from '@/lib/useDialogStack';
import { filterByTimeRange, computeGroupedStats } from '@/lib/timeRangeFilter';
import { addHiddenRace } from '@/store';

import ConditionImageList from "./subComponents/conditionImageList.vue";
import ConditionChart from "./subComponents/conditionChart.vue";
import DamageList from "./subComponents/damageList.vue";

export default defineComponent({
    components: {
        ConditionImageList,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const condNameMap = inject('condNameMap');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');
        const timeRangeMin = inject('timeRangeMin') as Ref<number | null>;
        const timeRangeMax = inject('timeRangeMax') as Ref<number | null>;
        const hiddenRaceIds = inject('hiddenRaceIds') as Ref<Set<number>>;

        const damageCollectorMap: Record<string, GroupedDamageCollector> = {};

        onUnmounted(() => {
            for (const v of Object.values(damageCollectorMap)) {
                dcManager.value.removeDamageCollector(v);
            }
        });

        const getGroupDC = (groupActor: GroupActor) => {
            const key = `group:${groupActor.id}`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key];
            }

            const dc = dcManager.value.getGroupedDamageCollector(v => !!groupActor.entityMap[v.TargetId], v => v.Id);
            damageCollectorMap[key] = dc;

            return dc;
        }

        const getSingleDC = (targetId: string) => {
            const key = `${targetId}`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key];
            }

            const dc = dcManager.value.getGroupedDamageCollector(v => v.TargetId == targetId, v => v.Id);
            damageCollectorMap[key] = dc;

            return dc;
        }

        const dialogStack = useDialogStack();

        const showEntityDetailDamageList = (targetId: string, attackerId: string) => {
            const entity = filteredGroupMap.value;
            // find the entity across all groups
            for (const gk in entity) {
                const e = entity[gk].entity[targetId];
                if (e) {
                    const attackerName = prettyName(entityMap.value[attackerId]?.actor) || attackerId;
                    const targetName = prettyName(e.actor)!;
                    dialogStack.open(DamageList, {
                        attackerName,
                        targetName,
                        damages: e.groupedDamages[attackerId],
                    }, `${attackerName} → ${targetName}`);
                    return;
                }
            }
        }

        const showEntityGroupDetailDamageList = (targetId: string, attackerId: string) => {
            const group = filteredGroupMap.value[targetId];
            if (!group) {
                return;
            }

            const attackerName = prettyName(entityMap.value[attackerId]?.actor) || attackerId;
            const targetName = prettyName(group.actor)!;
            dialogStack.open(DamageList, {
                attackerName,
                targetName,
                damages: group.groupedDamages[attackerId],
            }, `${attackerName} → ${targetName}`);
        }

        const entityMap = computed(() => {
            const m: Record<string, { actor: EntityActor, dc: GroupedDamageCollector }> = {};

            for (const k in actorManager.value.entityMap) {
                const v = actorManager.value.entityMap[k];

                m[k] = {
                    actor: v,
                    dc: getSingleDC(k),
                }
            }

            return m;
        });

        const groupMap = computed(() => {
            const m: Record<string, {
                actor: GroupActor,
                dc: GroupedDamageCollector,
                entity: Record<string, { actor: EntityActor, dc: GroupedDamageCollector }>,
            }> = {};

            for (const k in actorManager.value.groupMap) {
                const v = actorManager.value.groupMap[k];
                const entity: Record<string, { actor: EntityActor, dc: GroupedDamageCollector }> = {};

                for (const ek in v.entityMap) {
                    entity[ek] = entityMap.value[ek];
                }

                m[k] = {
                    actor: v,
                    dc: getGroupDC(v),
                    entity,
                }
            }

            return m;
        });

        type FilteredEntry = {
            totalDamage: number,
            groupedTotalDamages: Record<string, number>,
            groupedDamages: Record<string, EntityDamage[]>,
        };

        type FilteredGroupEntry = FilteredEntry & {
            actor: GroupActor,
            dc: GroupedDamageCollector,
            entity: Record<string, FilteredEntry & { actor: EntityActor, dc: GroupedDamageCollector }>,
        };

        const filteredGroupMap = computed(() => {
            const minAt = timeRangeMin.value;
            const maxAt = timeRangeMax.value;
            const hasFilter = minAt !== null && maxAt !== null;

            const m: Record<string, FilteredGroupEntry> = {};

            for (const k in groupMap.value) {
                const v = groupMap.value[k];

                const filteredEntity: Record<string, FilteredEntry & { actor: EntityActor, dc: GroupedDamageCollector }> = {};
                for (const ek in v.entity) {
                    const e = v.entity[ek];
                    if (hasFilter) {
                        const damages = filterByTimeRange(e.dc.damages, minAt, maxAt);
                        const stats = computeGroupedStats(damages, d => d.Id);
                        filteredEntity[ek] = { ...e, totalDamage: stats.totalDamage, groupedTotalDamages: stats.groupedTotalDamages, groupedDamages: stats.groupedDamages };
                    } else {
                        filteredEntity[ek] = { ...e, totalDamage: e.dc.totalDamage, groupedTotalDamages: e.dc.groupedTotalDamages, groupedDamages: e.dc.groupedDamages };
                    }
                }

                if (hasFilter) {
                    const damages = filterByTimeRange(v.dc.damages, minAt, maxAt);
                    const stats = computeGroupedStats(damages, d => d.Id);
                    m[k] = { ...v, totalDamage: stats.totalDamage, groupedTotalDamages: stats.groupedTotalDamages, groupedDamages: stats.groupedDamages, entity: filteredEntity };
                } else {
                    m[k] = { ...v, totalDamage: v.dc.totalDamage, groupedTotalDamages: v.dc.groupedTotalDamages, groupedDamages: v.dc.groupedDamages, entity: filteredEntity };
                }
            }

            return m;
        });

        const prettyName = (entity?: EntityActor | GroupActor) => prettyEntityName(entity, raceNameMap);

        const visibleGroups = computed(() =>
            Object.values(filteredGroupMap.value)
                .filter(v => !hiddenRaceIds.value.has(v.actor.raceId))
                .sort((a, b) => b.totalDamage - a.totalDamage)
        );

        const groupDuration = (groupedDamages: Record<string, EntityDamage[]>) => {
            let minAt = Infinity;
            let maxAt = -Infinity;
            for (const damages of Object.values(groupedDamages)) {
                for (const d of damages) {
                    if (d.At < minAt) minAt = d.At;
                    if (d.At > maxAt) maxAt = d.At;
                }
            }
            return formatDuration(maxAt > minAt ? maxAt - minAt : 0);
        };

        const arrayDps = (totalDamage: number, damages: EntityDamage[]) => {
            if (!damages || damages.length < 2) return 0;
            const duration = damages[damages.length - 1].At - damages[0].At;
            return duration > 0 ? totalDamage / duration : 0;
        };

        const groupDps = (totalDamage: number, groupedDamages: Record<string, EntityDamage[]>) => {
            let minAt = Infinity;
            let maxAt = -Infinity;
            for (const damages of Object.values(groupedDamages)) {
                for (const d of damages) {
                    if (d.At < minAt) minAt = d.At;
                    if (d.At > maxAt) maxAt = d.At;
                }
            }
            const duration = maxAt > minAt ? maxAt - minAt : 0;
            return duration > 0 ? totalDamage / duration : 0;
        };

        const hideGroup = (actor: GroupActor) => {
            const name = prettyName(actor) || actor.id;
            if (confirm(`${name}을(를) 숨기시겠습니까?`)) {
                addHiddenRace(actor.raceId);
            }
        };

        const showConditionChart = (actor: EntityActor) => {
            const name = prettyName(actor) || actor.id;

            // find the entity in filteredGroupMap to get per-attacker damages
            let attackers: { name: string, startTime: number, endTime: number }[] = [];
            for (const gk in filteredGroupMap.value) {
                const e = filteredGroupMap.value[gk].entity[actor.id];
                if (e && e.groupedDamages) {
                    attackers = Object.entries(e.groupedDamages)
                        .filter(([, damages]) => damages.length >= 2)
                        .map(([attackerId, damages]) => ({
                            id: attackerId,
                            name: prettyName(entityMap.value[attackerId]?.actor) || attackerId,
                            startTime: damages[0].At,
                            endTime: damages[damages.length - 1].At,
                        }))
                        .sort((a, b) => (b.endTime - b.startTime) - (a.endTime - a.startTime));
                    break;
                }
            }

            dialogStack.open(ConditionChart, {
                conditionHistory: actor.conditionHistory,
                attackers,
            }, `CC - ${name}`);
        };

        return {
            isLoading,
            region,

            skillNameMap,
            condNameMap,
            entityMap,
            filteredGroupMap,

            visibleGroups,
            hideGroup,
            showEntityDetailDamageList,
            showEntityGroupDetailDamageList,
            showConditionChart,
            getMabiNameColor,
            humanReadableNumber,
            formatDuration,
            groupDuration,
            arrayDps,
            groupDps,
            prettyEntityName: prettyName,
            getGroupDC,
            getSingleDC,
        }
    }
});

</script>