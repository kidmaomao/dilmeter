import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false }, optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
const storage = new Map();
globalThis.localStorage = { getItem: (k) => storage.get(k) ?? null, setItem: (k, v) => storage.set(k, v) };
globalThis.window = { dispatchEvent() {} };
try {
    const { initialDorchaQuantityRuntime: initial, applyDorchaQuantityObservation: apply,
        dorchaQuantityOverlayItem: overlay, formatDorchaQuantity } = await server.ssrLoadModule("/src/dorcha.ts");
    const { makeSkillCooldownRule, loadSkillCooldownSettings, saveSkillCooldownSettings,
        SKILL_COOLDOWN_STORAGE_KEY } = await server.ssrLoadModule("/src/skillCooldown.ts");
    const rule = makeSkillCooldownRule(27000);
    assert.equal(rule.quantityThreshold, 3);
    assert.equal(rule.scalePercent, 100);
    let state = initial();
    for (const value of [undefined, null, NaN, Infinity, -1, 16]) {
        state = apply(state, value, 3, 1000);
        assert.equal(state.observed, false);
        assert.equal(overlay(state, rule), undefined);
    }
    state = apply(state, 3, 3, 2000);
    assert.equal(state.generation, 0);
    assert.equal(overlay(state, rule), undefined, "exactly 3 is not below 3");
    state = apply(state, 2.75, 3, 3000);
    assert.equal(state.generation, 1);
    assert.equal(overlay(state, rule).quantityText, "2.75");
    assert.equal(overlay(state, rule).persistent, true);
    for (const value of [2.75, 2, 0]) state = apply(state, value, 3, 4000);
    assert.equal(state.generation, 1, "remaining low cannot repeatedly announce");
    state = apply(state, 3, 3, 5000);
    assert.equal(overlay(state, rule), undefined, "recovery dismisses the popup");
    state = apply(state, 2.5, 3, 6000);
    assert.equal(state.generation, 2);
    state = apply(state, 2.5, 1, 7000);
    assert.equal(state.generation, 2, "editing a threshold is not a new spend");
    assert.equal(state.below, false);
    assert.equal(apply(initial(), 2, 5, 1000).generation, 1, "first real reading honors a custom threshold");
    assert.equal(formatDorchaQuantity(2.999), "2.99", "display cannot round into the safe zone");
    assert.equal(overlay(initial(), { ...rule, alwaysVisible: true }).quantityText, "--");
    assert.equal(overlay(state, { ...rule, barOnly: true, alwaysVisible: true }), undefined);
    storage.set(SKILL_COOLDOWN_STORAGE_KEY, JSON.stringify({ rules: { 27000: { skillId: 27000, enabled: true, cooldownSeconds: 30, soundMode: "none" } } }));
    const settings = loadSkillCooldownSettings();
    assert.equal(settings.rules[27000].quantityThreshold, 3, "old CD profiles acquire the quantity default");
    assert.equal(settings.rules[27000].soundMode, "none");
    assert.equal(settings.rules[27000].scalePercent, 100, "old profiles retain the original popup size");
    settings.rules[27000].quantityThreshold = 5;
    settings.rules[27000].scalePercent = 150;
    saveSkillCooldownSettings(settings);
    assert.equal(loadSkillCooldownSettings().rules[27000].quantityThreshold, 5, "quantity preference survives saving");
    const savedRule = loadSkillCooldownSettings().rules[27000];
    assert.equal(savedRule.scalePercent, 150, "popup size survives saving");
    const scaledCard = overlay(apply(initial(), 2, savedRule.quantityThreshold, 8000), savedRule);
    assert.equal(scaledCard.scalePercent, 150, "the saved size reaches the preview card");
    assert.equal(scaledCard.x, savedRule.x, "scaling keeps the chosen top-left coordinate");
    assert.equal(scaledCard.y, savedRule.y);
    console.log("Dorcha: strict threshold, re-arm, missing values, decimal display, profile migration, and saved popup size verified");
} finally { await server.close(); }
