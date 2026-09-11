<template>
    <v-sheet class="d-flex align-center ma-2 flex-wrap" style="gap: 8px;">
        <v-autocomplete v-model="targetId" :items="targetIdList"
            :item-title="vv => `${vv[0] ? prettyEntityName(entityMap[vv[0]]?.actor) : 'all'} ${humanReadableNumber(vv[1] || 0)}`"
            :item-value="vv => vv[0]"
            :custom-filter="(value, query, item) => !query || (item?.raw[0] ? prettyEntityName(entityMap[item.raw[0]]?.actor) ?? '' : 'all').toLowerCase().includes(query.toLowerCase())"
            variant="outlined" density="compact" hide-details style="flex: 1; min-width: 150px;"
            clearable auto-select-first>
        </v-autocomplete>
        <v-text-field
            v-model="playerFilterText"
            prepend-inner-icon="mdi-magnify"
            placeholder="篩選玩家"
            variant="outlined"
            density="compact"
            hide-details
            clearable
            style="flex: 1; min-width: 120px; max-width: 200px;"
        />
        <v-tooltip :text="showChartIcons ? '隱藏圖表按鈕' : '顯示圖表按鈕'">
            <template v-slot:activator="{ props: tp }">
                <v-btn v-bind="tp" @click="showChartIcons = !showChartIcons"
                    :color="showChartIcons ? 'primary' : 'default'"
                    size="small" icon="mdi-chart-bar" variant="outlined" />
            </template>
        </v-tooltip>
        <v-btn @click="showCumulativeChart" color="primary" size="small" prepend-icon="mdi-chart-bar">
            Cumulative</v-btn>
        <v-btn @click="showDpsChart" color="primary" size="small" prepend-icon="mdi-chart-bar">
            DPS</v-btn>
    </v-sheet>

    <v-expansion-panels multiple v-for="v in filteredPcEntities" v-bind:key="v.actor.id">
        <template v-if="v.totalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <div class="d-flex align-center" style="width: 100%; gap: 4px;">
                        <span class="font-weight-medium">{{ prettyEntityName(v.actor) }}</span>
                        <template v-if="showChartIcons">
                            <v-btn v-if="v.actor.conditionHistory.length > 0"
                                @click.stop="showConditionChart(v.actor)" size="x-small" icon="mdi-chart-timeline" variant="text" />
                            <v-tooltip text="Cumulative"><template v-slot:activator="{ props: tp }">
                                <v-btn v-bind="tp" @click.stop="showAttackerCumulativeChart(v)" size="x-small" icon="mdi-chart-bar" variant="text" />
                            </template></v-tooltip>
                            <v-tooltip text="DPS"><template v-slot:activator="{ props: tp }">
                                <v-btn v-bind="tp" @click.stop="showAttackerDpsChart(v)" size="x-small" icon="mdi-chart-bar" variant="text" />
                            </template></v-tooltip>
                        </template>
                        <v-spacer />
                        <span style="min-width: 48px; text-align: center; font-size: 0.85em; opacity: 0.7;">{{ formatDuration((v.damages.length >= 2 ? v.damages[v.damages.length - 1].At - v.damages[0].At : 0) / 1000) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(v.totalDamage) }}</span>
                        <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(v.damages.length >= 2 && v.damages[v.damages.length - 1].At > v.damages[0].At ? v.totalDamage / ((v.damages[v.damages.length - 1].At - v.damages[0].At) / 1000) : 0) }}</span>
                        <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * v.totalDamage / allApplyDamage).toFixed(1) }}%</span>
                    </div>
                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <v-sheet
                        v-for="[skillId, damageBySkill] in Object.entries(v.groupedTotalDamages).sort(([, av], [_, bv]) => bv - av)"
                        v-bind:key="skillId" class="mb-2"
                        style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer; background: rgba(255,255,255,0.12);"
                        @click.stop="showEntityDetailDamageList(v.actor.id, targetId, +skillId)">
                        <!-- bar fill -->
                        <div :style="{ position: 'absolute', left: 0, top: 0, bottom: 0, width: `${Math.round(100 * damageBySkill / v.totalDamage)}%`, background: getMabiNameColor(skillNameMap[+skillId] || `unknownSkill:${skillId}`), opacity: 0.4 }" />
                        <!-- row 1: icon, name, buttons, damage, % -->
                        <div class="d-flex align-center pa-1" style="position: relative; gap: 4px;">
                            <img width="28" height="28" :src="`/res/skillimage/${region}/${skillId}/${skillId}.png`" style="border-radius: 2px;" />
                            <span class="font-weight-medium">{{ skillNameMap[+skillId] || `unknownSkill:${skillId}` }}</span>
                            <template v-if="showChartIcons">
                                <v-tooltip text="Distribution"><template v-slot:activator="{ props: tp }">
                                    <v-btn v-bind="tp" @click.stop="showSkillDistribution(v, +skillId)" size="x-small" icon="mdi-chart-bar" variant="text" density="compact" />
                                </template></v-tooltip>
                                <v-tooltip text="Cumulative"><template v-slot:activator="{ props: tp }">
                                    <v-btn v-bind="tp" @click.stop="showSkillCumulativeChart(v, +skillId)" size="x-small" icon="mdi-chart-bar" variant="text" density="compact" />
                                </template></v-tooltip>
                                <v-tooltip text="DPS"><template v-slot:activator="{ props: tp }">
                                    <v-btn v-bind="tp" @click.stop="showSkillDpsChart(v, +skillId)" size="x-small" icon="mdi-chart-bar" variant="text" density="compact" />
                                </template></v-tooltip>
                                <v-tooltip text="Count"><template v-slot:activator="{ props: tp }">
                                    <v-btn v-bind="tp" @click.stop="showSkillCountChart(v, +skillId)" size="x-small" icon="mdi-chart-bar" variant="text" density="compact" />
                                </template></v-tooltip>
                            </template>
                            <v-spacer />
                            <span style="min-width: 80px; text-align: center; color: #FFD54F;">{{ humanReadableNumber(damageBySkill) }}</span>
                            <span style="min-width: 80px; text-align: center; color: #42A5F5;">{{ humanReadableNumber(arrayDps(damageBySkill, v.groupedDamages[+skillId])) }}</span>
                            <span style="min-width: 56px; text-align: center; color: #66BB6A;">{{ (100 * damageBySkill / v.totalDamage).toFixed(1) }}%</span>
                        </div>
                        <!-- row 2: detailed stats -->
                        <div class="d-flex pa-1 pl-9" style="position: relative; gap: 8px; font-size: 0.8em; opacity: 0.8;">
                            <span>count: {{ v.groupedCount[+skillId] }}</span>
                            <span>crit: {{ v.groupedCriticalCount[+skillId] }}</span>
                            <span>avg: {{ humanReadableNumber(v.groupedCount[+skillId] ? damageBySkill / v.groupedCount[+skillId] : 0) }}</span>
                            <span>min: {{ humanReadableNumber(v.groupedMinDamages[+skillId] || 0) }}</span>
                            <span>max: {{ humanReadableNumber(v.groupedMaxDamages[+skillId] || 0) }}</span>
                        </div>
                    </v-sheet>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <v-sheet width="100%" class="mb-2"
                style="position: relative; overflow: hidden; border-radius: 4px; cursor: pointer; background: rgba(255,255,255,0.12);"
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
import { defineComponent, inject, ref, computed, onUnmounted, onMounted, type Ref } from "vue";

