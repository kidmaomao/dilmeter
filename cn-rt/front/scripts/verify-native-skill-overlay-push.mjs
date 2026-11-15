import assert from 'node:assert/strict';
import { createServer } from 'vite';
import { fileURLToPath } from 'node:url';

const server = await createServer({ configFile: false, root: fileURLToPath(new URL('..', import.meta.url)), logLevel: 'error', server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true } });
try {
    const { createSkillOverlayMessageGate, subscribeNativeSkillOverlayPush, NATIVE_SKILL_OVERLAY_EVENT } = await server.ssrLoadModule('/src/nativeSkillOverlayPush.ts');
    const target = new EventTarget();
    const accept = createSkillOverlayMessageGate();
    const applied = [];
    const stop = subscribeNativeSkillOverlayPush(target, message => { if (accept(message)) applied.push(message); });
    const send = message => target.dispatchEvent(new CustomEvent(NATIVE_SKILL_OVERLAY_EVENT, { detail: message }));
    const state = (revision, atMs = 1000, mechanics = []) => ({ type: 'skill-cooldown-state', revision, atMs, items: [], mechanics });
    const tornado = [{ key: 'perlon-tornado', endsAtMs: 4000 }];
    send(state(2, 1000, tornado));
    assert.equal(applied.length, 1, 'native delivery applies synchronously without a clock/poll timer');
    assert.equal(applied[0].mechanics, tornado);
    assert.equal(accept(state(1)), false, 'old HTTP response in the same millisecond must not clear the warning');
    assert.equal(accept(state(2)), false, 'duplicate recovery snapshot need not rerender');
    assert.equal(accept(state(undefined, 2000)), false, 'legacy broadcast cannot replace canonical native state');
    send(state(3, 999, tornado));
    assert.equal(applied.length, 2, 'wall-clock correction cannot freeze monotonic native revisions');
    assert.equal(accept(state(4, 1000)), true, 'new HTTP snapshot recovers a lost native push');
    send(state(3, 1001, tornado));
    assert.equal(applied.length, 2, 'late push cannot resurrect an older alert');
    for (const invalid of [null, {}, { type: 'other', atMs: 1000 }, state(5, NaN), state(-1), state(1.5), state(Infinity)]) send(invalid);
    assert.equal(applied.length, 2);
    stop();
    send(state(5));
    assert.equal(applied.length, 2, 'unmounted overlay stops receiving native events');
    const legacy = createSkillOverlayMessageGate();
    assert.equal(legacy(state(undefined, 1000)), true);
    assert.equal(legacy(state(undefined, 999)), false);
    assert.equal(legacy(state(undefined, 1000)), true, 'older backend without revisions remains usable');
    console.log('Native overlay push: immediate delivery, same-ms stale responses, clock correction, HTTP recovery and listener cleanup passed.');
} finally { await server.close(); }
