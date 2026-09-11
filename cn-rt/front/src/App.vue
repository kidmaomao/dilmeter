<template>
    <v-app v-if="isBuffOverlay" class="buff-overlay-app">
        <BuffOverlay />
    </v-app>
    <v-app v-else-if="isDebuffOverlay" class="debuff-overlay-app">
        <DebuffOverlay />
    </v-app>
    <v-app v-else-if="isSkillOverlay" class="skill-overlay-app">
        <SkillCooldownOverlay />
    </v-app>
    <v-app v-else class="main-dilmeter-app" :style="uiColorThemeVars">
        <v-main>
            <transition name="foreground-recovery-fade">
                <div
                    v-if="foregroundRecoveryActive && !isDesignPreview"
                    class="foreground-recovery-overlay"
                    role="status"
                    aria-live="polite"
                    aria-busy="true"
                >
                    <div class="foreground-recovery-card">
                        <v-progress-circular indeterminate color="primary" :size="58" :width="5" />
                        <strong>正在加载数据</strong>
                        <span>{{ fileLoadMessage || "正在追赶后台数据..." }}</span>
                        <small>{{ foregroundRecoveryDetail }}</small>
                        <v-progress-linear :model-value="fileLoadProgress" color="primary" height="7" rounded />
                    </div>
                </div>
            </transition>
            <v-toolbar v-if="!isDesignPreview" density="compact" class="px-2 app-toolbar">
                <v-toolbar-title class="text-body-1">
                    {{ appName }}
                    <span class="app-version">v{{ appVersion }}</span>
                    <v-chip class="ml-2" size="x-small" :color="statusColor" variant="tonal">
                        {{ statusLabel }}
                    </v-chip>
                </v-toolbar-title>
                <v-spacer />
                <button v-if="!isStandalone" type="button" class="skill-bar-settings-button" @click="skillBarSettingsOpen = true">
                    <v-icon icon="mdi-view-grid-plus-outline" size="14" />技能栏
                </button>
                <button v-if="!isStandalone" type="button" class="update-check-button" :class="{ available: updateInfo?.available }" :disabled="updatePending" @click="checkForUpdate(true)">
                    <v-icon :icon="updateInfo?.available ? 'mdi-download-circle' : 'mdi-update'" size="14" />
                    {{ updateInfo?.available ? `发现 v${updateInfo.latestVersion}` : "检查更新" }}
                </button>
                <label v-if="!isStandalone" class="ui-color-picker" title="参照洛奇界面颜色预设，选择会立即保存">
                    <span class="ui-color-swatch" aria-hidden="true" />
                    <span>UI</span>
                    <select v-model="uiColorTheme" aria-label="选择 UI 颜色" @change="updateUiColorTheme">
                        <option v-for="theme in uiColorThemes" :key="theme.id" :value="theme.id">{{ theme.name }}</option>
                    </select>
                </label>
                <label v-if="!isStandalone" class="dps-recording-switch" title="关闭后只隐藏 DPS 界面；网络数据仍会照常处理，Buff、Debuff 和技能 CD 提醒不受影响">
                    <input v-model="dpsMonitoringEnabled" type="checkbox" @change="updateDpsMonitoring" />
                    <span>显示 DPS</span>
                </label>
                <label v-if="!isStandalone" class="accelerator-switch" title="让程序同时检测加速器、本地代理和虚拟网卡连接">
                    <input
                        v-model="acceleratorMode"
                        type="checkbox"
                        :disabled="serverSettingPending"
                        @change="updateGameServer"
                    >
                    <span>加速器兼容</span>
                </label>
                <label v-if="!isStandalone" class="server-switcher">
                    <span>服务器</span>
                    <select
                        v-model="selectedGameServer"
                        :disabled="serverSettingPending || gameServerOptions.length === 0"
                        aria-label="选择洛奇 CN 服务器"
                        @change="updateGameServer"
                    >
                        <option v-for="option in gameServerOptions" :key="option.id" :value="option.id">
                            {{ option.name }}
                        </option>
                    </select>
                </label>
            </v-toolbar>

            <v-alert
                v-if="!isStandalone && !isDesignPreview"
                :type="isRecordReplay ? 'info' : runtimeStatus.state === 'error' ? 'error' : runtimeStatus.capturing ? 'success' : 'info'"
                variant="tonal"
                density="compact"
                class="ma-2 mb-0"
                :icon="isRecordReplay ? 'mdi-history' : runtimeStatus.capturing ? 'mdi-radar' : 'mdi-gamepad-variant-outline'"
            >
                {{ isRecordReplay ? `正在查看历史记录${loadedRecordName ? `：${loadedRecordName}` : ""}，实时数据暂不写入当前页面。` : runtimeStatus.message }}
                <span v-if="!isRecordReplay && runtimeStatus.interface" class="text-caption ml-2">{{ runtimeStatus.interface }}</span>
            </v-alert>

            <v-sheet v-if="isFileLoading && !isDesignPreview" class="d-flex align-center pa-2" style="gap: 10px">
                <v-icon icon="mdi-file-import-outline" size="small" />
                <span class="text-caption">{{ fileLoadMessage }}</span>
                <v-progress-linear :model-value="fileLoadProgress" color="primary" height="8" rounded />
                <span class="text-caption">{{ fileLoadProgress }}%</span>
            </v-sheet>

            <v-alert v-if="loadError && !isDesignPreview" type="error" variant="tonal" density="compact" class="ma-2" closable @click:close="loadError = ''">
                {{ loadError }}
            </v-alert>

            <GameDpsReport
                :is-file-loading="isFileLoading"
                :is-standalone="isStandalone"
                :dps-visible="dpsMonitoringEnabled"
                @refresh-info="loadFromServer"
                @view-records="recordBrowserOpen = true"
                @clear-data="clearData"
            >
                <template #reminder-settings-actions="{ dirty: reminderRulesDirty }">
                    <div v-if="!isStandalone" class="reminder-inline-settings" aria-label="桌面 Buff 悬浮设置">
                        <label class="reminder-overlay-checkbox" title="独立控制 Buff 与技能覆盖层；后台计时和统计不会停止">
                            <input v-model="appBuffSettings.overlayEnabled" type="checkbox" @change="previewOverlayAppearance" />
                            显示 Buff/技能
                        </label>
                        <label class="reminder-overlay-checkbox" title="锁定时 Buff、Debuff、技能 CD、瞄准、Boss 机制和层数提示均为鼠标穿透；解锁后可直接拖动">
                            <input v-model="appBuffSettings.locked" type="checkbox" @change="persistOverlayLock" />
                            锁定覆盖层
                        </label>
                        <label class="reminder-opacity-setting" title="同时调整 Buff、Debuff、技能图标与倒计时的透明度">
                            <span>透明度</span>
                            <input
                                v-model.number="appBuffSettings.opacity"
                                type="range"
                                min="20"
                                max="100"
                                step="5"
                                @input="previewOverlayAppearance"
                            />
                            <em>{{ appBuffSettings.opacity }}%</em>
                        </label>
                        <label class="reminder-dpi-setting" title="自动会读取原生悬浮窗当前显示器的缩放；手动档应与 Windows 显示缩放一致">
                            <span>DPI</span>
                            <select v-model.number="appBuffSettings.dpiPercent" @change="markAppBuffSettingsDirty">
                                <option :value="0">自动</option>
                                <option v-for="percent in [100, 125, 150, 175, 200, 225, 250, 300]" :key="percent" :value="percent">
                                    {{ percent }}%
                                </option>
                            </select>
                        </label>
                        <label title="桌面 Buff 图标的宽高">
                            <span>图标</span>
                            <input
                                v-model.number="appBuffSettings.iconSize"
                                type="number"
                                min="16"
                                max="80"
                                @input="markAppBuffSettingsDirty"
                            />
                            <em>px</em>
                        </label>
                        <label title="Buff 到期提醒的统一音量">
                            <span>音量</span>
                            <input
                                v-model.number="appBuffSettings.volume"
                                type="number"
                                min="0"
                                max="100"
                                @input="markAppBuffSettingsDirty"
                            />
                            <em>%</em>
                        </label>
                        <label class="reminder-coordinate-setting" title="以主屏幕左上角为 (0, 0)，输入后悬浮图标会立即移动">
                            <span>坐标</span>
                            <b>X</b>
                            <input v-model.number="buffOverlayX" type="number" min="-32000" max="32000" @input="markAppBuffSettingsDirty(); applyBuffOverlayPosition(false)" />
                            <b>Y</b>
                            <input v-model.number="buffOverlayY" type="number" min="-32000" max="32000" @input="markAppBuffSettingsDirty(); applyBuffOverlayPosition(false)" />
                        </label>
                        <button
                            type="button"
                            class="reminder-save-settings"
                            :class="{ 'needs-save': reminderRulesDirty || appBuffSettingsDirty, saved: settingsSaved }"
                            @click="saveAppBuffSettings(true)"
                        >
                            <v-icon :icon="settingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" size="13" />
                            {{ settingsSaved ? "已保存" : "保存设定" }}
                        </button>
                    </div>
                </template>
                <template #debuff-reminder-settings-actions>
                    <div v-if="!isStandalone" class="reminder-inline-settings debuff-inline-settings" aria-label="Boss Debuff 悬浮设置">
                        <label class="reminder-overlay-checkbox" title="独立控制 Boss Debuff 覆盖层">
                            <input v-model="appDebuffSettings.overlayEnabled" type="checkbox" @change="persistDebuffOverlayVisibility" />
                            显示 Debuff
                        </label>
                        <label title="Boss Debuff 提醒图标的宽高">
                            <span>图标</span>
                            <input v-model.number="appDebuffSettings.iconSize" type="number" min="16" max="80" @input="debuffSettingsSaved = false" />
                            <em>px</em>
                        </label>
                        <label title="Boss Debuff 缺失或临期提示音的统一音量；设为 0 可静音">
                            <span>音量</span>
                            <input v-model.number="appDebuffSettings.volume" type="number" min="0" max="100" @input="debuffSettingsSaved = false" />
                            <em>%</em>
                        </label>
                        <label class="reminder-coordinate-setting" title="Debuff 使用独立坐标，不会再与 Buff 排在同一行">
                            <span>坐标</span>
                            <b>X</b>
                            <input v-model.number="debuffOverlayX" type="number" min="-32000" max="32000" @input="debuffSettingsSaved = false; applyDebuffOverlayPosition(false)" />
                            <b>Y</b>
                            <input v-model.number="debuffOverlayY" type="number" min="-32000" max="32000" @input="debuffSettingsSaved = false; applyDebuffOverlayPosition(false)" />
                        </label>
                        <button type="button" class="reminder-save-settings debuff-save-settings" @click="saveAppDebuffSettings(true)">
                            <v-icon :icon="debuffSettingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" size="13" />
                            {{ debuffSettingsSaved ? "已保存" : "保存设定" }}
                        </button>
                    </div>
                </template>
            </GameDpsReport>

            <BattleRecordBrowser
                v-model="recordBrowserOpen"
                :busy="isFileLoading"
                :load-record="loadText"
            />

            <LogCleanupDialog v-if="!isStandalone && !isDesignPreview" />

            <SkillBarSettingsDialog v-if="!isStandalone && !isDesignPreview" v-model="skillBarSettingsOpen" />

            <v-dialog v-model="msgBoxOpen" max-width="520">
                <v-card>
                    <v-card-title>消息</v-card-title>
                    <v-card-text style="white-space: pre-wrap">{{ msgBoxText }}</v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn color="primary" @click="msgBoxOpen = false">关闭</v-btn>
                    </v-card-actions>
                </v-card>
            </v-dialog>

            <v-dialog v-model="updateDialogOpen" max-width="560">
                <v-card class="update-dialog-card">
                    <v-card-title><v-icon icon="mdi-update" class="mr-2" />在线更新</v-card-title>
                    <v-card-text v-if="updateInfo">
                        <p>当前版本：v{{ updateInfo.currentVersion }}　最新版本：v{{ updateInfo.latestVersion }}</p>
                        <p v-if="updateInfo.notes" class="update-notes">{{ updateInfo.notes }}</p>
                        <p v-if="!updateInfo.available">当前已经是最新版本。</p>
                        <p v-else>更新包会先经过 HTTPS 下载及 SHA-256 校验。确认无误后，软件会自动退出并替换程序文件；安装完成后不会自动重启，请手动重新打开软件。配置和战斗记录不会被删除。</p>
                    </v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn @click="updateDialogOpen = false">关闭</v-btn>
                        <v-btn v-if="updateInfo?.available" color="primary" :loading="updatePending" @click="downloadUpdate">立即更新并关闭</v-btn>
                    </v-card-actions>
                </v-card>
            </v-dialog>
        </v-main>
    </v-app>
