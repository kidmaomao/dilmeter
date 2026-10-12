import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const server = await createServer({ configFile: false, root, logLevel: 'error', resolve: { alias: { '@': path.join(root,'src') } }, server: { middlewareMode: true, hmr:false, ws:false }, optimizeDeps:{ noDiscovery:true, include:[] } });
try {
    const { holyEnergyState, energyReminderReady } = await server.ssrLoadModule('/src/skillEnergy.ts');
    assert.deepEqual(holyEnergyState({Metadata:'MCNPGIPT:f:0.5;'}),{Percent:0,Active:true});
    assert.equal(holyEnergyState({Metadata:'MCNPGV:f:100;',DisableAt:1},2000),undefined);
    assert.equal(holyEnergyState({Metadata:'MCNPGV:f:NaN;'}),undefined);
    const full={Percent:100,Active:true}, cd={usedAtMs:1000,readyAtMs:31000};
    assert.equal(energyReminderReady(59047,full,100,cd,30000),false);
    assert.equal(energyReminderReady(59047,full,100,undefined,32000),false);
    assert.equal(energyReminderReady(59047,full,100,cd,31000),true);
    assert.equal(energyReminderReady(59047,{Percent:95,Active:true},100,cd,32000),false);
    assert.equal(energyReminderReady(59085,full,100,undefined,32000),true);
    const { SKILL_COOLDOWN_STORAGE_KEY, loadSkillCooldownSettings, saveSkillCooldownSettings } = await server.ssrLoadModule('/src/skillCooldown.ts');
    const values = new Map();
    globalThis.localStorage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => values.set(key, value) };
    globalThis.window = { dispatchEvent() {} };
    // Old thresholds and the digits toggle must not bypass either usability gate.
    const darkCooldown = { usedAtMs: 100_000, readyAtMs: 130_000 };
    for (const showEnergyPercent of [true, false]) {
        localStorage.setItem(SKILL_COOLDOWN_STORAGE_KEY, JSON.stringify({
            coordinateVersion: 2,
            rules: { 59047: { skillId: 59047, progressThresholdPercent: 80, showEnergyPercent } },
        }));
        const migrated = loadSkillCooldownSettings();
        assert.equal(migrated.rules[59047].progressThresholdPercent, 100, 'old dark-energy thresholds migrate to full charge');
        migrated.rules[59047].progressThresholdPercent = 50;
        saveSkillCooldownSettings(migrated);
        assert.equal(loadSkillCooldownSettings().rules[59047].progressThresholdPercent, 100, 'saving cannot restore an early dark-energy threshold');
        assert.equal(loadSkillCooldownSettings().rules[59047].showEnergyPercent, showEnergyPercent);
        for (const threshold of [0, 80, 95, 100]) {
            assert.equal(energyReminderReady(59047, { Percent: 95, Active: true }, threshold, darkCooldown, 130_000), false, 'ready CD cannot bypass partial energy');
            assert.equal(energyReminderReady(59047, { Percent: 100, Active: true }, threshold, darkCooldown, 129_999), false, 'full energy cannot bypass CD');
            assert.equal(energyReminderReady(59047, { Percent: 100, Active: true }, threshold, undefined, 130_000), false, 'unobserved CD is not ready');
            assert.equal(energyReminderReady(59047, { Percent: 100, Active: false }, threshold, darkCooldown, 130_000), false, 'inactive gauge is not ready');
            assert.equal(energyReminderReady(59047, { Percent: 100, Active: true }, threshold, darkCooldown, 130_000), true, 'both gates ready at the exact CD boundary');
        }
    }
    assert.equal(energyReminderReady(59085, { Percent: 95, Active: true }, 95, undefined, 130_000), true, 'holy-energy warning threshold stays configurable');
    console.log('dark energy requires full charge and observed CD regardless of digits or legacy threshold');
    const { buildEventSnapshot } = await server.ssrLoadModule('/src/worker/buildEventSnapshot.ts');
    const { ActorManager } = await server.ssrLoadModule('/src/eventActor.ts');
    const { DamageCollectorManager } = await server.ssrLoadModule('/src/actionCollector.ts');
    const { hydrateFromSnapshot } = await server.ssrLoadModule('/src/worker/hydrateActorManager.ts');
    const events=[
        {EventId:1,At:100,Id:'player',Name:'player',RaceId:9001,OwnerId:'',GuildName:'',Height:1,Weight:1,Upper:1,Lower:1},
        {EventId:11,At:100,Id:'player',Reliable:true},
        {EventId:17,At:100,Id:'player',Private:true,Stats:[{StatId:28,Value:100},{StatId:30,Value:100}]},
        {EventId:6,At:102,Id:'player',AttackerId:'boss'},
        {EventId:17,At:104,Id:'player',Private:true,Stats:[{StatId:28,Value:80}]},
        {EventId:21,At:104,Id:'player',SkillId:59047,Percent:100,Active:true},
    ];
    const snapshot=buildEventSnapshot(events.map(e=>JSON.stringify(e)).join('\n'));
    const dc=new DamageCollectorManager(), live=new ActorManager(dc);
    events.forEach(e=>live.onEvent(e));
    assert.deepEqual(live.entityMap.player.vitalHistory,snapshot.entities.player.vitalHistory,'live and worker histories agree');
    assert.deepEqual(live.entityMap.player.vitalHistory.map(p=>p.dead),[false,true,false]);
    const restored=new ActorManager(new DamageCollectorManager());
    hydrateFromSnapshot(snapshot,restored,new DamageCollectorManager());
    assert.deepEqual(restored.entityMap.player.vitalHistory,live.entityMap.player.vitalHistory);
    assert.equal(restored.skillEnergy.player[59047].Percent,100);
    live.onEvent({EventId:11,At:110,Id:'player',Reset:true,Reliable:true});
    assert.deepEqual(live.skillEnergy,{});
    assert.equal(live.entityMap.player.vitalHistory.length,3,'connection reset preserves historical graph');
    console.log('1.5.2 verified: dual readiness, source conditions, live/worker death & HP, snapshot restoration, session reset.');
} finally { await server.close(); }
