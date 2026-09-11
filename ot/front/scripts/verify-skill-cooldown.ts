import { strict as assert } from "node:assert";
import { readFile } from "node:fs/promises";

const values = new Map<string, string>();
Object.defineProperty(globalThis, "localStorage", {
    value: {
        getItem: (key: string) => values.get(key) ?? null,
        setItem: (key: string, value: string) => values.set(key, String(value)),
        removeItem: (key: string) => values.delete(key),
        clear: () => values.clear(),
    },
});

const {
    SKILL_COOLDOWN_STORAGE_KEY,
    calculateMagnumAimDisplayProgress,
    calculateMagnumAimTiming,
    summarizeMagnumAimSamples,
    applyToahSpiritProgressObservation,
    applySkillCooldownAdjustment,
    applySkillCooldownObservation,
    builtinSkillCooldownRuleDescription,
    isCumulativeCooldownSkill,
    isLocalCooldownDamage,
    loadSkillCooldownSettings,
    settleSkillCooldownRuntime,
    shouldShowToahSpiritProgressOverlay,
    shouldRefreshSkillCooldownFromToah,
    techniqueCooldownActionFromCondition,
    toahSpiritDisplayPercent,
} = await import("../src/skillCooldown.ts");

localStorage.setItem(SKILL_COOLDOWN_STORAGE_KEY, JSON.stringify({
    iconSize: 40,
    coordinateVersion: 2,
    timeline: {
        enabled: true,
        x: 720,
        y: 440,
        width: 780,
        skillIds: [59145, 59144, 59145, 99999],
    },
    rules: {
        21002: { skillId: 21002, enabled: true, cooldownSeconds: 1, x: 700, y: 200 },
        59144: { skillId: 59144, enabled: true, cooldownSeconds: 7, x: 1240, y: 880 },
        59145: {
            skillId: 59145,
            enabled: true,
            cooldownSeconds: 12,
            x: 900,
            y: 300,
            soundMode: "custom",
            customSoundId: "a".repeat(64),
            customSoundName: "ready.wav",
        },
    },
}));

const loaded = loadSkillCooldownSettings();
assert.equal(loaded.rules[59144].soundMode, "default", "old settings migrate to the bundled CD sound");
assert.deepEqual(
    [loaded.rules[59144].x, loaded.rules[59144].y, loaded.rules[59145].x, loaded.rules[59145].y],
    [1240, 880, 900, 300],
    "each skill keeps its own coordinates",
);
assert.equal(loaded.rules[59145].soundMode, "custom");
assert.equal(loaded.rules[59145].customSoundName, "ready.wav");
assert.equal(loaded.rules[59145].shortCooldownSeconds, 1, "old Spiral Burst settings get the short-CD default");
assert.equal(loaded.rules[59145].cumulativeCooldownSeconds, 10, "old Spiral Burst settings get the cumulative-CD default");
assert.equal(loaded.rules[59144].progressThresholdPercent, 95, "old settings migrate to the Toah default threshold");
assert.equal(loaded.rules[59144].ownerMode, "auto", "old settings migrate to automatic player/pet ownership");
assert.equal(loaded.aimReminder.enabled, true, "old Magnum Shot rules migrate into the independent aim reminder");
assert.equal(loaded.aimReminder.alwaysVisible, false, "old aim settings keep the idle prompt hidden by default");
assert.equal(loaded.aimReminder.weaponRange, 2200, "old aim settings migrate to the Destruction Bow range");
assert.equal(loaded.aimReminder.rangeIdentificationLevel, 0, "old aim settings migrate without a range identification bonus");
assert.equal(loaded.aimReminder.calibrationPercent, 40, "the independent aim reminder defaults to 40% calibration");
assert.equal(loaded.aimReminder.ergSpeedPercent, 200, "the independent aim reminder defaults to Erg 200%");
assert.equal(loaded.aimReminder.fineTuneSeconds, 0, "the calculated best time starts without a user adjustment");
assert.equal(loaded.aimReminder.scalePercent, 100, "legacy settings get the default whole-reminder scale");
assert.deepEqual([loaded.aimReminder.x, loaded.aimReminder.y], [700, 200], "legacy Magnum coordinates migrate to the independent bar");
assert.equal("aimAssistEnabled" in loaded.rules[21002], false, "ordinary skill-CD rules no longer own aim settings");
assert.equal("timeline" in loaded, false, "legacy next-skill timeline settings are discarded");
assert.equal(isCumulativeCooldownSkill(59104), true, "Charging Strike uses cumulative cooldown");
assert.equal(isCumulativeCooldownSkill(59145), true, "Spiral Burst uses cumulative cooldown");
assert.equal(isCumulativeCooldownSkill(59144), false, "ordinary skills keep a fixed cooldown");
assert.match(builtinSkillCooldownRuleDescription(26002), /30%.*27049/, "Shuriken Storm shows its built-in spirit reset rule");
assert.match(builtinSkillCooldownRuleDescription(27203), /SLST27203.*2 秒/, "astrology shows its per-skill fixed reduction");
assert.match(builtinSkillCooldownRuleDescription(59180), /27069.*20%/, "Burning Soul shows its packet-driven 20% reduction");

