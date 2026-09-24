<template>
 <v-dialog :model-value="open" max-width="1200" scrollable @update:model-value="emit('update:open', $event)">
  <v-card class="healer-panel" aria-label="圣歌监测">
   <v-card-title class="healer-dialog-title"><span><v-icon icon="mdi-heart-pulse" size="20" /> 圣歌监测</span><button type="button" aria-label="关闭圣歌监测" @click="emit('update:open', false)">×</button></v-card-title>
   <section class="healer-basics" aria-label="基础设定">
     <div class="healer-section-title"><h2>基础设定</h2><span role="status">{{ saving ? '正在保存…' : dirty ? '有未保存的修改，请点击保存设定' : '设置已同步' }}</span></div>
     <div class="healer-basics-row">
      <label class="healer-master"><input v-model="draft.enabled" :disabled="saving || isRecordReplay" type="checkbox" @change="changed" />启用提醒</label>
      <label>音量 <input v-model.number="draft.volume" :disabled="saving || isRecordReplay" type="number" min="0" max="100" aria-label="圣歌提示音量" @input="changed" /> %</label>
      <label>悬浮图标 <input v-model.number="draft.iconSize" :disabled="saving || isRecordReplay" type="number" min="16" max="80" aria-label="悬浮图标大小" @input="changed" /> px</label>
      <label>提示字号 <input v-model.number="draft.text.fontSize" :disabled="saving || isRecordReplay" type="number" min="12" max="72" aria-label="提示字号" @input="changed" /> px</label>
      <label title="100% 完全不透明，数值越低越透明">悬浮窗透明度 <input v-model.number="draft.opacityPercent" :disabled="saving || isRecordReplay" type="number" min="20" max="100" step="5" aria-label="悬浮窗透明度" @input="changed" /> %</label>
      <button type="button" class="healer-save" :class="{ 'healer-save-pending': dirty && !saving && !isRecordReplay }" :disabled="saving || audioBusy || !state || isRecordReplay" @click="save()">{{ saving ? '保存中…' : dirty ? '保存设定 · 未保存' : '保存设定' }}</button>
     </div>
    </section>
   <v-card-text class="healer-panel-body">
    <p v-if="isRecordReplay" class="healer-error">历史回放中不提供实时提醒设定。</p>
    <p v-if="error" class="healer-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="healer-notice" role="status">{{ notice }}<button v-if="previewing" type="button" @click="stopPreview">停止预览</button></p>
    <section class="healer-templates" aria-label="通用队友模板">
     <div class="healer-section-title"><h2>通用模板 <span>{{ draft.templates.length }}/32</span></h2><button type="button" :disabled="saving || audioBusy || isRecordReplay || draft.templates.length >= 32" @click="openTemplate()">＋ 新建模板</button></div>
     <p class="healer-help">预先配置「队友1」「队友2」等模板；识别角色后即可套用血量、Buff、声音及坐标。套用后可单独微调，悬浮窗仍显示真实角色 ID。</p>
     <div class="healer-template-list"><div v-for="template in draft.templates" :key="template.id" class="healer-template-entry"><button type="button" :disabled="saving || audioBusy || isRecordReplay" :aria-label="`编辑模板 ${template.name}`" @click="openTemplate(undefined, template)"><strong>{{ template.name }}</strong><small>{{ template.health ? `血量 ≤${template.healthSettings.threshold}%` : '血量关闭' }} · {{ template.buffSettings.rules.length }} 个 Buff</small><span>编辑</span></button><button type="button" :disabled="saving || audioBusy || isRecordReplay" :aria-label="`删除模板 ${template.name}`" @click="removeTarget = { kind: 'template', key: template.id, name: template.name }">×</button></div></div>
     <p v-if="!draft.templates.length" class="healer-help">可新建模板，也可在已配置的队友上点击「存为模板」。</p>
    </section>
    <section class="healer-roster" aria-label="管理队友">
     <div class="healer-section-title"><h2>管理队友 <span>{{ draft.members.length }}</span></h2><button type="button" :aria-expanded="manageOpen" @click="manageOpen = !manageOpen">{{ manageOpen ? '收起可见角色' : '＋ 添加队友' }}</button></div>
     <p class="healer-help">勾选或取消「保存为常用」会立即保存当前设定，无需再点保存。常用队友及其设置会一直保留；开启软件后，请让队友切换一次地图以完成识别，未识别时暂停提醒。</p>
     <div v-if="manageOpen" class="healer-candidates">
      <input v-model="memberSearch" type="search" aria-label="搜索可见队友" placeholder="搜索当前可见角色的 ID" />
      <label class="healer-add-template">添加时套用 <select v-model="addTemplateId" aria-label="添加队友时使用的模板"><option value="">不使用模板</option><option v-for="template in draft.templates" :key="template.id" :value="template.id">{{ template.name }}</option></select></label>
      <p class="healer-help">这里只提供可见角色，请添加实际队友；未添加的角色不会被监控。</p>
      <div class="healer-candidate-list"><button v-for="member in candidates" :key="member.id" type="button" :disabled="saving || isRecordReplay || inRoster(member) || draft.members.length >= 32" @click="addMember(member)"><span>{{ member.name }}</span><small>{{ inRoster(member) ? '已添加 ✓' : '＋ 添加' }}</small></button></div>
      <p v-if="!candidates.length" class="healer-empty">暂未识别到可选角色，请让队友切换地图后重试。</p>
     </div>
     <fieldset :disabled="saving || isRecordReplay">
      <HealerMemberEditor v-for="(member, index) in draft.members" :key="member.key" :member="member" :live="memberState(member)" :index="index" :volume="draft.volume" :font-size="validFontSize" :icon-size="validIconSize" :opacity-percent="validOpacity" :templates="draft.templates" :disabled="saving || audioBusy" @change="changed" @favorite-change="saveFavorite(member)" @remove="removeTarget = { kind: 'member', key: member.key, name: member.name }" @save-template="openTemplate(member)" @apply-template="applyTemplate(member, $event)" @preview="preview(member, $event)" @busy="audioBusy = $event" @notice="showNotice" @error="error = $event" />
     </fieldset>
     <div v-if="!draft.members.length" class="healer-empty healer-empty-roster"><v-icon icon="mdi-account-multiple-outline" size="30" /><strong>先添加需要关注的队友</strong><span>然后分别设置血量和 Buff 提醒。</span></div>
    </section>
    <p class="healer-footnote">常用状态更改后自动保存；其他修改请点击顶部闪烁的「保存设定」。保存成功后停止闪烁，保存失败会保留修改并提示重试。血量超过 30 秒未更新会暂停提醒；已识别队友的已配置 Buff 窗立即显示，未观测的 Buff 按失效样式显示“补充”，不触发声音提醒。声音不依赖此窗口保持打开。</p>
   </v-card-text>
  </v-card>
 </v-dialog>
 <v-dialog :model-value="!!templateEdit" max-width="1100" scrollable :persistent="audioBusy" @update:model-value="!$event && (templateEdit = null)">
  <v-card v-if="templateEdit" class="healer-panel healer-template-dialog" aria-label="编辑通用队友模板">
   <v-card-title class="healer-dialog-title">{{ draft.templates.some(item => item.id === templateEdit!.id) ? '编辑通用模板' : '保存为通用模板' }}</v-card-title>
   <v-card-text class="healer-panel-body">
    <label class="healer-template-name">模板名称 <input v-model="templateEdit.member.name" type="text" maxlength="48" aria-label="模板名称" :disabled="audioBusy" /></label>
    <p class="healer-help">完成编辑后，点击主面板的「保存设定」长期保留。模板不绑定角色，不会自动开启监控。</p>
    <p v-if="error" role="alert" class="healer-error">{{ error }}</p>
    <p v-if="notice" role="status" class="healer-notice">{{ notice }}</p>
    <HealerMemberEditor :key="templateEdit.id" template-mode :member="templateEdit.member" :index="100" :volume="draft.volume" :font-size="validFontSize" :icon-size="validIconSize" :opacity-percent="validOpacity" :disabled="audioBusy" @preview="preview(templateEdit!.member, $event)" @busy="audioBusy = $event" @notice="showNotice" @error="error = $event" />
   </v-card-text>
   <div class="healer-dialog-actions"><button type="button" :disabled="audioBusy" @click="templateEdit = null">取消</button><button type="button" class="healer-primary" :disabled="audioBusy" @click="completeTemplate">完成编辑</button></div>
  </v-card>
 </v-dialog>
 <v-dialog :model-value="!!removeTarget" max-width="430" @update:model-value="!$event && (removeTarget = null)">
  <v-card v-if="removeTarget" class="healer-panel" role="alertdialog" :aria-label="removeTarget.kind === 'member' ? '确认移除队友' : '确认删除模板'">
   <v-card-title class="healer-dialog-title">{{ removeTarget.kind === 'member' ? '移除队友' : '删除模板' }}</v-card-title>
   <v-card-text class="healer-confirm-copy">确定{{ removeTarget.kind === 'member' ? '移除队友' : '删除模板' }}「{{ removeTarget.name }}」？<p>{{ removeTarget.kind === 'member' ? '保存设定后将停止监测该队友，通用模板会保留。' : '已套用此模板的队友设置会保留。删除后请保存设定。' }}</p></v-card-text>
   <div class="healer-dialog-actions"><button type="button" autofocus @click="removeTarget = null">取消</button><button type="button" class="healer-danger" @click="confirmRemove">确认{{ removeTarget.kind === 'member' ? '移除' : '删除' }}</button></div>
  </v-card>
 </v-dialog>
