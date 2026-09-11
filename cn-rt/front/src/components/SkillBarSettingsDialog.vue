<template>
    <v-dialog v-model="open" max-width="980" persistent>
        <v-card class="skill-bar-settings-card">
            <v-card-title class="skill-bar-title">
                <span><v-icon icon="mdi-view-grid-plus-outline" class="mr-2" />额外技能栏(测试)</span>
                <v-btn icon="mdi-close" variant="text" size="small" aria-label="关闭技能栏设置" @click="open = false" />
            </v-card-title>
            <v-card-text :inert="saving">
                <div class="skill-bar-switches">
                    <label><input v-model="draft.enabled" type="checkbox" />显示技能栏</label>
                    <label title="开启且锁定后，点击技能图标会在洛奇保持前台时发送绑定按键">
                        <input v-model="draft.inputEnabled" type="checkbox" />允许点击施放
                    </label>
                    <label title="锁定时允许施放并固定位置；解除锁定后只用于拖动，不会施放">
                        <input v-model="draft.locked" type="checkbox" />锁定技能栏
                    </label>
                    <span :class="draft.locked ? 'locked-note' : 'move-note'">
                        {{ inputModeNote }}
                    </span>
                </div>

                <div class="skill-bar-input-settings">
                    <span class="input-setting-title">施放鼠标键</span>
                    <label><input v-model="draft.clickButton" type="radio" name="skill-bar-click-button" value="left" />左键</label>
                    <label><input v-model="draft.clickButton" type="radio" name="skill-bar-click-button" value="right" />右键</label>
                    <small>技能栏区域会拦截左右键；只有这里选中的鼠标键会施放技能。</small>
                </div>

                <div class="stop-movement-settings">
                    <label title="处理游戏绕过普通窗口消息、仍读取物理左键而产生的点地移动">
                        <input v-model="draft.stopMovement" type="checkbox" @change="handleStopMovementToggle" />阻止技能栏左键点地（建议开启）
                    </label>
                    <span>停止键</span>
                    <button
                        ref="stopMovementCaptureButton"
                        type="button"
                        class="key-capture stop-key-capture"
                        :class="{ capturing: stopMovementCaptureActive }"
                        @click="startStopMovementKeyCapture"
                    >
                        {{ stopMovementKeyLabel }}
                    </button>
                    <button v-if="draft.stopKeyCode" type="button" class="cancel-capture" @click="clearStopMovementKey">清除</button>
                    <small :class="{ 'stop-warning': !draft.stopMovement }">无论技能用左键还是右键施放，物理左键落在技能栏任意位置时都会短按该键 20ms。该键必须能在游戏中停止点地移动、且不会触发技能。</small>
                </div>

                <div class="skill-bar-layout-settings">
                    <label>X <input v-model.number="draft.x" type="number" min="-32000" max="32000" /></label>
                    <label>Y <input v-model.number="draft.y" type="number" min="-32000" max="32000" /></label>
                    <label>每行 <input v-model.number="draft.columns" type="number" min="1" max="12" /> 格</label>
                    <label>图标 <input v-model.number="draft.iconSize" type="number" min="32" max="80" /> px</label>
                    <label>间距 <input v-model.number="draft.gap" type="number" min="0" max="12" /> px</label>
                    <label>透明度 <input v-model.number="draft.opacity" type="number" min="25" max="100" /> %</label>
                    <label>格子数 <input v-model.number="slotCount" type="number" min="1" max="48" /></label>
                </div>

                <div class="skill-bar-editor-layout">
                    <section class="skill-bar-preview-panel">
                        <header>
                            <strong>技能栏预览</strong>
                            <small>先选格子，再从右侧选择技能和按键</small>
                        </header>
                        <div class="skill-bar-preview" :style="previewStyle">
                            <button
                                v-for="(slot, index) in draft.slots"
                                :key="slot.id"
                                type="button"
                                :class="{ selected: selectedIndex === index, empty: !slot.skillId }"
                                :title="slot.skillName || `空格 ${index + 1}`"
                                @click="selectSlot(index)"
                            >
                                <span v-if="!slot.skillId" class="slot-number">{{ index + 1 }}</span>
                                <template v-else>
                                    <span class="slot-fallback">{{ slot.skillId }}</span>
                                    <img :src="`/skill-icons/${slot.skillId}.png`" alt="" @error="hideImage" />
                                    <kbd v-if="slot.keyLabel">{{ slot.keyLabel }}</kbd>
                                </template>
                            </button>
                        </div>
                        <p>解除锁定并保存后只用于定位；按住左键移动超过少量距离即可拖动，重新锁定后才能点击施放。</p>
                    </section>

                    <section class="skill-slot-editor">
                        <header>
                            <strong>第 {{ selectedIndex + 1 }} 格</strong>
                            <button type="button" class="clear-slot" :disabled="!selectedSlot.skillId" @click="clearSelectedSlot">清空格子</button>
                        </header>
                        <label class="skill-search-label">
                            搜索技能
                            <input v-model.trim="searchText" type="search" placeholder="输入技能名称或技能 ID" autocomplete="off" />
                        </label>
                        <div v-if="searchText && searchResults.length" class="skill-search-results">
                            <button v-for="skill in searchResults" :key="skill.id" type="button" @click="selectSkill(skill)">
                                <span class="search-icon"><span>{{ skill.id }}</span><img :src="`/skill-icons/${skill.id}.png`" alt="" @error="hideImage" /></span>
                                <span><strong>{{ skill.name }}</strong><small>ID {{ skill.id }}</small></span>
                            </button>
                        </div>
                        <div v-else-if="searchText" class="no-result">没有找到对应技能，也可以直接输入数字技能 ID。</div>

                        <div class="selected-skill-summary">
                            <span class="selected-icon">
                                <span>{{ selectedSlot.skillId || "?" }}</span>
                                <img v-if="selectedSlot.skillId" :src="`/skill-icons/${selectedSlot.skillId}.png`" alt="" @error="hideImage" />
                            </span>
                            <div>
                                <strong>{{ selectedSlot.skillName || "尚未选择技能" }}</strong>
                                <small>{{ selectedSlot.skillId ? `技能 ID ${selectedSlot.skillId}` : "从搜索结果中选择" }}</small>
                            </div>
                        </div>

                        <div class="slot-fields">
                            <label>
                                游戏按键序列
                                <button
                                    ref="keyCaptureButton"
                                    type="button"
                                    class="key-capture"
                                    :class="{ capturing: keyCaptureActive }"
                                    @click="startKeyCapture"
                                >
                                    {{ captureDisplayLabel }}
                                </button>
                            </label>
                            <div v-if="keyCaptureActive" class="capture-actions">
                                <button type="button" class="finish-capture" @click="finishKeyCapture">完成录制</button>
                                <button type="button" class="cancel-capture" @click="cancelKeyCapture">取消录制</button>
                            </div>
                            <button v-else-if="selectedSlot.keySequence.length" type="button" class="cancel-capture" @click="clearKey">清除按键</button>
                            <label>
                                技能 CD
                                <span><input v-model.number="selectedSlot.cooldownSeconds" type="number" min="0.1" max="86400" step="0.1" /> 秒</span>
                            </label>
                        </div>
                        <p class="sequence-note">录制时先完整按下并松开第一段，再录下一段。例如：同时按 <kbd>Ctrl+[</kbd>，松开后再按 <kbd>Q</kbd>，最后点“完成录制”。</p>
                        <p class="cooldown-note">技能栏会把该技能加入后台追踪；真正收到服务器确认的施放事件后才开始倒计时。</p>
                    </section>
                </div>

                <v-alert v-if="notice" :type="noticeType" density="compact" variant="tonal" class="mt-3">{{ notice }}</v-alert>
            </v-card-text>
            <v-card-actions class="skill-bar-actions">
                <span>游戏保持前台；左键防点地与技能施放鼠标键相互独立。</span>
                <v-spacer />
                <v-btn variant="text" :disabled="saving" @click="resetDraft">恢复已保存</v-btn>
                <v-btn color="primary" :loading="saving" :disabled="keyCaptureActive || stopMovementCaptureActive" @click="saveSettings">
                    <v-icon icon="mdi-content-save-outline" class="mr-1" />保存并应用
                </v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from "vue";
import { skillNameMap, resourceNameVersion } from "@/store";
import {
    SKILL_BAR_CHANNEL,
    isSupportedSkillBarCode,
    isSupportedSkillBarStopCode,
    loadSkillBarSettings,
    makeSkillBarSlot,
    normalizeSkillBarSettings,
    saveSkillBarSettings,
    skillBarHasContent,
	skillBarKeyLabel,
    skillBarSequenceLabel,
    syncNativeSkillBarSettings,
    type SkillBarSettings,
} from "@/skillBar";
import {
    loadSkillCooldownSettings,
    makeSkillCooldownRule,
    saveSkillCooldownSettings,
} from "@/skillCooldown";

const props = defineProps<{ modelValue: boolean }>();
const emit = defineEmits<{ (event: "update:modelValue", value: boolean): void }>();

const open = computed({
    get: () => props.modelValue,
    set: (value: boolean) => emit("update:modelValue", value),
});
const draft = ref<SkillBarSettings>(loadSkillBarSettings());
const selectedIndex = ref(0);
const searchText = ref("");
const keyCaptureActive = ref(false);
const keyCaptureButton = ref<HTMLButtonElement>();
const keyCaptureBackup = ref<string[][]>([]);
const activeChordCodes = ref<string[]>([]);
const pressedCodes = new Set<string>();
const stopMovementCaptureActive = ref(false);
const stopMovementCaptureButton = ref<HTMLButtonElement>();
const stopMovementPendingCode = ref("");
const saving = ref(false);
const notice = ref("");
const noticeType = ref<"success" | "warning" | "error" | "info">("info");
const lastNativePosition = ref<{ x: number; y: number }>();
let nativePositionTimer: number | undefined;

const selectedSlot = computed(() => draft.value.slots[Math.min(selectedIndex.value, draft.value.slots.length - 1)]);
const inputModeNote = computed(() => {
    const protection = draft.value.stopMovement && isSupportedSkillBarStopCode(draft.value.stopKeyCode)
        ? `左键防点地：${skillBarKeyLabel(draft.value.stopKeyCode)}`
        : "左键防点地尚未启用";
    if (!draft.value.locked) return `定位模式：只拖动，不会施放；${protection}`;
    if (!draft.value.inputEnabled) return `技能施放已关闭；${protection}`;
    return `${draft.value.clickButton === "right" ? "右键" : "左键"}施放；${protection}`;
});
const stopMovementKeyLabel = computed(() => {
    if (stopMovementCaptureActive.value) return stopMovementPendingCode.value
        ? `松开 ${skillBarKeyLabel(stopMovementPendingCode.value)} 完成`
        : "请按停止动作键…";
    return draft.value.stopKeyCode ? skillBarKeyLabel(draft.value.stopKeyCode) : "点击录制";
});
const captureDisplayLabel = computed(() => {
    if (!keyCaptureActive.value) return selectedSlot.value.keyLabel || "点击后录制";
    const preview = selectedSlot.value.keySequence.map((chord) => [...chord]);
    if (activeChordCodes.value.length) preview.push([...activeChordCodes.value]);
    return skillBarSequenceLabel(preview) || "请按第一段按键…";
});
const slotCount = computed({
    get: () => draft.value.slots.length,
    set: (raw: number) => {
        const count = Math.min(48, Math.max(1, Math.round(Number(raw) || 1)));
        while (draft.value.slots.length < count) draft.value.slots.push(makeSkillBarSlot(draft.value.slots.length));
        if (draft.value.slots.length > count) draft.value.slots.splice(count);
        selectedIndex.value = Math.min(selectedIndex.value, count - 1);
    },
});
const previewStyle = computed(() => ({
    "--preview-columns": String(Math.max(1, Math.min(Number(draft.value.columns) || 1, draft.value.slots.length))),
    "--preview-size": `${Math.min(64, Math.max(36, Number(draft.value.iconSize) || 48))}px`,
    "--preview-gap": `${Math.min(12, Math.max(0, Number(draft.value.gap) || 0))}px`,
}));
const searchResults = computed(() => {
    resourceNameVersion.value;
    const query = searchText.value.trim().toLowerCase();
    if (!query) return [];
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (id > 0 && id <= 65535) return [{ id, name: skillNameMap.value[id] || `技能 ${id}` }];
    }
    return Object.entries(skillNameMap.value)
        .map(([rawId, name]) => ({ id: Number(rawId), name: String(name || `技能 ${rawId}`) }))
        .filter((item) => item.id > 0 && (item.name.toLowerCase().includes(query) || String(item.id).includes(query)))
        .sort((left, right) => left.name.localeCompare(right.name, "zh-CN") || left.id - right.id)
        .slice(0, 12);
});

