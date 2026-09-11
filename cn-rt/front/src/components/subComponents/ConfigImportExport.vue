<template>
    <!-- 觸發按鈕 -->
    <v-btn
        size="small"
        variant="tonal"
        prepend-icon="mdi-swap-vertical"
        @click="open"
    >
        匯入 / 匯出
    </v-btn>

    <!-- ── Dialog ── -->
    <v-dialog v-model="dialogOpen" max-width="680" scrollable>
        <v-card>
            <v-card-title class="text-subtitle-1 py-3 px-4">
                設定 匯入 / 匯出
            </v-card-title>
            <v-divider />

            <v-card-text class="pa-4">

                <!-- ═══ 匯出 ═══ -->
                <div class="section-title mb-2">匯出</div>
                <div class="d-flex flex-wrap mb-2" style="gap: 8px;">
                    <v-checkbox
                        v-model="exportOpts.skillCC"
                        label="職業技能 CC 分析規則"
                        density="compact"
                        hide-details
                        class="flex-grow-0"
                    />
                    <v-checkbox
                        v-model="exportOpts.customCond"
                        label="自訂條件分析設定"
                        density="compact"
                        hide-details
                        class="flex-grow-0"
                    />
                </div>
                <v-textarea
                    :model-value="exportJson"
                    readonly
                    rows="8"
                    variant="outlined"
                    density="compact"
                    hide-details
                    font-family="monospace"
                    class="mb-2 export-area"
                    no-resize
                />
                <div class="d-flex" style="gap: 8px;">
                    <v-btn
                        size="small"
                        variant="tonal"
                        prepend-icon="mdi-content-copy"
                        @click="copyExport"
                    >
                        複製
                    </v-btn>
                    <v-btn
                        size="small"
                        variant="tonal"
                        prepend-icon="mdi-download"
                        @click="downloadExport"
                    >
                        下載 JSON
                    </v-btn>
                    <v-fade-transition>
                        <span v-if="copied" class="text-caption text-success align-self-center ml-1">
                            已複製！
                        </span>
                    </v-fade-transition>
                </div>

                <v-divider class="my-4" />

                <!-- ═══ 匯入 ═══ -->
                <div class="section-title mb-2">匯入</div>
                <div class="d-flex flex-wrap mb-2" style="gap: 8px;">
                    <v-checkbox
                        v-model="importOpts.skillCC"
                        label="職業技能 CC 分析規則"
                        density="compact"
                        hide-details
                        class="flex-grow-0"
                    />
                    <v-checkbox
                        v-model="importOpts.customCond"
                        label="自訂條件分析設定"
                        density="compact"
                        hide-details
                        class="flex-grow-0"
                    />
                </div>
                <v-textarea
                    v-model="importText"
                    placeholder="貼上 JSON 或使用「上傳檔案」"
                    rows="8"
                    variant="outlined"
                    density="compact"
                    hide-details
                    class="mb-2 import-area"
                    no-resize
                />
                <div class="d-flex align-center flex-wrap" style="gap: 8px;">
                    <v-btn
                        size="small"
                        variant="tonal"
                        prepend-icon="mdi-upload"
                        @click="triggerFileUpload"
                    >
                        上傳檔案
                    </v-btn>
                    <input
                        ref="fileInputRef"
                        type="file"
                        accept=".json,application/json"
                        style="display: none"
                        @change="handleFileUpload"
                    />
                    <v-btn
                        size="small"
                        color="warning"
                        variant="tonal"
                        prepend-icon="mdi-check"
                        :disabled="!importText.trim()"
                        @click="applyImport"
                    >
                        套用
                    </v-btn>
                    <span class="text-caption text-disabled">⚠ 套用將覆蓋選取的設定</span>
                </div>

                <!-- 匯入結果提示 -->
                <v-alert
                    v-if="importResult"
                    :type="importResult.ok ? 'success' : 'error'"
                    variant="tonal"
                    density="compact"
                    class="mt-3"
                    closable
                    @click:close="importResult = null"
                >
                    {{ importResult.msg }}
                </v-alert>

            </v-card-text>

            <v-divider />
            <v-card-actions class="px-4 py-2">
                <v-spacer />
                <v-btn variant="text" @click="dialogOpen = false">關閉</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script lang="ts">
import { defineComponent, ref, computed } from "vue";
import {
    type JobCCConfig,
    loadJobCCConfigs,
    saveJobCCConfigs,
} from "@/skillCCConfig";
import {
    type CustomConditionConfig,
    customConditionConfigs,
} from "@/summaryConfig";

// ── 統一格式 ─────────────────────────────────────────────────────

