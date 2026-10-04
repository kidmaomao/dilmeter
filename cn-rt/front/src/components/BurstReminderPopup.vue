<template>
 <div class="burst-popup" :class="[phase, { urgent, 'timing-unknown': item.timingUnknown, vertical: item.orientation === 'vertical' }]" :style="{ '--burst-progress': progress }" :title="title" role="status">
  <div class="burst-skill-icon">
   <img v-if="!iconFailed && item.skillId" :src="`/skill-icons/${item.skillId}.png`" :alt="t(skillName)" @error="iconFailed = true" />
   <v-icon v-else icon="mdi-flash" aria-hidden="true" />
   <v-icon class="burst-phase-icon" :icon="phase === 'ready' ? 'mdi-check-circle' : phase === 'effect' ? 'mdi-lightning-bolt-circle' : 'mdi-alert-decagram'" aria-hidden="true" />
  </div>
  <div class="burst-identity"><strong>{{ item.actorName || item.actorId || t('未知使用者') }}</strong><small>ID {{ item.actorId || '—' }}</small></div>
  <div class="burst-status"><span>{{ t(phaseLabel) }}</span><b v-if="phase !== 'ready' && !item.timingUnknown">{{ remainingText }}s</b></div>
  <div class="burst-progress" aria-hidden="true"><span /></div>
 </div>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { BossMechanicOverlayItem } from '@/skillCooldown';
const props = withDefaults(defineProps<{ item: BossMechanicOverlayItem; nowMs: number; translate?: (value: string) => string }>(), { translate: (value: string) => value });
const t = (value: string) => props.translate(value);
const iconFailed = ref(false);
watch(() => props.item.skillId, () => { iconFailed.value = false; });
const phase = computed(() => props.item.phase || (props.item.hideCountdown ? 'ready' : 'cast'));
const phaseLabel = computed(() => phase.value === 'ready' ? '冷却结束' : phase.value === 'effect' ? '正在生效' : '正在吟唱');
const remaining = computed(() => Math.max(0, props.item.endsAtMs - props.nowMs) / 1000);
const urgent = computed(() => phase.value === 'cast' && remaining.value <= 1);
const remainingText = computed(() => remaining.value >= 10 ? String(Math.ceil(remaining.value)) : (Math.ceil(remaining.value * 10) / 10).toFixed(1));
const progress = computed(() => String(Math.min(1, Math.max(0, (props.item.endsAtMs - props.nowMs) / Math.max(1, props.item.endsAtMs - props.item.startedAtMs)))));
const skillName = computed(() => props.item.skillName || (props.item.skillId === 58014 ? '万钧之力' : props.item.skillId === 59005 ? '崩坏波动' : props.item.name));
const title = computed(() => `${t(skillName.value)} · ${props.item.actorName || ''} · ID ${props.item.actorId || '—'} · ${t(phaseLabel.value)}${props.item.timingUnknown ? ' · ' + t('已确认状态，未收到持续时间') : ''}`);
</script>
<style scoped>
.burst-popup { --burst-accent: #edb965; position: relative; box-sizing: border-box; display: flex; flex-direction: column; align-items: center; width: 112px; height: 112px; padding: 7px 5px 10px; overflow: hidden; color: #f8f4e9; background: linear-gradient(145deg, #29271ef5, #11181bf5); border: 1px solid #9e8050; border-radius: 5px; box-shadow: 0 2px 8px #0007; font-family: "Microsoft YaHei UI", "Microsoft YaHei", sans-serif; transform: scale(var(--boss-mechanic-scale, 1)); transform-origin: left top; }
.burst-skill-icon { position: relative; flex-shrink: 0; display: grid; place-items: center; width: 40px; height: 40px; padding: 2px; box-sizing: border-box; border: 1px solid var(--burst-accent); border-radius: 3px; background: #101519; }
.burst-skill-icon img { width: 100%; height: 100%; object-fit: contain; }
.burst-skill-icon > .v-icon { color: var(--burst-accent); font-size: 26px; }
.burst-skill-icon > .burst-phase-icon { position: absolute; right: -7px; top: -4px; font-size: 15px; color: var(--burst-accent); background: #17201d; border-radius: 50%; }
.burst-identity { width: 100%; margin-top: 5px; text-align: center; }
.burst-identity strong { display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-size: 12px; line-height: 15px; font-weight: 600; }
.burst-identity small { display: block; font-size: 8px; line-height: 11px; color: #b9bbb2; white-space: nowrap; font-variant-numeric: tabular-nums; }
.burst-status { display: flex; align-items: baseline; justify-content: center; gap: 4px; width: 100%; margin-top: 4px; color: var(--burst-accent); font-size: 11px; line-height: 15px; white-space: nowrap; }
.burst-status b { font-weight: 650; font-variant-numeric: tabular-nums; }
.burst-progress { position: absolute; right: 7px; bottom: 5px; left: 7px; height: 3px; overflow: hidden; border-radius: 2px; background: #ffffff19; }
.burst-progress > span { display: block; width: 100%; height: 100%; background: var(--burst-accent); transform: scaleX(var(--burst-progress)); transform-origin: left center; }
.burst-popup.effect { --burst-accent: #89d1dc; border-color: #567f89; }
.burst-popup.ready { --burst-accent: #a6d797; border-color: #69875f; }
.burst-popup.urgent { --burst-accent: #ff8877; border-color: #b76e5f; }
.urgent .burst-phase-icon { animation: burst-pulse .7s ease-in-out infinite alternate; }
.vertical .burst-progress { top: 7px; bottom: 7px; left: auto; right: 3px; width: 3px; height: auto; }
.vertical .burst-progress > span { transform: scaleY(var(--burst-progress)); transform-origin: center bottom; }
.timing-unknown .burst-progress > span { transform: none; opacity: .45; }
@keyframes burst-pulse { from { opacity: .65; } to { opacity: 1; filter: drop-shadow(0 0 3px #ff887788); } }
@media (prefers-reduced-motion: reduce) { .urgent .burst-phase-icon { animation: none; } }
</style>
