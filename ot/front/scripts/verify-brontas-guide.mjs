import assert from 'node:assert/strict';
import { brontasGuideFresh, brontasGuideDeadline, brontasGuideMapRays, brontasGuideFootTimers, brontasGuidePlayer, brontasGuideTarget, brontasGuideNextTarget, brontasGuideDirection, brontasGuideCue } from '../src/brontasGuide.ts';
import { normalizeBossMechanicAlertSettings as normalize, saveBossMechanicAlertSettings as save, loadBossMechanicAlertSettings as load } from '../src/bossMechanicAlert.ts';

const view = {status:'move', instruction:'move', detail:'', clock:12, validUntilMs:1400, lethalAtMs:2600};
assert.equal(brontasGuideFresh(view,1399),true);
assert.equal(brontasGuideFresh(view,1400),false);
assert.equal(brontasGuideDeadline(view,1000),'1.6 s');
assert.equal(brontasGuideDeadline(view,1400),'--','expired route must not leave a reassuring timer');
assert.equal(brontasGuideDeadline({...view,validUntilMs:3000},2600),'已致命');
assert.equal(brontasGuideDeadline({...view,lethalAtMs:undefined},1100),'--');
const storage=new Map();
globalThis.localStorage={getItem:key=>storage.get(key)??null,setItem:(key,value)=>storage.set(key,value)};
const settings=normalize(null);
assert.equal(settings.brontasRocks.guidanceEnabled,false,'legacy settings must not silently enable movement advice');
Object.assign(settings.brontasRocks,{enabled:true,guidanceEnabled:true,x:-450,y:230,rotation:270});
save(settings);
assert.deepEqual(load().brontasRocks,settings.brontasRocks);
const rays = [
 {id:'old',axis:'horizontal',offset:0,halfWidth:.077,warningAtMs:1000,lethalAtMs:2600,endsAtMs:0},
 {id:'next',axis:'horizontal',offset:0,halfWidth:.077,warningAtMs:1600,lethalAtMs:3200,endsAtMs:0},
 {id:'cross',axis:'vertical',offset:.2,halfWidth:.077,warningAtMs:2000,lethalAtMs:3600,endsAtMs:0},
];
const live={...view,source:'packet',player:{x:.2,y:0,radius:.055,speed:0},rays,validUntilMs:4500};
assert.equal(brontasGuideDeadline(live,2700),'0.5 s','history must not hide the next real deadline');
assert.equal(brontasGuideDeadline(live,3300),'0.3 s');
assert.equal(brontasGuideDeadline(live,3700),'--','past records must not imply a permanent active hit');
assert.equal(brontasGuideDeadline(live,4500),'--','stalled stream expires independently');
assert.deepEqual(brontasGuideMapRays(live,2700).map(r=>r.id),['next','cross']);
assert.deepEqual(brontasGuideMapRays(live,3700).map(r=>r.id),['next','cross']);
console.log('Brontas guide: render-time expiry, absolute countdown, legacy defaults and persistence verified.');

const cellRays=[
 {id:'h0',axis:'horizontal',offset:0,halfWidth:1/3,warningAtMs:1000,lethalAtMs:2600,endsAtMs:0},
 {id:'v0',axis:'vertical',offset:0,halfWidth:1/3,warningAtMs:1000,lethalAtMs:2800,endsAtMs:0},
 {id:'v1',axis:'vertical',offset:2/3,halfWidth:1/3,warningAtMs:1000,lethalAtMs:3200,endsAtMs:0},
 {id:'remote-earlier',axis:'horizontal',offset:-2/3,halfWidth:1/3,warningAtMs:1000,lethalAtMs:2200,endsAtMs:0},
];
const local={...live,stage:'station',player:{x:0,y:0,radius:.055,speed:0},playerValidUntilMs:2700,rays:cellRays};
assert.deepEqual(brontasGuideFootTimers(local,2000).map(t=>[t.axis,t.seconds]),[['horizontal','0.6'],['vertical','0.8']]);
assert.deepEqual(brontasGuideFootTimers({...local,player:{...local.player,x:.7,y:.7}},2000).map(t=>[t.axis,t.seconds]),[['vertical','1.2']],'new cell replaces old timers without a new ray');
assert.deepEqual(brontasGuideFootTimers({...local,player:{...local.player,x:1/3,y:.7}},2000).map(t=>t.seconds),['1.2'],'boundary has one owner');
assert.deepEqual(brontasGuideFootTimers(local,2700),[],'stale position drops personal timers even with live rays');
assert.equal(brontasGuidePlayer(local,2700),undefined);
assert.deepEqual(brontasGuideFootTimers({...local,player:undefined},2000),[]);
assert.deepEqual(brontasGuideFootTimers({...local,stage:'map',player:undefined,rays:[]},2000),[]);
console.log('Brontas local grid: independent foot timers, cell switch, boundary and position expiry verified.');

