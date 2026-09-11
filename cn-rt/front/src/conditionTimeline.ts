export type ConditionTimelineTick = {
    seconds: number;
    pct: number;
    kind: "major" | "minor";
};

export type ConditionTimelineScale = {
    gridTicks: ConditionTimelineTick[];
    labelTicks: ConditionTimelineTick[];
    majorStep: number;
    minorStep: number;
};

type TimelineStep = {
    major: number;
    minor: number;
};

const TIMELINE_STEPS: TimelineStep[] = [
    { major: 1, minor: 0.25 },
    { major: 2, minor: 0.5 },
    { major: 5, minor: 1 },
    { major: 10, minor: 2 },
    { major: 15, minor: 5 },
    { major: 20, minor: 5 },
    { major: 30, minor: 10 },
    { major: 45, minor: 15 },
    { major: 60, minor: 15 },
    { major: 90, minor: 30 },
    { major: 120, minor: 30 },
    { major: 180, minor: 60 },
    { major: 240, minor: 60 },
    { major: 300, minor: 60 },
    { major: 450, minor: 150 },
    { major: 600, minor: 120 },
    { major: 900, minor: 300 },
    { major: 1_200, minor: 300 },
    { major: 1_800, minor: 600 },
    { major: 2_700, minor: 900 },
    { major: 3_600, minor: 900 },
    { major: 5_400, minor: 1_800 },
    { major: 7_200, minor: 1_800 },
    { major: 10_800, minor: 3_600 },
    { major: 14_400, minor: 3_600 },
    { major: 21_600, minor: 7_200 },
];

const DEFAULT_TIMELINE_WIDTH = 900;
const MIN_LABEL_GAP_PX = 54;
const MAX_LABEL_COUNT = 14;
const MAX_GRID_TICKS = 240;

function selectTimelineStep(duration: number, width: number): TimelineStep {
    const maxLabels = Math.max(2, Math.min(MAX_LABEL_COUNT, Math.floor(width / MIN_LABEL_GAP_PX)));
    const wantedStep = duration / Math.max(1, maxLabels - 1);
    const predefined = TIMELINE_STEPS.find((step) => step.major >= wantedStep);
    if (predefined) return predefined;

    const hours = Math.max(1, Math.ceil(wantedStep / 3_600));
    return {
        major: hours * 3_600,
        minor: Math.max(900, hours * 900),
    };
}

function isMajorTick(seconds: number, majorStep: number) {
    const ratio = seconds / majorStep;
    return Math.abs(ratio - Math.round(ratio)) < 0.0001;
}

/**
 * Builds a time scale whose labels stay readable at the measured track width.
 * The first and last labels are always retained; nearby interior labels are
 * omitted while their grid lines remain available for precise visual reading.
 */
export function buildConditionTimelineScale(
    durationSeconds: number,
    trackWidthPx: number,
): ConditionTimelineScale {
    const duration = Math.max(0, Number.isFinite(durationSeconds) ? durationSeconds : 0);
    const width = Number.isFinite(trackWidthPx) && trackWidthPx > 0
        ? trackWidthPx
        : DEFAULT_TIMELINE_WIDTH;

    if (duration === 0) {
        const origin = { seconds: 0, pct: 0, kind: "major" as const };
        return { gridTicks: [], labelTicks: [origin], majorStep: 1, minorStep: 1 };
    }

    const { major, minor } = selectTimelineStep(duration, width);
    const minLabelGapSeconds = duration * MIN_LABEL_GAP_PX / width;
    const labelTicks: ConditionTimelineTick[] = [
        { seconds: 0, pct: 0, kind: "major" },
    ];

    for (let seconds = major; seconds < duration; seconds += major) {
        const previous = labelTicks[labelTicks.length - 1].seconds;
        if (seconds - previous < minLabelGapSeconds || duration - seconds < minLabelGapSeconds) continue;
        labelTicks.push({ seconds, pct: seconds / duration * 100, kind: "major" });
    }
    labelTicks.push({ seconds: duration, pct: 100, kind: "major" });

    const gridTicks: ConditionTimelineTick[] = [];
    for (let seconds = minor; seconds < duration && gridTicks.length < MAX_GRID_TICKS; seconds += minor) {
        gridTicks.push({
            seconds,
            pct: seconds / duration * 100,
            kind: isMajorTick(seconds, major) ? "major" : "minor",
        });
    }

    return {
        gridTicks,
        labelTicks,
        majorStep: major,
        minorStep: minor,
    };
}