watch(() => props.modelValue, (visible) => {
    if (!visible) {
        stopKeyCaptureState();
		stopStopMovementCapture();
        return;
    }
    resetDraft();
    void refreshNativePosition(true);
});

function resetDraft() {
    draft.value = loadSkillBarSettings();
    selectedIndex.value = Math.min(selectedIndex.value, draft.value.slots.length - 1);
    searchText.value = "";
    stopKeyCaptureState();
	stopStopMovementCapture();
    notice.value = "";
}

async function refreshNativePosition(force = false) {
    try {
        const response = await fetch("/api/skill_bar", { cache: "no-store" });
        if (!response.ok) return;
        const state = await response.json() as { x?: number; y?: number };
        if (!Number.isFinite(Number(state.x)) || !Number.isFinite(Number(state.y))) return;
        const next = { x: Math.round(Number(state.x)), y: Math.round(Number(state.y)) };
        const nativeMoved = !lastNativePosition.value
            || next.x !== lastNativePosition.value.x
            || next.y !== lastNativePosition.value.y;
        if (force || nativeMoved) {
            draft.value.x = next.x;
            draft.value.y = next.y;
        }
        lastNativePosition.value = next;
    } catch {
        // The saved browser coordinates remain a safe fallback.
    }
}

function selectSkill(skill: { id: number; name: string }) {
    selectedSlot.value.skillId = skill.id;
    selectedSlot.value.skillName = skill.name;
    const cooldownRule = loadSkillCooldownSettings().rules[skill.id];
    if (cooldownRule) selectedSlot.value.cooldownSeconds = cooldownRule.cooldownSeconds;
    searchText.value = "";
}

