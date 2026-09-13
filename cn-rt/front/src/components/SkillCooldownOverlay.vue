<template>
    <main
        class="skill-overlay-root"
        :style="{
            ...overlayThemeStyle,
            '--skill-icon-size': `${settings.iconSize}px`,
            '--overlay-opacity': String(overlayAppearance.opacity / 100),
        }"
    >
        <section v-if="visibleTargetHealth" class="target-health-overlay-list" aria-label="安乐碎片机制血量">
            <div
                class="target-health-overlay-item"
                :style="targetHealthPositionStyle(visibleTargetHealth)"
                role="meter"
                aria-label="选中目标血量"
                :aria-valuenow="targetHealthPercent"
                aria-valuemin="0"
                aria-valuemax="100"
                :title="targetHealthTitle"
            >
                <div class="target-health-bar">
                    <span class="target-health-fill" :style="{ width: `${targetHealthPercent}%` }"></span>
                    <strong class="target-health-name">{{ visibleTargetHealth.name }} · {{ visibleTargetHealth.phaseLabel }}</strong>
                    <span class="target-health-percent">{{ targetHealthPercent.toFixed(2) }} %</span>
                </div>
            </div>
        </section>
        <section v-if="visibleAimReminder" class="aim-reminder-overlay-list" aria-label="穿心箭瞄准提醒">
            <article
                class="aim-reminder-overlay-item"
                :class="{
                    'aim-idle': !visibleAimReminder.active,
                    'aim-near-ready': isAimNearReady(visibleAimReminder),
                    'aim-ready': isAimReady(visibleAimReminder),
                    'aim-full': isAimFull(visibleAimReminder),
                }"
                :style="aimReminderPositionStyle(visibleAimReminder)"
            >
                <div class="aim-reminder-card">
                    <div class="aim-reminder-icon" aria-hidden="true">
                        <span>弓</span>
                        <img :src="visibleAimReminder.iconUrl" alt="" @error="hideImage" />
                    </div>
                    <div class="aim-reminder-body">
                        <div class="aim-reminder-title">
                            <strong>穿心</strong>
                            <span>85%</span>
                        </div>
                        <div class="aim-reminder-track" aria-hidden="true">
                            <span class="aim-reminder-fill" :style="aimReminderProgressStyle(visibleAimReminder)" />
                            <i class="aim-reminder-best-marker" />
                        </div>
                        <div class="aim-reminder-footer">
                            <span class="aim-reminder-buffs">{{ aimReminderBuffText(visibleAimReminder) }}</span>
                            <strong class="aim-reminder-percent">{{ aimReminderProgressText(visibleAimReminder) }}</strong>
                        </div>
                    </div>
                </div>
            </article>
        </section>
        <section v-if="visibleEffectTimers.length" class="effect-timer-overlay-list" aria-label="伤害增益效果计时">
            <article
                v-for="item in visibleEffectTimers"
                :key="`${item.key}-${item.generation}`"
                class="effect-timer-overlay-item"
                :class="`effect-${item.orientation}`"
                :style="effectTimerPositionStyle(item)"
                :title="`${item.name} · ${item.sourceType === 'skill' ? '技能' : '状态'} ${item.sourceId}`"
            >
                <div class="effect-timer-card">
                    <div class="effect-timer-icon" aria-hidden="true">
                        <span>{{ item.sourceType === "skill" ? "技" : "态" }}</span>
                        <img :src="item.iconUrl" alt="" @error="hideImage" />
                    </div>
                    <div class="effect-timer-body">
                        <strong v-if="item.orientation !== 'vertical'">{{ item.name }}</strong>
                        <div class="effect-timer-track" aria-hidden="true">
                            <i :style="effectTimerProgressStyle(item)" />
                            <span class="effect-timer-remaining">{{ effectTimerRemainingText(item) }}</span>
                        </div>
                    </div>
                </div>
            </article>
        </section>
        <section v-if="visibleItems.length" class="skill-overlay-list" aria-label="技能冷却完成提醒">
            <article
                v-for="item in visibleItems"
                :key="`${item.skillId}-${item.generation}`"
                class="skill-overlay-item"
                :class="{
                    cooling: isCooling(item),
                    'ready-burst': isReadyBurst(item),
                    persistent: item.alwaysVisible,
                    accumulating: isCumulativeAccumulating(item),
                    'progress-tracked': isProgressItem(item),
                    'progress-observed': item.progressObserved,
                    'progress-full': Number(item.progressPercent) >= 100,
                }"
                :style="itemPositionStyle(item)"
                :title="`${item.name}（技能 ${item.skillId}）`"
            >
                <div class="skill-particle-field" aria-hidden="true">
                    <v-icon v-for="index in 8" :key="index" icon="mdi-star-four-points" class="skill-particle" />
                </div>
                <div class="skill-icon-shell">
                    <span class="skill-icon-fallback">{{ item.skillId }}</span>
                    <img :src="item.iconUrl" :alt="`${item.name}图标`" @error="hideImage" />
                    <span v-if="item.petSkill" class="skill-pet-badge">宠</span>
                    <span
                        v-if="isCooling(item)"
                        class="skill-cooldown-sweep"
                        :style="cooldownSweepStyle(item)"
                        aria-hidden="true"
                    />
                    <span
                        v-if="isProgressItem(item)"
                        class="toah-progress-mask"
                        :style="toahProgressMaskStyle(item)"
                        aria-hidden="true"
                    />
                </div>
                <span v-if="isProgressItem(item)" class="skill-cooldown-countdown toah-progress-label">{{ toahProgressText(item) }}</span>
                <span v-else-if="isCumulativeAccumulating(item)" class="skill-cooldown-countdown cumulative-cooldown-label">{{ cumulativeCooldownText(item) }}</span>
                <span v-else-if="isCooling(item)" class="skill-cooldown-countdown">{{ remainingText(item) }}</span>
            </article>
        </section>
        <section v-if="visibleMechanics.length" class="boss-mechanic-overlay-list" aria-label="Boss 特殊机制倒计时">
            <article
                v-for="mechanic in visibleMechanics"
                :key="`${mechanic.key}-${mechanic.generation}`"
                class="boss-mechanic-overlay-item"
                :class="mechanicAlertClass(mechanic)"
                :style="mechanicPositionStyle(mechanic)"
                :title="mechanic.name"
            >
                <div class="boss-mechanic-card">
                    <v-icon :icon="mechanic.icon" class="boss-mechanic-icon" aria-hidden="true" />
                    <strong>{{ mechanicRemainingText(mechanic) }}</strong>
                    <span class="boss-mechanic-progress" :style="mechanicProgressStyle(mechanic)" aria-hidden="true" />
                </div>
            </article>
        </section>
        <section v-if="visibleStackAlerts.length" class="buff-stack-overlay-list" aria-label="Buff 层数提醒">
            <article
                v-for="alert in visibleStackAlerts"
                :key="`${alert.skillId ? 'skill-' + alert.skillId : alert.ccId}-${alert.generation}`"
                class="buff-stack-overlay-item"
                :class="{ persistent: alert.persistent }"
                :style="stackAlertPositionStyle(alert)"
            >
                <div class="buff-stack-card">
                    <span>{{ alert.name }}</span>
                    <strong>{{ alert.quantityText ?? alert.stack }}</strong>
                    <small>{{ alert.quantityUnit ?? "层" }}</small>
                </div>
            </article>
        </section>
    </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { loadBuffOverlaySettings, resolveOverlayDpiPercent } from "@/buffAlert";
import { loadUiColorTheme, uiColorThemeStyle } from "@/uiColorTheme";
import type { EffectTimerOverlayItem } from "@/effectTimer";
import {
    SKILL_COOLDOWN_CHANNEL,
    MAGNUM_BEST_SHOT_PROGRESS,
    calculateMagnumAimDisplayProgress,
    loadSkillCooldownSettings,
    shouldShowToahSpiritProgressOverlay,
    toahSpiritDisplayPercent,
    type AimReminderOverlayItem,
    type BossMechanicOverlayItem,
    type BuffStackAlertOverlayItem,
    type SkillCooldownOverlayItem,
    type SkillCooldownOverlayMessage,
    type TargetHealthBarOverlayItem,
} from "@/skillCooldown";