</template>

<script lang="ts">
import { computed, defineComponent, inject, onMounted, onUnmounted, ref, watch } from "vue";
import {
    BATTLE_RECORD_LOADED_EVENT,
    parseBattleRecord,
} from "@/battleRecord";
import GameDpsReport from "@/components/GameDpsReport.vue";
import BuffOverlay from "@/components/BuffOverlay.vue";
import DebuffOverlay from "@/components/DebuffOverlay.vue";
import SkillCooldownOverlay from "@/components/SkillCooldownOverlay.vue";
import SkillBarSettingsDialog from "@/components/SkillBarSettingsDialog.vue";
import LogCleanupDialog from "@/components/LogCleanupDialog.vue";
import BattleRecordBrowser from "@/components/BattleRecordBrowser.vue";
import { BOSS_MECHANIC_EVENT } from "@/bossMechanicAlert";
import { ensureCnResourceNames } from "@/cnResources";
import { loadBuffOverlaySettings, saveBuffOverlaySettings } from "@/buffAlert";
import { loadDebuffAlertSettings, saveDebuffAlertSettings } from "@/debuffAlert";
import { SocketClient } from "@/lib/socketClient";
import { eventIdCharacterConditionDisable, eventIdCharacterConditionEnable, eventIdDamage, eventIdMessageBox, eventIdSkillAction, eventIdSkillCooldown, eventIdSkillState, type eventBase, type eventCharacterConditionEnable, type eventDamage, type eventMessageBox, type eventSkillAction, type eventSkillCooldown, type eventSkillState } from "@/protocols";
import { EFFECT_TIMER_CONDITION_EVENT } from "@/effectTimer";
import { SKILL_ACTION_EVENT, SKILL_COOLDOWN_ADJUSTMENT_EVENT, SKILL_DAMAGE_EVENT, SKILL_STATE_EVENT, techniqueCooldownActionFromCondition } from "@/skillCooldown";
import { loadSkillBarSettings, saveSkillBarSettings, syncNativeSkillBarSettings } from "@/skillBar";
import { clearTimeRange } from "@/store";
import { normalizeSkillDisplayName } from "@/skillDisplay";
import {
    UI_COLOR_THEMES,
    loadUiColorTheme,
    saveUiColorTheme,
    uiColorThemeStyle as buildUiColorThemeStyle,
} from "@/uiColorTheme";
import { hydrateFromSnapshot } from "@/worker/hydrateActorManager";
import type { WorkerOutMessage, WorkerSnapshot } from "@/worker/workerProtocol";

interface AppRuntimeStatus {
    state: "starting" | "waiting_game" | "detecting_game" | "capturing" | "replay" | "error";
    message: string;
    gameRunning: boolean;
    capturing: boolean;
    interface: string;
    updatedAt: number;
}

interface AppInfo {
    name: string;
    version: string;
}

interface UpdateInfo {
    currentVersion: string;
    latestVersion: string;
    available: boolean;
    notes?: string;
    savedPath?: string;
    installing?: boolean;
}

interface GameServerOption {
    id: string;
    name: string;
    network: string;
    ports: string[];
}

interface GameServerSettings {
    selected: string;
    options: GameServerOption[];
    acceleratorMode: boolean;
    message?: string;
}