function clearSelectedSlot() {
    if (keyCaptureActive.value) cancelKeyCapture();
    const replacement = makeSkillBarSlot(selectedIndex.value);
    draft.value.slots.splice(selectedIndex.value, 1, replacement);
}

function selectSlot(index: number) {
    if (keyCaptureActive.value) cancelKeyCapture();
    selectedIndex.value = index;
}

function startKeyCapture() {
    if (saving.value || keyCaptureActive.value) return;
	stopStopMovementCapture();
    keyCaptureBackup.value = selectedSlot.value.keySequence.map((chord) => [...chord]);
    selectedSlot.value.keySequence = [];
    selectedSlot.value.keyCode = "";
    selectedSlot.value.keyLabel = "";
    activeChordCodes.value = [];
    pressedCodes.clear();
    keyCaptureActive.value = true;
    notice.value = "按下并松开一组键会记录一段；可继续录制下一段。";
    noticeType.value = "info";
    void nextTick(() => keyCaptureButton.value?.focus());
}

function captureKeyDown(event: KeyboardEvent) {
	if (stopMovementCaptureActive.value) {
		event.preventDefault();
		event.stopPropagation();
		if (event.repeat) return;
		if (!isSupportedSkillBarStopCode(event.code)) {
			noticeType.value = "warning";
			notice.value = `不能把 ${event.code || event.key} 作为停止键；请勿使用 Ctrl/Shift/Alt 或 CapsLock。`;
			return;
		}
		if (!stopMovementPendingCode.value) stopMovementPendingCode.value = event.code;
		return;
	}
    if (!keyCaptureActive.value) return;
    event.preventDefault();
    event.stopPropagation();
    if (!isSupportedSkillBarCode(event.code)) {
        noticeType.value = "warning";
        notice.value = `暂不支持 ${event.code || event.key}，请使用字母、数字、F1–F24、方向键或常用功能键。`;
        return;
    }
    if (event.repeat || pressedCodes.has(event.code)) return;
    if (activeChordCodes.value.length >= 4) {
        noticeType.value = "warning";
        notice.value = "每一段最多同时包含 4 个键。请先松开当前按键，再录制下一段。";
        return;
    }
    pressedCodes.add(event.code);
    activeChordCodes.value.push(event.code);
}

