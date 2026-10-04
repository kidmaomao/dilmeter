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
    <v-app v-else-if="isHealerOverlay" class="healer-overlay-app"><HealerOverlay /></v-app>
    <v-app v-else class="main-dilmeter-app modern-ui" :data-theme="uiColorTheme" :style="uiColorThemeVars">
        <v-main>
            <transition name="foreground-recovery-fade">
                <div
                    v-if="historyLoading && !isDesignPreview"
                    class="foreground-recovery-overlay"
                    role="status"
                    aria-live="polite"
                    aria-busy="true"
                >
                    <div class="foreground-recovery-card">
                        <v-progress-circular indeterminate color="primary" :size="58" :width="5" />
                        <strong>{{ $ui("正在加载所选场次") }}</strong>
                        <span>{{ $ui(fileLoadMessage || "正在读取场次记录...") }}</span>
                        <small>{{ $ui("只读取你选中的这段战斗记录，完成后自动打开。") }}</small>
                        <v-progress-linear :model-value="fileLoadProgress" color="primary" height="7" rounded />
                    </div>
                </div>
            </transition>
            <v-toolbar density="comfortable" class="app-toolbar">
                <v-toolbar-title class="text-body-1">
                    <img class="app-brand-icon" src="/app-icon.png" alt="" />
                    <strong>{{ appName }}</strong>
                    <span class="app-version" :title="appVersion">v{{ appVersion.includes('-ui-test.') ? appVersion.split('-ui-test.')[0] + ' · UI 测试版' : appVersion }}</span>
                </v-toolbar-title>
                <div class="app-toolbar-controls">
                <LanguageResourceSettings @loaded="resourceLoadError = ''" />
                <button v-if="!isStandalone" type="button" class="update-check-button" :class="{ available: updateInfo?.available }" :disabled="updatePending" @click="checkForUpdate(true)">
                    <v-icon :icon="updateInfo?.available ? 'mdi-download-circle' : 'mdi-update'" size="14" />
                    {{ $ui(updateInfo?.available ? `发现 v${updateInfo.latestVersion}` : "检查更新") }}
                </button>
                <div v-if="!isStandalone" class="ui-theme-control" :aria-label="$ui('UI 模式')">
                    <span>UI</span>
                    <div class="ui-theme-segmented">
                        <button v-for="mode in ['light', 'dark']" :key="mode" type="button" :class="{ active: uiColorTheme === mode }" :aria-pressed="uiColorTheme === mode" @click="uiColorTheme = mode; updateUiColorTheme()"><v-icon :icon="mode === 'light' ? 'mdi-white-balance-sunny' : 'mdi-weather-night'" size="14" />{{ $ui(mode === 'light' ? '浅色' : '深色') }}</button>
                    </div>
                </div>
                <label v-if="!isStandalone" class="dps-recording-switch" :title="$ui('关闭后只隐藏 DPS 界面；网络数据仍会照常处理，Buff、Debuff 和技能 CD 提醒不受影响')">
                    <input v-model="dpsMonitoringEnabled" type="checkbox" @change="updateDpsMonitoring" />
                    <span>{{ $ui("显示 DPS") }}</span>
                </label>
                <label v-if="!isStandalone" class="accelerator-switch" :title="$ui('让程序同时检测加速器、本地代理和虚拟网卡连接')">
                    <input
                        v-model="acceleratorMode"
                        type="checkbox"
                        :disabled="serverSettingPending"
                        @change="updateGameServer"
                    >
                    <span>{{ $ui("加速器兼容") }}</span>
                </label>
                <v-btn
                    v-if="!isStandalone"
                    class="server-settings-button"
                    size="small"
                    variant="outlined"
                    prepend-icon="mdi-server-network"
                    :loading="serverSettingPending"
                    @click="openServerSettings"
                >
                    {{ $ui(savedServerDisplay) }}
                </v-btn>
                </div>
                <div v-if="!isStandalone" class="app-runtime-notice" :class="{ error: runtimeStatus.state === 'error', capturing: runtimeStatus.capturing }" :title="$ui(runtimeNoticeDetail)" :aria-label="$ui(runtimeNoticeDetail)" role="status" aria-live="polite">
                    <v-icon :icon="isRecordReplay ? 'mdi-history' : runtimeStatus.state === 'error' ? 'mdi-alert-circle-outline' : runtimeStatus.capturing ? 'mdi-radar' : 'mdi-gamepad-variant-outline'" size="16" />
                    <span>{{ $ui(runtimeNoticeText) }}</span>
                </div>
            </v-toolbar>

            <v-sheet v-if="isFileLoading && !silentLiveLoading && !isDesignPreview" class="d-flex align-center pa-2" style="gap: 10px">
                <v-icon icon="mdi-file-import-outline" size="small" />
                <span class="text-caption">{{ $ui(fileLoadMessage) }}</span>
                <v-progress-linear :model-value="fileLoadProgress" color="primary" height="8" rounded />
                <span class="text-caption">{{ $ui(fileLoadProgress) }}%</span>
            </v-sheet>

            <v-alert v-if="loadError && !isDesignPreview" type="error" variant="tonal" density="compact" class="ma-2" closable @click:close="loadError = ''">
                {{ $ui(loadError) }}
            </v-alert>

            <v-alert v-if="resourceLoadError" type="warning" variant="tonal" density="compact" class="ma-2" closable @click:close="resourceLoadError = ''">{{ $ui(resourceLoadError) }}</v-alert>

            <GameDpsReport
                :is-file-loading="isFileLoading"
                :is-standalone="isStandalone"
                :dps-visible="dpsMonitoringEnabled"
                :battle-catalog="battleCatalog"
                :loaded-session-key="loadedSessionKey"
                :is-record-replay="isRecordReplay"
                :reminder-save-pending="reminderSavePending"
                @select-archived-session="loadArchivedSession"
                @return-live="loadFromServer"
                @refresh-info="loadFromServer"
                @view-records="recordBrowserOpen = true"
                @clear-data="clearData"
            >
                <template #team-reminders="{ active }">
                    <HealerMonitorPanel v-if="!isStandalone" embedded :open="active" :is-record-replay="isRecordReplay" />
                </template>
                <template #extra-skillbar="{ active }">
                    <SkillBarSettingsDialog v-if="!isStandalone" embedded :model-value="active" />
                </template>
                <template #reminder-save-actions="{ dirty, save, sync }">
                    <div class="reminder-save-actions">
                        <span role="status" aria-live="polite" :class="{ 'save-error': reminderSaveError }">{{ $ui(reminderSavePending ? '正在保存…' : reminderSaveError || (dirty || appBuffSettingsDirty || appDebuffSettingsDirty ? '有未保存修改' : '设置已同步')) }}</span>
                        <button type="button" class="reminder-save-all" :disabled="reminderSavePending" @click="saveAllReminderSettings(save, sync)">{{ $ui(reminderSavePending ? '保存中…' : '保存设定') }}</button>
                    </div>
                </template>
                <template #reminder-common-settings>
                    <div v-if="!isStandalone" class="reminder-inline-settings reminder-common-controls" :aria-label="$ui('覆盖层通用设置')">
                        <strong>{{ $ui("覆盖层通用") }}</strong>
                        <label class="reminder-overlay-checkbox" :title="$ui('锁定时鼠标穿透，不拦截游戏操作；解锁用于拖动已显示的提醒。也可使用各分类的坐标输入定位')">
                            <input v-model="appBuffSettings.locked" type="checkbox" @change="persistOverlayLock" />{{ $ui("\n                            锁定覆盖层\n                        ") }}</label>
                        <ReminderHelpTooltip :label="$ui('覆盖层锁定说明')" :text="$ui('勾选后鼠标穿透，不拦截游戏操作。取消勾选用于拖动已显示的提醒；也可在各分类中输入屏幕坐标定位。此项不控制额外技能栏。')" />
                        <label class="reminder-opacity-setting" :title="$ui('同时调整 Buff、Debuff、技能图标与倒计时的透明度')">
                            <span>{{ $ui("透明度") }}</span>
                            <input
                                v-model.number="appBuffSettings.opacity"
                                type="range"
                                min="20"
                                max="100"
                                step="5"
                                @input="previewOverlayAppearance"
                            />
                            <em>{{ $ui(appBuffSettings.opacity) }}%</em>
                        </label>
                        <label class="reminder-dpi-setting" :title="$ui('自动会读取原生悬浮窗当前显示器的缩放；手动档应与 Windows 显示缩放一致')">
                            <span>DPI</span>
                            <select v-model.number="appBuffSettings.dpiPercent" @change="markAppBuffSettingsDirty">
                                <option :value="0">{{ $ui("自动") }}</option>
                                <option v-for="percent in [100, 125, 150, 175, 200, 225, 250, 300]" :key="percent" :value="percent">
                                    {{ $ui(percent) }}%
                                </option>
                            </select>
                        </label>
                    </div>
                </template>
                <template #reminder-settings-actions>
                    <div v-if="!isStandalone" class="reminder-inline-settings" :aria-label="$ui('桌面 Buff 悬浮设置')">
                        <label class="reminder-overlay-checkbox" :title="$ui('独立控制 Buff 与技能覆盖层；后台计时和统计不会停止')">
                            <input v-model="appBuffSettings.overlayEnabled" type="checkbox" @change="previewOverlayAppearance" />{{ $ui("\n                            显示 Buff/技能\n                        ") }}</label>
                        <label :title="$ui('桌面 Buff 图标的宽高')">
                            <span>{{ $ui("图标") }}</span>
                            <input
                                v-model.number="appBuffSettings.iconSize"
                                type="number"
                                min="16"
                                max="80"
                                @input="markAppBuffSettingsDirty"
                            />
                            <em>px</em>
                        </label>
                        <label :title="$ui('Buff 到期提醒的统一音量')">
                            <span>{{ $ui("音量") }}</span>
                            <input
                                v-model.number="appBuffSettings.volume"
                                type="number"
                                min="0"
                                max="100"
                                @input="markAppBuffSettingsDirty"
                            />
                            <em>%</em>
                        </label>
                        <label class="reminder-coordinate-setting" :title="$ui('以主屏幕左上角为 (0, 0)，输入后悬浮图标会立即移动')">
                            <span>{{ $ui("坐标") }}</span>
                            <b>X</b>
                            <input v-model.number="buffOverlayX" type="number" min="-32000" max="32000" @input="markAppBuffSettingsDirty(); applyBuffOverlayPosition(false)" />
                            <b>Y</b>
                            <input v-model.number="buffOverlayY" type="number" min="-32000" max="32000" @input="markAppBuffSettingsDirty(); applyBuffOverlayPosition(false)" />
                        </label>
                    </div>
                </template>
                <template #debuff-reminder-settings-actions>
                    <div v-if="!isStandalone" class="reminder-inline-settings debuff-inline-settings" :aria-label="$ui('Boss Debuff 悬浮设置')">
                        <label class="reminder-overlay-checkbox" :title="$ui('独立控制 Boss Debuff 覆盖层')">
                            <input v-model="appDebuffSettings.overlayEnabled" type="checkbox" @change="persistDebuffOverlayVisibility" />{{ $ui("\n                            显示 Debuff\n                        ") }}</label>
                        <label :title="$ui('Boss Debuff 提醒图标的宽高')">
                            <span>{{ $ui("图标") }}</span>
                            <input v-model.number="appDebuffSettings.iconSize" type="number" min="16" max="80" @input="markAppDebuffSettingsDirty()" />
                            <em>px</em>
                        </label>
                        <label :title="$ui('Boss Debuff 缺失或临期提示音的统一音量；设为 0 可静音')">
                            <span>{{ $ui("音量") }}</span>
                            <input v-model.number="appDebuffSettings.volume" type="number" min="0" max="100" @input="markAppDebuffSettingsDirty()" />
                            <em>%</em>
                        </label>
                        <label class="reminder-coordinate-setting" :title="$ui('Debuff 使用独立坐标，不会再与 Buff 排在同一行')">
                            <span>{{ $ui("坐标") }}</span>
                            <b>X</b>
                            <input v-model.number="debuffOverlayX" type="number" min="-32000" max="32000" @input="markAppDebuffSettingsDirty(); applyDebuffOverlayPosition(false)" />
                            <b>Y</b>
                            <input v-model.number="debuffOverlayY" type="number" min="-32000" max="32000" @input="markAppDebuffSettingsDirty(); applyDebuffOverlayPosition(false)" />
                        </label>
                    </div>
                </template>
            </GameDpsReport>

            <BattleRecordBrowser
                v-model="recordBrowserOpen"
                :busy="isFileLoading"
                :load-record="loadText"
            />

            <v-dialog v-model="serverDialogOpen" max-width="560">
                <v-card class="server-settings-card">
                    <v-card-title class="d-flex align-center">
                        <v-icon icon="mdi-server-network" class="mr-2" />{{ $ui(" 服务器设置 ") }}</v-card-title>
                    <v-card-text>
                        <v-alert type="info" variant="tonal" density="compact" class="mb-4">{{ $ui(" IP 可填写单个 IPv4 或 CIDR 网段。修改后需要在游戏内切换一次地图，监测才会重新开始。 ") }}</v-alert>
                        <v-text-field
                            v-model="serverNetworkInput"
                            :label="$ui('服务器 IP / CIDR 网段')"
                            placeholder="211.147.76.0/24"
                            variant="outlined"
                            density="compact"
                            hide-details="auto"
                            class="mb-4"
                        />
                        <v-text-field
                            v-model="serverPortsInput"
                            :label="$ui('服务器端口')"
                            placeholder="11020, 11021, 11023"
                            :hint="$ui('多个端口可用逗号、分号或空格分隔')"
                            persistent-hint
                            variant="outlined"
                            density="compact"
                        />
                    </v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn :disabled="serverSettingPending" @click="serverDialogOpen = false">{{ $ui("取消") }}</v-btn>
                        <v-btn color="primary" :loading="serverSettingPending" @click="updateGameServer">{{ $ui("应用设置") }}</v-btn>
                    </v-card-actions>
                </v-card>
            </v-dialog>

            <LogCleanupDialog v-if="!isStandalone && !isDesignPreview" />



            <v-dialog v-model="msgBoxOpen" max-width="520">
                <v-card>
                    <v-card-title>{{ $ui("消息") }}</v-card-title>
                    <v-card-text style="white-space: pre-wrap">{{ $ui(msgBoxText) }}</v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn color="primary" @click="msgBoxOpen = false">{{ $ui("关闭") }}</v-btn>
                    </v-card-actions>
                </v-card>
            </v-dialog>

            <v-dialog v-model="updateDialogOpen" max-width="560">
                <v-card class="update-dialog-card">
                    <v-card-title><v-icon icon="mdi-update" class="mr-2" />{{ $ui("在线更新") }}</v-card-title>
                    <v-card-text v-if="updateInfo">
                        <p>{{ $ui("当前版本：v") }}{{ $ui(updateInfo.currentVersion) }}{{ $ui("　最新版本：v") }}{{ $ui(updateInfo.latestVersion) }}</p>
                        <p v-if="updateInfo.notes" class="update-notes">{{ $ui(updateInfo.notes) }}</p>
                        <p v-if="!updateInfo.available">{{ $ui("当前已经是最新版本。") }}</p>
                        <p v-else>{{ $ui("更新包会先经过 HTTPS 下载及 SHA-256 校验。确认无误后，软件会自动退出并替换程序文件；安装完成后不会自动重启，请手动重新打开软件。配置和战斗记录不会被删除。") }}</p>
                    </v-card-text>
                    <v-card-actions>
                        <v-spacer />
                        <v-btn @click="updateDialogOpen = false">{{ $ui("关闭") }}</v-btn>
                        <v-btn v-if="updateInfo?.available" color="primary" :loading="updatePending" @click="downloadUpdate">{{ $ui("立即更新并关闭") }}</v-btn>
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
import { loadResourceNames } from "@/resourceNames";
import { resourceRegion } from "@/uiLocale";
import LanguageResourceSettings from "@/components/LanguageResourceSettings.vue";
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
    loadUiColorTheme,
    saveUiColorTheme,
    uiColorThemeStyle as buildUiColorThemeStyle,
} from "@/uiColorTheme";
import { loadMainUiTheme, saveMainUiTheme, mainUiThemeStyle } from "@/mainUiTheme";
import { LiveBattleCursor, applyLiveDelta, needsLiveRecovery, type LiveBattleCatalog, type LiveBattleTarget } from "@/liveBattles";
import { bossDisplayName } from "@/bossDisplay";
import { ActorManager } from "@/eventActor";
import { DamageCollectorManager } from "@/actionCollector";
import HealerMonitorPanel from "@/components/HealerMonitorPanel.vue";
import ReminderHelpTooltip from "@/components/ReminderHelpTooltip.vue";
import HealerOverlay from "@/components/HealerOverlay.vue";
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

