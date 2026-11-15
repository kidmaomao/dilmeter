import type { SkillCooldownOverlayMessage, SkillCooldownOverlayItem, SkillCooldownRule } from "./skillCooldown";
import { isCumulativeCooldownSkill, TOAH_SPIRIT_SKILL_ID } from "./skillCooldown";
import { isEnergySkill } from "./skillEnergy";
import { DORCHA_MASTERY_SKILL_ID, normalizeDorchaThreshold, dorchaQuantityOverlayItem } from "./dorcha";

export type ReminderPreviewContent = Pick<SkillCooldownOverlayMessage, "items" | "aimReminder" | "effectTimers" | "stackAlerts" | "targetHealth">;
export type ReminderPreview = Partial<ReminderPreviewContent>;

let requestQueue: Promise<void> = Promise.resolve();

/** Send only the explicit sample. Never replace the live overlay snapshot. */
export function requestReminderPreview(content: ReminderPreview, durationMs: number, atMs = Date.now()): Promise<void> {
    const body = JSON.stringify({ ...content, durationMs, atMs });
    const request = requestQueue.catch(() => undefined).then(async () => {
        const response = await fetch("/api/reminder_preview", {
            method: "POST", headers: { "Content-Type": "application/json" }, body,
            signal: AbortSignal.timeout(5000),
        });
        if (!response.ok) throw new Error(await response.text());
    });
    requestQueue = request;
    return request;
}

export function createSkillReminderPreview(rule: SkillCooldownRule, name: string, iconUrl: string, atMs = Date.now()): ReminderPreview {
    if (rule.skillId === DORCHA_MASTERY_SKILL_ID) {
        const threshold = normalizeDorchaThreshold(rule.quantityThreshold);
        const item = dorchaQuantityOverlayItem({
            observed: true, quantity: Math.max(0, threshold - 1), below: true, threshold,
            triggeredAtMs: atMs, generation: atMs,
        }, { ...rule, enabled: true, barOnly: false })!;
        return { stackAlerts: [{ ...item, previewExpiresAtMs: atMs + 3600 }] };
    }
    const item: SkillCooldownOverlayItem = {
        skillId: rule.skillId, name, iconUrl, x: rule.x, y: rule.y,
        alwaysVisible: rule.alwaysVisible, showEnergyPercent: rule.showEnergyPercent,
        usedAtMs: atMs - 1000, readyAtMs: atMs + 900, generation: atMs,
        cooldownPhase: "full", previewExpiresAtMs: atMs + 3600,
    };
    if (isEnergySkill(rule.skillId)) {
        Object.assign(item, { readyAtMs: atMs, energyGate: true, energyActive: true, energyReady: true,
            cooldownObserved: true, progressPercent: 100, progressObserved: true,
            progressThresholdPercent: rule.progressThresholdPercent });
    } else if (rule.skillId === TOAH_SPIRIT_SKILL_ID) {
        Object.assign(item, { progressPercent: rule.progressThresholdPercent, progressObserved: true,
            progressThresholdPercent: rule.progressThresholdPercent });
    } else if (isCumulativeCooldownSkill(rule.skillId)) {
        item.cumulativeCooldownSeconds = rule.cumulativeCooldownSeconds;
        item.accumulatedCooldownSeconds = rule.cumulativeCooldownSeconds;
    }
    return { items: [item] };
}