const READY_ANIMATION_MS = 2600;
// The native window is resized to this computed paint area.  Keep enough
// transparent room for the largest (200%) mechanic glow and warning pulse so
// WebView2 never clips the card at custom DPI/size settings.
const PARTICLE_MARGIN = 96;
const ITEM_HORIZONTAL_PADDING = 12;
const ITEM_VERTICAL_PADDING = 15;
const AIM_REMINDER_WIDTH = 292;
const AIM_REMINDER_HEIGHT = 64;
const TARGET_HEALTH_WIDTH = 420;
const TARGET_HEALTH_HEIGHT = 38;
const EFFECT_TIMER_HORIZONTAL_WIDTH = 292;
const EFFECT_TIMER_HORIZONTAL_HEIGHT = 46;
const EFFECT_TIMER_VERTICAL_WIDTH = 46;
const EFFECT_TIMER_VERTICAL_HEIGHT = 174;
const settings = reactive(loadSkillCooldownSettings());
const initialOverlayAppearance = loadBuffOverlaySettings();
const overlayAppearance = reactive({
    enabled: initialOverlayAppearance.overlayEnabled,
    opacity: initialOverlayAppearance.opacity,
    dpiPercent: initialOverlayAppearance.dpiPercent,
});
const items = ref<SkillCooldownOverlayItem[]>([]);
const mechanics = ref<BossMechanicOverlayItem[]>([]);
const stackAlerts = ref<BuffStackAlertOverlayItem[]>([]);
const aimReminder = ref<AimReminderOverlayItem>();
const targetHealth = ref<TargetHealthBarOverlayItem>();
const effectTimers = ref<EffectTimerOverlayItem[]>([]);
const overlayThemeStyle = reactive(uiColorThemeStyle(loadUiColorTheme()));
const nowMs = ref(Date.now());
const observedDevicePixelRatio = ref(window.devicePixelRatio);
let channel: BroadcastChannel | undefined;
let clockTimer: number | undefined;
let pollTimer: number | undefined;
let aimBestPointTimer: number | undefined;
let scheduledAimBestPointKey = "";
let nativeClockTickListener: EventListener | undefined;
let lastOverlayPollAtMs = 0;
let lastMessageAt = 0;
let nativeStateKey = "";
let nativeStateSequence = 0;

const visibleItems = computed(() => items.value.filter((item) => {
    if (item.barOnly) return false;
    if (isProgressItem(item)) return shouldShowToahSpiritProgressOverlay(item);
    if (item.alwaysVisible) return true;
    if (item.readyAtMs <= 0) return false;
    const elapsed = nowMs.value - item.readyAtMs;
    return elapsed >= 0 && elapsed < READY_ANIMATION_MS;
}));
const visibleAimReminder = computed(() => aimReminder.value?.active || aimReminder.value?.alwaysVisible
    ? aimReminder.value
    : undefined);
const visibleMechanics = computed(() => mechanics.value.filter((item) => item.endsAtMs > nowMs.value));
const visibleStackAlerts = computed(() => stackAlerts.value.filter((item) => item.persistent || item.endsAtMs > nowMs.value));
const visibleEffectTimers = computed(() => effectTimers.value.filter((item) => item.enabled && (item.alwaysVisible || item.endsAtMs > nowMs.value)));
const visibleTargetHealth = computed(() => {
    const target = targetHealth.value;
    if (!target || !Number.isFinite(target.currentHealth) || !Number.isFinite(target.maximumHealth) || target.maximumHealth <= 0) {
        return undefined;
    }
    if (target.previewExpiresAtMs && target.previewExpiresAtMs <= nowMs.value) return undefined;
    return target;
});
const targetHealthPercent = computed(() => {
    const target = visibleTargetHealth.value;
    return target ? Math.max(0, Math.min(100, target.currentHealth / target.maximumHealth * 100)) : 0;
});
const targetHealthTitle = computed(() => {
    const target = visibleTargetHealth.value;
    if (!target) return "";
    return `${target.name}：${Math.max(0, target.currentHealth).toLocaleString("zh-CN")} / ${target.maximumHealth.toLocaleString("zh-CN")}`;
});

const effectiveScalePercent = computed(() => resolveOverlayDpiPercent(
    overlayAppearance.dpiPercent,
    observedDevicePixelRatio.value,
));

const effectiveScale = computed(() => effectiveScalePercent.value / 100);
const layoutBounds = computed(() => {
    const scale = effectiveScale.value;
    if (!visibleAimReminder.value && !visibleTargetHealth.value && !visibleEffectTimers.value.length && !visibleItems.value.length && !visibleMechanics.value.length && !visibleStackAlerts.value.length) {
        const minimumSize = Math.ceil(120 * scale);
        return { x: 0, y: 0, width: minimumSize, height: minimumSize };
    }
    const iconSize = settings.iconSize;
    const scaledMargin = PARTICLE_MARGIN * scale;
    const scaledIconSize = iconSize * scale;
    const skillMinX = visibleItems.value.map((item) => item.x);
    const skillMinY = visibleItems.value.map((item) => item.y);
    const skillMaxX = visibleItems.value.map((item) => item.x + scaledIconSize);
    const skillMaxY = visibleItems.value.map((item) => item.y + scaledIconSize);
    const mechanicMinX = visibleMechanics.value.map((item) => item.x);
    const mechanicMinY = visibleMechanics.value.map((item) => item.y);
    const mechanicMaxX = visibleMechanics.value.map((item) => item.x + 118 * mechanicScale(item) * scale);
    const mechanicMaxY = visibleMechanics.value.map((item) => item.y + 118 * mechanicScale(item) * scale);
    const stackMinX = visibleStackAlerts.value.map((item) => item.x);
    const stackMinY = visibleStackAlerts.value.map((item) => item.y);
    const stackMaxX = visibleStackAlerts.value.map((item) => item.x + 220 * mechanicScale(item) * scale);
    const stackMaxY = visibleStackAlerts.value.map((item) => item.y + 104 * mechanicScale(item) * scale);
    const aimMinX = visibleAimReminder.value ? [visibleAimReminder.value.x] : [];
    const aimMinY = visibleAimReminder.value ? [visibleAimReminder.value.y] : [];
    const aimScale = visibleAimReminder.value ? aimReminderScale(visibleAimReminder.value) : 1;
    const aimMaxX = visibleAimReminder.value ? [visibleAimReminder.value.x + AIM_REMINDER_WIDTH * aimScale * scale] : [];
    const aimMaxY = visibleAimReminder.value ? [visibleAimReminder.value.y + AIM_REMINDER_HEIGHT * aimScale * scale] : [];
    const healthMinX = visibleTargetHealth.value ? [visibleTargetHealth.value.x] : [];
    const healthMinY = visibleTargetHealth.value ? [visibleTargetHealth.value.y] : [];
    const healthScale = visibleTargetHealth.value ? targetHealthScale(visibleTargetHealth.value) : 1;
    const healthMaxX = visibleTargetHealth.value ? [visibleTargetHealth.value.x + TARGET_HEALTH_WIDTH * healthScale * scale] : [];
    const healthMaxY = visibleTargetHealth.value ? [visibleTargetHealth.value.y + TARGET_HEALTH_HEIGHT * healthScale * scale] : [];
    const effectMinX = visibleEffectTimers.value.map((item) => item.x);
    const effectMinY = visibleEffectTimers.value.map((item) => item.y);
    const effectMaxX = visibleEffectTimers.value.map((item) => item.x + effectTimerWidth(item) * effectTimerScale(item) * scale);
    const effectMaxY = visibleEffectTimers.value.map((item) => item.y + effectTimerHeight(item) * effectTimerScale(item) * scale);
    const minX = Math.floor(Math.min(...skillMinX, ...mechanicMinX, ...stackMinX, ...aimMinX, ...healthMinX, ...effectMinX) - scaledMargin);
    const minY = Math.floor(Math.min(...skillMinY, ...mechanicMinY, ...stackMinY, ...aimMinY, ...healthMinY, ...effectMinY) - scaledMargin);
    const maxX = Math.ceil(Math.max(...skillMaxX, ...mechanicMaxX, ...stackMaxX, ...aimMaxX, ...healthMaxX, ...effectMaxX) + scaledMargin);
    const maxY = Math.ceil(Math.max(...skillMaxY, ...mechanicMaxY, ...stackMaxY, ...aimMaxY, ...healthMaxY, ...effectMaxY) + scaledMargin);
    return {
        x: minX,
        y: minY,
        width: Math.max(Math.ceil(120 * scale), maxX - minX),
        height: Math.max(Math.ceil(120 * scale), maxY - minY),
    };
});