export default defineComponent({
    name: "App",
    components: { GameDpsReport, BuffOverlay, DebuffOverlay, SkillCooldownOverlay, SkillBarSettingsDialog, LogCleanupDialog, BattleRecordBrowser },
    setup() {
        const db = inject("db") as any;
        const region = inject("region") as any;
        const lang = inject("lang") as any;
        const regionList = inject("regionList") as any;
        const raceNameMap = inject("raceNameMap") as any;
        const skillNameMap = inject("skillNameMap") as any;
        const condNameMap = inject("condNameMap") as any;
        const itemNameMap = inject("itemNameMap") as any;
        const appEvent = inject("appEvent") as any;
        const actorManager = inject("actorManager") as any;
        const dcManager = inject("dcManager") as any;

        const isStandalone = __IS_STANDALONE__;
        const isBuffOverlay = new URLSearchParams(window.location.search).has("buffOverlay");
        const isDebuffOverlay = new URLSearchParams(window.location.search).has("debuffOverlay");
        const isSkillOverlay = new URLSearchParams(window.location.search).has("skillOverlay");
        const isDesignPreview = import.meta.env.DEV && new URLSearchParams(window.location.search).has("preview");
        if (isDesignPreview) document.documentElement.classList.add("design-preview-root");
        const socketConnected = ref(false);
        const appName = ref("DilmeterCN");
        const appVersion = ref("1.4.1");
        const runtimeStatus = ref<AppRuntimeStatus>({
            state: isStandalone ? "replay" : "starting",
            message: isStandalone ? "本地日志模式" : "正在连接桌面监测器…",
            gameRunning: false,
            capturing: false,
            interface: "",
            updatedAt: 0,
        });
        const msgBoxOpen = ref(false);
        const msgBoxText = ref("");
        const isFileLoading = ref(false);
        const fileLoadProgress = ref(0);
        const fileLoadMessage = ref("");
        const foregroundRecoveryActive = ref(false);
        const foregroundRecoveryAwayMs = ref(0);
        const foregroundRecoveryDetail = computed(() => {
            const seconds = Math.max(1, Math.round(foregroundRecoveryAwayMs.value / 1000));
            if (seconds >= 60) {
                const minutes = Math.floor(seconds / 60);
                const remainder = seconds % 60;
                return `已识别到后台停留 ${minutes} 分${remainder ? ` ${remainder} 秒` : ""}，完成同步后自动恢复。`;
            }
            return `已识别到后台停留 ${seconds} 秒，完成同步后自动恢复。`;
        });
        const loadError = ref("");
        const updateInfo = ref<UpdateInfo | null>(null);
        const updatePending = ref(false);
        const updateDialogOpen = ref(false);
        const uiColorThemes = UI_COLOR_THEMES;
        const uiColorTheme = ref(loadUiColorTheme());
        const uiColorThemeVars = computed(() => buildUiColorThemeStyle(uiColorTheme.value));
        watch(uiColorTheme, (themeId) => {
            const themeStyle = buildUiColorThemeStyle(themeId);
            for (const [property, color] of Object.entries(themeStyle)) {
                document.documentElement.style.setProperty(property, color);
            }
            window.dispatchEvent(new CustomEvent("dilmeter-ui-color-theme-changed", {
                detail: { themeId },
            }));
        }, { immediate: true });
        const isRecordReplay = ref(false);
        const loadedRecordName = ref("");
        const recordBrowserOpen = ref(false);
        const skillBarSettingsOpen = ref(false);
        const selectedGameServer = ref("irusha");
        const savedGameServer = ref("irusha");
        const acceleratorMode = ref(false);
        const savedAcceleratorMode = ref(false);
        const gameServerOptions = ref<GameServerOption[]>([]);
        const serverSettingPending = ref(false);
        const dpsMonitoringEnabled = ref(loadDpsMonitoringEnabled());
        const settingsSaved = ref(false);
        const appBuffSettingsDirty = ref(false);
        const appBuffSettings = ref(loadBuffOverlaySettings());
        const appDebuffSettings = ref(loadDebuffAlertSettings());
        const buffOverlayX = ref(40);
        const buffOverlayY = ref(100);
        const debuffOverlayX = ref(40);
        const debuffOverlayY = ref(160);
        const debuffSettingsSaved = ref(false);
        let statusTimer: number | undefined;
        let settingsSavedTimer: number | undefined;
        let debuffSettingsSavedTimer: number | undefined;
        let overlayPositionTimer: number | undefined;
        let buffOverlayMoveSequence = 0;
        let debuffOverlayMoveSequence = 0;
        let liveRefreshPending = false;
        let pageHiddenAtMs = 0;
        let windowInactiveAtMs = 0;
        let pageHiddenNativeTickCount = 0;
        let nativeReactiveTickCount = 0;
        let lastNativeReactiveTickAtMs = 0;
        let lastRecoveryRefreshAtMs = 0;
        let uiDormantNeedsReload = false;
        let uiResumePending = false;
        let foregroundRecoveryStartedAtMs = 0;
        let nativeReactiveTickListener: EventListener | undefined;
        let hostWindowStateListener: EventListener | undefined;

        const statusLabel = computed(() => {
            if (isStandalone) return "本地日志";
            if (isRecordReplay.value) return "记录回放";
            if (runtimeStatus.value.capturing) return "正在监测";
            if (runtimeStatus.value.state === "detecting_game") return "正在连接游戏";
            if (runtimeStatus.value.state === "error") return "需要处理";
            return socketConnected.value ? "等待游戏" : "正在启动";
        });
        const statusColor = computed(() => {
            if (isRecordReplay.value) return "info";
            if (runtimeStatus.value.state === "error") return "error";
            if (runtimeStatus.value.capturing) return "success";
            if (runtimeStatus.value.gameRunning) return "warning";
            return "info";
        });

        const socket = new SocketClient("/ws");
        socket.onConnect = (isConnected) => (socketConnected.value = isConnected);
        socket.onEvent = (events) => {
            if (isRecordReplay.value) return;
            for (const event of events) {
                if (event.EventId === eventIdMessageBox) {
                    msgBoxOpen.value = true;
                    msgBoxText.value += `${(event as eventMessageBox).Message}\n`;
                    continue;
                }
                if (event.EventId === eventIdSkillAction) {
                    actorManager.value.onEvent(event);
                    window.dispatchEvent(new CustomEvent<eventSkillAction>(SKILL_ACTION_EVENT, {
                        detail: event as eventSkillAction,
                    }));
                    window.dispatchEvent(new CustomEvent(BOSS_MECHANIC_EVENT, { detail: event }));
                    continue;
                }
                if (event.EventId === eventIdSkillState) {
                    actorManager.value.onEvent(event);
                    window.dispatchEvent(new CustomEvent<eventSkillState>(SKILL_STATE_EVENT, {
                        detail: event as eventSkillState,
                    }));
                    continue;
                }
                if (event.EventId === eventIdSkillCooldown) {
                    window.dispatchEvent(new CustomEvent<eventSkillCooldown>(SKILL_COOLDOWN_ADJUSTMENT_EVENT, {
                        detail: event as eventSkillCooldown,
                    }));
                    continue;
                }
                if (event.EventId === eventIdDamage) {
                    window.dispatchEvent(new CustomEvent<eventDamage>(SKILL_DAMAGE_EVENT, {
                        detail: event as eventDamage,
                    }));
                }
                actorManager.value.onEvent(event);
                if (event.EventId === eventIdCharacterConditionEnable || event.EventId === eventIdCharacterConditionDisable) {
                    window.dispatchEvent(new CustomEvent(EFFECT_TIMER_CONDITION_EVENT, { detail: event }));
                }
                if (event.EventId === eventIdCharacterConditionEnable) {
                    const techniqueAction = techniqueCooldownActionFromCondition(
                        event as eventCharacterConditionEnable,
                        actorManager.value.localEntityId,
                    );
                    if (techniqueAction) {
                        window.dispatchEvent(new CustomEvent<eventSkillAction>(SKILL_ACTION_EVENT, {
                            detail: techniqueAction,
                        }));
                    }
                }
            }
        };

        const ensureDpsCaptureEnabled = async () => {
            if (isStandalone) return;
            try {
                await fetch("/api/dps_recording", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    // This preference controls presentation only. Keeping the
                    // native event stream intact is required for channel-change
                    // identity, Buff ownership and Boss reminder state.
                    body: JSON.stringify({ enabled: true }),
                });
            } catch {
                // Live processing still continues if an older host lacks this endpoint.
            }
        };

        const updateDpsMonitoring = () => {
            saveDpsMonitoringEnabled(dpsMonitoringEnabled.value);
        };

        const clearData = () => {
            appEvent.value.dispatchEvent(new CustomEvent("clear"));
            actorManager.value.clear();
            dcManager.value.clear();
            clearTimeRange();
        };

        const applySnapshot = (snapshot: WorkerSnapshot) => {
            appEvent.value.dispatchEvent(new CustomEvent("clear"));
            clearTimeRange();
            hydrateFromSnapshot(snapshot, actorManager.value, dcManager.value);
        };

        const waitForNextPaint = () => new Promise<void>((resolve) => {
            window.requestAnimationFrame(() => window.requestAnimationFrame(() => resolve()));
        });

        const beginForegroundRecovery = async (awayMs: number): Promise<boolean> => {
            if (awayMs < 3_000 || foregroundRecoveryActive.value) return foregroundRecoveryActive.value;
            foregroundRecoveryAwayMs.value = awayMs;
            foregroundRecoveryStartedAtMs = Date.now();
            foregroundRecoveryActive.value = true;
            fileLoadProgress.value = Math.max(2, fileLoadProgress.value);
            fileLoadMessage.value = "正在追赶后台数据...";
            // Paint the loading layer before snapshot hydration can occupy the UI thread.
            await waitForNextPaint();
            return true;
        };

        const finishForegroundRecovery = async (shown: boolean) => {
            if (!shown || !foregroundRecoveryActive.value) return;
            fileLoadProgress.value = 100;
            fileLoadMessage.value = "数据已同步，正在恢复界面...";
            await waitForNextPaint();
            const remainingMs = Math.max(0, 700 - (Date.now() - foregroundRecoveryStartedAtMs));
            if (remainingMs > 0) {
                await new Promise<void>((resolve) => window.setTimeout(resolve, remainingMs));
            }
            foregroundRecoveryActive.value = false;
            foregroundRecoveryAwayMs.value = 0;
        };

        const loadJsonData = (ndjson: string, replayMode: boolean): Promise<void> => {
            return new Promise<void>((resolve, reject) => {
                isFileLoading.value = true;
                fileLoadProgress.value = 0;
                fileLoadMessage.value = "解析日志...";

                const worker = new Worker(new URL("./worker/eventWorker.ts", import.meta.url), { type: "module" });
                worker.onmessage = (e: MessageEvent<WorkerOutMessage>) => {
                    const msg = e.data;
                    if (msg.type === "progress") {
                        fileLoadProgress.value = Math.min(94, msg.pct);
                        fileLoadMessage.value = msg.phase === "parse" ? "解析日志..." : "重建统计...";
                        return;
                    }
                    if (msg.type === "done") {
                        void (async () => {
                            try {
                                fileLoadMessage.value = "正在更新界面...";
                                fileLoadProgress.value = Math.max(96, fileLoadProgress.value);
                                if (foregroundRecoveryActive.value) await waitForNextPaint();
                                isRecordReplay.value = replayMode;
                                applySnapshot(msg.snapshot);
                                fileLoadProgress.value = 100;
                                if (foregroundRecoveryActive.value) await waitForNextPaint();
                                isFileLoading.value = false;
                                worker.terminate();
                                resolve();
                            } catch (error) {
                                isFileLoading.value = false;
                                worker.terminate();
                                reject(error);
                            }
                        })();
                        return;
                    }
                    if (msg.type === "error") {
                        isFileLoading.value = false;
                        worker.terminate();
                        reject(new Error(msg.message));
                    }
                };
                worker.onerror = (err) => {
                    isFileLoading.value = false;
                    worker.terminate();
                    reject(err);
                };
                worker.postMessage({ type: "process", ndjson });
            });
        };

        const loadText = async (text: string, sourceName: string) => {
            loadError.value = "";
            const record = parseBattleRecord(text);
            if (record) {
                isRecordReplay.value = true;
                loadedRecordName.value = `${record.battle.bossName} / ${sourceName}`;
                applySnapshot(record.snapshot);
                appEvent.value.dispatchEvent(new CustomEvent(BATTLE_RECORD_LOADED_EVENT, {
                    detail: record.selection,
                }));
                return;
            }

            loadedRecordName.value = sourceName;
            await loadJsonData(text, true);
        };

        const loadFromServer = async (resumeLiveStream: boolean | Event = false): Promise<boolean> => {
            if (liveRefreshPending || isFileLoading.value) return false;
            liveRefreshPending = true;
            isFileLoading.value = true;
            fileLoadProgress.value = 0;
            fileLoadMessage.value = "正在读取 DPS 日志...";
            const previousHandler = socket.onEvent;
            const temporaryEvents: eventBase[] = [];
            let loaded = false;
            try {
                socket.onEvent = (events) => temporaryEvents.push(...events);
                // A dormant UI reconnects only after its temporary event sink
                // is installed. Events arriving beyond the log's EOF are then
                // replayed after hydration instead of being lost.
                if (resumeLiveStream === true) socket.resume();
                const res = await fetch("/api/packet_log", { cache: "reload" });
                if (!res.ok) throw new Error(`HTTP ${res.status}`);
                loadedRecordName.value = "";
                await loadJsonData(await res.text(), false);
                loaded = true;
            } catch (e) {
                loadError.value = `返回实时失败：${e}`;
            } finally {
                socket.onEvent = previousHandler;
                if (!isRecordReplay.value && temporaryEvents.length > 0) {
                    // Replay through the complete live-event pipeline. Calling
                    // ActorManager directly would silently lose skill-CD and Boss
                    // mechanic signals that arrived while the snapshot was built.
                    previousHandler?.(temporaryEvents);
                }
                actorManager.value.flushPendingUpdates?.();
                dcManager.value.flushPendingUpdates?.();
                liveRefreshPending = false;
                isFileLoading.value = false;
            }
            return loaded;
        };

        const flushPendingUiUpdates = () => {
            actorManager.value.flushPendingUpdates?.();
            dcManager.value.flushPendingUpdates?.();
        };

        const resumeDormantUi = async (awayMs = 0) => {
            if (uiResumePending) return;
            if (isStandalone || isRecordReplay.value) {
                socket.resume();
                return;
            }
            if (liveRefreshPending || isFileLoading.value) {
                window.setTimeout(() => void resumeDormantUi(awayMs), 250);
                return;
            }
            uiResumePending = true;
            const recoveryShown = await beginForegroundRecovery(awayMs);
            fileLoadMessage.value = "正在加载 DPS 统计...";
            try {
                // Reconnect into a temporary sink while the foreground log is
                // rebuilt. Native reminders never use this socket; the queued
                // tail exists only to make the eventual DPS snapshot complete.
                const loaded = await loadFromServer(true);
                uiDormantNeedsReload = !loaded;
            } finally {
                socket.resume();
                uiResumePending = false;
                await finishForegroundRecovery(recoveryShown);
            }
        };

        const refreshAfterBackground = async (awayMs: number) => {
            const recoveryShown = await beginForegroundRecovery(awayMs);
            try {
                await loadFromServer();
            } finally {
                await finishForegroundRecovery(recoveryShown);
            }
        };

        const recoverAfterBackground = (awayMs: number, forceSnapshot = false) => {
            flushPendingUiUpdates();
            // A current WebView2 runtime remains live because the host keeps its
            // controller active. This resync is an authoritative fallback for old
            // runtimes, sleep/resume and directly opened Chrome tabs. Several
            // foreground signals can arrive together, so only one resync is allowed.
            const currentMs = Date.now();
            const nativeTicksWhileHidden = nativeReactiveTickCount - pageHiddenNativeTickCount;
            const nativeHeartbeatStayedActive = nativeTicksWhileHidden >= 2
                && currentMs - lastNativeReactiveTickAtMs <= 2_000;
            pageHiddenNativeTickCount = nativeReactiveTickCount;
            const socketWasOpen = socket.isOpen;
            if (uiDormantNeedsReload || socket.isSuspended) {
                void resumeDormantUi(awayMs);
                return;
            }
            socket.ensureConnected();
            if (
                awayMs >= 15_000
                && (
                    forceSnapshot
                    || !nativeHeartbeatStayedActive
                    || !socketWasOpen
                )
                && currentMs - lastRecoveryRefreshAtMs >= 10_000
                && !isStandalone
                && !isRecordReplay.value
            ) {
                lastRecoveryRefreshAtMs = currentMs;
                void refreshAfterBackground(awayMs);
            }
        };

        const notePageBackground = () => {
            if (pageHiddenAtMs > 0) return;
            pageHiddenAtMs = Date.now();
            pageHiddenNativeTickCount = nativeReactiveTickCount;
            if (!isStandalone && !isRecordReplay.value) {
                uiDormantNeedsReload = true;
                socket.suspend();
            }
        };

        const noteWindowInactive = () => {
            if (windowInactiveAtMs > 0) return;
            windowInactiveAtMs = Date.now();
        };

        const handlePageForeground = () => {
            if (document.hidden) return;
            const currentMs = Date.now();
            const hiddenAwayMs = pageHiddenAtMs > 0 ? currentMs - pageHiddenAtMs : 0;
            const inactiveAwayMs = windowInactiveAtMs > 0 ? currentMs - windowInactiveAtMs : 0;
            pageHiddenAtMs = 0;
            windowInactiveAtMs = 0;
            recoverAfterBackground(
                Math.max(hiddenAwayMs, inactiveAwayMs),
                hiddenAwayMs === 0 && inactiveAwayMs >= 15_000,
            );
        };

        const handlePageVisibilityChange = () => {
            if (document.hidden) notePageBackground();
            else handlePageForeground();
        };

        const refreshRuntimeStatus = async () => {
            if (isStandalone) return;
            try {
                const response = await fetch("/api/status", { cache: "no-store" });
                if (response.ok) runtimeStatus.value = await response.json();
            } catch {
                runtimeStatus.value = {
                    ...runtimeStatus.value,
                    state: "starting",
                    message: "正在等待桌面服务启动…",
                    capturing: false,
                };
            }
        };

        const loadGameServerSettings = async () => {
            if (isStandalone) return;
            try {
                const response = await fetch("/api/game_server", { cache: "no-store" });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const settings = await response.json() as GameServerSettings;
                gameServerOptions.value = settings.options;
                selectedGameServer.value = settings.selected;
                savedGameServer.value = settings.selected;
                acceleratorMode.value = settings.acceleratorMode;
                savedAcceleratorMode.value = settings.acceleratorMode;
            } catch (e) {
                loadError.value = `服务器设置读取失败：${e}`;
            }
        };

        const loadAppInfo = async () => {
            try {
                const response = await fetch("/api/app_info", { cache: "no-store" });
                if (!response.ok) return;
                const info = await response.json() as AppInfo;
                if (info.name) appName.value = info.name;
                if (info.version) appVersion.value = info.version;
            } catch {
                // Keep the embedded CN defaults when the desktop API is unavailable.
            }
        };

        const checkForUpdate = async (showDialog = false) => {
            if (isStandalone || updatePending.value) return;
            updatePending.value = true;
            try {
                const response = await fetch("/api/update", { cache: "no-store" });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                updateInfo.value = await response.json() as UpdateInfo;
                if (showDialog || updateInfo.value.available) updateDialogOpen.value = true;
            } catch (error) {
                if (showDialog) loadError.value = `检查更新失败：${error}`;
            } finally {
                updatePending.value = false;
            }
        };

        const downloadUpdate = async () => {
            if (updatePending.value) return;
            updatePending.value = true;
            try {
                const response = await fetch("/api/update", {
                    method: "POST",
                    headers: { "X-Dilmeter-Action": "install-update" },
                });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                updateInfo.value = await response.json() as UpdateInfo;
                updateDialogOpen.value = false;
                msgBoxText.value = "更新包已通过校验，软件即将关闭并安装。安装完成后不会自动重启，请手动重新打开 Dilmeter。";
                msgBoxOpen.value = true;
            } catch (error) {
                loadError.value = `下载更新失败：${error}`;
            } finally {
                updatePending.value = false;
            }
        };

        const updateUiColorTheme = () => {
            uiColorTheme.value = saveUiColorTheme(uiColorTheme.value);
        };

        const updateGameServer = async () => {
            if (isStandalone || serverSettingPending.value) return;
            serverSettingPending.value = true;
            loadError.value = "";
            try {
                const response = await fetch("/api/game_server", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        selected: selectedGameServer.value,
                        acceleratorMode: acceleratorMode.value,
                    }),
                });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                const settings = await response.json() as GameServerSettings;
                gameServerOptions.value = settings.options;
                selectedGameServer.value = settings.selected;
                savedGameServer.value = settings.selected;
                acceleratorMode.value = settings.acceleratorMode;
                savedAcceleratorMode.value = settings.acceleratorMode;
                runtimeStatus.value = {
                    ...runtimeStatus.value,
                    state: "waiting_game",
                    message: settings.message || "服务器设置已更新。",
                    gameRunning: false,
                    capturing: false,
                    interface: "",
                };
            } catch (e) {
                selectedGameServer.value = savedGameServer.value;
                acceleratorMode.value = savedAcceleratorMode.value;
                loadError.value = `连接设置更新失败：${e}`;
            } finally {
                serverSettingPending.value = false;
            }
        };

        const saveAppBuffSettings = async (showConfirmation = false) => {
            window.dispatchEvent(new CustomEvent("dilmeter-save-settings-request"));
            appBuffSettings.value.iconSize = Math.min(80, Math.max(16, Math.round(Number(appBuffSettings.value.iconSize) || 20)));
            appBuffSettings.value.volume = Math.min(100, Math.max(0, Math.round(Number(appBuffSettings.value.volume) || 0)));
            appBuffSettings.value.dpiPercent = Number(appBuffSettings.value.dpiPercent) > 0
                ? Math.min(500, Math.max(50, Math.round(Number(appBuffSettings.value.dpiPercent))))
                : 0;
            // The reminder page owns the rule editor. Always merge its latest
            // persisted rules before saving global settings so opening this
            // dialog can never restore an older rule list.
            const latestReminderSettings = loadBuffOverlaySettings();
            appBuffSettings.value.rules = latestReminderSettings.rules;
            appBuffSettings.value.timeAdjustmentSeconds = latestReminderSettings.timeAdjustmentSeconds;
            saveBuffOverlaySettings(appBuffSettings.value);
            await fetch("/api/buff_overlay", {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ locked: appBuffSettings.value.locked }),
            }).catch(() => undefined);
            await applyBuffOverlayPosition(true, false);
            appBuffSettingsDirty.value = false;
            if (showConfirmation) {
                settingsSaved.value = true;
                if (settingsSavedTimer !== undefined) window.clearTimeout(settingsSavedTimer);
                settingsSavedTimer = window.setTimeout(() => (settingsSaved.value = false), 1600);
            }
        };

        const persistOverlayAppearance = () => {
            const latest = loadBuffOverlaySettings();
            latest.overlayEnabled = appBuffSettings.value.overlayEnabled !== false;
            latest.opacity = Math.min(100, Math.max(20, Math.round(Number(appBuffSettings.value.opacity) || 100)));
            saveBuffOverlaySettings(latest);
            // Keep App's global controls in sync without holding a stale copy
            // of the rules that are edited by GameDpsReport.
            appBuffSettings.value.overlayEnabled = latest.overlayEnabled;
            appBuffSettings.value.opacity = latest.opacity;
            appBuffSettings.value.rules = latest.rules;
        };

        const persistOverlayLock = () => {
            const latest = loadBuffOverlaySettings();
            latest.locked = appBuffSettings.value.locked !== false;
            saveBuffOverlaySettings(latest);
            appBuffSettings.value = latest;
            void fetch("/api/buff_overlay", {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ locked: latest.locked }),
            }).catch(() => undefined);
        };

        const toggleOverlayVisibility = () => {
            appBuffSettings.value.overlayEnabled = !appBuffSettings.value.overlayEnabled;
            persistOverlayAppearance();
        };

        const previewOverlayAppearance = () => {
            markAppBuffSettingsDirty();
            persistOverlayAppearance();
        };

        const markAppBuffSettingsDirty = () => {
            settingsSaved.value = false;
            appBuffSettingsDirty.value = true;
        };

        const loadBuffOverlayPosition = async () => {
            try {
                const response = await fetch("/api/buff_overlay/position", { cache: "no-store" });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                const position = await response.json() as { x?: number; y?: number };
                buffOverlayX.value = Math.round(Number(position.x) || 0);
                buffOverlayY.value = Math.round(Number(position.y) || 0);
            } catch {
                // Keep the saved/default values when the overlay is unavailable.
            }
        };

        const applyBuffOverlayPosition = async (save = true, showError = true) => {
            const rawX = Number(buffOverlayX.value);
            const rawY = Number(buffOverlayY.value);
            if (!Number.isFinite(rawX) || !Number.isFinite(rawY)) return;
            buffOverlayX.value = Math.min(32000, Math.max(-32000, Math.round(rawX)));
            buffOverlayY.value = Math.min(32000, Math.max(-32000, Math.round(rawY)));
            const clockSequence = Math.round((performance.timeOrigin + performance.now()) * 1000);
            buffOverlayMoveSequence = Math.max(buffOverlayMoveSequence + 1, clockSequence);
            try {
                const response = await fetch("/api/buff_overlay/position", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        x: buffOverlayX.value,
                        y: buffOverlayY.value,
                        save,
                        sequence: buffOverlayMoveSequence,
                    }),
                });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
            } catch (error) {
                if (showError) {
                    msgBoxText.value = `应用悬浮图标坐标失败：${error}`;
                    msgBoxOpen.value = true;
                }
            }
        };

        const loadDebuffOverlayPosition = async () => {
            try {
                const response = await fetch("/api/debuff_overlay/position", { cache: "no-store" });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                const position = await response.json() as { x?: number; y?: number };
                debuffOverlayX.value = Math.round(Number(position.x) || 0);
                debuffOverlayY.value = Math.round(Number(position.y) || 0);
            } catch {
                // Keep the saved/default values when the overlay is unavailable.
            }
        };

        const applyDebuffOverlayPosition = async (save = true, showError = true) => {
            const rawX = Number(debuffOverlayX.value);
            const rawY = Number(debuffOverlayY.value);
            if (!Number.isFinite(rawX) || !Number.isFinite(rawY)) return;
            debuffOverlayX.value = Math.min(32000, Math.max(-32000, Math.round(rawX)));
            debuffOverlayY.value = Math.min(32000, Math.max(-32000, Math.round(rawY)));
            const clockSequence = Math.round((performance.timeOrigin + performance.now()) * 1000);
            debuffOverlayMoveSequence = Math.max(debuffOverlayMoveSequence + 1, clockSequence);
            try {
                const response = await fetch("/api/debuff_overlay/position", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        x: debuffOverlayX.value,
                        y: debuffOverlayY.value,
                        save,
                        sequence: debuffOverlayMoveSequence,
                    }),
                });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
            } catch (error) {
                if (showError) {
                    msgBoxText.value = `应用 Debuff 悬浮图标坐标失败：${error}`;
                    msgBoxOpen.value = true;
                }
            }
        };

        const saveAppDebuffSettings = async (showConfirmation = false) => {
            window.dispatchEvent(new CustomEvent("dilmeter-save-settings-request"));
            appDebuffSettings.value.iconSize = Math.min(80, Math.max(16, Math.round(Number(appDebuffSettings.value.iconSize) || 30)));
            appDebuffSettings.value.volume = Math.min(100, Math.max(0, Math.round(Number(appDebuffSettings.value.volume) || 0)));
            const latest = loadDebuffAlertSettings();
            latest.overlayEnabled = appDebuffSettings.value.overlayEnabled !== false;
            latest.iconSize = appDebuffSettings.value.iconSize;
            latest.volume = appDebuffSettings.value.volume;
            saveDebuffAlertSettings(latest);
            appDebuffSettings.value = latest;
            await applyDebuffOverlayPosition(true, false);
            if (showConfirmation) {
                debuffSettingsSaved.value = true;
                if (debuffSettingsSavedTimer !== undefined) window.clearTimeout(debuffSettingsSavedTimer);
                debuffSettingsSavedTimer = window.setTimeout(() => (debuffSettingsSaved.value = false), 1600);
            }
        };

        const toggleDebuffOverlayVisibility = () => {
            const latest = loadDebuffAlertSettings();
            latest.overlayEnabled = !appDebuffSettings.value.overlayEnabled;
            saveDebuffAlertSettings(latest);
            appDebuffSettings.value = latest;
            debuffSettingsSaved.value = false;
        };

        const persistDebuffOverlayVisibility = () => {
            const latest = loadDebuffAlertSettings();
            latest.overlayEnabled = appDebuffSettings.value.overlayEnabled !== false;
            saveDebuffAlertSettings(latest);
            appDebuffSettings.value = latest;
            debuffSettingsSaved.value = false;
        };

        const refreshAppDebuffSettings = () => {
            appDebuffSettings.value = loadDebuffAlertSettings();
            debuffSettingsSaved.value = false;
        };

        const refreshAppBuffSettings = () => {
            appBuffSettings.value = loadBuffOverlaySettings();
        };

        onMounted(async () => {
            if (isBuffOverlay || isDebuffOverlay || isSkillOverlay) return;
            void (async () => {
                const settings = loadSkillBarSettings();
                try {
                    const response = await fetch("/api/skill_bar", { cache: "no-store" });
                    if (response.ok) {
                        const nativeState = await response.json() as { x?: number; y?: number; positionSet?: boolean };
                        if (nativeState.positionSet) {
                            if (Number.isFinite(Number(nativeState.x))) settings.x = Math.round(Number(nativeState.x));
                            if (Number.isFinite(Number(nativeState.y))) settings.y = Math.round(Number(nativeState.y));
                            saveSkillBarSettings(settings);
                        }
                    }
                } catch {
                    // Keep the browser-side portable settings when native state is unavailable.
                }
                await syncNativeSkillBarSettings(settings);
            })().catch(() => undefined);
            region.value = "cn";
            lang.value = "cn";
            regionList.value = ["cn"];
            window.addEventListener("dilmeter-debuff-alert-settings", refreshAppDebuffSettings as EventListener);
            window.addEventListener("dilmeter-buff-alert-settings", refreshAppBuffSettings as EventListener);
            refreshAppDebuffSettings();
            nativeReactiveTickListener = (() => {
                const tickAtMs = Date.now();
                const tickGapMs = lastNativeReactiveTickAtMs > 0
                    ? tickAtMs - lastNativeReactiveTickAtMs
                    : 0;
                nativeReactiveTickCount += 1;
                lastNativeReactiveTickAtMs = tickAtMs;
                flushPendingUiUpdates();
                // Lock screen / sleep does not always generate a WebView
                // visibility or host minimize event. A gap in the native clock
                // is the reliable signal in that case.
                if (tickGapMs >= 15_000 && pageHiddenAtMs === 0) {
                    recoverAfterBackground(tickGapMs, true);
                }
            }) as EventListener;
            window.addEventListener("dilmeter-native-tick", nativeReactiveTickListener);
            hostWindowStateListener = ((event: CustomEvent<{ minimized?: boolean; awayMs?: number }>) => {
                if (event.detail?.minimized) {
                    notePageBackground();
                    return;
                }
                pageHiddenAtMs = 0;
                windowInactiveAtMs = 0;
                recoverAfterBackground(Math.max(0, Number(event.detail?.awayMs) || 0));
            }) as EventListener;
            window.addEventListener("dilmeter-host-window-state", hostWindowStateListener);
            document.addEventListener("visibilitychange", handlePageVisibilityChange);
            window.addEventListener("pagehide", notePageBackground);
            window.addEventListener("pageshow", handlePageForeground);
            window.addEventListener("blur", noteWindowInactive);
            window.addEventListener("focus", handlePageForeground);

            if (!isStandalone) {
                void ensureDpsCaptureEnabled();
                void loadAppInfo();
                window.setTimeout(() => void checkForUpdate(false), 3500);
                void loadBuffOverlayPosition();
                void loadDebuffOverlayPosition();
				overlayPositionTimer = window.setInterval(() => {
					if (appBuffSettings.value.locked) return;
					void loadBuffOverlayPosition();
					void loadDebuffOverlayPosition();
				}, 120);
                socket.connect();
                void loadGameServerSettings();
                void refreshRuntimeStatus();
                statusTimer = window.setInterval(refreshRuntimeStatus, 1500);
            }

            let databaseResourceError: unknown;
            try {
                await db.value.tryOpen();
                const [races, skills, conds, items] = await Promise.all([
                    db.value.getSortedListData("RaceList", "cn"),
                    db.value.getSortedListData("SkillList", "cn"),
                    db.value.getSortedListData("CharCondList", "cn"),
                    db.value.getSortedListData("ItemList", "cn"),
                ]);
                races.forEach((v: any) => (raceNameMap.value[v.Id] = `${toSimplified(db.value.getCurLangString(v.Name))} ${v.Id}`));
                skills.forEach((v: any) => {
                    const resourceName = toSimplified(db.value.getCurLangString(v.Name));
                    skillNameMap.value[v.Id] = normalizeSkillDisplayName(v.Id, resourceName);
                });
                conds.forEach((v: any) => (condNameMap.value[v.Id] = toSimplified(db.value.getCurLangString(v.Name))));
                items.forEach((v: any) => (itemNameMap.value[v.Id] = toSimplified(db.value.getCurLangString(v.Name))));
            } catch (e) {
                databaseResourceError = e;
            }

            // Always merge the bundled CN snapshot. IndexedDB can open
            // successfully with empty or stale tables after an interrupted
            // update, which previously left new IDs displayed as placeholders.
            // The packaged snapshot is the authoritative source for CN/RT.
            try {
                await ensureCnResourceNames();
                if (databaseResourceError) {
                    loadError.value = `正在使用随软件提供的 CN 资料；本机资料缓存更新失败：${databaseResourceError}`;
                }
            } catch (bundledResourceError) {
                if (databaseResourceError) {
                    loadError.value = `资料表加载失败：${databaseResourceError}`;
                } else {
                    console.warn("Bundled CN resource names failed to load", bundledResourceError);
                }
            }
        });

        onUnmounted(() => {
            if (statusTimer !== undefined) window.clearInterval(statusTimer);
            if (settingsSavedTimer !== undefined) window.clearTimeout(settingsSavedTimer);
            if (debuffSettingsSavedTimer !== undefined) window.clearTimeout(debuffSettingsSavedTimer);
            if (overlayPositionTimer !== undefined) window.clearInterval(overlayPositionTimer);
            window.removeEventListener("dilmeter-debuff-alert-settings", refreshAppDebuffSettings as EventListener);
            window.removeEventListener("dilmeter-buff-alert-settings", refreshAppBuffSettings as EventListener);
            if (nativeReactiveTickListener) window.removeEventListener("dilmeter-native-tick", nativeReactiveTickListener);
            if (hostWindowStateListener) window.removeEventListener("dilmeter-host-window-state", hostWindowStateListener);
            document.removeEventListener("visibilitychange", handlePageVisibilityChange);
            window.removeEventListener("pagehide", notePageBackground);
            window.removeEventListener("pageshow", handlePageForeground);
            window.removeEventListener("blur", noteWindowInactive);
            window.removeEventListener("focus", handlePageForeground);
            if (isDesignPreview) document.documentElement.classList.remove("design-preview-root");
        });

        return {
            isStandalone,
            isBuffOverlay,
            isDebuffOverlay,
            isSkillOverlay,
            isDesignPreview,
            appName,
            appVersion,
            updateInfo,
            updatePending,
            updateDialogOpen,
            checkForUpdate,
            downloadUpdate,
            uiColorThemes,
            uiColorTheme,
            uiColorThemeVars,
            updateUiColorTheme,
            socketConnected,
            runtimeStatus,
            statusLabel,
            statusColor,
            msgBoxOpen,
            msgBoxText,
            isFileLoading,
            fileLoadProgress,
            fileLoadMessage,
            foregroundRecoveryActive,
            foregroundRecoveryDetail,
            loadError,
            isRecordReplay,
            loadedRecordName,
            recordBrowserOpen,
            skillBarSettingsOpen,
            selectedGameServer,
            acceleratorMode,
            gameServerOptions,
            serverSettingPending,
            dpsMonitoringEnabled,
            settingsSaved,
            appBuffSettingsDirty,
            debuffSettingsSaved,
            appBuffSettings,
            appDebuffSettings,
            buffOverlayX,
            buffOverlayY,
            debuffOverlayX,
            debuffOverlayY,
            saveAppBuffSettings,
            saveAppDebuffSettings,
            toggleOverlayVisibility,
            toggleDebuffOverlayVisibility,
            persistOverlayAppearance,
            persistOverlayLock,
            persistDebuffOverlayVisibility,
            previewOverlayAppearance,
            markAppBuffSettingsDirty,
            applyBuffOverlayPosition,
            applyDebuffOverlayPosition,
            updateGameServer,
            updateDpsMonitoring,
            loadText,
            loadFromServer,
            clearData,
        };
    },
});

