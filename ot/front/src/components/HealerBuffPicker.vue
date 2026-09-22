<template>
 <div class="healer-buff-picker">
  <div class="healer-buff-search"><label :for="inputId">搜索 Buff</label><input :id="inputId" v-model="query" type="search" placeholder="名称或 CC ID，如战争序曲、锐利" @keyup.enter="addFirst" /><button type="button" :disabled="!available || rules.length >= 16" @click="addFirst">＋ 添加</button></div>
  <div class="healer-buff-results">
   <button v-for="item in options" :key="item.id" type="button" class="healer-search-card" :disabled="configured(item.id) || rules.length >= 16" :aria-label="`添加 ${item.name}`" @click="emit('add', makeHealerRule(item.id, item.name))">
    <img :src="`/condition-icons/${item.id}.png`" alt="" @error="($event.target as HTMLImageElement).style.visibility = 'hidden'" /><span><strong>{{ item.name }}</strong><small>CC {{ item.id }}</small></span><b>{{ configured(item.id) ? '✓' : '＋' }}</b>
   </button>
   <span v-if="query && !options.length" class="healer-muted">没有匹配项，可输入 CC ID 添加。</span>
  </div>
 </div>
</template>
<script setup lang="ts">
import { computed, inject, ref, type Ref } from 'vue';
import { conditions } from '@/data/condition';
import { makeHealerRule, type HealerBuffRule } from '@/healerMonitorTypes';
const props = defineProps<{ rules: HealerBuffRule[]; inputId: string }>();
const emit = defineEmits<{ add: [rule: HealerBuffRule] }>();
const query = ref('');
const resources = inject<Ref<Record<number, string>>>('condNameMap', ref({}));
const names = computed<Record<string, string>>(() => ({ ...conditions, ...resources.value, 511: '状态支援：锐利', 1004: '锐利之眼', 680: '战争序曲', 192: '活跃进行曲' }));
function searchText(text: string) {
 const traditional = '連續擊閃護轉輪迴龍雙槍夢喚劍戰鬥風闇聖靈術彈衝範圍傷強體輕復藥賦詠禱絕對稱號標記減緩暈敵騎寵鍛煉鍊製採釣魚藝樂詩進階變詛陣與為損讓發時間銳態潑躍';
 const simplified = '连续击闪护转轮回龙双枪梦唤剑战斗风暗圣灵术弹冲范围伤强体轻复药赋咏祷绝对称号标记减缓晕敌骑宠锻炼炼制采钓鱼艺乐诗进阶变诅阵与为损让发时间锐态泼跃';
 return [...text.trim().toLowerCase()].map(char => traditional.includes(char) ? simplified[traditional.indexOf(char)] : char).join('');
}
const configured = (id: number) => props.rules.some(rule => rule.ccId === id);
const options = computed(() => {
 const text = searchText(query.value);
 if (!text) return [680, 192].map(id => ({ id, name: names.value[id] }));
 if (/^\d+$/.test(text)) { const id = Number(text); return Number.isSafeInteger(id) && id <= 4294967295 ? [{ id, name: searchText(names.value[id] || `Buff ${id}`) }] : []; }
 return Object.entries(names.value).filter(([, name]) => searchText(name).includes(text)).slice(0, 20).map(([id, name]) => ({ id: Number(id), name: searchText(name) }));
});
const available = computed(() => options.value.find(option => !configured(option.id)));
function addFirst() { if (available.value && props.rules.length < 16) emit('add', makeHealerRule(available.value.id, available.value.name)); }
</script>
<style scoped>
.healer-buff-search { display: flex; align-items: center; gap: 8px; font-size: 12px; }
.healer-buff-search input { flex: 1; min-width: 80px; }
input, button { color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); padding: 5px 8px; font-size: 12px; }
.healer-buff-results { display: flex; gap: 6px; flex-wrap: wrap; margin: 8px 0 12px; }
.healer-search-card { display: flex; align-items: center; gap: 7px; text-align: left; width: 215px; }
.healer-search-card img { width: 24px; height: 24px; border: 1px solid var(--ui-theme-border); }
.healer-search-card span { flex: 1; display: grid; }
.healer-search-card strong { font-size: 11px; font-weight: 600; }
.healer-search-card small { font-size: 9px; }
.healer-search-card:disabled { opacity: .5; cursor: default; }
.healer-muted { color: var(--ui-theme-muted); font-size: 12px; }
</style>