function targetHealthPositionStyle(item: TargetHealthBarOverlayItem) {
    const scale = effectiveScale.value;
    const sizeScale = targetHealthScale(item);
    return {
        left: `${(item.x - layoutBounds.value.x) / scale}px`,
        top: `${(item.y - layoutBounds.value.y) / scale}px`,
        width: `${TARGET_HEALTH_WIDTH * sizeScale}px`,
        height: `${TARGET_HEALTH_HEIGHT * sizeScale}px`,
        "--target-health-scale": String(sizeScale),
        opacity: String(Math.min(1, Math.max(.2, Number(item.opacityPercent || 100) / 100))),
    };
}

function effectTimerWidth(item: EffectTimerOverlayItem) {
    return item.orientation === "vertical" ? EFFECT_TIMER_VERTICAL_WIDTH : EFFECT_TIMER_HORIZONTAL_WIDTH;
}

function effectTimerHeight(item: EffectTimerOverlayItem) {
    return item.orientation === "vertical" ? EFFECT_TIMER_VERTICAL_HEIGHT : EFFECT_TIMER_HORIZONTAL_HEIGHT;
}

function effectTimerScale(item: EffectTimerOverlayItem) {
    return Math.min(2, Math.max(.5, (Number(item.scalePercent) || 100) / 100));
}

function effectTimerPositionStyle(item: EffectTimerOverlayItem) {
    const scale = effectiveScale.value;
    const sizeScale = effectTimerScale(item);
    return {
        left: `${(item.x - layoutBounds.value.x) / scale}px`,
        top: `${(item.y - layoutBounds.value.y) / scale}px`,
        width: `${effectTimerWidth(item) * sizeScale}px`,
        height: `${effectTimerHeight(item) * sizeScale}px`,
        "--effect-timer-scale": String(sizeScale),
        opacity: String(Math.min(1, Math.max(.2, Number(item.opacityPercent || 100) / 100))),
    };
}

function effectTimerProgress(item: EffectTimerOverlayItem) {
    if (item.endsAtMs <= item.startedAtMs) return 0;
    return Math.min(1, Math.max(0, (item.endsAtMs - nowMs.value) / (item.endsAtMs - item.startedAtMs)));
}

function effectTimerProgressStyle(item: EffectTimerOverlayItem) {
    const progress = effectTimerProgress(item);
    // Keep healthy timers cool and shift the whole fill through amber to red
    // near expiry. A computed gradient keeps the warning colour visible even
    // while the fill itself is shrinking.
    const hue = Math.round(6 + 158 * Math.pow(progress, .85));
    const startHue = Math.max(0, hue - 8);
    const endHue = Math.min(180, hue + 14);
    const direction = item.orientation === "vertical" ? "0deg" : "90deg";
    return {
        transform: item.orientation === "vertical" ? `scaleY(${progress})` : `scaleX(${progress})`,
        background: `linear-gradient(${direction}, hsl(${startHue} 92% 50%), hsl(${endHue} 90% 66%))`,
        boxShadow: `0 0 10px hsl(${hue} 95% 58% / .72)`,
    };
}

function effectTimerRemainingText(item: EffectTimerOverlayItem) {
    const remaining = Math.max(0, item.endsAtMs - nowMs.value) / 1000;
    return remaining >= 10 ? `${Math.ceil(remaining)}s` : `${(Math.ceil(remaining * 10) / 10).toFixed(1)}s`;
}

function targetHealthScale(item: Pick<TargetHealthBarOverlayItem, "scalePercent">) {
    return Math.min(2, Math.max(.5, (Number(item.scalePercent) || 100) / 100));
}

function aimReminderPositionStyle(item: AimReminderOverlayItem) {
    const scale = effectiveScale.value;
    const sizeScale = aimReminderScale(item);
    return {
        left: `${(item.x - layoutBounds.value.x) / scale}px`,
        top: `${(item.y - layoutBounds.value.y) / scale}px`,
        width: `${AIM_REMINDER_WIDTH * sizeScale}px`,
        height: `${AIM_REMINDER_HEIGHT * sizeScale}px`,
        "--aim-reminder-scale": String(sizeScale),
    };
}

function aimReminderScale(item: Pick<AimReminderOverlayItem, "scalePercent">) {
    return Math.min(2, Math.max(.5, (Number(item.scalePercent) || 100) / 100));
}

function itemPositionStyle(item: SkillCooldownOverlayItem) {
    const scale = effectiveScale.value;
    return {
        left: `${(item.x - layoutBounds.value.x) / scale - ITEM_HORIZONTAL_PADDING}px`,
        top: `${(item.y - layoutBounds.value.y) / scale - ITEM_VERTICAL_PADDING}px`,
    };
}

function mechanicPositionStyle(item: BossMechanicOverlayItem) {
    const scale = effectiveScale.value;
    const sizeScale = mechanicScale(item);
    return {
        left: `${(item.x - layoutBounds.value.x) / scale}px`,
        top: `${(item.y - layoutBounds.value.y) / scale}px`,
        width: `${118 * sizeScale}px`,
        height: `${118 * sizeScale}px`,
        "--boss-mechanic-scale": String(sizeScale),
        "--boss-mechanic-warning-scale": String(sizeScale * 1.045),
    };
}

function stackAlertPositionStyle(item: BuffStackAlertOverlayItem) {
    const scale = effectiveScale.value;
    const sizeScale = mechanicScale(item);
    return {
        left: `${(item.x - layoutBounds.value.x) / scale}px`,
        top: `${(item.y - layoutBounds.value.y) / scale}px`,
        width: `${220 * sizeScale}px`,
        height: `${104 * sizeScale}px`,
        "--boss-mechanic-scale": String(sizeScale),
    };
}

function mechanicScale(item: Pick<BossMechanicOverlayItem, "scalePercent">) {
    return Math.min(2, Math.max(.5, (Number(item.scalePercent) || 100) / 100));
}

function mechanicRemainingText(item: BossMechanicOverlayItem) {
    const remaining = Math.max(0, item.endsAtMs - nowMs.value) / 1000;
    return remaining >= 10 ? String(Math.ceil(remaining)) : (Math.ceil(remaining * 10) / 10).toFixed(1);
}

function mechanicProgressStyle(item: BossMechanicOverlayItem) {
    const total = Math.max(1, item.endsAtMs - item.startedAtMs);
    const remaining = Math.max(0, item.endsAtMs - nowMs.value);
    return { transform: `scaleX(${Math.min(1, remaining / total)})` };
}

function mechanicAlertClass(item: BossMechanicOverlayItem) {
    const remaining = Math.max(0, item.endsAtMs - nowMs.value) / 1000;
    const orbEarlyWarning = item.key === "miel-orb" && remaining <= 6.5 && remaining > 1.5;
    const finalWarning = item.key === "miel-orb"
        ? remaining <= 1.5
        : item.key === "miel-laser" && remaining <= 1;
    return {
        "warning-phase": orbEarlyWarning,
        "danger-phase": finalWarning,
    };
}

function isCooling(item: SkillCooldownOverlayItem) {
    if (isProgressItem(item)) return false;
    return item.usedAtMs > 0 && item.readyAtMs > nowMs.value;
}

function isCumulativeAccumulating(item: SkillCooldownOverlayItem) {
    return item.cooldownPhase === "accumulating" && Number(item.cumulativeCooldownSeconds) > 0;
}

function cumulativeCooldownText(item: SkillCooldownOverlayItem) {
    const accumulated = Math.round((Number(item.accumulatedCooldownSeconds) || 0) * 10) / 10;
    const limit = Math.round((Number(item.cumulativeCooldownSeconds) || 0) * 10) / 10;
    return `累计 ${accumulated}/${limit}s`;
}