function captureKeyUp(event: KeyboardEvent) {
	if (stopMovementCaptureActive.value) {
		event.preventDefault();
		event.stopPropagation();
		if (!stopMovementPendingCode.value || event.code !== stopMovementPendingCode.value) return;
		draft.value.stopKeyCode = stopMovementPendingCode.value;
		draft.value.stopMovement = true;
		const label = skillBarKeyLabel(draft.value.stopKeyCode);
		stopStopMovementCapture();
		noticeType.value = "success";
		notice.value = `停止键已设为 ${label}。请确认它在游戏中能停止点地移动、且不会触发技能。`;
		return;
	}
    if (!keyCaptureActive.value) return;
    event.preventDefault();
    event.stopPropagation();
    if (!pressedCodes.delete(event.code) || pressedCodes.size) return;
    commitActiveChord();
}

function commitActiveChord() {
    if (!activeChordCodes.value.length) return;
    if (selectedSlot.value.keySequence.length >= 8) {
        activeChordCodes.value = [];
        finishKeyCapture();
        return;
    }
    selectedSlot.value.keySequence.push([...activeChordCodes.value]);
    activeChordCodes.value = [];
    updateSelectedKeyLabels();
    if (selectedSlot.value.keySequence.length >= 8) finishKeyCapture();
}

