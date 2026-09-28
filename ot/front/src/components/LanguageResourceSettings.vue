<template>
    <label class="locale-setting">
        <span>{{ $ui("语言") }}</span>
        <select :value="uiLocale" aria-label="界面语言 / 介面語言" @change="changeLocale">
            <option value="zh-CN">简体中文</option>
            <option value="zh-TW">繁體中文</option>
        </select>
    </label>
    <button class="resource-settings-button" type="button" @click="open = true">
        {{ $ui("资料") }}：{{ resourceRegion.toUpperCase() }}
    </button>
    <v-dialog v-model="open" max-width="520">
        <v-card>
            <v-card-title>{{ $ui("游戏资料") }}</v-card-title>
            <v-card-text>
                <p>{{ $ui("选择技能、状态、才能与怪物名称所用的服务器资料。此选项不会更改抓包服务器地址。") }}</p>
                <v-select v-model="selected" :items="$uiItems(options)" :label="$ui('资料服务器')" :disabled="pending" class="mt-4" />
                <p>{{ $ui("TW 资料来自 Prilus，首次载入需要联网，成功后可离线使用缓存。") }}</p>
                <a href="https://prilus.gitlab.io/" target="_blank" rel="noopener noreferrer">Prilus Mabi Tool</a>
                <v-alert v-if="message" :type="failed ? 'error' : 'info'" class="mt-3" density="compact">{{ $ui(message) }}</v-alert>
                <p v-if="version" class="mt-2">{{ $ui("资料版本") }}：{{ new Date(version * 1000).toLocaleString(uiLocale) }}</p>
            </v-card-text>
            <v-card-actions>
                <v-btn :disabled="pending" @click="apply(true)">{{ $ui("重新下载") }}</v-btn>
                <v-spacer />
                <v-btn :disabled="pending" @click="open = false">{{ $ui("关闭") }}</v-btn>
                <v-btn color="primary" :loading="pending" @click="apply(false)">{{ $ui("载入资料") }}</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>
<script setup lang="ts">
import { computed, ref, watch } from "vue";
import { uiLocale, uiText, setUiLocale, resourceRegion, type UiLocale, type ResourceRegion } from "@/uiLocale";
import { loadResourceNames, resourceNamesLoading as pending } from "@/resourceNames";

const open = ref(false);
const emit = defineEmits<{ loaded: [] }>();
const selected = ref<ResourceRegion>(resourceRegion.value);
const message = ref("");
const failed = ref(false);
const version = ref(0);
const options = computed(() => [
    { title: uiText("CN 国服（内置／外置资源包）"), value: "cn" },
    { title: uiText("TW 台服（Prilus）"), value: "tw" },
]);
watch(open, value => { if (value) selected.value = resourceRegion.value; });
function changeLocale(event: Event) {
    setUiLocale((event.target as HTMLSelectElement).value as UiLocale);
}
async function apply(refresh: boolean) {
    if (pending.value) return;
    failed.value = false;
    message.value = "正在载入资料…";
    try {
        const result = await loadResourceNames(selected.value, refresh);
        emit("loaded");
        version.value = result.version;
        message.value = result.cached ? "资料更新失败，已载入此服务器的本地缓存。" : "资料已载入，选择已保存。";
        message.value += ` 技能 ${result.counts.skills} 项、状态 ${result.counts.conditions} 项、${selected.value === 'tw' ? '秘法才能' : '阿尔卡纳'} ${result.counts.multiClasses} 项。`;
    } catch (error) {
        failed.value = true;
        message.value = `载入失败，仍使用原有资料：${error instanceof Error ? error.message : error}`;
    }
}
</script>
<style scoped>
.locale-setting { display: inline-flex; align-items: center; gap: 4px; margin-right: 8px; font-size: 12px; }
.locale-setting select, .resource-settings-button { color: inherit; background: var(--ui-theme-control, #222); border: 1px solid var(--ui-theme-border, #555); border-radius: 4px; padding: 4px 6px; font-size: 12px; }
.resource-settings-button { margin-right: 8px; white-space: nowrap; }
p { line-height: 1.65; }
</style>
