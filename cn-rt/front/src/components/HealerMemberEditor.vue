<template>
 <article class="healer-member" :class="{ 'healer-member-compact': !templateMode }" :aria-label="`${member.name}的监控设置`">
  <header class="healer-member-heading">
   <span class="healer-presence" :class="{ active: live?.active }" />
   <div class="healer-member-name" :title="`${member.name} · ${status}`"><strong>{{ member.name }}</strong><small>{{ status }}</small></div>
   <label v-if="!templateMode" class="healer-favorite" title="勾选或取消后立即保存当前设定"><input v-model="member.favorite" type="checkbox" :disabled="disabled" :aria-label="`${member.name}保存为常用`" @change="emit('favoriteChange')" />保存为常用</label>
   <button v-if="!templateMode" type="button" class="healer-remove-member" :disabled="disabled" :aria-label="`移除队友 ${member.name}`" @click="emit('remove')">移除</button>
   <button v-if="!templateMode" type="button" class="healer-section-toggle healer-template-toggle" :class="{ selected: section === 'template' }" :disabled="disabled" :aria-expanded="section === 'template'" :aria-controls="`healer-member-details-${index}`" :aria-label="`${member.name}模板设定`" @click="toggleSection('template')">模板<v-icon :icon="section === 'template' ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="16" /></button>
   <div class="healer-quick-toggles">
    <div class="healer-quick-setting"><input v-if="!templateMode" v-model="member.health" type="checkbox" :disabled="disabled" :aria-label="`监测 ${member.name} 血量`" @change="changed" /><button type="button" class="healer-section-toggle" :class="{ selected: section === 'health' }" :disabled="disabled" :aria-expanded="section === 'health'" :aria-controls="`healer-member-details-${index}`" :aria-label="`${member.name}血量设定`" @click="toggleSection('health')">血量提醒<v-icon :icon="section === 'health' ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="16" /></button></div>
    <div class="healer-quick-setting"><input v-if="!templateMode" v-model="member.buffSettings.enabled" type="checkbox" :disabled="disabled" :aria-label="`监测 ${member.name} Buff`" @change="changed" /><button type="button" class="healer-section-toggle" :class="{ selected: section === 'buff' }" :disabled="disabled" :aria-expanded="section === 'buff'" :aria-controls="`healer-member-details-${index}`" :aria-label="`${member.name}Buff设定`" @click="toggleSection('buff')">Buff 提醒<v-icon :icon="section === 'buff' ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="16" /></button></div>
    <div class="healer-quick-setting"><input v-if="!templateMode" v-model="member.skillSettings.enabled" type="checkbox" :disabled="disabled" :aria-label="`监测 ${member.name} 技能`" @change="changed" /><button type="button" class="healer-section-toggle" :class="{ selected: section === 'skill' }" :disabled="disabled" :aria-expanded="section === 'skill'" :aria-controls="`healer-member-details-${index}`" :aria-label="`${member.name}技能设定`" @click="toggleSection('skill')">技能提醒<v-icon :icon="section === 'skill' ? 'mdi-chevron-up' : 'mdi-chevron-down'" size="16" /></button></div>
   </div>

  </header>
  <div v-if="section" :id="`healer-member-details-${index}`" class="healer-member-details">

  <div v-if="section === 'template' && !templateMode" class="healer-template-actions">
   <label>套用模板 <select v-model="selectedTemplate" :disabled="disabled || !templates?.length" :aria-label="`${member.name}的通用模板`"><option value="">选择模板</option><option v-for="template in templates || []" :key="template.id" :value="template.id">{{ template.name }}</option></select></label>
   <button type="button" :disabled="disabled || !templates?.some(template => template.id === selectedTemplate)" :aria-label="`为 ${member.name} 套用模板`" @click="emit('applyTemplate', selectedTemplate)">套用</button>
   <button type="button" :disabled="disabled || (templates?.length || 0) >= 32" :aria-label="`将 ${member.name} 存为模板`" @click="emit('saveTemplate')">存为模板</button>
  </div>
  <section v-if="section === 'buff' || section === 'skill'" class="healer-shared-overlay healer-detail healer-overlay-controls" :aria-label="`${member.name}Buff与技能坐标与预览`">
   <h3>Buff / 技能共用悬浮窗 · 坐标与预览</h3>
   <div class="healer-config-line"><label><input v-model="member.buffSettings.overlay.enabled" type="checkbox" @change="changed" />显示 Buff</label><label><input v-model="member.skillSettings.overlay.enabled" type="checkbox" @change="changed" />显示技能</label></div>
   <div class="healer-config-line"><label>横坐标 X <input v-model.number="member.buffSettings.overlay.x" type="number" min="-32000" max="32000" :aria-label="`${member.name}共用悬浮窗横坐标`" @input="changed" /></label><label>纵坐标 Y <input v-model.number="member.buffSettings.overlay.y" type="number" min="-32000" max="32000" :aria-label="`${member.name}共用悬浮窗纵坐标`" @input="changed" /></label><button type="button" :disabled="disabled || !buffPreview.cards.length" :aria-label="`${member.name}Buff与技能屏幕预览`" @click="emit('preview', 'buff')">屏幕预览 8 秒</button></div>
   <small>Buff 与技能共用此坐标；血量窗口在血量设定中调整。</small>
   <div v-if="buffPreview.cards.length" class="healer-visual-preview"><HealerOverlayVisual :group="buffPreview" :font-size="fontSize" :icon-size="iconSize" :opacity-percent="opacityPercent" /></div>
   <p v-else>请先在 Buff 或技能设定中添加需要显示的项目。</p>
  </section>
  <section v-if="section === 'health'" class="healer-member-section">
   <div class="healer-section-heading">
    <label v-if="templateMode"><input v-model="member.health" type="checkbox" :aria-label="`监测 ${member.name} 血量`" @change="changed" /><strong>血量提示</strong></label>
    <strong v-if="!templateMode">血量提示</strong>
    <span class="healer-inline-status" :class="{ warning: live?.healthState === 'low' }">{{ healthLabel }}</span>
   </div>
   <div class="healer-detail">
    <section class="healer-overlay-controls healer-health-overlay" :aria-label="`${member.name}血量坐标与预览`">
     <h3>血量悬浮窗 · 坐标与预览</h3>
    <div class="healer-config-line"><label><input v-model="member.healthSettings.overlay.enabled" type="checkbox" :aria-label="`${member.name}血量悬浮窗提示`" @change="changed" />悬浮窗提示</label><label>横坐标 X <input v-model.number="member.healthSettings.overlay.x" type="number" min="-32000" max="32000" :aria-label="`${member.name}血量横坐标`" @input="changed" /></label><label>纵坐标 Y <input v-model.number="member.healthSettings.overlay.y" type="number" min="-32000" max="32000" :aria-label="`${member.name}血量纵坐标`" @input="changed" /></label><button type="button" :disabled="disabled" :aria-label="`${member.name}血量屏幕预览`" @click="emit('preview', 'health')">屏幕预览 8 秒</button></div>
    <div class="healer-visual-preview"><HealerOverlayVisual :group="healthPreview" :font-size="fontSize" :icon-size="iconSize" :opacity-percent="opacityPercent" /></div>
    </section>
    <div class="healer-config-line"><label>血量 ≤ <input v-model.number="member.healthSettings.threshold" type="number" min="5" max="95" :aria-label="`${member.name}血量阈值`" @input="changed" /> % 时提示</label></div>
    <div class="healer-config-line"><label><input v-model="member.healthSettings.soundEnabled" type="checkbox" :aria-label="`${member.name}血量声音提示`" @change="changed" />声音提示</label><HealerSoundPicker v-model="member.healthSettings.sound" :label="`${member.name}血量`" :volume="volume" :disabled="disabled || !member.healthSettings.soundEnabled" @update:model-value="changed" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)" /></div>
    <div class="healer-config-line"><label>每轮声音提醒 <input v-model.number="member.healthSettings.repeatCount" type="number" min="1" max="10" :aria-label="`${member.name}血量提醒次数`" :disabled="disabled || !member.healthSettings.soundEnabled || member.healthSettings.sound.kind === 'none'" @input="changed" /> 次</label><label>重复间隔 <input v-model.number="member.healthSettings.repeatIntervalSeconds" type="number" min="2" max="300" :aria-label="`${member.name}血量提醒间隔秒数`" :disabled="disabled || !member.healthSettings.soundEnabled || member.healthSettings.sound.kind === 'none' || member.healthSettings.repeatCount <= 1" @input="changed" /> 秒</label><small>次数含首次；恢复安全血量后结束本轮。悬浮提示持续更新。</small></div>
   </div>
  </section>
  <section v-if="section === 'buff'" class="healer-member-section">
   <div class="healer-section-heading">
    <label v-if="templateMode"><input v-model="member.buffSettings.enabled" type="checkbox" :aria-label="`监测 ${member.name} Buff`" @change="changed" /><strong>Buff 提示</strong></label>
    <strong v-if="!templateMode">Buff 提示</strong>
    <div class="healer-collapsed-icons"><img v-for="rule in member.buffSettings.rules" :key="rule.ccId" :src="`/condition-icons/${rule.ccId}.png`" :alt="rule.name" :title="`${rule.name} · ${buffLabel(rule.ccId)}`" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'" /><span v-if="!member.buffSettings.rules.length">展开添加 Buff</span></div>
   </div>
   <div class="healer-detail">
    <div class="healer-config-line"><label><input v-model="member.buffSettings.soundEnabled" type="checkbox" :aria-label="`${member.name}Buff声音提示`" @change="changed" />声音提示</label><small>每个 Buff 可分别设置音效、提前时间、提醒次数与间隔。次数含首次，补上或刷新后重新计数。</small></div>
    <HealerBuffPicker :rules="member.buffSettings.rules" :input-id="`healer-buff-search-${index}`" @add="addRule" />
    <HealerRuleEditor v-for="rule in member.buffSettings.rules" :key="rule.ccId" :rule="rule" :volume="volume" :status="buffLabel(rule.ccId)" :disabled="disabled" :sound-disabled="!member.buffSettings.soundEnabled" @change="changed" @remove="removeRule(rule.ccId)" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)" />
   </div>
  </section>
 <HealerSkillEditor v-if="section === 'skill'" :hide-enable="!templateMode" :member="member" :live="live" :volume="volume" :font-size="fontSize" :icon-size="iconSize" :opacity-percent="opacityPercent" :disabled="disabled" :template-mode="templateMode" @change="changed" @preview="emit('preview', $event)" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)" />
  </div>
 </article>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { healerCombinedPreview } from '@/healerMonitorTypes';