function updateSelectedKeyLabels() {
    const sequence = selectedSlot.value.keySequence;
    selectedSlot.value.keyCode = sequence.length === 1 && sequence[0].length === 1 ? sequence[0][0] : "";
    selectedSlot.value.keyLabel = skillBarSequenceLabel(sequence);
}

function finishKeyCapture() {
    if (!keyCaptureActive.value) return;
    if (activeChordCodes.value.length) commitActiveChord();
    if (!selectedSlot.value.keySequence.length) {
        selectedSlot.value.keySequence = keyCaptureBackup.value.map((chord) => [...chord]);
        noticeType.value = "warning";
        notice.value = "没有录到按键，已保留原来的按键序列。";
    } else {
        noticeType.value = "success";
        notice.value = `已录制：${skillBarSequenceLabel(selectedSlot.value.keySequence)}`;
    }
    updateSelectedKeyLabels();
    stopKeyCaptureState();
}

function cancelKeyCapture() {
    selectedSlot.value.keySequence = keyCaptureBackup.value.map((chord) => [...chord]);
    updateSelectedKeyLabels();
    stopKeyCaptureState();
    notice.value = "已取消录制，原按键序列未改变。";
    noticeType.value = "info";
}

function stopKeyCaptureState() {
    keyCaptureActive.value = false;
    activeChordCodes.value = [];
    pressedCodes.clear();
    keyCaptureBackup.value = [];
}

function startStopMovementKeyCapture() {
	if (saving.value || stopMovementCaptureActive.value) return;
	if (keyCaptureActive.value) cancelKeyCapture();
	stopMovementPendingCode.value = "";
	stopMovementCaptureActive.value = true;
	noticeType.value = "info";
	notice.value = "请按下并松开一个游戏能够识别并停止点地移动、但不会触发技能的单键；物理左键落在技能栏任意位置时都会短按 20ms。";
	void nextTick(() => stopMovementCaptureButton.value?.focus());
}