function isProgressItem(item: SkillCooldownOverlayItem) {
    return typeof item.progressPercent === "number" || typeof item.progressObserved === "boolean";
}

function aimReminderProgress(item: AimReminderOverlayItem) {
    if (!item.active) return 0;
    return calculateMagnumAimDisplayProgress(
        item.startedAtMs,
        item.readyAtMs,
        nowMs.value,
        item.calibrationPercent,
    );
}

function isAimNearReady(item: AimReminderOverlayItem) {
    return aimReminderProgress(item) >= 0.78 && !isAimReady(item);
}

function isAimReady(item: AimReminderOverlayItem) {
    return aimReminderProgress(item) >= MAGNUM_BEST_SHOT_PROGRESS;
}

function isAimFull(item: AimReminderOverlayItem) {
    return aimReminderProgress(item) >= 1;
}

function aimReminderProgressStyle(item: AimReminderOverlayItem) {
    return { transform: `scaleX(${aimReminderProgress(item)})` };
}

function aimReminderProgressText(item: AimReminderOverlayItem) {
    return `${Math.floor(aimReminderProgress(item) * 100)}%`;
}

function aimReminderBuffText(item: AimReminderOverlayItem) {
    if (!item.active) return "等待穿心锁定目标";
    const names = Array.isArray(item.buffNames) ? item.buffNames.filter(Boolean) : [];
    const multiplier = Number(item.speedMultiplier) || 1;
    return `${names.length ? names.join(" + ") : "无临时瞄速 Buff"} · ×${Number.isInteger(multiplier) ? multiplier : multiplier.toFixed(1)}`;
}

function toahProgressText(item: SkillCooldownOverlayItem) {
    if (!item.progressObserved) return "--";
    return `${toahSpiritDisplayPercent(item.progressPercent)}%`;
}

function toahProgressMaskStyle(item: SkillCooldownOverlayItem) {
    const percent = item.progressObserved
        ? Math.min(100, Math.max(0, Number(item.progressPercent) || 0))
        : 0;
    return { height: `${100 - percent}%` };
}

function isReadyBurst(item: SkillCooldownOverlayItem) {
    if (isCumulativeAccumulating(item)) return false;
    if (item.readyAtMs <= 0) return false;
    const elapsed = nowMs.value - item.readyAtMs;
    return elapsed >= 0 && elapsed < READY_ANIMATION_MS;
}

function remainingText(item: SkillCooldownOverlayItem) {
    const remaining = Math.max(0, item.readyAtMs - nowMs.value) / 1000;
    if (remaining >= 10) return `${Math.ceil(remaining)}s`;
    return `${Math.ceil(remaining * 10) / 10}s`;
}

function cooldownSweepStyle(item: SkillCooldownOverlayItem) {
    const totalMs = Math.max(1, item.readyAtMs - item.usedAtMs);
    const elapsedMs = Math.min(totalMs, Math.max(0, nowMs.value - item.usedAtMs));
    const progressDegrees = Math.round((elapsedMs / totalMs) * 3600) / 10;
    return { "--cooldown-progress": `${progressDegrees}deg` };
}

function hideImage(event: Event) {
    (event.currentTarget as HTMLImageElement).style.display = "none";
}

function applyMessage(message: SkillCooldownOverlayMessage) {
    if (message.type !== "skill-cooldown-state" || message.atMs < lastMessageAt) return;
    lastMessageAt = message.atMs;
    items.value = Array.isArray(message.items) ? message.items : [];
    aimReminder.value = message.aimReminder?.active || message.aimReminder?.alwaysVisible
        ? message.aimReminder
        : undefined;
    targetHealth.value = message.targetHealth;
    effectTimers.value = Array.isArray(message.effectTimers) ? message.effectTimers : [];
    scheduleAimBestPoint(aimReminder.value);
    mechanics.value = Array.isArray(message.mechanics) ? message.mechanics : [];
    stackAlerts.value = Array.isArray(message.stackAlerts) ? message.stackAlerts : [];
    settings.iconSize = Math.min(96, Math.max(24, Number(message.settings?.iconSize) || 48));
    syncNativeVisibility();
}

function scheduleAimBestPoint(item: AimReminderOverlayItem | undefined) {
    const key = item?.active
        ? `${item.generation}:${item.startedAtMs}:${item.readyAtMs}`
        : "";
    if (key === scheduledAimBestPointKey) return;
    scheduledAimBestPointKey = key;
    if (aimBestPointTimer !== undefined) {
        window.clearTimeout(aimBestPointTimer);
        aimBestPointTimer = undefined;
    }
    if (!item?.active) return;
    const bestAtMs = Number(item.readyAtMs);
    if (!Number.isFinite(bestAtMs) || bestAtMs <= 0) return;
    const delayMs = bestAtMs - Date.now();
    if (delayMs <= 0) {
        // Even if packet/render delivery is late, expose one exact 85% frame
        // so the burst animation begins from the intended threshold.
        nowMs.value = bestAtMs;
        return;
    }
    aimBestPointTimer = window.setTimeout(() => {
        // Pin one render exactly to the configured best-shot instant. Fast
        // aim buffs can finish inside one ordinary 50ms clock interval; this
        // prevents the charge burst from visually starting at 100% instead.
        nowMs.value = bestAtMs;
        aimBestPointTimer = undefined;
        syncNativeVisibility();
    }, delayMs);
}

async function pollState(atMs = Date.now(), force = false) {
    if (!force && atMs - lastOverlayPollAtMs < 175) return;
    lastOverlayPollAtMs = atMs;
    refreshOverlayAppearance();
    try {
        const response = await fetch("/api/skill_overlay/state", { cache: "no-store" });
        if (!response.ok) return;
        applyMessage(await response.json() as SkillCooldownOverlayMessage);
    } catch {
        // The desktop host may be closing.
    }
}

function syncNativeVisibility() {
    const regularActive = overlayAppearance.enabled && (Boolean(visibleAimReminder.value) || visibleEffectTimers.value.length > 0 || visibleItems.value.length > 0 || visibleMechanics.value.length > 0 || visibleStackAlerts.value.length > 0);
    const active = regularActive || Boolean(visibleTargetHealth.value);
    const bounds = layoutBounds.value;
    const stateKey = `${active}:${settings.iconSize}:${effectiveScalePercent.value}:${bounds.x}:${bounds.y}:${bounds.width}:${bounds.height}`;
    if (nativeStateKey === stateKey) return;
    nativeStateKey = stateKey;
    const clockSequence = Math.round((performance.timeOrigin + performance.now()) * 1000);
    nativeStateSequence = Math.max(nativeStateSequence + 1, clockSequence);
    void fetch("/api/skill_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active, ...bounds, sequence: nativeStateSequence }),
    }).catch(() => undefined);
}

function refreshOverlayAppearance() {
    const next = loadBuffOverlaySettings();
    Object.assign(overlayThemeStyle, uiColorThemeStyle(loadUiColorTheme()));
    if (
        overlayAppearance.enabled === next.overlayEnabled
        && overlayAppearance.opacity === next.opacity
        && overlayAppearance.dpiPercent === next.dpiPercent
    ) return;
    overlayAppearance.enabled = next.overlayEnabled;
    overlayAppearance.opacity = next.opacity;
    overlayAppearance.dpiPercent = next.dpiPercent;
    syncNativeVisibility();
}

onMounted(() => {
    document.documentElement.classList.add("skill-overlay-page");
    try {
        channel = new BroadcastChannel(SKILL_COOLDOWN_CHANNEL);
        channel.onmessage = (event: MessageEvent<SkillCooldownOverlayMessage>) => {
            if (event.data?.type === "skill-cooldown-state") applyMessage(event.data);
        };
    } catch {
        channel = undefined;
    }
    void pollState(Date.now(), true);
    pollTimer = window.setInterval(() => void pollState(Date.now()), 200);
    clockTimer = window.setInterval(() => {
        nowMs.value = Date.now();
        observedDevicePixelRatio.value = window.devicePixelRatio;
        syncNativeVisibility();
    }, 50);
    nativeClockTickListener = (() => {
        const currentMs = Date.now();
        nowMs.value = currentMs;
        observedDevicePixelRatio.value = window.devicePixelRatio;
        void pollState(currentMs);
        syncNativeVisibility();
    }) as EventListener;
    window.addEventListener("dilmeter-native-tick", nativeClockTickListener);
});

