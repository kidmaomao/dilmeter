/** Geometry is in mineral-radius units, map +Y up, after display rotation. */
export interface BrontasPoint { x: number; y: number }
export interface BrontasGuideView {
 motion?: { from:BrontasPoint; to:BrontasPoint; atMs:number; untilMs:number; speed:number };
 rayLifetimeModel?: string;
 source?: 'packet';
 stage?: 'map' | 'station';
 facing?: 'outward' | 'inward' | 'north' | 'south';
 guidanceKind?: 'route' | 'escape';
 guidanceReason?: string;
 paceUntilMs?: number;
 sourceAtMs?: number;
 computedAtMs?: number;
 guidance?: 'waiting' | 'hold' | 'pace' | 'move' | 'blocked';
 guidanceValidUntilMs?: number;
 playerValidUntilMs?: number;
 receivedCount?: number;
 pendingCount?: number;
 status: 'waiting' | 'watch' | 'axis' | 'enter' | 'hold' | 'blocked' | 'move';
 instruction: string;
 detail: string;
 clock: number;
 validUntilMs: number;
 lethalAtMs?: number;
 player?: BrontasPoint & { radius: number; speed: number };
 target?: BrontasPoint;
 nextTarget?: BrontasPoint;
 stepAxis?: 'horizontal' | 'vertical';
 rays?: { id: string; axis: 'horizontal' | 'vertical'; warningAtMs: number; lethalAtMs: number; endsAtMs: number; offset?: number; halfWidth?: number }[];
}

// Render-time expiry also covers a stalled backend, failed poll, or hidden
// WebView waking later. Never leave an old movement arrow on the game.
export function brontasGuideFresh(view: BrontasGuideView, nowMs: number): boolean {
 return nowMs < view.validUntilMs;
}

export function brontasGuideDeadline(view: BrontasGuideView, nowMs: number): string {
 if (view.source === 'packet') {
  if (!brontasGuideFresh(view, nowMs)) return '--';
  const next = Math.min(...brontasGuideFootTimers(view,nowMs).map(r => r.lethalAtMs));
  return Number.isFinite(next) ? `${((next-nowMs)/1000).toFixed(1)} s` : '--';
 }
 if (!brontasGuideFresh(view, nowMs) || !view.lethalAtMs) return '--';
 return view.lethalAtMs <= nowMs ? '已致命' : `${((view.lethalAtMs-nowMs)/1000).toFixed(1)} s`;
}

// One stripe per local channel. An active finite ray wins visually; personal
// timers independently retain the next hit, including a repeated same-lane ray.
export function brontasGuideMapRays(view: BrontasGuideView, nowMs: number) {
 if (view.source !== 'packet') return view.rays || [];
 const bands = new Map<string, NonNullable<BrontasGuideView['rays']>[number]>();
 for (const ray of view.rays || []) {
  if (ray.endsAtMs && nowMs>=ray.endsAtMs) continue;
  const key = `${ray.axis}:${ray.offset}`;
  const prior = bands.get(key);
  const active = ray.endsAtMs>nowMs && ray.lethalAtMs<=nowMs;
  const priorActive = prior && prior.endsAtMs>nowMs && prior.lethalAtMs<=nowMs;
  if (active && !priorActive) { bands.set(key,ray);continue; }
  if (priorActive) continue;
  const upcoming = ray.lethalAtMs > nowMs;
  if (!prior || (upcoming && prior.lethalAtMs <= nowMs) ||
      (upcoming && prior.lethalAtMs > nowMs && ray.lethalAtMs < prior.lethalAtMs) ||
      (!upcoming && prior.lethalAtMs <= nowMs && ray.lethalAtMs > prior.lethalAtMs)) bands.set(key, ray);
 }
 return [...bands.values()];
}

export function brontasGuidePlayer(view: BrontasGuideView, nowMs: number) {
 if (!brontasGuideFresh(view,nowMs) || (view.playerValidUntilMs && nowMs >= view.playerValidUntilMs) || !view.player) return undefined;
 const m=view.motion;
 if (!m) return view.player;
 const dx=m.to.x-m.from.x, dy=m.to.y-m.from.y, distance=Math.hypot(dx,dy);
 const travel=Math.min(distance, m.speed*Math.max(0,Math.min(nowMs,m.untilMs)-m.atMs)/1000);
 const p={...view.player,x:m.from.x+(distance ? dx/distance*travel : 0),y:m.from.y+(distance ? dy/distance*travel : 0)};
 // Leaving the local square hides the personal marker/timers until a new fix.
 return Math.abs(p.x)<1 && Math.abs(p.y)<1 ? p : undefined;
}

