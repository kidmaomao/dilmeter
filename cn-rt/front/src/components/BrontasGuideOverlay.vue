<template>
 <div class="brontas-guide" :data-stage="station ? 'station' : 'map'" :data-cue="station ? cue.mode : undefined">
  <header><strong>{{ text('二王 50%') }}</strong><span v-if="preview" class="preview">{{ text('预览') }}</span><span v-if="station">{{ view.clock }}{{ text('点') }}</span></header>
  <div class="brontas-guide-body" :class="{ overview: !station }">
   <svg :viewBox="station ? '-1.53 -1.53 3.06 3.06' : '-1.18 -1.18 2.36 2.36'" role="img" :aria-label="text(station ? '当前点位九宫格射线' : '落石点位地图')">
    <defs><clipPath :id="clipId"><circle r="1.42" /></clipPath></defs>
    <circle :r="station ? 1.42 : 1.05" fill="#101e28" stroke="#648698" stroke-width=".02" />
    <template v-if="!station">
     <path d="M-.62 0H.62 M0-.62V.62" stroke="#314650" stroke-width=".012" />
     <g v-for="clock in [12,3,6,9]" :key="clock" :transform="`translate(${clock === 3 ? .76 : clock === 9 ? -.76 : 0},${clock === 12 ? -.76 : clock === 6 ? .76 : 0})`">
      <circle r=".21" :fill="present(clock) ? '#efb955' : '#192b35'" :stroke="present(clock) ? '#ffe2a6' : '#3c525e'" stroke-width=".015" />
      <text y=".06" text-anchor="middle" :fill="present(clock) ? '#17222a' : '#708792'" font-size=".18" font-weight="700">{{ clock }}</text>
     </g>
     <text x="0" y=".055" text-anchor="middle" fill="#b7cbd5" font-size=".14">{{ text('落石') }}</text>
    </template>
    <g v-else>
     <g v-if="fresh" :clip-path="`url(#${clipId})`" class="ray-bands">
      <rect v-for="ray in mapRays" :key="ray.id"
       :x="ray.axis === 'vertical' ? (ray.offset! < -.5 ? -1.42 : ray.offset!-ray.halfWidth!) : -1.42"
       :y="ray.axis === 'horizontal' ? (ray.offset! > .5 ? -1.42 : -ray.offset!-ray.halfWidth!) : -1.42"
       :width="ray.axis === 'vertical' ? 2*ray.halfWidth! + (Math.abs(ray.offset!) > .5 ? .42 : 0) : 2.84"
       :height="ray.axis === 'horizontal' ? 2*ray.halfWidth! + (Math.abs(ray.offset!) > .5 ? .42 : 0) : 2.84"
       :fill="ray.lethalAtMs <= nowMs ? '#f06277' : ray.lethalAtMs-nowMs<=1100 ? '#ffc14d' : '#4aa5d4'" fill-opacity=".48" />
     </g>
     <path :clip-path="`url(#${clipId})`" d="M-.333333-1.42V1.42 M.333333-1.42V1.42 M-1.42-.333333H1.42 M-1.42 .333333H1.42" fill="none" stroke="#678491" stroke-width=".018" />
     <g v-if="target && player && (cue.mode==='go' || cue.mode==='urgent')" class="movement-guide" :class="{escape:view.guidanceKind==='escape'}">
      <path v-if="nextTarget" class="guide-next" :d="`M${target.x} ${-target.y} L${nextTarget.x} ${-nextTarget.y}`" fill="none" :stroke="guideColor" stroke-width=".035" stroke-dasharray=".08 .07" opacity=".7" />
      <circle v-if="nextTarget" class="guide-next-target" :cx="nextTarget.x" :cy="-nextTarget.y" r=".055" fill="none" :stroke="guideColor" stroke-width=".02" opacity=".7" />
      <circle class="guide-target" :cx="target.x" :cy="-target.y" r=".085" :fill="guideColor" />
      <path class="guide-current" :d="`M${player.x} ${-player.y} L${target.x} ${-target.y}`" :stroke="guideColor" stroke-width=".055" />
      <path :d="arrowHead" fill="none" :stroke="guideColor" stroke-width=".055" stroke-linecap="round" stroke-linejoin="round" />
     </g>
     <circle v-if="player" class="player-dot" :cx="player.x" :cy="-player.y" r=".075" fill="#fff" stroke="#60d8e7" stroke-width=".035" />
     <circle v-if="player && cue.mode==='wait'" class="pace-player-ring" :cx="player.x" :cy="-player.y" r=".17" fill="none" stroke="currentColor" stroke-width=".025" />
     <circle class="cue-map-ring" r="1.46" fill="none" stroke="currentColor" stroke-width=".035" />
    </g>
   </svg>
   <div v-if="station" class="brontas-guide-timing" :class="{ dual:timers.length>1 }">
    <span class="timing-caption">{{ text(activeHit ? '射线判定中' : '距下一次判定') }}</span>
    <div v-for="timer in timers" :key="timer.axis" class="foot-timer" :data-axis="timer.axis" :class="{urgent:timer.lethalAtMs-nowMs<=700}">
     <small v-if="timers.length>1">{{ text(timer.axis === 'horizontal' ? '横线' : '竖线') }}</small><div><strong>{{ timer.seconds }}</strong><span>s</span></div>
    </div>
    <div v-if="!timers.length" class="empty-timer"><strong>--</strong><small>{{ text(!fresh ? '信号中断' : !player ? '定位中' : activeHit ? '等待消退' : '脚下未预警') }}</small></div>
   </div>
  </div>
  <div v-if="station" :key="`${cue.mode}:${cue.detail}`" class="action-cue" :class="`cue-${cue.mode}`" :data-cue="cue.mode">
   <svg class="cue-icon" viewBox="-16 -16 32 32" aria-hidden="true">
    <path v-if="cue.mode==='wait'" d="M-5-9V9 M5-9V9" fill="none" stroke="currentColor" stroke-width="5" />
    <path v-else-if="cue.mode==='go' || cue.mode==='urgent'" d="M-11 0H10 M3-8L11 0 3 8" :transform="`rotate(${cue.angle})`" fill="none" stroke="currentColor" stroke-width="3.5" stroke-linecap="round" stroke-linejoin="round" />
    <path v-else d="M0-10V3 M0 9V10" fill="none" stroke="currentColor" stroke-width="4" stroke-linecap="round" />
   </svg>
   <div class="cue-copy" aria-live="polite" aria-atomic="true"><strong class="cue-title">{{ text(cue.title) }}</strong><span class="guide-direction">{{ text(cue.detail) }}</span></div>
   <div v-if="cue.seconds" class="pace-countdown"><strong>{{ cue.seconds }}</strong><small>s</small></div>
   <span v-else-if="cue.mode==='go' || cue.mode==='urgent'" class="cue-step">{{ text('一格') }}</span>
  </div>
 </div>
