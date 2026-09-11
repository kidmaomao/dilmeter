<template>
    <main
        class="buff-overlay-root"
        :class="{ locked: settings.locked }"
        :style="{
            '--buff-icon-size': `${settings.iconSize}px`,
            '--overlay-opacity': String(settings.opacity / 100),
        }"
    >
        <section
            v-if="visibleItems.length"
            class="overlay-buff-list"
            aria-label="Buff 到期提醒"
        >
            <div
                class="overlay-buff-items"
                @pointerdown="beginDrag"
                @pointermove="continueDrag"
                @pointerup="finishDrag"
                @pointercancel="finishDrag"
                @dragstart.prevent
            >
                <article
                    v-for="item in visibleItems"
                    :key="`${item.kind || 'buff'}-${item.ccId}-${item.appliedAt}-${item.state || 'active'}`"
                    class="overlay-buff"
                    :class="{ flashing: shouldFlash(item), preview: item.preview, inactive: item.active === false, missing: item.state === 'missing', debuff: item.kind === 'debuff' }"
                    :title="`${item.name}（CC ${item.ccId}）`"
                >
                    <div class="overlay-icon-ring">
                        <span class="overlay-icon-fallback">{{ item.ccId }}</span>
                        <img :src="item.iconUrl" :alt="`${item.name}图标`" @load="showImage" @error="retryImage" />
                    </div>
                    <span v-if="item.active !== false && !item.preview && remainingSeconds(item) !== null" class="overlay-countdown">
                        {{ displaySeconds(item) }}
                    </span>
                </article>
            </div>

        </section>
    </main>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from "vue";
import {
    BUFF_OVERLAY_CHANNEL,
    loadBuffOverlaySettings,
    resolveOverlayDpiPercent,
    type BuffOverlayItem,
    type BuffOverlayMessage,
} from "@/buffAlert";

const settings = reactive(loadBuffOverlaySettings());
const items = ref<BuffOverlayItem[]>([]);
const now = ref(Date.now() / 1000);
let clockTimer: number | undefined;
let statePollTimer: number | undefined;
let nativeClockTickListener: EventListener | undefined;
let lastOverlayPollAtMs = 0;
let channel: BroadcastChannel | undefined;
let nativeStateKey = "";
let lastMessageAt = 0;
let dragFrame: number | undefined;
let pendingDragPosition: { x: number; y: number } | undefined;
let dragState: {
    pointerId: number;
    startPointerX: number;
    startPointerY: number;
    startWindowX: number;
    startWindowY: number;
} | undefined;

const visibleItems = computed(() => items.value.filter((item) => {
    if (item.active === false) return true;
    const remaining = remainingSeconds(item);
    return remaining === null || remaining > 0;
}));

function remainingSeconds(item: BuffOverlayItem): number | null {
    if (item.expiresAt === null) return null;
    return Math.max(0, item.expiresAt - now.value);
}

function displaySeconds(item: BuffOverlayItem) {
    const remaining = remainingSeconds(item);
    if (remaining === null) return "";
    return Math.ceil(remaining);
}

function shouldFlash(item: BuffOverlayItem) {
    if (item.active === false) return false;
    const remaining = remainingSeconds(item);
    return item.flashEnabled && remaining !== null && remaining > 0 && remaining <= item.flashThresholdSeconds;
}

function syncNativeVisibility() {
    const active = settings.overlayEnabled && visibleItems.value.length > 0;
    const itemCount = visibleItems.value.length;
    const iconSize = settings.iconSize;
    const scalePercent = resolveOverlayDpiPercent(settings.dpiPercent, window.devicePixelRatio);
    const stateKey = `${active}:${itemCount}:${iconSize}:${scalePercent}:${settings.locked}`;
    if (nativeStateKey === stateKey) return;
    nativeStateKey = stateKey;
    void fetch("/api/buff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active, itemCount, iconSize, scalePercent }),
    }).catch(() => undefined);
}

function beginDrag(event: PointerEvent) {
    if (settings.locked || event.button !== 0) return;
    event.preventDefault();
    const target = event.currentTarget as HTMLElement;
    target.setPointerCapture(event.pointerId);
    dragState = {
        pointerId: event.pointerId,
        startPointerX: event.screenX,
        startPointerY: event.screenY,
        startWindowX: window.screenX,
        startWindowY: window.screenY,
    };
}

