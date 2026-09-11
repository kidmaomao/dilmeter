<template>
    <v-sheet class="d-flex flex-column" style="height: 100%;">
        <v-sheet width="100%" class="d-flex pa-2 mb-2 flex-shrink-0">
            {{ attackerName }} -> {{ targetName }}
        </v-sheet>

        <v-virtual-scroll :items="damages" style="flex: 1; min-height: 0;" item-height="80">
            <template v-slot:default="{ item }">
                <v-card min-height="80">
                    <v-sheet class="d-flex">
                        <v-sheet width="32" class="mr-2">
                            <img width="32" height="32"
                                :src='`/res/skillimage/${region}/${item.SkillId}/${item.SkillId}.png`' />
                        </v-sheet>

                        <v-sheet>
                            <p>
                                <condition-image-list :conditions="item.Conditions" />
                                ->
                                <condition-image-list :conditions="item.TargetConditions" />
                            </p>
                            <p>
                                {{ skillNameMap[item.SkillId] }} {{ humanReadableNumber(item.Damage) }}
                                {{ item.IsCritical ? '크리티컬' : '' }}
                                {{ item.IsDelayed ? '추가 대미지' : '' }}
                            </p>
                        </v-sheet>
                    </v-sheet>
                </v-card>
            </template>
        </v-virtual-scroll>
    </v-sheet>
</template>

<script lang="ts">
import { defineComponent, PropType, inject } from 'vue';

import type { EntityDamage } from '@/eventActor';
import { humanReadableNumber } from '@/lib/util';

import ConditionImageList from './conditionImageList.vue';

export default defineComponent({
    components: {
        ConditionImageList,
    },
    props: {
        attackerName: {
            type: String,
            required: true,
        },
        targetName: {
            type: String,
            required: true,
        },
        damages: {
            type: Array as PropType<EntityDamage[]>,
            required: true,
        },
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const skillNameMap = inject('skillNameMap');

        return {
            isLoading,
            region,
            skillNameMap,
            humanReadableNumber,
        }
    }
});

</script>
