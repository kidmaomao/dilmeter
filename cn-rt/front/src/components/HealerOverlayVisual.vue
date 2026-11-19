<template>
 <article class="healer-team" :style="style" :aria-label="display('队友监控')">
  <div v-for="row in rows" :key="row.key" class="healer-team-row">
   <div class="healer-team-heading" :class="{ 'has-curtain': row.curtain, 'is-low': !row.curtain && row.health?.flash }">
    <i v-if="!row.curtain && row.health?.value.endsWith('%')" class="healer-hp-fill" aria-hidden="true" :style="{ width: percentWidth(row.health.value) }" />
    <strong class="healer-team-name" :title="row.name"><span v-if="row.partySlot" class="healer-team-slot">{{ row.partySlot }}</span>{{ row.name }}</strong>
    <b v-if="row.curtain" class="healer-resource-value" :title="display('生命帷幕剩余吸收量')">{{ row.curtain.replace(/\B(?=(\d{3})+(?!\d))/g, ',') }}</b>
    <b v-else-if="row.health" class="healer-resource-value" :title="display(row.health.title)">HP {{ row.health.value }}</b>
   </div>
   <div v-if="row.mana" class="healer-mana" :class="{ 'is-low': row.mana.flash, 'is-stale': row.mana.state === 'stale' }"
    :title="display(row.mana.state === 'stale' ? '上次同步的魔法值' : row.mana.title)">
    <i v-if="row.mana.value.endsWith('%')" aria-hidden="true" :style="{ width: percentWidth(row.mana.value) }" /><b>MP {{ row.mana.value }}</b>
   </div>
   <div v-if="row.icons.length" class="healer-team-status">
    <span v-for="card in row.icons" :key="card.key" class="healer-team-status-item"
     :class="[card.category === 'skill' ? 'healer-status-skill' : 'healer-status-buff', { 'is-warning': card.flash, 'is-muted': card.state === 'missing' || card.state === 'unknown' }]"
     role="img" :aria-label="`${display(card.title)} · ${display(card.value)}`" :title="`${display(card.title)} · ${display(card.value)}`">
     <img :src="card.category === 'skill' ? `/skill-icons/${card.skillId}.png` : `/condition-icons/${card.ccId}.png`" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'" />
     <b v-if="card.badge" class="healer-status-value" :style="{ '--badge-units': card.badgeUnits }">{{ card.badge }}</b>
    </span>
   </div>
  </div>
 </article>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { healerStatusBadge, type HealerCard, type HealerOverlayGroup } from '@/healerMonitorTypes';
const display = (value: string) => value;

const props = defineProps<{ group: HealerOverlayGroup; fontSize: number; iconSize: number; scalePercent?: number; opacityPercent?: number }>();
function percentWidth(value: string) {
 return `${Math.max(0, Math.min(100, Number.parseFloat(value) || 0))}%`;
}
function statusIcon(card: HealerCard) {
 const badge = display(healerStatusBadge(card));
 const badgeUnits = Math.max(1, badge.length * (/^[\x20-\x7e]+$/.test(badge) ? 0.62 : 1));
 return { ...card, badge, badgeUnits };
}
const rows = computed(() => (props.group.rows ?? []).map(row => ({
 ...row, icons: [...(row.skills ?? []), ...(row.buffs ?? [])].map(statusIcon),
})));
const style = computed(() => {
 const textSize = Math.max(10, Math.floor(props.fontSize * 3 / 4));
 return {
  transform: `scale(${Math.max(50, Math.min(200, props.scalePercent ?? 100)) / 100})`,
  transformOrigin: 'top left',
  opacity: Math.max(20, Math.min(100, props.opacityPercent ?? 100)) / 100,
  '--healer-font': `${props.fontSize}px`,
  '--healer-text': `${textSize}px`,
  '--healer-line': `${props.fontSize + 8}px`,
  '--healer-icon': `${props.group.cellWidth ?? props.iconSize}px`,
  '--healer-cell-height': `${props.group.cellHeight ?? props.iconSize}px`,
  '--healer-mana-height': `${textSize + 6}px`,
  '--healer-name-width': `${props.group.nameWidth ?? props.group.width}px`,
  '--healer-columns': props.group.columns,
  width: `${props.group.width}px`,
  height: `${props.group.height}px`,
 };
});
</script>

