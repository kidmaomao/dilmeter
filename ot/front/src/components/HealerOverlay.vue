<template>
 <main class="healer-overlay-root" :aria-label="$ui('圣歌监测悬浮提醒')" :style="{ zoom: displayZoom }">
  <HealerOverlayVisual v-for="group in frame?.groups || []" :key="group.key" :group="group" :font-size="frame!.fontSize" :icon-size="frame!.iconSize" :scale-percent="frame!.scalePercent" :opacity-percent="frame!.opacityPercent" :style="{ position: 'absolute', left: `${group.x - frame!.x}px`, top: `${group.y - frame!.y}px` }" />
 </main>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import type { HealerOverlayFrame } from '@/healerMonitorTypes';
import HealerOverlayVisual from './HealerOverlayVisual.vue';
const frame = ref<HealerOverlayFrame | null>(null);
const deviceScale = ref(window.devicePixelRatio || 1);
// WebView2 already scales CSS pixels. Apply only the difference from the host's target DPI.
const displayZoom = computed(() => frame.value?.dpiPercent ? frame.value.dpiPercent / (100 * deviceScale.value) : 1);
let busy = false, disposed = false, lastRefresh = 0;
let timer: ReturnType<typeof setInterval> | undefined;
async function refresh() {
 if (disposed || busy || Date.now() - lastRefresh < 100) return;
 busy = true; lastRefresh = Date.now();
 try {
  const response = await fetch('/api/healer_overlay', { cache: 'no-store', signal: AbortSignal.timeout(2000) });
  if (!response.ok) throw new Error('unavailable');
  const value = await response.json() as HealerOverlayFrame;
  if (!disposed) {
   deviceScale.value = window.devicePixelRatio || 1;
   frame.value = value;
  }
 } catch { if (!disposed) frame.value = null; }
 finally { busy = false; }
}
onMounted(() => {
 document.documentElement.classList.add('healer-overlay-document');
 void refresh(); timer = setInterval(refresh, 125);
 window.addEventListener('dilmeter-native-tick', refresh);
});
onBeforeUnmount(() => { disposed = true; clearInterval(timer); window.removeEventListener('dilmeter-native-tick', refresh); document.documentElement.classList.remove('healer-overlay-document'); });
</script>
<style>
html.healer-overlay-document, html.healer-overlay-document body, html.healer-overlay-document #app, .healer-overlay-app, .healer-overlay-app .v-application__wrap { margin: 0 !important; padding: 0 !important; width: 100%; height: 100%; min-height: 0 !important; overflow: hidden !important; background: transparent !important; }
.healer-overlay-root { position: relative; width: 100%; height: 100%; background: transparent; pointer-events: none; }
</style>
