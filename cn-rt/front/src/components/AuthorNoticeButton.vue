<template>
    <button type="button" class="author-notice-button" @click="open = true">
        <v-icon icon="mdi-information-outline" size="14" />{{ t('作者说明') }}
    </button>
    <v-dialog v-model="open" max-width="540">
        <v-card>
            <v-card-title>{{ t('作者说明') }}</v-card-title>
            <v-card-text>
                <p>{{ t('使用前建议阅读作者发布的软件说明，了解功能、使用方法和更新内容。') }}</p>
                <a class="author-notice-link" href="https://www.noginogi.sbs/article/20260724" target="_blank" rel="noopener noreferrer" @click="openLink">{{ t('阅读作者说明') }}</a>
                <p>{{ t('如果上方页面无法打开，请使用备用入口：') }}</p>
                <a class="author-notice-link" href="https://cn.noginogi.sbs/article/20260724" target="_blank" rel="noopener noreferrer" @click="openLink">{{ t('备用入口（cn.noginogi.sbs）') }}</a>
                <p v-if="linkError" role="alert">{{ t(linkError) }}</p>
            </v-card-text>
            <v-card-actions><v-spacer /><v-btn @click="open = false">{{ t('关闭') }}</v-btn></v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import { openExternalLink } from '@/lib/openExternalLink';
const props = defineProps<{ translate?: (text: string) => string }>();
const open = ref(false);
const linkError = ref('');
function openLink(event: MouseEvent) {
    void openExternalLink(event, message => { linkError.value = message; });
}
const t = (text: string) => props.translate?.(text) ?? text;
</script>

<style scoped>
.author-notice-button { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; padding: 4px 8px; border: 1px solid var(--ui-theme-border); border-radius: 5px; color: var(--ui-theme-text); background: var(--ui-theme-control); font-size: 12px; }
.author-notice-link { display: inline-block; margin: 12px 0 18px; text-decoration: underline; }
</style>
