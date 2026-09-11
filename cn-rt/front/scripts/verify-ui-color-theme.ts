import { strict as assert } from "node:assert";
import { readFileSync } from "node:fs";
import {
    DEFAULT_UI_COLOR_THEME,
    UI_COLOR_THEMES,
    normalizeUiColorThemeId,
    uiColorThemeStyle,
} from "../src/uiColorTheme.ts";

const REQUIRED_THEME_KEYS = [
    "accent",
    "canvas",
    "surface",
    "raised",
    "control",
    "border",
    "text",
    "muted",
    "onAccent",
] as const;

function relativeLuminance(hex: string): number {
    const channels = hex.slice(1).match(/.{2}/g)?.map((value) => Number.parseInt(value, 16) / 255) ?? [];
    const linear = channels.map((value) => value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4);
    return 0.2126 * linear[0] + 0.7152 * linear[1] + 0.0722 * linear[2];
}

function contrastRatio(foreground: string, background: string): number {
    const [bright, dark] = [relativeLuminance(foreground), relativeLuminance(background)]
        .sort((left, right) => right - left);
    return (bright + 0.05) / (dark + 0.05);
}

assert.equal(UI_COLOR_THEMES.length, 14, "all referenced Mabinogi interface colours are available");
assert.equal(new Set(UI_COLOR_THEMES.map((theme) => theme.id)).size, UI_COLOR_THEMES.length);
assert.equal(normalizeUiColorThemeId("sea-blue"), "sea-blue");
assert.equal(normalizeUiColorThemeId("unknown"), DEFAULT_UI_COLOR_THEME);