import { getMabiNameColor, prettyEntityName, humanReadableNumber, formatDuration } from '@/lib/util';
import { getDisplayName } from "@/store";
import type { EntityDamage, EntityActor } from '@/eventActor';
import { DamageCollectorBase, DualGroupedDamageCollector, GroupedDamageCollector } from '@/actionCollector';
import { useDialogStack } from '@/lib/useDialogStack';
import { filterByTimeRange, computeGroupedStats } from '@/lib/timeRangeFilter';

import ConditionChart from '@/components/subComponents/conditionChart.vue';
import DamageChart from '@/components/subComponents/damageChart.vue';
import DamageDistribution from '@/components/subComponents/damageDistribution.vue';
import DamageList from '@/components/subComponents/damageList.vue';

export default defineComponent({
    components: {
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const appEvent = inject('appEvent');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');
        const timeRangeMin = inject('timeRangeMin');
        const timeRangeMax = inject('timeRangeMax');

        const damageCollectorMap: Record<string, DamageCollectorBase> = {};

        onUnmounted(() => {
            appEvent.value.removeEventListener('clear', clearTarget);

            for (const v of Object.values(damageCollectorMap)) {
                dcManager.value.removeDamageCollector(v);
            }
        });

        const getDC = (attackerId: string) => {
            const key = `${attackerId}`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key] as DualGroupedDamageCollector;
            }

            const dc = dcManager.value.getDualGroupedDamageCollector(v => v.Id == attackerId, v => v.TargetId, v => `${v.SkillId}`);
            damageCollectorMap[key] = dc;

            return dc;
        }

        const getTargetDC = () => {
            const key = `target`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key] as GroupedDamageCollector;
            }

            const dc = dcManager.value.getGroupedDamageCollector(() => true, v => v.TargetId);
            damageCollectorMap[key] = dc;

            return dc;
        }
        const targetDC = ref(getTargetDC());

        const dialogStack = useDialogStack();

        const showEntityDetailDamageList = (attackerId: string, targetId: string, skillId: number) => {
            const entity = entityMap.value[attackerId];
            if (!entity) {
                return;
            }

            const damages = targetId ?
                entity.dc.dualGroupedDamages[targetId][skillId] :
                entity.dc.grouped2Damages[skillId];

            const attackerName = prettyName(entity.actor)!;
            const targetName = skillNameMap.value[skillId] || `unknownSkill:${skillId}`;
            dialogStack.open(DamageList, {
                attackerName,
                targetName,
                damages: damages,
            }, `${attackerName} → ${targetName}`);
        }

        const showEntityAllDamageList = (attackerId: string) => {
            const entity = entityMapWithTargetData.value[attackerId];
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
            const m: Record<string, { actor: EntityActor, dc: DualGroupedDamageCollector }> = {};

            for (const k in actorManager.value.entityMap) {
                const v = actorManager.value.entityMap[k];

                m[k] = {
                    actor: v,
                    dc: getDC(k),
                }
            }

            return m;
        });

        const entityMapWithTargetData = computed(() => {
            const m: Record<string, EntityExtended> = {};
            const minAt = timeRangeMin.value;
            const maxAt = timeRangeMax.value;
            const hasFilter = minAt !== null && maxAt !== null;

            for (const k in entityMap.value) {
                const v = entityMap.value[k];

                const baseDamages = targetId.value ? v.dc.groupedDamages[targetId.value] || [] : v.dc.damages;
                const damages = filterByTimeRange(baseDamages, minAt, maxAt);

                if (hasFilter) {
                    const stats = computeGroupedStats(damages, d => `${d.SkillId}`);
                    m[k] = {
                        ...v,
                        totalDamage: stats.totalDamage,
                        damages,
                        groupedTotalDamages: stats.groupedTotalDamages,
                        groupedDamages: stats.groupedDamages,
                        groupedMinDamages: stats.groupedMinDamages,
                        groupedMaxDamages: stats.groupedMaxDamages,
                        groupedCount: stats.groupedCount,
                        groupedCriticalCount: stats.groupedCriticalCount,
                    };
                } else {
                    const totalDamage = targetId.value ? v.dc.groupedTotalDamages[targetId.value] || 0 : v.dc.totalDamage;
                    const groupedTotalDamages = targetId.value ? v.dc.dualGroupedTotalDamages[targetId.value] : v.dc.grouped2TotalDamages;
                    const groupedDamages = targetId.value ? v.dc.dualGroupedDamages[targetId.value] : v.dc.grouped2Damages;
                    const groupedMinDamages = targetId.value ? v.dc.dualGroupedMinDamages[targetId.value] : v.dc.grouped2MinDamages;
                    const groupedMaxDamages = targetId.value ? v.dc.dualGroupedMaxDamages[targetId.value] : v.dc.grouped2MaxDamages;
                    const groupedCount = targetId.value ? v.dc.dualGroupedCount[targetId.value] : v.dc.grouped2Count;
                    const groupedCriticalCount = targetId.value ? v.dc.dualGroupedCriticalCount[targetId.value] : v.dc.grouped2CriticalCount;

                    m[k] = {
                        ...v,
                        totalDamage,
                        damages,
                        groupedTotalDamages,
                        groupedDamages,
                        groupedMinDamages,
                        groupedMaxDamages,
                        groupedCount,
                        groupedCriticalCount,
                    };
                }
            }

            return m;
        })

        const pcEntities = computed(() =>
            Object.values(entityMapWithTargetData.value).filter(v => v.actor.isPC).sort((a, b) => b.totalDamage - a.totalDamage));

        const targetId = ref('');
        const targetIdList = computed(() => {
            const minAt = timeRangeMin.value;
            const maxAt = timeRangeMax.value;
            const hasFilter = minAt !== null && maxAt !== null;

            if (hasFilter) {
                const filtered = filterByTimeRange(targetDC.value.damages, minAt, maxAt);
                const stats = computeGroupedStats(filtered, d => d.TargetId);
                const list = Object.entries(stats.groupedTotalDamages)
                    .sort(([, av], [, bv]) => bv - av);
                list.unshift(['', stats.totalDamage]);
                return list;
            }

            const list = Object.entries(targetDC.value.groupedTotalDamages)
                .sort(([, av], [, bv]) => bv - av);

            list.unshift(['', targetDC.value.totalDamage]);

            return list;
        });

        const clearTarget = () => {
            targetId.value = '';
        }

        const showChartIcons = ref(true);
        const playerFilterText = ref('');

        const filteredPcEntities = computed(() => {
            const q = playerFilterText.value?.trim().toLowerCase();
            return pcEntities.value.filter(v =>
                v.totalDamage > 0 &&
                (!q || (prettyName(v.actor) || '').toLowerCase().includes(q))
            );
        });

        const getChartEntities = () =>
            pcEntities.value.filter(v => v.totalDamage > 10000).map(v => ({ name: prettyName(v.actor)!, damages: v.damages }));

        const getTargetTitle = () =>
            targetId.value ? prettyName(entityMap.value[targetId.value]?.actor) || targetId.value : 'All';

        const showCumulativeChart = () => {
            dialogStack.open(DamageChart, {
                entities: getChartEntities(),
                mode: 'cumulative',
            }, `Cumulative - ${getTargetTitle()}`);
        }

        const showDpsChart = () => {
            dialogStack.open(DamageChart, {
                entities: getChartEntities(),
                mode: 'dps',
            }, `DPS - ${getTargetTitle()}`);
        }

        const showAttackerCumulativeChart = (v: EntityExtended) => {
            const name = prettyName(v.actor)!;
            dialogStack.open(DamageChart, {
                entities: [{ name, damages: v.damages }],
                mode: 'cumulative',
            }, `Cumulative - ${name}`);
        }

        const showAttackerDpsChart = (v: EntityExtended) => {
            const name = prettyName(v.actor)!;
            dialogStack.open(DamageChart, {
                entities: [{ name, damages: v.damages }],
                mode: 'dps',
            }, `DPS - ${name}`);
        }

        const showSkillDistribution = (v: EntityExtended, skillId: number) => {
            const attackerName = prettyName(v.actor)!;
            const skillName = skillNameMap.value[skillId] || `unknownSkill:${skillId}`;
            const damages = v.groupedDamages?.[skillId] || [];
            dialogStack.open(DamageDistribution, { damages }, `${attackerName} - ${skillName} Distribution`);
        }

        const getSkillChartEntity = (v: EntityExtended, skillId: number) => {
            const skillName = skillNameMap.value[skillId] || `unknownSkill:${skillId}`;
            const damages = v.groupedDamages?.[skillId] || [];
            return { name: skillName, damages, attackerName: prettyName(v.actor)! };
        }

        const showSkillCumulativeChart = (v: EntityExtended, skillId: number) => {
            const e = getSkillChartEntity(v, skillId);
            dialogStack.open(DamageChart, {
                entities: [{ name: e.name, damages: e.damages }],
                mode: 'cumulative',
            }, `${e.attackerName} - ${e.name} Cumulative`);
        }

        const showSkillDpsChart = (v: EntityExtended, skillId: number) => {
            const e = getSkillChartEntity(v, skillId);
            dialogStack.open(DamageChart, {
                entities: [{ name: e.name, damages: e.damages }],
                mode: 'dps',
            }, `${e.attackerName} - ${e.name} DPS`);
        }

        const showSkillCountChart = (v: EntityExtended, skillId: number) => {
            const e = getSkillChartEntity(v, skillId);
            dialogStack.open(DamageChart, {
                entities: [{ name: e.name, damages: e.damages }],
                mode: 'count',
            }, `${e.attackerName} - ${e.name} Usage Count`);
        }

        onMounted(() => {
            appEvent.value.addEventListener('clear', clearTarget);
        })

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalDamage, 0));

        const arrayDps = (totalDamage: number, damages: EntityDamage[]) => {
            if (!damages || damages.length < 2) return 0;
            const durationMs = damages[damages.length - 1].At - damages[0].At;
            return durationMs > 0 ? totalDamage / (durationMs / 1000) : 0;
        };

        const prettyName = (entity?: EntityActor) => {
            const raw = prettyEntityName(entity, raceNameMap);
            return raw != null ? getDisplayName(raw) : raw;
        };

        const showConditionChart = (actor: EntityActor) => {
            const name = prettyName(actor) || actor.id;
            const props: Record<string, any> = {
                conditionHistory: actor.conditionHistory,
            };

            // if a target is selected, bound the chart to the damage time range
            const entity = entityMapWithTargetData.value[actor.id];
            if (targetId.value && entity && entity.damages.length >= 2) {
                props.startTime = entity.damages[0].At;
                props.endTime = entity.damages[entity.damages.length - 1].At;
            }

            dialogStack.open(ConditionChart, props, `CC - ${name}`);
        };

        return {
            isLoading,
            region,

            pcEntities,
            filteredPcEntities,
            allApplyDamage,
            targetId,
            targetIdList,
            skillNameMap,
            entityMap: entityMapWithTargetData,
            showChartIcons,
            playerFilterText,

            showCumulativeChart,
            showDpsChart,
            showAttackerCumulativeChart,
            showAttackerDpsChart,
            showSkillDistribution,
            showSkillCumulativeChart,
            showSkillDpsChart,
            showSkillCountChart,
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

type EntityExtended = {
    actor: EntityActor,
    dc: DualGroupedDamageCollector,
    totalDamage: number,
    damages: EntityDamage[],
    groupedTotalDamages: Record<string, number>,
    groupedDamages: Record<string, EntityDamage[]>,
    groupedMinDamages: Record<string, number>,
    groupedMaxDamages: Record<string, number>,
    groupedCount: Record<string, number>,
    groupedCriticalCount: Record<string, number>,
};


</script>