<template>
 <article class="healer-team" :style="style" :aria-label="display('队友监控')">
  <div v-for="row in group.rows" :key="row.key" class="healer-team-row">
   <div class="healer-team-heading" :class="{ 'has-status': row.skills?.length || row.buffs?.length }">
    <strong class="healer-team-name" :title="row.name"><span v-if="row.partySlot" class="healer-team-slot">{{ row.partySlot }}</span>{{ row.name }}</strong>
    <div v-if="row.skills?.length || row.buffs?.length" class="healer-team-status">
     <div
      v-for="card in [...(row.skills ?? []), ...(row.buffs ?? [])]"
      :key="card.key" class="healer-team-status-item"
      :class="{ 'is-warning': card.flash, 'is-muted': card.state === 'missing' || card.state === 'unknown' }"
      :title="`${display(card.title)} · ${display(card.value)}`"
     >
      <span class="healer-team-icon">
       <img :src="card.skillId ? `/skill-icons/${card.skillId}.png` : `/condition-icons/${card.ccId}.png`" :alt="display(card.title)" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'" />
       <span v-if="card.state === 'missing'" aria-hidden="true">!</span>
      </span>
      <span>{{ display(card.value) }}</span>
     </div>
    </div>
   </div>
   <div v-if="row.health || row.mana || row.curtain" class="healer-team-vitals">
    <span v-if="row.curtain" class="healer-vital has-curtain" :title="display('生命帷幕剩余吸收量')">
     <b>{{ row.curtain.replace(/\B(?=(\d{3})+(?!\d))/g, ',') }}</b>
    </span>
    <span v-else-if="row.health" class="healer-vital" :class="{ 'is-low': row.health.flash }" :title="display(row.health.title)" :aria-label="`${display('血量')} ${row.health.value}`">
     <i :style="{ width: percentWidth(row.health.value) }" /><b>{{ row.health.value }}</b>
    </span>
    <span v-if="row.mana" class="healer-vital is-mana" :class="{ 'is-low': row.mana.flash, 'is-stale': row.mana.state === 'stale' }" :title="display(row.mana.state === 'stale' ? '上次同步的魔法值' : row.mana.title)" :aria-label="`${display('魔法值')} ${row.mana.value}`">
     <i :style="{ width: percentWidth(row.mana.value) }" /><b>{{ row.mana.value }}</b>
    </span>
   </div>
  </div>
 </article>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import type { HealerOverlayGroup } from '@/healerMonitorTypes';
const display = (value: string) => value;
const props = defineProps<{ group: HealerOverlayGroup; fontSize: number; iconSize: number; scalePercent?: number; opacityPercent?: number }>();
function percentWidth(value: string) {
 if (!value.endsWith('%')) return '0%';
 return `${Math.max(0, Math.min(100, Number.parseFloat(value) || 0))}%`;
}
const style = computed(() => {
 const textSize = Math.max(10, Math.floor(props.fontSize * 3 / 4));
 return {
  transform: `scale(${Math.max(50, Math.min(200, props.scalePercent ?? 100)) / 100})`,
  transformOrigin: 'top left',
  opacity: Math.max(20, Math.min(100, props.opacityPercent ?? 100)) / 100,
  '--healer-font': `${props.fontSize}px`,
  '--healer-text': `${textSize}px`,
  '--healer-line': `${props.fontSize + 4}px`,
  '--healer-icon': `${props.iconSize}px`,
  '--healer-vitals-height': `${Math.max(14, textSize + 4)}px`,
  '--healer-name-width': `${props.group.nameWidth}px`,
  '--healer-cell-width': `${props.group.cellWidth}px`,
  '--healer-columns': props.group.columns,
  width: `${props.group.width}px`,
  height: `${props.group.height}px`,
 };
});
</script>

<style scoped>
.healer-team {
 box-sizing: border-box;
 color: #fff9f4;
 font-family: "Microsoft YaHei UI", "Microsoft YaHei", sans-serif;
 font-size: var(--healer-font);
 user-select: none;
}
.healer-team {
 padding: 4px 8px;
 border-radius: 5px;
 background: #17171e;
 box-shadow: inset 0 0 0 1px #555360, 0 2px 6px #0008;
}
.healer-team-row { padding: 4px 0; }
.healer-team-row + .healer-team-row { box-shadow: inset 0 1px #ffffff26; }
.healer-team-heading { display: grid; grid-template-columns: var(--healer-name-width); align-items: start; }
.healer-team-heading.has-status { grid-template-columns: var(--healer-name-width) 1fr; gap: 8px; }
.healer-team-name {
 display: block;
 line-height: var(--healer-line);
 white-space: nowrap;
 overflow: hidden;
 text-overflow: ellipsis;
 font-weight: 650;
}
.healer-team-slot { margin-right: .45em; color: #bab5c7; font-variant-numeric: tabular-nums; }
.has-status .healer-team-name { align-content: center; height: max(var(--healer-line), var(--healer-icon)); }
.healer-team-status { display: grid; grid-template-columns: repeat(var(--healer-columns), var(--healer-cell-width)); gap: 4px; }
.healer-team-status-item {
 display: flex;
 align-items: center;
 gap: 4px;
 min-height: max(var(--healer-line), var(--healer-icon));
 font-size: var(--healer-text);
 line-height: var(--healer-line);
 white-space: nowrap;
 font-variant-numeric: tabular-nums;
}
.healer-team-icon {
 position: relative;
 flex: 0 0 auto;
 width: var(--healer-icon);
 height: var(--healer-icon);
}
.healer-team-icon img { width: 100%; height: 100%; display: block; object-fit: contain; }
.healer-team-icon span { position: absolute; inset: 0; display: grid; place-items: center; color: #ffe0a6; font-weight: 800; text-shadow: 0 1px 2px #000; }
.is-muted { color: #bbb7c3; }
.is-muted img { opacity: .43; filter: grayscale(1); }
.is-warning .healer-team-icon { animation: healer-buff-flash .8s ease-in-out infinite alternate; }
@keyframes healer-buff-flash { to { filter: brightness(1.6) drop-shadow(0 0 3px #ffae66); } }
@media (prefers-reduced-motion: reduce) { .is-warning .healer-team-icon { animation: none; filter: brightness(1.6); } }

.healer-team-vitals {
 display: grid;
 gap: 3px;
 margin-top: 4px;
 font-size: var(--healer-text);
 line-height: var(--healer-vitals-height);
 font-variant-numeric: tabular-nums;
}
.healer-vital {
 position: relative;
 display: grid;
 place-items: center;
 overflow: hidden;
 height: var(--healer-vitals-height);
 border-radius: 3px;
 background: #3c242b;
}
.healer-vital.is-mana { background: #26354b; }
.healer-vital.is-mana.is-stale { opacity: .68; }
.healer-vital i { position: absolute; inset: 0 auto 0 0; background: #b33f4b; }
.healer-vital.is-mana i { background: #3f78b2; }
.healer-vital b { position: relative; font-weight: 650; text-shadow: 0 1px 2px #000; }
.healer-vital.is-low { box-shadow: inset 0 0 0 1px #ffa287; }
.healer-vital.has-curtain {
 color: #a0f9b9;
 background: #20362d;
 box-shadow: inset 0 0 0 1px #72e6a8;
}
.has-curtain b { font-weight: 750; }

</style>