function continueDrag(event: PointerEvent) {
    if (!dragState || dragState.pointerId !== event.pointerId || settings.locked) return;
    event.preventDefault();
    pendingDragPosition = {
        x: Math.round(dragState.startWindowX + event.screenX - dragState.startPointerX),
        y: Math.round(dragState.startWindowY + event.screenY - dragState.startPointerY),
    };
    if (dragFrame !== undefined) return;
    dragFrame = window.requestAnimationFrame(() => {
        dragFrame = undefined;
        if (pendingDragPosition) void moveNativeOverlay(pendingDragPosition, false);
    });
}

function finishDrag(event: PointerEvent) {
    if (!dragState || dragState.pointerId !== event.pointerId) return;
    event.preventDefault();
    const target = event.currentTarget as HTMLElement;
    if (target.hasPointerCapture(event.pointerId)) target.releasePointerCapture(event.pointerId);
    const finalPosition = {
        x: Math.round(dragState.startWindowX + event.screenX - dragState.startPointerX),
        y: Math.round(dragState.startWindowY + event.screenY - dragState.startPointerY),
    };
    dragState = undefined;
    pendingDragPosition = undefined;
    if (dragFrame !== undefined) {
        window.cancelAnimationFrame(dragFrame);
        dragFrame = undefined;
    }
    void moveNativeOverlay(finalPosition, true);
}

async function moveNativeOverlay(position: { x: number; y: number }, save: boolean) {
    try {
        await fetch("/api/buff_overlay/position", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                ...position,
                save,
                sequence: Math.round((performance.timeOrigin + performance.now()) * 1000),
            }),
        });
    } catch {
        // The native window may be closing.
    }
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
    const next = loadBuffOverlaySettings();
    settings.locked = next.locked;
    settings.iconSize = next.iconSize;
    settings.volume = next.volume;
    settings.dpiPercent = next.dpiPercent;
    settings.overlayEnabled = next.overlayEnabled;
    settings.opacity = next.opacity;
    settings.rules = next.rules;
    syncNativeVisibility();
}

function refreshOverlayAppearance() {
    const next = loadBuffOverlaySettings();
    if (
        settings.overlayEnabled === next.overlayEnabled
        && settings.opacity === next.opacity
        && settings.dpiPercent === next.dpiPercent
    ) return;
    settings.overlayEnabled = next.overlayEnabled;
    settings.opacity = next.opacity;
    settings.dpiPercent = next.dpiPercent;
    syncNativeVisibility();
}

function applyOverlayMessage(message: BuffOverlayMessage) {
    if (message.type !== "buff-state" || message.at < lastMessageAt) return;
    lastMessageAt = message.at;
    items.value = Array.isArray(message.items) ? message.items : [];
    if (message.settings) {
        settings.locked = Boolean(message.settings.locked);
        settings.iconSize = Math.min(80, Math.max(16, Number(message.settings.iconSize) || 20));
        settings.dpiPercent = Number(message.settings.dpiPercent) > 0
            ? Math.min(500, Math.max(50, Math.round(Number(message.settings.dpiPercent))))
            : 0;
    }
    syncNativeVisibility();
}

async function pollOverlayState(atMs = Date.now(), force = false) {
    if (!force && atMs - lastOverlayPollAtMs < 175) return;
    lastOverlayPollAtMs = atMs;
    refreshOverlayAppearance();
    try {
        const response = await fetch("/api/buff_overlay/state", { cache: "no-store" });
        if (!response.ok) return;
        applyOverlayMessage(await response.json() as BuffOverlayMessage);
    } catch {
        // The native host may be closing.
    }
}

