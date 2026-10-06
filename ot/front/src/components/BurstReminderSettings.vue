<template>
 <section class="burst-settings teammate-settings-panel" :aria-label="t('爆发提醒设置')">
  <header class="teammate-panel-header"><strong><v-icon icon="mdi-lightning-bolt-outline" size="20" /> {{ t('爆发提醒') }}</strong></header>
  <section class="teammate-basics" :aria-label="t('基础设定')">
   <div class="teammate-section-heading"><h2>{{ t('基础设定') }}</h2><span role="status">{{ t(saving ? '正在保存…' : dirty ? '有未保存的修改' : '设置已同步') }}</span><button type="button" class="teammate-save" :class="{ 'healer-save-pending': dirty && !saving }" :disabled="saving" @click="emit('save')">{{ t(saving ? '保存中…' : dirty ? '保存设定 · 未保存' : '保存设定') }}</button></div>
   <div class="teammate-basics-row"><label><input v-model="settings.enabled" type="checkbox" @change="emit('change')" />{{ t('启用提醒') }}</label><label>{{ t('音量') }}<input v-model.number="settings.volume" type="number" min="0" max="100" @input="emit('change')" />%</label><label><input v-model="settings.includeSelf" type="checkbox" @change="emit('change')" />{{ t('监测本人') }}</label><label><input v-model="settings.includeTeammates" type="checkbox" @change="emit('change')" />{{ t('监测圣歌监控中选中的队友') }}</label></div>
  </section>
  <div class="teammate-panel-body">
  <p>{{ t('队友冷却按观测到的释放和填写的秒数估算；请填入队友实际冷却。尚未观测到释放时不会报就绪。吟唱倒计时可校准，取消、状态生效或离开视野后结束。') }}</p>
  <article v-for="id in [59005, 58014]" :key="id">
   <strong>{{ skillResourceName(id, settings.rules[id].name) }}</strong><small>{{ t(id === 59005 ? 'Boss 崩坏状态 · CC803' : '觉醒 · CC516 · 最长 10 秒') }}</small>
   <div class="burst-line"><label v-if="id === 59005">{{ t('冷却秒数') }}<input v-model.number="settings.rules[id].cooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @input="emit('change')" /></label><label>{{ t('吟唱秒数') }}<input v-model.number="settings.rules[id].castSeconds" type="number" min="0.1" max="60" step="0.1" @input="emit('change')" /></label><label>{{ t('进度条方向') }}<select v-model="settings.rules[id].orientation" @change="emit('change')"><option value="horizontal">{{ t('横向') }}</option><option value="vertical">{{ t('纵向') }}</option></select></label></div>
   <div class="burst-line"><label>X<input v-model.number="settings.rules[id].cast.x" type="number" min="-32000" max="32000" @input="emit('change')" /></label><label>Y<input v-model.number="settings.rules[id].cast.y" type="number" min="-32000" max="32000" @input="emit('change')" /></label><label>{{ t('大小') }}<input v-model.number="settings.rules[id].cast.scalePercent" type="number" min="50" max="200" step="5" @input="emit('change')" />%</label></div>
   <div v-for="phase in phases.filter(phase => id === 59005 || phase.key !== 'ready')" :key="phase.key" class="burst-line burst-phase">
    <label><input v-model="settings.rules[id][phase.key].enabled" type="checkbox" @change="emit('change')" />{{ t(phase.name) }}</label>

    <label v-if="phase.key !== 'effect'"><input v-model="settings.rules[id][phase.key].soundEnabled" type="checkbox" @change="emit('change')" />{{ t('音效') }}</label><button @click="emit('preview', settings.rules[id], phase.key)">{{ t('预览 8 秒') }}</button>
   <div v-if="phase.key === 'ready' && settings.rules[id].ready.enabled" class="burst-line cooldown-options">
    <label><input v-model="settings.rules[id].cooldownAlwaysVisible" type="checkbox" @change="emit('change')" />{{ t('一直显示') }}</label>
    <label><input v-model="settings.rules[id].cooldownAlertEnabled" type="checkbox" @change="emit('change')" />{{ t('提前') }}<input v-model.number="settings.rules[id].cooldownLeadSeconds" type="number" min="0" max="86400" step="0.1" :disabled="!settings.rules[id].cooldownAlertEnabled" :aria-label="t('崩坏冷却提前提醒秒数')" @input="emit('change')" />{{ t('秒放大提醒') }}</label>
    <button type="button" @click="emit('preview', settings.rules[id], 'cooldown')">{{ t('小窗预览 8 秒') }}</button><button type="button" @click="emit('preview', settings.rules[id], 'alert')">{{ t('放大提醒预览 8 秒') }}</button>
    <small>{{ t('两项可同时开启：图标下保留已就绪队友，同时显示下一位的冷却秒数；进入提前秒数后放大提醒。再次释放后从已就绪名单移除，音效在就绪时播放一次。') }}</small>
    <small>{{ t('个别队友在技能提醒中填写了崩坏冷却时，优先使用该秒数；未填写时使用上方冷却秒数。') }}</small>
   </div>
   </div>
  </article>
  <p>{{ t('同一张卡片依次显示吟唱与生效状态。万钧觉醒从生效起计时 10 秒，举着或放下技能不会延长或缩短计时；收到明确的觉醒移除时结束。崩坏读取实际状态期限。多人提示自动错开。') }}</p>
  </div>
 </section>