assert.equal(normalize(null).brontasRocks.facing,'outward');
assert.equal(normalize({brontasRocks:{rotation:90}}).brontasRocks.facing,'outward','default follows the requested outward view');
for (const facing of ['outward','inward','north','south']) {
 const options=normalize({brontasRocks:{facing}});save(options);assert.equal(load().brontasRocks.facing,facing);
}
assert.equal(normalize({brontasRocks:{facing:'invalid'}}).brontasRocks.facing,'outward');
const guidance={...local,guidance:'move',guidanceValidUntilMs:2300,target:{x:-2/3,y:2/3}};
assert.deepEqual(brontasGuideTarget(guidance,2000),guidance.target);
assert.equal(brontasGuideDirection(guidance,2000),'向左上');
assert.equal(brontasGuideTarget(guidance,2300),undefined,'never retain a route after its safe departure window');
assert.equal(brontasGuideDirection(guidance,2300),'观察');
assert.equal(brontasGuideTarget({...guidance,playerValidUntilMs:1900},2000),undefined);
assert.equal(brontasGuideTarget({...guidance,guidance:'blocked'},2000),undefined);
assert.equal(brontasGuideDirection({...guidance,target:undefined,guidance:'hold'},2000),'保持');
console.log('Facing presets and independently expiring movement target verified.');

const moving={...local,playerValidUntilMs:1700,validUntilMs:4000,motion:{from:{x:.25,y:0},to:{x:.8,y:0},atMs:1000,untilMs:1700,speed:1}};
assert.equal(brontasGuidePlayer(moving,1100).x,.35);
assert.equal(brontasGuideFootTimers(moving,1100).find(t=>t.axis==='vertical').lethalAtMs,3200,'crossing a boundary switches countdown between packets');
assert.equal(brontasGuidePlayer(moving,1600).x,.8,'stop at destination');
assert.equal(brontasGuidePlayer(moving,1700),undefined,'stale motion must disappear');
assert.equal(brontasGuidePlayer({...moving,motion:undefined,player:{...local.player,x:-.6}},1100).x,-.6,'new position applies immediately');
const finite={...live,rays:[{...rays[0],endsAtMs:3200},{...rays[1],endsAtMs:3800}]};
assert.deepEqual(brontasGuideMapRays(finite,3800),[],'expired bands are removed');
console.log('Bounded motion, immediate correction and per-frame foot-cell switching verified.');

assert.equal(brontasGuideDirection({...guidance,target:{x:0,y:.10}},2000),'向上','short steps must still name a direction');

const corner={...guidance,stepAxis:'horizontal',target:{x:.48,y:0},nextTarget:{x:.48,y:-.48}};
assert.equal(brontasGuideDirection(corner,2000),'向右');
assert.deepEqual(brontasGuideNextTarget(corner,2000),{x:.48,y:-.48});
const sideways={...corner,player:{...local.player,x:.12,y:.02}};
assert.deepEqual(brontasGuideTarget(sideways,2000),{x:.48,y:.02},'animation keeps first leg horizontal');
assert.deepEqual(brontasGuideNextTarget(sideways,2000),{x:.48,y:-.48},'second leg stays perpendicular');
assert.equal(brontasGuideTarget({...sideways,player:{...sideways.player,y:.2}},2000),undefined,'invalidate significant off-route drift');
assert.equal(brontasGuideNextTarget(corner,2300),undefined,'expiry also drops second leg');
assert.equal(brontasGuideTarget({...corner,target:{x:.9,y:0}},2000),undefined,'never display a multi-cell step');
assert.equal(brontasGuideDirection({...corner,guidanceKind:'escape'},2000),'立即向右');
console.log('Orthogonal one-cell steps, perpendicular next leg, drift and expiry verified.');

const pacing={...corner,guidance:'pace',paceUntilMs:2200};
assert.equal(brontasGuideDirection(pacing,2000),'稍等 0.2秒');
assert.equal(brontasGuideDirection(pacing,2151),'稍等 0.1秒');
assert.equal(brontasGuideDirection({...pacing,paceUntilMs:0},2000),'等脚下出线');
assert.equal(brontasGuideTarget(pacing,2000),undefined,'a queued route is not a movement cue');
assert.equal(brontasGuideNextTarget(pacing,2000),undefined);
assert.equal(brontasGuideTarget(pacing,2250),undefined,'clock alone must not release an unchecked arrow');
assert.equal(brontasGuideDirection({...pacing,guidanceValidUntilMs:2200},2200),'观察');
assert.equal(brontasGuideDirection({...pacing,playerValidUntilMs:1900},2000),'观察');
assert.equal(brontasGuideTarget({...pacing,guidance:'move'},2250).x,.48,'fresh backend release enables the preserved route');
console.log('Bait-pause countdown, waiting cue, freshness and backend-only release verified.');

assert.deepEqual(brontasGuideCue(pacing,2000),{mode:'wait',title:'等拍',detail:'起步准备',seconds:'0.2',angle:0});
assert.equal(brontasGuideCue({...pacing,paceUntilMs:0},2000).detail,'等脚下出线');
assert.equal(brontasGuideCue(pacing,2200).title,'暂无指引','zero countdown cannot say go or intentional wait');
assert.equal(brontasGuideCue({...pacing,guidanceValidUntilMs:1900},2000).mode,'unavailable');
assert.equal(brontasGuideCue({...pacing,validUntilMs:1900},2000).detail,'信号中断');
assert.equal(brontasGuideCue({...pacing,playerValidUntilMs:1900},2000).detail,'定位中');
assert.equal(brontasGuideCue({...corner,guidance:'blocked'},2000).detail,'暂无可用路线');
assert.equal(brontasGuideCue({...corner,guidance:'waiting'},2000).detail,'等待测速');
assert.equal(brontasGuideCue(corner,2000).title,'移动');
assert.equal(brontasGuideCue({...corner,guidanceKind:'escape'},2000).title,'快躲');
assert.equal(brontasGuideCue({...corner,player:{...corner.player,y:.2}},2000).mode,'unavailable','off-axis stale arrow is not a go cue');
console.log('Explicit wait, go, urgent and unavailable actions verified.');
