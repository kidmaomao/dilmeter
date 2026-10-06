import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "vite";
const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const server = await createServer({ configFile: false, root, logLevel: "error",
    server: { middlewareMode: true, hmr: false }, optimizeDeps: { noDiscovery: true, include: [] },
    resolve: { alias: { "@": path.join(root, "src") } } });
globalThis.localStorage = { getItem: () => null, setItem() {} };
globalThis.window = { dispatchEvent() {} };
try {
    const { buildArcanaKpi: build, kpiMetadataNumber, hydroChargeIsFull } = await server.ssrLoadModule("/src/arcanaKpi.ts");
    const { normalizeSkillDisplayName } = await server.ssrLoadModule("/src/skillDisplay.ts");
    assert.equal(normalizeSkillDisplayName(59165, "旧名称"), "间奏斩");
    assert.equal(normalizeSkillDisplayName(24103), "逆龙袭");
    assert.equal(normalizeSkillDisplayName(59185), "疾风突刺");
    const session = { bossEntityId: "boss", startAt: 0, endAt: 30, totalDuration: 30,
        invincibleIntervals: [{ start: 0, end: 29 }], effectiveDuration: 1, inactiveDuration: 29 };
    const condition = (cc, at, metadata = "", extra = {}) => ({ Id: "player", At: at, CCId: cc,
        DisableAt: 0, AttackerId: "player", Metadata: metadata, ...extra });
    const actor = (id, states = []) => ({ id, conditionHistory: states.map(([at, list]) => ({ At: at, List: list })) });
    const boss = actor("boss", [[0, []], [10, [condition(277, 10)]], [20, []]]);
    const player = actor("player", [[0, []]]);
    const action = (skill, at, extra = {}) => ({ EventId: 10, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: skill, SubSkillId: 0, CombatActionId: at * 100 + skill, SourceId: "player", IsFallback: false, ...extra });
    const input = { session, boss, player, jobName: "元素骑士", actions: [], dorchaMinimum: .5 };
    const metric = (report, id) => report.rows.find((row) => row.id === id);
    assert.equal(build(input).activeSeconds, 20, "only recorded immunity is removed; arbitrary damage gaps stay");
    assert.equal(build(input).excludedSeconds, 10);
    const expiredBoss = actor("boss", [[0, [condition(494, 0, "", { DisableAtMs: 5000 })]]]);
    assert.equal(build({ ...input, boss: expiredBoss }).activeSeconds, 25, "expiry works without a disable packet");
    const overlapping = actor("boss", [[0, [condition(277, 0), condition(494, 0)]], [5, []]]);
    assert.equal(build({ ...input, boss: overlapping }).excludedSeconds, 5, "overlapping immunity is counted once");
    const combo = actor("player", [[0, [condition(321, 0, "MCRSTKCNT:4:2;")]],
        [5, [condition(321, 5, "MCRSTKCNT:4:4;")]], [25, []]]);
    assert.equal(metric(build({ ...input, player: combo }), "combo").value, 50 / 15, "stacks use the captured CN field and time weighting");
    assert.equal(kpiMetadataNumber("A:f:NaN;", "A"), null);
    assert.equal(kpiMetadataNumber("AB:f:10;", "A"), null);
    const pairActions = [action(59026, 1), action(59028, 3), action(59028, 3), action(59028, 4, { IsFallback: true }),
        action(59026, 8), action(59028, 12), action(59026, 22), action(59028, 23), action(59028, 24), action(59026, 29)];
    const pair = metric(build({ ...input, actions: pairActions }), "rush");
    assert.equal(pair.samples, 2); assert.equal(pair.value, 1.5, "deduplicates casts, ignores fallback and rejects pairs crossing immunity");
    const chainedBoss = actor("boss", [[0, []], [3, [condition(948, 3, "", { DisableAtMs: 6000, AttackerId: "teammate" })]], [10, [condition(277, 10)]], [20, []]]);
    const darkReport = (extra = {}) => build({ ...input, jobName: "黑魔导士", ...extra });
    const chainState = (at, active, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: 59041, Signal: "lightning-chain-state", Kind: 814, Count: active ? 1 : 0, Complete: true, ...extra });
    const dragonActions = [action(59040, 2), action(59040, 4), action(59040, 4), action(59040, 7), action(59040, 12),
        action(59040, 5, { Id: "other" }), action(59040, 5.5, { SourceId: "other" }), action(59040, 6, { IsFallback: true })];
    const dark = darkReport({ boss: chainedBoss, actions: dragonActions, arcanaSignals: [chainState(3, true), chainState(6, false)] });
    assert.ok(Math.abs(metric(dark, "dragon-chain").value - 100 / 3) < 1e-8, "own active-chain releases divided by own total releases; duplicates and immune casts excluded");
    assert.equal(metric(dark, "dragon-chain").samples, 3);
    assert.equal(metric(dark, "dragon-chain").unit, "%");
    assert.equal(metric(dark, "dragon"), undefined);
    assert.equal(metric(dark, "seal"), undefined, "old shared-debuff idle rate is removed");
    assert.equal(metric(darkReport({ boss: actor("boss", [[0, []]]), actions: [action(59040, 2)], arcanaSignals: [chainState(1, false)] }), "dragon-chain").value, 0);
    assert.equal(metric(darkReport({ boss: actor("boss"), actions: [action(59040, 2)] }), "dragon-chain").status, "missing-data");
    assert.equal(metric(darkReport({ actions: [action(30102, 2)], arcanaSignals: [chainState(1, false)] }), "dragon-chain").value, null, "no dragon casts is not 0% coverage");
    assert.equal(metric(darkReport({ actions: [action(59040, 30)], arcanaSignals: [chainState(20, true)] }), "dragon-chain").value, 100, "chain active at the final release is included");
    assert.equal(metric(darkReport({ actions: [action(59040, 30)], arcanaSignals: [chainState(20, true), chainState(30, false)] }), "dragon-chain").value, 0, "actual end at the exact release is honored");
    const chainRate = (signals, extra = {}) => metric(darkReport({ boss: actor("boss", [[0, []]]), actions: [action(59040, 13), action(59040, 18)], arcanaSignals: signals, ...extra }), "dragon-chain");
    assert.equal(chainRate([chainState(1, true), chainState(13, false)]).value, 0, "a 12-second active lifetime does not extend to the 17-second cooldown");
    assert.equal(chainRate([chainState(1, true), chainState(18, false)]).value, 50, "a strengthened 17-second lifetime is taken from its own stop signal");
    assert.equal(chainRate([chainState(1, false), chainState(2, true, { Id: "other" })]).value, 0, "teammate chain cannot satisfy the player's own active state");
    assert.equal(chainRate([chainState(1, false)], { boss: chainedBoss, actions: [action(59040, 4)] }).value, 0, "target CC 948 does not prove the player enabled Chain");
    assert.equal(chainRate([chainState(1, true)], { boss: actor("boss"), actions: [action(59040, 4)] }).value, 100, "own Chain lifetime is independent of target condition history");
    assert.equal(chainRate([chainState(31, true)]).status, "missing-data", "a later fight's state does not establish current observations");
    assert.equal(chainRate([chainState(1, true, { Complete: false })]).status, "missing-data");
    assert.equal(chainRate([chainState(1, true, { Kind: 948 })]).status, "missing-data");
    const thunder = metric(darkReport({ actions: [action(59040, 2), action(30102, 2.5), action(30102, 2.5),
        action(30101, 3), action(30104, 4), action(30102, 5.5), action(30102, 6, { IsFallback: true }),
        action(30102, 6.5, { Id: "other" }), action(59040, 7), action(30102, 8), action(59040, 22), action(59040, 29)] }), "thunder");
    assert.equal(thunder.value, 1);
    assert.equal(thunder.samples, 2, "ordinary Thunder releases, not damage segments or other lightning skills; zero-count complete interval included, immunity-spanning pair excluded");
    assert.equal(metric(darkReport({ actions: [action(59040, 1), action(30102, 2, { CombatActionId: 100 }), action(59040, 7)] }), "thunder").status, "missing-data", "known old Thunder mode-flag records must not show an undercounted average");
    const cooldown = (at, extra = {}) => ({ EventId: 20, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: 59045, Reset: true, ReduceMs: 0, ...extra });
    const sealReport = (times, extra = {}) => darkReport({ boss: actor("boss", [[0, []]]),
        session: { ...session, endAt: 160 }, actions: times.map((at) => action(59045, at)), ...extra });
    const sealMetric = (times, extra = {}) => metric(sealReport(times, extra), "seal-wait");
    const waitBoss = actor("boss", [[0, []], [10, [condition(277, 10)]], [50, []], [60, [condition(494, 60)]], [90, []]]);
    const waitActions = [0, 52, 95, 125, 160].map((at) => action(59045, at));
    const waitInput = { boss: waitBoss, actions: [...waitActions, waitActions[1], action(59045, 31, { Id: "other" }),
        action(59045, 32, { SourceId: "other" }), action(59045, 34, { IsFallback: true })] };
    const wait = sealMetric([], waitInput);
    assert.equal(wait.value, 3, "ready-to-next-cast waits are 2,5,0,5 seconds; cooldown continues through immunity");
    assert.equal(wait.samples, 4);
    assert.equal(wait.unit, "秒");
    assert.equal(metric(sealReport([], waitInput), "piercing").status, "missing-data", "seal wait is independent of missing Dorcha history");
    const sharedSealBoss = actor("boss", waitBoss.conditionHistory.map((state) => [state.At,
        [...state.List, condition(931, 0, "", { AttackerId: "other" })]]));
    assert.deepEqual(sealMetric([], { ...waitInput, boss: sharedSealBoss, dorchaMinimum: undefined }), wait, "shared seal debuff and shortage threshold do not affect own cooldown wait");
    assert.equal(sealMetric([0, 30, 60]).value, 0, "every use immediately after cooldown gives an observed zero");
    assert.equal(sealMetric([0, 60], { boss: actor("boss", [[0, []], [35, [condition(277, 35)]], [50, []]]) }).value, 15, "immunity only removes its overlap with already-ready waiting");
    assert.equal(sealMetric([0, 70], { boss: actor("boss", [[0, []], [30, [condition(277, 30)]], [70, []]]) }).value, 0, "using immediately when an immune boss returns incurs no wait");
    assert.equal(sealMetric([0, 12, 45], { boss }).value, 3, "immune-period release is not a scored use but still establishes the next cooldown");
    assert.equal(sealMetric([0, 25], { skillCooldowns: [cooldown(10)] }).value, 15, "own reset advances readiness");
    const reduction = cooldown(20, { Reset: false, ReduceMs: 7000 });
    assert.equal(sealMetric([0, 28], { skillCooldowns: [reduction, reduction, cooldown(5, { Id: "other" }), cooldown(7, { SkillId: 30102 })] }).value, 5, "reduction is deduplicated and owner/skill-specific");
    assert.equal(sealMetric([0, 40], { skillCooldowns: [cooldown(35)] }).value, 10, "late reset cannot erase earlier idle time");
    assert.equal(sealMetric([0, 25], { boss, skillCooldowns: [cooldown(5)] }).value, 10, "reset-adjusted idle time also excludes immunity");
    const unknownSeal = sealMetric([0, 20, 55]);
    assert.equal(unknownSeal.value, 5); assert.equal(unknownSeal.samples, 1);
    assert.ok(unknownSeal.detail.includes("1 个周期短于已知冷却"));
    assert.equal(sealMetric([0, 20]).status, "missing-data", "unexplained short cycle is not reported as a perfect zero wait");
    assert.equal(sealMetric([0]).value, null, "first and unfinished last waits have no measured origin/end");
    assert.equal(sealMetric([]).status, "missing-data");
    const sameMsActions = [action(59045, 1, { Sequence: 10 }), action(59045, 20, { Sequence: 30 })];
    assert.equal(sealMetric([], { actions: sameMsActions, skillCooldowns: [cooldown(1, { Sequence: 11 }), cooldown(20, { Sequence: 29 })] }).value, 19, "same-millisecond reset ordering is taken from server event sequence");
    const stat = (at, value) => ({ EventId: 17, At: at, Id: "player", Private: true, Stats: [{ StatId: 196, Value: value }] });
    const darkBoss = actor("boss", [[0, []], [6, [condition(931, 6)]], [10, [condition(277, 10)]], [20, []]]);
    const piercingPlayer = actor("player", [[0, [condition(938, 0, "", { DisableAtMs: 5000 })]]]);
    const quantities = [stat(-1, .5), stat(4, .49), stat(8, 2), stat(15, 0), stat(25, 3)];
    const shortage = build({ ...input, jobName: "黑魔导士", boss: darkBoss, player: piercingPlayer, statUpdates: quantities });
    assert.equal(metric(shortage, "piercing").value, 40, ".5 is sufficient, only .49 is insufficient, and expiry is honored");
    assert.equal(metric(shortage, "seal-wait").value, null, "seal debuff is not a release record");
    const reset = build({ ...input, jobName: "黑魔导士", statUpdates: [stat(-1, 0), stat(6, -1), stat(22, 1)] });
    assert.equal(metric(reset, "piercing").status, "missing-data", "reset must not carry a pre-channel quantity forward");
    const sacred = actor("player", [[0, [condition(1179, 0, "MCNPGV:f:0;")]], [8, [condition(1179, 8, "MCNPGV:f:50;")]],
        [22, [condition(1179, 22, "MCNPGV:f:100;")]], [23, [condition(1179, 23, "MCNPGV:f:40;")]]]);
    const charge = metric(build({ ...input, player: sacred, jobName: "圣盾骑士" }), "sacrifice");
    assert.equal(charge.samples, 1); assert.equal(charge.value, 12, "charge clock pauses during immunity; partial cycles excluded");
    const musicPlayer = actor("player", [[0, [condition(999, 0, "SBBV:f:15000;SBBSB:f:3500;SBBDDR:f:0.647737;"),
        condition(680, 0, "MCMBAMAX:f:96.7;"), condition(192, 0, "MFCP:f:89.7;LSMA:f:91.2;"), condition(193, 0, "SPDPC:f:1.84315;")]],
        [12, [condition(999, 12, "SBBSB:f:99999;SBBDDR:f:0.99;")]],
        [22, [condition(680, 22, "MCMBAMAX:f:120;", { AttackerId: "someone-else" })]]]);
    const sonicBoss = actor("boss", [[0, [condition(1176, 0, "SBPD:f:23;")]], [10, [condition(277, 10), condition(1176, 10, "SBPD:f:100;")]], [20, []]]);
    const saint = build({ ...input, player: musicPlayer, boss: sonicBoss, jobName: "圣光颂唱者" });
    assert.equal(metric(saint, "sonic-protection").value, 23);
    assert.equal(saint.rows.length, 6, "protection shares one row; Vivace magic attack and casting speed each have a row");
    assert.equal(metric(saint, "sonic-magic-protection"), undefined);
    assert.ok(Math.abs(metric(saint, "sonic-curtain").value - 64.7737) < 1e-8, "curtain uses the damage-reduction ratio, not either absorption quantity; immunity peaks excluded");
    assert.equal(metric(saint, "sonic-curtain").unit, "%");
    assert.equal(metric(build({ ...input, player: actor("player", [[0, [condition(999, 0, "SBBSB:f:3500;")]]]), jobName: "圣光颂唱者" }), "sonic-curtain").status, "missing-data", "old absorption quantities do not stand in for missing reduction rates");
    const roundedSonic = actor("boss", [[0, [condition(1176, 0, "SBPD:f:17;MCSTCT:2:5;")]]]);
    assert.equal(metric(build({ ...input, boss: roundedSonic, jobName: "圣光颂唱者" }), "sonic-protection").value, 17, "SBPD is already the total; do not multiply by stacks or invent the unrounded value");
    const decimalSonic = actor("boss", [[0, [condition(1176, 0, "SBPD:f:16.75;")]]]);
    assert.equal(metric(build({ ...input, boss: decimalSonic, jobName: "圣光颂唱者" }), "sonic-protection").value, 16.75, "frontend does not truncate precision supplied by a capture");
    assert.equal(metric(saint, "music-war").value, 96.7, "other players' performances are not attributed to this player");
    assert.equal(metric(saint, "music-active").value, 89.7);
    assert.equal(metric(saint, "music-active-magic").value, 91.2);
    assert.equal(metric(saint, "music-active-magic").unit, "%");
    const activeReport = (states, extra = {}) => build({ ...input, player: actor("player", states), jobName: "圣光颂唱者", ...extra });
    const activePeaks = activeReport([[0, [condition(192, 0, "MFCP:f:88.528008;LSMA:f:90.298561;LSFA:f:44.264004;AFCP:f:44.264004;")]],
        [5, [condition(192, 5, "MFCP:f:89;LSMA:f:85;")]],
        [12, [condition(192, 12, "MFCP:f:200;LSMA:f:200;")]],
        [22, [condition(192, 22, "MFCP:f:300;LSMA:f:300;", { AttackerId: "other" })]]]);
    assert.equal(metric(activePeaks, "music-active").value, 89, "casting peak comes from MFCP and excludes immunity and other performers");
    assert.equal(metric(activePeaks, "music-active-magic").value, 90.298561, "magic attack uses its own LSMA percentage peak without multiplying by 100");
    const onlyCast = activeReport([[0, [condition(192, 0, "MFCP:f:88.528008;LSFA:f:44.264004;AFCP:f:44.264004;MCMLSADR:f:0.902986;")]]]);
    assert.equal(metric(onlyCast, "music-active").value, 88.528008);
    assert.equal(metric(onlyCast, "music-active-magic").status, "missing-data", "other Vivace effects do not replace a missing magic attack field");
    const onlyMagic = activeReport([[0, [condition(192, 0, "LSMA:f:90.298561;")]]]);
    assert.equal(metric(onlyMagic, "music-active-magic").value, 90.298561);
    assert.equal(metric(onlyMagic, "music-active").status, "missing-data");
    const lateActive = activeReport([[0, []], [31, [condition(192, 31, "MFCP:f:88.528008;LSMA:f:90.298561;")]]]);
    assert.equal(metric(lateActive, "music-active").status, "no-samples", "a post-fight cast does not become a fight sample");
    assert.equal(metric(lateActive, "music-active-magic").status, "no-samples");
    assert.ok(Math.abs(metric(saint, "music-march").value - 84.315) < 1e-8);
    const aim = build({ ...input, jobName: "流星射手", aimSamples: [
        { entityId: "player", atMs: 2000, rate: .5 }, { entityId: "player", atMs: 12000, rate: 1 },
        { entityId: "other", atMs: 4000, rate: 1 }, { entityId: "player", atMs: 22000, rate: .9 }] });
    assert.equal(metric(aim, "aim").value, 70);
    assert.equal(metric(aim, "aim").label, "穿心平均瞄准率（估算）");
    const teammateAim = build({ ...input, localEntityId: "player", player: actor("teammate"), jobName: "流星射手",
        aimSamples: [{ entityId: "teammate", targetId: "boss", atMs: 2000, rate: 1 }] });
    assert.equal(metric(teammateAim, "aim").status, "missing-data", "local aim parameters cannot establish a teammate's aim rate");
    assert.equal(metric(teammateAim, "aim").value, null);
    assert.ok(metric(teammateAim, "aim").detail.includes("队友"));
    const stingerPairs = build({ ...input, jobName: "流星射手", actions: [action(59060, 1), action(21002, 2), action(21002, 4),
        action(21002, 8), action(59060, 9), action(59060, 22), action(21002, 23), action(21002, 24), action(21002, 24),
        action(21002, 25, { IsFallback: true }), action(59060, 29)] });
    assert.equal(metric(stingerPairs, "magnum").value, 2.5);
    assert.equal(metric(stingerPairs, "magnum").samples, 2, "uses canonical fire and Magnum releases, not multi-hit fallback damage");
    const publicShot = (skill, at, extra = {}) => action(skill, at, { Id: "teammate", SourceId: "teammate", IsLocal: false, ...extra });
    const partyStinger = (actions, signals = []) => build({ ...input, localEntityId: "player", player: actor("teammate"),
        jobName: "流星射手", actions, arcanaSignals: signals });
    const publicStinger = partyStinger([publicShot(59060, 1), publicShot(59060, 1.6, { IsFallback: true }),
        publicShot(59060, 1.8, { IsFallback: true }), publicShot(59060, 2, { IsFallback: true }),
        publicShot(21002, 3, { CombatActionId: 11 }), publicShot(21002, 4, { CombatActionId: 12 }),
        publicShot(21002, 4, { CombatActionId: 12 }), publicShot(21002, 5, { IsFallback: true }), publicShot(59060, 9)]);
    assert.equal(metric(publicStinger, "magnum").value, 2, "confirmed public party shots are usable; three Blazing hits and damage-only Magnum fallbacks are not casts");
    assert.equal(metric(publicStinger, "magnum").samples, 1);
    assert.equal(metric(publicStinger, "aim").value, null, "public releases cannot provide private aim parameters");
    const oldPartyStinger = partyStinger([publicShot(59060, 1, { IsFallback: true }), publicShot(21002, 3, { IsFallback: true }), publicShot(59060, 9, { IsFallback: true })]);
    assert.equal(metric(oldPartyStinger, "magnum").status, "missing-data");
    assert.ok(metric(oldPartyStinger, "magnum").detail.includes("回放原始抓包"), "old damage-only logs explain how to obtain missing public release data");
    const hydroSignal = (cast, end, value, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(end), AtMs: end * 1000,
        CastAtMs: cast * 1000, SkillId: 59061, Signal: "hydro-charge-sample", Kind: 825, Complete: true, Count: 1,
        TargetId: "boss", Value: value, Range: 3530, ...extra });
    const hydroSignals = [hydroSignal(1, 1.6, 7.5), hydroSignal(3, 3.155, 29.47)];
    const partyHydro = partyStinger([], [...hydroSignals.map((signal) => ({ ...signal, Id: "teammate" })), hydroSignals[0]]);
    assert.equal(metric(partyHydro, "hydro-full").value, 50, "public party Hydro geometry retains its owner and does not borrow self samples");
    assert.equal(metric(partyHydro, "hydro-full").samples, 2);
    const hydroReport = (signals) => build({ ...input, jobName: "流星射手", arcanaSignals: signals });
    const hydroFull = metric(hydroReport([...hydroSignals, hydroSignals[0], hydroSignal(9, 21, 7.5),
        hydroSignal(4, 4.6, 7.5, { Id: "other" }), hydroSignal(5, 5.6, 7.5, { TargetId: "other-boss" }),
        hydroSignal(6, 6.6, 7.5, { Complete: false }), hydroSignal(7, 7.6, NaN), hydroSignal(31, 31.6, 7.5)]), "hydro-full");
    assert.equal(hydroFull.samples, 2, "full-charge rate is deduplicated and requires a complete cast, owner, boss and active window");
    assert.equal(hydroFull.value, 50, "one fully narrowed release and one wider release is 50% full-charge rate");
    assert.equal(hydroFull.status, "measured");
    assert.equal(hydroChargeIsFull(7.5), true);
    assert.equal(hydroChargeIsFull(7.50005), true, "floating point tolerance is small");
    assert.equal(hydroChargeIsFull(7.51), false, "near-full but still wider is not full charge");
    assert.equal(hydroChargeIsFull(29.47), false);
    assert.equal(hydroChargeIsFull(6), null, "unsupported narrower ranges are unknown");
    assert.equal(hydroChargeIsFull(NaN), null);
    assert.equal(hydroChargeIsFull(undefined), null);
    assert.equal(metric(hydroReport([]), "hydro-full").status, "missing-data");
    assert.equal(metric(hydroReport([hydroSignals[1]]), "hydro-full").value, 0, "observed partial-only casts give a real zero");
    assert.equal(metric(hydroReport([hydroSignal(1, 1.15, 7.5)]), "hydro-full").value, 100, "geometry, not animation duration, identifies full charge");
    assert.equal(metric(hydroReport([hydroSignal(1, 3, 29.47)]), "hydro-full").value, 0, "a longer interval is not sufficient proof of full charge");
    const hydroUnknown = metric(hydroReport([hydroSignals[0], hydroSignal(1, 1.6, 29.47), hydroSignal(3, 3.6, 6)]), "hydro-full");
    assert.equal(hydroUnknown.value, null);
    assert.equal(hydroUnknown.samples, 0);
    assert.equal(hydroUnknown.status, "missing-data", "conflicting feedback and unsupported ranges are not partial charge");
    const hydroMixed = metric(hydroReport([...hydroSignals, hydroSignal(4, 4.6, 6)]), "hydro-full");
    assert.equal(hydroMixed.value, 50);
    assert.equal(hydroMixed.samples, 2, "unknown feedback does not dilute the rate");
    assert.ok(hydroMixed.detail.includes("1 次反馈无法判定"));
    assert.equal(metric(hydroReport([hydroSignal(12, 12.6, 7.5)]), "hydro-full").status, "no-samples");
    const fighterCombo = (cast, reverse, kick, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(kick), AtMs: kick * 1000,
        SkillId: 24201, Signal: "fighter-combo", CastAtMs: cast * 1000, TargetId: "boss", Count: reverse == null ? 0 : 1,
        ReverseAtMs: reverse == null ? undefined : reverse * 1000, ReverseEndAtMs: reverse == null ? undefined : (reverse + .05) * 1000,
        ReadyAtMs: (kick - .005) * 1000, Complete: true, ...extra });
    const fighterSamples = [fighterCombo(2, 2.5, 3), fighterCombo(5, 5.501, 6), fighterCombo(12, 12.3, 12.6), fighterCombo(7, null, 9)];
    const fighterReport = (signals, extra = {}) => build({ ...input, jobName: "狂怒斗士", arcanaSignals: signals, ...extra });
    const fighter = fighterReport([...fighterSamples, fighterSamples[0], fighterSamples[3],
        fighterCombo(21, 21.2, 21.6, { Id: "other" }), fighterCombo(22, null, 24, { TargetId: "other-boss" }),
        fighterCombo(9.5, 9.7, 21), fighterCombo(25, 25.2, 26, { Complete: false }), fighterCombo(31, null, 33)]);
    assert.equal(metric(fighter, "reverse-saved").label, "逆龙节省后摇平均时间（估算）");
    assert.equal(metric(fighter, "reverse-saved").unit, "秒");
    assert.equal(metric(fighter, "reverse-saved").samples, 2);
    assert.ok(Math.abs(metric(fighter, "reverse-saved").value + .08) < 1e-8, "video baseline minus reverse mean; duplicates, immunity, incomplete casts, other owners/targets and other fights are excluded");
    assert.equal(metric(fighter, "reverse-saved").status, "estimated");
    assert.equal(metric(fighter, "counter"), undefined, "success rate is replaced by saved time without a 500ms cutoff");
    const unequalDurations = fighterReport([fighterCombo(1, null, 3), fighterCombo(5, null, 9),
        fighterCombo(21, 21.2, 21.5), fighterCombo(24, 25, 25.5)]);
    assert.ok(Math.abs(metric(unequalDurations, "reverse-saved").value + .08) < 1e-8, "observed normal samples do not replace the approved fixed video baseline; valid late reversals count");
    assert.equal(metric(fighterReport([fighterCombo(2, null, 3), fighterCombo(5, 5.2, 7)]), "reverse-saved").value, -1.08,
        "retain negative savings if reversed combos are slower");
    assert.equal(metric(fighterReport([fighterSamples[3], fighterCombo(2, 2.3, 3, { ReadyAtMs: undefined })]), "reverse-saved").status,
        "no-samples", "missing kick readiness cannot prove a completed cancel");
    assert.equal(metric(fighterReport([fighterCombo(2, null, 3)]), "reverse-saved").status, "no-samples", "normal controls alone cannot measure savings");
    assert.equal(metric(fighterReport([fighterSamples[0]]), "reverse-saved").status, "estimated", "reverse-only fights use the explicit video reference");
    assert.ok(Math.abs(metric(fighterReport([fighterSamples[3], fighterCombo(21, 21.2, 21.6)],
        { session: { ...session, startAt: 21 } }), "reverse-saved").value - .32) < 1e-8, "only current-fight reverse intervals enter the estimate");
    assert.equal(metric(build({ ...input, jobName: "狂怒斗士", actions: [action(24201, 2), action(24103, 2.3), action(24301, 3)] }), "reverse-saved").status,
        "missing-data", "ordinary attack records cannot reveal the preparation-only animation cancel");
    const fighterEnergy = (at, signal, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(at), AtMs: at * 1000, SkillId: 0,
        Signal: signal, Count: 0, Complete: false, ...extra });
    const energyHistory = [fighterEnergy(0, "fighter-energy-baseline", { Value: 80, Rate: 10, Maximum: 400 }),
        fighterEnergy(5, "fighter-energy-delta", { Value: -100 }), fighterEnergy(24, "fighter-energy-delta", { Value: -100 }),
        fighterEnergy(24, "fighter-spend-start", { SkillId: 59185, CastAtMs: 24000 }),
        fighterEnergy(25, "fighter-spend-end", { SkillId: 59185, CastAtMs: 24000, Complete: true })];
    assert.equal(metric(fighterReport(energyHistory), "energy-idle").value, 12, "integrates natural growth, exact 100-point boundary, consumption, immunity and time actually using a spender");
    assert.equal(metric(fighterReport(energyHistory.slice(1)), "energy-idle").status, "estimated", "unknown initial energy produces bounds under confirmed regeneration and cap");
    assert.ok(metric(fighterReport(energyHistory.slice(1)), "energy-idle").upperValue > metric(fighterReport(energyHistory.slice(1)), "energy-idle").value);
    const { estimateFighterIdle } = await server.ssrLoadModule("/src/fighterEnergyBounds.ts");
    const uncertain = (signals, end = 10) => estimateFighterIdle(signals, [{ start: 0, end }], end);
    const laterSpend = uncertain([fighterEnergy(0, "fighter-energy-delta", { Value: 6 }), fighterEnergy(1, "fighter-energy-delta", { Value: -200 })]);
    assert.equal(laterSpend.lower, 1, "a later 200-point spend proves the preceding second had at least 100 points");
    assert.equal(laterSpend.upper, 10);
    const fullSpend = uncertain([fighterEnergy(0, "fighter-energy-delta", { Value: -400 })]);
    assert.equal(fullSpend.lower, 0); assert.equal(fullSpend.upper, 0, "known cap and a 400-point spend determine the remaining energy without initialization");
    const resetBounds = uncertain([fighterEnergy(0, "fighter-energy-delta", { Value: 6 }),
        fighterEnergy(1, "fighter-energy-reset"), fighterEnergy(2, "fighter-energy-delta", { Value: -200 })]);
    assert.equal(resetBounds.lower, 0, "future spending cannot constrain the previous connection's gauge");
    assert.equal(uncertain([fighterEnergy(0, "fighter-energy-delta", { Value: -500 })]), undefined, "impossible changes must not produce reassuring bounds");
    // Independent simulation of known hidden starting gauges: every true idle
    // duration must remain inside the inferred interval, including capped gains.
    for (let trial = 0; trial < 24; trial++) {
        let value = (trial * 71) % 401, at = 0, trueIdle = 0;
        const signals = [];
        for (let step = 0; step < 18; step++) {
            const next = at + 1 + ((trial + step * 3) % 5) / 2;
            trueIdle += Math.max(0, next - Math.max(at, at + (100 - value) / 3));
            value = Math.min(400, value + (next - at) * 3); at = next;
            const delta = step % 3 === 0 && value >= 100 ? -100 : Math.min(400 - value, 20 + (step % 4) * 20);
            value += delta; signals.push(fighterEnergy(at, "fighter-energy-delta", { Value: delta }));
        }
        const finish = at + 5;
        trueIdle += Math.max(0, finish - Math.max(at, at + (100 - value) / 3));
        const inferred = uncertain(signals, finish);
        assert.ok(inferred && inferred.lower <= trueIdle + 1e-6 && inferred.upper >= trueIdle - 1e-6, `hidden gauge trial ${trial} remains bounded`);
    }
    assert.equal(metric(fighterReport([fighterEnergy(0, "fighter-energy-baseline", { Value: 99, Rate: 0, Maximum: 400 })]), "energy-idle").value, 0);
    assert.equal(metric(fighterReport([fighterEnergy(0, "fighter-energy-baseline", { Value: 100, Rate: 0, Maximum: 400 })]), "energy-idle").value, 20);
    assert.equal(metric(fighterReport([...energyHistory, fighterEnergy(6, "fighter-energy-reset")]), "energy-idle").status, "estimated", "post-reset deltas may constrain a new range without carrying old energy forward");
    const interruptedSpend = [fighterEnergy(1, "fighter-energy-baseline", { Value: 200, Rate: 3, Maximum: 400 }),
        fighterEnergy(2, "fighter-spend-start", { SkillId: 59185, CastAtMs: 2000 }), fighterEnergy(8, "fighter-energy-reset")];
    assert.equal(metric(fighterReport(interruptedSpend, { session: { ...session, startAt: 20 } }), "energy-idle").status,
        "missing-data", "a disconnected spend cannot mask missing energy by excluding the whole later fight");
    assert.equal(metric(fighterReport([...interruptedSpend, fighterEnergy(19, "fighter-energy-baseline", { Value: 200, Rate: 3, Maximum: 400 })],
        { session: { ...session, startAt: 20 } }), "energy-idle").value, 10, "a previous connection's incomplete spend ends at reset");
    assert.equal(metric(fighterReport([fighterEnergy(0, "fighter-energy-baseline", { Value: 400, Rate: 3, Maximum: 400 }),
        fighterEnergy(1, "fighter-energy-delta", { Value: -200 }), fighterEnergy(2, "fighter-energy-delta", { Value: -300 })]), "energy-idle").status,
        "missing-data", "inconsistent consumption invalidates the timeline");
    const sniper = (cast, at, count, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: 59123, Signal: "sniper-counter", CastAtMs: cast * 1000, TargetId: "boss", Count: count,
        Phase: count === 0 ? 2 : 6, Complete: false, ...extra });
    const sixShots = [sniper(.5, .8, 0), sniper(.5, 1, 1), sniper(.5, 2, 3), sniper(.5, 3.1, 6, { Phase: 7, Complete: true })];
    const gunner = (signals, extra = {}) => build({ ...input, jobName: "枪炮师", arcanaSignals: signals, ...extra });
    assert.equal(metric(gunner(sixShots), "sniper").value, 6, "uses cumulative server counter despite missing intermediate shot packets");
    assert.equal(metric(gunner([...sixShots, sixShots.at(-1)]), "sniper").samples, 1, "duplicate completion is not a second cast");
    assert.equal(metric(gunner(sixShots.slice(1)), "sniper").status, "no-samples", "capture beginning mid-cast has no zero baseline");
    assert.equal(metric(gunner(sixShots.slice(0, -1)), "sniper").status, "no-samples", "unfinished sniper excluded");
    assert.equal(metric(gunner([]), "sniper").status, "missing-data", "old damage-only logs cannot recover shot count");
    const regressingShots = [...sixShots.slice(0, 3), sniper(.5, 2.5, 2), sixShots.at(-1)];
    assert.equal(metric(gunner(regressingShots), "sniper").status, "no-samples");
    const splitShots = [sniper(8, 8.5, 0), sniper(8, 9, 1), sniper(8, 21, 6, { Phase: 7, Complete: true })];
    assert.equal(metric(gunner(splitShots), "sniper").status, "no-samples", "cast crossing immunity excluded even if its shots border immunity");
    assert.equal(metric(gunner(sixShots, { session: { ...session, startAt: 1, endAt: 3, totalDuration: 2 } }), "sniper").value, 6,
        "pre-fight zero baseline and final-hit subsecond completion remain usable");
    const zoneSample = (cast, at, count, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(at), AtMs: at * 1000,
        SkillId: 59121, Signal: "domain-sample", CastAtMs: cast * 1000, TargetId: "boss", Count: count, Complete: true,
        ObjectIds: Array.from({ length: count }, (_, i) => `4546800126427136${i + 1}`), ...extra });
    const zones = [zoneSample(1, 2, 1), zoneSample(3, 4, 2), zoneSample(21, 22, 3)];
    const zoneReport = gunner([...zones, zones[0], zoneSample(11, 12, 3), zoneSample(5, 6, 3, { Id: "other" }), zoneSample(7, 8, 3, { TargetId: "other-boss" })]);
    assert.equal(metric(zoneReport, "domain-count").value, 2);
    assert.equal(metric(zoneReport, "domain-count").samples, 3, "complete heavy samples averaged once each; player, target and immunity respected");
    assert.equal(metric(gunner([zoneSample(1, 2, 0)]), "domain-count").value, 0, "explicit empty list is a measured zero");
    assert.equal(metric(gunner([{ ...zones[0], Signal: "domain-linked", TargetId: undefined }]), "domain-count").status, "missing-data", "unbound list is not target coverage");
    assert.equal(metric(gunner([zoneSample(1, 2, 2, { ObjectIds: ["same", "same"] })]), "domain-count").status, "no-samples");
    assert.equal(metric(zoneReport, "domain-time"), undefined, "Gunner KPI no longer includes domain coverage time");
    const chemicalCounter = { EventId: 22, Id: "player", At: 2, AtMs: 2200, SkillId: 59144,
        Signal: "chemical-hit-count", TargetId: "boss", Count: 5, Complete: false };
    const chemical = build({ ...input, jobName: "禁术炼金师", arcanaSignals: [chemicalCounter] });
    assert.equal(metric(chemical, "chemical").status, "missing-data", "raw count alone cannot stand in for a complete cast");
    assert.equal(metric(chemical, "chemical").value, null);
    assert.equal(metric(build({ ...input, jobName: "禁术炼金师", arcanaSignals: [{ ...chemicalCounter, Id: "other" }] }), "chemical").status, "missing-data");
    const completed = (cast, first, last, count, skill, signal, extra = {}) => ({ EventId: 22, Id: "player", At: Math.floor(last),
        AtMs: last * 1000, CastAtMs: cast * 1000, FirstHitAtMs: first * 1000, TargetId: "boss", Count: count,
        SkillId: skill, Signal: signal, Complete: true, ...extra });
    const chemicalSample = (cast, first, last, count, extra) => completed(cast, first, last, count, 59144, "chemical-sample", extra);
    const chemicalSamples = [chemicalSample(1, 2, 3, 5), chemicalSample(21, 22, 23, 3)];
    const chemicalReport = (signals, extra = {}) => build({ ...input, jobName: "禁术炼金师", arcanaSignals: signals, ...extra });
    assert.equal(metric(chemicalReport([...chemicalSamples, chemicalSamples[0], chemicalSample(8, 9, 21, 5),
        chemicalSample(4, 5, 6, 5, { TargetId: "other-boss" }), chemicalSample(24, 25, 26, 5, { Id: "other" })]), "chemical").value, 3,
        "subtracts one base attack per complete cast, deduplicates, and excludes casts crossing immunity and other entities");
    assert.equal(metric(chemicalReport([chemicalSample(1, 2, 3, 1)]), "chemical").value, 0, "explicit base-only cast is zero extras");
    assert.equal(metric(chemicalReport([chemicalSample(1, 2, 3, 5, { Complete: false })]), "chemical").status, "no-samples");
    assert.equal(metric(chemicalReport([chemicalSamples[0]], { session: { ...session, startAt: 2, endAt: 3 } }), "chemical").value, 4,
        "cast preparation may precede the first boss damage; final subsecond hit remains within its damage second");
    const act7Sample = (cast, first, last, empty, extra) => completed(cast, first, last, empty, 54105, "act7-sample",
        { Phase: 3, ObjectIds: [`puppet-${cast}`], ...extra });
    const puppeteerReport = (signals) => build({ ...input, jobName: "旋律操纵师", arcanaSignals: signals });
    const act7Samples = [act7Sample(1, 1.1, 2.1, 0), act7Sample(3, 3.1, 4.1, 1), act7Sample(21, 21.1, 22.1, 0)];
    assert.ok(Math.abs(metric(puppeteerReport([...act7Samples, act7Samples[1], act7Sample(8, 9, 21, 1),
        act7Sample(5, 5.1, 6.1, 1, { Phase: 2 }), act7Sample(24, 24.1, 25.1, 1, { Id: "other" })]), "act7").value - 100 / 3) < 1e-8,
        "counts completed empty charges once, rejects incomplete attack phases, immunity and other owners");
    assert.equal(metric(puppeteerReport([act7Sample(1, 1.1, 2.1, 1, { Complete: false })]), "act7").status, "no-samples");
    const interludeSample = (cast, first, last, count, extra) => completed(cast, first, last, count, 59165, "interlude-sample", extra);
    const interludeSamples = [interludeSample(4, 4, 4.2, 1), interludeSample(22, 22, 22.2, 3)];
    assert.equal(metric(puppeteerReport([...interludeSamples, interludeSamples[0], interludeSample(12, 12, 12.2, 3),
        interludeSample(24, 24, 24.2, 3, { TargetId: "another-boss" })]), "puppets").value, 2);
    assert.equal(metric(puppeteerReport([interludeSample(4, 4, 4.2, 3, { Complete: false })]), "puppets").status, "no-samples");
    assert.equal(metric(puppeteerReport(interludeSamples), "interlude-wait").status, "missing-data", "missing canonical release history is not a zero wait");
    const interludeWait = (actions, extra = {}) => metric(build({ ...input, jobName: "旋律操纵师", actions, ...extra }), "interlude-wait");
    const interludeReleases = [action(59165, 1.2), action(59165, 4.7), action(59165, 4.7), action(59165, 22.4), action(59165, 28.9),
        action(59165, 6, { IsFallback: true }), action(59165, 7, { Id: "other" }),
        action(59165, 8, { SourceId: "puppet" }), action(59165, 9, { MechanicSignal: "effect" }), action(59165, 31)];
    const interludeInterval = interludeWait(interludeReleases);
    assert.ok(Math.abs(interludeInterval.value - 17.7 / 3) < 1e-8, "release-to-release gaps retain fractional seconds and subtract only actual boss immunity");
    assert.equal(interludeInterval.samples, 3, "duplicate, fallback, foreign and out-of-session actions cannot add intervals");
    assert.ok(Math.abs(interludeWait([action(59165, 1.2), action(59165, 4.7)], { boss: actor("boss", [[0, []]]) }).value - 3.5) < 1e-8,
        "includes the full interval without subtracting a skill cooldown");
    assert.equal(interludeWait([action(59165, 1), action(59165, 12), action(59165, 22)]).value, null, "does not bridge over a release made during immunity");
    assert.equal(interludeWait([action(59165, 2)]).status, "no-samples", "one release has no completed interval");
    assert.equal(interludeWait([action(59165, 2, { IsFallback: true })]).status, "missing-data");
    assert.equal(interludeWait([action(59165, 28.8), action(59165, 30.8), action(59165, 31)]).value, 2,
        "the inclusive final battle second retains real fractional release times, while later seconds remain excluded");
    assert.equal(interludeWait([action(59165, 28.8), action(59165, 30.8)], { boss: actor("boss", [[0, []], [30, [condition(277, 30)]]]) }).value, null,
        "immunity beginning in the final second still invalidates that release");
    for (const jobName of ["元素骑士", "圣光颂唱者", "黑魔导士", "流星射手", "圣盾骑士", "爆裂骑士枪", "枪炮师", "禁术炼金师", "旋律操纵师", "狂怒斗士"]) {
        assert.ok(build({ ...input, jobName }).rows.length > 0, jobName);
    }
    assert.equal(build({ ...input, jobName: null }).rows.length, 0);
    const { buildEventSnapshot } = await server.ssrLoadModule("/src/worker/buildEventSnapshot.ts");
    const snapshot = buildEventSnapshot([stat(0, 2), stat(1, .25)].map(JSON.stringify).join("\n"));
    assert.equal(snapshot.statUpdates.length, 2, "resource histories survive the log worker even before entity appearance");
    const { ActorManager } = await server.ssrLoadModule("/src/eventActor.ts");
    const { DamageCollectorManager } = await server.ssrLoadModule("/src/actionCollector.ts");
    const { hydrateFromSnapshot } = await server.ssrLoadModule("/src/worker/hydrateActorManager.ts");
    const { createBattleRecord, parseBattleRecord } = await server.ssrLoadModule("/src/battleRecord.ts");
    const appearance = (Id, RaceId) => ({ EventId: 1, At: -2, Id, RaceId, Name: Id, GuildName: "", OwnerId: "",
        Height: 1, Weight: 1, Upper: 1, Lower: 1 });
    const earlyConditions = [
        { EventId: 4, ...condition(999, 1, "SBBDDR:f:0.8;", { DisableAtMs: 9000 }) },
        { EventId: 4, ...condition(999, 1, "SBBDDR:f:0.6;", { DisableAtMs: 9000 }) },
        { EventId: 5, At: 3, Id: "player", CCId: 999 },
    ];
    const earlyReplay = buildEventSnapshot([...earlyConditions, { ...appearance("player", 8001), At: 5 }].map(JSON.stringify).join("\n"));
    const earlyActor = earlyReplay.entities.player;
    assert.equal(earlyActor.conditionHistory[0].At, 1, "early Buffs keep their original observation time");
    assert.equal(earlyActor.conditionHistory.length, 3, "same-second metadata changes and pre-appearance disables survive log import");
    assert.equal(earlyActor.conditionMap[999], undefined, "a completed early Buff is not resurrected as live");
    assert.equal(metric(build({ ...input, player: earlyActor, jobName: "圣光颂唱者" }), "sonic-curtain").value, 80);
    const liveEarly = new ActorManager(new DamageCollectorManager());
    for (const event of [...earlyConditions, { ...appearance("player", 8001), At: 5 }]) liveEarly.onEvent(event);
    assert.equal(liveEarly.entityMap.player.conditionHistory.length, 3, "live and imported histories preserve the same early peak");
    assert.equal(liveEarly.entityMap.player.conditionMap[999], undefined);
    const resetEarly = buildEventSnapshot([earlyConditions[0], { EventId: 11, At: 2, Id: "0", Reliable: false, Reset: true },
        { ...appearance("player", 8001), At: 5 }].map(JSON.stringify).join("\n"));
    assert.equal(resetEarly.entities.player.conditionHistory.length, 0, "channel reset discards unfinished identity buffers");
    const pendingReplay = buildEventSnapshot(earlyConditions.map(JSON.stringify).join("\n"));
    const resumed = new ActorManager(new DamageCollectorManager()), resumedCollector = new DamageCollectorManager();
    hydrateFromSnapshot(pendingReplay, resumed, resumedCollector);
    resumed.onEvent({ ...appearance("player", 8001), At: 5 });
    assert.equal(resumed.entityMap.player.conditionHistory.length, 3, "a partial log retains unknown-actor conditions across hydration");
    const damage = (At) => ({ EventId: 3, At, Id: "player", TargetId: "boss", SkillId: 59040, Damage: 100, IsCritical: false, IsDelayed: false });
    const replay = buildEventSnapshot([appearance("player", 8001), appearance("boss", 7601),
        { EventId: 4, ...condition(192, 0, "MFCP:f:160;LSMA:f:150;", { DisableAtMs: 90000 }) },
        { EventId: 4, ...condition(192, 1, "MFCP:f:88.528008;LSMA:f:90.298561;", { DisableAtMs: 90000 }) },
        ...sixShots, ...zones, ...chemicalSamples, ...act7Samples, ...interludeSamples, ...fighterSamples, ...energyHistory, zoneSample(31, 32, 3),
        stat(-1, .5), stat(0, .25), damage(1), stat(6, 1), damage(30), stat(31, 10)].map(JSON.stringify).join("\n"));
    const collector = new DamageCollectorManager(), manager = new ActorManager(collector);
    hydrateFromSnapshot(replay, manager, collector);
    const originalSaint = build({ ...input, session: { ...session, startAt: 1 }, player: manager.entityMap.player, musicPerformances: manager.musicPerformances, jobName: "圣光颂唱者" });
    assert.equal(metric(originalSaint, "music-active-magic").value, 150, "an opening performance survives replacement before the first hit");
    manager.kpiAimSamples.push({ entityId: "player", targetId: "boss", atMs: 2500, rate: .8 });
    const record = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    assert.deepEqual(record.snapshot.statUpdates.map((event) => event.At), [0, 6], "export preserves the latest pre-fight quantity and excludes later observations");
    assert.equal(record.snapshot.kpiAimSamples[0].rate, .8);
    assert.equal(record.snapshot.arcanaSignals.length, sixShots.length + zones.length + chemicalSamples.length + act7Samples.length + interludeSamples.length + fighterSamples.length + energyHistory.length, "complete samples and pre-fight energy baseline survive export; later heavy excluded");
    assert.equal(record.snapshot.arcanaSignals[0].Count, 0);
    assert.equal(record.snapshot.arcanaSignals.find((signal) => signal.Signal === "domain-sample").ObjectIds[0], "45468001264271361", "zone IDs survive JSON as exact strings");
    hydrateFromSnapshot(record.snapshot, manager, collector);
    assert.equal(manager.statUpdates.length, 2); assert.equal(manager.kpiAimSamples.length, 1);
    assert.equal(manager.arcanaSignals.length, 23);
    const replayGunner = gunner(manager.arcanaSignals);
    assert.equal(metric(replayGunner, "sniper").value, 6); assert.equal(metric(replayGunner, "domain-count").value, 2);
    assert.equal(metric(chemicalReport(manager.arcanaSignals), "chemical").value, 3);
    assert.ok(Math.abs(metric(puppeteerReport(manager.arcanaSignals), "act7").value - 100 / 3) < 1e-8);
    assert.equal(metric(puppeteerReport(manager.arcanaSignals), "puppets").value, 2);
    assert.ok(Math.abs(metric(fighterReport(manager.arcanaSignals), "reverse-saved").value + .08) < 1e-8);
    assert.equal(metric(fighterReport(manager.arcanaSignals), "energy-idle").value, 12);
    const replaySaint = build({ ...input, session: { ...session, startAt: 1 }, player: manager.entityMap.player, musicPerformances: manager.musicPerformances, jobName: "圣光颂唱者" });
    assert.equal(metric(replaySaint, "music-active-magic").value, 150, "magic attack survives worker import and battle-record export/reimport");
    assert.equal(metric(replaySaint, "music-active").value, 160, "casting speed remains independent after replay");
    assert.deepEqual(replaySaint.rows, originalSaint.rows, "performance peaks survive export without restoring replaced active Buffs");
    manager.clear();
    assert.equal(manager.statUpdates.length, 0); assert.equal(manager.kpiAimSamples.length, 0);
    assert.equal(manager.arcanaSignals.length, 0);
    const hydroSnapshot = buildEventSnapshot([appearance("player", 8001), appearance("boss", 7601),
        ...hydroSignals, damage(1), damage(30)].map(JSON.stringify).join("\n"));
    hydrateFromSnapshot(hydroSnapshot, manager, collector);
    const hydroRecord = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    assert.deepEqual(hydroRecord.snapshot.arcanaSignals, hydroSignals, "raw charge geometry survives worker import and battle export exactly");
    hydrateFromSnapshot(hydroRecord.snapshot, manager, collector);
    assert.equal(metric(hydroReport(manager.arcanaSignals), "hydro-full").samples, 2);
    assert.equal(metric(hydroReport(manager.arcanaSignals), "hydro-full").value, 50);
    const darkActions = [action(59045, 1, { Sequence: 10 }), action(59045, 20, { Sequence: 30 }), action(59045, 30, { Sequence: 50 }),
        action(59040, 2), action(59040, 4), action(59040, 7), action(59040, 22), action(59040, 29),
        action(30102, 2.5), action(30102, 5), action(30102, 23)];
    const earlyCooldown = cooldown(10, { Sequence: 20 });
    const darkEvents = [earlyCooldown, appearance("player", 8001), appearance("boss", 7601),
        chainState(-10, true), chainState(-1, false), chainState(3, true, { TargetId: "other-target" }), chainState(6, false),
        ...darkActions, cooldown(25, { Sequence: 40 }), cooldown(-1), cooldown(31), cooldown(18, { SkillId: 30102 }),
        cooldown(15, { Id: "other" }),
        { EventId: 4, ...condition(948, 3, "", { Id: "boss", DisableAtMs: 6000, AttackerId: "other" }) },
        { EventId: 5, At: 6, Id: "boss", CCId: 948 },
        { EventId: 4, ...condition(277, 10, "", { Id: "boss" }) },
        { EventId: 5, At: 20, Id: "boss", CCId: 277 }, damage(1), damage(30)];
    const darkSnapshot = buildEventSnapshot(darkEvents.map(JSON.stringify).join("\n"));
    const liveDark = new ActorManager(new DamageCollectorManager());
    darkEvents.forEach((event) => liveDark.onEvent(event));
    assert.deepEqual(darkSnapshot.skillCooldowns, liveDark.skillCooldowns, "live and worker paths retain even pre-appearance cooldown events");
    manager.clear();
    hydrateFromSnapshot(darkSnapshot, manager, collector);
    const managedDark = () => darkReport({ player: manager.entityMap.player, boss: manager.entityMap.boss,
        session: { ...session, startAt: 1 }, actions: manager.skillActions, skillCooldowns: manager.skillCooldowns, arcanaSignals: manager.arcanaSignals });
    const originalDark = managedDark();
    assert.equal(metric(originalDark, "seal-wait").value, 2.5);
    assert.equal(metric(originalDark, "dragon-chain").value, 20);
    assert.equal(metric(originalDark, "thunder").value, 1);
    const darkRecord = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    assert.deepEqual(darkRecord.snapshot.skillCooldowns.map((event) => event.At), [10, 25, 18], "only participating players' in-fight cooldown updates are exported");
    assert.deepEqual(darkRecord.snapshot.arcanaSignals.map((event) => event.At), [-1, 3, 6], "latest pre-fight Chain state is retained, including an active connection to a different target");
    hydrateFromSnapshot(darkRecord.snapshot, manager, collector);
    assert.deepEqual(managedDark().rows, originalDark.rows, "all black-mage metrics survive battle export/reimport, including reset-adjusted waiting");
    manager.clear(); assert.equal(manager.skillCooldowns.length, 0);
    delete darkRecord.snapshot.skillCooldowns;
    hydrateFromSnapshot(darkRecord.snapshot, manager, collector);
    assert.equal(manager.skillCooldowns.length, 0, "legacy records without cooldown adjustments still load");
    const chainResetEvents = [{ EventId: 11, Id: "player", At: 0, Reliable: true, Reset: false }, chainState(1, true),
        { EventId: 11, Id: "0", At: 8, Reliable: false, Reset: true }];
    const chainResetSnapshot = buildEventSnapshot(chainResetEvents.map(JSON.stringify).join("\n"));
    const liveChainReset = new ActorManager(new DamageCollectorManager());
    chainResetEvents.forEach((event) => liveChainReset.onEvent(event));
    assert.deepEqual(chainResetSnapshot.arcanaSignals, liveChainReset.arcanaSignals, "live and worker paths close previous Chain state at connection reset");
    assert.equal(chainRate(chainResetSnapshot.arcanaSignals, { actions: [action(59040, 4), action(59040, 22)] }).value, 50, "old Chain cannot carry across a connection reset");
    for (const reset of [{ Id: "player", Reset: true }, { Id: "other", Reset: false }]) {
        const events = [{ EventId: 11, Id: "player", At: 0, Reliable: true, Reset: false },
            fighterEnergy(1, "fighter-energy-baseline", { Value: 200, Rate: 3, Maximum: 400 }),
            { EventId: 11, At: 8, Reliable: true, ...reset }];
        const snapshot = buildEventSnapshot(events.map(JSON.stringify).join("\n"));
        const live = new ActorManager(new DamageCollectorManager());
        events.forEach((event) => live.onEvent(event));
        assert.deepEqual(live.arcanaSignals, snapshot.arcanaSignals, "live/import energy invalidation agrees on connection and identity changes");
        assert.equal(metric(fighterReport(snapshot.arcanaSignals, { session: { ...session, startAt: 20 } }), "energy-idle").status,
            "missing-data", "pre-reset energy must not become a known baseline in a later fight");
    }
    for (const reset of [{ EventId: 11, Id: "observer", At: 8, Reset: true }, { EventId: 2, Id: "other", At: 8 }]) {
        const events = [{ EventId: 11, Id: "observer", At: 0, Reliable: true }, appearance("other", 9001),
            chainState(1, true, { Id: "other" }), fighterEnergy(1, "fighter-energy-baseline", { Id: "other", Value: 200, Rate: 3, Maximum: 400 }), reset];
        const snapshot = buildEventSnapshot(events.map(JSON.stringify).join("\n"));
        const live = new ActorManager(new DamageCollectorManager()); events.forEach((event) => live.onEvent(event));
        assert.deepEqual(live.arcanaSignals, snapshot.arcanaSignals);
        assert.ok(snapshot.arcanaSignals.some((e) => e.Id === "other" && e.Signal === "lightning-chain-reset" && e.At === 8), "teammate chain stops on disappearance and reconnect");
        assert.ok(snapshot.arcanaSignals.some((e) => e.Id === "other" && e.Signal === "fighter-energy-reset" && e.At === 8), "teammate resources do not carry into a later presence");
    }
    assert.equal(metric(build({ ...input, actions: [action(59026, 1), action(59028, 2, { IsFallback: true }), action(59026, 8)] }), "rush").status,
        "missing-data", "unconfirmed teammate phase must not masquerade as zero casts");
    assert.equal(metric(build({ ...input, localEntityId: "observer", jobName: "狂怒斗士", arcanaSignals: [] }), "energy-idle").status, "missing-data");
    const interludeReplay = buildEventSnapshot([appearance("player", 8001), appearance("boss", 7601),
        ...interludeReleases, damage(1), damage(30),
        { EventId: 4, ...condition(277, 10, "", { Id: "boss" }) },
        { EventId: 5, At: 20, Id: "boss", CCId: 277 }].map(JSON.stringify).join("\n"));
    hydrateFromSnapshot(interludeReplay, manager, collector);
    const managedInterlude = () => interludeWait(manager.skillActions, { session: { ...session, startAt: 1 }, boss: manager.entityMap.boss });
    const originalInterludeWait = managedInterlude();
    const interludeRecord = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    hydrateFromSnapshot(interludeRecord.snapshot, manager, collector);
    assert.deepEqual(managedInterlude(), originalInterludeWait, "release interval, millisecond precision and immunity survive record export/reimport");
    const boundsEvents = [appearance("player", 8001), appearance("boss", 7601),
        fighterEnergy(-10, "fighter-energy-bounds", { Value: 0, UpperValue: 200, Rate: 3, Maximum: 400 }),
        fighterEnergy(-2, "fighter-energy-delta", { Value: 20 }), damage(1), damage(30)];
    hydrateFromSnapshot(buildEventSnapshot(boundsEvents.map(JSON.stringify).join("\n")), manager, collector);
    const originalBounds = metric(fighterReport(manager.arcanaSignals, { session: { ...session, startAt: 1 } }), "energy-idle");
    const boundsRecord = parseBattleRecord(JSON.stringify(createBattleRecord(manager, collector, "boss", "player", { bossName: "test" })));
    assert.equal(boundsRecord.snapshot.arcanaSignals[0].UpperValue, 200, "pre-fight checkpoint range survives export");
    hydrateFromSnapshot(boundsRecord.snapshot, manager, collector);
    assert.deepEqual(metric(fighterReport(manager.arcanaSignals, { session: { ...session, startAt: 1 } }), "energy-idle"), originalBounds);
    const openingMusic = [
        appearance("player", 8001), appearance("boss", 7601),
        { EventId: 4, ...condition(680, -90, "MCMBAMAX:f:999;", { DisableAt: -1 }) },
        { EventId: 4, ...condition(680, -70, "MCMBAMAX:f:998;") }, // unknown duration
        { EventId: 4, ...condition(680, -5, "MCMBAMAX:f:96.7;", { Id: "non-attacker", DisableAt: 50 }) },
        { EventId: 4, ...condition(680, -5, "MCMBAMAX:f:96.7;", { DisableAt: 50 }) },
        { EventId: 5, Id: "player", At: -4, CCId: 680 },
        { EventId: 5, Id: "non-attacker", At: -4, CCId: 680 },
        { EventId: 4, ...condition(192, -3, "MFCP:f:80;LSMA:f:90;", { DisableAt: 50 }) },
        { EventId: 4, ...condition(193, -2, "SPDPC:f:1.6;", { DisableAt: 50 }) },
        { EventId: 4, ...condition(680, 2, "MCMBAMAX:f:997;", { AttackerId: "other-performer", DisableAt: 50 }) },
        { EventId: 4, ...condition(277, 10, "", { Id: "boss" }) },
        { EventId: 4, ...condition(680, 15, "MCMBAMAX:f:996;", { DisableAt: 50 }) },
        { EventId: 5, Id: "boss", At: 20, CCId: 277 },
        damage(1), damage(30),
    ];
    const musicReport = (m) => build({ ...input, session: { ...session, startAt: 1 },
        player: m.entityMap.player, boss: m.entityMap.boss, jobName: "圣光颂唱者", musicPerformances: m.musicPerformances });
    hydrateFromSnapshot(buildEventSnapshot(openingMusic.map(JSON.stringify).join("\n")), manager, collector);
    const openingReport = musicReport(manager);
    assert.equal(metric(openingReport, "music-war").value, 96.7, "opening War cast on others remains after both recipients switch songs");
    assert.equal(metric(openingReport, "music-war").samples, 1, "group recipients are one performance; expired, unknown-duration, other casters and immune casts are excluded");
    assert.equal(metric(openingReport, "music-active-magic").value, 90);
    assert.equal(metric(openingReport, "music-active").value, 80);
    assert.ok(Math.abs(metric(openingReport, "music-march").value - 60) < 1e-8);
    const liveMusic = new ActorManager(new DamageCollectorManager());
    for (const event of openingMusic) liveMusic.onEvent(event);
    assert.deepEqual(musicReport(liveMusic).rows, openingReport.rows, "live and worker performance evidence agree");
    const openingRecord = createBattleRecord(manager, collector, "boss", "player", { bossName: "music" });
    assert.equal(openingRecord.snapshot.entities["non-attacker"], undefined);
    hydrateFromSnapshot(parseBattleRecord(JSON.stringify(openingRecord)).snapshot, manager, collector);
    assert.deepEqual(musicReport(manager).rows, openingReport.rows, "recipient need not attack or remain in the saved actor roster");
    const checkpointMusic = new ActorManager(new DamageCollectorManager());
    checkpointMusic.onEvent(appearance("player", 8001));
    checkpointMusic.onEvent({ EventId: 23, ...condition(680, -5, "MCMBAMAX:f:96.7;", { DisableAt: 50 }) });
    assert.equal(checkpointMusic.entityMap.player.conditionMap[680], undefined, "checkpoint evidence never restores an active song");
    assert.equal(metric(build({ ...input, player: checkpointMusic.entityMap.player, jobName: "圣光颂唱者", musicPerformances: checkpointMusic.musicPerformances }), "music-war").value, 96.7);
    manager.onEvent({ EventId: 11, At: 31, Id: "player", Reliable: true, Reset: true });
    assert.equal(metric(build({ ...input, session: { ...session, startAt: 32, endAt: 40 }, player: manager.entityMap.player,
        jobName: "圣光颂唱者", musicPerformances: manager.musicPerformances }), "music-war").value, null, "opening songs do not cross a connection reset");
    console.log("Arcana KPI verified: ten professions, immunity/expiry, ownership, resources, opening/team Saint performances, Gunner counters, Chemical extras, puppet/interlude counts, Fighter natural energy/animation cancels and battle-record replay.");
} finally { await server.close(); }
