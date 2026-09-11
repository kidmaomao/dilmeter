<template>
    <section class="skill-timeline-component" aria-label="团队技能时间轴">
        <div class="skill-timeline-toolbar">
            <div class="skill-timeline-player">
                <v-icon icon="mdi-account-clock-outline" size="14" />
                <label v-if="allowPlayerSelection" for="skill-timeline-player">查看队员</label>
                <select
                    v-if="allowPlayerSelection"
                    id="skill-timeline-player"
                    :value="personalPlayer?.entityId || ''"
                    aria-label="选择要查看个人时间轴的队员"
                    @change="selectTimelinePlayer"
                >
                    <option v-for="player in players" :key="player.entityId" :value="player.entityId">{{ player.label }}</option>
                </select>
                <strong v-else>个人技能</strong>
            </div>
            <div class="skill-timeline-zoom" aria-label="时间轴缩放">
                <span>横轴密度</span>
                <button
                    v-for="option in zoomOptions"
                    :key="option"
                    type="button"
                    :class="{ active: pixelsPerSecond === option }"
                    @click="pixelsPerSecond = option"
                >{{ option }}×</button>
            </div>
            <span class="skill-timeline-summary">
                {{ activeRows.length }}/{{ personalRows.length }} 个伤害技能 · {{ totalUses }} 次施放
            </span>
            <button
                v-if="personalRows.length > defaultPersonalRowCount"
                type="button"
                class="skill-timeline-more"
                @click="showAllPersonal = !showAllPersonal"
            >{{ showAllPersonal ? "收起到前 10" : `显示更多（${personalRows.length - defaultPersonalRowCount}）` }}</button>
        </div>

        <div v-if="activeRows.length" class="skill-timeline-scroll">
            <div class="skill-timeline-stage" :style="{ width: `${labelWidth + canvasWidth}px` }">
                <div class="skill-timeline-axis-row">
                    <div class="skill-timeline-sticky-label axis-label">技能</div>
                    <div class="skill-timeline-axis" :style="{ width: `${canvasWidth}px` }">
                        <i
                            v-for="tick in ticks"
                            :key="tick.seconds"
                            :class="{ major: tick.major }"
                            :style="{ left: `${tick.left}px` }"
                        />
                        <span
                            v-for="tick in labelTicks"
                            :key="`label-${tick.seconds}`"
                            :style="{ left: `${tick.left}px` }"
                        >{{ formatElapsed(tick.seconds) }}</span>
                    </div>
                </div>

                <div
                    v-for="row in activeRows"
                    :key="row.key"
                    class="skill-timeline-row"
                >
                    <div class="skill-timeline-sticky-label row-label">
                        <img v-if="row.iconUrl" :src="row.iconUrl" :alt="`${row.label}图标`" @error="hideImage" />
                        <span>
                            <strong>{{ row.label }}</strong>
                            <small>{{ formatDamage(row.totalDamage) }} · {{ row.uses.length }} 次</small>
                        </span>
                    </div>
                    <div class="skill-timeline-track" :style="{ width: `${canvasWidth}px` }">
                        <i
                            v-for="tick in ticks"
                            :key="`grid-${row.key}-${tick.seconds}`"
                            class="timeline-grid-line"
                            :class="{ major: tick.major }"
                            :style="{ left: `${tick.left}px` }"
                        />
                        <button
                            v-for="(use, index) in positionedUses(row.uses)"
                            :key="`${row.key}-${use.skillId}-${use.at}-${index}`"
                            type="button"
                            class="skill-use-node"
                            :style="{ left: `${use.left}px`, top: `${use.top}px` }"
                            :title="`${use.name} · ${formatElapsed(use.at - startAt)}`"
                        >
                            <span>{{ use.skillId }}</span>
                            <img :src="use.iconUrl" :alt="use.name" @error="hideImage" />
                        </button>
                    </div>
                </div>
            </div>
        </div>
        <div v-else class="skill-timeline-empty">
            <v-icon icon="mdi-timeline-clock-outline" size="22" />
            <strong>本场尚未记录到该队员的伤害技能</strong>
            <span>旧版战斗记录可能只有伤害事件；新记录会同时保存服务器确认的技能施放。</span>
        </div>
        <p class="skill-timeline-note">
            横轴为战斗时间。仅统计所选队员本体实际造成伤害的技能，按技能总伤害排序，默认显示前 10 个；不再提供拥挤的全队同屏时间轴。
        </p>
    </section>
