<template>
    <v-card ref="el" class="floating-window elevation-8" :style="windowStyle" @pointerdown="bringToFront">
        <v-toolbar density="compact" color="primary" @pointerdown.stop="onDragStart"
            style="cursor: move; user-select: none;">
            <v-toolbar-title class="text-body-2">{{ title }}</v-toolbar-title>
            <v-btn :icon="minimized ? 'mdi-window-maximize' : 'mdi-window-minimize'" size="small" variant="text"
                @click="minimized = !minimized" @pointerdown.stop />
            <v-btn icon="mdi-close" size="small" variant="text" @click="$emit('close')" @pointerdown.stop />
        </v-toolbar>
        <template v-if="!minimized">
            <v-card-text class="pa-0 flex-grow-1 d-flex flex-column" style="overflow: hidden; min-height: 0;">
                <slot />
            </v-card-text>
            <div class="floating-window__resize" @pointerdown.stop="onResizeStart" />
        </template>
    </v-card>
</template>

<script lang="ts">
import { defineComponent, ref, computed, onMounted } from 'vue';

let topZIndex = 100;

export default defineComponent({
    props: {
        title: { type: String, default: '' },
        initWidth: { type: Number, default: 600 },
        initHeight: { type: Number, default: 500 },
    },
    emits: ['close'],
    setup(props) {
        const el = ref<HTMLElement>();
        const x = ref(0);
        const y = ref(0);
        const w = ref(props.initWidth);
        const h = ref(props.initHeight);
        const zIndex = ref(++topZIndex);
        const minimized = ref(false);

        onMounted(() => {
            x.value = Math.max(0, (window.innerWidth - w.value) / 2);
            y.value = Math.max(0, (window.innerHeight - h.value) / 2);
        });

        const windowStyle = computed(() => ({
            position: 'fixed' as const,
            left: `${x.value}px`,
            top: `${y.value}px`,
            width: minimized.value ? '300px' : `${w.value}px`,
            height: minimized.value ? 'auto' : `${h.value}px`,
            zIndex: zIndex.value,
            display: 'flex',
            flexDirection: 'column' as const,
        }));

        const bringToFront = () => {
            zIndex.value = ++topZIndex;
        };

        const onDragStart = (e: PointerEvent) => {
            bringToFront();
            const startX = e.clientX - x.value;
            const startY = e.clientY - y.value;
            const target = e.currentTarget as HTMLElement;
            target.setPointerCapture(e.pointerId);

            const onMove = (e: PointerEvent) => {
                x.value = Math.max(0, e.clientX - startX);
                y.value = Math.max(0, e.clientY - startY);
            };
            const onUp = () => {
                target.removeEventListener('pointermove', onMove);
                target.removeEventListener('pointerup', onUp);
            };
            target.addEventListener('pointermove', onMove);
            target.addEventListener('pointerup', onUp);
        };

        const onResizeStart = (e: PointerEvent) => {
            bringToFront();
            const startX = e.clientX;
            const startY = e.clientY;
            const startW = w.value;
            const startH = h.value;
            const target = e.currentTarget as HTMLElement;
            target.setPointerCapture(e.pointerId);

            const onMove = (e: PointerEvent) => {
                w.value = Math.max(300, startW + (e.clientX - startX));
                h.value = Math.max(200, startH + (e.clientY - startY));
            };
            const onUp = () => {
                target.removeEventListener('pointermove', onMove);
                target.removeEventListener('pointerup', onUp);
            };
            target.addEventListener('pointermove', onMove);
            target.addEventListener('pointerup', onUp);
        };

        return { el, windowStyle, bringToFront, onDragStart, onResizeStart, minimized };
    },
});
</script>

<style scoped>
.floating-window__resize {
    position: absolute;
    right: 0;
    bottom: 0;
    width: 16px;
    height: 16px;
    cursor: nwse-resize;
}

.floating-window__resize::after {
    content: '';
    position: absolute;
    right: 3px;
    bottom: 3px;
    width: 8px;
    height: 8px;
    border-right: 2px solid rgba(255, 255, 255, 0.3);
    border-bottom: 2px solid rgba(255, 255, 255, 0.3);
}
</style>
