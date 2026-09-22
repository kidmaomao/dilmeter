<template>
    <span class="healer-sound-picker">
        <label>音效
            <select :value="modelValue.kind" :aria-label="`${label}音效`" :disabled="disabled" @change="setKind">
                <option value="none">不提示</option>
                <option value="electronic">内置电子音</option>
                <option value="healer-health">晓晓：队友血量过低</option>
                <option value="healer-music">晓晓：队友音乐时间到了</option>
                <option value="healer-buff">晓晓：队友增益即将结束</option>
                <option value="voice">晓晓：音乐要结束了</option>
                <option value="skill-ready">轻快提示音</option>
                <option value="custom">自定义音效</option>
            </select>
        </label>
        <label v-if="modelValue.kind === 'custom'" class="healer-file-picker">
            <input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav" :aria-label="`${label}自定义音效`" :disabled="disabled" @change="upload" />
            <span>{{ modelValue.name || '选择 MP3 / WAV' }}</span>
        </label>
        <slot />
        <button type="button" :aria-label="`试听${label}`" :disabled="disabled || modelValue.kind === 'none' || modelValue.kind === 'custom' && !modelValue.soundId" @click="preview">试听</button>
    </span>
</template>

<script setup lang="ts">
import type { HealerSound } from '@/healerMonitorTypes';
const props = defineProps<{ modelValue: HealerSound; volume: number; label: string; disabled?: boolean }>();
const emit = defineEmits<{ 'update:modelValue': [value: HealerSound]; busy: [value: boolean]; notice: [message: string]; error: [message: string] }>();
function setKind(event: Event) { emit('update:modelValue', { ...props.modelValue, kind: (event.target as HTMLSelectElement).value }); }
async function upload(event: Event) {
    const input = event.target as HTMLInputElement, file = input.files?.[0]; input.value = '';
    if (!file) return;
    if (!/\.(mp3|wav)$/i.test(file.name) || file.size <= 0 || file.size > 10 * 1024 * 1024) { emit('error', '请选择不超过 10 MB 的 MP3 或 WAV 文件。'); return; }
    emit('busy', true);
    try {
        const body = new FormData(); body.append('file', file, file.name);
        const response = await fetch('/api/buff_sound/upload', { method: 'POST', body, signal: AbortSignal.timeout(15000) });
        if (!response.ok) throw new Error((await response.text()).trim());
        const result = await response.json() as { soundId: string; displayName: string };
        if (!result.soundId) throw new Error('未收到音效编号');
        emit('update:modelValue', { kind: 'custom', soundId: result.soundId, name: result.displayName || file.name });
        emit('notice', '音效已导入，请试听并保存设定。');
    } catch (reason) { emit('error', `导入音效失败：${String(reason)}`); }
    finally { emit('busy', false); }
}
async function preview() {
    emit('busy', true);
    try {
        const response = await fetch('/api/buff_sound', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ kind: props.modelValue.kind, soundId: props.modelValue.soundId, volume: props.volume }), signal: AbortSignal.timeout(30000) });
        if (!response.ok) throw new Error((await response.text()).trim());
        emit('notice', '已试听；若无声，请检查音量和托盘静音开关。');
    } catch (reason) { emit('error', `试听失败：${String(reason)}`); }
    finally { emit('busy', false); }
}
</script>

<style scoped>
.healer-sound-picker, label { display: inline-flex; align-items: center; flex-wrap: wrap; gap: 5px; }
select, button, .healer-file-picker { padding: 3px 6px; color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); font-size: 11px; }
button:disabled { opacity: .5; }
select { max-width: 205px; }
.healer-file-picker { position: relative; max-width: 180px; cursor: pointer; }
.healer-file-picker input { position: absolute; inset: 0; width: 100%; opacity: 0; cursor: pointer; }
.healer-file-picker span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.healer-file-picker:focus-within { outline: 2px solid var(--ui-color-accent); }
</style>