</template>

<script setup lang="ts">
import { computed, ref, watch, type PropType } from "vue";

export type SkillTimelineUse = {
    at: number;
    skillId: number;
    name: string;
    iconUrl: string;
    damage: number;
};

export type SkillTimelinePlayer = {
    entityId: string;
    label: string;
    uses: SkillTimelineUse[];
};

type TimelineRow = {
    key: string;
    label: string;
    iconUrl: string;
    uses: SkillTimelineUse[];
    totalDamage: number;
};

const props = defineProps({
    players: {
        type: Array as PropType<SkillTimelinePlayer[]>,
        required: true,
    },
    personalPlayerId: { type: String, default: "" },
    allowPlayerSelection: { type: Boolean, default: false },
    startAt: { type: Number, required: true },
    endAt: { type: Number, required: true },
});

const emit = defineEmits<{
    (event: "update:personal-player-id", playerId: string): void;
}>();
const zoomOptions = [2, 4, 8];
const pixelsPerSecond = ref(8);
const defaultPersonalRowCount = 10;
const showAllPersonal = ref(false);
const labelWidth = 154;
const duration = computed(() => Math.max(1, props.endAt - props.startAt));
const canvasWidth = computed(() => Math.max(860, Math.ceil(duration.value * pixelsPerSecond.value)));
const personalPlayer = computed(() => props.players.find((player) => player.entityId === props.personalPlayerId) ?? props.players[0]);
watch(() => props.personalPlayerId, () => { showAllPersonal.value = false; });

const personalRows = computed<TimelineRow[]>(() => {
    const groups = new Map<number, SkillTimelineUse[]>();
    for (const use of personalPlayer.value?.uses ?? []) {
        const list = groups.get(use.skillId) ?? [];
        list.push(use);
        groups.set(use.skillId, list);
    }
    return [...groups.entries()]
        .map(([skillId, uses]) => ({
            key: `skill-${skillId}`,
            label: uses[0]?.name || `技能 ${skillId}`,
            iconUrl: uses[0]?.iconUrl || "",
            uses: dedupeUses(uses),
            totalDamage: uses.reduce((sum, use) => sum + Math.max(0, use.damage), 0),
        }))
        .filter((row) => row.totalDamage > 0 && row.uses.length > 0)
        .sort((left, right) => right.totalDamage - left.totalDamage || left.label.localeCompare(right.label, "zh-CN"));
});

const activeRows = computed(() => showAllPersonal.value ? personalRows.value : personalRows.value.slice(0, defaultPersonalRowCount));
const totalUses = computed(() => activeRows.value.reduce((sum, row) => sum + row.uses.length, 0));

const tickStep = computed(() => {
    const desiredSeconds = 70 / pixelsPerSecond.value;
    return [5, 10, 15, 30, 60, 120, 300].find((value) => value >= desiredSeconds) ?? 300;
});
const ticks = computed(() => {
    const result: Array<{ seconds: number; left: number; major: boolean }> = [];
    const minor = Math.max(1, tickStep.value / 2);
    for (let seconds = 0; seconds <= duration.value + 0.001; seconds += minor) {
        result.push({
            seconds,
            left: seconds * pixelsPerSecond.value,
            major: Math.abs(seconds % tickStep.value) < 0.001,
        });
    }
    return result;
});
const labelTicks = computed(() => ticks.value.filter((tick) => tick.major));

