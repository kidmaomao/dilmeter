<template>
    <v-dialog :model-value="modelValue" max-width="660" :persistent="pending" @update:model-value="emit('update:modelValue', $event)">
        <v-card class="speech-maker">
            <v-card-title>{{ t('提醒语音制作') }}</v-card-title>
            <v-card-text>
                <p>{{ t('应用目标：') }}{{ targetLabel }}</p>
                <label class="speech-field">{{ t('语音来源') }}
                    <select v-model="provider" :disabled="pending" @change="loadProvider">
                        <option value="edge">{{ t('Edge 自然语音 · 晓晓（联网生成）') }}</option>
                        <option value="local">{{ t('Windows 本地语音（离线生成）') }}</option>
                    </select>
                </label>
                <template v-if="provider === 'edge'">
                    <ol v-if="!edgeReady" class="speech-help">
                        <li>{{ t('请下载安装包：请至') }}
                            <a href="https://gear.noginogi.sbs/downloads/" target="_blank" rel="noopener noreferrer" @click="openLink">{{ t('gear 本站') }}</a>
                            / <a :href="mirrorUrl" target="_blank" rel="noopener noreferrer" @click="openLink">{{ t('备用链接') }}</a>
                            {{ t('下载晓晓语音包裹。') }}
                        </li>
                        <li>{{ t('解压至当前 DPS 软件根目录。') }}</li>
                        <li>{{ t('解压后点击“自动监测”按钮。') }}</li>
                    </ol>
                    <p v-if="linkError" class="speech-error" role="alert">{{ t(linkError) }}</p>
                    <div class="speech-dependency" role="status" aria-live="polite">
                        <span :class="{ 'speech-ready': edgeReady, 'speech-error': !loading && !edgeReady }">
                            <b aria-hidden="true">{{ edgeReady || addonFound ? '✓' : loading ? '…' : '×' }}</b> {{ t(dependencyMessage) }}
                        </span>
                        <button type="button" :disabled="loading || pending" @click="loadProvider">{{ t(loading ? '检测中…' : '自动监测') }}</button>
                    </div>
                    <div v-if="!loading && !edgeReady" class="speech-location">
                        <p>{{ t('是否已解压至这个文件夹？') }}</p>
                        <code>{{ applicationDir || t('当前 DPS 软件所在文件夹') }}</code>
                        <img src="/images/edge-tts-folder-layout.svg" :alt="t('同一文件夹中放置 Dilmeter.exe、data 文件夹和 addons 文件夹')" />
                    </div>
                    <small>{{ t('联网生成时，文字会发送给 Microsoft；保存后可离线播放。') }}</small>
                </template>
                <label v-else class="speech-field">{{ t('系统语音') }}
                    <select v-model="voice" :disabled="loading || pending">
                        <option v-if="!voices.length" value="">{{ t(loading ? '正在读取…' : '未找到系统语音') }}</option>
                        <option v-for="item in voices" :key="item.name" :value="item.name">{{ item.description || item.name }}（{{ item.culture }}）</option>
                    </select>
                </label>
                <label class="speech-field">{{ t('提醒文字') }}
                    <textarea v-model="text" :disabled="pending" maxlength="120" rows="3" />
                    <small>{{ Array.from(text).length }}/120</small>
                </label>
                <label class="speech-field">{{ t('语速（-5 慢，0 正常，5 快）') }}
                    <input v-model.number="rate" :disabled="pending" type="number" min="-5" max="5" step="1" />
                </label>
                <p v-if="status" role="status" :class="{ 'speech-error': failed }">{{ t(status) }}</p>
            </v-card-text>
            <v-card-actions>
                <v-spacer />
                <v-btn :disabled="pending" @click="emit('update:modelValue', false)">{{ t('关闭') }}</v-btn>
                <v-btn color="primary" :loading="pending" :disabled="!canGenerate" @click="generate">{{ t('生成、选用并试听') }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { openExternalLink } from '@/lib/openExternalLink';

interface SpeechResult { soundId: string; displayName: string; }
interface Voice { name: string; description: string; culture: string; }
const props = defineProps<{ modelValue: boolean; targetLabel: string; initialText: string; translate?: (text: string) => string }>();
const emit = defineEmits<{ 'update:modelValue': [value: boolean]; generated: [result: SpeechResult] }>();
const t = (value: string) => props.translate?.(value) ?? value;
const provider = ref<'edge' | 'local'>('edge');
const text = ref('');
const rate = ref(0);
const voice = ref('');
const voices = ref<Voice[]>([]);
const loading = ref(false);
const pending = ref(false);
const edgeReady = ref(false);
const dependencyMessage = ref('');
const applicationDir = ref('');
const addonFound = ref(false);
const mirrorUrl = 'https://noginogi-downloads.netlify.app/downloads/';
const linkError = ref('');
function openLink(event: MouseEvent) {
    void openExternalLink(event, message => { linkError.value = message; });
}
const status = ref('');
const failed = ref(false);
let loadGeneration = 0;
const canGenerate = computed(() => !loading.value && !pending.value && text.value.trim().length > 0
    && Number.isInteger(rate.value) && rate.value >= -5 && rate.value <= 5
    && (provider.value === 'edge' ? edgeReady.value : Boolean(voice.value)));

watch(() => props.modelValue, (open) => {
    if (!open) return;
    text.value = props.initialText;
    rate.value = 0;
    status.value = '';
    failed.value = false;
    void loadProvider();
}, { immediate: true });

async function loadProvider() {
    const generation = ++loadGeneration;
    const selected = provider.value;
    loading.value = true;
    edgeReady.value = false;
    addonFound.value = false;
    dependencyMessage.value = '正在自动监测依赖…';
    status.value = '';
    failed.value = false;
    try {
        if (selected === 'edge') {
            const locationResponse = await fetch('/api/edge_tts?check=location', { cache: 'no-store', signal: AbortSignal.timeout(5000) });
            if (!locationResponse.ok) throw new Error((await locationResponse.text()).trim());
            const location = await locationResponse.json();
            if (generation !== loadGeneration) return;
            applicationDir.value = location.applicationDir || '';
            addonFound.value = location.addonPresent === true;
            if (addonFound.value) dependencyMessage.value = '已找到语音包，正在检查安装…';
        }
        const response = await fetch(selected === 'edge' ? '/api/edge_tts' : '/api/local_tts', { cache: 'no-store', signal: AbortSignal.timeout(15000) });
        if (!response.ok) throw new Error((await response.text()).trim());
        const result = await response.json();
        if (generation !== loadGeneration) return;
        if (selected === 'edge') {
            edgeReady.value = result.ready === true;
            addonFound.value = edgeReady.value;
            applicationDir.value = result.applicationDir || applicationDir.value;
            dependencyMessage.value = result.message || '依赖检测未返回结果，请重试';
        } else {
            voices.value = Array.isArray(result.voices) ? result.voices : [];
            voice.value = (voices.value.find(item => item.culture.toLowerCase().startsWith('zh')) ?? voices.value[0])?.name || '';
        }
    } catch (error) {
        if (generation !== loadGeneration) return;
        failed.value = true;
        addonFound.value = false;
        dependencyMessage.value = '自动监测失败，请重试';
        status.value = `读取语音依赖失败：${String(error)}`;
    } finally {
        if (generation === loadGeneration) loading.value = false;
    }
}

async function generate() {
    if (!canGenerate.value) return;
    pending.value = true;
    failed.value = false;
    status.value = provider.value === 'edge' ? '正在联网生成晓晓语音…' : '正在生成 Windows 本地语音…';
    try {
        const response = await fetch(provider.value === 'edge' ? '/api/edge_tts' : '/api/local_tts', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json', 'X-Dilmeter-Action': 'generate-edge-tts' },
            body: JSON.stringify({ text: text.value, rate: rate.value, voice: voice.value }),
            signal: AbortSignal.timeout(65000),
        });
        if (!response.ok) throw new Error((await response.text()).trim());
        const result = await response.json() as SpeechResult;
        if (!result.soundId) throw new Error('生成结果缺少音效编号');
        emit('generated', result);
        status.value = '语音已生成并选用，请试听后保存提醒设定。';
    } catch (error) {
        failed.value = true;
        status.value = `生成失败：${String(error)}`;
    } finally {
        pending.value = false;
    }
}
</script>

