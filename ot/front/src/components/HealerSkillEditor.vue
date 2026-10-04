<template>
 <section class="healer-skill-section">
  <header><strong v-if="hideEnable">{{ t('技能提醒') }}</strong><label v-else><input v-model="member.skillSettings.enabled" type="checkbox" @change="emit('change')" /><strong>{{ t('技能提醒') }}</strong></label><span>{{ member.skillSettings.rules.length }} {{ t('个技能') }}</span><button :aria-expanded="open" @click="open = !open">{{ t(open ? '收起' : '展开设定') }}</button></header>
  <div v-if="open" class="detail">
   <p>{{ t('从观测到的技能释放开始计算队友冷却，用于对齐爆发。未观测到释放时显示“未观测”；请填写包含减冷却后的实际秒数。') }}</p>
   <small>{{ t("Buff 与技能共用一个窗口，分组显示；每行最多 4 个图标，超出自动换行。") }}</small>
   <div class="line"><label><input v-model="member.skillSettings.soundEnabled" type="checkbox" @change="emit('change')" />{{ t('声音提示') }}</label><label><input v-model="member.skillSettings.overlay.enabled" type="checkbox" @change="emit('change')" />{{ t('悬浮窗提示') }}</label><label>X<input v-model.number="member.buffSettings.overlay.x" type="number" min="-32000" max="32000" @input="emit('change')" /></label><label>Y<input v-model.number="member.buffSettings.overlay.y" type="number" min="-32000" max="32000" @input="emit('change')" /></label><button :disabled="disabled || !member.skillSettings.rules.length" @click="emit('preview', 'skill')">{{ t('屏幕预览 8 秒') }}</button></div>
   <div class="line"><EffectTimerSourcePicker source-type="skill" :source-id="selectedId" :options="options" :normalize-search="normalizeNameSearch" :translate="translate" @select="add" /><small>{{ t('搜索名称或技能 ID 添加，最多 16 个') }}</small></div>
   <div v-for="rule in member.skillSettings.rules" :key="rule.skillId" class="skill-rule"><div class="line"><strong>{{ skillResourceName(rule.skillId, rule.name) }}</strong><small>{{ rule.skillId }}</small><span>{{ t(status(rule.skillId)) }}</span><label>{{ t('冷却秒数') }}<input v-model.number="rule.cooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @input="emit('change')" /></label><button :disabled="disabled" @click="remove(rule.skillId)">{{ t('删除') }}</button></div><HealerSoundPicker v-model="rule.sound" :label="member.name + rule.name" :volume="volume" :disabled="disabled || !member.skillSettings.soundEnabled" @update:model-value="emit('change')" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)" /></div>
   <div v-if="preview.cards.length" class="visual-preview"><HealerOverlayVisual :group="preview" :font-size="fontSize" :icon-size="iconSize" :opacity-percent="opacityPercent" /></div>
  </div>
 </section>