function positionedUses(uses: SkillTimelineUse[]) {
    const laneEnds = [-Infinity, -Infinity, -Infinity];
    return uses
        .filter((use) => use.at >= props.startAt && use.at <= props.endAt)
        .map((use) => {
            const left = Math.max(0, (use.at - props.startAt) * pixelsPerSecond.value);
            let lane = laneEnds.findIndex((laneEnd) => left - laneEnd >= 29);
            if (lane < 0) lane = laneEnds.indexOf(Math.min(...laneEnds));
            laneEnds[lane] = left;
            return { ...use, left, top: 8 };
        });
}

function selectTimelinePlayer(event: Event) {
    emit("update:personal-player-id", (event.currentTarget as HTMLSelectElement).value);
}

function dedupeUses(uses: SkillTimelineUse[]): SkillTimelineUse[] {
    const sorted = [...uses].sort((left, right) => left.at - right.at || left.skillId - right.skillId);
    const result: SkillTimelineUse[] = [];
    const lastUseAtBySkill = new Map<number, number>();
    for (const use of sorted) {
        const previousAt = lastUseAtBySkill.get(use.skillId);
        if (previousAt !== undefined && Math.abs(previousAt - use.at) <= 1) continue;
        result.push(use);
        lastUseAtBySkill.set(use.skillId, use.at);
    }
    return result;
}

function formatDamage(value: number): string {
    if (value >= 100_000_000) return `${(value / 100_000_000).toFixed(value >= 1_000_000_000 ? 1 : 2)}亿`;
    if (value >= 10_000) return `${(value / 10_000).toFixed(value >= 1_000_000 ? 1 : 2)}万`;
    return Math.round(value).toLocaleString("zh-CN");
}

