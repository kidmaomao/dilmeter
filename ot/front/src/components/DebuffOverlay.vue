<template>
    <main
        class="debuff-overlay-root"
        :style="{
            '--debuff-icon-size': `${settings.iconSize}px`,
            '--overlay-opacity': String(overlayAppearance.opacity / 100),
        }"
    >
        <section
            v-if="activeTargetHealth || (activeBoss && visibleItems.length)"
            class="overlay-debuff-list"
            aria-label="目标血量与 Boss Debuff 提醒"
        >
            <div
                v-if="activeTargetHealth"
                class="target-health-bar"
                role="meter"
                aria-label="选中目标血量"
                :aria-valuenow="targetHealthPercent"
                aria-valuemin="0"
                aria-valuemax="100"
                :title="targetHealthTitle"
            >
                <span class="target-health-fill" :style="{ width: `${targetHealthPercent}%` }"></span>
                <strong class="target-health-name">{{ activeTargetHealth.name }}</strong>
                <span class="target-health-percent">{{ targetHealthPercent.toFixed(2) }} %</span>
            </div>
            <div v-if="activeBoss && visibleItems.length" class="overlay-boss-arrival" role="status">{{ activeBoss.name }}</div>
            <div v-if="activeBoss && visibleItems.length" class="overlay-debuff-items">
                <article
                    v-for="item in visibleItems"
                    :key="`${item.ccId}-${item.appliedAt}-${item.state}`"
                    class="overlay-debuff"
                    :class="{ flashing: item.state === 'expiring', missing: item.state === 'missing' }"
                    :title="`${item.name}（CC ${item.ccId}）`"
                >
                    <div class="overlay-icon-ring">
                        <span class="overlay-icon-fallback">{{ item.ccId }}</span>
                        <img :src="item.iconUrl" :alt="`${item.name}图标`" @load="showImage" @error="retryImage" />
                    </div>
                    <span v-if="item.state === 'expiring' && remainingSeconds(item) !== null" class="overlay-countdown">
                        {{ displaySeconds(item) }}
                    </span>
                </article>
            </div>

        </section>
    </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import { loadBuffOverlaySettings, resolveOverlayDpiPercent } from "@/buffAlert";
import {
    DEBUFF_OVERLAY_CHANNEL,
    loadDebuffAlertSettings,
    type DebuffOverlayBoss,
    type DebuffOverlayItem,
    type DebuffOverlayMessage,
    type TargetHealthOverlayItem,
} from "@/debuffAlert";

const settings = reactive(loadDebuffAlertSettings());
const initialOverlayAppearance = loadBuffOverlaySettings();
const overlayAppearance = reactive({ opacity: initialOverlayAppearance.opacity });
const items = ref<DebuffOverlayItem[]>([]);
const boss = ref<DebuffOverlayBoss | null>(null);
const targetHealth = ref<TargetHealthOverlayItem | null>(null);
const now = ref(Date.now() / 1000);
let clockTimer: number | undefined;
let statePollTimer: number | undefined;
let nativeClockTickListener: EventListener | undefined;
let lastOverlayPollAtMs = 0;
let channel: BroadcastChannel | undefined;
let nativeStateKey = "";
let lastMessageAt = 0;

const visibleItems = computed(() => settings.overlayEnabled ? items.value.filter((item) => {
    if (item.state === "missing") return true;
    const remaining = remainingSeconds(item);
    return remaining !== null && remaining > 0;
}) : []);
const activeBoss = computed(() => settings.overlayEnabled ? boss.value : null);
const activeTargetHealth = computed(() => {
    const target = settings.overlayEnabled ? targetHealth.value : null;
    if (!target || !Number.isFinite(target.currentHealth) || !Number.isFinite(target.maximumHealth) || target.maximumHealth <= 0) return null;
    return target;
});
const targetHealthPercent = computed(() => {
    const target = activeTargetHealth.value;
    return target ? Math.max(0, Math.min(100, target.currentHealth / target.maximumHealth * 100)) : 0;
});
const targetHealthTitle = computed(() => {
    const target = activeTargetHealth.value;
    if (!target) return "";
    return `${target.name}：${Math.max(0, target.currentHealth).toLocaleString("zh-CN")} / ${target.maximumHealth.toLocaleString("zh-CN")}`;
});

function remainingSeconds(item: DebuffOverlayItem): number | null {
    if (item.expiresAt === null) return null;
    return Math.max(0, item.expiresAt - now.value);
}

