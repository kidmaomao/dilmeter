/** Geometry is in mineral-radius units, map +Y up, after display rotation. */
export interface BrontasPoint { x: number; y: number }
export interface BrontasGuideView {
 status: 'waiting' | 'watch' | 'axis' | 'enter' | 'hold' | 'blocked' | 'move';
 instruction: string;
 detail: string;
 clock: number;
 validUntilMs: number;
 lethalAtMs?: number;
 player?: BrontasPoint & { radius: number; speed: number };
 target?: BrontasPoint;
 rays?: { id: string; axis: 'horizontal' | 'vertical'; warningAtMs: number; lethalAtMs: number; endsAtMs: number; offset?: number; halfWidth?: number }[];
}

// Render-time expiry also covers a stalled backend, failed poll, or hidden
// WebView waking later. Never leave an old movement arrow on the game.
export function brontasGuideFresh(view: BrontasGuideView, nowMs: number): boolean {
 return nowMs < view.validUntilMs;
}

export function brontasGuideDeadline(view: BrontasGuideView, nowMs: number): string {
 if (!brontasGuideFresh(view, nowMs) || !view.lethalAtMs) return '--';
 return view.lethalAtMs <= nowMs ? '已致命' : `${((view.lethalAtMs-nowMs)/1000).toFixed(1)} s`;
}