// Never keep the countdown of the previous cell after a position update.
export function brontasGuideFootTimers(view: BrontasGuideView, nowMs: number) {
 const player=brontasGuidePlayer(view,nowMs);
 if (!player || view.stage === 'map') return [];
 return (['horizontal','vertical'] as const).flatMap(axis => {
  const coordinate=axis === 'horizontal' ? player.y : player.x;
  const rays=(view.rays || []).filter(r => r.axis===axis && r.offset!==undefined && r.halfWidth!==undefined &&
   coordinate>=r.offset-r.halfWidth-1e-9 && coordinate<r.offset+r.halfWidth-1e-9 &&
   r.lethalAtMs>nowMs && (!r.endsAtMs || r.endsAtMs>nowMs));
  const deadline=Math.min(...rays.map(r=>r.lethalAtMs));
  return Number.isFinite(deadline) ? [{axis,lethalAtMs:deadline,seconds:((deadline-nowMs)/1000).toFixed(1)}] : [];
 });
}

export function brontasGuideTarget(view: BrontasGuideView,nowMs:number) {
 const player=brontasGuidePlayer(view,nowMs);
 if (!player || !view.target || view.stage==='map') return undefined;
 if (view.source==='packet' && (view.guidance!=='move' || !view.guidanceValidUntilMs || nowMs>=view.guidanceValidUntilMs)) return undefined;
 const target={...view.target};
 if(view.stepAxis==='horizontal') {if(Math.abs(target.y-player.y)>.08)return undefined;target.y=player.y;}
 if(view.stepAxis==='vertical') {if(Math.abs(target.x-player.x)>.08)return undefined;target.x=player.x;}
 if(view.stepAxis && Math.hypot(target.x-player.x,target.y-player.y)>2/3+1e-7)return undefined;
 return target;
}

export function brontasGuideNextTarget(view:BrontasGuideView,nowMs:number) {
 const target=brontasGuideTarget(view,nowMs);
 if(!target || !view.nextTarget || !view.stepAxis)return undefined;
 const next={...view.nextTarget};
 if(view.stepAxis==='horizontal')next.x=target.x;else next.y=target.y;
 const distance=Math.hypot(next.x-target.x,next.y-target.y);
 return distance>1e-7 && distance<=2/3+1e-7 ? next : undefined;
}

export function brontasGuideDirection(view:BrontasGuideView,nowMs:number):string {
 const target=brontasGuideTarget(view,nowMs),player=brontasGuidePlayer(view,nowMs);
 if (player && view.guidance==='pace' && (view.guidanceValidUntilMs || 0)>nowMs) {
  return view.paceUntilMs && view.paceUntilMs>nowMs ? `稍等 ${(Math.ceil((view.paceUntilMs-nowMs)/100)/10).toFixed(1)}秒` : '等脚下出线';
 }
 if (!target || !player) return view.guidance==='hold' && (view.guidanceValidUntilMs || 0)>nowMs && player ? '保持' : '观察';
 const dx=target.x-player.x,dy=target.y-player.y;
 if(Math.hypot(dx,dy)<.06) return '保持';
 const x=Math.abs(dx)>.04 ? dx>0?'右':'左' : '';
 const y=Math.abs(dy)>.04 ? dy>0?'上':'下' : '';
 return `${view.guidanceKind==='escape' ? '立即向' : '向'}${x}${y}`;
}

// A deliberate bait pause must look different from missing/stale guidance.
// The clock may expire a cue, but only a fresh planner result may say "go".
export function brontasGuideCue(view:BrontasGuideView,nowMs:number) {
 const unavailable=(detail:string)=>({mode:'unavailable',title:'暂无指引',detail,seconds:'',angle:0});
 if(!brontasGuideFresh(view,nowMs))return unavailable('信号中断');
 const player=brontasGuidePlayer(view,nowMs);
 if(!player)return unavailable('定位中');
 if(view.source==='packet' && (!view.guidanceValidUntilMs || nowMs>=view.guidanceValidUntilMs))return unavailable('等待更新');
 const wait=(detail:string,seconds='')=>({mode:'wait',title:'等拍',detail,seconds,angle:0});
 if(view.guidance==='pace') {
  if(view.paceUntilMs && nowMs>=view.paceUntilMs)return unavailable('等待更新');
  return view.paceUntilMs ? wait('起步准备',(Math.ceil((view.paceUntilMs-nowMs)/100)/10).toFixed(1)) : wait('等脚下出线');
 }
 if(view.guidance==='hold')return wait('等脚下出线');
 const target=brontasGuideTarget(view,nowMs);
 if(target) {
  const dx=target.x-player.x,dy=target.y-player.y;
  if(Math.hypot(dx,dy)<.06)return wait('等下一步');
  const urgent=view.guidanceKind==='escape';
  return {mode:urgent?'urgent':'go',title:urgent?'快躲':'移动',detail:brontasGuideDirection(view,nowMs).replace(/^立即/,''),seconds:'',angle:Math.atan2(-dy,dx)*180/Math.PI};
 }
 if(view.guidance==='blocked')return unavailable('暂无可用路线');
 if(player.speed<=0)return unavailable('等待测速');
 return unavailable(view.guidance==='move'?'位置变化':'等待更新');
}