type UnifiedConfig = {
    version: 1;
    skillCCConfigs?: JobCCConfig[];
    customConditionConfigs?: CustomConditionConfig[];
};

export default defineComponent({
    name: "ConfigImportExport",
    setup() {
        const dialogOpen   = ref(false);
        const copied       = ref(false);
        const importText   = ref("");
        const fileInputRef = ref<HTMLInputElement | null>(null);
        const importResult = ref<{ ok: boolean; msg: string } | null>(null);

        const exportOpts = ref({ skillCC: true, customCond: true });
        const importOpts = ref({ skillCC: true, customCond: true });

        // ── 匯出 JSON ─────────────────────────────────────────────

        const exportJson = computed((): string => {
            const obj: UnifiedConfig = { version: 1 };
            if (exportOpts.value.skillCC) {
                obj.skillCCConfigs = loadJobCCConfigs();
            }
            if (exportOpts.value.customCond) {
                obj.customConditionConfigs = customConditionConfigs.value;
            }
            return JSON.stringify(obj, null, 2);
        });

        function open() {
            importText.value   = "";
            importResult.value = null;
            dialogOpen.value   = true;
        }

        async function copyExport() {
            try {
                await navigator.clipboard.writeText(exportJson.value);
                copied.value = true;
                setTimeout(() => { copied.value = false; }, 2000);
            } catch {
                /* clipboard blocked */
            }
        }

        function downloadExport() {
            const blob = new Blob([exportJson.value], { type: "application/json" });
            const url  = URL.createObjectURL(blob);
            const a    = document.createElement("a");
            a.href     = url;
            a.download = `mabidil-config-${new Date().toISOString().slice(0, 10)}.json`;
            a.click();
            URL.revokeObjectURL(url);
        }

        // ── 匯入 ─────────────────────────────────────────────────

        function triggerFileUpload() {
            fileInputRef.value?.click();
        }

        function handleFileUpload(event: Event) {
            const file = (event.target as HTMLInputElement).files?.[0];
            if (!file) return;
            const reader = new FileReader();
            reader.onload = (e) => {
                importText.value = (e.target?.result as string) ?? "";
                // 讀取完後重置 input，讓同一個檔案可以重複上傳
                if (fileInputRef.value) fileInputRef.value.value = "";
            };
            reader.readAsText(file);
        }

        function applyImport() {
            importResult.value = null;
            const text = importText.value.trim();
            if (!text) return;

            let parsed: UnifiedConfig;
            try {
                parsed = JSON.parse(text);
            } catch {
                importResult.value = { ok: false, msg: "JSON 格式錯誤，請確認內容是否正確" };
                return;
            }

            if (typeof parsed !== "object" || parsed === null) {
                importResult.value = { ok: false, msg: "無效的設定格式" };
                return;
            }

            let applied: string[] = [];

            // 套用職業技能 CC 規則
            if (importOpts.value.skillCC && Array.isArray(parsed.skillCCConfigs)) {
                try {
                    saveJobCCConfigs(parsed.skillCCConfigs as JobCCConfig[]);
                    applied.push("職業技能 CC 分析規則");
                } catch {
                    importResult.value = { ok: false, msg: "套用職業技能 CC 規則時發生錯誤" };
                    return;
                }
            }

            // 套用自訂條件設定
            if (importOpts.value.customCond && Array.isArray(parsed.customConditionConfigs)) {
                try {
                    const data = parsed.customConditionConfigs as CustomConditionConfig[];
                    customConditionConfigs.value = data;
                    localStorage.setItem("summaryConditionConfigs", JSON.stringify(data));
                    applied.push("自訂條件分析設定");
                } catch {
                    importResult.value = { ok: false, msg: "套用自訂條件時發生錯誤" };
                    return;
                }
            }

            if (applied.length === 0) {
                importResult.value = {
                    ok: false,
                    msg: "JSON 中未找到選取的設定區塊，請確認勾選項目是否符合檔案內容",
                };
                return;
            }

            importResult.value = {
                ok: true,
                msg: `已成功套用：${applied.join("、")}（重新整理頁面後完整生效）`,
            };
        }

        return {
            dialogOpen,
            exportOpts,
            importOpts,
            exportJson,
            importText,
            fileInputRef,
            importResult,
            copied,
            open,
            copyExport,
            downloadExport,
            triggerFileUpload,
            handleFileUpload,
            applyImport,
        };
    },
});
</script>

<style scoped>
.section-title {
    font-size: 0.8rem;
    font-weight: 600;
    color: rgba(0, 0, 0, 0.6);
}

:deep(.export-area textarea),
:deep(.import-area textarea) {
    font-family: "Consolas", "Courier New", monospace;
    font-size: 0.72rem;
}
</style>