const aimSettings = {
    weaponRange: 2200,
    rangeIdentificationLevel: 0,
    calibrationPercent: 40,
    ergSpeedPercent: 200,
    fineTuneSeconds: 0,
};
const aimCases = [
    [{}, 1],
    [{ rapid: true }, 2],
    [{ finalShot: true }, 2.4],
    [{ latikaSecret: true }, 4],
    [{ finalShot: true, rapid: true }, 2.4],
    [{ latikaSecret: true, rapid: true }, 4],
    [{ finalShot: true, latikaSecret: true }, 9.6],
    [{ finalShot: true, latikaSecret: true, rapid: true }, 9.6],
] as const;
for (const [buffs, multiplier] of aimCases) {
    const timing = calculateMagnumAimTiming(aimSettings, buffs);
    assert.equal(timing.temporarySpeedMultiplier, multiplier);
    assert.equal(timing.speedMultiplier, multiplier * 2, "Erg 200% multiplies the effective temporary aim speed");
}
assert.equal(
    calculateMagnumAimTiming(aimSettings, {}).durationMs,
    188,
    "2200 range, 40% calibration and Erg 200% reach displayed 85% in about 0.188s",
);
assert.equal(calculateMagnumAimTiming(aimSettings, {}).fullDurationMs, 752, "the same setup reaches displayed 100% in about 0.752s");
assert.equal(
    calculateMagnumAimTiming({ ...aimSettings, weaponRange: 1600, calibrationPercent: 20, ergSpeedPercent: 100 }, {}).durationMs,
    1344,
    "1600 range and 20% correction follow the fixed-distance standard formula",
);
assert.equal(
    calculateMagnumAimTiming({ ...aimSettings, weaponRange: 2300, calibrationPercent: 20, ergSpeedPercent: 100 }, {}).durationMs,
    1011,
    "2300 range and 20% correction follow the fixed-distance standard formula",
);
assert.equal(
    calculateMagnumAimTiming({ ...aimSettings, rangeIdentificationLevel: 20 }, {}).effectiveRange,
    3600,
    "range identification adds 70 range per level to the selected weapon",
);
assert.equal(
    calculateMagnumAimTiming({ ...aimSettings, fineTuneSeconds: -0.12 }, { finalShot: true, latikaSecret: true }).durationMs,
    7,
    "the user adjustment is applied before temporary Final Shot × Latika speed",
);
assert.equal(calculateMagnumAimDisplayProgress(1000, 1360, 1000, 40), 0.4, "40% calibration starts the active bar at 40%");
assert.equal(calculateMagnumAimDisplayProgress(1000, 1360, 1040, 40), 0.5, "elapsed time squares the rate remaining after calibration");
assert.equal(calculateMagnumAimDisplayProgress(1000, 1360, 1360, 40), 0.85, "system 70% is displayed and marked at 85%");
assert.ok(
    Math.abs(calculateMagnumAimDisplayProgress(1000, 1360, 1640, 40) - 0.9) < 1e-9,
    "system 80% receives the post-70% display bonus and shows 90%",
);
assert.equal(calculateMagnumAimDisplayProgress(1000, 1360, 2440, 40), 1, "the nonlinear bar continues from best shot to 100%");
assert.deepEqual(
    summarizeMagnumAimSamples([
        { atMs: 100_200, rate: 0.8 },
        { atMs: 100_800, rate: 0.9 },
        { atMs: 102_000, rate: 0.5 },
        { atMs: 100_400, rate: 2 },
    ], 100, 100),
    { count: 3, averageRate: 0.9 },
    "the combat summary averages only current-session samples and clamps rates",
);
assert.equal(summarizeMagnumAimSamples([], 100, 100), undefined, "an empty session has no Magnum aim aggregate");