interface GameServerSettings {
    network: string;
    ports: string[];
    acceleratorMode: boolean;
    message?: string;
}

const defaultServerNetwork = "211.147.76.0/24";
const defaultServerPorts = ["11020", "11021", "11023"];

export default defineComponent({
    name: "App",
    components: { LanguageResourceSettings, ReminderHelpTooltip, GameDpsReport, BuffOverlay, DebuffOverlay, SkillCooldownOverlay, SkillBarSettingsDialog, LogCleanupDialog, BattleRecordBrowser, HealerMonitorPanel, HealerOverlay },
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
        const isHealerOverlay = new URLSearchParams(window.location.search).has("healerOverlay");
        const isDesignPreview = import.meta.env.DEV && new URLSearchParams(window.location.search).has("preview");
        if (isDesignPreview) document.documentElement.classList.add("design-preview-root");
        const socketConnected = ref(false);
        const appName = ref("DilmeterOT");
        const appVersion = ref("1.5.2 R2");
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
        const historyLoading = ref(false);
        const silentLiveLoading = ref(false);
        const battleCatalog = ref<LiveBattleCatalog | null>(null);
        const loadedSessionKey = ref("");
        const liveCursor = new LiveBattleCursor();
        let catalogPending = false;
        let catalogTimer: number | undefined;
        const loadError = ref("");
        const resourceLoadError = ref("");
        const updateInfo = ref<UpdateInfo | null>(null);
        const updatePending = ref(false);
        const updateDialogOpen = ref(false);
        const isOverlayWindow = isBuffOverlay || isDebuffOverlay || isSkillOverlay || isHealerOverlay;
        const uiColorTheme = ref(isOverlayWindow ? loadUiColorTheme() : loadMainUiTheme());
        const uiColorThemeVars = computed(() => isOverlayWindow ? buildUiColorThemeStyle(uiColorTheme.value) : mainUiThemeStyle(uiColorTheme.value));
        if (!isOverlayWindow) document.documentElement.classList.add("main-ui-root");
        watch(uiColorTheme, (themeId) => {
            const themeStyle = isOverlayWindow ? buildUiColorThemeStyle(themeId) : mainUiThemeStyle(themeId);
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
        const serverDialogOpen = ref(false);
        const serverNetworkInput = ref(defaultServerNetwork);
        const serverPortsInput = ref(defaultServerPorts.join(", "));
        const savedServerNetwork = ref(defaultServerNetwork);
        const savedServerPorts = ref([...defaultServerPorts]);
        const acceleratorMode = ref(false);
        const savedAcceleratorMode = ref(false);
        const serverSettingPending = ref(false);
        const dpsMonitoringEnabled = ref(loadDpsMonitoringEnabled());
        const settingsSaved = ref(false);
        const appBuffSettingsDirty = ref(false);
        const appDebuffSettingsDirty = ref(false);
        const reminderSavePending = ref(false);
        const reminderSaveError = ref("");
        const appBuffSettings = ref(loadBuffOverlaySettings());
        const appDebuffSettings = ref(loadDebuffAlertSettings());
        const buffOverlayX = ref(40);
        const buffOverlayY = ref(100);
        const debuffOverlayX = ref(40);
        const debuffOverlayY = ref(160);
        const debuffSettingsSaved = ref(false);
        const savedServerDisplay = computed(() => `${savedServerNetwork.value} · ${savedServerPorts.value.join(", ")}`);
        let statusTimer: number | undefined;
        let settingsSavedTimer: number | undefined;
        let debuffSettingsSavedTimer: number | undefined;
        let overlayPositionTimer: number | undefined;
        let buffOverlayMoveSequence = 0;
        let debuffOverlayMoveSequence = 0;
        let liveRefreshPending = false;
        let uiDormantNeedsReload = false;
        let uiResumePending = false;
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
        const runtimeNoticeDetail = computed(() => isRecordReplay.value
            ? `正在查看历史记录${loadedRecordName.value ? `：${loadedRecordName.value}` : ""}，实时数据暂不写入当前页面。`
            : `${runtimeStatus.value.message}${runtimeStatus.value.interface ? ` · ${runtimeStatus.value.interface}` : ""}`);
        const runtimeNoticeText = computed(() => {
            if (isRecordReplay.value) return `历史记录${loadedRecordName.value ? `：${loadedRecordName.value}` : "回放中"}`;
            if (runtimeStatus.value.state === "error") return `需要处理：${runtimeStatus.value.message}`;
            if (runtimeStatus.value.capturing) return "正在监测战斗";
            if (runtimeStatus.value.state === "detecting_game") return "正在连接洛奇…";
            if (runtimeStatus.value.state === "waiting_game" || socketConnected.value) return "等待洛奇启动 · 可查看历史记录";
            return "正在连接监测器…";
        });
        const statusColor = computed(() => {
            if (isRecordReplay.value) return "info";
            if (runtimeStatus.value.state === "error") return "error";
            if (runtimeStatus.value.capturing) return "success";
            if (runtimeStatus.value.gameRunning) return "warning";
            return "info";
        });

        const socket = new SocketClient("/ws");
        socket.onConnect = (isConnected) => {
            socketConnected.value = isConnected;
            if (isConnected && !isRecordReplay.value && !liveRefreshPending && !isFileLoading.value) {
                void loadFromServer(true);
            }
        };
        socket.onEvent = (events) => {
            if (isRecordReplay.value) return;
            for (const event of events) {
                if (!liveCursor.accept(event)) continue;
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
            loadedSessionKey.value = "";
        };

        const applySnapshot = (snapshot: WorkerSnapshot) => {
            // Materialize off to the side so a failed restore leaves the
            // previously displayed report intact and collectible as one unit.
            const nextCollector = new DamageCollectorManager();
            const nextActors = new ActorManager(nextCollector);
            hydrateFromSnapshot(snapshot, nextActors, nextCollector);
            actorManager.value.prepareSnapshot();
            appEvent.value.dispatchEvent(new CustomEvent("clear"));
            clearTimeRange();
            dcManager.value = nextCollector;
            actorManager.value = nextActors;
        };

        const waitForNextPaint = () => new Promise<void>((resolve) => {
            window.requestAnimationFrame(() => window.requestAnimationFrame(() => resolve()));
        });

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
                                if (!silentLiveLoading.value) await waitForNextPaint();
                                applySnapshot(msg.snapshot);
                                isRecordReplay.value = replayMode;
                                fileLoadProgress.value = 100;
                                if (!silentLiveLoading.value) await waitForNextPaint();
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
            loadedSessionKey.value = "";
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

        const refreshBattleCatalog = async () => {
            if (isStandalone || isDesignPreview || catalogPending || document.hidden) return;
            catalogPending = true;
            try {
                const response = await fetch("/api/live_battles", { cache: "no-store", signal: AbortSignal.timeout(15_000) });
                if (!response.ok) return;
                battleCatalog.value = await response.json() as LiveBattleCatalog;
                // Only a new live window replaces the resident details. Listing
                // archived targets never reads their damage logs.
                if (!isRecordReplay.value && battleCatalog.value.currentSessionKey
                    && (battleCatalog.value.currentSessionKey !== liveCursor.sessionKey
                        || battleCatalog.value.sequence > liveCursor.sequence)) {
                    await loadFromServer(true);
                }
            } catch { /* Retain the previous catalog during a transient reconnect. */ }
            finally { catalogPending = false; }
        };

        const loadFromServer = async (incremental: boolean | Event = false): Promise<boolean> => {
            if (liveRefreshPending || isFileLoading.value || isStandalone || isDesignPreview) return false;
            liveRefreshPending = true;
            silentLiveLoading.value = true;
            isFileLoading.value = true;
            loadError.value = "";
            const previousHandler = socket.onEvent;
            const temporaryEvents: eventBase[] = [];
            let loaded = false;
            try {
                socket.onEvent = (events) => {
                    for (const event of events) temporaryEvents.push(event);
                };
                socket.resume();
                const response = await fetch(liveCursor.requestUrl(incremental === true && !isRecordReplay.value), {
                    cache: "no-store", signal: AbortSignal.timeout(30_000),
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                const sessionKey = response.headers.get("X-Dilmeter-Session") || "";
                const sequence = Number(response.headers.get("X-Dilmeter-Sequence")) || 0;
                const delta = response.headers.get("X-Dilmeter-Delta") === "true";
                const ndjson = await response.text();
                if (delta) {
                    await applyLiveDelta(ndjson, (events) => previousHandler?.(events),
                        () => new Promise<void>((resolve) => window.setTimeout(resolve, 0)));
                    liveCursor.sequence = Math.max(liveCursor.sequence, sequence);
                } else {
                    await loadJsonData(ndjson, false);
                    liveCursor.sequence = sequence;
                }
                liveCursor.sessionKey = sessionKey;
                loadedSessionKey.value = sessionKey;
                isRecordReplay.value = false;
                loadedRecordName.value = "";
                loaded = true;
            } catch (error) {
                loadError.value = `同步近期战斗失败：${error}`;
            } finally {
                socket.onEvent = previousHandler;
                if (!isRecordReplay.value) {
                    // The disk snapshot already includes every sequence at or
                    // below its watermark. Only the genuinely new tail applies.
                    for (let offset = 0; offset < temporaryEvents.length; offset += 512) {
                        previousHandler?.(temporaryEvents.slice(offset, offset + 512)
                            .filter((event) => !loaded || Number(event.Sequence) > 0 || event.EventId < 0));
                    }
                }
                flushPendingUiUpdates();
                liveRefreshPending = false;
                isFileLoading.value = false;
                silentLiveLoading.value = false;
            }
            return loaded;
        };

        const loadArchivedSession = async (target: LiveBattleTarget) => {
            if (isFileLoading.value || liveRefreshPending) return;
            isFileLoading.value = true;
            historyLoading.value = true;
            fileLoadProgress.value = 0;
            fileLoadMessage.value = "正在读取所选场次...";
            loadError.value = "";
            try {
                await waitForNextPaint();
                const response = await fetch(`/api/live_battles?session=${encodeURIComponent(target.sessionKey)}`, {
                    cache: "no-store", signal: AbortSignal.timeout(30_000),
                });
                if (!response.ok) throw new Error(`HTTP ${response.status}`);
                await loadJsonData(await response.text(), true);
                loadedSessionKey.value = target.sessionKey;
                loadedRecordName.value = `${bossDisplayName(target, raceNameMap.value)} · ${new Date(target.startedAt * 1000).toLocaleTimeString()}`;
                appEvent.value.dispatchEvent(new CustomEvent(BATTLE_RECORD_LOADED_EVENT, {
                    detail: { bossEntityId: target.entityId, playerEntityId: "" },
                }));
            } catch (error) {
                loadError.value = `读取所选场次失败：${error}`;
            } finally {
                historyLoading.value = false;
                isFileLoading.value = false;
            }
        };

        const flushPendingUiUpdates = () => {
            actorManager.value.flushPendingUpdates?.();
            dcManager.value.flushPendingUpdates?.();
        };

        const resumeDormantUi = async () => {
            if (uiResumePending || liveRefreshPending || isFileLoading.value) return;
            uiResumePending = true;
            try {
                const loaded = await loadFromServer(true);
                uiDormantNeedsReload = !loaded;
            } finally {
                socket.resume();
                uiResumePending = false;
            }
        };

        const recoverAfterBackground = () => {
            flushPendingUiUpdates();
            if (isStandalone || isRecordReplay.value || document.hidden) return;
            if (uiDormantNeedsReload || needsLiveRecovery(socket.isSuspended, socket.isOpen)) {
                void resumeDormantUi();
            } else {
                socket.ensureConnected();
                void refreshBattleCatalog();
            }
        };

        const notePageBackground = () => {
            if (!isStandalone && !isRecordReplay.value) {
                uiDormantNeedsReload = true;
                socket.suspend();
            }
        };
        const handlePageForeground = () => {
            if (!document.hidden) recoverAfterBackground();
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
                savedServerNetwork.value = settings.network;
                savedServerPorts.value = [...settings.ports];
                serverNetworkInput.value = settings.network;
                serverPortsInput.value = settings.ports.join(", ");
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
                // Keep the embedded OT defaults when the desktop API is unavailable.
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
            uiColorTheme.value = isOverlayWindow ? saveUiColorTheme(uiColorTheme.value) : saveMainUiTheme(uiColorTheme.value);
        };

        const openServerSettings = () => {
            serverNetworkInput.value = savedServerNetwork.value;
            serverPortsInput.value = savedServerPorts.value.join(", ");
            acceleratorMode.value = savedAcceleratorMode.value;
            serverDialogOpen.value = true;
        };

        const splitServerPorts = (value: string) =>
            value.split(/[,，;；\s]+/).map((port) => port.trim()).filter(Boolean);

        const updateGameServer = async () => {
            if (isStandalone || serverSettingPending.value) return;
            serverSettingPending.value = true;
            loadError.value = "";
            try {
                const response = await fetch("/api/game_server", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: JSON.stringify({
                        network: serverNetworkInput.value.trim(),
                        ports: splitServerPorts(serverPortsInput.value),
                        acceleratorMode: acceleratorMode.value,
                    }),
                });
                if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
                const settings = await response.json() as GameServerSettings;
                savedServerNetwork.value = settings.network;
                savedServerPorts.value = [...settings.ports];
                serverNetworkInput.value = settings.network;
                serverPortsInput.value = settings.ports.join(", ");
                acceleratorMode.value = settings.acceleratorMode;
                savedAcceleratorMode.value = settings.acceleratorMode;
                serverDialogOpen.value = false;
                runtimeStatus.value = {
                    ...runtimeStatus.value,
                    state: "waiting_game",
                    message: settings.message || "服务器设置已更新。",
                    gameRunning: false,
                    capturing: false,
                    interface: "",
                };
            } catch (e) {
                acceleratorMode.value = savedAcceleratorMode.value;
                loadError.value = `连接设置更新失败：${e}`;
            } finally {
                serverSettingPending.value = false;
            }
        };

        const saveAppBuffSettings = async (showConfirmation = false, saveRules = true) => {
            const draft = { ...appBuffSettings.value };
            if (saveRules) window.dispatchEvent(new CustomEvent("dilmeter-save-settings-request"));
            appBuffSettings.value = draft;
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
            const lockSaved = await fetch("/api/buff_overlay", {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify({ locked: appBuffSettings.value.locked }),
            }).then(response => response.ok).catch(() => false);
            const positionSaved = await applyBuffOverlayPosition(true, false);
            appBuffSettingsDirty.value = false;
            if (showConfirmation) {
                settingsSaved.value = true;
                if (settingsSavedTimer !== undefined) window.clearTimeout(settingsSavedTimer);
                settingsSavedTimer = window.setTimeout(() => (settingsSaved.value = false), 1600);
            }
            return lockSaved && positionSaved;
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
            appBuffSettings.value = { ...appBuffSettings.value, locked: latest.locked, rules: latest.rules };
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

        const markAppDebuffSettingsDirty = () => {
            debuffSettingsSaved.value = false;
            appDebuffSettingsDirty.value = true;
        };

        const saveAllReminderSettings = async (saveRules: () => Promise<boolean>, syncFinal: () => Promise<boolean>) => {
            if (reminderSavePending.value) return;
            reminderSavePending.value = true;
            reminderSaveError.value = "";
            // Rule persistence emits synchronous refresh events. Keep both sets
            // of global controls before saving so neither draft can be replaced.
            const buffDraft = { ...appBuffSettings.value };
            const debuffDraft = { ...appDebuffSettings.value };
            try {
                await saveRules();
                let appearanceSaved = true;
                if (!isStandalone) {
                    appBuffSettings.value = buffDraft;
                    const buffSaved = await saveAppBuffSettings(false, false);
                    appDebuffSettings.value = debuffDraft;
                    const debuffSaved = await saveAppDebuffSettings(false, false);
                    appearanceSaved = buffSaved && debuffSaved;
                }
                const nativeSynced = await syncFinal();
                if (!appearanceSaved || !nativeSynced) throw new Error("sync failed");
                appBuffSettingsDirty.value = false;
                appDebuffSettingsDirty.value = false;
            } catch {
                appBuffSettingsDirty.value = true;
                appDebuffSettingsDirty.value = true;
                reminderSaveError.value = "保存未完成，请重试";
            } finally {
                reminderSavePending.value = false;
            }
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
            if (!Number.isFinite(rawX) || !Number.isFinite(rawY)) return false;
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
                return true;
            } catch (error) {
                if (showError) {
                    msgBoxText.value = `应用悬浮图标坐标失败：${error}`;
                    msgBoxOpen.value = true;
                }
                return false;
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
            if (!Number.isFinite(rawX) || !Number.isFinite(rawY)) return false;
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
                return true;
            } catch (error) {
                if (showError) {
                    msgBoxText.value = `应用 Debuff 悬浮图标坐标失败：${error}`;
                    msgBoxOpen.value = true;
                }
                return false;
            }
        };

        const saveAppDebuffSettings = async (showConfirmation = false, saveRules = true) => {
            const draft = { ...appDebuffSettings.value };
            if (saveRules) window.dispatchEvent(new CustomEvent("dilmeter-save-settings-request"));
            appDebuffSettings.value = draft;
            appDebuffSettings.value.iconSize = Math.min(80, Math.max(16, Math.round(Number(appDebuffSettings.value.iconSize) || 30)));
            appDebuffSettings.value.volume = Math.min(100, Math.max(0, Math.round(Number(appDebuffSettings.value.volume) || 0)));
            const latest = loadDebuffAlertSettings();
            latest.overlayEnabled = appDebuffSettings.value.overlayEnabled !== false;
            latest.iconSize = appDebuffSettings.value.iconSize;
            latest.volume = appDebuffSettings.value.volume;
            saveDebuffAlertSettings(latest);
            appDebuffSettings.value = latest;
            const positionSaved = await applyDebuffOverlayPosition(true, false);
            appDebuffSettingsDirty.value = !positionSaved;
            if (showConfirmation) {
                debuffSettingsSaved.value = true;
                if (debuffSettingsSavedTimer !== undefined) window.clearTimeout(debuffSettingsSavedTimer);
                debuffSettingsSavedTimer = window.setTimeout(() => (debuffSettingsSaved.value = false), 1600);
            }
            return positionSaved;
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
            const latest = loadDebuffAlertSettings();
            appDebuffSettings.value = appDebuffSettingsDirty.value
                ? { ...latest, iconSize: appDebuffSettings.value.iconSize, volume: appDebuffSettings.value.volume, overlayEnabled: appDebuffSettings.value.overlayEnabled }
                : latest;
            debuffSettingsSaved.value = false;
        };

        const refreshAppBuffSettings = () => {
            const latest = loadBuffOverlaySettings();
            appBuffSettings.value = appBuffSettingsDirty.value
                ? { ...latest, ...appBuffSettings.value, rules: latest.rules, timeAdjustmentSeconds: latest.timeAdjustmentSeconds }
                : latest;
        };

        onMounted(async () => {
            if (isBuffOverlay || isDebuffOverlay || isSkillOverlay || isHealerOverlay) return;
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

            window.addEventListener("dilmeter-debuff-alert-settings", refreshAppDebuffSettings as EventListener);
            window.addEventListener("dilmeter-buff-alert-settings", refreshAppBuffSettings as EventListener);
            refreshAppDebuffSettings();
            nativeReactiveTickListener = (() => {
                flushPendingUiUpdates();
                // A slow UI tick never initiates another reconstruction.
            }) as EventListener;
            window.addEventListener("dilmeter-native-tick", nativeReactiveTickListener);
            hostWindowStateListener = ((event: CustomEvent<{ minimized?: boolean; awayMs?: number }>) => {
                if (event.detail?.minimized) {
                    notePageBackground();
                    return;
                }
                recoverAfterBackground();
            }) as EventListener;
            window.addEventListener("dilmeter-host-window-state", hostWindowStateListener);
            document.addEventListener("visibilitychange", handlePageVisibilityChange);
            window.addEventListener("pagehide", notePageBackground);
            window.addEventListener("pageshow", handlePageForeground);
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
                void refreshBattleCatalog();
                catalogTimer = window.setInterval(refreshBattleCatalog, 2000);
            }

            try {
                const result = await loadResourceNames(resourceRegion.value);
                if (result.cached) resourceLoadError.value = "资料更新失败，正在使用所选服务器的本地缓存。";
            } catch (e) {
                resourceLoadError.value = `资料载入失败，请在「资料」中重试：${e}`;
            }
        });

        onUnmounted(() => {
            socket.suspend();
            if (catalogTimer !== undefined) window.clearInterval(catalogTimer);
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
            window.removeEventListener("focus", handlePageForeground);
            if (isDesignPreview) document.documentElement.classList.remove("design-preview-root");
        });

        return {
            isStandalone,
            isBuffOverlay,
            isDebuffOverlay,
            isSkillOverlay,
            isHealerOverlay,
            isDesignPreview,
            appName,
            appVersion,
            updateInfo,
            updatePending,
            updateDialogOpen,
            checkForUpdate,
            downloadUpdate,
            uiColorTheme,
            uiColorThemeVars,
            updateUiColorTheme,
            socketConnected,
            runtimeStatus,
            runtimeNoticeText,
            runtimeNoticeDetail,
            statusLabel,
            statusColor,
            msgBoxOpen,
            msgBoxText,
            isFileLoading,
            fileLoadProgress,
            fileLoadMessage,
            historyLoading,
            silentLiveLoading,
            battleCatalog,
            loadedSessionKey,
            loadArchivedSession,
            loadError,
            resourceLoadError,
            isRecordReplay,
            loadedRecordName,
            recordBrowserOpen,
            serverDialogOpen,
            serverNetworkInput,
            serverPortsInput,
            acceleratorMode,
            serverSettingPending,
            dpsMonitoringEnabled,
            savedServerDisplay,
            settingsSaved,
            appBuffSettingsDirty,
            appDebuffSettingsDirty,
            reminderSavePending,
            reminderSaveError,
            saveAllReminderSettings,
            markAppDebuffSettingsDirty,
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
            openServerSettings,
            updateGameServer,
            updateDpsMonitoring,
            loadText,
            loadFromServer,
            clearData,
        };
    },
});


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

.app-toolbar :deep(.v-toolbar__content) {
    height: auto !important;
    min-height: 48px;
    flex-wrap: wrap;
    row-gap: 4px;
    padding: 4px 0;
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
