<template>
    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.id">
        <v-expansion-panel>
            <v-expansion-panel-title>
                <v-sheet>
                    {{ getDisplayName(v.name) }} {{ raceNameMap[v.raceId] }} {{ v.guildName }}
                </v-sheet>
            </v-expansion-panel-title>
            <v-expansion-panel-text class="pa-3">
                <v-sheet width="100%" class="mb-2">
                    h: {{ v.body.Height.toFixed(2) }} w: {{ v.body.Weight.toFixed(2) }} u: {{
                        v.body.Upper.toFixed(2) }} l: {{ v.body.Lower.toFixed(2) }}
                </v-sheet>
                <v-sheet width="100%" class="mb-2">
                    <condition-image-list :conditions="Object.values(v.conditionMap).sort((a, b) => a.CCId - b.CCId)" />
                </v-sheet>

                <v-sheet width="100%"
                    v-for="item in Object.values(v.equipItemMap).sort((a, b) => a.PocketType - b.PocketType)"
                    v-bind:key="item.PocketType" class="d-flex mb-2">
                    <v-sheet width="48px" height="96px"
                        :style='`background: url("/res/invimage/${region}/${item.ItemId}/${item.ItemId}.png") no-repeat; background-position: center;`' />
                    <v-sheet>
                        {{ itemNameMap[item.ItemId] }} {{ item.PocketType }}
                    </v-sheet>
                </v-sheet>
            </v-expansion-panel-text>
        </v-expansion-panel>
    </v-expansion-panels>
</template>

<script lang="ts">
import { defineComponent, inject, computed, onMounted } from "vue";
import { getDisplayName } from "@/store";

import { getMabiNameColor } from '@/lib/util';

import ConditionImageList from './subComponents/conditionImageList.vue';

export default defineComponent({
    components: {
        ConditionImageList,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const condNameMap = inject('condNameMap');
        const itemNameMap = inject('itemNameMap');
        const actorManager = inject('actorManager');

        const pcEntities = computed(() =>
            Object.values(actorManager.value.entityMap).filter(v => v.isPC).sort((a, b) => a.name.localeCompare(b.name)));

        onMounted(() => {
            console.log(pcEntities);
        });

        return {
            isLoading,
            region,

            pcEntities,
            raceNameMap,
            condNameMap,
            itemNameMap,

            getMabiNameColor,
            getDisplayName,
        }
    }
});

</script>