const cumulativeRule = {
    skillId: 59145,
    cooldownSeconds: 15,
    shortCooldownSeconds: 3,
    cumulativeCooldownSeconds: 15,
};
const ordinaryCooldown = applySkillCooldownObservation(
    { skillId: 27203, cooldownSeconds: 10 },
    undefined,
    { At: 100, AtMs: 100_000, IsFallback: false },
);
const reducedOrdinaryCooldown = applySkillCooldownAdjustment(ordinaryCooldown, {
    At: 103,
    AtMs: 103_000,
    ReduceMs: 2000,
});
assert.equal(reducedOrdinaryCooldown?.readyAtMs, 108_000, "SLST reduction subtracts from the existing remaining cooldown");
const resetOrdinaryCooldown = applySkillCooldownAdjustment(reducedOrdinaryCooldown, {
    At: 105,
    AtMs: 105_000,
    Reset: true,
});
assert.equal(resetOrdinaryCooldown?.readyAtMs, 105_000, "27049 resets the exact packet-named skill immediately");
assert.equal(resetOrdinaryCooldown?.cooldownPhase, "idle", "a reset marks the skill as ready");
assert.equal(applySkillCooldownAdjustment(undefined, {
    At: 105,
    AtMs: 105_000,
    Reset: true,
}), undefined, "a reset never creates state for an idle or unselected skill");
const cumulativeUse1 = applySkillCooldownObservation(
    cumulativeRule,
    undefined,
    { At: 100, AtMs: 100_000, IsFallback: false },
);
assert.equal(cumulativeUse1.cooldownPhase, "accumulating");
assert.equal(cumulativeUse1.accumulatedCooldownSeconds, 3, "the first use immediately adds one full short CD");
assert.equal(cumulativeUse1.accumulatedReadyAtMs, 103_000);
assert.equal(cumulativeUse1.shortReadyAtMs, 0, "cumulative cooldown has no separately refreshed short window");
assert.equal(cumulativeUse1.readyAtMs, 100_000, "the skill stays usable below the cumulative limit");
const cumulativeUse2 = applySkillCooldownObservation(
    cumulativeRule,
    cumulativeUse1,
    { At: 101, AtMs: 101_000, IsFallback: true },
);
assert.equal(cumulativeUse2.accumulatedCooldownSeconds, 5, "after 1s, the remaining 2s plus a new 3s becomes 5s");
assert.equal(cumulativeUse2.accumulatedReadyAtMs, 106_000, "the 5s pool now returns to zero 5s after the second use");
assert.equal(cumulativeUse2.shortReadyAtMs, 0, "a reuse extends only the accumulated pool");
const cumulativeAfterOneSecond = settleSkillCooldownRuntime(cumulativeRule, cumulativeUse2, 102_000);
assert.equal(cumulativeAfterOneSecond?.accumulatedCooldownSeconds, 4, "accumulated CD counts down while the skill is not used");
const cumulativeUse3 = applySkillCooldownObservation(
    cumulativeRule,
    cumulativeUse2,
    { At: 102, AtMs: 102_000, IsFallback: true },
);
assert.equal(cumulativeUse3.accumulatedCooldownSeconds, 7, "after another 1s, the remaining 4s plus a new 3s becomes 7s");
assert.equal(cumulativeUse3.accumulatedReadyAtMs, 109_000);
const almostClearedCumulative = settleSkillCooldownRuntime(cumulativeRule, cumulativeUse2, 105_999);
assert.equal(almostClearedCumulative?.accumulatedCooldownSeconds, 0.1, "the time pool keeps its final fractional tick");
const clearedCumulative = settleSkillCooldownRuntime(cumulativeRule, cumulativeUse2, 106_000);
assert.equal(clearedCumulative?.cooldownPhase, "idle", "5s of accumulated CD clears exactly 5s after the second use");
assert.equal(clearedCumulative?.accumulatedCooldownSeconds, 0);