</template>

<script setup lang="ts">
import { computed, useId, ref, onMounted, onUnmounted } from 'vue';
import { brontasGuideFresh, brontasGuideMapRays, brontasGuideFootTimers, brontasGuidePlayer, brontasGuideTarget, brontasGuideNextTarget, brontasGuideCue, type BrontasGuideView } from '@/brontasGuide';
const props = defineProps<{ view: BrontasGuideView; rocks: {clock:number}[]; nowMs:number; preview:boolean; translate?: (text:string) => string }>();
// Animate just this card. Do not rerender the entire skill overlay at 60Hz.
const frameMs = ref(Date.now());
const nowMs = computed(() => Math.max(props.nowMs, frameMs.value));
let frame = 0;
const animate = () => { frameMs.value=Date.now(); frame=requestAnimationFrame(animate); };
onMounted(() => { frame=requestAnimationFrame(animate); });
onUnmounted(() => { cancelAnimationFrame(frame); });
const text = (value: string) => props.translate?.(value) ?? value;
const clipId = `brontas-zone-${useId()}`;
const fresh = computed(() => brontasGuideFresh(props.view, nowMs.value));
const station = computed(() => props.view.stage === 'station' || (!props.view.stage && !!props.view.clock && !!props.view.player));
const player = computed(() => brontasGuidePlayer(props.view,nowMs.value));
const mapRays = computed(() => brontasGuideMapRays(props.view,nowMs.value).filter(r=>r.offset!==undefined && r.halfWidth!==undefined && (!r.endsAtMs || r.endsAtMs>nowMs.value)));
const timers = computed(() => brontasGuideFootTimers(props.view,nowMs.value));
const activeHit = computed(()=> {
 const p=player.value;
 return !!p && (props.view.rays || []).some(r=>r.lethalAtMs<=nowMs.value && r.endsAtMs>nowMs.value && r.offset!==undefined && r.halfWidth!==undefined &&
  (r.axis==='horizontal'?p.y:p.x)>=r.offset-r.halfWidth && (r.axis==='horizontal'?p.y:p.x)<r.offset+r.halfWidth);
});
const target = computed(()=>brontasGuideTarget(props.view,nowMs.value));
const nextTarget = computed(()=>brontasGuideNextTarget(props.view,nowMs.value));
const cue = computed(()=>brontasGuideCue(props.view,nowMs.value));
const guideColor=computed(()=>props.view.guidanceKind==='escape' ? '#ff7185' : '#8cf49a');
const arrowHead = computed(()=>{
 const p=player.value,q=target.value;if(!p||!q)return '';
 const d=Math.hypot(q.x-p.x,q.y-p.y);if(d<.01)return '';
 const dx=(q.x-p.x)/d,dy=-(q.y-p.y)/d,x=q.x,y=-q.y;
 return `M${x-dx*.18-dy*.10} ${y-dy*.18+dx*.10} L${x} ${y} L${x-dx*.18+dy*.10} ${y-dy*.18-dx*.10}`;
});
const present = (clock:number) => props.rocks.some(r=>r.clock===clock);
</script>

