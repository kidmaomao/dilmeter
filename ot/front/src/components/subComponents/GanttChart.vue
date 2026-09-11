<template>
    <div class="gantt-root">
        <div class="gantt-scroll-wrap">
            <div class="gantt-inner">

                <!-- ── 資料列 ─────────────────────────────── -->
                <div
                    v-for="(row, ri) in rows"
                    :key="ri"
                    class="gantt-row"
                    :class="{ 'gantt-row-header': row.isHeader }"
                >
                    <div class="gantt-label">
                        <img
                            v-if="row.iconUrl"
                            :src="row.iconUrl"
                            width="16" height="16"
                            class="gantt-label-icon"
                        />
                        <div class="gantt-label-text">
                            <div class="gantt-label-name" :title="row.label">{{ row.label }}</div>
                            <div v-if="row.sublabel" class="gantt-label-sub">{{ row.sublabel }}</div>
                        </div>
                    </div>
                    <div class="gantt-bars">
                        <!-- 格線 -->
                        <div
                            v-for="tick in ticks"
                            :key="tick.value"
                            class="gantt-grid"
                            :style="{ left: tick.pct + '%' }"
                        />
                        <!-- 時間段 -->
                        <div
                            v-for="(seg, si) in row.segments"
                            :key="si"
                            class="gantt-bar"
                            :style="{
                                left: segLeft(seg.start) + '%',
                                width: Math.max(segWidth(seg.start, seg.end), 0.15) + '%',
                                background: row.color,
                            }"
                        />
                        <!-- Boss 無敵期間遮罩 -->
                        <div
                            v-for="(iv, ii) in clippedInvincible"
                            :key="`iv-${ii}`"
                            class="gantt-invinc"
                            :style="{
                                left: segLeft(iv.start) + '%',
                                width: Math.max(segWidth(iv.start, iv.end), 0.15) + '%',
                            }"
                        />
                    </div>
                </div>

                <!-- ── X 軸 ───────────────────────────────── -->
                <div class="gantt-axis-row">
                    <div class="gantt-label" />
                    <div class="gantt-axis">
                        <div
                            v-for="tick in ticks"
                            :key="tick.value"
                            class="gantt-tick"
                            :style="{ left: tick.pct + '%' }"
                        >{{ tick.label }}</div>
                    </div>
                </div>

            </div>
        </div>
    </div>
</template>

<script lang="ts">
import { defineComponent, computed, type PropType } from "vue";

export interface GanttSegment {
    start: number;
    end: number;
}

export interface GanttRow {
    label: string;
    sublabel?: string;
    segments: GanttSegment[];
    color: string;
    isHeader?: boolean;
    iconUrl?: string;
}

export default defineComponent({
    name: "GanttChart",
    props: {
        rows:               { type: Array as PropType<GanttRow[]>,     required: true },
        timeStart:          { type: Number,                            required: true },
        timeEnd:            { type: Number,                            required: true },
        xMode:              { type: String as PropType<"elapsed" | "clock">, default: "elapsed" },
        invincibleIntervals:{ type: Array as PropType<GanttSegment[]>, default: () => [] },
    },
    setup(props) {
        const duration = computed(() => Math.max(props.timeEnd - props.timeStart, 1));

        const segLeft  = (start: number) => ((start - props.timeStart) / duration.value) * 100;
        const segWidth = (start: number, end: number) => ((end - start) / duration.value) * 100;

        // 裁切到 [timeStart, timeEnd] 範圍內，避免遮罩超出邊界
        const clippedInvincible = computed(() =>
            props.invincibleIntervals
                .map((iv) => ({
                    start: Math.max(iv.start, props.timeStart),
                    end:   Math.min(iv.end === Infinity ? props.timeEnd : iv.end, props.timeEnd),
                }))
                .filter((iv) => iv.start < iv.end),
        );

        const tickInterval = computed(() => {
            const d = duration.value;
            for (const iv of [5, 10, 15, 20, 30, 60, 90, 120, 180, 300, 600]) {
                if (d / iv <= 12) return iv;
            }
            return 600;
        });

        const ticks = computed(() => {
            const iv  = tickInterval.value;
            const out: { value: number; pct: number; label: string }[] = [];
            let t = Math.ceil(props.timeStart / iv) * iv;
            while (t <= props.timeEnd) {
                out.push({
                    value: t,
                    pct: ((t - props.timeStart) / duration.value) * 100,
                    label: props.xMode === "elapsed"
                        ? fmtElapsed(t - props.timeStart)
                        : fmtClock(t),
                });
                t += iv;
            }
            return out;
        });

        const fmtElapsed = (sec: number): string => {
            if (sec < 60) return `${Math.round(sec)}s`;
            const m = Math.floor(sec / 60);
            const s = Math.round(sec % 60);
            return s === 0 ? `${m}m` : `${m}m${s}s`;
        };

        const fmtClock = (ts: number): string =>
            new Date(ts * 1000).toLocaleTimeString("zh-TW", { hour12: false });

        return { ticks, segLeft, segWidth, clippedInvincible };
    },
});
</script>