let cumulativeFull = cumulativeUse1;
for (let atMs = 101_000; atMs <= 106_000; atMs += 1_000) {
    cumulativeFull = applySkillCooldownObservation(
        cumulativeRule,
        cumulativeFull,
        { At: Math.floor(atMs / 1000), AtMs: atMs, IsFallback: true },
    );
}
assert.equal(cumulativeFull.cooldownPhase, "full");
assert.equal(cumulativeFull.accumulatedCooldownSeconds, 15, "continued use builds decaying debt to the configured limit");
assert.equal(cumulativeFull.readyAtMs, 121_000, "reaching the limit starts the full skill cooldown");
assert.equal(cumulativeFull.shortReadyAtMs, 0, "no short window remains during full cooldown");
assert.equal(cumulativeFull.accumulatedReadyAtMs, 0, "full cooldown no longer has a decaying debt deadline");
const cumulativeEarlyUse = applySkillCooldownObservation(
    cumulativeRule,
    cumulativeFull,
    { At: 107, AtMs: 107_000, IsFallback: true },
);
assert.equal(cumulativeEarlyUse, cumulativeFull, "full cooldown rejects impossible early re-use packets");
const cumulativeAfterReady = applySkillCooldownObservation(
    cumulativeRule,
    cumulativeFull,
    { At: 122, AtMs: 122_000, IsFallback: false },
);
assert.equal(cumulativeAfterReady.accumulatedCooldownSeconds, 3, "a new cycle starts by adding the first full short CD");
assert.equal(cumulativeAfterReady.cooldownPhase, "accumulating");
const expiredCumulative = settleSkillCooldownRuntime(cumulativeRule, cumulativeUse1, 103_000);
assert.equal(expiredCumulative?.cooldownPhase, "idle", "an unused first 3s pool returns to the initial state");
assert.equal(expiredCumulative?.accumulatedCooldownSeconds, 0);
assert.equal(expiredCumulative?.readyAtMs, 0);

let capturedSpiral = applySkillCooldownObservation(cumulativeRule, undefined, {
    At: 1_787_936_641,
    AtMs: 1_787_936_641_822,
    IsFallback: false,
});
capturedSpiral = applySkillCooldownObservation(cumulativeRule, capturedSpiral, {
    At: 1_787_936_642,
    AtMs: 1_787_936_642_635,
    IsFallback: true,
});
assert.equal(capturedSpiral.generation, 2, "the first damage-derived Spiral Burst repeat from the captured packet log is counted");
assert.equal(capturedSpiral.accumulatedCooldownSeconds, 5.2, "the captured 0.813s repeat adds 3s to the 2.187s remaining pool");
for (const atMs of [1_787_936_644_173, 1_787_936_645_554, 1_787_936_646_881, 1_787_936_648_598]) {
    capturedSpiral = applySkillCooldownObservation(cumulativeRule, capturedSpiral, {
        At: Math.floor(atMs / 1000),
        AtMs: atMs,
        IsFallback: true,
    });
}
assert.equal(capturedSpiral.generation, 6, "all captured Spiral Burst repeat actions are accepted");
assert.equal(capturedSpiral.cooldownPhase, "accumulating");
assert.equal(capturedSpiral.accumulatedCooldownSeconds, 11.3);