function handleStopMovementToggle() {
	if (!draft.value.stopMovement) {
		stopStopMovementCapture();
		return;
	}
	if (!isSupportedSkillBarStopCode(draft.value.stopKeyCode)) startStopMovementKeyCapture();
}

function stopStopMovementCapture() {
	stopMovementCaptureActive.value = false;
	stopMovementPendingCode.value = "";
}

function clearStopMovementKey() {
	stopStopMovementCapture();
	draft.value.stopMovement = false;
	draft.value.stopKeyCode = "";
}

function clearKey() {
    selectedSlot.value.keyCode = "";
    selectedSlot.value.keyLabel = "";
    selectedSlot.value.keySequence = [];
}

function syncCooldownRules(settings: SkillBarSettings) {
    const cooldownSettings = loadSkillCooldownSettings();
    const activeSkillIds = new Set(settings.slots.filter((slot) => slot.skillId > 0).map((slot) => slot.skillId));
    for (const [rawId, rule] of Object.entries(cooldownSettings.rules)) {
        if (rule.barOnly && !activeSkillIds.has(Number(rawId))) delete cooldownSettings.rules[Number(rawId)];
    }
    settings.slots.forEach((slot, index) => {
        if (slot.skillId <= 0) return;
        let rule = cooldownSettings.rules[slot.skillId];
        if (!rule) {
            rule = makeSkillCooldownRule(slot.skillId, settings.x + index * 4, settings.y + settings.iconSize + 12);
            rule.barOnly = true;
            rule.alwaysVisible = false;
            rule.soundMode = "none";
            cooldownSettings.rules[slot.skillId] = rule;
        }
        if (rule.barOnly) {
            rule.enabled = true;
            rule.cooldownSeconds = Math.min(86400, Math.max(.1, Number(slot.cooldownSeconds) || 30));
        }
    });
    saveSkillCooldownSettings(cooldownSettings);
}

async function saveSettings() {
    if (saving.value) return;
    if (keyCaptureActive.value || stopMovementCaptureActive.value) {
        noticeType.value = "warning";
        notice.value = "请先完成或取消当前按键录制，再保存设置。";
        return;
    }
    saving.value = true;
    notice.value = "";
    try {
		if (draft.value.stopMovement && !isSupportedSkillBarStopCode(draft.value.stopKeyCode)) {
			noticeType.value = "warning";
			notice.value = "请先录制一个能停止点地移动、且不会触发技能的安全单键，或关闭“阻止技能栏左键点地”。";
			return;
		}
        const normalized = normalizeSkillBarSettings(draft.value);
		// Apply to the native window first. A failed native request must not leave
		// localStorage claiming that an unapplied setting was saved successfully.
        const nativeState = await syncNativeSkillBarSettings(normalized);
        const saved = saveSkillBarSettings(normalized);
        draft.value = saved;
        syncCooldownRules(saved);
        lastNativePosition.value = { x: Math.round(nativeState.x), y: Math.round(nativeState.y) };
        if (typeof BroadcastChannel !== "undefined") {
            const channel = new BroadcastChannel(SKILL_BAR_CHANNEL);
            channel.postMessage(saved);
            channel.close();
        }
        noticeType.value = "success";
        notice.value = saved.enabled && skillBarHasContent(saved)
            ? "技能栏设定已保存并显示。若要拖动位置，请先解除锁定后再保存。"
            : "技能栏设定已保存；添加技能并开启显示后会出现在桌面。";
    } catch (error) {
        noticeType.value = "error";
        notice.value = `技能栏保存失败：${error instanceof Error ? error.message : String(error)}`;
    } finally {
        saving.value = false;
    }
}

function hideImage(event: Event) {
    (event.currentTarget as HTMLImageElement).style.display = "none";
}

onMounted(() => {
    window.addEventListener("keydown", captureKeyDown, true);
    window.addEventListener("keyup", captureKeyUp, true);
    nativePositionTimer = window.setInterval(() => {
        if (props.modelValue) void refreshNativePosition();
    }, 120);
});
onUnmounted(() => {
	stopStopMovementCapture();
    window.removeEventListener("keydown", captureKeyDown, true);
    window.removeEventListener("keyup", captureKeyUp, true);
    if (nativePositionTimer !== undefined) window.clearInterval(nativePositionTimer);
});
</script>

