import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';
const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)), logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true } });
try {
    const { createSkillReminderPreview, requestReminderPreview } = await server.ssrLoadModule('/src/reminderPreview.ts');
    const { makeSkillCooldownRule } = await server.ssrLoadModule('/src/skillCooldown.ts');
    for (const id of [59000, 59145, 27000, 27012, 59047, 59085, 35111, 35130]) {
        const rule = makeSkillCooldownRule(id, -200, 350);
        rule.enabled = false;
        const before = JSON.stringify(rule), preview = createSkillReminderPreview(rule, '测试', 'icon.png', 100000);
        assert.equal(JSON.stringify(rule), before, 'preview cannot modify settings or enable a disabled rule');
        const item = (preview.items ?? preview.stackAlerts)[0];
        assert.equal(item.previewExpiresAtMs, 103600);
        assert.equal(item.x, -200);
        assert.equal(item.generation, 100000);
        if (id === 59000) assert.equal(item.readyAtMs, 100900);
        if ([59047, 59085, 35111, 35130].includes(id)) assert.equal(item.energyReady, true);
        if (id === 27000) assert.equal(item.persistent, true);
    }
    const calls = [];
    let finishFirst;
    globalThis.fetch = async (url, init) => {
        calls.push({ url, body: JSON.parse(init.body) });
        if (calls.length === 1) await new Promise(resolve => { finishFirst = resolve; });
        return { ok: true };
    };
    const first = requestReminderPreview({ items: [{ skillId: 59000 }] }, 3600, 100000);
    const second = requestReminderPreview({ stackAlerts: [{ ccId: 1 }] }, 3000, 100010);
    await new Promise(resolve => setTimeout(resolve, 0));
    assert.equal(calls.length, 1, 'rapid clicks are sent in order');
    finishFirst();
    await Promise.all([first, second]);
    assert.equal(calls.length, 2);
    assert.equal(calls[1].url, '/api/reminder_preview');
    assert.equal(calls[1].body.items, undefined, 'one preview must not replace unrelated live/preview items');
    assert.equal(calls[1].body.atMs, 100010);
    globalThis.fetch = async () => ({ ok: false, text: async () => 'rejected' });
    await assert.rejects(requestReminderPreview({ items: [] }, 3600), /rejected/);
    globalThis.fetch = async () => ({ ok: true });
    await requestReminderPreview({ items: [{ skillId: 59000 }] }, 3600);
    console.log('Reminder previews: independent payloads, disabled-rule samples, energy/quantity variants, ordered clicks and error recovery passed.');
} finally { await server.close(); }