const toahAt94 = applyToahSpiritProgressObservation(undefined, 94, 95, 1_000);
assert.equal(toahAt94.generation, 0, "Toah progress below the threshold stays idle");
const toahAt95 = applyToahSpiritProgressObservation(toahAt94, 95, 95, 2_000);
assert.equal(toahAt95.generation, 1, "Toah progress crossing the threshold triggers once");
const toahAt100 = applyToahSpiritProgressObservation(toahAt95, 100, 95, 3_000);
assert.equal(toahAt100.generation, 1, "continuing to full charge does not repeat the threshold alert");
assert.equal(toahAt100.fullChargeGeneration, 1, "the first 100 percent observation refreshes tracked cooldowns once");
const toahCorrection = applyToahSpiritProgressObservation(toahAt100, 99.8, 95, 4_000);
assert.equal(toahCorrection.generation, 1, "small server corrections do not re-arm progress alerts");
const toahFullAgain = applyToahSpiritProgressObservation(toahCorrection, 100, 95, 4_500);
assert.equal(toahFullAgain.fullChargeGeneration, 2, "returning to 100 after any below-full reading refreshes cooldowns again");
const toahSpent = applyToahSpiritProgressObservation(toahFullAgain, 90, 95, 5_000);
const toahSecondCharge = applyToahSpiritProgressObservation(toahSpent, 96, 95, 6_000);
assert.equal(toahSecondCharge.generation, 2, "spending more than five points re-arms the progress threshold");
const toahReset = applyToahSpiritProgressObservation(toahSecondCharge, undefined, 95, 7_000);
assert.equal(toahReset.observed, false, "a channel reset clears stale Toah progress without losing history");
for (let techniqueSkillId = 58000; techniqueSkillId <= 58018; techniqueSkillId += 1) {
    assert.equal(
        shouldRefreshSkillCooldownFromToah(techniqueSkillId),
        false,
        `technique ${techniqueSkillId} must keep its cooldown at full Toah charge`,
    );
}
assert.equal(shouldRefreshSkillCooldownFromToah(27012), false, "Toah Spirit never refreshes its own reminder");
assert.equal(shouldRefreshSkillCooldownFromToah(58100), true, "Stardust Barrage still refreshes at full Toah charge");
assert.equal(shouldRefreshSkillCooldownFromToah(58101), true, "Stardust Blast still refreshes at full Toah charge");
assert.equal(shouldRefreshSkillCooldownFromToah(35024), true, "ordinary skills still refresh at full Toah charge");
assert.equal(shouldRefreshSkillCooldownFromToah(35024, true), false, "pet skills never refresh at full Toah charge");
assert.equal(toahSpiritDisplayPercent(65.9), 65, "Toah overlay percentage truncates instead of rounding");
assert.equal(shouldShowToahSpiritProgressOverlay({
    alwaysVisible: false,
    progressObserved: true,
    progressPercent: 94.9,
    progressThresholdPercent: 95,
}), false, "non-persistent Toah overlay stays hidden below its configured threshold");
assert.equal(shouldShowToahSpiritProgressOverlay({
    alwaysVisible: false,
    progressObserved: true,
    progressPercent: 95,
    progressThresholdPercent: 95,
}), true, "non-persistent Toah overlay remains visible after reaching its configured threshold");
assert.equal(shouldShowToahSpiritProgressOverlay({
    alwaysVisible: false,
    progressObserved: true,
    progressPercent: 99.9,
    progressThresholdPercent: 95,
}), true, "non-persistent Toah overlay stays visible while charging toward full");
assert.equal(shouldShowToahSpiritProgressOverlay({
    alwaysVisible: false,
    progressObserved: true,
    progressPercent: 100,
    progressThresholdPercent: 95,
}), false, "non-persistent Toah overlay disappears at full charge");
assert.equal(shouldShowToahSpiritProgressOverlay({
    alwaysVisible: true,
    progressObserved: true,
    progressPercent: 100,
    progressThresholdPercent: 95,
}), true, "persistent Toah overlay stays visible at full charge");
const skillOverlaySource = await readFile(new URL("../src/components/SkillCooldownOverlay.vue", import.meta.url), "utf8");
const reportSource = await readFile(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
const teamTimelineSource = await readFile(new URL("../src/components/TeamSkillTimeline.vue", import.meta.url), "utf8");
assert.match(
    skillOverlaySource,
    /:not\(\.ready-burst\):not\(\.progress-tracked\)/,
    "non-persistent Toah progress must not inherit the ordinary ready-animation hide rule",
);
assert.match(skillOverlaySource, /aimReminder\.value\?\.alwaysVisible/, "the overlay accepts an idle always-visible aim reminder");
assert.match(reportSource, /未瞄准时一直显示/, "the aim settings expose the always-visible option");
assert.match(reportSource, /class="aim-reminder-save skill-cooldown-save"/, "the aim save control shares the standard themed save-button treatment");
assert.match(reportSource, /穿心平均瞄准率/, "configured aim reminder users receive a Magnum average aim row");
assert.match(reportSource, /SKILL_STATE_EVENT[\s\S]*?handleSkillState/, "Magnum aim start and end events feed the battle aggregate");
assert.match(reportSource, /nativeReminderSettingsSyncQueue/, "desktop settings writes are serialized to prevent stale saves");
assert.match(reportSource, /if \(response\.ok\) return/, "save success waits for a successful desktop response");
assert.match(
    reportSource,
    /短时 CD[\s\S]*?累计 CD[\s\S]*?技能 CD/,
    "Spiral Burst and Charging Strike expose all three cooldown inputs",
);
assert.match(reportSource, /content: "累计冷却"/, "the special rule is named cumulative cooldown");
assert.match(reportSource, /武器基础射程[\s\S]*?鉴定射程[\s\S]*?合计射程[\s\S]*?瞄准校准[\s\S]*?尔格瞄准加成[\s\S]*?延迟微调[\s\S]*?85%最佳时间/, "Magnum Shot collects range, correction and Erg inputs before displaying the best time");
assert.match(reportSource, /毁灭弓 2200[\s\S]*?释魂弓 2000[\s\S]*?释魂弩 2100/, "the range input offers the requested weapon presets");
assert.match(reportSource, /合计射程＝武器基础射程＋鉴定等级×70[\s\S]*?系统 70%[^。]*85%[\s\S]*?延迟微调/, "the aim settings explain range identification, the 85% marker and latency correction");
assert.match(reportSource, /class="aim-reminder-settings"[\s\S]*?无需在技能 CD 中添加穿心箭/, "aim reminder is a separate settings section");
assert.match(skillOverlaySource, /<span>85%<\/span>[\s\S]*?aim-reminder-best-marker[\s\S]*?aim-reminder-footer[\s\S]*?aim-reminder-percent/, "the compact aim bar marks 85% above and keeps the smaller live number below");
assert.doesNotMatch(skillOverlaySource, /aim-reminder-card::before/, "the aim reminder has no full-card translucent background");
assert.match(skillOverlaySource, /\.aim-ready \.aim-reminder-icon[\s\S]*?aim-reminder-best-icon-glow[\s\S]*?aim-reminder-best-track-glow/, "reaching 85% gives the icon and bar a strong ready-style light pulse");
assert.match(skillOverlaySource, /aim-reminder-icon-charge-burst[\s\S]*?aim-reminder-icon-charge-ring[\s\S]*?aim-reminder-icon-burst-rays/, "the 85% skill icon contracts, charges and releases an animated radial burst");
assert.match(skillOverlaySource, /\.aim-ready \.aim-reminder-icon img[\s\S]*?1 both/, "the 85% charge burst plays once when the best-shot state begins");
assert.match(skillOverlaySource, /\.aim-full \.aim-reminder-icon[\s\S]*?aim-reminder-final-release[\s\S]*?1 both/, "100% starts one distinct final release effect");
assert.doesNotMatch(skillOverlaySource, /aim-reminder-(?:best-icon-glow|icon-charge-burst|icon-charge-ring|icon-burst-rays|final-release)[^;\n]*infinite/, "aim completion effects never loop while the high-frequency skill stays prepared");
assert.match(skillOverlaySource, /scheduleAimBestPoint[\s\S]*?nowMs\.value = bestAtMs[\s\S]*?prevents the charge burst from visually starting at 100%/, "fast aim buffs pin an exact 85% render before the ordinary clock can jump to 100%");
assert.match(reportSource, /整体大小[\s\S]*?aimReminder\.scalePercent/, "the whole aim reminder exposes a saved percentage scale");
assert.match(skillOverlaySource, /uiColorThemeStyle[\s\S]*?var\(--ui-color-accent\)/, "aim reminder follows the selected software colour palette");
assert.doesNotMatch(reportSource, /下一技能提示（循环）|skill-timeline-settings/, "the next-skill settings feature is removed");
assert.doesNotMatch(skillOverlaySource, /下一技能|skill-cycle-queue|orderSkillCooldownQueue/, "the desktop next-skill queue is removed");
assert.doesNotMatch(
    teamTimelineSource,
    /全队成员|mode === ["']team["']|teamRows/,
    "the crowded all-team skill timeline is removed",
);
assert.match(
    teamTimelineSource,
    /allowPlayerSelection[\s\S]*?查看队员[\s\S]*?update:personal-player-id/,
    "showing team identities enables switching between each player's personal timeline",
);
assert.match(
    teamTimelineSource,
    /background: var\(--ui-theme-control[\s\S]*?border[^;]*var\(--ui-theme-border/,
    "the team skill timeline follows the selected UI palette",
);
assert.match(
    reportSource,
    /function previewSkillCooldown\([\s\S]*?setTimeout\(\(\) => void previewSkillCooldownSound\(rule, false\), 900\)[\s\S]*?setTimeout\(\(\) => void previewSkillCooldownSound\(rule, false\), 900\)/,
    "both Toah and ordinary position previews play the configured sound at animation completion",
);
assert.match(
    reportSource,
    /function previewBuffStackAlert\([\s\S]*?publishSkillCooldownOverlayState\(true\)[\s\S]*?previewBuffAlertSound\(rule\.stackSoundMode/,
    "the stack prompt preview publishes its position and plays the configured sound",
);

const firstFallback = applySkillCooldownObservation(
    { cooldownSeconds: 30 },
    undefined,
    { At: 100, AtMs: 100_000, IsFallback: true },
);
const repeatedFallback = applySkillCooldownObservation(
    { cooldownSeconds: 30 },
    firstFallback,
    { At: 105, AtMs: 105_000, IsFallback: true },
);
assert.equal(repeatedFallback, firstFallback, "continuous damage must not restart a running cooldown");

const confirmedRefresh = applySkillCooldownObservation(
    { cooldownSeconds: 30 },
    firstFallback,
    { At: 105, AtMs: 105_000, IsFallback: false },
);
assert.equal(confirmedRefresh.usedAtMs, 105_000, "a confirmed recast may refresh cooldown early");
assert.equal(confirmedRefresh.generation, 2);

const duplicateConfirmed = applySkillCooldownObservation(
    { cooldownSeconds: 30 },
    confirmedRefresh,
    { At: 105, AtMs: 105_700, IsFallback: false },
);
assert.equal(duplicateConfirmed, confirmedRefresh, "native and browser technique signals must be deduplicated");

const fallbackAfterReady = applySkillCooldownObservation(
    { cooldownSeconds: 30 },
    firstFallback,
    { At: 131, AtMs: 131_000, IsFallback: true },
);
assert.equal(fallbackAfterReady.usedAtMs, 131_000, "fallback may start the next cooldown after ready");

assert.equal(isLocalCooldownDamage(35024, "local", "local"), true, "local damaging deployables can start their cooldown");
assert.equal(isLocalCooldownDamage(54151, "puppet", "local", "local"), true, "owned puppet damage can start its cooldown");
assert.equal(isLocalCooldownDamage(59169, "puppet", "local", "local"), true, "all configured puppet ranges are supported");
assert.equal(isLocalCooldownDamage(50001, "pet", "local", "local"), false, "normal pet damage must stay excluded");
assert.equal(isLocalCooldownDamage(54151, "other-puppet", "local", "other-player"), false, "another player's puppet must stay excluded");

const techniqueAction = techniqueCooldownActionFromCondition({
    EventId: 4,
    At: 200,
    Id: "local",
    CCId: 479,
    DisableAtMs: 380_250,
    DurationMs: 180_000,
}, "local");
assert.equal(techniqueAction?.SkillId, 58000, "active-technique CC maps to its skill ID");
assert.equal(techniqueAction?.AtMs, 200_250, "exact technique use time is derived from expiry and duration");
assert.equal(techniqueAction?.IsFallback, false, "active-technique CC is an authoritative use signal");
assert.equal(techniqueCooldownActionFromCondition({
    EventId: 4,
    At: 400,
    Id: "local",
    CCId: 479,
    DisableAtMs: 399_000,
    DurationMs: 180_000,
}, "local"), null, "expired channel snapshots must not start a technique cooldown");

console.log("skill cooldown sound, decaying cumulative cooldown, captured Spiral Burst fallback actions, removed next-skill queue, themed team timeline, coordinates, and puppet damage fallback verified");
