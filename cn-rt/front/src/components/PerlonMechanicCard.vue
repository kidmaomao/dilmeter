<template>
    <div v-if="tornado" class="perlon-tornado-alert" :style="{ opacity }" :class="{ 'is-teammate': item.tornadoTarget === 'teammate' }">
        <v-icon icon="mdi-alert-outline" class="perlon-tornado-symbol" aria-hidden="true" />
        <span>{{ tornadoTitle }}</span>
        <strong>{{ tornadoActive ? '快躲' : remaining + 's' }}</strong>
    </div>
    <div v-else class="perlon-mechanic-card" :style="{ opacity }" :class="{ sun, 'area-warning': areaWarning }">
        <svg v-if="resonance" class="perlon-mode-icon" :data-mode="sun ? 'sun' : 'moon'" viewBox="0 0 24 24" aria-hidden="true">
            <template v-if="sun">
                <circle cx="12" cy="12" r="5" fill="currentColor" />
                <path d="M12 1v3m0 16v3M1 12h3m16 0h3M4.2 4.2l2.1 2.1m11.4 11.4 2.1 2.1M4.2 19.8l2.1-2.1M17.7 6.3l2.1-2.1" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" />
            </template>
            <path v-else d="M21 13.1A9.2 9.2 0 0 1 10.9 3a9.2 9.2 0 1 0 10.1 10.1Z" fill="currentColor" />
        </svg>
        <span class="perlon-title">{{ item.name }}</span>
        <strong class="perlon-countdown">{{ item.timingUnknown ? '--' : remaining }}</strong>
        <small class="perlon-instruction">{{ areaWarning ? '仍在区域 · 快离开' : item.timingUnknown ? '等待切换信号' : '预计切换 · 秒' }}</small>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { BossMechanicOverlayItem } from '@/skillCooldown';

const props = defineProps<{ item: BossMechanicOverlayItem; nowMs: number }>();
const opacity = computed(() => Math.min(100, Math.max(20, props.item.opacityPercent ?? 100)) / 100);
const resonance = computed(() => props.item.key === 'perlon-resonance');
const tornado = computed(() => props.item.key === 'perlon-tornado');
const countdownEnd = computed(() => tornado.value ? (props.item.tornadoStartsAtMs ?? props.item.endsAtMs) : props.item.endsAtMs);
const tornadoActive = computed(() => tornado.value && props.nowMs >= countdownEnd.value);
const tornadoTitle = computed(() => {
    if (props.item.tornadoTarget === 'teammate') return '队友 · 龙卷风';
    if (props.item.tornadoTarget === 'unknown') return '龙卷风 · 目标未知';
    return '尔格龙卷风';
});
const sun = computed(() => resonance.value && props.item.resonanceMode === 'release');
const areaWarning = computed(() => sun.value && (props.item.areaWarningUntilMs ?? 0) > props.nowMs);
const remaining = computed(() => {
    const seconds = Math.max(0, countdownEnd.value - props.nowMs) / 1000;
    return seconds >= 10 ? String(Math.ceil(seconds)) : (Math.ceil(seconds * 10) / 10).toFixed(1);
});
</script>

<style scoped>
.perlon-mechanic-card {
    --perlon-border: rgba(183, 212, 243, .98);
    --perlon-icon: #c8e3ff;
    --perlon-glow-rgb: 76, 163, 255;
    --perlon-rim-rgb: 235, 246, 255;
    --perlon-text-glow: #a6d2ff;
    --perlon-text-outline: rgba(8, 20, 32, .86);
    position: relative;
    box-sizing: border-box;
    width: 118px;
    height: 118px;
    overflow: hidden;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 5px;
    border: 2px solid var(--perlon-border);
    border-radius: 12px;
    /* Match the ordinary Boss reminders: light tint, bright rim and broad glow. */
    background: rgba(8, 10, 12, .14);
    box-shadow: 0 0 10px rgba(var(--perlon-rim-rgb), .78), 0 0 25px rgba(var(--perlon-glow-rgb), .72), inset 0 0 6px rgba(0, 0, 0, .12);
    color: #faf7ff;
    transform: scale(var(--boss-mechanic-scale, 1));
    transform-origin: top left;
}
.perlon-mechanic-card.sun { --perlon-border: rgba(244, 213, 138, .98); --perlon-icon: #ffd975; --perlon-glow-rgb: 255, 177, 62; --perlon-rim-rgb: 255, 243, 196; --perlon-text-glow: #ffce75; --perlon-text-outline: rgba(24, 12, 3, .86); }
.perlon-mode-icon { position: absolute; width: 88px; height: 88px; right: -5px; top: 13px; color: var(--perlon-icon); opacity: .24; filter: drop-shadow(0 0 5px rgba(var(--perlon-glow-rgb), .92)); }
.perlon-title, .perlon-countdown, .perlon-instruction { position: relative; z-index: 1; margin: 0; text-align: center; }
.perlon-title { font-size: 12px; font-weight: 700; line-height: 18px; text-shadow: 0 1px 3px #000; }
.perlon-countdown { align-self: center; font-size: 42px; line-height: 48px; font-weight: 800; letter-spacing: -1px; font-variant-numeric: tabular-nums; color: #fff; -webkit-text-stroke: 1.4px var(--perlon-text-outline); text-shadow: 0 0 7px #fff, 0 0 15px var(--perlon-text-glow), 0 2px 2px rgba(0, 0, 0, .62); }
.perlon-instruction { font-size: 11px; line-height: 17px; color: #f6f1ff; text-shadow: 0 1px 3px #000; }
.area-warning { animation: perlon-area-warning .7s steps(1, end) infinite; }
.area-warning .perlon-instruction { font-weight: 700; color: #fff; }
.perlon-tornado-alert {
    --tornado-color: #ff2c43;
    position: relative;
    width: 118px;
    height: 118px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: flex-end;
    gap: 2px;
    color: #fff;
    text-shadow: 0 1px 3px #000, 0 0 5px #000;
    transform: scale(var(--boss-mechanic-scale, 1));
    transform-origin: top left;
}
.perlon-tornado-alert.is-teammate { --tornado-color: #fff; }
.perlon-tornado-symbol {
    position: absolute;
    top: 0;
    width: 80px;
    height: 80px;
    font-size: 80px;
    color: var(--tornado-color);
    filter: drop-shadow(0 0 2px #000) drop-shadow(0 0 8px color-mix(in srgb, var(--tornado-color) 68%, transparent));
    animation: perlon-tornado-pulse 420ms ease-in-out 2;
}
.perlon-tornado-alert span { font-size: 12px; font-weight: 700; line-height: 16px; }
.perlon-tornado-alert strong { font-size: 22px; font-weight: 800; line-height: 24px; font-variant-numeric: tabular-nums; }
@keyframes perlon-tornado-pulse {
    0%, 100% { transform: scale(1); }
    50% { transform: scale(1.28); }
}
@media (prefers-reduced-motion: reduce) { .perlon-tornado-symbol { animation: none; } }
@keyframes perlon-area-warning {
    0%, 100% { filter: brightness(1.05); }
    50% { filter: brightness(1.62); border-color: #fff9df; box-shadow: 0 0 14px #fff, 0 0 34px rgba(var(--perlon-glow-rgb), 1), inset 0 0 10px rgba(var(--perlon-glow-rgb), .22); }
}
</style>
