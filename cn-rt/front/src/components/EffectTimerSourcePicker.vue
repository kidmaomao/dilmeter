<template>
    <v-menu v-model="open" :close-on-content-click="false" location="bottom start" :offset="4" :min-width="280" :max-width="420" @after-enter="focusSearch">
        <template #activator="{ props: activatorProps }">
            <button v-bind="activatorProps" type="button" class="effect-source-trigger" :aria-label="translate('选择技能或状态')" :title="selectedName">
                <span>{{ translate(selectedName) }}</span><v-icon icon="mdi-chevron-down" size="14" />
            </button>
        </template>
        <div class="effect-source-menu" @keydown.esc.stop.prevent="open = false">
            <label :for="searchId">{{ translate(sourceType === 'skill' ? '选择技能' : '选择角色状态') }}</label>
            <input
                :id="searchId" ref="searchInput" v-model="query" type="search" autocomplete="off"
                :placeholder="translate('搜索名称或 ID')" :aria-label="translate('搜索名称或 ID')"
                role="combobox" aria-expanded="true" aria-autocomplete="list" :aria-controls="listId"
                :aria-activedescendant="filteredOptions.length ? `${listId}-${activeIndex}` : undefined"
                @keydown.down.stop.prevent="moveActive(1)" @keydown.up.stop.prevent="moveActive(-1)"
                @keydown.enter.stop="selectActive"
            />
            <v-virtual-scroll
                v-if="filteredOptions.length" :id="listId" ref="optionList" :items="filteredOptions"
                :height="Math.min(252, filteredOptions.length * 42)" :item-height="42" role="listbox" :aria-label="translate('技能或状态列表')"
            >
                <template #default="{ item, index }">
                    <button
                        :id="`${listId}-${index}`" type="button" role="option" :aria-selected="item.id === sourceId"
                        class="effect-source-option" :class="{ active: index === activeIndex, selected: item.id === sourceId }"
                        @click="selectOption(item)" @pointermove="activeIndex = index"
                    >
                        <span><strong>{{ translate(item.name) }}</strong><small>{{ translate(sourceType === 'skill' ? '技能' : '状态') }} ID {{ item.id }}</small></span>
                        <v-icon v-if="item.id === sourceId" icon="mdi-check" size="14" />
                    </button>
                </template>
            </v-virtual-scroll>
            <p v-else class="effect-source-empty">{{ translate('没有匹配项，可直接输入数字 ID。') }}</p>
            <small class="effect-source-count">{{ translate(`共 ${filteredOptions.length} 项`) }}</small>
        </div>
    </v-menu>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, useId, watch } from 'vue';
import type { EffectTimerSourceType } from '@/effectTimer';

interface SourceOption { id: number; name: string }
const props = withDefaults(defineProps<{
    sourceType: EffectTimerSourceType;
    sourceId: number;
    options: SourceOption[];
    normalizeSearch: (text: string) => string;
    translate?: (text: string) => string;
}>(), { translate: (text: string) => text });
const emit = defineEmits<{ select: [option: SourceOption] }>();
const open = ref(false);
const query = ref('');
const activeIndex = ref(0);
const searchInput = ref<HTMLInputElement | null>(null);
const optionList = ref<{ scrollToIndex: (index: number) => void } | null>(null);
const searchId = `effect-source-search-${useId()}`;
const listId = `effect-source-list-${useId()}`;
const fallbackName = (id: number) => `${props.sourceType === 'skill' ? '技能' : '状态'} ${id}`;
const selectedName = computed(() => props.options.find(item => item.id === props.sourceId)?.name || fallbackName(props.sourceId));
const filteredOptions = computed(() => {
    const text = props.normalizeSearch(query.value.trim()).toLowerCase();
    const options = props.options.filter(item =>
        !text || props.normalizeSearch(item.name).toLowerCase().includes(text) || String(item.id).includes(text),
    ).sort((a, b) => a.id - b.id);
    if (/^\d+$/.test(text)) {
        const id = Number(text);
        const min = props.sourceType === 'skill' ? 1 : 0;
        if (Number.isSafeInteger(id) && id >= min && id <= 4_294_967_295) {
            const exact = options.find(item => item.id === id) || { id, name: fallbackName(id) };
            return [exact, ...options.filter(item => item.id !== id)];
        }
    }
    return options;
});

watch(open, value => {
    if (!value) return;
    query.value = '';
    activeIndex.value = Math.max(0, filteredOptions.value.findIndex(item => item.id === props.sourceId));
});
watch(filteredOptions, options => {
    const selected = query.value.trim() ? -1 : options.findIndex(item => item.id === props.sourceId);
    activeIndex.value = Math.max(0, selected);
    void nextTick(() => optionList.value?.scrollToIndex(activeIndex.value));
}, { immediate: true });

function focusSearch() {
    searchInput.value?.focus();
    optionList.value?.scrollToIndex(activeIndex.value);
}

function moveActive(direction: number) {
    if (!filteredOptions.value.length) return;
    activeIndex.value = Math.min(filteredOptions.value.length - 1, Math.max(0, activeIndex.value + direction));
    optionList.value?.scrollToIndex(activeIndex.value);
}

function selectActive(event: KeyboardEvent) {
    if (event.isComposing) return;
    event.preventDefault();
    const option = filteredOptions.value[activeIndex.value];
    if (option) selectOption(option);
}

function selectOption(option: SourceOption) {
    emit('select', option);
    open.value = false;
}
</script>

<style scoped>
.effect-source-trigger {
    display: flex; align-items: center; justify-content: space-between; gap: 8px;
    width: 100%; min-width: 0; height: 25px; padding: 0 7px; border-radius: 3px;
    color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border);
    font-size: 10px; text-align: left;
}
.effect-source-trigger > span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.effect-source-trigger:hover, .effect-source-trigger:focus-visible { border-color: var(--ui-color-accent); }
.effect-source-menu {
    padding: 8px; border-radius: 4px; color: var(--ui-theme-text); background: var(--ui-theme-surface);
    border: 1px solid var(--ui-theme-border); box-shadow: 0 5px 16px #0006; font-size: 11px;
}
.effect-source-menu > label { display: block; margin-bottom: 6px; font-weight: 700; }
.effect-source-menu > input {
    box-sizing: border-box; width: 100%; height: 29px; margin-bottom: 6px; padding: 0 7px;
    color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); outline: none;
}
.effect-source-menu > input:focus { border-color: var(--ui-color-accent); }
.effect-source-option {
    display: flex; align-items: center; justify-content: space-between; width: 100%; height: 42px; padding: 3px 7px;
    color: var(--ui-theme-text); text-align: left; border-radius: 2px;
}
.effect-source-option > span { display: grid; min-width: 0; }
.effect-source-option strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 11px; font-weight: 600; }
.effect-source-option small, .effect-source-count, .effect-source-empty { color: var(--ui-theme-muted); font-size: 10px; }
.effect-source-option.active { background: var(--ui-theme-surface-hover); }
.effect-source-option.selected { color: var(--ui-color-accent); }
.effect-source-count { display: block; margin-top: 5px; }
.effect-source-empty { padding: 10px 4px; }
</style>
