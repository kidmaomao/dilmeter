<template>
    <v-dialog v-model="open" max-width="590" :persistent="cleaning">
        <v-card class="log-cleanup-card">
            <template v-if="result">
                <v-card-title class="log-cleanup-title">
                    <v-icon icon="mdi-delete-restore" color="success" />
                    历史日志已移入回收站
                </v-card-title>
                <v-card-text>
                    <v-alert type="success" variant="tonal" density="comfortable">
                        已将 {{ result.movedCount }} 个日志文件（{{ formatBytes(result.movedBytes) }}）移入 Windows 回收站。
                    </v-alert>
                    <p class="log-cleanup-recycle-note">
                        <v-icon icon="mdi-alert-circle-outline" size="18" />
                        请手动清空 Windows 回收站，才能真正释放这些磁盘空间。
                    </p>
                </v-card-text>
                <v-card-actions>
                    <v-spacer />
                    <v-btn color="primary" @click="open = false">知道了</v-btn>
                </v-card-actions>
            </template>

            <template v-else>
                <v-card-title class="log-cleanup-title">
                    <v-icon icon="mdi-file-clock-outline" color="warning" />
                    是否清理历史日志？
                </v-card-title>
                <v-card-text v-if="status">
                    <p class="log-cleanup-reason">{{ promptReason }}</p>

                    <div class="log-cleanup-summary">
                        <span><b>{{ status.totalCount }}</b> 个日志文件</span>
                        <span>共 <b>{{ formatBytes(status.totalBytes) }}</b></span>
                        <span v-if="status.oldestModifiedAt">最早：<b>{{ formatDateTime(status.oldestModifiedAt) }}</b></span>
                    </div>

                    <label class="log-cleanup-date-label" for="log-cleanup-before">清理此日期之前的日志</label>
                    <input
                        id="log-cleanup-before"
                        v-model="beforeDate"
                        class="log-cleanup-date-input"
                        type="date"
                        :max="today"
                        :disabled="cleaning"
                        @change="refreshPreview"
                    >
                    <p class="log-cleanup-date-help">所选日期当天及之后的日志会保留，本次运行正在写入的日志也不会处理。</p>

                    <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-3">
                        {{ error }}
                    </v-alert>
                    <v-alert v-if="confirming" type="warning" variant="tonal" density="compact" class="mb-3">
                        请确认：将 {{ status.eligibleCount }} 个日志文件（{{ formatBytes(status.eligibleBytes) }}）移入 Windows 回收站？
                    </v-alert>
                    <div v-else class="log-cleanup-preview" aria-live="polite">
                        <v-progress-circular v-if="previewing" indeterminate :size="18" :width="2" />
                        <template v-else>
                            将处理 <b>{{ status.eligibleCount }}</b> 个文件，共 <b>{{ formatBytes(status.eligibleBytes) }}</b>。
                        </template>
                    </div>

                    <p class="log-cleanup-recycle-note">
                        <v-icon icon="mdi-delete-restore" size="18" />
                        文件只会移入回收站，不会直接永久删除；之后仍需由你手动清空回收站。
                    </p>
                </v-card-text>
                <v-card-actions>
                    <v-btn :disabled="cleaning" @click="closeDialog">暂不清理</v-btn>
                    <v-spacer />
                    <v-btn v-if="confirming" :disabled="cleaning" @click="confirming = false">返回修改</v-btn>
                    <v-btn
                        v-if="confirming"
                        color="error"
                        :loading="cleaning"
                        @click="cleanLogs"
                    >
                        确认移入回收站
                    </v-btn>
                    <v-btn
                        v-else
                        color="primary"
                        :disabled="previewing || !status || status.eligibleCount === 0"
                        @click="confirming = true"
                    >
                        清理这些日志
                    </v-btn>
                </v-card-actions>
            </template>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from "vue";

interface LogCleanupStatus {
    totalCount: number;
    totalBytes: number;
    oldestModifiedAt?: string;
    newestModifiedAt?: string;
    sizeThresholdBytes: number;
    ageThresholdMonths: number;
    exceedsSize: boolean;
    exceedsAge: boolean;
    shouldPrompt: boolean;
    defaultBeforeDate: string;
    beforeDate: string;
    eligibleCount: number;
    eligibleBytes: number;
}

interface LogCleanupResult {
    movedCount: number;
    movedBytes: number;
    beforeDate: string;
    message: string;
}

const open = ref(false);
const status = ref<LogCleanupStatus | null>(null);
const result = ref<LogCleanupResult | null>(null);
const beforeDate = ref("");
const previewing = ref(false);
const cleaning = ref(false);
const confirming = ref(false);
const error = ref("");

