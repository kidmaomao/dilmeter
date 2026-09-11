<template>
    <template v-for="cond in filteredConditions" v-bind:key="cond.CCId">
        <img width="16" height="16"
            @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
            @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
            @click="e => onClickCond(e as MouseEvent, cond)"
            :src='`/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
    </template>

    <v-tooltip v-if="condTooltip" v-model="condTooltipValue" :activator="condTooltipParent">
        {{ condNameMap[condTooltip.CCId] }}
    </v-tooltip>
</template>

<script lang="ts">
import { defineComponent, PropType, inject, ref, computed, Ref } from 'vue';
import type { EntityCondition } from '@/eventActor';
import { addHiddenCC } from '@/store';

export default defineComponent({
    props: {
        conditions: {
            type: Array as PropType<EntityCondition[]>,
            required: true,
        },
    },
    setup(props) {
        const region = inject('region');
        const condNameMap = inject('condNameMap') as Ref<Record<number, string>>;
        const hiddenCCIds = inject('hiddenCCIds') as Ref<Set<number>>;

        const filteredConditions = computed(() =>
            props.conditions.filter(c => !hiddenCCIds.value.has(c.CCId))
        );

        const condTooltipParent = ref<HTMLElement>();
        const condTooltipValue = ref(false);
        const condTooltip = ref<EntityCondition>();

        const setCondTooltip = (el: HTMLElement, cond?: EntityCondition) => {
            condTooltip.value = cond;
            condTooltipParent.value = el;
            condTooltipValue.value = !!cond;
        }

        const onClickCond = (e: MouseEvent, cond: EntityCondition) => {
            if (e.shiftKey) {
                const name = condNameMap.value[cond.CCId] ?? `CC ${cond.CCId}`;
                if (confirm(`${name}을(를) 숨기시겠습니까?`)) {
                    addHiddenCC(cond.CCId);
                }
                return;
            }
            setCondTooltip(e.target! as HTMLElement, cond);
        }

        return {
            region,
            condNameMap,
            filteredConditions,
            condTooltip,
            condTooltipParent,
            condTooltipValue,
            setCondTooltip,
            onClickCond,
        }
    }
});

</script>
