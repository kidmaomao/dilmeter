<template>
 <component :is="embedded ? 'div' : VDialog" :model-value="open" :class="{ 'embedded-settings': embedded }" max-width="1200" scrollable @update:model-value="emit('update:open', $event)">
  <v-card class="healer-panel teammate-settings-panel" :class="{ 'healer-embedded': embedded }" aria-label="圣歌监控">
   <v-card-title class="healer-dialog-title teammate-panel-header"><span><v-icon icon="mdi-heart-pulse" size="20" /> 圣歌监控</span><button v-if="!embedded" type="button" aria-label="关闭圣歌监控" @click="emit('update:open', false)">×</button></v-card-title>
   <section class="healer-basics teammate-basics" aria-label="基础设定">
     <div class="healer-section-title teammate-section-heading"><h2>基础设定</h2><span role="status">{{ saving ? '正在保存…' : dirty ? '有未保存的修改' : '设置已同步' }}</span><button type="button" class="healer-save teammate-save" :class="{ 'healer-save-pending': dirty && !saving && !isRecordReplay }" :disabled="saving || audioBusy || !state || isRecordReplay" @click="save()">{{ saving ? '保存中…' : dirty ? '保存设定 · 未保存' : '保存设定' }}</button></div>
     <div class="healer-basics-row teammate-basics-row">
      <label class="healer-master"><input v-model="draft.enabled" :disabled="saving || isRecordReplay" type="checkbox" @change="changed" />启用提醒</label>
      <label>音量 <input v-model.number="draft.volume" :disabled="saving || isRecordReplay" type="number" min="0" max="100" aria-label="圣歌提示音量" @input="changed" /> %</label>
     </div>
     <div class="healer-basics-row healer-team-controls" aria-label="队友浮窗">
      <strong>队友浮窗</strong>
      <label>队友浮窗 X <input v-model.number="draft.text.x" :disabled="saving || isRecordReplay" type="number" min="-32000" max="32000" aria-label="队友浮窗横坐标" @input="changed" /></label>
      <label>Y <input v-model.number="draft.text.y" :disabled="saving || isRecordReplay" type="number" min="-32000" max="32000" aria-label="队友浮窗纵坐标" @input="changed" /></label>
      <label>大小 <input v-model.number="draft.text.scalePercent" :disabled="saving || isRecordReplay" type="number" min="50" max="200" step="5" aria-label="队友浮窗大小" @input="changed" /> %</label>
      <label title="100% 完全不透明，数值越低越透明">队友浮窗透明度 <input v-model.number="draft.opacityPercent" :disabled="saving || isRecordReplay" type="number" min="20" max="100" step="5" aria-label="队友浮窗透明度" @input="changed" /> %</label>
      <button type="button" :disabled="saving || isRecordReplay" @click="preview()">预览队友浮窗</button>
     </div>
    </section>
   <v-card-text class="healer-panel-body teammate-panel-body">
    <p v-if="isRecordReplay" class="healer-error">历史回放中不提供实时提醒设定。</p>
    <p v-if="error" class="healer-error" role="alert">{{ error }}</p>
    <p v-if="notice" class="healer-notice" role="status">{{ notice }}<button v-if="previewing" type="button" @click="stopPreview">停止预览</button></p>
    <section class="healer-templates" aria-label="通用队友模板">
     <div class="healer-section-title"><h2>通用模板 <span>{{ draft.templates.length }}/32</span></h2><button type="button" :disabled="saving || audioBusy || isRecordReplay || draft.templates.length >= 32" @click="openTemplate()">＋ 新建模板</button></div>
     <p class="healer-help">预先配置「队友1」「队友2」等模板。识别角色后可套用血量、Buff、技能和音效，手动套用后也可单独微调。</p>
     <div class="healer-template-list"><div v-for="template in draft.templates" :key="template.id" class="healer-template-entry"><button type="button" :disabled="saving || audioBusy || isRecordReplay" :aria-label="`编辑模板 ${template.name}`" @click="openTemplate(undefined, template)"><strong>{{ template.name }}</strong><small>{{ healerTemplateSummary(template) }}</small><span>编辑</span></button><button type="button" :disabled="saving || audioBusy || isRecordReplay" :aria-label="`删除模板 ${template.name}`" @click="removeTarget = { kind: 'template', key: template.id, name: template.name }">×</button></div></div>
     <p v-if="!draft.templates.length" class="healer-help">可新建模板，也可在已配置的队友上点击「存为模板」。</p>
    </section>
    <section class="healer-self-mana" aria-label="自身魔法值监测">
     <div class="healer-section-title"><h2>自身魔法值</h2><button v-if="!selfMember" type="button" :disabled="saving || audioBusy || isRecordReplay || draft.members.length >= 32" @click="addSelfMana">＋ 添加自身魔法值监测</button></div>
     <p class="healer-help">本人常驻队友浮窗。启用自身魔法值监测后，蓝条显示在本人血条下方。</p>
     <HealerMemberEditor v-if="selfMember" :member="selfMember" :live="memberState(selfMember)" :index="99" :expanded="expandedMember === selfMember.key" :volume="draft.volume" :disabled="saving || audioBusy || isRecordReplay" @expand="expandedMember = selfMember.key" @change="changed" @remove="removeSelfMana" @preview="preview(selfMember, $event)" @busy="audioBusy = $event" @notice="showNotice" @error="error = $event" />
    </section>
    <section class="healer-roster" aria-label="管理队友">
     <div class="healer-section-title"><h2>管理队友 <span>{{ favoriteMembers.length + temporaryMembers.length }}</span></h2><button type="button" :aria-expanded="manageOpen" @click="manageOpen = !manageOpen">{{ manageOpen ? '收起可见角色' : '＋ 添加队友' }}</button></div>
     <div class="healer-auto-add">
      <label><input v-model="draft.autoAddInDungeon" type="checkbox" :disabled="saving || isRecordReplay" @change="changeAutoAdd" />自动添加组队成员和副本队友</label>
      <label v-if="draft.autoAddInDungeon">使用模板 <select v-model="draft.autoAddTemplateId" :disabled="saving || isRecordReplay" aria-label="自动添加队友模板" @change="changed">
       <option value="">选择通用模板</option>
       <option v-for="template in draft.templates" :key="template.id" :value="template.id">{{ template.name }}</option>
      </select></label>
      <p class="healer-help">收到游戏队伍名单后直接显示队员。开启自动添加可给队员套用模板，也可在副本内名单尚未同步时监测附近角色。</p>
      <p v-if="draft.autoAddInDungeon && !draft.templates.length" class="healer-help">请先在通用模板中新建一个模板。</p>
     </div>
     <div v-if="manageOpen" class="healer-candidates">
      <input v-model="memberSearch" type="search" aria-label="搜索可见队友" placeholder="搜索当前可见角色的 ID" />
      <label class="healer-add-template">添加时套用 <select v-model="addTemplateId" aria-label="添加队友时使用的模板"><option value="">不使用模板</option><option v-for="template in draft.templates" :key="template.id" :value="template.id">{{ template.name }}</option></select></label>
      <p class="healer-help">在附近角色中选择需要手动添加的队友。</p>
      <div class="healer-candidate-list"><button v-for="member in candidates" :key="member.id" type="button" :disabled="saving || isRecordReplay || inRoster(member) || draft.members.length >= 32" @click="addMember(member)"><span>{{ member.name }}</span><small>{{ inRoster(member) ? '已添加 ✓' : '＋ 添加' }}</small></button></div>
      <p v-if="!candidates.length" class="healer-empty">暂未识别到可选角色，请让队友切换地图后重试。</p>
     </div>
     <div v-for="group in rosterGroups" :key="group.key" class="healer-roster-group">
      <button v-if="group.favorite" type="button" class="healer-roster-toggle" aria-controls="healer-favorite-members" :aria-expanded="favoritesOpen" :aria-label="favoritesOpen ? '收起常用队友' : '展开常用队友'" @click="favoritesOpen = !favoritesOpen">
       <v-icon :icon="favoritesOpen ? 'mdi-chevron-down' : 'mdi-chevron-right'" size="16" /><strong>常用队友</strong><span>{{ group.members.length }}</span>
      </button>
      <h3 v-else>临时队友 <span>{{ group.members.length }}</span></h3>
      <fieldset v-show="!group.favorite || favoritesOpen" :id="group.favorite ? 'healer-favorite-members' : undefined" :disabled="saving || isRecordReplay">
       <HealerMemberEditor v-for="(member, index) in group.members" :key="member.key" :expanded="expandedMember === member.key" @expand="expandedMember = member.key" :member="member" :live="memberState(member)" :index="group.favorite ? index : index + 32" :volume="draft.volume" :templates="draft.templates" :automatic="isInstanceMember(member)" :disabled="saving || audioBusy" @change="memberChanged(member)" @favorite-change="toggleFavorite(member)" @remove="removeTarget = { kind: 'member', key: member.key, name: member.name }" @save-template="openTemplate(member)" @apply-template="applyTemplate(member, $event)" @preview="preview(member, $event)" @busy="audioBusy = $event" @notice="showNotice" @error="error = $event" />
       <p v-if="!group.members.length" class="healer-help">{{ group.favorite ? '暂无常用队友' : '暂无临时队友' }}</p>
      </fieldset>
     </div>
    </section>
    <p class="healer-footnote">识别到阿尔卡纳职业后会添加默认乐曲和技能，可在队友设置中编辑或删除。改动请点击「保存设定」。</p>
   </v-card-text>
  </v-card>
 </component>
 <v-dialog :model-value="!!templateEdit" max-width="1100" scrollable :persistent="audioBusy" @update:model-value="!$event && (templateEdit = null)">
  <v-card v-if="templateEdit" class="healer-panel healer-template-dialog" aria-label="编辑通用队友模板">
   <v-card-title class="healer-dialog-title">{{ draft.templates.some(item => item.id === templateEdit!.id) ? '编辑通用模板' : '保存为通用模板' }}</v-card-title>
   <v-card-text class="healer-panel-body">
    <label class="healer-template-name">模板名称 <input v-model="templateEdit.member.name" type="text" maxlength="48" aria-label="模板名称" :disabled="audioBusy" /></label>
    <p class="healer-help">完成编辑后，点击主面板的「保存设定」长期保留。模板可用于手动添加或副本自动添加。</p>
    <p v-if="error" role="alert" class="healer-error">{{ error }}</p>
    <p v-if="notice" role="status" class="healer-notice">{{ notice }}</p>
    <HealerMemberEditor :key="templateEdit.id" template-mode :member="templateEdit.member" :index="100" :volume="draft.volume" :disabled="audioBusy" @preview="preview(templateEdit!.member, $event)" @busy="audioBusy = $event" @notice="showNotice" @error="error = $event" />
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
import { VDialog } from 'vuetify/components';
import HealerMemberEditor from './HealerMemberEditor.vue';
import { cloneHealerSettings, makeHealerMember, makeHealerSelf, healerTemplateSummary, makeHealerSettings, healerTemplateFromMember, applyHealerTemplate, type HealerMemberTemplate, type HealerMemberChoice, type HealerMemberState, type HealerMonitorState } from '@/healerMonitorTypes';
const props = defineProps<{ open: boolean; isRecordReplay: boolean; embedded?: boolean }>();
const emit = defineEmits<{ 'update:open': [value: boolean] }>();
const state = ref<HealerMonitorState | null>(null), draft = ref(makeHealerSettings());
const settingsDirty = ref(false), saving = ref(false), audioBusy = ref(false), error = ref(''), notice = ref('');
const temporaryEdits = ref<Record<string, HealerMemberChoice>>({});
const dirty = computed(() => settingsDirty.value || Object.keys(temporaryEdits.value).length > 0);
const expandedMember = ref(''), favoritesOpen = ref(false);
const selfMember = computed(() => draft.value.members.find(member => member.self));
const favoriteMembers = computed(() => draft.value.members.filter(member => !member.self && member.favorite).sort(byPartyOrder));
const temporaryMembers = computed(() => {
 const manual = draft.value.members.filter(member => !member.self && !member.favorite);
 if (!draft.value.autoAddInDungeon) return manual.sort(byPartyOrder);
 const automatic = (state.value?.temporaryMembers ?? []).filter(member => !inRoster(member))
  .map(member => temporaryEdits.value[member.key] ?? member);
 return [...manual, ...automatic].sort(byPartyOrder);
});
const rosterGroups = computed(() => [
 { key: 'temporary', favorite: false, members: temporaryMembers.value },
 { key: 'favorite', favorite: true, members: favoriteMembers.value },
]);
function addSelfMana() { if (selfMember.value || draft.value.members.length >= 32) return; draft.value.members.push(makeHealerSelf()); expandedMember.value = 'self'; changed(); }
function removeSelfMana() { draft.value.members = draft.value.members.filter(member => !member.self); changed(); }
const manageOpen = ref(false), memberSearch = ref(''), previewing = ref(false), addTemplateId = ref('');
const templateEdit = ref<{ id: string; member: HealerMemberChoice } | null>(null);
const removeTarget = ref<{ kind: 'member' | 'template'; key: string; name: string } | null>(null);
const candidates = computed(() => (state.value?.members || []).filter(member => member.key !== 'self' && member.id && member.active && (!member.automatic || !draft.value.autoAddInDungeon) && member.name.toLowerCase().includes(memberSearch.value.trim().toLowerCase())));
const inRoster = (member: Pick<HealerMemberChoice, 'name' | 'id'>) => draft.value.members.some(choice => !choice.self && (choice.name === member.name || choice.id === member.id && !!choice.id));
const isInstanceMember = (member: HealerMemberChoice) => member.key.startsWith('instance:');
const memberState = (member: HealerMemberChoice) => state.value?.members.find(value => value.key === member.key)
 ?? state.value?.members.find(value => !member.self && (member.id ? value.id === member.id : value.name === member.name));
