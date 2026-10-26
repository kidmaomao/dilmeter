import type { BossMechanicAlertRule } from './bossMechanicAlert';

/** The server retains explicit previews across normal encounter refreshes. */
export async function requestBossMechanicPreview(rule: BossMechanicAlertRule, scalePercent: number, persistent = false): Promise<void> {
    const durationSeconds = persistent ? 8 : rule.key === 'miel-green-curtain'
        ? rule.warningSeconds ?? 10 : rule.countdownSeconds;
    const response = await fetch('/api/boss_mechanic_preview', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key: rule.key, durationSeconds, x: persistent ? rule.persistentX : rule.x, y: persistent ? rule.persistentY : rule.y, scalePercent: persistent ? rule.persistentScalePercent : scalePercent, ...(persistent ? { passiveStyle: rule.persistentStyle ?? "timer", opacityPercent: rule.persistentOpacityPercent } : {}) }),
        signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) throw new Error(await response.text());
}
