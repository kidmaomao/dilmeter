import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createServer } from 'vite';
import { readFileSync } from 'node:fs';

globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.__IS_STANDALONE__ = false;
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const originalFetch = globalThis.fetch;
globalThis.fetch = async (url, options) => String(url).endsWith('.wasm')
    ? new Response(readFileSync(path.join(root, 'node_modules/brotli-dec-wasm/pkg/brotli_dec_wasm_bg.wasm')), { headers: { 'Content-Type': 'application/wasm' } })
    : originalFetch(url, options);
const server = await createServer({ configFile: false, root, logLevel: 'error',
    resolve: { alias: { '@': path.join(root, 'src') } },
    server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] } });
try {
    const { gameUiText: text, skillResourceName: skill } = await server.ssrLoadModule('/src/gameTerms.ts');
    const { resourceRegion, uiLocale } = await server.ssrLoadModule('/src/uiLocale.ts');
    const { skillNameMap, condNameMap, multiClassNameMap } = await server.ssrLoadModule('/src/store.ts');
    const { TW_GAME_TERMS: terms } = await server.ssrLoadModule('/src/data/twGameTerms.ts');
    const { buildArcanaKpi: build } = await server.ssrLoadModule('/src/arcanaKpi.ts');
    resourceRegion.value = 'tw'; uiLocale.value = 'zh-TW';
    assert.equal(skill(58014, '万钧之力'), '力量團聚');
    assert.equal(skill(59005, '崩坏波动'), '崩壞的波動');
    assert.equal(text('重炮炮火时的领域平均覆盖个数'), '重裝武力時的轉化領域平均覆蓋個數');
    assert.equal(text('闪电链开启期间龙炎占比'), '暗雷連結開啟期間龍焰佔比');
    assert.equal(text('逆龙'), '連續技能 : 狂暴拳擊');
    assert.equal(skill(65530, '不可信国服名称'), '技能 65530', 'unknown IDs must not invent TW names');
    for (const term of terms) for (const alias of term.aliases) assert.equal(text(alias), term.tw, `${term.kind} ${term.id}`);
    uiLocale.value = 'zh-CN';
    assert.equal(text('万钧之力'), '力量團聚', 'resource names follow server independently of interface language');
    skillNameMap.value = { 58014: '力量團聚（资源更新）' };
    assert.equal(skill(58014), '力量團聚（资源更新）', 'successfully loaded resource takes precedence');
    skillNameMap.value = {};
    uiLocale.value = 'zh-TW';
    const jobs = ['元素骑士','圣光颂唱者','黑魔导士','流星射手','圣盾骑士','爆裂骑士枪','枪炮师','禁术炼金师','旋律操纵师','狂怒斗士'];
    let rows = 0;
    for (const jobName of jobs) {
        const report = build({ jobName, player: { id: 'self', conditionHistory: [] }, boss: { id: 'boss', conditionHistory: [] },
            session: { bossEntityId: 'boss', startAt: 0, endAt: 30, totalDuration: 30, effectiveDuration: 30, inactiveDuration: 0, invincibleIntervals: [] }, actions: [] });
        assert.ok(report.rows.length, jobName);
        for (const row of report.rows) {
            const label = text(row.label), detail = text(row.detail);
            assert.ok(!/萬鈞|閃電鏈|重炮炮火|逆龍|魔法封鎖|多爾卡|聖光頌唱者/.test(label + detail), label);
            rows++;
        }
    }
    resourceRegion.value = 'cn'; uiLocale.value = 'zh-CN';
    assert.equal(text('万钧之力'), '万钧之力', 'CN presentation and persistence vocabulary remain unchanged');
    assert.equal(text('重炮炮火时的领域平均覆盖个数'), '重炮炮火时的领域平均覆盖个数');
    console.log(`TW terminology passed: ${terms.length} verified IDs, ${jobs.length} jobs, ${rows} KPI rows, offline fallback and region/language separation.`);
} finally { globalThis.fetch = originalFetch; await server.close(); }
