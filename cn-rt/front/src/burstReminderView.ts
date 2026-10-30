import type { BurstRule } from './burstReminder';
import type { BossMechanicOverlayItem } from './skillCooldown';

export const BURST_PREVIEW_MS = 8000;
export type BurstPreviewPhase = 'cast' | 'ready' | 'effect' | 'cooldown' | 'alert';

export function isBurstCooldownGroup(item: BossMechanicOverlayItem): boolean {
 return item.skillId === 59005 && (item.phase === 'cooldown' || item.phase === 'ready');
}

export function isBurstPowerEffectGroup(item: BossMechanicOverlayItem): boolean {
 return item.skillId === 58014 && item.phase === 'effect' && !!item.effectActors?.length;
}

export function visibleBurstEffectActors(item: BossMechanicOverlayItem, nowMs: number) {
 return (item.effectActors || []).filter(actor => actor.startedAtMs <= nowMs && actor.endsAtMs > nowMs);
}

export function burstRemainingText(endsAtMs: number, nowMs: number): string {
 const remaining = Math.max(0, endsAtMs - nowMs) / 1000;
 return remaining >= 10 ? String(Math.ceil(remaining)) : (Math.ceil(remaining * 10) / 10).toFixed(1);
}

export function burstPopupDimensions(item: BossMechanicOverlayItem): { width: number; height: number } {
 if (item.brontasGuide) return { width: 320, height: 340 };
 if (item.rocks?.length) return { width: 260, height: 180 };
 if (item.passiveStyle) return item.passiveStyle === "bar" ? { width: 180, height: 30 } : { width: 104, height: 34 };
 if (isBurstPowerEffectGroup(item)) return { width: 128, height: Math.max(112, 80 + 18 * item.effectActors!.length) };
 if (isBurstCooldownGroup(item) || item.compact) {
  let rows = item.readyActors?.length || 0;
  if (item.phase !== 'ready' || rows === 0) rows++;
  return item.compact ? { width: 112, height: Math.max(80, 48 + 16 * rows) } : { width: 128, height: Math.max(112, 80 + 16 * rows) };
 }
 const size = item.label ? 112 : 118;
 return { width: size, height: size };
}

export function createBurstPreview(rule: BurstRule, phase: BurstPreviewPhase, nowMs = Date.now()): BossMechanicOverlayItem {
 const cooldown = rule.skillId === 59005 && (phase === 'ready' || phase === 'cooldown' || phase === 'alert');
 const label = `预览 · ${rule.name}`;
 return {
  key: `burst-preview-${phase}`, name: label, label, actorId: 'preview-b', actorName: cooldown ? '队友 B' : '预览队友',
  effectActors: rule.skillId === 58014 && phase === 'effect' ? ['队友 A', '队友 B', '队友 C'].map((actorName, index) => ({ actorId: `preview-${index}`, actorName, startedAtMs: nowMs, endsAtMs: nowMs + BURST_PREVIEW_MS })) : undefined,
  readyActors: cooldown ? [{ actorId: 'preview-a', actorName: '队友 A' }, ...(phase === 'ready' ? [{ actorId: 'preview-b', actorName: '队友 B' }] : [])] : undefined,
  nextReadySoon: phase === 'alert', skillId: rule.skillId, skillName: rule.name, phase: phase === 'alert' ? 'cooldown' : phase,
  compact: phase === 'cooldown', orientation: rule.orientation, hideCountdown: phase === 'ready', icon: 'mdi-alert-decagram',
  x: rule.cast.x, y: rule.cast.y, scalePercent: rule.cast.scalePercent,
  startedAtMs: nowMs, endsAtMs: nowMs + BURST_PREVIEW_MS, previewExpiresAtMs: nowMs + BURST_PREVIEW_MS, generation: nowMs,
 };
}