<style scoped>
.speech-maker { color: var(--ui-theme-text); background: var(--ui-theme-panel); }
.speech-maker p { margin: 10px 0; line-height: 1.6; }
.speech-field { display: flex; flex-direction: column; gap: 5px; margin: 14px 0; }
.speech-field select, .speech-field textarea, .speech-field input, .speech-dependency button { border: 1px solid var(--ui-theme-border); border-radius: 5px; padding: 7px 9px; color: inherit; background: var(--ui-theme-control); }
.speech-field input { width: 100px; }
.speech-field textarea { resize: vertical; }
.speech-help { padding: 8px 10px 8px 30px; border: 1px solid var(--ui-theme-border); border-radius: 6px; font-size: 13px; }
.speech-help a { color: var(--ui-theme-accent, #72b6ed); text-decoration: underline; }
.speech-location code { display: block; overflow-wrap: anywhere; user-select: text; }
.speech-location img { display: block; width: 100%; max-width: 420px; margin: 8px 0 12px; }
.speech-ready { color: #278a52; }
.speech-help summary { cursor: pointer; font-weight: 600; }
.speech-help ol { padding-left: 24px; }
.speech-help li { margin: 9px 0; }
.speech-help code, .speech-help small { display: block; overflow-wrap: anywhere; margin: 5px 0; }
.speech-help code { padding: 5px; background: var(--ui-theme-control); user-select: text; }
.speech-dependency { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; margin: 12px 0; font-size: 13px; }
.speech-error { color: #d34444; }
</style>