onUnmounted(() => {
    nativeStateKey = "false:0:0";
    const clockSequence = Math.round((performance.timeOrigin + performance.now()) * 1000);
    nativeStateSequence = Math.max(nativeStateSequence + 1, clockSequence);
    void fetch("/api/skill_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active: false, sequence: nativeStateSequence }),
    }).catch(() => undefined);
    document.documentElement.classList.remove("skill-overlay-page");
    channel?.close();
    if (clockTimer !== undefined) window.clearInterval(clockTimer);
    if (pollTimer !== undefined) window.clearInterval(pollTimer);
    if (aimBestPointTimer !== undefined) window.clearTimeout(aimBestPointTimer);
    if (nativeClockTickListener) {
        window.removeEventListener("dilmeter-native-tick", nativeClockTickListener);
        nativeClockTickListener = undefined;
    }
});
</script>

<style>
html.skill-overlay-page,
html.skill-overlay-page body,
html.skill-overlay-page #app,
html.skill-overlay-page .v-application,
.skill-overlay-app {
    width: 100%;
    height: 100%;
    overflow: hidden !important;
    background: transparent !important;
}
</style>

<style scoped>
.skill-overlay-root {
    position: relative;
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    overflow: hidden;
    color: #ffffff;
    background: transparent;
    font-family: "Microsoft YaHei", sans-serif;
    pointer-events: none;
    user-select: none;
}

.target-health-overlay-list {
    position: absolute;
    z-index: 10;
    inset: 0;
    opacity: var(--overlay-opacity);
}

.target-health-overlay-item {
    position: absolute;
    color: #fff;
}

.target-health-bar {
    position: relative;
    box-sizing: border-box;
    width: 420px;
    height: 38px;
    overflow: hidden;
    color: #fff;
    background: rgba(35, 35, 37, .96);
    border: 2px solid rgba(12, 12, 13, .98);
    box-shadow: 0 2px 5px rgba(0, 0, 0, .72);
    font-size: 17px;
    font-weight: 900;
    line-height: 34px;
    text-shadow: 0 1px 2px #000, 1px 0 2px #000;
    transform: scale(var(--target-health-scale));
    transform-origin: left top;
}

