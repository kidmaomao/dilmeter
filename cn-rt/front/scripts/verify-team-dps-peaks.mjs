import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const server = await createServer({ configFile: false, root, logLevel: 'error',
    server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] } });
try {
    const { buildTeamDpsTimeline: build } = await server.ssrLoadModule('/src/teamDps.ts');
    const { explainDpsPeak: explain, selectDpsPeaks: select } = await server.ssrLoadModule('/src/teamDpsPeaks.ts');
    const player = { entityId: 'self', label: '圣盾骑士', damages: [
        { At: 100, Damage: 10, SkillId: 1 }, { At: 101, Damage: 20, SkillId: 1 },
        { At: 105, Damage: 40, SkillId: 1 },
        { At: 106, Damage: 300, SkillId: 59085, Conditions: [
            { CCId: 487, At: 102, AttackerId: 'self' },
            { CCId: 680, At: 102, AttackerId: 'ally', DisableAt: 106 }, // expired at the hit
            { CCId: 388, At: 102, AttackerId: 'self' }, // permanent / earlier buff hidden
            { CCId: 63, At: 106, AttackerId: 'self' },
        ], TargetConditions: [
            { CCId: 803, At: 103, AttackerId: 'ally' },
            { CCId: 913, At: 106, AttackerId: 'pet' },
            { CCId: 1093, At: 105 },
        ] },
        { At: 107, Damage: 100, SkillId: 58009, Conditions: [{ CCId: 487, At: 102, AttackerId: 'self' }], TargetConditions: [{ CCId: 803, At: 103, AttackerId: 'ally' }] },
        { At: 110, Damage: 100, SkillId: 59085 },
        { At: 111, Damage: 99999, SkillId: 1 },
    ] };
    const context = { actors: {
        self: { label: '圣盾骑士', isLocal: true }, ally: { label: '黑魔导士', isTeamMember: true }, pet: { label: '猫', ownerId: 'ally' },
    }, actions: [
        { Id: 'self', At: 102, SkillId: 58005 },
        { Id: 'ally', At: 108, SkillId: 58014 },
        { Id: 'ally', At: 103, SkillId: 59005 },
        { Id: 'ally', At: 108, SkillId: 53101, IsFallback: true },
    ] };
    const timeline = build([player], 100, 110, 1, { dpsMode: 'rolling', windowSeconds: 5 });
    for (const point of timeline.total.slice(1)) {
        const peak = explain(point, [player], 100, context);
        assert.equal(peak.damage / (point.custom.to - point.custom.from), point.y, 'peak attribution uses plotted window boundaries, including opening hit');
        assert.equal(peak.contributions.reduce((s, r) => s + r.damage, 0), peak.damage);
    }
    const peak = explain(timeline.total.at(-1), [player], 100, context);
    assert.equal(peak.damage, 500);
    assert.equal(peak.previousDps, 14);
    assert.deepEqual(peak.contributions.map(r => [r.skillId, r.damage, r.share, r.increase]), [[59085, 400, .8, 400], [58009, 100, .2, 100]]);
    assert.equal(peak.contributions[1].name, '连击');
    assert.equal(peak.effects.filter(e => e.name === '时间扭曲').length, 1, 'same effect on repeated hits is deduplicated');
    assert.equal(peak.effects.find(e => e.name === '时间扭曲').source, '自己');
    assert.equal(peak.effects.find(e => e.name === '崩坏波动').source, '队友 黑魔导士');
    assert.equal(peak.effects.find(e => e.name === '崩坏波动').target, 'Boss');
    assert.equal(peak.effects.find(e => e.name === '喵咪的奔袭').source, '队友 黑魔导士 的召唤物');
    assert.equal(peak.effects.find(e => e.name === '最大保护减少').source, '来源未记录');
    assert(!peak.effects.some(e => e.name === '战争序曲' || e.name === '最大伤害增加'));
    assert.equal(peak.effects.find(e => e.name === '万钧之力').evidence, 'action', 'cast alone must not claim active coverage');
    assert.equal(peak.effects.find(e => e.name === '万钧之力').target, '覆盖未确认');
    assert.equal(peak.effects.find(e => e.name === '攻击力增加').newlyApplied, true);
    assert.deepEqual(new Set(peak.effects.slice(0, 3).map(e => e.name)), new Set(['时间扭曲', '崩坏波动', '万钧之力']), 'named support effects remain visible before common conditions');
    const noAttacker = { ...player, damages: [{ At: 106, Damage: 1, Conditions: [{ CCId: 487, At: 102 }], TargetConditions: [{ CCId: 803, At: 103 }] }] };
    const inferred = explain(timeline.total.at(-1), [noAttacker], 100, context);
    assert.equal(inferred.effects.find(e => e.name === '时间扭曲').source, '自己', 'only confirmed self-use may fill missing self-condition attacker');
    assert.equal(inferred.effects.find(e => e.name === '崩坏波动').source, '来源未记录', 'do not infer team debuff attacker from nearby cast');
    const all = [{ ...player, damages: [{ At: 106, Damage: 300, SkillId: 1 }] }, { entityId: 'ally', label: '黑魔导士', damages: [{ At: 106, Damage: 700, SkillId: 1 }] }];
    assert.deepEqual(explain(timeline.total.at(-1), all, 100).contributions.map(r => [r.actorId, r.share]), [['ally', .7], ['self', .3]], 'team skill contributions retain distinct actors');
    const point = (x, y) => ({ x, y, custom: { elapsed: x, from: Math.max(0,x-5), to:x, damage:0, dashed:false } });
    const values = [point(0,0),point(5,100),point(6,100),point(7,0),point(10,200),point(11,0),point(20,50),point(21,0),point(40,20),point(41,0)];
    assert.deepEqual(select(values).map(p=>p.x), [10,20], 'stronger close peak wins; low noise is omitted');
    assert.deepEqual(select(values.slice(0,4)).map(p=>p.x), [6], 'flat maximum has one marker');
    assert.deepEqual(select([point(0,0),point(1,0)]), []);
    console.log('DPS peaks verified: exact windows, damage shares, combo, local/team/pet sources, active vs expired effects, uncertain coverage and separated peaks.');
} finally { await server.close(); }