function byPartyOrder(first: HealerMemberChoice, second: HealerMemberChoice): number {
 const firstSlot = memberState(first)?.partySlot ?? Number.MAX_SAFE_INTEGER;
 const secondSlot = memberState(second)?.partySlot ?? Number.MAX_SAFE_INTEGER;
 return firstSlot - secondSlot;
}
const changed = () => { settingsDirty.value = true; notice.value = ''; };
function memberChanged(member: HealerMemberChoice) {
 if (isInstanceMember(member)) {
  temporaryEdits.value[member.key] = member;
  notice.value = '';
 } else {
  changed();
 }
}
function changeAutoAdd() {
 if (draft.value.autoAddInDungeon && !draft.value.autoAddTemplateId) {
  draft.value.autoAddTemplateId = draft.value.templates[0]?.id ?? '';
 }
 changed();
}
async function toggleFavorite(member: HealerMemberChoice) {
 if (member.favorite) {
  member.favorite = false;
  changed();
  await save(`已取消 ${member.name} 的常用状态。`);
  return;
 }
 if (isInstanceMember(member) && draft.value.members.length >= 32) {
  error.value = '队友列表已满，请先移除一位队友。';
  return;
 }
 const favorite = { ...member, key: `player:${member.name}`, favorite: true };
 const saved = await save(`已将 ${member.name} 添加为常用。`, favorite);
 if (saved) {
  favoritesOpen.value = true;
  expandedMember.value = favorite.key;
 }
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
 applyHealerTemplate(member, template); memberChanged(member);
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
 else {
  draft.value.templates = draft.value.templates.filter(template => template.id !== target.key);
  if (addTemplateId.value === target.key) addTemplateId.value = '';
  if (draft.value.autoAddTemplateId === target.key) {
   draft.value.autoAddInDungeon = false;
   draft.value.autoAddTemplateId = '';
  }
 }
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
  if (state.value?.dungeonSession !== value.dungeonSession) temporaryEdits.value = {};
  state.value = value;
  if (!settingsDirty.value) draft.value = cloneHealerSettings(value.settings);
  if (error.value.startsWith('无法获取')) error.value = '';
 } catch { if (!disposed && current === generation) error.value = '无法获取实时监控状态，请确认桌面监测器仍在运行。'; }
 finally { loading = false; }
}
function validSettings(extra: HealerMemberChoice[] = []): boolean {
 const numbers = [[draft.value.volume, 0, 100], [draft.value.text.scalePercent, 50, 200], [draft.value.opacityPercent, 20, 100], [draft.value.text.x, -32000, 32000], [draft.value.text.y, -32000, 32000]];
 const mana = selfMember.value?.manaSettings;
 if (mana) numbers.push([mana.threshold, 5, 95], [mana.repeatCount, 1, 10], [mana.repeatIntervalSeconds, 2, 300]);
 const choices = [...draft.value.members, ...draft.value.templates, ...extra];
 for (const member of choices) {
  numbers.push([member.healthSettings.threshold, 5, 95], [member.healthSettings.repeatCount, 1, 10], [member.healthSettings.repeatIntervalSeconds, 2, 300]);
  for (const rule of member.skillSettings.rules) { if (!Number.isInteger(rule.skillId) || rule.skillId < 1 || rule.skillId > 65535 || !Number.isFinite(rule.cooldownSeconds) || rule.cooldownSeconds < .1 || rule.cooldownSeconds > 86400) { error.value = '请填写有效技能 ID 和冷却时间（0.1–86400 秒）。'; return false; } }
  for (const rule of member.buffSettings.rules) numbers.push([rule.warningSeconds, 0, 60], [rule.flashSeconds, 0, 60], [rule.manualDurationSeconds, 1, 86400], [rule.repeatCount, 1, 10], [rule.repeatIntervalSeconds, 2, 300], [rule.deathLoss.repeatCount, 1, 10], [rule.deathLoss.repeatIntervalSeconds, 2, 300]);
 }
 if (!numbers.every(([value, min, max]) => Number.isInteger(value) && value >= min && value <= max)) { error.value = '请填写有效数值：图标 16–80、字号 12–72、透明度 20–100%、血量/魔法值 5–95%、提前时间 0–60 秒、提醒次数 1–10 次、间隔 2–300 秒。'; return false; }
 if (mana?.sound.kind === 'custom' && !mana.sound.soundId) { error.value = '自身魔法值的自定义音效尚未选择文件。'; return false; }
 for (const member of choices) {
  const sounds = [member.healthSettings.sound, ...member.skillSettings.rules.map(rule => rule.sound), ...member.buffSettings.rules.flatMap(rule => [rule.sound, rule.deathLoss.sound])];
  if (sounds.some(sound => sound.kind === 'custom' && !sound.soundId)) { error.value = `${member.name} 的自定义音效尚未选择文件。`; return false; }
 }
 return true;
}
async function save(successNotice?: string, favorite?: HealerMemberChoice) {
 if (!state.value || saving.value || audioBusy.value || props.isRecordReplay) return false;
 const temporary = draft.value.autoAddInDungeon
  ? Object.values(temporaryEdits.value).filter(member => member.id !== favorite?.id)
  : [];
 if (!validSettings(favorite ? [...temporary, favorite] : temporary)) return false;
 if (draft.value.autoAddInDungeon && !draft.value.templates.some(template => template.id === draft.value.autoAddTemplateId)) {
  error.value = '请为自动添加的队友选择一个通用模板。';
  return false;
 }
 saving.value = true; generation++;
 try {
  const settings = cloneHealerSettings(draft.value);
  if (favorite) {
   const index = settings.members.findIndex(member => member.key === favorite.key);
   if (index < 0) settings.members.push(favorite); else settings.members[index] = favorite;
  }
  settings.soundEnabled = true; settings.text.enabled = true;
  const body = { ...settings, dungeonSession: state.value.dungeonSession, temporaryMembers: temporary };
  const response = await fetch('/api/healer_monitor', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(body), signal: AbortSignal.timeout(8000) });
  if (!response.ok) throw new Error((await response.text()).trim());
  draft.value = cloneHealerSettings(await response.json());
  settingsDirty.value = false;
  temporaryEdits.value = {};
  const dungeonChanged = response.headers.get('X-Dilmeter-Dungeon-Changed') === '1';
  showNotice(dungeonChanged ? '已保存设定。队伍或副本已切换，临时队友的修改已失效。' : successNotice ?? (temporary.length ? '已保存。临时队友的修改在当前队伍或副本内有效。' : '已保存。'));
  return true;
 } catch (reason) {
  error.value = `保存失败：${String(reason)}`;
  return false;
 } finally { saving.value = false; void refresh(); }
}
async function preview(member?: HealerMemberChoice, kind: 'team' | 'health' | 'mana' | 'buff' | 'skill' = 'team') {
 if (!validSettings(member ? [member] : [])) return;
 try {
  const response = await fetch('/api/healer_monitor/text_preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: draft.value.text, iconSize: draft.value.iconSize, opacityPercent: draft.value.opacityPercent, member, kind }), signal: AbortSignal.timeout(5000) });
  if (!response.ok) throw new Error((await response.text()).trim());
  previewing.value = true; clearTimeout(previewTimer); previewTimer = setTimeout(() => { previewing.value = false; }, 8000);
  showNotice(member ? `${member.name} 的浮窗预览持续 8 秒。` : '队友浮窗示例预览持续 8 秒。');
 } catch (reason) { error.value = `预览失败：${String(reason)}`; }
}
async function stopPreview() {
 previewing.value = false; clearTimeout(previewTimer);
 try { await fetch('/api/healer_monitor/text_preview', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ stop: true }), signal: AbortSignal.timeout(3000) }); } catch { /* Preview expires automatically. */ }
}
watch(() => props.open, open => { generation++; clearInterval(timer); if (open) { void refresh(); timer = setInterval(refresh, 300); } else { templateEdit.value = null; removeTarget.value = null; if (previewing.value) void stopPreview(); } }, { immediate: true });
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
.healer-error, .healer-notice { margin: 12px 0; padding: 9px 12px; border-left: 2px solid currentColor; background: var(--ui-theme-control); font-size: 12px; line-height: 1.6; }
.healer-error { color: #ffb4a7; }.healer-notice { color: #bcd3a0; }.healer-notice button { margin-left: 15px; }

.healer-team-controls { margin-top: 12px; }
.healer-auto-add { display: flex; align-items: center; gap: 8px 16px; flex-wrap: wrap; padding: 10px 0 12px; }
.healer-auto-add .healer-help { flex-basis: 100%; margin: 0; }
.healer-auto-add select { max-width: 230px; }
.healer-roster-group { border-top: 1px solid var(--ui-theme-border); padding: 8px 0; }
.healer-roster-group h3, .healer-roster-toggle { font-size: 13px; font-weight: 650; margin: 0 0 7px; }
.healer-roster-toggle { display: inline-flex; align-items: center; gap: 5px; border: 0; background: transparent; padding: 3px 0; }
.healer-roster-group h3 span, .healer-roster-toggle span { color: var(--ui-theme-muted); font-size: 11px; font-weight: 400; margin-left: 5px; }
</style>