<style scoped>
.gantt-root { width: 100%; }

.gantt-scroll-wrap { width: 100%; }

.gantt-inner { width: 100%; }

/* ── 列 ── */
.gantt-row {
    display: flex;
    align-items: stretch;
    height: 34px;
}
.gantt-axis-row {
    display: flex;
    align-items: stretch;
}

/* ── 標籤欄 ── */
.gantt-label {
    flex: 0 0 110px;
    padding: 0 8px 0 4px;
    display: flex;
    flex-direction: row;
    align-items: center;
    gap: 4px;
    overflow: hidden;
}
.gantt-label-icon {
    flex-shrink: 0;
    image-rendering: pixelated;
}
.gantt-label-text {
    display: flex;
    flex-direction: column;
    justify-content: center;
    min-width: 0;
}
.gantt-label-name {
    font-size: 0.75rem;
    font-weight: 500;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}
.gantt-label-sub {
    font-size: 0.65rem;
    opacity: 0.55;
    margin-top: 1px;
}
.gantt-row-header .gantt-label-name { font-weight: 700; }

/* ── 橫條區域 ── */
.gantt-bars {
    flex: 1;
    position: relative;
    margin: 3px 0;
}

/* 格線 */
.gantt-grid {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 1px;
    background: rgba(128, 128, 128, 0.18);
    pointer-events: none;
    z-index: 0;
}

/* 時間段橫條 */
.gantt-bar {
    position: absolute;
    top: 4px;
    bottom: 4px;
    border-radius: 2px;
    opacity: 0.82;
    transition: opacity 0.1s;
    min-width: 2px;
    z-index: 1;
}
.gantt-bar:hover { opacity: 1; }

/* Boss 無敵期間遮罩（疊在橫條上方） */
.gantt-invinc {
    position: absolute;
    top: 0;
    bottom: 0;
    background: repeating-linear-gradient(
        -45deg,
        rgba(239, 68, 68, 0.18),
        rgba(239, 68, 68, 0.18) 3px,
        rgba(239, 68, 68, 0.06) 3px,
        rgba(239, 68, 68, 0.06) 6px
    );
    border-left:  1px solid rgba(239, 68, 68, 0.35);
    border-right: 1px solid rgba(239, 68, 68, 0.35);
    pointer-events: none;
    z-index: 2;
}

/* ── X 軸 ── */
.gantt-axis {
    flex: 1;
    position: relative;
    height: 20px;
}
.gantt-tick {
    position: absolute;
    font-size: 0.65rem;
    color: rgba(128, 128, 128, 0.75);
    transform: translateX(-50%);
    white-space: nowrap;
    top: 3px;
    pointer-events: none;
}
</style>