const contrastFailures: string[] = [];
for (const theme of UI_COLOR_THEMES) {
    for (const key of REQUIRED_THEME_KEYS) {
        assert.match(theme[key], /^#[0-9a-f]{6}$/i, `${theme.name} is missing a valid ${key} colour`);
    }
    for (const surface of [theme.canvas, theme.surface, theme.raised]) {
        const ratio = contrastRatio(theme.text, surface);
        if (ratio < 4.5) contrastFailures.push(`${theme.name} text ${ratio.toFixed(2)} on ${surface}`);
    }
    const accentRatio = contrastRatio(theme.onAccent, theme.accent);
    if (accentRatio < 4.5) contrastFailures.push(`${theme.name} onAccent ${accentRatio.toFixed(2)}`);
    const style = uiColorThemeStyle(theme.id);
    assert.equal(style["--ui-color-accent"], theme.accent, `${theme.name} exposes its accent token`);
    assert.equal(style["--ui-theme-text"], theme.text, `${theme.name} exposes its text token`);
    assert.equal(style["--ui-theme-on-accent"], theme.onAccent, `${theme.name} exposes its button-label token`);
    console.log(`UI colour theme verified: ${theme.name}`);
}
assert.deepEqual(contrastFailures, [], "every semantic palette must meet WCAG AA contrast");

const seaBlueStyle = uiColorThemeStyle("sea-blue");
assert.equal(seaBlueStyle["--ui-color-accent"], "#4fdbdd");
assert.equal(seaBlueStyle["--ui-theme-canvas"], "#91cdd7");
assert.equal(seaBlueStyle["--ui-color-scheme"], "light");
assert.match(uiColorThemeStyle("doll-pink")["--v-theme-primary"], /^\d+, \d+, \d+$/);

const appSource = readFileSync(new URL("../src/App.vue", import.meta.url), "utf8");
const mainSource = readFileSync(new URL("../src/main.ts", import.meta.url), "utf8");
const themeCss = readFileSync(new URL("../src/uiColorTheme.css", import.meta.url), "utf8");
const reportSource = readFileSync(new URL("../src/components/GameDpsReport.vue", import.meta.url), "utf8");
const skillTimelineSource = readFileSync(new URL("../src/components/TeamSkillTimeline.vue", import.meta.url), "utf8");
const recordBrowserSource = readFileSync(new URL("../src/components/BattleRecordBrowser.vue", import.meta.url), "utf8");
assert.doesNotMatch(appSource, /mix-blend-mode|main-dilmeter-app::(?:before|after)/);
assert.match(mainSource, /uiColorTheme\.css/);
assert.match(themeCss, /background:\s*var\(--ui-theme-canvas\)\s*!important/);
assert.match(themeCss, /\.v-overlay-container/);
assert.doesNotMatch(themeCss, /\.condition-card,\s*\.skill-row/);
assert.doesNotMatch(themeCss, /\.condition-timeline-segment,\s*\.skill-progress-track span/);
assert.match(themeCss, /DPS skill ranking keeps Mabinogi's light track and green damage fill/);
assert.match(themeCss, /\.skill-row[\s\S]{0,250}linear-gradient\(#f5f8f9, #e2e8eb 55%, #cbd4d9\)/);
assert.match(themeCss, /\.skill-progress-track span[\s\S]{0,180}linear-gradient\(#a8f328, #82dc04 55%, #60b900\)/);
assert.match(themeCss, /\.skill-name-text[\s\S]{0,300}text-shadow:/);
assert.match(reportSource, /\.skill-row[\s\S]{0,500}-webkit-text-stroke:\s*0\.35px #0b0b0b/);
assert.match(themeCss, /\.skill-timeline-component[\s\S]*?\.skill-timeline-zoom button\.active[\s\S]*?var\(--ui-theme-on-accent\)/);
assert.match(themeCss, /\.skill-timeline-summary[\s\S]*?var\(--ui-theme-muted\)/);
assert.match(themeCss, /\.team-chart-view-switch button[\s\S]*?var\(--ui-theme-control\)/);
assert.match(themeCss, /\.team-chart-view-switch button\.active[\s\S]*?var\(--ui-theme-selected\)/);
assert.match(themeCss, /\.team-chart-view-switch button\.active[\s\S]*?var\(--ui-theme-on-accent\)/);
assert.match(themeCss, /\.skill-bar-settings-button[\s\S]*?var\(--ui-theme-control\)/);
assert.match(themeCss, /\.skill-bar-settings-button \.v-icon[\s\S]*?var\(--ui-color-accent\)/);
assert.match(themeCss, /:where\([\s\S]*?\.aim-reminder-save[\s\S]*?\.skill-cooldown-save[\s\S]*?var\(--ui-theme-text\)/);
assert.match(themeCss, /:where\([\s\S]*?\.aim-reminder-save[\s\S]*?\.skill-cooldown-save[\s\S]*?var\(--ui-theme-raised\)/);
assert.match(skillTimelineSource, /\.skill-timeline-scroll[\s\S]*?var\(--ui-theme-inset\)/);
assert.match(skillTimelineSource, /\.skill-timeline-axis[\s\S]*?var\(--ui-theme-surface\)/);
assert.match(recordBrowserSource, /\.record-browser-dialog[\s\S]*?var\(--ui-theme-canvas\)/);
assert.match(recordBrowserSource, /\.record-browser-dialog[\s\S]*?var\(--ui-theme-border\)/);
assert.match(recordBrowserSource, /\.record-view-button[\s\S]*?var\(--ui-theme-selected\)/);
assert.match(recordBrowserSource, /\.record-view-button[\s\S]*?var\(--ui-theme-on-accent\)/);
assert.match(recordBrowserSource, /\.record-primary-metrics strong[\s\S]*?var\(--ui-color-accent\)/);
assert.match(recordBrowserSource, /class="record-bulk-delete-button"[\s\S]*?requestBatchDelete/);
assert.match(recordBrowserSource, /deletableLoadedRecords = computed\(\(\) => records\.value\.filter\(\(record\) => !record\.active\)\)/);
assert.match(recordBrowserSource, /:disabled="record\.active \|\| Boolean\(operationName\)"/);
assert.match(recordBrowserSource, /for \(const record of candidates\)[\s\S]*?method: "DELETE"/);
assert.match(recordBrowserSource, /已批量删除 \$\{deletedNames\.size\} 条记录/);
assert.match(recordBrowserSource, /\.record-bulk-delete-button[\s\S]*?var\(--record-danger\)/);

console.log("Mabinogi semantic UI colour palettes, Team Chart controls, skill timelines, and history record browser verified");
