<template>
    <div class="healer-rule-editor">
        <div class="healer-rule-identity">
            <span class="healer-condition-icon"><span>CC</span><img :src="`/condition-icons/${rule.ccId}.png`" :alt="`${rule.name}图标`" @error="($event.target as HTMLImageElement).style.display = 'none'" /></span>
            <strong v-if="builtin">{{ rule.name }}</strong><input v-else v-model="rule.name" class="healer-rule-name" maxlength="48" :aria-label="`Buff ${rule.ccId}名称`" @input="emit('change')" />
            <small>CC {{ rule.ccId }}</small>
        </div>
        <label><input v-model="rule.overlayEnabled" type="checkbox" :aria-label="`${rule.name}显示图标`" @change="emit('change')" />显示图标</label>
        <label>时长来源<select v-model="rule.durationMode" :aria-label="`${rule.name}时长来源`" @change="emit('change')"><option value="auto">自动读取</option><option value="manual">手动固定时长</option></select></label>
        <label v-if="rule.durationMode === 'manual'">固定时长（秒）<input v-model.number="rule.manualDurationSeconds" :aria-label="`${rule.name}固定时长`" type="number" min="1" max="86400" @input="emit('change')" /></label>
        <label><input v-model="rule.flashEnabled" type="checkbox" :aria-label="`${rule.name}到期前闪烁`" @change="emit('change')" />到期前闪烁</label>
        <label>闪烁提前（秒）<input v-model.number="rule.flashSeconds" :aria-label="`${rule.name}闪烁提前秒数`" type="number" min="0" max="60" @input="emit('change')" /></label>
        <HealerSoundPicker v-model="rule.sound" :label="rule.name" :volume="volume" :disabled="disabled || soundDisabled" @update:model-value="emit('change')" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)">
            <label>音效提前（秒）<input v-model.number="rule.warningSeconds" :aria-label="`${rule.name}音效提前秒数`" type="number" min="0" max="60" @input="emit('change')" /></label>
        </HealerSoundPicker>
        <label title="同一轮即将结束／已结束共用次数，包含首次">提醒次数<input v-model.number="rule.repeatCount" :aria-label="`${rule.name}提醒次数`" type="number" min="1" max="10" :disabled="disabled || soundDisabled || rule.sound.kind === 'none'" @input="emit('change')" /></label>
        <label title="最短间隔；多条声音同时触发时会排开播放">间隔（秒）<input v-model.number="rule.repeatIntervalSeconds" :aria-label="`${rule.name}提醒间隔秒数`" type="number" min="2" max="300" :disabled="disabled || soundDisabled || rule.sound.kind === 'none' || rule.repeatCount <= 1" @input="emit('change')" /></label>
        <button v-if="!builtin" type="button" class="healer-rule-remove" :aria-label="`删除Buff ${rule.name}`" @click="emit('remove')"><v-icon icon="mdi-trash-can-outline" size="13" />删除</button>
        <div class="healer-death-settings">
            <label><input v-model="rule.deathLoss.enabled" type="checkbox" :aria-label="`${rule.name}死亡丢失提醒`" @change="emit('change')" />死亡丢失提醒</label>
            <HealerSoundPicker v-model="rule.deathLoss.sound" :label="`${rule.name}死亡丢失`" :volume="volume" :disabled="disabled || soundDisabled || !rule.deathLoss.enabled" @update:model-value="emit('change')" @busy="emit('busy', $event)" @notice="emit('notice', $event)" @error="emit('error', $event)" />
            <label>提醒次数<input v-model.number="rule.deathLoss.repeatCount" :aria-label="`${rule.name}死亡丢失提醒次数`" type="number" min="1" max="10" :disabled="disabled || soundDisabled || !rule.deathLoss.enabled || rule.deathLoss.sound.kind === 'none'" @input="emit('change')" /></label>
            <label>间隔（秒）<input v-model.number="rule.deathLoss.repeatIntervalSeconds" :aria-label="`${rule.name}死亡丢失提醒间隔秒数`" type="number" min="2" max="300" :disabled="disabled || soundDisabled || !rule.deathLoss.enabled || rule.deathLoss.sound.kind === 'none' || rule.deathLoss.repeatCount <= 1" @input="emit('change')" /></label>
            <small>与正常到期独立计数；关闭后不播放死亡丢失声音，悬浮图标仍显示缺失。</small>
        </div>
        <span class="healer-rule-status">{{ status }}</span>
    </div>
</template>

<script setup lang="ts">
import HealerSoundPicker from './HealerSoundPicker.vue';
import type { HealerBuffRule } from '@/healerMonitorTypes';
defineProps<{ rule: HealerBuffRule; volume: number; status: string; builtin?: boolean; disabled?: boolean; soundDisabled?: boolean }>();
const emit = defineEmits<{ change: []; remove: []; busy: [value: boolean]; notice: [message: string]; error: [message: string] }>();
</script>

<style scoped>
.healer-rule-editor { display: flex; align-items: center; flex-wrap: wrap; gap: 7px; padding: 8px; margin-bottom: 5px; background: var(--ui-theme-raised); border: 1px solid var(--ui-theme-border); font-size: 11px; }
.healer-rule-identity { display: grid; grid-template-columns: 25px minmax(80px,1fr); gap: 1px 7px; margin-right: auto; min-width: 145px; }
.healer-condition-icon { position: relative; display: grid; place-items: center; width: 24px; height: 24px; grid-row: span 2; border: 1px solid var(--ui-theme-border); background: #151811; font-size: 8px; }
.healer-condition-icon img { position: absolute; inset: 1px; width: 20px; height: 20px; object-fit: contain; }
.healer-rule-identity small { font-size: 9px; color: var(--ui-theme-muted); }
label { display: flex; align-items: center; gap: 4px; white-space: nowrap; }
input[type=checkbox] { accent-color: var(--ui-color-accent); }
input[type=number] { width: 48px; }
input, select, button { padding: 3px 5px; border: 1px solid var(--ui-theme-border); background: var(--ui-theme-control); color: var(--ui-theme-text); font-size: 11px; }
.healer-rule-name { padding: 0; border: 0; background: transparent; width: 115px; font-weight: 600; }
.healer-rule-remove { border-color: #a34a42; }
.healer-death-settings { flex: 1 0 100%; display: flex; flex-wrap: wrap; align-items: center; gap: 7px 12px; border-top: 1px solid var(--ui-theme-border); padding-top: 8px; }
.healer-death-settings small { flex-basis: 100%; color: var(--ui-theme-muted); font-size: 10px; }
.healer-rule-status { flex: 1 0 100%; padding: 3px 6px; border-left: 2px solid #b2c091; background: var(--ui-theme-control); color: var(--ui-theme-muted); font-size: 10px; }
</style>