</template>
<script setup lang="ts">
import { gameUiText, skillResourceName } from '@/gameTerms';
import type { BurstRule, BurstSettings } from '@/burstReminder';
const props = withDefaults(defineProps<{ settings: BurstSettings; dirty: boolean; saving: boolean; translate?: (value: string) => string }>(), { translate: (value: string) => value });
const t = gameUiText;
const phases: { key: 'ready' | 'cast' | 'effect'; name: string }[] = [{ key: 'ready', name: '冷却提醒' }, { key: 'cast', name: '吟唱倒计时' }, { key: 'effect', name: '生效状态与倒计时' }];
const emit = defineEmits<{ change: []; save: []; preview: [rule: BurstRule, phase: 'ready' | 'cast' | 'effect' | 'cooldown' | 'alert'] }>();
</script>
<style scoped>
.burst-settings { margin: 8px 0; padding: 8px; border: 1px solid var(--ui-theme-border); color: var(--ui-theme-text); background: var(--ui-theme-raised); font-size: 10px; }
header, .burst-line { display: flex; align-items: center; flex-wrap: wrap; gap: 8px 12px; }
header { padding-bottom: 7px; border-bottom: 1px solid var(--ui-theme-border); }
header > button { margin-left: auto; } header > span { color: #edc47d; font-size: 9px; } header strong { color: var(--ui-color-accent); font-size: 12px; }
article { padding: 8px; margin-top: 8px; border: 1px solid var(--ui-theme-border); background: var(--ui-theme-surface); font-size: 10px; }
article > small { margin-left: 10px; } .burst-line { margin-top: 8px; } .burst-phase > label:first-child { min-width: 100px; }
label { display: inline-flex; align-items: center; gap: 5px; font-size: 10px; }
input[type=number] { width: 62px; } input[type=checkbox] { accent-color: var(--ui-color-accent); }
input, select, button { height: 25px; padding: 0 6px; border: 1px solid var(--ui-theme-border); color: var(--ui-theme-text); background: var(--ui-theme-control); font: inherit; font-size: 10px; }
button { cursor: pointer; } button:disabled { opacity: .5; } p, small { font-size: 9px; color: var(--ui-theme-muted); line-height: 1.6; }
.cooldown-options { flex-basis: 100%; padding: 4px 0 4px 12px; border-left: 2px solid var(--ui-theme-border); }
.cooldown-options small { flex-basis: 100%; }
</style>