onMounted(() => {
    document.documentElement.classList.add("buff-overlay-page");
    try {
        channel = new BroadcastChannel(BUFF_OVERLAY_CHANNEL);
        channel.onmessage = (event: MessageEvent<BuffOverlayMessage>) => {
            if (event.data?.type === "buff-state") applyOverlayMessage(event.data);
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
    window.addEventListener("dilmeter-buff-alert-settings", reloadSettings as EventListener);
});

onUnmounted(() => {
    nativeStateKey = "false:0:0:false";
    void fetch("/api/buff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ active: false }),
    }).catch(() => undefined);
    document.documentElement.classList.remove("buff-overlay-page");
    channel?.close();
    if (clockTimer !== undefined) window.clearInterval(clockTimer);
    if (statePollTimer !== undefined) window.clearInterval(statePollTimer);
    if (nativeClockTickListener) {
        window.removeEventListener("dilmeter-native-tick", nativeClockTickListener);
        nativeClockTickListener = undefined;
    }
    if (dragFrame !== undefined) window.cancelAnimationFrame(dragFrame);
    window.removeEventListener("storage", reloadSettings);
    window.removeEventListener("dilmeter-buff-alert-settings", reloadSettings as EventListener);
});
</script>

<style>
html.buff-overlay-page,
html.buff-overlay-page body,
html.buff-overlay-page #app,
html.buff-overlay-page .v-application,
.buff-overlay-app {
    width: 100%;
    height: 100%;
    overflow: hidden !important;
    background: transparent !important;
}
</style>

<style scoped>
.buff-overlay-root {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    padding: 8px;
    background: transparent;
    font-family: "Microsoft YaHei", sans-serif;
    user-select: none;
    pointer-events: none;
}

.overlay-buff-list {
    display: flex;
    align-items: flex-start;
    gap: 4px;
    width: max-content;
    max-width: 100%;
    min-width: 20px;
    height: calc(var(--buff-icon-size) + 18px);
    pointer-events: none;
    opacity: var(--overlay-opacity);
    transition: opacity .12s linear;
}

.overlay-buff-items {
    display: flex;
    align-items: flex-start;
    gap: 6px;
    width: max-content;
    height: calc(var(--buff-icon-size) + 18px);
    pointer-events: none;
}

.overlay-buff {
    position: relative;
    flex: 0 0 var(--buff-icon-size);
    width: var(--buff-icon-size);
    min-width: var(--buff-icon-size);
    max-width: var(--buff-icon-size);
    height: calc(var(--buff-icon-size) + 18px);
}

.overlay-icon-ring {
    position: relative;
    box-sizing: border-box;
    width: var(--buff-icon-size);
    height: var(--buff-icon-size);
    min-width: var(--buff-icon-size);
    max-width: var(--buff-icon-size);
    min-height: var(--buff-icon-size);
    max-height: var(--buff-icon-size);
    aspect-ratio: 1 / 1;
    overflow: hidden;
    background: rgba(10, 12, 12, .4);
    border: 1px solid #cbffff;
    border-radius: max(3px, calc(var(--buff-icon-size) * .18));
    box-shadow:
        0 0 2px #ffffff,
        0 0 5px #6cf6ff,
        0 0 9px rgba(41, 217, 255, .85);
}

.locked .overlay-icon-ring {
    border-color: transparent;
    border-radius: 1px;
    box-shadow: none;
}

.overlay-icon-ring img {
    position: relative;
    display: block;
    width: 100%;
    height: 100%;
    min-width: 100%;
    max-width: 100%;
    min-height: 100%;
    max-height: 100%;
    object-fit: cover;
}

.overlay-buff.inactive .overlay-icon-ring img {
    filter: grayscale(1) brightness(.48) contrast(.9);
    opacity: .78;
}

.overlay-buff.inactive .overlay-icon-ring {
    background: rgba(0, 0, 0, .65);
}

.overlay-buff.inactive:not(.preview) .overlay-icon-ring {
    border-color: #858c8c;
    box-shadow: 0 0 3px rgba(190, 200, 200, .55);
}

.locked .overlay-buff.inactive .overlay-icon-ring {
    border-color: transparent;
    box-shadow: none;
}

.overlay-buff.debuff.missing .overlay-icon-ring,
.locked .overlay-buff.debuff.missing .overlay-icon-ring {
    border-color: #ff8068;
    box-shadow: 0 0 4px #ff3d24, 0 0 10px rgba(255, 54, 30, .78);
}

.overlay-icon-fallback {
    position: absolute;
    inset: 0;
    display: grid;
    place-items: center;
    overflow: hidden;
    color: #dffcff;
    background: #273737;
    font-size: max(6px, calc(var(--buff-icon-size) * .25));
    font-weight: 800;
}

.overlay-countdown {
    position: absolute;
    top: calc(var(--buff-icon-size) + 1px);
    left: 50%;
    width: max-content;
    min-width: 100%;
    height: 16px;
    padding: 0;
    transform: translateX(-50%);
    color: #ffffff;
    background: transparent;
    font-size: clamp(10px, calc(var(--buff-icon-size) * .38), 16px);
    font-weight: 800;
    line-height: 16px;
    text-align: center;
    white-space: nowrap;
    -webkit-text-stroke: 2px #000000;
    paint-order: stroke fill;
    text-shadow:
        -1px -1px 0 #000000,
        1px -1px 0 #000000,
        -1px 1px 0 #000000,
        1px 1px 0 #000000;
}

.overlay-buff.flashing .overlay-icon-ring {
    animation: buff-expiring .58s ease-in-out infinite alternate;
}

.overlay-buff.preview .overlay-icon-ring {
    filter: brightness(1.08);
}

@keyframes buff-expiring {
    from { filter: brightness(1); transform: scale(1); }
    to { filter: brightness(1.75) saturate(1.2); transform: scale(.9); }
}
</style>