function displaySeconds(item: DebuffOverlayItem) {
    const remaining = remainingSeconds(item);
    return remaining === null ? "" : Math.ceil(remaining);
}

function syncNativeVisibility() {
    const hasBoss = activeBoss.value !== null && visibleItems.value.length > 0;
    const hasHealth = activeTargetHealth.value !== null;
    const active = hasBoss || hasHealth;
    const itemCount = visibleItems.value.length;
    const iconSize = settings.iconSize;
    const scalePercent = resolveOverlayDpiPercent(0, window.devicePixelRatio);
    const stateKey = `${active}:${activeBoss.value?.entityId ?? ""}:${activeTargetHealth.value?.entityId ?? ""}:${itemCount}:${iconSize}:${scalePercent}`;
    if (nativeStateKey === stateKey) return;
    nativeStateKey = stateKey;
    void fetch("/api/debuff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active, itemCount, iconSize, scalePercent, hasBoss, hasHealth }),
    }).catch(() => undefined);
}

function showImage(event: Event) {
    const image = event.currentTarget as HTMLImageElement;
    image.style.display = "";
    image.dataset.retryCount = "0";
}

function retryImage(event: Event) {
    const image = event.currentTarget as HTMLImageElement;
    const retryCount = Math.max(0, Number(image.dataset.retryCount) || 0);
    if (retryCount >= 2) {
        image.style.display = "none";
        return;
    }
    image.style.display = "none";
    image.dataset.retryCount = String(retryCount + 1);
    const retryUrl = new URL(image.src, window.location.href);
    retryUrl.searchParams.set("retry", String(Date.now()));
    window.setTimeout(() => {
        image.src = retryUrl.toString();
    }, retryCount === 0 ? 750 : 2000);
}

function reloadSettings() {
    const next = loadDebuffAlertSettings();
    settings.overlayEnabled = next.overlayEnabled;
    settings.iconSize = next.iconSize;
    settings.volume = next.volume;
    settings.firstRoundGraceSeconds = next.firstRoundGraceSeconds;
    settings.rules = next.rules;
    syncNativeVisibility();
}

function applyOverlayMessage(message: DebuffOverlayMessage) {
    if (message.type !== "debuff-state" || message.at < lastMessageAt) return;
    lastMessageAt = message.at;
    boss.value = message.boss ?? null;
    targetHealth.value = message.targetHealth ?? null;
    items.value = Array.isArray(message.items) ? message.items : [];
    settings.iconSize = Math.min(80, Math.max(16, Number(message.settings?.iconSize) || 30));
    settings.overlayEnabled = message.settings?.overlayEnabled !== false;
    syncNativeVisibility();
}

async function pollOverlayState(atMs = Date.now(), force = false) {
    if (!force && atMs - lastOverlayPollAtMs < 175) return;
    lastOverlayPollAtMs = atMs;
    reloadSettings();
    refreshOverlayAppearance();
    try {
        const response = await fetch("/api/debuff_overlay/state", { cache: "no-store" });
        if (!response.ok) return;
        applyOverlayMessage(await response.json() as DebuffOverlayMessage);
    } catch {
        // The native host may be closing.
    }
}

function refreshOverlayAppearance() {
    const next = loadBuffOverlaySettings();
    if (overlayAppearance.opacity === next.opacity) return;
    overlayAppearance.opacity = next.opacity;
}

onMounted(() => {
    document.documentElement.classList.add("debuff-overlay-page");
    try {
        channel = new BroadcastChannel(DEBUFF_OVERLAY_CHANNEL);
        channel.onmessage = (event: MessageEvent<DebuffOverlayMessage>) => {
            if (event.data?.type === "debuff-state") applyOverlayMessage(event.data);
        };
    } catch {
        channel = undefined;
    }
    void pollOverlayState(Date.now(), true);
    statePollTimer = window.setInterval(() => void pollOverlayState(Date.now()), 250);
    clockTimer = window.setInterval(() => {
        now.value = Date.now() / 1000;
        syncNativeVisibility();
    }, 200);
    nativeClockTickListener = (() => {
        const currentMs = Date.now();
        now.value = currentMs / 1000;
        void pollOverlayState(currentMs);
        syncNativeVisibility();
    }) as EventListener;
    window.addEventListener("dilmeter-native-tick", nativeClockTickListener);
    window.addEventListener("storage", reloadSettings);
    window.addEventListener("dilmeter-debuff-alert-settings", reloadSettings as EventListener);
    window.addEventListener("dilmeter-buff-alert-settings", refreshOverlayAppearance as EventListener);
});

