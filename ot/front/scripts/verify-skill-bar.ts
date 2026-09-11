import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";

const values = new Map<string, string>();
Object.defineProperty(globalThis, "localStorage", {
    value: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, String(value)),
        removeItem: (key: string) => values.delete(key),
        clear: () => values.clear(),
    },
});

const {
    SKILL_BAR_STORAGE_KEY,
    isSupportedSkillBarCode,
    isSupportedSkillBarStopCode,
    loadSkillBarSettings,
    nativeSkillBarPayload,
    normalizeSkillBarKeySequence,
    normalizeSkillBarSettings,
    skillBarBounds,
    skillBarChordLabel,
    skillBarKeyLabel,
    skillBarSequenceLabel,
} = await import("../src/skillBar.ts");

const settingsDialogSource = readFileSync(new URL("../src/components/SkillBarSettingsDialog.vue", import.meta.url), "utf8");
assert.match(settingsDialogSource, /额外技能栏\(测试\)/, "the settings dialog uses the requested test-feature title");

localStorage.setItem(SKILL_BAR_STORAGE_KEY, JSON.stringify({
    enabled: true,
    inputEnabled: true,
    locked: true,
    x: 700,
    y: 800,
    columns: 2,
    iconSize: 52,
    gap: 4,
    opacity: 85,
    slots: [
        { id: "one", skillId: 21002, skillName: "穿心箭", keyCode: "KeyQ", keySequence: [["ControlLeft", "BracketLeft"], ["KeyQ"]], cooldownSeconds: 2.45 },
        { id: "two", skillId: 0, keyCode: "MediaPlayPause" },
        { id: "three", skillId: 59145, keyCode: "F12", cooldownSeconds: 10 },
    ],
}));

const loaded = loadSkillBarSettings();
assert.equal(loaded.enabled, true);
assert.equal(loaded.clickButton, "left", "legacy settings keep left-click activation");
assert.equal(loaded.stopMovement, false, "stop-key compensation is opt-in");
assert.equal(loaded.stopKeyCode, "", "legacy settings do not gain a fixed stop key");
assert.equal(loaded.slots[0].keyLabel, "Ctrl+[ → Q");
assert.equal(loaded.slots[0].keyCode, "", "multi-step sequences do not masquerade as a legacy single key");
assert.deepEqual(loaded.slots[0].keySequence, [["ControlLeft", "BracketLeft"], ["KeyQ"]]);
assert.equal(loaded.slots[0].cooldownSeconds, 2.5, "cooldowns normalize to one decimal place");
assert.equal(loaded.slots[1].keyCode, "", "unsupported keys are discarded");
assert.deepEqual(skillBarBounds(loaded), { x: 700, y: 800, width: 114, height: 114 });
assert.equal(isSupportedSkillBarCode("NumpadEnter"), true);
assert.equal(isSupportedSkillBarCode("MediaPlayPause"), false);
assert.equal(isSupportedSkillBarStopCode("KeyS"), true);
assert.equal(isSupportedSkillBarStopCode("CapsLock"), false, "toggle keys cannot be movement stop keys");
assert.equal(isSupportedSkillBarStopCode("ControlLeft"), false, "modifiers cannot be movement stop keys");
assert.equal(skillBarKeyLabel("ControlRight"), "RCtrl");
assert.equal(skillBarChordLabel(["BracketLeft", "ControlLeft"]), "Ctrl+[");
assert.equal(skillBarSequenceLabel([["ControlLeft", "BracketLeft"], ["KeyQ"]]), "Ctrl+[ → Q");
assert.deepEqual(normalizeSkillBarKeySequence([["KeyQ", "KeyQ", "MediaPlayPause"], [], ["F12"]]), [["KeyQ"], ["F12"]]);
const nativePayload = nativeSkillBarPayload(loaded);
assert.equal(nativePayload.active, true);
assert.equal(nativePayload.clickButton, "left");
assert.equal(nativePayload.stopMovement, false);
assert.equal(nativePayload.stopKeyCode, "");
assert.equal("strongIsolation" in nativePayload, false, "the retired focus-switch option is not sent to native code");
assert.equal(nativePayload.slots[0].skillId, 21002);
assert.equal(nativePayload.slots[0].keyCode, "");
assert.deepEqual(nativePayload.slots[0].keySequence, [["ControlLeft", "BracketLeft"], ["KeyQ"]]);
assert.deepEqual(loaded.slots[2].keySequence, [["F12"]], "legacy single-key settings are migrated");
assert.equal(nativePayload.width, 114);
assert.equal(nativePayload.height, 114);

const clamped = normalizeSkillBarSettings({ columns: 99, iconSize: 2, opacity: 500, slots: [] });
assert.equal(clamped.columns, 12);
assert.equal(clamped.iconSize, 32);
assert.equal(clamped.opacity, 100);
assert.equal(clamped.slots.length, 1, "the bar always keeps one editable slot");

const migratedDisabledBarrier = normalizeSkillBarSettings({ strongIsolation: false } as never);
assert.equal(migratedDisabledBarrier.stopMovement, false, "legacy strongIsolation=false migrates to movement interruption off");
const migratedEnabledBarrier = normalizeSkillBarSettings({ strongIsolation: true } as never);
assert.equal(migratedEnabledBarrier.stopMovement, false, "legacy strongIsolation=true does not silently enable a stop key");
const explicitInputMode = normalizeSkillBarSettings({
    clickButton: "right",
    stopMovement: true,
    stopKeyCode: "F8",
});
assert.equal(explicitInputMode.clickButton, "right");
assert.equal(explicitInputMode.stopMovement, true, "a valid user-selected stop key is retained when switching buttons");
assert.equal(explicitInputMode.stopKeyCode, "F8", "the user-selected stop key is retained");
const optInLeftMode = normalizeSkillBarSettings({
    clickButton: "left",
    stopMovement: true,
    stopKeyCode: "KeyS",
});
assert.equal(optInLeftMode.stopMovement, true);
assert.equal(optInLeftMode.stopKeyCode, "KeyS");
const invalidInputMode = normalizeSkillBarSettings({ clickButton: "middle", stopKeyCode: "MediaPlayPause" } as never);
assert.equal(invalidInputMode.clickButton, "left");
assert.equal(invalidInputMode.stopKeyCode, "");
assert.equal(invalidInputMode.stopMovement, false);
const unsafeStopInputMode = normalizeSkillBarSettings({ stopMovement: true, stopKeyCode: "CapsLock" });
assert.equal(unsafeStopInputMode.stopKeyCode, "");
assert.equal(unsafeStopInputMode.stopMovement, false);
const trimmedStopInputMode = normalizeSkillBarSettings({ stopMovement: true, stopKeyCode: " KeyS " });
assert.equal(trimmedStopInputMode.stopKeyCode, "KeyS", "stop keys are stored in canonical form");
assert.equal(trimmedStopInputMode.stopMovement, true);

console.log("skill bar verification passed");
