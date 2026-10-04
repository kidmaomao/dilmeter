<template>
    <section class="arcana-kpi-panel" aria-label="阿尔卡纳职业 KPI 数据">
        <header class="arcana-kpi-header">
            <strong>阿尔卡纳职业 KPI</strong>
            <span class="arcana-kpi-tag">KPI 试用</span>
            <span v-if="sample" class="arcana-kpi-tag">样例数据</span>
            <span class="arcana-kpi-clock">统计 {{ report.activeSeconds.toFixed(2) }} 秒 · 已排除无敌 {{ report.excludedSeconds.toFixed(2) }} 秒</span>
        </header>
        <p class="arcana-kpi-note">按当前角色的阿尔卡纳职业显示数据。仅统计首领可输出期间；普通等待时间保留，未记录的机制无法自动扣除。</p>
        <p v-if="!report.jobName" class="arcana-kpi-empty">尚未识别阿尔卡纳职业，识别后显示对应指标。</p>
        <p v-else-if="report.jobName === '爆裂骑士枪'" class="arcana-kpi-empty">骑士枪职业 KPI 暂未开放，后续补充。</p>
        <dl v-else class="arcana-kpi-table">
            <div v-for="item in report.rows" :key="item.id" class="arcana-kpi-row">
                <dt>{{ item.label }}</dt>
                <dd>
                    <strong v-if="item.value !== null">{{ formatValue(item.value) }}<template v-if="item.upperValue !== undefined && formatValue(item.upperValue) !== formatValue(item.value)">～{{ formatValue(item.upperValue) }}</template><small>{{ item.unit }}</small></strong>
                    <span v-else class="arcana-kpi-unavailable">{{ statusText(item.status) }}</span>
                    <span v-if="item.samples !== undefined && item.value !== null" class="arcana-kpi-samples">{{ item.samples }} 个样本</span>
                </dd>
                <p>{{ item.detail }}</p>
            </div>
        </dl>
    </section>
</template>

<script setup lang="ts">
import type { ArcanaKpiReport, KpiStatus } from "../arcanaKpi";
defineProps<{ report: ArcanaKpiReport; sample?: boolean }>();
const formatValue = (value: number) => value.toLocaleString("zh-CN", { maximumFractionDigits: 2, minimumFractionDigits: 2 });
const statusText = (status: KpiStatus) => status === "pending" ? "待确认口径" : status === "missing-data" ? "缺少数据" : "暂无有效样本";
</script>

<style scoped>
.arcana-kpi-panel { margin-top: 12px; border: 1px solid var(--ui-theme-border); background: var(--ui-theme-surface); color: var(--ui-theme-text); }
.arcana-kpi-header { display: flex; align-items: center; flex-wrap: wrap; gap: 8px; padding: 9px 11px; background: var(--ui-theme-raised); border-bottom: 1px solid var(--ui-theme-border); }
.arcana-kpi-header strong { font-size: 14px; }
.arcana-kpi-tag { border: 1px solid var(--ui-theme-border); border-radius: 3px; padding: 1px 5px; color: var(--ui-theme-muted); font-size: 11px; }
.arcana-kpi-clock { margin-left: auto; font-size: 11px; color: var(--ui-theme-muted); }
.arcana-kpi-note, .arcana-kpi-empty { margin: 0; padding: 8px 11px; color: var(--ui-theme-muted); font-size: 11px; line-height: 1.6; }
.arcana-kpi-empty { padding-bottom: 14px; }
.arcana-kpi-table { margin: 0; }
.arcana-kpi-row { display: grid; grid-template-columns: minmax(0, 1fr) auto; gap: 4px 15px; padding: 8px 11px; border-top: 1px solid var(--ui-theme-border); }
.arcana-kpi-row dt { align-self: center; font-size: 13px; }
.arcana-kpi-row dd { margin: 0; text-align: right; }
.arcana-kpi-row dd strong { font-size: 14px; color: var(--ui-color-accent); }
.arcana-kpi-row small { margin-left: 5px; font-size: 11px; font-weight: 400; }
.arcana-kpi-row p { grid-column: 1 / -1; margin: 0; font-size: 11px; line-height: 1.5; color: var(--ui-theme-muted); }
.arcana-kpi-unavailable, .arcana-kpi-samples { color: var(--ui-theme-muted); font-size: 11px; }
.arcana-kpi-samples { display: block; margin-top: 2px; }
@media (max-width: 520px) { .arcana-kpi-clock { width: 100%; margin-left: 0; } .arcana-kpi-row { gap: 4px 8px; } }
</style>
