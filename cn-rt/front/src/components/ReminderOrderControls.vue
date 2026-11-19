<template>
    <div class="reminder-order-controls" role="group" aria-label="调整图标顺序">
        <button type="button" :disabled="disabled || index === 0" :aria-label="`上移 ${name}`" title="向前移动" @click="emit('move', -1)">
            <v-icon icon="mdi-chevron-up" size="15" />
        </button>
        <button type="button" :disabled="disabled || index === count - 1" :aria-label="`下移 ${name}`" title="向后移动" @click="emit('move', 1)">
            <v-icon icon="mdi-chevron-down" size="15" />
        </button>
    </div>
</template>

<script setup lang="ts">
defineProps<{ name: string; index: number; count: number; disabled?: boolean }>();
const emit = defineEmits<{ move: [direction: -1 | 1] }>();
</script>

<style scoped>
.reminder-order-controls { display: inline-flex; gap: 2px; }
button { display: grid; place-items: center; width: 24px; height: 24px; padding: 0; color: #e6e9e2; background: linear-gradient(#464941, #282a26); border: 1px solid #62685a; cursor: pointer; }
button:disabled { color: #666b62; border-color: #3c4038; cursor: default; }
</style>