onUnmounted(() => {
    nativeStateKey = "false:0:0:0";
    void fetch("/api/debuff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active: false }),
    }).catch(() => undefined);
    document.documentElement.classList.remove("debuff-overlay-page");
    channel?.close();
    if (clockTimer !== undefined) window.clearInterval(clockTimer);
    if (statePollTimer !== undefined) window.clearInterval(statePollTimer);
    if (nativeClockTickListener) {
        window.removeEventListener("dilmeter-native-tick", nativeClockTickListener);
        nativeClockTickListener = undefined;
    }
    window.removeEventListener("storage", reloadSettings);
    window.removeEventListener("dilmeter-debuff-alert-settings", reloadSettings as EventListener);
    window.removeEventListener("dilmeter-buff-alert-settings", refreshOverlayAppearance as EventListener);
});
</script>

<style>
html.debuff-overlay-page,
html.debuff-overlay-page body,
html.debuff-overlay-page #app,
html.debuff-overlay-page .v-application,
.debuff-overlay-app {
    width: 100%;
    height: 100%;
    overflow: hidden !important;
    background: transparent !important;
}
</style>

<style scoped>
.debuff-overlay-root {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    padding: 8px;
    background: transparent;
    font-family: "Microsoft YaHei", sans-serif;
    user-select: none;
    pointer-events: none;
}

.overlay-debuff-list {
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 4px;
    width: max-content;
    max-width: 100%;
    min-width: 20px;
    height: auto;
    pointer-events: none;
    opacity: var(--overlay-opacity);
    transition: opacity .12s linear;
}

.target-health-bar {
    position: relative;
    box-sizing: border-box;
    width: 420px;
    max-width: 100%;
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

.overlay-boss-arrival {
    box-sizing: border-box;
    min-width: 210px;
    max-width: 420px;
    height: 22px;
    overflow: hidden;
    color: #fff3b0;
    font-size: 14px;
    font-weight: 900;
    line-height: 22px;
    white-space: nowrap;
    text-overflow: ellipsis;
    text-shadow:
        -1px -1px 0 #000,
        1px -1px 0 #000,
        -1px 1px 0 #000,
        1px 1px 0 #000,
        0 0 5px rgba(255, 185, 48, .95);
}

.overlay-debuff-items {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    width: max-content;
    height: calc(var(--debuff-icon-size) + 18px);
    pointer-events: none;
}

.overlay-debuff {
    position: relative;
    flex: 0 0 var(--debuff-icon-size);
    width: var(--debuff-icon-size);
    min-width: var(--debuff-icon-size);
    max-width: var(--debuff-icon-size);
    height: calc(var(--debuff-icon-size) + 18px);
}

.overlay-icon-ring {
    position: relative;
    box-sizing: border-box;
    width: var(--debuff-icon-size);
    height: var(--debuff-icon-size);
    overflow: hidden;
    background: rgba(24, 8, 7, .62);
    border: 1px solid #ff8068;
    border-radius: max(3px, calc(var(--debuff-icon-size) * .16));
    box-shadow: 0 0 4px #ff3d24, 0 0 10px rgba(255, 54, 30, .78);
}

.overlay-debuff.expiring .overlay-icon-ring {
    border-color: #ffd16b;
    box-shadow: 0 0 4px #ffb52e, 0 0 10px rgba(255, 180, 42, .72);
}

.overlay-icon-ring img {
    position: relative;
    display: block;
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.overlay-icon-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    overflow: hidden;
    color: #ffe8df;
    background: #3a211d;
    font-size: max(6px, calc(var(--debuff-icon-size) * .25));
    font-weight: 800;
}

.overlay-countdown {
    position: absolute;
    top: calc(var(--debuff-icon-size) + 1px);
    left: 50%;
    width: max-content;
    min-width: 100%;
    transform: translateX(-50%);
    color: #fff;
    font-size: clamp(10px, calc(var(--debuff-icon-size) * .38), 16px);
    font-weight: 800;
    line-height: 16px;
    text-align: center;
    white-space: nowrap;
    -webkit-text-stroke: 2px #000;
    paint-order: stroke fill;
}

.overlay-debuff.flashing .overlay-icon-ring {
    animation: debuff-expiring .58s ease-in-out infinite alternate;
}

@keyframes debuff-expiring {
    from { filter: brightness(1); transform: scale(1); }
    to { filter: brightness(1.8) saturate(1.25); transform: scale(.9); }
}
</style>
