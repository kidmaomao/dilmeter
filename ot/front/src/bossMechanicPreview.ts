import type { BossMechanicAlertRule } from './bossMechanicAlert';

export interface BossMechanicPreviewOptions {
    resonanceMode?: "absorb" | "release";
    areaWarning?: boolean;
    tornadoTarget?: "self" | "teammate";
}

/** The server retains explicit previews across normal encounter refreshes. */
export async function requestBossMechanicPreview(rule: BossMechanicAlertRule, scalePercent: number, persistent = false, perlon?: BossMechanicPreviewOptions): Promise<void> {
    const durationSeconds = persistent ? 8 : rule.key === 'miel-green-curtain'
        ? rule.warningSeconds ?? 10 : rule.countdownSeconds;
    const response = await fetch('/api/boss_mechanic_preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...perlon, key: rule.key, durationSeconds, x: persistent ? rule.persistentX : rule.x, y: persistent ? rule.persistentY : rule.y, scalePercent: persistent ? rule.persistentScalePercent : rule.scalePercent ?? scalePercent, opacityPercent: persistent ? rule.persistentOpacityPercent : rule.opacityPercent, ...(persistent ? { passiveStyle: rule.persistentStyle ?? "timer" } : {}) }),
        signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) throw new Error(await response.text());
}