function toSimplified(text: string): string {
    const map: Record<string, string> = {
        "連": "连", "續": "续", "擊": "击", "閃": "闪", "焰": "焰", "護": "护", "盾": "盾",
        "轉": "转", "移": "移", "龍": "龙", "爆": "爆", "炎": "炎", "箭": "箭", "迅": "迅",
        "捷": "捷", "雙": "双", "槍": "枪", "鍊": "炼", "金": "金", "噩": "噩",
        "夢": "梦", "召": "召", "喚": "唤", "魔": "魔", "劍": "剑", "戰": "战", "鬥": "斗",
        "風": "风", "火": "火", "冰": "冰", "雷": "雷", "闇": "暗", "聖": "圣", "靈": "灵",
        "術": "术", "彈": "弹", "衝": "冲", "範": "范", "圍": "围", "傷": "伤",
        "害": "害", "強": "强", "化": "化", "弱": "弱", "體": "体", "輕": "轻", "重": "重",
        "復": "复", "藥": "药", "賦": "赋", "予": "予", "詠": "咏", "唱": "唱", "祈": "祈",
        "禱": "祷", "絕": "绝", "對": "对", "稱": "称", "號": "号", "標": "标", "記": "记",
        "減": "减", "緩": "缓", "暈": "晕", "敵": "敌", "騎": "骑", "寵": "宠", "鍛": "锻",
        "煉": "炼", "製": "制", "作": "作", "採": "采", "集": "集", "釣": "钓", "魚": "鱼",
        "藝": "艺", "樂": "乐", "詩": "诗", "進": "进", "階": "阶", "變": "变", "身": "身",
    };
    return text.replace(/[^\x00-\x7F]/g, (ch) => map[ch] ?? ch);
}