function formatElapsed(rawSeconds: number): string {
    const seconds = Math.max(0, Math.round(rawSeconds));
    const minutes = Math.floor(seconds / 60);
    return `${String(minutes).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function hideImage(event: Event) {
    (event.currentTarget as HTMLImageElement).style.display = "none";
}
</script>

<style scoped>
.skill-timeline-component {
    min-height: 440px;
    color: var(--ui-theme-text);
    background: var(--ui-theme-canvas);
}
.skill-timeline-toolbar { display: flex; align-items: center; gap: 10px; margin-bottom: 8px; }
.skill-timeline-player,
.skill-timeline-zoom { display: flex; align-items: center; gap: 4px; }
.skill-timeline-player { min-width: 180px; color: var(--ui-theme-text); font-size: 10px; }
.skill-timeline-player label { color: var(--ui-theme-muted); white-space: nowrap; }
.skill-timeline-player strong { font-size: 10px; }
.skill-timeline-player select { min-width: 128px; height: 26px; padding: 0 24px 0 7px; color: var(--ui-theme-text); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); font-size: 10px; }
.skill-timeline-zoom button,
.skill-timeline-more { display: inline-flex; align-items: center; justify-content: center; gap: 4px; height: 26px; padding: 0 10px; color: var(--ui-theme-text); background: linear-gradient(var(--ui-theme-raised), var(--ui-theme-control)); border: 1px solid var(--ui-theme-border); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--ui-theme-canvas) 62%, transparent); font-size: 10px; cursor: pointer; }
.skill-timeline-zoom button:hover,
.skill-timeline-more:hover { background: linear-gradient(var(--ui-theme-surface-hover), var(--ui-theme-control)); }
.skill-timeline-zoom button.active { color: var(--ui-theme-on-accent); background: var(--ui-theme-selected); border-color: var(--ui-color-accent); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--ui-theme-on-accent) 28%, transparent); }
.skill-timeline-zoom { margin-left: auto; color: var(--ui-theme-muted); font-size: 10px; }
.skill-timeline-zoom button { width: 31px; padding: 0; }
.skill-timeline-more { border-color: var(--ui-color-accent); white-space: nowrap; }
.skill-timeline-summary { color: var(--ui-theme-muted); font-size: 10px; white-space: nowrap; }
.skill-timeline-scroll { max-height: 500px; overflow: auto; background: var(--ui-theme-inset); border: 1px solid var(--ui-theme-border-soft); scrollbar-color: var(--ui-theme-border) var(--ui-theme-control); scrollbar-width: thin; }
.skill-timeline-stage { min-width: 100%; }
.skill-timeline-axis-row,
.skill-timeline-row { display: grid; grid-template-columns: 154px 1fr; }
.skill-timeline-axis-row { position: sticky; z-index: 8; top: 0; height: 35px; }
.skill-timeline-sticky-label { position: sticky; z-index: 6; left: 0; box-sizing: border-box; background: linear-gradient(90deg, var(--ui-theme-raised), var(--ui-theme-surface)); border-right: 1px solid var(--ui-theme-border-soft); }
.axis-label { display: flex; align-items: center; padding-left: 11px; color: var(--ui-theme-text); border-bottom: 1px solid var(--ui-theme-border-soft); font-size: 10px; font-weight: 700; }
.skill-timeline-axis { position: relative; background: var(--ui-theme-surface); border-bottom: 1px solid var(--ui-theme-border-soft); }
.skill-timeline-axis i,
.timeline-grid-line { position: absolute; top: 0; bottom: 0; width: 1px; background: color-mix(in srgb, var(--ui-theme-border-soft) 34%, transparent); pointer-events: none; }
.skill-timeline-axis i.major,
.timeline-grid-line.major { background: color-mix(in srgb, var(--ui-color-accent) 24%, transparent); }
.skill-timeline-axis span { position: absolute; top: 9px; transform: translateX(-50%); color: var(--ui-theme-muted); font-size: 9px; font-variant-numeric: tabular-nums; white-space: nowrap; }
.skill-timeline-axis span:first-of-type { transform: none; }
.skill-timeline-row { min-height: 46px; border-bottom: 1px solid var(--ui-theme-border-soft); }
.row-label { display: flex; align-items: center; gap: 8px; min-width: 0; padding: 5px 8px; }
.row-label img { width: 29px; height: 29px; object-fit: cover; background: var(--ui-theme-inset); border: 1px solid var(--ui-theme-border); box-shadow: 0 0 0 1px var(--ui-theme-control); }
.row-label > span { display: grid; min-width: 0; }
.row-label strong { overflow: hidden; color: var(--ui-theme-text); font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }
.row-label small { margin-top: 2px; color: var(--ui-theme-muted); font-size: 8px; }
.skill-timeline-track { position: relative; min-height: inherit; background: linear-gradient(90deg, color-mix(in srgb, var(--ui-color-accent) 4%, transparent), transparent 34%); }
.skill-use-node { position: absolute; z-index: 2; display: grid; place-items: center; width: 25px; height: 25px; padding: 0; overflow: hidden; transform: translateX(-50%); color: var(--ui-theme-muted); background: var(--ui-theme-control); border: 1px solid var(--ui-theme-border); border-radius: 3px; box-shadow: 0 1px 3px color-mix(in srgb, var(--ui-theme-canvas) 76%, transparent), 0 0 5px color-mix(in srgb, var(--ui-color-accent) 28%, transparent); cursor: help; }
.skill-use-node:hover { z-index: 4; border-color: var(--ui-color-accent); transform: translateX(-50%) scale(1.2); }
.skill-use-node span { font-size: 6px; }
.skill-use-node img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; }
.skill-timeline-empty { display: grid; place-items: center; align-content: center; min-height: 370px; color: var(--ui-theme-muted); background: var(--ui-theme-surface); border: 1px solid var(--ui-theme-border-soft); text-align: center; }
.skill-timeline-empty strong { margin-top: 8px; color: var(--ui-theme-text); font-size: 12px; }
.skill-timeline-empty span { margin-top: 4px; font-size: 9px; }
.skill-timeline-note { margin: 7px 0 0; color: var(--ui-theme-muted); font-size: 9px; }
</style>