<style scoped>
.healer-team {
 box-sizing: border-box;
 padding: 3px 6px;
 border-radius: 5px;
 background: rgba(18, 20, 25, .95);
 box-shadow: inset 0 0 0 1px #555360, 0 2px 6px #0008;
 color: #fff9f4;
 font-family: "Microsoft YaHei UI", "Microsoft YaHei", sans-serif;
 font-size: var(--healer-font);
 font-variant-numeric: tabular-nums;
 user-select: none;
}
.healer-team-row { padding: 4px 0; }
.healer-team-row + .healer-team-row { box-shadow: inset 0 1px #ffffff29; }
.healer-team-heading, .healer-mana {
 position: relative;
 display: flex;
 align-items: center;
 box-sizing: border-box;
 overflow: hidden;
 padding: 0 4px;
 border-radius: 2px;
}
.healer-team-heading { height: var(--healer-line); gap: 4px; background: #ffffff06; }
.healer-hp-fill, .healer-mana i { position: absolute; inset: 0 auto 0 0; box-sizing: border-box; }
.healer-hp-fill { background: #b047545c; border-bottom: 3px solid #e64e65; }
.healer-team-heading.has-curtain { color: #a7f2bf; background: #20362d; box-shadow: inset 0 -3px #72c99a; }
.healer-team-name {
 position: relative;
 flex: 1 1 var(--healer-name-width);
 min-width: 0;
 overflow: hidden;
 white-space: nowrap;
 text-overflow: ellipsis;
 font-weight: 650;
 line-height: var(--healer-line);
}
.healer-team-slot { margin-right: .4em; color: #d4d1d8; }
.healer-resource-value, .healer-mana b {
 position: relative;
 white-space: nowrap;
 font-size: var(--healer-text);
 font-weight: 650;
 line-height: 1;
}
.healer-mana {
 justify-content: flex-end;
 margin-top: 2px;
 height: var(--healer-mana-height);
 background: #26354b;
}
.healer-mana i { background: #3079b8; }
.healer-mana.is-stale { opacity: .68; }
.healer-team-heading.is-low, .healer-mana.is-low { box-shadow: inset 0 0 0 1px #ffa287; }
.healer-team-status {
 display: grid;
 grid-template-columns: repeat(var(--healer-columns), var(--healer-icon));
 grid-auto-rows: var(--healer-cell-height);
 gap: 2px;
 margin-top: 4px;
 padding-left: var(--healer-font);
}
.healer-status-skill { --healer-icon-border: #91c8f1; }
.healer-status-buff { --healer-icon-border: #e9c36a; }
.healer-team-status-item { position: relative; overflow: hidden; border-radius: 2px; background: #191c22; }
.healer-team-status-item::after { content: ""; position: absolute; inset: 0; border: 1px solid var(--healer-icon-border); border-radius: inherit; pointer-events: none; }
.healer-team-status-item img { width: var(--healer-icon); height: var(--healer-icon); display: block; object-fit: contain; }
.healer-team-status-item.is-muted img { opacity: .58; filter: grayscale(1); }
.healer-status-value {
 position: absolute;
 inset: auto 1px 1px;
 height: min(calc(var(--healer-text) + 2px), calc(var(--healer-icon) * .5));
 display: grid;
 place-items: center;
 background: #080a0ef2;
 color: #fff;
 font-family: Consolas, "Microsoft YaHei UI", monospace;
 font-size: min(var(--healer-text), calc(var(--healer-icon) * .44), calc((var(--healer-icon) - 4px) / var(--badge-units)));
 font-weight: 700;
 line-height: 1.05;
 white-space: nowrap;
}
.is-warning { animation: healer-buff-flash .8s ease-in-out infinite alternate; }
@keyframes healer-buff-flash { to { filter: brightness(1.6) drop-shadow(0 0 3px #ffae66); } }
@media (prefers-reduced-motion: reduce) { .is-warning { animation: none; filter: brightness(1.6); } }
</style>
