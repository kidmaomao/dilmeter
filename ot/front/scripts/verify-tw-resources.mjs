import assert from 'node:assert/strict';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { brotliCompressSync, brotliDecompressSync } from 'node:zlib';
import { readFileSync } from 'node:fs';
import { createServer } from 'vite';
import { indexedDB, IDBKeyRange } from 'fake-indexeddb';

const saved = new Map();
Object.assign(globalThis, { indexedDB, IDBKeyRange, __IS_STANDALONE__: false,
    localStorage: { getItem: key => saved.get(key) ?? null, setItem: (key, value) => saved.set(key, value) },
});
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const server = await createServer({ configFile: false, root, logLevel: 'error',
    resolve: { alias: { '@': path.join(root, 'src') } },
    server: { middlewareMode: true, hmr: false, ws: false }, optimizeDeps: { noDiscovery: true, include: [] },
});
const originalFetch = globalThis.fetch;
try {
    const { ResourceData } = await server.ssrLoadModule('/src/protos/resourceNames.ts');
    const twClasses = ['元素騎士', '聖詠者', '縛魂者', '秘術遊俠', '聖盾守衛', '爆裂槍兵', '幻變槍手', '禁忌鍊金士', '旋律人偶師', '狂怒鬥士'];
    const makeData = (region, version = 100) => ResourceData.create({
        Version: { CreatedAt: version },
        StringTable: [{ Id: 'skill', Str: region === 'tw' ? '潛伏點燃' : '国服技能' }, { Id: 'boss', Str: region === 'tw' ? '雷楠的米勒' : '国服首领' }, { Id: 'condition', Str: region === 'tw' ? '神聖護盾' : '国服状态' }],
        SkillList: [{ Id: 59047, Name: 'skill' }], RaceList: [{ Id: 7603, Name: 'boss' }],
        CharCondList: [{ Id: 1, Name: 'condition' }, { Id: 476, Name: region === 'tw' ? '要害貫通' : '致命穿透' }, { Id: 511, Name: '銳利' }, { Id: 680, Name: '戰場的序曲' }, { Id: 1120, Name: '賦予自然力' }], ItemList: [{ Id: 1, Name: 'condition' }],
        MultiClassList: twClasses.map((Name, index) => ({ Id: index + 1, Name, SkillBindingIds: [59000 + index * 20] })),
    });
    let offline = false, corrupt = false;
    const requests = [];
    globalThis.fetch = async url => {
        if (String(url).endsWith('.wasm')) return new Response(readFileSync(path.join(root,'node_modules/brotli-dec-wasm/pkg/brotli_dec_wasm_bg.wasm')), {headers:{'Content-Type':'application/wasm'}});
        requests.push(String(url));
        if (offline) throw new Error('offline');
        const region = String(url).includes('/tw/') ? 'tw' : 'cn';
        if (String(url).includes('resourceversion')) return Response.json({ CreatedAt: 100 });
        return new Response(corrupt ? new Uint8Array([0,1,2]) : brotliCompressSync(ResourceData.toBinary(makeData(region))));
    };
    const locale = await server.ssrLoadModule('/src/uiLocale.ts');
    const store = await server.ssrLoadModule('/src/store.ts');
    const { loadResourceNames, resourceNamesLoading } = await server.ssrLoadModule('/src/resourceNames.ts');
    const { bossDisplayName } = await server.ssrLoadModule('/src/bossDisplay.ts');
    const { normalizeSkillDisplayName } = await server.ssrLoadModule('/src/skillDisplay.ts');
    const { jobDisplayName, conditionDefinitions, conditionResourceName, jobDetailText, arcanaLabel } = await server.ssrLoadModule('/src/gameNameDisplay.ts');
    const { resVerCall, resDataCall } = await server.ssrLoadModule('/src/lib/apicall.ts');

    await loadResourceNames('cn');
    assert.equal(store.skillNameMap.value[59047], '烬火引燃');
    offline = true;
    await assert.rejects(loadResourceNames('tw'), /资料不完整/);
    assert.equal(locale.resourceRegion.value, 'cn', 'failed first download keeps selection');
    assert.equal(store.skillNameMap.value[59047], '烬火引燃', 'failed switch keeps existing names');
    assert.equal(resourceNamesLoading.value, false);

    offline = false;
    await loadResourceNames('tw');
    assert.equal(locale.resourceRegion.value, 'tw');
    assert.equal(store.region.value, 'tw');
    assert.equal(store.lang.value, 'tw');
    assert.equal(store.skillNameMap.value[59047], '潛伏點燃', 'TW skill must not be replaced by CN override');
    assert.equal(normalizeSkillDisplayName(59047, '潛伏點燃'), '潛伏點燃');
    assert.equal(conditionResourceName(511, '状态支援：锐利'), '銳利');
    assert.equal(conditionResourceName(680, '战争序曲'), '戰場的序曲');
    assert.equal(conditionResourceName(1120, '赐予自然力'), '賦予自然力');
    const defaults = [{ id: 476, name: '致命穿透', detail: '特性' }];
    const options = conditionDefinitions(defaults, { 476: '要害貫通' }, store.condNameMap.value, 'tw');
    for (const query of ['要害', '要害贯通', '要害貫通']) {
        assert(options.some(item => item.id === 476 && locale.normalizeNameSearch(item.name).includes(locale.normalizeNameSearch(query))), `TW search: ${query}`);
    }
    assert.equal(options.find(item => item.id === 476).name, conditionResourceName(476));
    assert.equal(options.find(item => item.id === 476).detail, '特性');
    assert.equal(defaults[0].name, '致命穿透', 'search never rewrites stored defaults');
    assert.equal(conditionDefinitions(defaults, {}, store.condNameMap.value, 'cn').find(item => item.id === 476).name, '致命穿透');
    const cnClasses = ['元素骑士', '圣光颂唱者', '黑魔导士', '流星射手', '圣盾骑士', '爆裂骑士枪', '枪炮师', '禁术炼金师', '旋律操纵师', '狂怒斗士'];
    assert.deepEqual(cnClasses.map(jobDisplayName), twClasses, 'all ten talents use MultiClassList');
    assert.equal(jobDisplayName('我的自定义职业'), '我的自定义职业');
    assert.equal(jobDetailText('圣盾骑士 / 枪炮师'), '聖盾守衛 / 幻變槍手');
    assert.equal(arcanaLabel(), '秘法才能');
    assert.equal(bossDisplayName({ raceId:7603, name:'123' },store.raceNameMap.value), '雷楠的米勒');
    assert.equal(saved.get('dilmeter-resource-region-v1'),'tw');
    assert(requests.includes('/res/resourcedata/tw/tw_resourcedata.bin.br'));
    assert(requests.includes('/local-res/resourcedata/cn/cn_resourcedata.bin.br'));

    locale.setUiLocale('zh-TW');
    assert.equal(locale.uiText('载入资料 / 保存设定 / 射线'), '載入資料 / 保存設定 / 射線');
    assert.equal(locale.normalizeNameSearch('神聖護盾'),locale.normalizeNameSearch('神圣护盾'));
    assert.equal(store.skillNameMap.value[59047],'潛伏點燃');
    locale.setUiLocale('zh-CN');
    assert.equal(locale.uiText('载入资料'),'载入资料');
    assert.equal(locale.resourceRegion.value,'tw','UI language never switches server data');

    // Expire the version check, then prove a failed online update still opens
    // the selected server cache without ever falling back to CN silently.
    const connection = await new Promise((resolve,reject) => { const r=indexedDB.open('prilus_mabi_db',2);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error); });
    await new Promise((resolve,reject) => { const tx=connection.transaction('version','readwrite');tx.objectStore('version').put(0,'LatestVersionCheckAt_tw');tx.oncomplete=resolve;tx.onabort=()=>reject(tx.error); });
    connection.close();
    offline = true;
    const cached = await loadResourceNames('tw');
    assert.equal(cached.cached,true);
    assert.equal(store.skillNameMap.value[59047],'潛伏點燃');
    await assert.rejects(loadResourceNames('tw',true),/offline/);
    assert.equal(store.skillNameMap.value[59047],'潛伏點燃');
    assert.equal(resourceNamesLoading.value,false);
    assert.equal(store.loadingCount.value,0,'failed requests balance loading state');

    offline = false; corrupt = true;
    await assert.rejects(loadResourceNames('tw',true));
    assert.equal(store.skillNameMap.value[59047],'潛伏點燃','corrupt data never replaces names');
    corrupt = false;
    await loadResourceNames('cn');
    assert.equal(store.skillNameMap.value[59047],'烬火引燃','switch back restores CN overrides');
    assert.equal(jobDisplayName('黑魔导士'), '黑魔导士');
    assert.equal(conditionResourceName(680, '战争序曲'), '战争序曲');
    assert.equal(arcanaLabel(), '阿尔卡纳职业');
    await resVerCall('resourceversion/tw/tw_resourceversion.json');
    await resDataCall('resourcedata/tw/tw_resourcedata.bin.br');
    assert.equal(store.loadingCount.value,0,'successful requests balance loading state');

    // Old releases cached the same version without decoding MultiClassList.
    // A fresh version timestamp must not prevent upgrading that partial cache.
    const oldCache = await new Promise((resolve,reject) => { const r=indexedDB.open('prilus_mabi_db',2);r.onsuccess=()=>resolve(r.result);r.onerror=()=>reject(r.error); });
    await new Promise((resolve,reject) => { const tx=oldCache.transaction('data','readwrite');tx.objectStore('data').delete('MultiClassList_tw');tx.oncomplete=resolve;tx.onabort=()=>reject(tx.error); });
    oldCache.close();
    const beforeUpgrade = requests.filter(url => url.includes('/resourcedata/tw/')).length;
    await loadResourceNames('tw');
    assert(requests.filter(url => url.includes('/resourcedata/tw/')).length > beforeUpgrade);
    assert.equal(store.multiClassNameMap.value[3], '縛魂者', 'old caches automatically acquire talent names');

    if (process.env.DILMETER_PRILUS_LIVE === '1') {
        const response = await originalFetch('https://mabires.pril.cc/resourcedata/tw/tw_resourcedata.bin.br', { signal: AbortSignal.timeout(30000) });
        assert.equal(response.status, 200);
        const compressed = new Uint8Array(await response.arrayBuffer());
        const actual = ResourceData.fromBinary(brotliDecompressSync(compressed));
        globalThis.fetch = async url => String(url).includes('resourceversion')
            ? Response.json(actual.Version) : new Response(compressed);
        const loaded = await loadResourceNames('tw', true);
        const strings = Object.fromEntries(actual.StringTable.map(row => [row.Id, row.Str]));
        const resolve = key => strings[key] || key;
        for (const row of actual.SkillList) assert.equal(store.skillNameMap.value[row.Id], resolve(row.Name).trim(), `skill ${row.Id}`);
        for (const row of actual.CharCondList) {
            assert.equal(store.condNameMap.value[row.Id], resolve(row.Name), `condition ${row.Id}`);
            assert.equal(conditionResourceName(row.Id, '国服名称'), resolve(row.Name).trim() || '国服名称');
        }
        for (const row of actual.MultiClassList) assert.equal(store.multiClassNameMap.value[row.Id], resolve(row.Name), `talent ${row.Id}`);
        assert.deepEqual(cnClasses.map(jobDisplayName), twClasses);
        console.log('Live Prilus TW verified:', JSON.stringify({ ...loaded, talents: store.multiClassNameMap.value,
            skills: [59047, 59104, 59145, 58101].map(id => [id, store.skillNameMap.value[id]]),
            conditions: [511, 680, 1120].map(id => [id, store.condNameMap.value[id]]) }));
    }
    console.log('TW resources verified: server isolation, official names, language independence, search, persistence, offline cache, failed/corrupt downloads and CN restoration.');
} finally { globalThis.fetch=originalFetch; await server.close(); }