<style scoped>
.skill-bar-settings-card { color: var(--ui-theme-text); background: var(--ui-theme-surface); border: 1px solid var(--ui-theme-border); }
.skill-bar-title { display: flex; align-items: center; justify-content: space-between; color: var(--ui-color-accent-soft); background: var(--ui-theme-panel); border-bottom: 1px solid var(--ui-theme-border); font-size: 15px; }
.skill-bar-switches { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 18px; padding: 8px 10px; background: var(--ui-theme-panel); border: 1px solid var(--ui-theme-border-soft); }
.skill-bar-switches label { display: flex; align-items: center; gap: 6px; font-size: 12px; }
.skill-bar-switches span { margin-left: auto; padding: 3px 7px; font-size: 10px; border-radius: 2px; }
.locked-note { color: #d8f6ff; background: #245264; }
.move-note { color: #fff0b6; background: #6a5322; }
.skill-bar-input-settings { display: flex; flex-wrap: wrap; align-items: center; gap: 7px 14px; margin-top: 8px; padding: 7px 10px; color: var(--ui-theme-text-muted); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); font-size: 10px; }
.skill-bar-input-settings label { display: flex; align-items: center; gap: 5px; }
.skill-bar-input-settings small { margin-left: auto; color: var(--ui-theme-text-muted); font-size: 9px; }
.stop-movement-settings { display: flex; flex-wrap: wrap; align-items: center; gap: 7px 10px; margin-top: 6px; padding: 7px 10px; color: var(--ui-theme-text-muted); background: #171d20; border: 1px solid #615b3f; font-size: 10px; }
.stop-movement-settings label { display: flex; align-items: center; gap: 5px; color: #ffe7a1; }
.stop-movement-settings small { flex: 1 1 320px; color: #d2c791; font-size: 9px; }
.stop-movement-settings small.stop-warning { color: #ffb5a8; }
.stop-key-capture { min-width: 112px; height: 26px; }
.input-setting-title { color: var(--ui-color-accent-soft); font-weight: 700; }
.skill-bar-layout-settings { display: flex; flex-wrap: wrap; gap: 7px; margin: 10px 0; }
.skill-bar-layout-settings label { display: flex; align-items: center; gap: 4px; padding: 4px 6px; color: var(--ui-theme-text-muted); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); font-size: 10px; }
.skill-bar-layout-settings input { width: 58px; color: var(--ui-theme-text); background: var(--ui-theme-input); border: 1px solid var(--ui-theme-border); }
.skill-bar-editor-layout { display: grid; grid-template-columns: minmax(350px, 1.1fr) minmax(330px, .9fr); gap: 12px; }
.skill-bar-preview-panel, .skill-slot-editor { min-height: 320px; padding: 10px; background: var(--ui-theme-panel); border: 1px solid var(--ui-theme-border); }
.skill-bar-preview-panel > header, .skill-slot-editor > header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 9px; }
.skill-bar-preview-panel header strong, .skill-slot-editor header strong { color: var(--ui-color-accent-soft); font-size: 12px; }
.skill-bar-preview-panel header small { color: var(--ui-theme-text-muted); font-size: 9px; }
.skill-bar-preview { display: grid; grid-template-columns: repeat(var(--preview-columns), var(--preview-size)); gap: var(--preview-gap); align-content: start; min-height: 210px; padding: 14px; overflow: auto; background: #081011; border: 1px solid #4f666b; }
.skill-bar-preview button { position: relative; width: var(--preview-size); height: var(--preview-size); padding: 0; overflow: hidden; color: #fff; background: #182123; border: 1px solid #769096; cursor: pointer; }
.skill-bar-preview button.selected { border: 2px solid #ffdc62; box-shadow: 0 0 9px #ffdc62; }
.skill-bar-preview button.empty { border-style: dashed; opacity: .7; }
.skill-bar-preview img, .selected-icon img, .search-icon img { position: absolute; inset: 1px; width: calc(100% - 2px); height: calc(100% - 2px); object-fit: cover; }
.slot-number { color: #82979b; font-size: 12px; }
.slot-fallback { position: absolute; inset: 0; display: grid; place-items: center; font-size: 8px; }
.skill-bar-preview kbd { position: absolute; z-index: 3; right: 1px; bottom: 1px; padding: 0 2px; color: #fff6a8; background: rgba(0,0,0,.76); border: 1px solid #ddd; font-size: 8px; }
.skill-bar-preview-panel > p, .cooldown-note, .sequence-note { margin: 8px 0 0; color: var(--ui-theme-text-muted); font-size: 9px; }
.sequence-note kbd { padding: 1px 3px; color: #fff2a8; background: #1d292c; border: 1px solid #526a70; }
.clear-slot, .cancel-capture { padding: 3px 7px; color: #ffd7d7; background: #392525; border: 1px solid #7b5050; font-size: 9px; }
.clear-slot:disabled { opacity: .4; }
.skill-search-label { display: grid; gap: 4px; color: var(--ui-theme-text-muted); font-size: 10px; }
.skill-search-label input { height: 29px; padding: 0 8px; color: var(--ui-theme-text); background: var(--ui-theme-input); border: 1px solid var(--ui-theme-border); }
.skill-search-results { display: grid; max-height: 138px; margin-top: 4px; overflow-y: auto; background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); }
.skill-search-results button { display: grid; grid-template-columns: 32px 1fr; gap: 7px; align-items: center; padding: 4px; color: var(--ui-theme-text); background: transparent; border: 0; border-bottom: 1px solid var(--ui-theme-border-soft); text-align: left; cursor: pointer; }
.skill-search-results button:hover { background: var(--ui-theme-hover); }
.skill-search-results button > span:last-child { display: grid; }
.skill-search-results small { color: var(--ui-theme-text-muted); font-size: 8px; }
.search-icon, .selected-icon { position: relative; display: grid; place-items: center; width: 30px; height: 30px; overflow: hidden; background: #161c1d; border: 1px solid #718186; font-size: 7px; }
.no-result { padding: 8px; color: var(--ui-theme-text-muted); font-size: 9px; }
.selected-skill-summary { display: grid; grid-template-columns: 46px 1fr; gap: 9px; align-items: center; margin: 10px 0; padding: 8px; background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border-soft); }
.selected-icon { width: 44px; height: 44px; }
.selected-skill-summary > div { display: grid; }
.selected-skill-summary strong { color: var(--ui-color-accent-soft); font-size: 12px; }
.selected-skill-summary small { color: var(--ui-theme-text-muted); font-size: 9px; }
.slot-fields { display: grid; grid-template-columns: minmax(140px, 1fr) auto; gap: 8px; align-items: end; }
.slot-fields label { display: grid; gap: 4px; color: var(--ui-theme-text-muted); font-size: 10px; }
.slot-fields label:last-child { grid-column: 1 / -1; }
.slot-fields label span { display: flex; align-items: center; gap: 5px; }
.slot-fields input { width: 90px; height: 28px; padding: 0 6px; color: var(--ui-theme-text); background: var(--ui-theme-input); border: 1px solid var(--ui-theme-border); }
.key-capture { height: 30px; color: #eaffff; background: #263f45; border: 1px solid #6da6b1; font-weight: 700; }
.key-capture.capturing { color: #2a2105; background: #ffe17b; border-color: #fff2b6; }
.capture-actions { display: flex; gap: 5px; }
.finish-capture { padding: 3px 7px; color: #dfffe8; background: #25452f; border: 1px solid #5d9a70; font-size: 9px; }
.skill-bar-actions { border-top: 1px solid var(--ui-theme-border); }
.skill-bar-actions > span { color: var(--ui-theme-text-muted); font-size: 9px; }

@media (max-width: 760px) { .skill-bar-editor-layout { grid-template-columns: 1fr; } }
</style>