.target-health-fill {
    position: absolute;
    inset: 0 auto 0 0;
    background: linear-gradient(180deg, #d22128 0%, #b80f17 66%, #951017 100%);
    transition: width .12s linear;
}

.target-health-name,
.target-health-percent {
    position: absolute;
    z-index: 1;
    top: 0;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
}

.target-health-name {
    left: 10px;
    right: 98px;
}

.target-health-percent {
    right: 10px;
    width: 88px;
    text-align: right;
}

.effect-timer-overlay-list {
    position: absolute;
    z-index: 9;
    inset: 0;
    opacity: var(--overlay-opacity);
}

.effect-timer-overlay-item {
    position: absolute;
    transform-origin: left top;
}

.effect-timer-card {
    display: flex;
    box-sizing: border-box;
    width: 292px;
    height: 46px;
    gap: 8px;
    padding: 0;
    color: #f4fbff;
    background: none;
    border: 0;
    border-radius: 0;
    box-shadow: none;
    transform: scale(var(--effect-timer-scale));
    transform-origin: left top;
}

.effect-timer-icon {
    position: relative;
    flex: 0 0 46px;
    box-sizing: border-box;
    width: 46px;
    height: 46px;
    overflow: hidden;
    display: grid;
    place-items: center;
    background: rgba(5, 9, 12, .86);
    border: 1px solid rgba(219, 250, 255, .72);
    border-radius: 7px;
}

.effect-timer-icon span {
    color: rgba(214, 248, 255, .84);
    font-size: 13px;
    font-weight: 800;
}

.effect-timer-icon img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.effect-timer-body {
    position: relative;
    display: flex;
    flex-direction: column;
    justify-content: center;
    flex: 1;
    min-width: 0;
    gap: 4px;
}

.effect-timer-body strong {
    overflow: hidden;
    font-size: 15px;
    white-space: nowrap;
    text-overflow: ellipsis;
}

.effect-timer-track {
    position: relative;
    box-sizing: border-box;
    width: 100%;
    height: 20px;
    overflow: hidden;
    background: rgba(2, 7, 10, .78);
    border: 1px solid rgba(196, 239, 249, .56);
    border-radius: 999px;
    box-shadow: 0 2px 8px rgba(0, 0, 0, .48), inset 0 0 6px rgba(0, 0, 0, .62);
}

.effect-timer-track i {
    position: absolute;
    inset: 0;
    display: block;
    width: 100%;
    height: 100%;
    transform-origin: left center;
}

.effect-timer-remaining {
    position: absolute;
    z-index: 1;
    inset: 0;
    display: grid;
    place-items: center;
    color: #fff;
    font-size: 12px;
    font-weight: 900;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    text-shadow: 0 1px 3px #000, 0 0 4px rgba(0, 0, 0, .82);
}

.effect-timer-overlay-item.effect-vertical .effect-timer-card {
    width: 46px;
    height: 174px;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 0;
}

.effect-timer-overlay-item.effect-vertical .effect-timer-body {
    width: 32px;
    min-height: 0;
    align-items: center;
    justify-content: flex-end;
}

.effect-timer-overlay-item.effect-vertical .effect-timer-track {
    width: 32px;
    height: 120px;
}

.effect-timer-overlay-item.effect-vertical .effect-timer-track i {
    transform-origin: center bottom;
}

.effect-timer-overlay-item.effect-vertical .effect-timer-remaining {
    font-size: 10px;
    letter-spacing: -.2px;
}

.skill-overlay-list {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    opacity: var(--overlay-opacity);
    transition: opacity .12s linear;
}

.skill-overlay-item {
    position: absolute;
    display: grid;
    place-items: center;
    width: calc(var(--skill-icon-size) + 24px);
    height: calc(var(--skill-icon-size) + 30px);
}

.skill-icon-shell {
    position: relative;
    z-index: 2;
    box-sizing: border-box;
    width: var(--skill-icon-size);
    height: var(--skill-icon-size);
    overflow: hidden;
    background: rgba(9, 12, 15, .82);
    border: 1px solid rgba(236, 250, 255, .9);
    border-radius: max(3px, calc(var(--skill-icon-size) * .1));
    box-shadow: 0 0 5px rgba(218, 250, 255, .9), 0 0 13px rgba(82, 224, 255, .68);
    transform-origin: center;
}

.skill-icon-shell img {
    position: relative;
    z-index: 1;
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.skill-pet-badge {
    position: absolute;
    z-index: 5;
    top: 1px;
    right: 1px;
    padding: 0 2px;
    color: #fff;
    background: rgba(26, 87, 43, .92);
    border: 1px solid rgba(218, 255, 226, .9);
    border-radius: 2px;
    font-size: max(7px, calc(var(--skill-icon-size) * .18));
    font-weight: 900;
    line-height: 1.2;
    text-shadow: 0 1px 2px #000, 1px 0 2px #000;
}

.skill-icon-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    color: #dffbff;
    background: #26343b;
    font-size: max(7px, calc(var(--skill-icon-size) * .2));
    font-weight: 800;
}

.skill-overlay-item.cooling .skill-icon-shell img {
    filter: brightness(.9) saturate(.92);
    opacity: 1;
}

.skill-overlay-item.persistent:not(.cooling) .skill-icon-shell img {
    filter: brightness(1.08) saturate(1.08);
}

.skill-overlay-item.accumulating .skill-icon-shell {
    border-color: #ffd27b;
    box-shadow: 0 0 5px rgba(255, 218, 137, .9), 0 0 13px rgba(255, 171, 55, .62);
}

.skill-cooldown-sweep {
    position: absolute;
    z-index: 2;
    inset: 0;
    background:
        conic-gradient(
            from -90deg,
            rgba(2, 5, 8, .08) 0deg var(--cooldown-progress),
            rgba(0, 2, 4, .82) var(--cooldown-progress) 360deg
        );
    box-shadow:
        inset 0 0 0 1px rgba(174, 225, 237, .2),
        inset 0 0 12px rgba(0, 0, 0, .74);
    pointer-events: none;
}

.skill-cooldown-sweep::after {
    position: absolute;
    inset: 0;
    border: 1px solid rgba(212, 245, 255, .22);
    box-shadow: inset 0 0 5px rgba(122, 225, 255, .18);
    content: "";
}

.toah-progress-mask {
    position: absolute;
    z-index: 3;
    top: 0;
    right: 0;
    left: 0;
    box-sizing: border-box;
    max-height: 100%;
    background: rgba(126, 128, 132, .93);
    border-bottom: 2px solid rgba(18, 13, 24, .95);
    box-shadow: inset 0 0 8px rgba(0, 0, 0, .34), 0 1px 0 rgba(227, 205, 255, .76);
    transition: height .18s linear;
    pointer-events: none;
}

.progress-full .toah-progress-mask {
    height: 0 !important;
    border-bottom-width: 0;
    box-shadow: none;
}

.progress-tracked:not(.progress-observed) .skill-icon-shell img {
    filter: grayscale(1) brightness(.42);
}

.skill-overlay-item:not(.persistent):not(.ready-burst):not(.progress-tracked) {
    opacity: 0;
}

.aim-reminder-overlay-list {
    position: absolute;
    z-index: 9;
    inset: 0;
    opacity: var(--overlay-opacity);
}

.aim-reminder-overlay-item {
    position: absolute;
    width: 292px;
    height: 64px;
    color: var(--ui-theme-text);
}

.aim-reminder-card {
    position: relative;
    box-sizing: border-box;
    display: grid;
    grid-template-columns: 44px minmax(0, 1fr);
    align-items: center;
    gap: 7px;
    width: 292px;
    height: 64px;
    padding: 3px 2px;
    overflow: visible;
    color: var(--ui-theme-text);
    filter: drop-shadow(0 2px 2px rgba(0, 0, 0, .72));
    transform: scale(var(--aim-reminder-scale));
    transform-origin: left top;
}

.aim-reminder-icon {
    position: relative;
    box-sizing: border-box;
    display: grid;
    place-items: center;
    width: 44px;
    height: 44px;
    overflow: visible;
    color: var(--ui-theme-on-accent);
    background: var(--ui-color-accent);
    border: 1px solid var(--ui-theme-border);
    border-radius: 2px;
    box-shadow: 0 0 5px rgba(var(--ui-color-rgb), .58);
    font-size: 12px;
    font-weight: 900;
}

.aim-reminder-icon > span {
    position: relative;
    z-index: 2;
}

.aim-reminder-icon img {
    position: absolute;
    z-index: 3;
    inset: 0;
    width: 100%;
    height: 100%;
    border-radius: 1px;
    object-fit: cover;
}

.aim-reminder-overlay-item.aim-idle .aim-reminder-card {
    opacity: .72;
    filter: grayscale(.35) drop-shadow(0 2px 2px rgba(0, 0, 0, .72));
}

.aim-reminder-overlay-item.aim-idle .aim-reminder-icon {
    box-shadow: 0 0 4px rgba(var(--ui-color-rgb), .32);
}

.aim-reminder-body {
    position: relative;
    display: grid;
    grid-template-rows: 16px 16px 14px;
    gap: 2px;
    min-width: 0;
}

.aim-reminder-title {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    min-width: 0;
}

.aim-reminder-title strong {
    color: var(--ui-theme-text);
    font-size: 12px;
    font-weight: 900;
    line-height: 15px;
    text-shadow: 0 1px 2px rgba(0, 0, 0, .9);
    white-space: nowrap;
}

.aim-reminder-title span {
    position: absolute;
    left: 85%;
    color: #ffd65c;
    font-size: 10px;
    font-weight: 900;
    line-height: 15px;
    text-shadow: 0 1px 2px #000;
    transform: translateX(-50%);
    white-space: nowrap;
}

.aim-reminder-track {
    position: relative;
    box-sizing: border-box;
    height: 16px;
    overflow: visible;
    background: linear-gradient(90deg, var(--ui-theme-control) 0 85%, rgba(255, 190, 45, .3) 85% 100%);
    border: 1px solid var(--ui-theme-border);
    box-shadow: inset 0 1px 3px rgba(0, 0, 0, .72);
}

.aim-reminder-fill {
    position: absolute;
    z-index: 1;
    top: 1px;
    bottom: 1px;
    left: 1px;
    display: block;
    width: calc(100% - 2px);
    background: linear-gradient(90deg, var(--ui-color-accent) 0, var(--ui-theme-text) 84.5%, #ffd34e 85%, #ff7043 100%);
    box-shadow: 0 0 5px rgba(var(--ui-color-rgb), .84);
    transform-origin: left center;
    transition: transform .045s linear;
}

.aim-reminder-best-marker {
    position: absolute;
    z-index: 2;
    top: -3px;
    bottom: -3px;
    left: 85%;
    width: 2px;
    background: #ffe078;
    box-shadow: 0 0 4px #fff4bd, 0 0 8px #ff9d2e;
    transform: translateX(-1px);
}

.aim-reminder-best-marker::before {
    position: absolute;
    top: -3px;
    left: -3px;
    width: 8px;
    height: 3px;
    background: #fff0a6;
    content: "";
    box-shadow: 0 0 4px #ffb43c;
}

.aim-reminder-footer {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 6px;
    min-width: 0;
}

.aim-reminder-buffs {
    overflow: hidden;
    color: var(--ui-theme-muted);
    font-size: 8px;
    font-weight: 800;
    line-height: 14px;
    text-overflow: ellipsis;
    text-shadow: 0 1px 2px rgba(0, 0, 0, .86);
    white-space: nowrap;
}

.aim-reminder-percent {
    flex: 0 0 auto;
    min-width: 32px;
    color: var(--ui-theme-muted);
    font-family: "Consolas", "Microsoft YaHei", sans-serif;
    font-size: 11px;
    font-weight: 900;
    font-variant-numeric: tabular-nums;
    letter-spacing: 0;
    line-height: 14px;
    text-align: right;
    text-shadow: 0 2px 2px #000, 0 0 4px rgba(var(--ui-color-rgb), .65);
}

.aim-reminder-overlay-item.aim-near-ready .aim-reminder-best-marker {
    filter: brightness(1.2);
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-icon {
    border-color: #fff7c7;
    filter: brightness(1.24) saturate(1.08);
    box-shadow: 0 0 8px #fff8d7, 0 0 16px rgba(255, 212, 86, .9), 0 0 24px rgba(255, 139, 44, .68);
}

.aim-reminder-overlay-item.aim-full .aim-reminder-icon {
    animation: aim-reminder-final-release .64s cubic-bezier(.16, .82, .2, 1) 1 both;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-icon img {
    animation: aim-reminder-icon-charge-burst .78s cubic-bezier(.2, .82, .24, 1) 1 both;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-icon::before,
.aim-reminder-overlay-item.aim-ready .aim-reminder-icon::after {
    position: absolute;
    z-index: 1;
    top: 50%;
    left: 50%;
    box-sizing: border-box;
    width: 58px;
    height: 58px;
    pointer-events: none;
    content: "";
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-icon::before {
    border: 2px solid #fff4b2;
    border-radius: 50%;
    box-shadow: 0 0 7px #ffffff, 0 0 15px #ffc843, inset 0 0 8px rgba(255, 214, 84, .72);
    animation: aim-reminder-icon-charge-ring .78s cubic-bezier(.18, .8, .22, 1) 1 both;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-icon::after {
    width: 78px;
    height: 78px;
    background: repeating-conic-gradient(
        from 0deg,
        rgba(255, 255, 240, .98) 0deg 2deg,
        rgba(255, 191, 48, .82) 2deg 5deg,
        transparent 5deg 45deg
    );
    filter: drop-shadow(0 0 4px #ffffff) drop-shadow(0 0 7px #ffad31);
    animation: aim-reminder-icon-burst-rays .78s cubic-bezier(.16, .78, .22, 1) 1 both;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-percent,
.aim-reminder-overlay-item.aim-ready .aim-reminder-title span {
    color: #ffe06f;
    filter: brightness(1.3);
    text-shadow: 0 1px 2px #000, 0 0 7px #ffffff, 0 0 12px #ffad32;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-track {
    border-color: #ffd45a;
    filter: brightness(1.22) saturate(1.1);
    box-shadow: inset 0 0 3px rgba(255, 255, 255, .42), 0 0 7px #fff9cf, 0 0 13px #ffae32;
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-fill {
    filter: brightness(1.35) saturate(1.12);
}

.aim-reminder-overlay-item.aim-ready .aim-reminder-best-marker {
    filter: brightness(1.8);
    transform: translateX(-1px) scaleY(1.28);
}

.aim-reminder-overlay-item.aim-full .aim-reminder-percent {
    color: #ff8b60;
}

.aim-reminder-overlay-item.aim-full .aim-reminder-track {
    animation: aim-reminder-final-track-flash .64s cubic-bezier(.16, .82, .2, 1) 1 both;
}

.skill-overlay-item.ready-burst:not(.persistent) .skill-icon-shell {
    animation: skill-ready-appear 2.6s cubic-bezier(.16, .76, .24, 1) both;
}

.skill-overlay-item.ready-burst.persistent .skill-icon-shell {
    animation: skill-ready-persistent .92s cubic-bezier(.2, .82, .22, 1) both;
}

.skill-particle-field {
    position: absolute;
    z-index: 3;
    top: 50%;
    left: 50%;
    width: 1px;
    height: 1px;
    pointer-events: none;
}

.skill-particle {
    --dx: 0px;
    --dy: -34px;
    position: absolute;
    top: -9px;
    left: -9px;
    color: #efffff;
    filter: drop-shadow(0 0 5px #36e7ff) drop-shadow(0 0 11px #d8fbff);
    font-size: 18px;
    opacity: 0;
}

.ready-burst .skill-particle {
    animation: skill-particle-flight 1.45s ease-out both;
}

.skill-particle:nth-child(2) { --dx: 32px; --dy: -32px; animation-delay: .06s; color: #fff4b4; }
.skill-particle:nth-child(3) { --dx: 46px; --dy: 0px; animation-delay: .12s; }
.skill-particle:nth-child(4) { --dx: 32px; --dy: 32px; animation-delay: .18s; color: #fff4b4; }
.skill-particle:nth-child(5) { --dx: 0px; --dy: 44px; animation-delay: .24s; }
.skill-particle:nth-child(6) { --dx: -32px; --dy: 32px; animation-delay: .3s; color: #fff4b4; }
.skill-particle:nth-child(7) { --dx: -46px; --dy: 0px; animation-delay: .36s; }
.skill-particle:nth-child(8) { --dx: -32px; --dy: -32px; animation-delay: .42s; color: #fff4b4; }

.skill-cooldown-countdown {
    position: absolute;
    z-index: 4;
    top: calc(50% + var(--skill-icon-size) / 2 + 3px);
    left: 50%;
    min-width: var(--skill-icon-size);
    transform: translateX(-50%);
    color: #ffffff;
    font-size: max(10px, calc(var(--skill-icon-size) * .24));
    font-weight: 800;
    line-height: 16px;
    text-align: center;
    text-shadow:
        -2px -2px 0 #000000,
        2px -2px 0 #000000,
        -2px 2px 0 #000000,
        2px 2px 0 #000000,
        0 0 3px #000000;
}

.cumulative-cooldown-label {
    color: #ffe2a4;
    font-size: max(7px, calc(var(--skill-icon-size) * .16));
    white-space: nowrap;
}

.toah-progress-label {
    font-variant-numeric: tabular-nums;
    letter-spacing: -.3px;
}

.boss-mechanic-overlay-list {
    position: absolute;
    inset: 0;
    opacity: var(--overlay-opacity);
}

.boss-mechanic-overlay-item {
    position: absolute;
    transform-origin: left top;
    animation: boss-mechanic-enter .34s cubic-bezier(.2, .85, .28, 1) both;
}

.boss-mechanic-card {
    position: relative;
    box-sizing: border-box;
    display: grid;
    justify-items: center;
    grid-template-rows: 30px 1fr;
    align-items: center;
    width: 118px;
    height: 118px;
    padding: 8px 9px 11px;
    color: #fffef7;
    background: rgba(8, 10, 12, .14);
    border: 2px solid rgba(255, 226, 139, .98);
    box-shadow: 0 0 10px rgba(255, 243, 196, .78), 0 0 25px rgba(255, 112, 43, .72), inset 0 0 6px rgba(0, 0, 0, .12);
    transform: scale(var(--boss-mechanic-scale));
    transform-origin: left top;
}

.boss-mechanic-icon {
    color: #fff3ae;
    font-size: 27px;
    filter: drop-shadow(0 0 5px rgba(255, 147, 54, .92));
}

.boss-mechanic-overlay-item strong {
    align-self: start;
    margin-top: -4px;
    color: #ffffff;
    font-size: 54px;
    font-weight: 950;
    font-variant-numeric: tabular-nums;
    letter-spacing: -2px;
    line-height: 65px;
    -webkit-text-stroke: 1.4px rgba(24, 12, 3, .86);
    text-shadow: 0 0 7px #ffffff, 0 0 15px rgba(255, 171, 65, 1), 0 2px 2px rgba(0, 0, 0, .62);
}

.boss-mechanic-overlay-item.warning-phase .boss-mechanic-card,
.boss-mechanic-overlay-item.danger-phase .boss-mechanic-card {
    border-color: #fff2ec;
    box-shadow: 0 0 14px #ffffff, 0 0 34px rgba(255, 41, 26, 1), inset 0 0 10px rgba(255, 30, 20, .22);
    animation: boss-mechanic-warning .34s ease-in-out infinite alternate;
}

.boss-mechanic-overlay-item.warning-phase strong,
.boss-mechanic-overlay-item.danger-phase strong,
.boss-mechanic-overlay-item.warning-phase .boss-mechanic-icon,
.boss-mechanic-overlay-item.danger-phase .boss-mechanic-icon {
    color: #fff8f7;
    filter: drop-shadow(0 0 4px #ffffff) drop-shadow(0 0 12px #ff2418);
}

.boss-mechanic-overlay-item.warning-phase .boss-mechanic-progress,
.boss-mechanic-overlay-item.danger-phase .boss-mechanic-progress {
    background: #ff3528;
    box-shadow: 0 0 12px #ff1a0d;
}

.boss-mechanic-progress {
    position: absolute;
    right: 9px;
    bottom: 7px;
    left: 9px;
    height: 3px;
    background: #ffb34f;
    box-shadow: 0 0 7px rgba(255, 118, 51, .88);
    transform-origin: left center;
}

.buff-stack-overlay-list {
    position: absolute;
    inset: 0;
    opacity: var(--overlay-opacity);
}

.buff-stack-overlay-item {
    position: absolute;
    transform-origin: left top;
    animation: buff-stack-alert 3s cubic-bezier(.2, .86, .25, 1) both;
}

.buff-stack-card {
    box-sizing: border-box;
    display: grid;
    grid-template-columns: 1fr auto;
    grid-template-rows: auto 1fr;
    align-items: end;
    width: 220px;
    min-height: 96px;
    padding: 10px 16px;
    color: #ffffff;
    background: rgba(8, 12, 18, .58);
    border: 1px solid rgba(112, 213, 255, .86);
    box-shadow: 0 0 22px rgba(69, 192, 255, .48), inset 0 0 20px rgba(0, 0, 0, .62);
    transform: scale(var(--boss-mechanic-scale));
    transform-origin: left top;
}

.buff-stack-overlay-item.persistent {
    animation: boss-mechanic-enter .28s cubic-bezier(.2, .86, .25, 1) both;
}

.buff-stack-card span {
    grid-column: 1 / 3;
    overflow: hidden;
    color: #c9f1ff;
    font-size: 15px;
    font-weight: 800;
    text-overflow: ellipsis;
    text-shadow: 0 2px 2px #000000;
    white-space: nowrap;
}

.buff-stack-card strong {
    justify-self: end;
    font-size: 48px;
    line-height: 58px;
    -webkit-text-stroke: 2px #000000;
    text-shadow: 0 0 10px rgba(75, 202, 255, .9), 0 3px 3px #000000;
}

.buff-stack-card small {
    align-self: end;
    margin: 0 0 9px 5px;
    color: #dff7ff;
    font-size: 16px;
    font-weight: 800;
    text-shadow: 0 2px 2px #000000;
}

@keyframes boss-mechanic-enter {
    0% { opacity: 0; transform: scale(.72); }
    70% { opacity: 1; transform: scale(1.06); }
    100% { opacity: 1; transform: scale(1); }
}

@keyframes boss-mechanic-warning {
    /* Preserve the user's configured card size while pulsing.  The old
       literal 1 -> 1.045 animation reset a 50-95% card to 100%, so its
       countdown visibly escaped the allocated item bounds. */
    from { filter: brightness(1.05); transform: scale(var(--boss-mechanic-scale)); }
    to { filter: brightness(1.62); transform: scale(var(--boss-mechanic-warning-scale)); }
}

@keyframes buff-stack-alert {
    0% { opacity: 0; transform: scale(.68); filter: brightness(1.8); }
    16% { opacity: 1; transform: scale(1.08); filter: brightness(1.28); }
    25% { transform: scale(1); filter: brightness(1); }
    82% { opacity: 1; transform: scale(1); }
    100% { opacity: 0; transform: scale(1.12); }
}

@keyframes skill-ready-appear {
    0% { opacity: 0; transform: scale(.45); filter: brightness(1.8); }
    22% { opacity: 1; transform: scale(1.22); filter: brightness(1.45); }
    55% { opacity: 1; transform: scale(1.02); filter: brightness(1); }
    100% { opacity: 0; transform: scale(1.38); filter: brightness(1.18); }
}

@keyframes skill-ready-persistent {
    0% {
        opacity: .62;
        transform: scale(.82);
        filter: brightness(1.9) saturate(1.25);
        box-shadow: 0 0 3px rgba(255, 255, 255, .9), 0 0 8px rgba(90, 226, 255, .72);
    }
    30% {
        opacity: 1;
        transform: scale(1.25);
        filter: brightness(1.48) saturate(1.18);
        box-shadow: 0 0 9px rgba(255, 255, 255, 1), 0 0 25px rgba(83, 226, 255, .95);
    }
    52% {
        transform: scale(.93);
        filter: brightness(1.06) saturate(1.08);
    }
    72% {
        transform: scale(1.08);
        box-shadow: 0 0 7px rgba(230, 253, 255, .95), 0 0 18px rgba(83, 226, 255, .78);
    }
    100% {
        opacity: 1;
        transform: scale(1);
        filter: brightness(1) saturate(1);
        box-shadow: 0 0 5px rgba(218, 250, 255, .9), 0 0 13px rgba(82, 224, 255, .68);
    }
}

@keyframes skill-particle-flight {
    0% { opacity: 0; transform: translate(0, 0) scale(.2) rotate(0deg); }
    24% { opacity: 1; }
    100% { opacity: 0; transform: translate(var(--dx), var(--dy)) scale(1.15) rotate(95deg); }
}

@keyframes aim-reminder-marker-pulse {
    from { filter: brightness(1); transform: translateX(-1px) scaleY(1); }
    to { filter: brightness(1.65); transform: translateX(-1px) scaleY(1.28); }
}

@keyframes aim-reminder-best-icon-glow {
    from {
        filter: brightness(1.08);
        box-shadow: 0 0 5px #fff8d7, 0 0 11px rgba(255, 212, 86, .86), 0 0 17px rgba(255, 139, 44, .58);
    }
    to {
        filter: brightness(1.48) saturate(1.16);
        box-shadow: 0 0 9px #ffffff, 0 0 20px #ffd45a, 0 0 32px rgba(255, 112, 38, .92);
    }
}

@keyframes aim-reminder-final-release {
    0% {
        filter: brightness(1.24) saturate(1.08);
        box-shadow: 0 0 8px #fff8d7, 0 0 16px rgba(255, 212, 86, .9), 0 0 24px rgba(255, 139, 44, .68);
        transform: scale(1);
    }
    28% {
        filter: brightness(1.92) saturate(1.2);
        box-shadow: 0 0 12px #ffffff, 0 0 25px #ffd45a, 0 0 38px rgba(255, 104, 34, .96);
        transform: scale(.88);
    }
    54% {
        filter: brightness(1.58) saturate(1.16);
        box-shadow: 0 0 11px #ffffff, 0 0 22px #ffbc42, 0 0 34px rgba(255, 94, 36, .9);
        transform: scale(1.2);
    }
    76% {
        filter: brightness(1.32) saturate(1.1);
        transform: scale(.97);
    }
    100% {
        filter: brightness(1.24) saturate(1.08);
        box-shadow: 0 0 8px #fff8d7, 0 0 16px rgba(255, 212, 86, .9), 0 0 24px rgba(255, 139, 44, .68);
        transform: scale(1);
    }
}

@keyframes aim-reminder-final-track-flash {
    0% { filter: brightness(1.22) saturate(1.1); }
    42% {
        filter: brightness(1.72) saturate(1.22);
        box-shadow: inset 0 0 4px rgba(255, 255, 255, .72), 0 0 10px #ffffff, 0 0 20px #ff8b37;
    }
    100% {
        filter: brightness(1.22) saturate(1.1);
        box-shadow: inset 0 0 3px rgba(255, 255, 255, .42), 0 0 7px #fff9cf, 0 0 13px #ffae32;
    }
}

@keyframes aim-reminder-icon-charge-burst {
    0% { filter: brightness(.92) saturate(.96); transform: scale(.92); }
    28% { filter: brightness(1.12) saturate(1.06); transform: scale(.86); }
    48% { filter: brightness(1.82) saturate(1.18); transform: scale(1.13); }
    66% { filter: brightness(1.28) saturate(1.1); transform: scale(1.02); }
    100% { filter: brightness(1.04); transform: scale(1); }
}

@keyframes aim-reminder-icon-charge-ring {
    0% { opacity: 0; transform: translate(-50%, -50%) scale(.48); }
    26% { opacity: .92; transform: translate(-50%, -50%) scale(.72); }
    48% { opacity: 1; transform: translate(-50%, -50%) scale(.96); }
    100% { opacity: 0; transform: translate(-50%, -50%) scale(1.55); }
}

@keyframes aim-reminder-icon-burst-rays {
    0%, 30% { opacity: 0; transform: translate(-50%, -50%) rotate(0deg) scale(.32); }
    48% { opacity: 1; transform: translate(-50%, -50%) rotate(8deg) scale(.82); }
    72% { opacity: .66; transform: translate(-50%, -50%) rotate(18deg) scale(1.08); }
    100% { opacity: 0; transform: translate(-50%, -50%) rotate(27deg) scale(1.42); }
}

@keyframes aim-reminder-best-track-glow {
    from {
        filter: brightness(1.08);
        box-shadow: inset 0 1px 3px rgba(0, 0, 0, .72), 0 0 6px rgba(255, 197, 58, .82);
    }
    to {
        filter: brightness(1.48) saturate(1.15);
        box-shadow: inset 0 0 3px rgba(255, 255, 255, .58), 0 0 8px #fff9cf, 0 0 17px #ffae32;
    }
}

@keyframes aim-reminder-best-text-glow {
    from { filter: brightness(1.05); text-shadow: 0 1px 2px #000, 0 0 5px #ff9d2e; }
    to { filter: brightness(1.55); text-shadow: 0 1px 2px #000, 0 0 7px #ffffff, 0 0 13px #ffad32; }
}

@keyframes aim-reminder-best-marker-glow {
    from { filter: brightness(1.2); transform: translateX(-1px) scaleY(1.05); }
    to { filter: brightness(2); transform: translateX(-1px) scaleY(1.45); }
}
</style>