<style scoped>
.brontas-guide {width:320px;height:224px;box-sizing:border-box;padding:12px 16px;background:rgba(9,18,27,.94);border:1px solid #56788d;border-radius:12px;color:#ecf5fa;font-family:"Microsoft YaHei",sans-serif;transform:scale(var(--boss-mechanic-scale,1));transform-origin:top left}
header {display:flex;align-items:center;gap:8px;height:24px;font-size:15px}header>span:last-child {margin-left:auto;color:#9ddedc;font-size:13px}header .preview {font-size:10px!important;color:#9bb2c0!important;border:1px solid #466071;border-radius:3px;padding:1px 4px}
.brontas-guide-body {display:flex;align-items:center;gap:14px;height:176px}.brontas-guide-body.overview {justify-content:center}svg {width:176px;height:176px;flex-shrink:0}.brontas-guide-timing {display:flex;flex-direction:column;justify-content:center;gap:7px;width:92px}.foot-timer small,.empty-timer small {font-size:11px;color:#abc2cd}.foot-timer>div {display:flex;align-items:baseline;gap:4px;line-height:46px}.foot-timer strong {font-size:42px;font-variant-numeric:tabular-nums;color:#ffbf57}.foot-timer span {font-size:12px;color:#abbec9}.foot-timer.urgent strong {color:#ff7580}.empty-timer {display:flex;flex-direction:column;gap:7px}.empty-timer strong {font-size:28px;color:#91a8b5}
.brontas-guide[data-stage="station"] {height:278px}
.brontas-guide[data-cue="wait"] {--cue-color:#8ecbff;--cue-bg:#19334b}
.brontas-guide[data-cue="go"] {--cue-color:#8cf49a;--cue-bg:#163a26}
.brontas-guide[data-cue="urgent"] {--cue-color:#ff7185;--cue-bg:#451e2a}
.brontas-guide[data-cue="unavailable"] {--cue-color:#b6c2cc;--cue-bg:#26323c}
.cue-map-ring,.pace-player-ring {color:var(--cue-color)}
.cue-map-ring {opacity:.8}
.action-cue {display:flex;align-items:center;gap:10px;box-sizing:border-box;height:48px;margin-top:6px;padding:4px 10px;border:1px solid var(--cue-color);border-radius:7px;background:var(--cue-bg);color:var(--cue-color)}
.action-cue .cue-icon {width:30px;height:30px;flex:0 0 30px}
.cue-copy {display:flex;flex:1;flex-direction:column;gap:1px;min-width:0;line-height:1.15}
.cue-title {font-size:20px;font-weight:800;letter-spacing:1px;white-space:nowrap}
.guide-direction {font-size:11px;color:inherit;white-space:nowrap}
.pace-countdown {display:flex;align-items:baseline;gap:3px;font-variant-numeric:tabular-nums;white-space:nowrap}
.pace-countdown strong {font-size:26px;line-height:1}.pace-countdown small {font-size:12px}.cue-step {font-size:16px;font-weight:700}
.cue-go,.cue-urgent {animation:cue-arrival .4s ease-out}
@keyframes cue-arrival {from{box-shadow:0 0 12px var(--cue-color)}to{box-shadow:0 0 0 transparent}}
@media (prefers-reduced-motion:reduce) {.cue-go,.cue-urgent {animation:none}}
.timing-caption,.region-label {color:#9eafb9;font-size:12px;white-space:nowrap}.region-label {margin-top:4px}.dual {gap:2px}.dual .foot-timer>div {line-height:31px}.dual .foot-timer strong {font-size:30px}.dual .region-label {margin-top:0}.movement-guide {stroke-linecap:round}
.movement-guide.escape {opacity:1}
</style>
