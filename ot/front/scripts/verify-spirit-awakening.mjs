import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

const values = new Map();
globalThis.localStorage = {
    getItem: key => values.get(key) ?? null,
    setItem: (key, value) => values.set(key, String(value)),
};
globalThis.window = new EventTarget();
const server = await createServer({
    configFile: false, root: fileURLToPath(new URL('..', import.meta.url)),
    logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false },
    optimizeDeps: { noDiscovery: true, include: [] },
});
try {
    const { SPIRIT_AWAKENING_SKILL_IDS, isEnergySkill, energyReminderReady } = await server.ssrLoadModule('/src/skillEnergy.ts');
    const { makeSkillCooldownRule, loadSkillCooldownSettings, saveSkillCooldownSettings } = await server.ssrLoadModule('/src/skillCooldown.ts');
    const { PureActorManager, PureDamageCollectorManager } = await server.ssrLoadModule('/src/worker/pureEventActor.ts');
    const protocols = await server.ssrLoadModule('/src/protocols.ts');
    const actor = new PureActorManager(new PureDamageCollectorManager());
    actor.onEvent({ EventId: protocols.eventIdLocalEntity, Id: 'self', At: 100, Reliable: true });
    const settings = loadSkillCooldownSettings();
    for (const id of SPIRIT_AWAKENING_SKILL_IDS) {
        assert.equal(isEnergySkill(id), true);
        const rule = makeSkillCooldownRule(id);
        assert.equal(rule.alwaysVisible, true);
        rule.progressThresholdPercent = 80;
        settings.rules[id] = rule;
        for (const [percent, ready] of [[0, false], [50, false], [99, false], [100, true]]) {
            const energy = { EventId: protocols.eventIdSkillEnergy, Id: 'self', At: 100, SkillId: id, Percent: percent, Active: true };
            actor.onEvent(energy);
            assert.equal(energyReminderReady(id, actor.skillEnergy.self[id], 80, undefined, 100000), ready);
        }
        actor.onEvent({ EventId: protocols.eventIdSkillAction, Id: 'self', At: 100, SkillId: id });
        assert.equal(actor.skillEnergy.self[id].Percent, 100, '58015 cast preserves charge until server consumes it');
        assert.equal(energyReminderReady(id, { Percent: 100, Active: false }, 100, undefined, 100000), false);
        assert.equal(energyReminderReady(id, undefined, 100, undefined, 100000), false);
    }
    saveSkillCooldownSettings(settings);
    const loaded = loadSkillCooldownSettings();
    for (const id of SPIRIT_AWAKENING_SKILL_IDS) assert.equal(loaded.rules[id].progressThresholdPercent, 100);
    actor.onEvent({ EventId: protocols.eventIdSkillEnergy, Id: 'self', At: 101, SkillId: 35130, Percent: 0, Active: true });
    assert.equal(energyReminderReady(35130, actor.skillEnergy.self[35130], 100, undefined, 999999999), false, 'spent charge cannot recover on a timer');
    assert.equal(actor.skillEnergy.self[35122].Percent, 100, 'other weapon charge remains independent');
    actor.onEvent({ EventId: protocols.eventIdLocalEntity, Id: 'self', At: 102, Reset: true, Reliable: true });
    assert.deepEqual(actor.skillEnergy, {}, 'same-character reconnect clears charge');
    assert.equal(energyReminderReady(59047, { Percent: 100, Active: true }, 100, undefined, 100000), false, 'dark energy still requires observed CD');
    assert.equal(energyReminderReady(59085, { Percent: 100, Active: true }, 100, undefined, 100000), true);
    console.log('Spirit awakening: 20 weapon skills, saved settings, worker charge, extra use and reconnect passed.');
} finally {
    await server.close();
}