import type { HealerMemberChoice, HealerMemberTemplate, HealerMemberState, HealerBuffRule, HealerCard, HealerOverlayGroup } from '@/healerMonitorTypes';
import HealerSkillEditor from './HealerSkillEditor.vue';
import HealerSoundPicker from './HealerSoundPicker.vue';
import HealerRuleEditor from './HealerRuleEditor.vue';
import HealerBuffPicker from './HealerBuffPicker.vue';
import HealerOverlayVisual from './HealerOverlayVisual.vue';
const props = defineProps<{ member: HealerMemberChoice; live?: HealerMemberState; volume: number; fontSize: number; iconSize: number; opacityPercent: number; index: number; disabled?: boolean; expanded?: boolean; templateMode?: boolean; templates?: HealerMemberTemplate[] }>();
const emit = defineEmits<{ expand: []; change: []; favoriteChange: []; remove: []; saveTemplate: []; applyTemplate: [id: string]; preview: [kind: 'health' | 'buff' | 'skill']; busy: [value: boolean]; notice: [message: string]; error: [message: string] }>();
const section = ref(props.templateMode ? 'health' : ''), selectedTemplate = ref('');
watch(() => props.expanded, value => { if (value === false && !props.templateMode) section.value = ''; });
function toggleSection(value: string) { section.value = section.value === value ? '' : value; if (section.value) emit('expand'); }
const changed = () => emit('change');
const status = computed(() => props.templateMode ? '通用设置 · 可套用于任何队友' : props.live?.active ? '已识别 · 当前可见' : props.live?.waitingReason || '等待识别 · 请让队友切换一次地图');
const healthLabel = computed(() => props.live?.healthPercent != null ? `${Math.round(props.live.healthPercent)}%` : props.live?.healthState === 'stale' ? '等待血量更新' : '等待血量数据');
function buffLabel(id: number) { const value = props.live?.buffs[id]; if (!value || value.state === 'unknown') return '未观测 · 不触发声音提醒'; if (value.state === 'missing') return value.lossReason === 'death' ? '因死亡消失 · 需要补充' : '已结束 · 需要补充'; return value.remainingSeconds == null ? '生效中' : `${value.remainingSeconds} 秒${value.state === 'expiring' ? ' · 即将结束' : ''}`; }
function addRule(rule: HealerBuffRule) { if (!props.member.buffSettings.rules.some(value => value.ccId === rule.ccId) && props.member.buffSettings.rules.length < 16) { props.member.buffSettings.rules.push(rule); changed(); } }
function removeRule(id: number) { props.member.buffSettings.rules = props.member.buffSettings.rules.filter(rule => rule.ccId !== id); changed(); }
const card = (key: string, title: string, value: string, ccId = 0): HealerCard => ({ key, title, value, ccId, memberKey: props.member.key, name: props.member.name, category: key === 'health' ? 'health' : 'buff', state: 'active', flash: false, x: 0, y: 0 });
const healthPreview = computed<HealerOverlayGroup>(() => ({ key: 'health', name: props.member.name, kind: 'health', x: 0, y: 0, width: Math.max(220, props.fontSize * 13), height: props.fontSize * 4 + 24, nameWidth: 0, cellWidth: props.iconSize, cards: [card('health', `血量≤${props.member.healthSettings.threshold}%`, '20%')] }));
const buffPreview = computed<HealerOverlayGroup>(() => healerCombinedPreview(props.member, props.fontSize, props.iconSize));
</script>
<style scoped>
.healer-member { margin: 10px 0; border: 1px solid var(--ui-theme-border); background: var(--ui-theme-surface); }
.healer-member-heading { display: flex; align-items: center; gap: 10px; padding: 12px 14px; background: var(--ui-theme-raised); }
.healer-presence { width: 7px; height: 7px; border-radius: 50%; background: #797c73; flex-shrink: 0; }
.healer-presence.active { background: #a9bf86; box-shadow: 0 0 0 3px #a9bf8617; }
.healer-member-name { display: grid; gap: 3px; min-width: 0; }
.healer-member-name strong { font-size: 14px; overflow-wrap: anywhere; }
.healer-member-name small { font-size: 11px; color: var(--ui-theme-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.healer-favorite { white-space: nowrap; }
.healer-template-actions { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 8px 14px; border-bottom: 1px solid var(--ui-theme-border); }
.healer-template-actions select { min-width: 125px; max-width: 220px; }
.healer-member-section + .healer-member-section { border-top: 1px solid var(--ui-theme-border); }
.healer-section-heading { display: flex; align-items: center; gap: 15px; min-height: 44px; padding: 8px 14px; }
.healer-section-heading > button { margin-left: auto; border: none; background: transparent; color: var(--ui-color-accent); }
.healer-section-heading label { min-width: 98px; }
.healer-inline-status { color: var(--ui-theme-muted); font-size: 12px; }
.healer-inline-status.warning { color: #ffc292; }
.healer-detail { padding: 2px 14px 14px 28px; }
.healer-config-line { display: flex; align-items: center; flex-wrap: wrap; gap: 9px 20px; margin: 10px 0; }
.healer-config-line small { color: var(--ui-theme-muted); font-size: 11px; }
label { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; }
input[type=checkbox] { accent-color: var(--ui-color-accent); }
input[type=number] { width: 68px; }
input[type=number], select, button { border: 1px solid var(--ui-theme-border); background: var(--ui-theme-control); color: var(--ui-theme-text); padding: 4px 7px; font-size: 12px; }
button { cursor: pointer; }
button:disabled { opacity: .45; cursor: default; }
.healer-remove-member { color: var(--ui-theme-muted); }
.healer-collapsed-icons { display: flex; gap: 5px; flex-wrap: wrap; align-items: center; }
.healer-collapsed-icons img { width: 23px; height: 23px; border: 1px solid var(--ui-theme-border); padding: 2px; background: var(--ui-theme-control); }
.healer-collapsed-icons span { color: var(--ui-theme-muted); font-size: 11px; }
.healer-visual-preview { overflow: auto; max-width: 100%; padding: 12px; margin-top: 12px; border: 1px dashed var(--ui-theme-border); background: var(--ui-theme-control); }
@media (max-width: 680px) { .healer-member-heading { flex-wrap: wrap; }.healer-favorite { margin-left: 17px; }.healer-detail { padding-left: 14px; }.healer-section-heading { gap: 7px; } }

.healer-shared-overlay { padding-top: 14px; }
.healer-overlay-controls h3 { font-size: 12px; font-weight: 600; color: var(--ui-theme-text); }
.healer-health-overlay { padding: 10px 0 14px; margin-bottom: 14px; border-bottom: 1px solid var(--ui-theme-border); }
.healer-shared-overlay small, .healer-shared-overlay p { color: var(--ui-theme-muted); font-size: 11px; }
.healer-quick-setting { display: inline-flex; align-items: center; gap: 3px; }
.healer-section-toggle { display: inline-flex; align-items: center; gap: 3px; padding: 5px 6px; border: 1px solid transparent; border-radius: 4px; background: transparent; white-space: nowrap; }
.healer-section-toggle:hover, .healer-section-toggle.selected { background: var(--ui-theme-control); color: var(--ui-color-accent); border-color: var(--ui-theme-border); }
.healer-section-toggle:focus-visible { outline: 2px solid var(--ui-color-accent); outline-offset: 2px; }
.healer-shared-overlay { border-bottom: 1px solid var(--ui-theme-border); }
</style>
