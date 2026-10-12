<template>
  <section class="obs-effects-panel teammate-settings-panel" aria-label="OBS特效(测试)">
    <header class="teammate-panel-header"><strong><v-icon icon="mdi-movie-open-outline" size="20" />OBS特效(测试)</strong></header>
    <div class="teammate-panel-body">
      <p class="obs-effects-notice">需搭配直播软件使用，暂为内部测试使用</p>
      <div class="obs-effects-control">
        <label :title="statusText">
          <input type="checkbox" :checked="enabled" :disabled="pending || !reachable" aria-describedby="obs-effects-status" @change="toggle(($event.target as HTMLInputElement).checked)" />
          <span>启用 OBS 特效</span>
        </label>
        <small id="obs-effects-status" role="status">{{ statusText }}</small>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { ref, onMounted, onBeforeUnmount } from 'vue';
const base = 'http://127.0.0.1:18765/mabinogi';
const controllerId = crypto.randomUUID();
const enabled = ref(false), reachable = ref(false), pending = ref(false);
const statusText = ref('检查星伴连接…');
let actionError = '';
let session = '', mine = false, stopped = false, revision = 0, ownedGeneration = 0, timer: ReturnType<typeof setTimeout>;
let controlStream: EventSource | undefined, controlSession = '';
async function command(input: Record<string, unknown>) {
  const r = await fetch(base + '/api/activation', { method: 'POST', headers: { 'Content-Type': 'application/json', 'X-OBS-Control': '1' }, body: JSON.stringify({ session, controllerId, generation: ownedGeneration, mode: 'live', ...input }), signal: AbortSignal.timeout(2200) });
  const data = await r.json(); if (!r.ok) throw Error(data.error); return data;
}
async function refresh() {
  if (pending.value || stopped) return;
  const ticket = revision;
  try {
    const r = await fetch(base + '/api/status', { signal: AbortSignal.timeout(2200) });
    if (!r.ok) throw Error(r.status === 404 ? 'unsupported' : 'unavailable');
    const data = await r.json();
    if (!data.activation || !data.session) throw Error('unsupported');
    if (ticket !== revision || stopped) return;
    if (mine && (session !== data.session || !data.activation.enabled || data.activation.owner !== controllerId || data.activation.generation !== ownedGeneration)) mine = false;
    session = data.session; reachable.value = true;
    if (controlSession !== session) {
      controlStream?.close(); controlSession = session;
      controlStream = new EventSource(`${base}/api/controller?controllerId=${controllerId}&session=${session}`);
      controlStream.onerror = () => { mine = false; enabled.value = false; statusText.value = '星伴控制连接中断，请重新开启'; };
    }
    if (mine) await command({ enabled: true, renew: true });
    if (ticket !== revision || stopped) return;
    enabled.value = data.activation.enabled;
    const missing = data.mappings.filter((m: { enabled: boolean; available: boolean }) => m.enabled && !m.available).length;
    statusText.value = actionError || (!data.connected ? '未连接 dilmeter' : missing ? `缺少 ${missing} 份素材` : !data.localId ? enabled.value ? '已勾选 · 等待角色信息' : '可开启 · 等待角色信息' : enabled.value ? data.activation.mode === 'preview' ? '仅手动测试' : '已连接 · 已开启' : '已连接 · 已关闭');
  } catch (error) { if (ticket !== revision || stopped) return; mine = false; enabled.value = false; reachable.value = false; statusText.value = error instanceof Error && error.message === 'unsupported' ? '星伴版本未接入大招特效' : '星伴未启动或连接中断'; }
}
async function toggle(value: boolean) {
  pending.value = true;
  revision++; actionError = '';
  try { const data = await command({ enabled: value }); enabled.value = data.activation.enabled; mine = value; ownedGeneration = data.activation.generation; statusText.value = value ? '已连接 · 已开启' : '已关闭'; }
  catch (error) { mine = false; enabled.value = false; actionError = error instanceof Error ? error.message : '开启失败，请检查连接'; statusText.value = actionError; }
  finally { pending.value = false; }
}
async function poll() { await refresh(); if (!stopped) timer = setTimeout(poll, 1500); }
function release() {
  controlStream?.close();
  if (mine && session) void fetch(base + '/api/activation', { method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json', 'X-OBS-Control': '1' }, body: JSON.stringify({ session, controllerId, generation: ownedGeneration, enabled: false, release: true }) }).catch(() => {});
  mine = false;
}
onMounted(() => { void poll(); window.addEventListener('pagehide', release); });
onBeforeUnmount(() => { stopped = true; clearTimeout(timer); window.removeEventListener('pagehide', release); release(); });
</script>

<style scoped>
.obs-effects-notice { margin-bottom: 18px; color: var(--ui-theme-muted); }
.obs-effects-control { display: flex; flex-direction: column; gap: 10px; font-size: 12px; }
.obs-effects-control label { display: flex; align-items: center; gap: 8px; width: fit-content; cursor: pointer; }
.obs-effects-control input { accent-color: var(--ui-color-accent); }
.obs-effects-control small { color: var(--ui-theme-muted); overflow-wrap: anywhere; }
</style>
