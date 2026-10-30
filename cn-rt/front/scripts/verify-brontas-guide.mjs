import assert from 'node:assert/strict';
import { brontasGuideFresh, brontasGuideDeadline } from '../src/brontasGuide.ts';
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
console.log('Brontas guide: render-time expiry, absolute countdown, legacy defaults and persistence verified.');