const DPS_MONITORING_STORAGE_KEY = "dilmeter-cn-dps-monitoring-v1";

function loadDpsMonitoringEnabled(): boolean {
    try { return localStorage.getItem(DPS_MONITORING_STORAGE_KEY) !== "false"; }
    catch { return true; }
}

function saveDpsMonitoringEnabled(enabled: boolean): void {
    try { localStorage.setItem(DPS_MONITORING_STORAGE_KEY, enabled ? "true" : "false"); }
    catch { /* keep the current session setting */ }
}
</script>

<style scoped>
.app-toolbar {
    background: linear-gradient(#252525, #101010);
    color: #f2f2f2;
    border-bottom: 1px solid #4b4b4b;
}

.app-version {
    margin-left: 6px;
    color: #9c9c9c;
    font-size: 11px;
    font-weight: 600;
}

.update-check-button,
.skill-bar-settings-button {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 28px;
    margin-right: 8px;
    padding: 0 9px;
    color: #d8d8d8;
    background: linear-gradient(#353535, #1b1b1b);
    border: 1px solid #5f5f5f;
    font-size: 10px;
    cursor: pointer;
}
.skill-bar-settings-button {
    margin-right: 7px;
    color: var(--ui-theme-text);
    border-color: var(--ui-theme-border);
    background: linear-gradient(var(--ui-theme-raised), var(--ui-theme-control));
}
.skill-bar-settings-button .v-icon { color: var(--ui-color-accent); }
.skill-bar-settings-button:hover {
    border-color: var(--ui-color-accent);
    background: linear-gradient(var(--ui-theme-surface-hover), var(--ui-theme-control));
}
.update-check-button.available { color: #f5ffe9; border-color: #82ba46; box-shadow: inset 0 0 8px rgba(123, 210, 42, .2); }
.update-check-button:disabled { opacity: .55; cursor: wait; }
.update-notes { padding: 9px; white-space: pre-wrap; background: #181818; border: 1px solid #484848; }

.ui-color-picker {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 28px;
    margin-right: 9px;
    padding: 0 5px 0 7px;
    color: #e4e4e4;
    background: linear-gradient(#353535, #1b1b1b);
    border: 1px solid color-mix(in srgb, var(--ui-color-accent) 58%, #555);
    font: 700 10px "Microsoft YaHei", sans-serif;
}

.ui-color-picker select {
    width: 86px;
    height: 21px;
    padding: 0 18px 0 5px;
    color: #fff;
    background: #151713;
    border: 1px solid color-mix(in srgb, var(--ui-color-accent) 72%, #555);
    outline: none;
    font: inherit;
}

.ui-color-swatch {
    width: 10px;
    height: 10px;
    background: var(--ui-color-accent);
    border: 1px solid rgba(255, 255, 255, .75);
    border-radius: 50%;
    box-shadow: 0 0 5px rgba(var(--ui-color-rgb), .8);
}

.main-dilmeter-app input[type="checkbox"],
.main-dilmeter-app input[type="radio"],
.main-dilmeter-app input[type="range"] {
    accent-color: var(--ui-color-accent) !important;
}

.reminder-inline-settings {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 6px;
    margin-left: auto;
    white-space: nowrap;
}

.reminder-inline-settings label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 27px;
    padding: 0 6px;
    color: #d8ded2;
    background: rgba(11, 14, 10, .58);
    border: 1px solid #5c6951;
    font-size: 10px;
}

.reminder-inline-settings .reminder-overlay-checkbox {
    color: #efffe6;
    border-color: #78915e;
    cursor: pointer;
}

.reminder-overlay-checkbox input {
    margin: 0;
    accent-color: #78d800;
}

.reminder-overlay-toggle {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    height: 27px;
    padding: 0 9px;
    color: #e7f3df;
    background: #26351f;
    border: 1px solid #78915e;
    box-shadow: inset 0 0 0 1px rgba(0, 0, 0, .55);
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
}

.reminder-overlay-toggle:hover {
    background: #34492a;
}

.reminder-overlay-toggle.paused {
    color: #c9c9c9;
    background: #292929;
    border-color: #6a6a6a;
}

.reminder-opacity-setting input[type="range"] {
    width: 72px;
    height: 16px;
    margin: 0;
    accent-color: #78d800;
    cursor: pointer;
}

.reminder-opacity-setting em {
    min-width: 30px;
    color: #efffdc;
    text-align: right;
}

.reminder-inline-settings input[type="number"] {
    box-sizing: border-box;
    width: 46px;
    height: 21px;
    padding: 0 4px;
    color: #ffffff;
    background: #151713;
    border: 1px solid #78826f;
    outline: none;
    font-size: 10px;
    font-weight: 700;
}

.reminder-inline-settings select {
    box-sizing: border-box;
    height: 21px;
    padding: 0 18px 0 5px;
    color: #ffffff;
    background: #151713;
    border: 1px solid #78826f;
    outline: none;
    font-size: 10px;
    font-weight: 700;
}

.reminder-inline-settings em,
.reminder-inline-settings b {
    color: #aeb7a6;
    font-size: 9px;
    font-style: normal;
    font-weight: 700;
}

.reminder-coordinate-setting input[type="number"] {
    width: 58px;
}

.reminder-save-settings {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    height: 27px;
    padding: 0 11px;
    color: #ffffff;
    background: linear-gradient(#51643e, #27321f);
    border: 1px solid #839e68;
    box-shadow: inset 0 0 0 1px rgba(0, 0, 0, .55);
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
}

.reminder-save-settings:hover {
    background: linear-gradient(#637d4a, #334329);
}

.reminder-save-settings.needs-save {
    animation: reminder-save-attention 1.05s ease-in-out infinite;
}

.reminder-save-settings.saved {
    color: #f3ffe7;
    background: linear-gradient(#6f934e, #334a25);
    border-color: #b3e583;
    box-shadow: 0 0 8px rgba(153, 231, 95, .68), inset 0 0 0 1px rgba(0, 0, 0, .45);
}

@keyframes reminder-save-attention {
    0%, 100% { border-color: #839e68; box-shadow: 0 0 0 rgba(171, 239, 105, 0), inset 0 0 0 1px rgba(0, 0, 0, .55); }
    50% { border-color: #c3ed92; box-shadow: 0 0 9px rgba(171, 239, 105, .82), inset 0 0 0 1px rgba(0, 0, 0, .35); }
}

.debuff-inline-settings label {
    color: #e5e8e0;
    background: rgba(20, 23, 18, .76);
    border-color: #59614e;
}

.debuff-inline-settings .reminder-overlay-checkbox {
    color: #f1f4ec;
    border-color: #748064;
}

.debuff-save-settings {
    background: linear-gradient(#526042, #2c3425);
    border-color: #7f916a;
}

.debuff-save-settings:hover {
    background: linear-gradient(#637650, #36432c);
}

@media (max-width: 1120px) {
    .reminder-inline-settings {
        flex-wrap: wrap;
    }
}

.server-switcher {
    display: flex;
    align-items: center;
    gap: 7px;
    min-width: 215px;
    color: #cfcfcf;
    font-size: 12px;
    font-weight: 600;
}

.accelerator-switch {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-right: 12px;
    color: #d8d8d8;
    font: 700 12px "Microsoft YaHei", sans-serif;
    cursor: pointer;
    white-space: nowrap;
}

.dps-recording-switch {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-right: 12px;
    color: #d8d8d8;
    font: 700 12px "Microsoft YaHei", sans-serif;
    cursor: pointer;
    white-space: nowrap;
}

.dps-recording-switch input {
    width: 15px;
    height: 15px;
    margin: 0;
    accent-color: #77d900;
}

.accelerator-switch input {
    width: 15px;
    height: 15px;
    margin: 0;
    accent-color: #77d900;
}

.accelerator-switch:has(input:disabled) {
    color: #777777;
    cursor: wait;
}

.server-switcher select {
    width: 150px;
    height: 30px;
    padding: 0 27px 0 9px;
    color: #f2f2f2;
    background: linear-gradient(#3e3e3e, #171717);
    border: 1px solid #656565;
    border-radius: 0;
    box-shadow: inset 0 0 0 1px #111111;
    outline: none;
    font: 700 12px "Microsoft YaHei", sans-serif;
}

.server-switcher select:focus {
    border-color: #9b9b9b;
}

.server-switcher select:disabled {
    color: #777777;
}

.foreground-recovery-overlay {
    position: fixed;
    inset: 0;
    z-index: 10000;
    display: grid;
    place-items: center;
    padding: 24px;
    background: rgba(4, 6, 4, .78);
    backdrop-filter: blur(3px);
}

.foreground-recovery-card {
    display: grid;
    grid-template-columns: 58px minmax(220px, 360px);
    align-items: center;
    gap: 5px 17px;
    width: min(500px, calc(100vw - 48px));
    padding: 22px 24px;
    color: #edf5e7;
    background: linear-gradient(145deg, rgba(36, 43, 32, .98), rgba(13, 16, 12, .98));
    border: 1px solid color-mix(in srgb, var(--ui-color-accent) 62%, #687060);
    box-shadow: 0 18px 55px rgba(0, 0, 0, .72), inset 0 0 22px rgba(var(--ui-color-rgb), .08);
}

.foreground-recovery-card .v-progress-circular {
    grid-row: 1 / 4;
}

.foreground-recovery-card strong {
    font-size: 18px;
    letter-spacing: .06em;
}

.foreground-recovery-card span {
    color: #d8e5cf;
    font-size: 12px;
}

.foreground-recovery-card small {
    color: #aeb9a6;
    font-size: 10px;
}

.foreground-recovery-card .v-progress-linear {
    grid-column: 1 / -1;
    margin-top: 11px;
}

.foreground-recovery-fade-enter-active,
.foreground-recovery-fade-leave-active {
    transition: opacity .18s ease;
}

.foreground-recovery-fade-enter-from,
.foreground-recovery-fade-leave-to {
    opacity: 0;
}

:deep(.v-main) {
    min-height: 100vh;
    background: #050505;
}
</style>
