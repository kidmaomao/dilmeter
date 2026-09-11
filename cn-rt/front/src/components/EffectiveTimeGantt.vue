<template>
    <v-card variant="outlined">
        <v-card-title class="text-subtitle-1 py-2 px-3 d-flex align-center">
            有效輸出時間
            <v-spacer />
            <v-btn-toggle
                v-if="!collapsed"
                v-model="xMode"
                mandatory
                density="compact"
                variant="outlined"
                class="mr-2"
            >
                <v-btn value="elapsed" size="x-small">經過時間</v-btn>
                <v-btn value="clock"   size="x-small">實際時刻</v-btn>
            </v-btn-toggle>
            <v-btn
                icon
                size="x-small"
                variant="text"
                @click="collapsed = !collapsed"
            >
                <v-icon size="small" class="text-disabled">{{ collapsed ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
            </v-btn>
        </v-card-title>
        <v-expand-transition>
        <div v-if="!collapsed">
            <v-divider />
            <v-card-text class="pa-3">

                <GanttChart
                    :rows="ganttRows"
                    :time-start="timeStart"
                    :time-end="timeEnd"
                    :x-mode="xMode"
                />

                <!-- Checkbox 控制顯示玩家 -->
                <v-divider class="mt-3 mb-2" />
                <div class="d-flex flex-wrap" style="gap: 2px 16px;">
                    <v-checkbox
                        v-for="p in playerList"
                        :key="p.entityId"
                        v-model="visibleIds"
                        :value="p.entityId"
                        :label="p.name"
                        density="compact"
                        hide-details
                        class="flex-grow-0"
                    />
                </div>

            </v-card-text>
        </div>
        </v-expand-transition>
    </v-card>
</template>

<script lang="ts">
import { defineComponent, inject, computed, ref, watch, type Ref, type PropType } from "vue";
import { ActorManager, EntityActor } from "@/eventActor";
import { buildEffectiveSegments, type BossSummary } from "@/summaryCollector";
import { getDisplayName } from "@/store";
import GanttChart, { type GanttRow } from "@/components/subComponents/GanttChart.vue";

// 玩家顏色循環
const PLAYER_COLORS = [
    "#4CAF50", "#2196F3", "#FF9800", "#E91E63", "#9C27B0",
    "#00BCD4", "#FF5722", "#607D8B", "#795548", "#009688",
];

type Segment = { start: number; end: number };

/** 多個玩家的時間段取聯集 */
function unionSegs(segsArr: Segment[][]): Segment[] {
    const all = segsArr.flat().sort((a, b) => a.start - b.start);
    if (!all.length) return [];
    const result: Segment[] = [{ ...all[0] }];
    for (let i = 1; i < all.length; i++) {
        const last = result[result.length - 1];
        if (all[i].start <= last.end) {
            last.end = Math.max(last.end, all[i].end);
        } else {
            result.push({ ...all[i] });
        }
    }
    return result;
}

/** 將時間段裁切，去除與無敵期間重疊的部分 */
function clipSegments(segs: Segment[], invincible: Segment[]): Segment[] {
    if (!invincible.length) return segs;
    let result: Segment[] = segs.map((s) => ({ ...s }));
    for (const inv of invincible) {
        const next: Segment[] = [];
        for (const seg of result) {
            if (inv.end <= seg.start || inv.start >= seg.end) {
                next.push(seg);                                   // 無重疊
            } else {
                if (seg.start < inv.start) next.push({ start: seg.start, end: inv.start });
                if (seg.end   > inv.end)   next.push({ start: inv.end,   end: seg.end   });
            }
        }
        result = next;
    }
    return result.filter((s) => s.end > s.start);
}

export default defineComponent({
    name: "EffectiveTimeGantt",
    components: { GanttChart },
    props: {
        summary:        { type: Object as PropType<BossSummary>, default: null },
        bossEntityKey:  { type: String, default: "" },
    },
    setup(props) {
        const actorManager = inject("actorManager") as Ref<ActorManager>;

        const collapsed  = ref(false);
        const xMode      = ref<"elapsed" | "clock">("elapsed");
        const visibleIds = ref<string[]>([]);

        const session         = computed(() => props.summary?.session ?? null);
        const timeStart       = computed(() => session.value?.startAt ?? 0);
        const timeEnd         = computed(() => session.value?.endAt   ?? 0);
        const effectiveDur    = computed(() => session.value?.effectiveDuration ?? 0);

        const playerList = computed(() =>
            (props.summary?.players ?? []).map((p) => ({
                entityId: p.entityId,
                name: getDisplayName(p.name),
            })),
        );

        // Boss / summary 切換時，重設可見玩家（全選）
        watch(
            () => props.bossEntityKey,
            () => { visibleIds.value = playerList.value.map((p) => p.entityId); },
            { immediate: true },
        );
        watch(playerList, (list) => {
            visibleIds.value = list.map((p) => p.entityId);
        });

        // 各玩家的有效輸出時間段（已裁切無敵期間）
        const playerSegments = computed((): Map<string, Segment[]> => {
            const map  = new Map<string, Segment[]>();
            const sess = props.summary?.session;
            if (!sess) return map;

            // 無敵期間（Infinity 結尾替換為 endAt）
            const invincSegs: Segment[] = sess.invincibleIntervals.map((iv) => ({
                start: iv.start,
                end:   iv.end === Infinity ? sess.endAt : iv.end,
            }));

            for (const p of props.summary!.players) {
                const entity = actorManager.value.entityMap[p.entityId] as EntityActor | undefined;
                if (!entity) continue;
                const damages = entity.applyDamages.filter(
                    (d) =>
                        d.TargetId === props.bossEntityKey &&
                        d.Damage > 0 &&
                        d.At >= sess.startAt &&
                        d.At <= sess.endAt,
                );
                const raw     = buildEffectiveSegments(damages);
                const clipped = clipSegments(raw, invincSegs);
                map.set(p.entityId, clipped);
            }
            return map;
        });

        // 甘特圖列
        const ganttRows = computed((): GanttRow[] => {
            const effDur = effectiveDur.value;
            const visible = playerList.value.filter((p) => visibleIds.value.includes(p.entityId));
            const visSegs = visible.map((p) => playerSegments.value.get(p.entityId) ?? []);

            const union    = unionSegs(visSegs);
            const unionDur = union.reduce((s, seg) => s + (seg.end - seg.start), 0);

            const pctOf = (dur: number) =>
                effDur > 0 ? `${((dur / effDur) * 100).toFixed(1)}%` : "";

            const rows: GanttRow[] = [
                {
                    label:    "全體",
                    sublabel: pctOf(unionDur),
                    segments: union,
                    color:    "rgba(99, 102, 241, 0.8)",
                    isHeader: true,
                },
            ];

            visible.forEach((p, i) => {
                const segs = playerSegments.value.get(p.entityId) ?? [];
                const dur  = segs.reduce((s, seg) => s + (seg.end - seg.start), 0);
                rows.push({
                    label:    p.name,
                    sublabel: pctOf(dur),
                    segments: segs,
                    color:    PLAYER_COLORS[i % PLAYER_COLORS.length],
                });
            });

            return rows;
        });

        return { collapsed, xMode, visibleIds, playerList, ganttRows, timeStart, timeEnd };
    },
});
</script>
