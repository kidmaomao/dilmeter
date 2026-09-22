<template>
 <article v-if="group.kind === 'health'" class="healer-health-card" :style="style" aria-label="队友血量提醒">
  <div class="healer-health-heading"><span class="healer-cross">✚</span><strong :title="group.name">{{ group.name }}</strong><span>需要治疗</span></div>
  <div class="healer-health-value"><b>{{ group.cards[0]?.value }}</b><span>{{ group.cards[0]?.title }}</span></div>
  <div class="healer-health-track"><i :style="{ width: healthPercent + '%' }" /></div>
 </article>
 <article v-else class="healer-buff-strip" :style="style" aria-label="队友 Buff 提醒">
  <strong class="healer-owner" :style="{ width: group.nameWidth + 'px' }" :title="group.name"><span>队友</span>{{ group.name }}</strong>
  <div class="healer-buff-icons">
   <div v-for="card in group.cards" :key="card.key" class="healer-buff-tile" :class="{ 'is-warning': card.flash, 'is-missing': card.state === 'missing', 'is-unknown': card.state === 'unknown' }" :title="`${group.name} · ${card.title}`">
    <div class="healer-small-icon"><img :src="`/condition-icons/${card.ccId}.png`" :alt="card.title" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'" /><span v-if="card.state === 'missing'">!</span></div>
    <span class="healer-countdown">{{ card.value.replace(/s$/, '') }}</span>
   </div>
  </div>
 </article>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import type { HealerOverlayGroup } from '@/healerMonitorTypes';
const props = defineProps<{ group: HealerOverlayGroup; fontSize: number; iconSize: number; opacityPercent?: number }>();
const healthPercent = computed(() => Math.max(0, Math.min(100, parseFloat(props.group.cards[0]?.value || '0') || 0)));
const style = computed(() => ({ opacity: Math.max(20, Math.min(100, props.opacityPercent ?? 100)) / 100, '--healer-font': `${props.fontSize}px`, '--healer-icon': `${props.iconSize}px`, '--healer-cell': `${props.group.cellWidth || props.iconSize}px`, width: `${props.group.width}px`, height: `${props.group.height}px` }));
</script>
<style scoped>
.healer-health-card, .healer-buff-strip { box-sizing: border-box; color: #fff9ed; font-family: "Microsoft YaHei UI", "Microsoft YaHei", sans-serif; font-size: var(--healer-font); user-select: none; }
.healer-health-card { display: flex; flex-direction: column; justify-content: space-between; padding: 9px 12px; background: #24201b; border: 1px solid #edc67b; border-left: 3px solid #ffab69; border-radius: 4px; box-shadow: 0 0 9px #b66e3044, inset 0 0 18px #bb472214; }
.healer-health-heading { display: flex; align-items: center; gap: 7px; min-width: 0; line-height: 1.35; }
.healer-health-heading strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.healer-health-heading > span:last-child { margin-left: auto; flex-shrink: 0; font-size: .7em; color: #e5c9a5; }
.healer-cross { color: #ffc586; }
.healer-health-value { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
.healer-health-value b { font-size: 2em; line-height: 1.3; font-weight: 750; color: #fff8ed; font-variant-numeric: tabular-nums; text-shadow: 0 2px 2px #120d08; }
.healer-health-value > span { font-size: .75em; color: #e5bfa0; }
.healer-health-track { height: 3px; background: #42372b; border-radius: 2px; overflow: hidden; }
.healer-health-track i { display: block; height: 100%; background: #ff9d71; }
.healer-buff-strip { display: flex; align-items: center; gap: 12px; padding: 6px 8px; border-radius: 3px; background: #181b19; border: 1px solid #827759; box-shadow: 0 2px 6px #0008; }
.healer-owner { flex-shrink: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: #fff7df; line-height: 1.2; font-weight: 600; }
.healer-owner > span { display: block; font-size: min(12px, var(--healer-font)); font-weight: 400; color: #bfb18d; letter-spacing: 1px; }
.healer-buff-icons { display: flex; gap: 6px; align-items: flex-start; }
.healer-buff-tile { width: var(--healer-cell); display: flex; flex-direction: column; align-items: center; gap: 2px; }
.healer-small-icon { position: relative; box-sizing: border-box; width: var(--healer-icon); height: var(--healer-icon); border: 1px solid #c8b37b; background: #292a23; box-shadow: inset 0 0 2px #000; }
.healer-small-icon img { display: block; width: 100%; height: 100%; padding: 2px; object-fit: contain; image-rendering: auto; box-sizing: border-box; }
.healer-small-icon > span { position: absolute; inset: 0; display: grid; place-items: center; font-weight: 900; font-size: 1.2em; color: #ffd49a; text-shadow: 1px 1px #000; }
.healer-countdown { color: #fff; font-size: var(--healer-font); font-variant-numeric: tabular-nums; line-height: 1; white-space: nowrap; text-shadow: 1px 1px #000, -1px -1px #000; }
.is-missing img, .is-unknown img { opacity: .35; filter: grayscale(1); }
.is-warning .healer-small-icon { animation: healer-buff-flash .8s ease-in-out infinite alternate; }
@keyframes healer-buff-flash { to { border-color: #ffe8b9; box-shadow: 0 0 5px #ffae66; } }
@media (prefers-reduced-motion: reduce) { .is-warning .healer-small-icon { animation: none; border-color: #ffaf70; } }
</style>
