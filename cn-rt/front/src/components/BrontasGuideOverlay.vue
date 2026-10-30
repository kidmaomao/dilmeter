<template>
 <div class="brontas-guide" :data-status="fresh ? view.status : 'waiting'">
  <header><strong>{{ text("二王 50% · 走位提示") }}</strong><span>{{ text(preview ? '预览' : fresh && view.status !== 'waiting' ? '实时' : '未连接') }}</span></header>
  <div class="brontas-guide-stations">
   <span v-for="clock in [12, 3, 6, 9]" :key="clock" :class="{ present: rocks.some(r => r.clock === clock), selected: fresh && view.clock === clock }">{{ clock }}{{ text("点") }} {{ rocks.some(r => r.clock === clock) ? '◆' : '—' }}</span>
  </div>
  <div class="brontas-guide-body">
   <svg viewBox="-1.15 -1.15 2.3 2.3" role="img" aria-label="落石区内走位图，地图上方为上">
    <defs><clipPath :id="clipId"><circle r="1" /></clipPath></defs>
    <circle r="1" fill="#111f2a" stroke="#65849b" stroke-width=".025" />
    <path d="M-1 0H1 M0-1V1" stroke="#293d4c" stroke-width=".012" />
    <g v-if="fresh && view.player" :clip-path="`url(#${clipId})`">
     <template v-for="ray in view.rays || []" :key="ray.id">
      <rect v-if="ray.offset !== undefined && ray.halfWidth !== undefined && (!ray.endsAtMs || ray.endsAtMs > nowMs)"
       :x="ray.axis === 'vertical' ? ray.offset-ray.halfWidth : -1"
       :y="ray.axis === 'horizontal' ? -ray.offset-ray.halfWidth : -1"
       :width="ray.axis === 'vertical' ? 2*ray.halfWidth : 2"
       :height="ray.axis === 'horizontal' ? 2*ray.halfWidth : 2"
       :fill="ray.lethalAtMs > nowMs ? '#ffbf47' : '#ff4b5d'" fill-opacity=".45" />
     </template>
     <template v-if="view.target && view.status === 'move'">
      <path :d="`M${view.player.x} ${-view.player.y} L${view.target.x} ${-view.target.y}`" stroke="#68eadb" stroke-width=".04" />
      <path :d="arrowPath" fill="none" stroke="#68eadb" stroke-width=".04" stroke-linecap="round" stroke-linejoin="round" />
      <circle :cx="view.target.x" :cy="-view.target.y" r=".08" fill="#68eadb" />
     </template>
     <circle :cx="view.player.x" :cy="-view.player.y" :r="view.player.radius" fill="#f5faff" stroke="#73b7ff" stroke-width=".035" />
    </g>
    <text v-if="!fresh || !view.player" x="0" y=".07" text-anchor="middle" fill="#a5b9c9" font-size=".22">{{ text("待定位") }}</text>
   </svg>
   <div class="brontas-guide-timing"><small>{{ text("最早射线致命") }}</small><strong>{{ text(deadline) }}</strong><span>{{ text(fresh && view.clock ? `${view.clock}点区域` : '等待点位定位') }}</span></div>
  </div>
  <strong class="brontas-guide-action">{{ text(fresh ? view.instruction : '数据过期 · 停止走位引导') }}</strong>
  <p>{{ text(fresh ? view.detail : '等待新的识别结果') }}</p>
  <footer>{{ text("地图方向") }} · {{ text(opposite) }}<br />{{ text("箭头仅针对当前已识别射线") }}</footer>
 </div>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue';
import { brontasGuideFresh, brontasGuideDeadline, type BrontasGuideView } from '@/brontasGuide';
const props = defineProps<{ view: BrontasGuideView; rocks: {clock:number}[]; nowMs:number; preview:boolean; translate?: (text:string) => string }>();
const text = (value: string) => props.translate?.(value) ?? value;
const clipId = `brontas-zone-${useId()}`;
const fresh = computed(() => brontasGuideFresh(props.view, props.nowMs));
const deadline = computed(() => brontasGuideDeadline(props.view, props.nowMs));
const opposite = computed(() => {
 const c=props.view.clock;
 return fresh.value && c ? `对侧 ${((c+5)%12)+1}点；注意共享射线` : '12↔6 / 3↔9 注意对侧';
});
const arrowPath = computed(() => {
 const p=props.view.player, q=props.view.target;
 if (!p || !q) return '';
 const d=Math.hypot(q.x-p.x,q.y-p.y);
 if (d<.01) return '';
 const x=q.x, y=-q.y, dx=(q.x-p.x)/d, dy=-(q.y-p.y)/d;
 return `M${x-dx*.17-dy*.09} ${y-dy*.17+dx*.09} L${x} ${y} L${x-dx*.17+dy*.09} ${y-dy*.17-dx*.09}`;
});
</script>

<style scoped>
.brontas-guide { width:320px;height:340px;box-sizing:border-box;padding:14px 16px;background:rgba(9,18,27,.94);border:1px solid #56788d;border-radius:12px;color:#ecf5fa;font-family:"Microsoft YaHei",sans-serif;transform:scale(var(--boss-mechanic-scale,1));transform-origin:top left; }
header {display:flex;align-items:center;justify-content:space-between;font-size:16px;line-height:24px} header span {font-size:11px;color:#9eb7c7;border:1px solid #466071;padding:0 6px;border-radius:4px}
.brontas-guide-stations {display:flex;justify-content:space-between;margin:10px 0 3px;font-size:12px;color:#617584}.brontas-guide-stations .present {color:#e5b95f}.brontas-guide-stations .selected {color:#72eee0;text-decoration:underline;text-underline-offset:5px}
.brontas-guide-body {display:flex;align-items:center;gap:10px;height:154px}svg {width:154px;height:154px;flex-shrink:0}.brontas-guide-timing {display:flex;flex-direction:column;gap:9px}.brontas-guide-timing small {color:#a5b9c9;font-size:11px}.brontas-guide-timing strong {font-size:29px;font-variant-numeric:tabular-nums;color:#ffbf47}.brontas-guide-timing span {font-size:11px;color:#b8cedb}
.brontas-guide-action {display:block;font-size:17px;line-height:26px;white-space:nowrap;letter-spacing:-.5px}.brontas-guide p {font-size:11px;line-height:18px;color:#b8cedb;margin:3px 0 9px}footer {font-size:10px;line-height:17px;border-top:1px solid #344651;padding-top:7px;color:#8fa9ba}
[data-status="move"] .brontas-guide-action {color:#72eee0}[data-status="blocked"] .brontas-guide-action {color:#ff8290}[data-status="waiting"] .brontas-guide-action {color:#e5b95f}
</style>
