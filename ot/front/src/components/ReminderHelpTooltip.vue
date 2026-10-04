<template>
    <v-tooltip v-model="open" :text="text" location="top" max-width="400" :open-on-click="false">
        <template #activator="{ props }">
            <button v-bind="props" type="button" class="reminder-help" :aria-label="label" @click="open = true">
                <v-icon icon="mdi-help-circle-outline" size="17" />
            </button>
        </template>
    </v-tooltip>
</template>

<script setup lang="ts">
import { ref } from 'vue';
withDefaults(defineProps<{ text: string; label?: string }>(), { label: '查看设置说明' });
const open = ref(false);
</script>

<style scoped>
.reminder-help { display: inline-grid; place-items: center; width: 26px; height: 26px; padding: 0; flex: 0 0 26px; border: 0; border-radius: 50%; color: var(--ui-theme-accent-text); background: transparent; cursor: help; vertical-align: middle; }
.reminder-help:hover, .reminder-help:focus-visible { background: var(--ui-theme-selection-soft); }
</style>