const localDateString = (date: Date) => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, "0");
    const day = String(date.getDate()).padStart(2, "0");
    return `${year}-${month}-${day}`;
};

const today = localDateString(new Date());

const promptReason = computed(() => {
    if (!status.value) return "";
    if (status.value.exceedsSize && status.value.exceedsAge) {
        return "日志总大小已经超过 10 GB，并且包含两个月以前的历史日志。";
    }
    if (status.value.exceedsSize) return "日志总大小已经超过 10 GB。";
    return "检测到两个月以前的历史日志。";
});

const formatBytes = (value: number) => {
    if (!Number.isFinite(value) || value <= 0) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
    const digits = index >= 3 ? 2 : index >= 2 ? 1 : 0;
    return `${(value / 1024 ** index).toFixed(digits)} ${units[index]}`;
};

const formatDateTime = (value: string) => {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return value;
    return new Intl.DateTimeFormat("zh-CN", {
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
    }).format(date);
};

const responseError = async (response: Response) => {
    try {
        const data = await response.json() as { error?: string };
        return data.error || `请求失败（${response.status}）`;
    } catch {
        return `请求失败（${response.status}）`;
    }
};

const loadStatus = async (showOnlyWhenNeeded: boolean) => {
    const query = beforeDate.value ? `?before=${encodeURIComponent(beforeDate.value)}` : "";
    const response = await fetch(`/api/log_cleanup${query}`, { cache: "no-store" });
    if (!response.ok) throw new Error(await responseError(response));
    const next = await response.json() as LogCleanupStatus;
    status.value = next;
    if (!beforeDate.value) beforeDate.value = next.defaultBeforeDate;
    if (showOnlyWhenNeeded && next.shouldPrompt) open.value = true;
};

const refreshPreview = async () => {
    confirming.value = false;
    error.value = "";
    if (!beforeDate.value) return;
    previewing.value = true;
    try {
        await loadStatus(false);
    } catch (reason) {
        error.value = reason instanceof Error ? reason.message : String(reason);
    } finally {
        previewing.value = false;
    }
};

const closeDialog = () => {
    confirming.value = false;
    open.value = false;
};

const cleanLogs = async () => {
    if (!status.value || status.value.eligibleCount === 0 || !beforeDate.value) return;
    cleaning.value = true;
    error.value = "";
    try {
        const response = await fetch("/api/log_cleanup", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ before: beforeDate.value }),
        });
        if (!response.ok) throw new Error(await responseError(response));
        result.value = await response.json() as LogCleanupResult;
    } catch (reason) {
        error.value = reason instanceof Error ? reason.message : String(reason);
        confirming.value = false;
    } finally {
        cleaning.value = false;
    }
};

onMounted(() => {
    void loadStatus(true).catch(() => {
        // 日志提示属于辅助功能，检查失败不应阻止主界面启动。
    });
});
</script>

<style scoped>
.log-cleanup-card {
    border: 1px solid rgba(var(--v-theme-warning), 0.35);
}

.log-cleanup-title {
    display: flex;
    align-items: center;
    gap: 10px;
}

.log-cleanup-reason {
    margin: 0 0 14px;
    line-height: 1.6;
}

.log-cleanup-summary {
    display: flex;
    flex-wrap: wrap;
    gap: 8px 18px;
    margin-bottom: 18px;
    padding: 12px 14px;
    border-radius: 8px;
    background: rgba(var(--v-theme-on-surface), 0.055);
    font-size: 13px;
}

.log-cleanup-date-label {
    display: block;
    margin-bottom: 6px;
    font-size: 13px;
    font-weight: 700;
}

.log-cleanup-date-input {
    width: min(100%, 260px);
    padding: 9px 11px;
    border: 1px solid rgba(var(--v-theme-on-surface), 0.28);
    border-radius: 6px;
    background: rgb(var(--v-theme-surface));
    color: rgb(var(--v-theme-on-surface));
    color-scheme: light dark;
}

.log-cleanup-date-help {
    margin: 7px 0 13px;
    color: rgba(var(--v-theme-on-surface), 0.68);
    font-size: 12px;
    line-height: 1.55;
}

.log-cleanup-preview {
    display: flex;
    align-items: center;
    min-height: 38px;
    gap: 8px;
    margin-bottom: 8px;
    padding: 9px 12px;
    border-radius: 6px;
    background: rgba(var(--v-theme-primary), 0.09);
    font-size: 13px;
}

.log-cleanup-recycle-note {
    display: flex;
    align-items: flex-start;
    gap: 7px;
    margin: 14px 0 0;
    color: rgba(var(--v-theme-on-surface), 0.76);
    font-size: 12px;
    line-height: 1.55;
}
</style>