</template>
<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue';
import HealerMemberEditor from './HealerMemberEditor.vue';
import { cloneHealerSettings, makeHealerMember, makeHealerSettings, healerTemplateFromMember, applyHealerTemplate, type HealerMemberTemplate, type HealerMemberChoice, type HealerMemberState, type HealerMonitorState } from '@/healerMonitorTypes';
const props = defineProps<{ open: boolean; isRecordReplay: boolean }>();
const emit = defineEmits<{ 'update:open': [value: boolean] }>();
const state = ref<HealerMonitorState | null>(null), draft = ref(makeHealerSettings());
const dirty = ref(false), saving = ref(false), audioBusy = ref(false), error = ref(''), notice = ref('');
const manageOpen = ref(false), memberSearch = ref(''), previewing = ref(false), addTemplateId = ref('');
const templateEdit = ref<{ id: string; member: HealerMemberChoice } | null>(null);
const removeTarget = ref<{ kind: 'member' | 'template'; key: string; name: string } | null>(null);
const validOpacity = computed(() => Math.max(20, Math.min(100, Number(draft.value.opacityPercent) || 100)));
const validFontSize = computed(() => Math.max(12, Math.min(72, Number(draft.value.text.fontSize) || 16)));
const validIconSize = computed(() => Math.max(16, Math.min(80, Number(draft.value.iconSize) || 30)));
const candidates = computed(() => (state.value?.members || []).filter(member => member.id && member.active && member.name.toLowerCase().includes(memberSearch.value.trim().toLowerCase())));
const inRoster = (member: HealerMemberState) => draft.value.members.some(choice => choice.name === member.name || choice.id === member.id && !!choice.id);
const memberState = (member: HealerMemberChoice) => state.value?.members.find(value => value.key === member.key || value.name === member.name);
const changed = () => { dirty.value = true; notice.value = ''; };
function saveFavorite(member: HealerMemberChoice) {
 changed();
 void save(member.favorite ? `已将 ${member.name} 保存为常用，当前设定已保存。` : `已取消 ${member.name} 的常用状态，当前设定已保存。`);
}
function showNotice(value: string) { notice.value = value; error.value = ''; }
function addMember(member: HealerMemberState) {
 if (inRoster(member) || draft.value.members.length >= 32) return;
 const choice = makeHealerMember(member, draft.value.members.length);
 const template = draft.value.templates.find(item => item.id === addTemplateId.value);
 if (template) applyHealerTemplate(choice, template);
 draft.value.members.push(choice); changed();
}
function applyTemplate(member: HealerMemberChoice, id: string) {
 const template = draft.value.templates.find(item => item.id === id);
 if (!template) return;
 applyHealerTemplate(member, template); changed();
 showNotice(`已为 ${member.name} 套用「${template.name}」，请保存设定。`);
}
function openTemplate(source?: HealerMemberChoice, template?: HealerMemberTemplate) {
 if (!template && draft.value.templates.length >= 32) return;
 let number = 1;
 while (draft.value.templates.some(item => item.name === `队友${number}`)) number++;
 const member = makeHealerMember({ id: '', name: template?.name || `队友${number}` }, draft.value.templates.length);
 if (source) applyHealerTemplate(member, healerTemplateFromMember(source, '', member.name));
 if (template) applyHealerTemplate(member, template);
 templateEdit.value = { id: template?.id || crypto.randomUUID(), member };
 error.value = ''; notice.value = '';
}
function completeTemplate() {
 const edit = templateEdit.value;
 if (!edit) return;
 const name = edit.member.name.trim();
 if (!name || [...name].length > 48) { error.value = '请填写 1–48 个字的模板名称。'; return; }
 if (draft.value.templates.some(item => item.id !== edit.id && item.name === name)) { error.value = '模板名称已存在，请使用其他名称。'; return; }
 if (!validSettings([edit.member])) return;
 const template = healerTemplateFromMember(edit.member, edit.id, name);
 const index = draft.value.templates.findIndex(item => item.id === edit.id);
 if (index < 0) draft.value.templates.push(template); else draft.value.templates[index] = template;
 templateEdit.value = null; changed(); showNotice(`模板「${name}」已更新，请点击「保存设定」长期保留。`);
}
function confirmRemove() {
 const target = removeTarget.value;
 if (!target) return;
 if (target.kind === 'member') draft.value.members = draft.value.members.filter(member => member.key !== target.key);
 else { draft.value.templates = draft.value.templates.filter(template => template.id !== target.key); if (addTemplateId.value === target.key) addTemplateId.value = ''; }
 removeTarget.value = null; changed();
}
let loading = false, disposed = false, generation = 0;
let timer: ReturnType<typeof setInterval> | undefined, previewTimer: ReturnType<typeof setTimeout> | undefined;
async function refresh() {
 if (loading || saving.value || disposed || !props.open) return;
 loading = true; const current = generation;
 try {
  const response = await fetch('/api/healer_monitor', { cache: 'no-store', signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error('读取失败');
  const value = await response.json() as HealerMonitorState;
  if (!value.settings || !Array.isArray(value.members)) throw new Error('状态异常');
  if (disposed || current !== generation) return;
  state.value = value;
  if (!dirty.value) draft.value = cloneHealerSettings(value.settings);
  if (error.value.startsWith('无法获取')) error.value = '';
 } catch { if (!disposed && current === generation) error.value = '无法获取实时监控状态，请确认桌面监测器仍在运行。'; }
 finally { loading = false; }
}
function validSettings(extra: HealerMemberChoice[] = []): boolean {
 const numbers = [[draft.value.volume, 0, 100], [draft.value.text.fontSize, 12, 72], [draft.value.iconSize, 16, 80], [draft.value.opacityPercent, 20, 100]];
 const choices = [...draft.value.members, ...draft.value.templates, ...extra];
 for (const member of choices) {
  numbers.push([member.healthSettings.threshold, 5, 95], [member.healthSettings.repeatCount, 1, 10], [member.healthSettings.repeatIntervalSeconds, 2, 300]);
  for (const position of [member.healthSettings.overlay, member.buffSettings.overlay]) numbers.push([position.x, -32000, 32000], [position.y, -32000, 32000]);
  for (const rule of member.buffSettings.rules) numbers.push([rule.warningSeconds, 0, 60], [rule.flashSeconds, 0, 60], [rule.manualDurationSeconds, 1, 86400], [rule.repeatCount, 1, 10], [rule.repeatIntervalSeconds, 2, 300], [rule.deathLoss.repeatCount, 1, 10], [rule.deathLoss.repeatIntervalSeconds, 2, 300]);
 }
 if (!numbers.every(([value, min, max]) => Number.isInteger(value) && value >= min && value <= max)) { error.value = '请填写有效数值：图标 16–80、字号 12–72、透明度 20–100%、血量 5–95%、提前时间 0–60 秒、提醒次数 1–10 次、间隔 2–300 秒。'; return false; }
 for (const member of choices) {
  const sounds = [member.healthSettings.sound, ...member.buffSettings.rules.flatMap(rule => [rule.sound, rule.deathLoss.sound])];
  if (sounds.some(sound => sound.kind === 'custom' && !sound.soundId)) { error.value = `${member.name} 的自定义音效尚未选择文件。`; return false; }
 }
 return true;
}
async function save(successNotice = '已保存。通用模板和常用队友将在下次启动后继续保留。') {
 if (!state.value || saving.value || audioBusy.value || props.isRecordReplay || !validSettings()) return;
 saving.value = true; generation++;
 try {
  const settings = cloneHealerSettings(draft.value);
  settings.soundEnabled = true; settings.text.enabled = true;
  const response = await fetch('/api/healer_monitor', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(settings), signal: AbortSignal.timeout(8000) });
  if (!response.ok) throw new Error((await response.text()).trim());
  draft.value = cloneHealerSettings(await response.json()); dirty.value = false;
  showNotice(successNotice);
 } catch (reason) { error.value = `保存失败：${String(reason)}`; }
 finally { saving.value = false; void refresh(); }
}
async function preview(member: HealerMemberChoice, kind: 'health' | 'buff') {
 if (!validSettings([member])) return;
 try {
  const response = await fetch('/api/healer_monitor/text_preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: draft.value.text, iconSize: draft.value.iconSize, opacityPercent: draft.value.opacityPercent, member, kind }), signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error((await response.text()).trim());
  previewing.value = true; clearTimeout(previewTimer); previewTimer = setTimeout(() => { previewing.value = false; }, 8000);
  showNotice(`${member.name} 的${kind === 'health' ? '血量' : 'Buff'}悬浮窗已按当前坐标预览 8 秒；预览不触发声音。`);
 } catch (reason) { error.value = `预览失败：${String(reason)}`; }
}
async function stopPreview() {
 previewing.value = false; clearTimeout(previewTimer);
 try { await fetch('/api/healer_monitor/text_preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stop: true }), signal: AbortSignal.timeout(3000) }); } catch { /* Preview expires automatically. */ }
}
watch(() => props.open, open => { generation++; clearInterval(timer); if (open) { void refresh(); timer = setInterval(refresh, 750); } else { templateEdit.value = null; removeTarget.value = null; if (previewing.value) void stopPreview(); } }, { immediate: true });
onBeforeUnmount(() => { disposed = true; generation++; clearInterval(timer); clearTimeout(previewTimer); if (previewing.value) void stopPreview(); });
</script>
<style scoped>
.healer-panel { color: var(--ui-theme-text); background: var(--ui-theme-surface); border: 1px solid var(--ui-theme-border); border-radius: 3px !important; }
.healer-dialog-title { display: flex; align-items: center; justify-content: space-between; padding: 13px 18px; font-size: 16px; border-bottom: 1px solid var(--ui-theme-border); background: var(--ui-theme-raised); }
.healer-dialog-title button { border: none; background: transparent; font-size: 25px; line-height: 1; }
.healer-panel-body { padding: 0 18px 14px !important; }
.healer-basics { flex: 0 0 auto; padding: 15px 18px 16px; background: var(--ui-theme-surface); border-bottom: 1px solid var(--ui-theme-border); }
.healer-section-title { display: flex; align-items: center; justify-content: space-between; gap: 12px; margin-bottom: 12px; }
.healer-section-title h2 { font-size: 13px; font-weight: 650; margin: 0; }
.healer-section-title h2 span { font-size: 11px; font-weight: 400; margin-left: 7px; color: var(--ui-theme-muted); }
.healer-section-title > span { font-size: 11px; color: var(--ui-theme-muted); }
.healer-basics-row { display: flex; align-items: center; gap: 12px 24px; flex-wrap: wrap; }
label { display: inline-flex; align-items: center; gap: 7px; font-size: 12px; }
input[type=checkbox] { accent-color: var(--ui-color-accent); }
input[type=number] { width: 57px; }
input[type=number], input[type=search], input[type=text], select, button { color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); padding: 5px 8px; font-size: 12px; }
button { cursor: pointer; }
button:disabled { opacity: .5; cursor: default; }
.healer-save { margin-left: auto; color: var(--ui-color-accent); border-color: var(--ui-color-accent); min-width: 88px; }
.healer-save-pending { color: #ffe6ae; border-color: #dab65e; animation: healer-save-pulse 1.8s ease-in-out infinite; }
@keyframes healer-save-pulse { 0%, 100% { background: var(--ui-theme-control); box-shadow: 0 0 0 0 #dab65e00; } 50% { background: #5c4729; box-shadow: 0 0 0 3px #dab65e55; } }
@media (prefers-reduced-motion: reduce) { .healer-save-pending { animation: none; background: #5c4729; box-shadow: 0 0 0 2px #dab65e55; } }
.healer-roster, .healer-templates { padding-top: 20px; }
.healer-templates { padding-bottom: 10px; border-bottom: 1px solid var(--ui-theme-border); }
.healer-template-list { display: flex; flex-wrap: wrap; gap: 8px; }
.healer-template-entry { display: flex; max-width: 100%; }
.healer-template-entry > button:first-child { display: flex; align-items: center; flex-wrap: wrap; gap: 9px; text-align: left; }
.healer-template-entry strong { max-width: 240px; overflow-wrap: anywhere; }
.healer-template-entry small { color: var(--ui-theme-muted); }
.healer-template-entry span { color: var(--ui-color-accent); }
.healer-add-template { margin-top: 10px; }
.healer-template-name { margin-top: 18px; }
.healer-template-name input { width: 240px; }
.healer-dialog-actions { display: flex; justify-content: flex-end; gap: 10px; padding: 14px 18px; border-top: 1px solid var(--ui-theme-border); }
.healer-primary { color: var(--ui-color-accent); border-color: var(--ui-color-accent); }
.healer-danger { color: #ffc0b4; border-color: #a27669; }
.healer-confirm-copy { padding: 20px !important; font-size: 14px; overflow-wrap: anywhere; }
.healer-confirm-copy p { margin-top: 10px; font-size: 12px; color: var(--ui-theme-muted); }
.healer-help, .healer-footnote { color: var(--ui-theme-muted); font-size: 12px; line-height: 1.7; margin: 8px 0 12px; }
.healer-footnote { margin: 14px 0 0; font-size: 11px; }
.healer-candidates { border: 1px solid var(--ui-theme-border); padding: 12px; background: var(--ui-theme-control); }
.healer-candidates > input { width: 100%; }
.healer-candidate-list { display: flex; flex-wrap: wrap; gap: 7px; max-height: 220px; overflow: auto; }
.healer-candidate-list button { display: flex; align-items: center; justify-content: space-between; gap: 24px; min-width: 210px; text-align: left; padding: 9px; }
.healer-candidate-list small { color: var(--ui-theme-muted); font-size: 11px; }
fieldset { border: none; padding: 0; min-width: 0; }
.healer-empty { color: var(--ui-theme-muted); font-size: 12px; text-align: center; margin: 14px 0; }
.healer-empty-roster { display: grid; justify-items: center; gap: 8px; padding: 30px 12px; border: 1px dashed var(--ui-theme-border); }
.healer-empty-roster strong { color: var(--ui-theme-text); font-size: 13px; font-weight: 500; }
.healer-error, .healer-notice { margin: 12px 0; padding: 9px 12px; border-left: 2px solid currentColor; background: var(--ui-theme-control); font-size: 12px; line-height: 1.6; }
.healer-error { color: #ffb4a7; }.healer-notice { color: #bcd3a0; }.healer-notice button { margin-left: 15px; }
</style>
