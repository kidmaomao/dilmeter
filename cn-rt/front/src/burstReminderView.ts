import type { BurstRule } from './burstReminder';
import type { BossMechanicOverlayItem } from './skillCooldown';

export const BURST_PREVIEW_MS = 8000;
export type BurstPreviewPhase = 'cast' | 'ready' | 'effect' | 'cooldown' | 'alert';

export function isBurstCooldownGroup(item: BossMechanicOverlayItem): boolean {
 return item.skillId === 59005 && (item.phase === 'cooldown' || item.phase === 'ready');
}

export function burstPopupDimensions(item: BossMechanicOverlayItem): { width: number; height: number } {
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
  readyActors: cooldown ? [{ actorId: 'preview-a', actorName: '队友 A' }, ...(phase === 'ready' ? [{ actorId: 'preview-b', actorName: '队友 B' }] : [])] : undefined,
  nextReadySoon: phase === 'alert', skillId: rule.skillId, skillName: rule.name, phase: phase === 'alert' ? 'cooldown' : phase,
  compact: phase === 'cooldown', orientation: rule.orientation, hideCountdown: phase === 'ready', icon: 'mdi-alert-decagram',
  x: rule.cast.x, y: rule.cast.y, scalePercent: rule.cast.scalePercent,
  startedAtMs: nowMs, endsAtMs: nowMs + BURST_PREVIEW_MS, previewExpiresAtMs: nowMs + BURST_PREVIEW_MS, generation: nowMs,
 };
}