</template>
<script setup lang="ts">
import { computed, inject, ref, type Ref } from 'vue';
import { skillResourceName } from '@/gameTerms';
import { normalizeNameSearch } from '@/uiLocale';
import { skills } from '@/data/skill';
import { healerCombinedPreview, makeHealerSound, type HealerMemberChoice, type HealerMemberState, type HealerOverlayGroup } from '@/healerMonitorTypes';
import EffectTimerSourcePicker from './EffectTimerSourcePicker.vue';
import HealerSoundPicker from './HealerSoundPicker.vue';
import HealerOverlayVisual from './HealerOverlayVisual.vue';
const props = withDefaults(defineProps<{ member: HealerMemberChoice; live?: HealerMemberState; volume: number; fontSize: number; iconSize: number; opacityPercent: number; disabled?: boolean; templateMode?: boolean; hideEnable?: boolean; translate?: (value: string) => string }>(), { translate: (value: string) => value });
const emit = defineEmits<{ change: []; preview: [kind: 'skill']; busy: [value: boolean]; notice: [message: string]; error: [message: string] }>();
const t = (value: string) => props.translate(value);
const open = ref(!!props.templateMode), selectedId = ref(59005);
const resources = inject<Ref<Record<number, string>>>('skillNameMap', ref({}));
const normalizeSearch = (value: string) => {
 const traditional = '連續擊閃護轉輪迴龍雙槍夢喚劍戰鬥風闇聖靈術彈衝範圍傷強體輕復藥賦詠禱絕對稱號標記減緩暈敵騎寵鍛煉鍊製採釣魚藝樂詩進階變詛陣與為損讓發時間銳態壞萬鈞覺動凍結';
 const simplified = '连续击闪护转轮回龙双枪梦唤剑战斗风暗圣灵术弹冲范围伤强体轻复药赋咏祷绝对称号标记减缓晕敌骑宠锻炼炼制采钓鱼艺乐诗进阶变诅阵与为损让发时间锐态坏万钧觉动冻结';
 return [...value].map(char => traditional.includes(char) ? simplified[traditional.indexOf(char)] : char).join('');
};
const options = computed(() => Object.entries({ ...skills, ...resources.value, 59005: '崩坏波动', 58014: '万钧之力' }).map(([id, name]) => ({ id: Number(id), name: normalizeSearch(name) })).filter(item => item.id > 0 && item.id <= 65535 && item.id !== 58014));
function add(item: { id: number; name: string }) { if (item.id === 58014) { emit('notice', '万钧之力不监控冷却，请在爆发提示中设置吟唱与觉醒状态。'); return; } if (!Number.isInteger(item.id) || item.id < 1 || item.id > 65535) { emit('error', '技能 ID 必须为 1–65535。'); return; } selectedId.value = item.id; if (props.disabled || props.member.skillSettings.rules.length >= 16 || props.member.skillSettings.rules.some(rule => rule.skillId === item.id)) return; props.member.skillSettings.rules.push({ skillId: item.id, name: item.name, cooldownSeconds: item.id === 59005 ? 60 : 30, sound: makeHealerSound('skill-ready') }); emit('change'); }
function remove(id: number) { props.member.skillSettings.rules = props.member.skillSettings.rules.filter(rule => rule.skillId !== id); emit('change'); }
function status(id: number) { const state = props.live?.skills?.[id]; return state?.state === 'ready' ? '就绪' : state?.state === 'cooling' ? `${state.remainingSeconds} 秒` : '未观测'; }
const preview = computed<HealerOverlayGroup>(() => healerCombinedPreview(props.member, props.fontSize, props.iconSize));
</script>
<style scoped>
header strong { font-size: 12px; } header > button { border: none; background: transparent; color: var(--ui-color-accent); } .skill-rule { font-size: 11px; } .skill-rule strong { font-size: 11px; } .skill-rule input, .skill-rule button { font-size: 11px; }
.healer-skill-section { border-top: 1px solid var(--ui-theme-border); } header, .line { display: flex; align-items: center; flex-wrap: wrap; gap: 9px 16px; } header { padding: 12px 14px; } header > button { margin-left: auto; } .detail { padding: 0 14px 14px; } .line { margin: 9px 0; } p, small, header > span { font-size: 11px; color: var(--ui-theme-muted); } p { line-height: 1.6; } label { display: inline-flex; align-items: center; gap: 5px; font-size: 12px; } input[type=number] { width: 70px; } input[type=checkbox] { accent-color: var(--ui-color-accent); } input, button { padding: 4px 6px; color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); font-size: 12px; } button { cursor: pointer; } button:disabled { opacity: .5; } .skill-rule { border: 1px solid var(--ui-theme-border); padding: 8px; margin-top: 9px; } .visual-preview { padding: 12px; margin-top: 12px; overflow: auto; border: 1px dashed var(--ui-theme-border); }
</style>
