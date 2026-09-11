<template>
    <v-container fluid class="pa-2">

        <!-- ── Boss 選擇器 ─────────────────────────────────────────── -->
        <v-row dense class="mb-2">
            <v-col cols="12" sm="5">
                <v-autocomplete
                    v-model="selectedBossId"
                    :items="bossOptions"
                    item-title="label"
                    item-value="id"
                    label="選擇 Boss / 怪物"
                    density="compact"
                    hide-details
                    clearable
                    auto-select-first
                />
            </v-col>
            <v-col cols="12" sm="7" class="d-flex align-center flex-wrap" style="gap:6px;">
                <v-btn
                    v-for="q in quickBossOptions"
                    :key="q.raceId"
                    size="small"
                    :variant="quickRaceFilter === q.raceId ? 'flat' : 'tonal'"
                    :color="quickRaceFilter === q.raceId ? 'primary' : undefined"
                    :disabled="!q.entityKey"
                    @click="quickSelectBoss(q.raceId, q.entityKey)"
                >
                    {{ q.name }}
                </v-btn>
                <v-spacer />
                <PlayerRenameDialog :players="renamePlayers" />
            </v-col>
        </v-row>

        <!-- ── 未選擇提示 ──────────────────────────────────────────── -->
        <v-row v-if="!selectedBossId" dense>
            <v-col cols="12">
                <v-alert
                    type="info"
                    variant="tonal"
                    density="compact"
                    icon="mdi-cursor-pointer"
                >
                    請從上方選擇 Boss / 怪物以顯示 Summary
                </v-alert>
            </v-col>
        </v-row>

        <!-- ── 無資料 ──────────────────────────────────────────────── -->
        <v-row v-else-if="!summary" dense>
            <v-col cols="12">
                <v-alert type="warning" variant="tonal" density="compact">
                    此目標尚無足夠的傷害紀錄
                </v-alert>
            </v-col>
        </v-row>

        <template v-else>

            <!-- ── 工具列 ──────────────────────────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12" class="d-flex justify-end">
                    <ConfigImportExport />
                </v-col>
            </v-row>

            <!-- ── Section 1: 戰鬥概覽 ──────────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12">
                    <v-card variant="outlined">
                        <v-card-title
                            class="text-subtitle-1 py-2 px-3 d-flex align-center"
                            style="cursor: pointer; user-select: none"
                            @click="secCollapsed.overview = !secCollapsed.overview"
                        >
                            戰鬥概覽
                            <v-spacer />
                            <v-icon size="small" class="text-disabled">{{ secCollapsed.overview ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                        </v-card-title>
                        <v-expand-transition>
                        <div v-if="!secCollapsed.overview">
                        <v-divider />
                        <v-card-text class="pa-3">
                            <v-row dense>
                                <v-col cols="6" sm="3">
                                    <div class="text-caption text-disabled">總戰鬥時間</div>
                                    <div class="text-body-1 font-weight-bold">
                                        {{ fmtDuration(summary.session.totalDuration) }}
                                    </div>
                                    <div class="text-caption text-disabled">
                                        {{ fmtTime(summary.session.startAt) }}
                                        ～
                                        {{ fmtTime(summary.session.endAt) }}
                                    </div>
                                </v-col>
                                <v-col cols="6" sm="3">
                                    <div class="text-caption text-disabled">有效輸出時間</div>
                                    <div class="text-body-1 font-weight-bold text-success">
                                        {{ fmtDuration(summary.session.effectiveDuration) }}
                                    </div>
                                    <div class="text-caption text-disabled">
                                        {{ fmtPct(summary.session.effectiveDuration / summary.session.totalDuration) }}
                                    </div>
                                </v-col>
                                <v-col cols="6" sm="3">
                                    <div class="text-caption text-disabled">無法輸出時間</div>
                                    <div class="text-body-1 font-weight-bold text-error">
                                        {{ fmtDuration(summary.session.inactiveDuration) }}
                                    </div>
                                    <div class="text-caption text-disabled">
                                        無敵段：{{ summary.session.invincibleIntervals.length }} 次
                                    </div>
                                </v-col>
                                <v-col cols="6" sm="3">
                                    <div class="text-caption text-disabled">全隊總傷害</div>
                                    <div class="text-body-1 font-weight-bold">
                                        {{ Math.round(summary.totalDamage).toLocaleString() }}
                                    </div>
                                    <div class="text-caption text-disabled">
                                        有效 DPS {{ fmtDps(summary.totalDamage / summary.session.effectiveDuration) }}
                                    </div>
                                </v-col>
                            </v-row>

                            <!-- 時間分布 Timeline -->
                            <v-row dense class="mt-2">
                                <v-col cols="12">
                                    <div class="d-flex align-center mb-1" style="gap: 8px">
                                        <span class="text-caption text-disabled">時間分布</span>
                                        <v-chip
                                            v-if="selectedTimeRange"
                                            size="x-small"
                                            color="primary"
                                            closable
                                            @click:close="selectedTimeRange = null"
                                        >
                                            {{ fmtTime(selectedTimeRange.startAt) }} – {{ fmtTime(selectedTimeRange.endAt) }}
                                        </v-chip>
                                    </div>
                                    <div class="d-flex rounded overflow-hidden" style="height: 12px; gap: 1px">
                                        <div
                                            v-for="(seg, i) in buildTimeline(summary.session)"
                                            :key="i"
                                            :class="[
                                                seg.active ? 'bg-success' : 'bg-error',
                                                seg.active ? 'timeline-seg-active' : '',
                                                selectedTimeRange && seg.startAt === selectedTimeRange.startAt && seg.endAt === selectedTimeRange.endAt
                                                    ? 'timeline-seg-selected' : ''
                                            ]"
                                            :style="{ flex: seg.flex }"
                                            :title="seg.active ? `${fmtTime(seg.startAt)} – ${fmtTime(seg.endAt)}` : undefined"
                                            @click="seg.active && (selectedTimeRange = (selectedTimeRange && seg.startAt === selectedTimeRange.startAt && seg.endAt === selectedTimeRange.endAt) ? null : { startAt: seg.startAt, endAt: seg.endAt })"
                                        />
                                    </div>
                                    <div class="d-flex text-caption text-disabled mt-1" style="gap: 12px">
                                        <span><span class="text-success">■</span> 有效（可點擊篩選）</span>
                                        <span><span class="text-error">■</span> 無效（無敵 / 空窗）</span>
                                    </div>
                                </v-col>
                            </v-row>

                            <!-- Boss 技能使用（摺疊） -->
                            <v-divider class="mt-3 mb-1" />
                            <div
                                class="d-flex align-center py-1"
                                style="cursor: pointer; user-select: none"
                                @click="bossSkillCollapsed = !bossSkillCollapsed"
                            >
                                <span class="text-caption text-disabled flex-grow-1">BOSS 技能使用次數</span>
                                <v-icon size="small" class="text-disabled">
                                    {{ bossSkillCollapsed ? 'mdi-chevron-down' : 'mdi-chevron-up' }}
                                </v-icon>
                            </div>
                            <v-expand-transition>
                                <div v-if="!bossSkillCollapsed">
                                    <v-data-table
                                        :headers="bossSkillHeaders"
                                        :items="summary.bossSkillStats"
                                        density="compact"
                                        :items-per-page="-1"
                                        hide-default-footer
                                        no-data-text="無資料"
                                        :sort-by="[{ key: 'totalHits', order: 'desc' }]"
                                    >
                                        <template #item.skillId="{ item }">
                                            {{ skillNameMap[item.skillId] ?? `#${item.skillId}` }}
                                        </template>
                                        <template #item.totalDamage="{ item }">
                                            {{ Math.round(item.totalDamage).toLocaleString() }}
                                        </template>
                                    </v-data-table>
                                </div>
                            </v-expand-transition>
                            <!-- Boss CC 狀態覆蓋率（摺疊） -->
                            <template v-if="bossCondCoverage.length">
                                <v-divider class="mt-3 mb-1" />
                                <div class="d-flex align-center py-1" style="gap: 4px">
                                    <span
                                        class="text-caption text-disabled flex-grow-1"
                                        style="cursor: pointer; user-select: none"
                                        @click="bossCondCollapsed = !bossCondCollapsed"
                                    >BOSS 狀態覆蓋率</span>
                                    <v-btn-toggle
                                        v-model="bossCondTab"
                                        mandatory
                                        density="compact"
                                        variant="outlined"
                                    >
                                        <v-btn value="icons" size="x-small">圖示</v-btn>
                                        <v-btn value="gantt" size="x-small">甘特</v-btn>
                                    </v-btn-toggle>
                                    <v-btn
                                        size="x-small"
                                        :variant="bossCondSettingsOpen ? 'tonal' : 'text'"
                                        :color="bossCondIncludeIds.length ? 'primary' : undefined"
                                        icon="mdi-filter-cog-outline"
                                        @click.stop="bossCondSettingsOpen = !bossCondSettingsOpen"
                                    />
                                    <v-icon
                                        size="small"
                                        class="text-disabled"
                                        style="cursor: pointer"
                                        @click="bossCondCollapsed = !bossCondCollapsed"
                                    >
                                        {{ bossCondCollapsed ? 'mdi-chevron-down' : 'mdi-chevron-up' }}
                                    </v-icon>
                                </div>

                                <!-- 篩選設定 -->
                                <v-expand-transition>
                                    <div v-if="bossCondSettingsOpen" class="mb-2">
                                        <v-autocomplete
                                            v-model="bossCondIncludeIds"
                                            :items="bossCondOptions"
                                            item-title="name"
                                            item-value="ccId"
                                            label="顯示的狀態（空白 = 全部）"
                                            density="compact"
                                            hide-details
                                            multiple
                                            chips
                                            closable-chips
                                            clearable
                                        />
                                    </div>
                                </v-expand-transition>

                                <v-expand-transition>
                                    <div v-if="!bossCondCollapsed">

                                        <!-- 圖示 tab -->
                                        <div v-if="bossCondTab === 'icons'" class="d-flex flex-wrap mt-1" style="gap: 6px">
                                            <v-tooltip
                                                v-for="row in bossCondFilteredCoverage"
                                                :key="row.ccId"
                                                location="top"
                                            >
                                                <template #activator="{ props: tp }">
                                                    <div
                                                        v-bind="tp"
                                                        class="d-flex flex-column align-center"
                                                        style="width: 44px; gap: 2px"
                                                    >
                                                        <img
                                                            :src="`/res/characterconditionimage/${region}/${row.ccId}/${row.ccId}.png`"
                                                            width="32" height="32"
                                                            style="image-rendering: pixelated"
                                                        />
                                                        <span class="text-caption font-weight-medium" style="font-size: 0.65rem; text-align: center; line-height: 1.2">
                                                            {{ fmtPct(row.coverageEffective) }}
                                                        </span>
                                                        <span class="text-caption text-disabled" style="font-size: 0.6rem; text-align: center; line-height: 1.2">
                                                            {{ fmtPct(row.coverageTotal) }}
                                                        </span>
                                                    </div>
                                                </template>
                                                <div class="text-caption">
                                                    <div class="font-weight-bold">{{ row.name }}</div>
                                                    <div>有效時間覆蓋：{{ fmtPct(row.coverageEffective) }}</div>
                                                    <div class="text-disabled">總時間覆蓋：{{ fmtPct(row.coverageTotal) }}</div>
                                                    <div class="text-disabled">活躍時長：{{ fmtDuration(row.activeSec) }}</div>
                                                </div>
                                            </v-tooltip>
                                        </div>

                                        <!-- 甘特 tab -->
                                        <template v-else-if="bossCondTab === 'gantt' && bossCondGanttData">
                                            <div class="d-flex justify-end mb-1">
                                                <v-btn-toggle
                                                    v-model="bossCondXMode"
                                                    mandatory
                                                    density="compact"
                                                    variant="outlined"
                                                >
                                                    <v-btn value="elapsed" size="x-small">經過時間</v-btn>
                                                    <v-btn value="clock"   size="x-small">實際時刻</v-btn>
                                                </v-btn-toggle>
                                            </div>
                                            <GanttChart
                                                :rows="bossCondGanttData.rows"
                                                :time-start="bossCondGanttData.timeStart"
                                                :time-end="bossCondGanttData.timeEnd"
                                                :invincible-intervals="bossCondGanttData.invincible"
                                                :x-mode="bossCondXMode"
                                            />
                                        </template>

                                    </div>
                                </v-expand-transition>
                            </template>
                        </v-card-text>
                        </div>
                        </v-expand-transition>
                    </v-card>
                </v-col>
            </v-row>

            <!-- ── DPS 曲線圖 ────────────────────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12">
                    <v-card variant="outlined">
                        <v-card-title
                            class="text-subtitle-1 py-2 px-3 d-flex align-center"
                            style="cursor: pointer; user-select: none"
                            @click="secCollapsed.dpsChart = !secCollapsed.dpsChart"
                        >
                            DPS 曲線
                            <v-spacer />
                            <v-icon size="small" class="text-disabled">{{ secCollapsed.dpsChart ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                        </v-card-title>
                        <v-expand-transition>
                        <div v-if="!secCollapsed.dpsChart">
                            <v-divider />
                            <v-card-text class="pa-3">
                                <summary-dps-chart
                                    :entities="dpsChartEntities"
                                    :start-at="summary.session.startAt"
                                    :end-at="summary.session.endAt"
                                />
                            </v-card-text>
                        </div>
                        </v-expand-transition>
                    </v-card>
                </v-col>
            </v-row>

            <!-- ── Section 2: 玩家輸出 ──────────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12">
                    <v-card variant="outlined">
                        <v-card-title class="text-subtitle-1 py-2 px-3 d-flex align-center">
                            玩家輸出
                            <v-spacer />
                            <v-menu
                                v-model="colSettingsOpen"
                                :close-on-content-click="false"
                                location="bottom end"
                            >
                                <template v-slot:activator="{ props }">
                                    <v-btn
                                        size="x-small"
                                        variant="text"
                                        icon="mdi-table-cog"
                                        v-bind="props"
                                        class="ml-1"
                                    />
                                </template>
                                <v-card min-width="200" class="pa-1">
                                    <div class="text-caption text-disabled px-3 pt-2 pb-1 font-weight-bold">玩家欄位</div>
                                    <v-list density="compact" class="pa-0">
                                        <v-list-item
                                            v-for="col in PLAYER_COL_DEFS"
                                            :key="col.key"
                                            class="px-2 py-0"
                                        >
                                            <v-checkbox
                                                :label="col.label"
                                                :model-value="visiblePlayerCols.includes(col.key)"
                                                @update:model-value="(v) => togglePlayerCol(col.key, !!v)"
                                                density="compact"
                                                hide-details
                                            />
                                        </v-list-item>
                                    </v-list>
                                    <v-divider class="my-1" />
                                    <div class="text-caption text-disabled px-3 pt-1 pb-1 font-weight-bold">技能欄位</div>
                                    <v-list density="compact" class="pa-0">
                                        <v-list-item
                                            v-for="col in SKILL_COL_DEFS"
                                            :key="col.key"
                                            class="px-2 py-0"
                                        >
                                            <v-checkbox
                                                :label="col.label"
                                                :model-value="visibleSkillCols.includes(col.key)"
                                                @update:model-value="(v) => toggleSkillCol(col.key, !!v)"
                                                density="compact"
                                                hide-details
                                            />
                                        </v-list-item>
                                    </v-list>
                                </v-card>
                            </v-menu>
                            <v-btn
                                icon
                                size="x-small"
                                variant="text"
                                class="ml-1"
                                @click="secCollapsed.playerOutput = !secCollapsed.playerOutput"
                            >
                                <v-icon size="small" class="text-disabled">{{ secCollapsed.playerOutput ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-btn>
                        </v-card-title>
                        <v-expand-transition>
                        <div v-if="!secCollapsed.playerOutput">
                        <v-divider />

                        <!-- Table header（pr-10 補齊 expansion-panel 箭頭佔位） -->
                        <v-row
                            dense
                            no-gutters
                            align="center"
                            class="px-4 py-1 text-caption text-disabled"
                            style="border-bottom: 1px solid rgba(128,128,128,0.2); padding-right: 52px !important"
                        >
                            <v-col sm="2">名稱</v-col>
                            <v-col v-if="visiblePlayerCols.includes('totalDamage')"   sm="2" class="text-right pr-2">總傷害 / 佔比</v-col>
                            <v-col v-if="visiblePlayerCols.includes('activeDamage')"  sm="2" class="text-right pr-2">主動 / 被動</v-col>
                            <v-col v-if="visiblePlayerCols.includes('critRate')"      sm="1" class="text-right pr-2">暴擊</v-col>
                            <v-col v-if="visiblePlayerCols.includes('totalDPS')"      sm="1" class="text-right pr-2">全場 DPS</v-col>
                            <v-col v-if="visiblePlayerCols.includes('effectiveDPS')"  sm="2" class="text-right pr-2">有效 DPS</v-col>
                            <v-col v-if="visiblePlayerCols.includes('individualDPS')" sm="2" class="text-right pr-2">
                                個人有效 DPS
                                <v-tooltip location="top" max-width="240">
                                    <template v-slot:activator="{ props }">
                                        <v-icon v-bind="props" size="x-small" class="ml-1">mdi-information-outline</v-icon>
                                    </template>
                                    <div>括號內為個人有效時間 / 團隊有效時間</div>
                                    <div class="mt-1 text-disabled">超過 100% 表示機制 / 無敵時間時仍有對王造成傷害</div>
                                </v-tooltip>
                            </v-col>
                        </v-row>

                        <v-expansion-panels variant="accordion" flat>
                            <v-expansion-panel
                                v-for="player in summary.players"
                                :key="player.entityId"
                            >
                                <v-expansion-panel-title class="py-1 px-3">
                                    <v-row dense align="center" no-gutters>
                                        <!-- 名稱 -->
                                        <v-col cols="12" sm="2">
                                            <div class="text-body-2 font-weight-medium text-truncate">
                                                {{ getDisplayName(player.name) }}
                                            </div>
                                            <div class="text-caption text-disabled">
                                                <span v-if="player.jobName" class="text-primary mr-1">{{ player.jobName }}</span>
                                                {{ raceNameMap[player.raceId] ?? player.raceId }}
                                            </div>
                                        </v-col>
                                        <!-- 總傷害 + 佔比 -->
                                        <v-col v-if="visiblePlayerCols.includes('totalDamage')" cols="6" sm="2" class="text-right pr-2">
                                            <div class="text-body-2">
                                                {{ Math.round(player.totalDamage).toLocaleString() }}
                                            </div>
                                            <div class="text-caption text-disabled">
                                                {{ fmtPct(summary.effectiveBossDamage > 0 ? player.totalDamage / summary.effectiveBossDamage : 0) }}
                                            </div>
                                        </v-col>
                                        <!-- 主動 / 被動 -->
                                        <v-col v-if="visiblePlayerCols.includes('activeDamage')" cols="6" sm="2" class="text-right pr-2">
                                            <div class="text-body-2">
                                                {{ fmtPct(player.activeDamageRatio) }}
                                                <span class="text-disabled">/</span>
                                                {{ fmtPct(player.passiveDamageRatio) }}
                                            </div>
                                        </v-col>
                                        <!-- 暴擊率 -->
                                        <v-col v-if="visiblePlayerCols.includes('critRate')" cols="6" sm="1" class="text-right pr-2">
                                            <div class="text-body-2">
                                                {{ fmtPct(player.critRate) }}
                                            </div>
                                        </v-col>
                                        <!-- 全場 DPS -->
                                        <v-col v-if="visiblePlayerCols.includes('totalDPS')" cols="6" sm="1" class="text-right pr-2">
                                            <div class="text-body-2">{{ fmtDps(player.totalDPS) }}</div>
                                        </v-col>
                                        <!-- 有效 DPS -->
                                        <v-col v-if="visiblePlayerCols.includes('effectiveDPS')" cols="6" sm="2" class="text-right pr-2">
                                            <div class="text-body-2">{{ fmtDps(player.effectiveDPS) }}</div>
                                        </v-col>
                                        <!-- 個人有效 DPS -->
                                        <v-col v-if="visiblePlayerCols.includes('individualDPS')" cols="6" sm="2" class="text-right pr-2">
                                            <div class="text-body-2">
                                                {{ fmtDps(player.individualEffectiveDPS) }}
                                            </div>
                                            <div class="text-caption text-disabled">
                                                {{ fmtDuration(player.individualEffectiveTime) }}
                                                <span class="ml-1">({{ fmtPct(summary.session.effectiveDuration > 0 ? player.individualEffectiveTime / summary.session.effectiveDuration : 0) }})</span>
                                            </div>
                                        </v-col>
                                    </v-row>
                                </v-expansion-panel-title>

                                <v-expansion-panel-text class="pa-0">
                                    <!-- 傷害佔比條 -->
                                    <div class="px-3 py-1">
                                        <v-progress-linear
                                            :model-value="summary.effectiveBossDamage > 0 ? (player.totalDamage / summary.effectiveBossDamage) * 100 : 0"
                                            color="primary"
                                            height="4"
                                            rounded
                                        />
                                    </div>

                                    <!-- 武器 -->
                                    <div v-if="player.weapons.length > 0" class="px-3 pb-2 d-flex flex-wrap" style="gap: 4px 16px;">
                                        <div
                                            v-for="w in player.weapons"
                                            :key="w.pocketType"
                                        >
                                            <span class="text-body-2">{{ itemNameMap[w.itemId] ?? `#${w.itemId}` }}</span>
                                            <span class="text-caption text-disabled ml-1">Pocket {{ w.pocketType }}</span>
                                        </div>
                                    </div>

                                    <!-- 技能明細 -->
                                    <v-data-table
                                        :headers="skillStatHeaders"
                                        :items="player.skillStats"
                                        density="compact"
                                        :items-per-page="-1"
                                        hide-default-footer
                                        no-data-text="無技能資料"
                                        :sort-by="[{ key: 'totalDamage', order: 'desc' }]"
                                    >
                                        <template #item.skillId="{ item }">
                                            <div>
                                                {{ skillNameMap[item.skillId] ?? `#${item.skillId}` }}
                                            </div>
                                            <!-- 觸發來源（僅被動技能顯示） -->
                                            <div
                                                v-if="item.triggerSources.length > 0"
                                                class="d-flex flex-wrap mt-1"
                                                style="gap: 4px"
                                            >
                                                <v-chip
                                                    v-for="src in item.triggerSources"
                                                    :key="src.skillId"
                                                    size="x-small"
                                                    variant="tonal"
                                                    color="secondary"
                                                    :title="`${skillNameMap[src.skillId] ?? `#${src.skillId}`}：${src.count} 次觸發，${Math.round(src.damage).toLocaleString()} 傷害`"
                                                >
                                                    {{ skillNameMap[src.skillId] ?? `#${src.skillId}` }}
                                                    <span class="text-disabled ml-1">
                                                        {{ fmtPct(item.totalDamage > 0 ? src.damage / item.totalDamage : 0) }}
                                                    </span>
                                                </v-chip>
                                            </div>
                                        </template>
                                        <template #item.useCount="{ item }">
                                            {{ item.totalHits.toLocaleString() }}
                                        </template>
                                        <template #item.usesPerMinute="{ item }">
                                            {{ item.usesPerMinute.toFixed(1) }}
                                        </template>
                                        <template #item.totalDamage="{ item }">
                                            {{ Math.round(item.totalDamage).toLocaleString() }}
                                        </template>
                                        <template #item.damagePerUse="{ item }">
                                            {{ Math.round(item.damagePerUse).toLocaleString() }}
                                        </template>
                                        <template #item.critRate="{ item }">
                                            <span v-if="item.noCritRate" class="text-disabled">—</span>
                                            <span v-else>{{ fmtPct(item.critRate) }}</span>
                                        </template>
                                        <template #item.critDmg="{ item }">
                                            <span v-if="item.maxCritDamage == null" class="text-disabled">—</span>
                                            <span v-else>
                                                {{ Math.round(item.minCritDamage!).toLocaleString() }}
                                                <span class="text-disabled"> ~ </span>
                                                {{ Math.round(item.maxCritDamage).toLocaleString() }}
                                            </span>
                                        </template>
                                        <template #item.nonCritDmg="{ item }">
                                            <span v-if="item.maxNonCritDamage == null" class="text-disabled">—</span>
                                            <span v-else>
                                                {{ Math.round(item.minNonCritDamage!).toLocaleString() }}
                                                <span class="text-disabled"> ~ </span>
                                                {{ Math.round(item.maxNonCritDamage).toLocaleString() }}
                                            </span>
                                        </template>
                                        <template #header.categoryRatio>
                                            <span>佔比</span>
                                            <v-tooltip location="top" max-width="260">
                                                <template v-slot:activator="{ props }">
                                                    <v-icon
                                                        v-bind="props"
                                                        size="x-small"
                                                        class="ml-1 text-disabled"
                                                    >mdi-information-outline</v-icon>
                                                </template>
                                                <div>
                                                    <div><b>分類佔比</b>：此技能佔同類別（主動 / 被動）總傷害的比例</div>
                                                    <div class="mt-1"><b>總體佔比</b>：此技能佔玩家總傷害的比例</div>
                                                    <div class="mt-1 text-disabled">格式：分類% / 總體%</div>
                                                </div>
                                            </v-tooltip>
                                        </template>
                                        <template #item.categoryRatio="{ item }">
                                            <template v-for="r in [computeSkillRatios(player, item)]" :key="0">
                                                <span>{{ fmtPct(r.categoryRatio) }}</span>
                                                <span class="text-disabled mx-1">/</span>
                                                <span>{{ fmtPct(r.overallRatio) }}</span>
                                            </template>
                                        </template>
                                    </v-data-table>
                                </v-expansion-panel-text>
                            </v-expansion-panel>
                        </v-expansion-panels>

                        <div
                            v-if="summary.players.length === 0"
                            class="text-center text-caption text-disabled py-4"
                        >
                            此 Boss 無玩家傷害紀錄
                        </div>
                        </div>
                        </v-expand-transition>
                    </v-card>
                </v-col>
            </v-row>

            <!-- ── Section 2.5: 有效輸出甘特圖 ────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12">
                    <EffectiveTimeGantt
                        :summary="summary"
                        :boss-entity-key="selectedBossId"
                    />
                </v-col>
            </v-row>

            <!-- ── Section 2.6: 玩家技能 CC 分析 ────────────────────────── -->
            <v-row dense class="mb-2">
                <v-col cols="12">
                    <PlayerSkillCCAnalysis
                        :summary="summary"
                        :boss-entity-key="selectedBossId"
                    />
                </v-col>
            </v-row>

            <!-- ── Section 3: 自訂條件 ──────────────────────────────── -->
            <v-row dense>
                <v-col cols="12">
                    <v-card variant="outlined">
                        <v-card-title class="text-subtitle-1 py-2 px-3 d-flex align-center">
                            自訂條件分析
                            <v-spacer />
                            <v-btn
                                size="x-small"
                                variant="text"
                                prepend-icon="mdi-cog"
                                @click="conditionConfigOpen = true"
                            >
                                管理條件
                            </v-btn>
                            <v-btn
                                icon
                                size="x-small"
                                variant="text"
                                class="ml-1"
                                @click="secCollapsed.customConditions = !secCollapsed.customConditions"
                            >
                                <v-icon size="small" class="text-disabled">{{ secCollapsed.customConditions ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-btn>
                        </v-card-title>
                        <v-expand-transition>
                        <div v-if="!secCollapsed.customConditions">
                        <v-divider />

                        <!-- 尚無自訂條件 -->
                        <div
                            v-if="customConditionConfigs.length === 0"
                            class="text-center text-caption text-disabled py-6"
                        >
                            <v-icon size="32" class="mb-2 d-block">mdi-filter-plus-outline</v-icon>
                            尚無自訂條件，點擊右上角「管理條件」新增
                        </div>

                        <!-- 各自訂條件結果 -->
                        <template v-else>
                            <div
                                v-for="result in summary.customConditions"
                                :key="result.configId"
                                class="pa-3"
                            >
                                <!-- 條件標題列 -->
                                <v-row dense align="center" class="mb-1">
                                    <v-col>
                                        <span class="text-subtitle-2 font-weight-bold">
                                            {{ result.displayName }}
                                        </span>
                                    </v-col>
                                    <v-col cols="auto" class="text-right">
                                        <span class="text-caption text-disabled mr-3">
                                            {{ result.groups.length }} 段
                                        </span>
                                        <span class="text-caption text-disabled mr-3">
                                            總計 {{ Math.round(result.totalDamage).toLocaleString() }}
                                        </span>
                                        <span class="text-caption text-disabled">
                                            平均 {{ Math.round(result.averageDamage).toLocaleString() }}
                                        </span>
                                    </v-col>
                                </v-row>

                                <!-- 整合 header（pr 補齊箭頭佔位） -->
                                <div
                                    class="d-flex align-center px-4 py-1 text-caption text-disabled"
                                    style="border-bottom: 1px solid rgba(128,128,128,0.2); padding-right: 52px !important"
                                >
                                    <div style="width: 28px">段</div>
                                    <div style="width: 152px">時間</div>
                                    <div style="width: 68px; text-align: right; padding-right: 8px">持續</div>
                                    <div
                                        v-for="name in conditionMatrices[result.configId].playerNames"
                                        :key="name"
                                        style="flex: 1; text-align: right; padding-right: 8px"
                                    >{{ name }}</div>
                                    <div style="width: 96px; text-align: right">合計</div>
                                </div>

                                <!-- 整合 expansion panels：title 顯示矩陣資料，展開顯示玩家詳細 -->
                                <v-expansion-panels variant="accordion" flat>
                                    <v-expansion-panel
                                        v-for="(group, gi) in result.groups"
                                        :key="group.index"
                                    >
                                        <v-expansion-panel-title class="py-1 px-3">
                                            <div class="d-flex align-center" style="width: 100%">
                                                <div style="width: 28px" class="text-body-2 font-weight-medium">
                                                    {{ group.index }}
                                                </div>
                                                <div style="width: 152px" class="text-caption text-disabled">
                                                    {{ fmtTime(group.startAt) }} ～ {{ fmtTime(group.endAt) }}
                                                </div>
                                                <div style="width: 68px; text-align: right; padding-right: 8px" class="text-body-2">
                                                    {{ fmtDuration(group.duration) }}
                                                </div>
                                                <div
                                                    v-for="(dmg, j) in conditionMatrices[result.configId].rows[gi].damages"
                                                    :key="j"
                                                    style="flex: 1; text-align: right; padding-right: 8px"
                                                    class="text-body-2"
                                                >
                                                    <span v-if="dmg !== null">{{ Math.round(dmg).toLocaleString() }}</span>
                                                    <span v-else class="text-disabled">—</span>
                                                </div>
                                                <div style="width: 96px; text-align: right" class="text-body-2 font-weight-medium">
                                                    {{ Math.round(group.totalDamage).toLocaleString() }}
                                                    <div class="text-caption text-disabled">
                                                        {{ fmtPct(result.totalDamage > 0 ? group.totalDamage / result.totalDamage : 0) }}
                                                    </div>
                                                </div>
                                            </div>
                                        </v-expansion-panel-title>

                                        <v-expansion-panel-text class="pa-0">
                                            <!-- 玩家列 header -->
                                            <v-row
                                                dense no-gutters align="center"
                                                class="px-4 py-1 text-caption text-disabled"
                                                style="border-bottom: 1px solid rgba(128,128,128,0.2); padding-right: 52px !important"
                                            >
                                                <v-col sm="3">名稱</v-col>
                                                <v-col sm="3" class="text-right pr-2">總傷害 / 佔比</v-col>
                                                <v-col sm="3" class="text-right pr-2">主動 / 被動</v-col>
                                                <v-col sm="3" class="text-right pr-2">暴擊</v-col>
                                            </v-row>

                                            <!-- 各玩家 expansion panel -->
                                            <v-expansion-panels variant="accordion" flat>
                                                <v-expansion-panel
                                                    v-for="p in group.players"
                                                    :key="p.entityId"
                                                >
                                                    <v-expansion-panel-title class="py-1 px-3">
                                                        <v-row dense align="center" no-gutters>
                                                            <v-col sm="3">
                                                                <div class="text-body-2 font-weight-medium text-truncate">
                                                                    {{ getDisplayName(p.name) }}
                                                                </div>
                                                            </v-col>
                                                            <v-col sm="3" class="text-right pr-2">
                                                                <div class="text-body-2">
                                                                    {{ Math.round(p.totalDamage).toLocaleString() }}
                                                                </div>
                                                                <div class="text-caption text-disabled">
                                                                    {{ fmtPct(group.totalDamage > 0 ? p.totalDamage / group.totalDamage : 0) }}
                                                                </div>
                                                            </v-col>
                                                            <v-col sm="3" class="text-right pr-2">
                                                                <div class="text-body-2">
                                                                    {{ fmtPct(p.totalDamage > 0 ? (p.totalDamage - p.passiveDamage) / p.totalDamage : 0) }}
                                                                    <span class="text-disabled">/</span>
                                                                    {{ fmtPct(p.totalDamage > 0 ? p.passiveDamage / p.totalDamage : 0) }}
                                                                </div>
                                                            </v-col>
                                                            <v-col sm="3" class="text-right pr-2">
                                                                <div class="text-body-2">
                                                                    {{ p.critEligible > 0 ? fmtPct(p.critHits / p.critEligible) : '—' }}
                                                                </div>
                                                            </v-col>
                                                        </v-row>
                                                    </v-expansion-panel-title>

                                                    <v-expansion-panel-text class="pa-0">
                                                        <!-- 傷害佔比條 -->
                                                        <div class="px-3 py-1">
                                                            <v-progress-linear
                                                                :model-value="group.totalDamage > 0 ? (p.totalDamage / group.totalDamage) * 100 : 0"
                                                                color="primary"
                                                                height="4"
                                                                rounded
                                                            />
                                                        </div>
                                                        <!-- 技能序列 -->
                                                        <v-data-table
                                                            :headers="skillSeqHeaders"
                                                            :items="p.skillSequence"
                                                            density="compact"
                                                            :items-per-page="-1"
                                                            hide-default-footer
                                                            no-data-text="無技能紀錄"
                                                        >
                                                            <template #item.at="{ item }">
                                                                {{ fmtTime(item.at) }}
                                                            </template>
                                                            <template #item.skillId="{ item }">
                                                                <span>{{ skillNameMap[item.skillId] ?? `#${item.skillId}` }}</span>
                                                                <span v-if="item.triggerSkillId" class="text-caption text-disabled ml-1">from:{{ skillNameMap[item.triggerSkillId] ?? `#${item.triggerSkillId}` }}</span>
                                                            </template>
                                                            <template #item.totalDamage="{ item }">
                                                                {{ Math.round(item.totalDamage).toLocaleString() }}
                                                            </template>
                                                            <template #item.isCrit="{ item }">
                                                                <v-icon v-if="item.isCrit" size="x-small" color="warning">
                                                                    mdi-lightning-bolt
                                                                </v-icon>
                                                            </template>
                                                        </v-data-table>
                                                    </v-expansion-panel-text>
                                                </v-expansion-panel>
                                            </v-expansion-panels>
                                        </v-expansion-panel-text>
                                    </v-expansion-panel>
                                </v-expansion-panels>

                                <!-- 合計列 -->
                                <div
                                    class="d-flex align-center px-4 py-1"
                                    style="border-top: 2px solid rgba(128,128,128,0.25)"
                                >
                                    <div style="width: 28px" class="text-caption font-weight-medium">合計</div>
                                    <div style="width: 152px"></div>
                                    <div style="width: 68px"></div>
                                    <div
                                        v-for="(total, j) in conditionMatrices[result.configId].totals"
                                        :key="j"
                                        style="flex: 1; text-align: right; padding-right: 8px"
                                        class="text-body-2 font-weight-medium"
                                    >
                                        {{ total !== null ? Math.round(total).toLocaleString() : '—' }}
                                    </div>
                                    <div style="width: 96px; text-align: right" class="text-body-2 font-weight-bold">
                                        {{ Math.round(result.totalDamage).toLocaleString() }}
                                    </div>
                                </div>

                                <v-divider v-if="result !== summary.customConditions[summary.customConditions.length - 1]" class="mt-2" />
                            </div>
                        </template>
                        </div>
                        </v-expand-transition>
                    </v-card>
                </v-col>
            </v-row>
        </template>

        <!-- ── 自訂條件管理 Dialog ─────────────────────────────────── -->
        <v-dialog v-model="conditionConfigOpen" max-width="700px" scrollable>
            <v-card>
                <v-card-title class="d-flex align-center">
                    <v-icon class="mr-2">mdi-filter-cog</v-icon>
                    自訂條件管理
                    <v-spacer />
                    <v-btn icon variant="text" @click="conditionConfigOpen = false">
                        <v-icon>mdi-close</v-icon>
                    </v-btn>
                </v-card-title>
                <v-divider />

                <v-card-text style="max-height: 500px">
                    <!-- 條件列表 -->
                    <div
                        v-if="customConditionConfigs.length === 0"
                        class="text-center text-caption text-disabled py-4"
                    >
                        尚無自訂條件
                    </div>

                    <v-list density="compact" class="pa-0">
                        <v-list-item
                            v-for="(config, idx) in customConditionConfigs"
                            :key="config.id"
                            class="px-0 py-1"
                        >
                            <template #prepend>
                                <div class="d-flex flex-column mr-1">
                                    <v-btn
                                        icon
                                        size="x-small"
                                        variant="text"
                                        :disabled="idx === 0"
                                        @click="reorderCustomCondition(idx, idx - 1)"
                                    >
                                        <v-icon size="small">mdi-chevron-up</v-icon>
                                    </v-btn>
                                    <v-btn
                                        icon
                                        size="x-small"
                                        variant="text"
                                        :disabled="idx === customConditionConfigs.length - 1"
                                        @click="reorderCustomCondition(idx, idx + 1)"
                                    >
                                        <v-icon size="small">mdi-chevron-down</v-icon>
                                    </v-btn>
                                </div>
                            </template>

                            <v-row dense align="center" no-gutters>
                                <v-col>
                                    <div class="text-body-2 font-weight-medium">
                                        {{ config.displayName || '（未命名）' }}
                                    </div>
                                    <div class="text-caption text-disabled">
                                        {{ describeFilter(config.filter, condNameMap) }}
                                        ／
                                        {{ config.groupBy === 'eachEntry' ? '每段分組' : '合計' }}
                                    </div>
                                </v-col>
                                <v-col cols="auto">
                                    <v-btn
                                        icon
                                        size="small"
                                        variant="text"
                                        @click="startEditCondition(config)"
                                    >
                                        <v-icon size="small">mdi-pencil</v-icon>
                                    </v-btn>
                                    <v-btn
                                        icon
                                        size="small"
                                        variant="text"
                                        color="error"
                                        @click="removeCustomCondition(config.id)"
                                    >
                                        <v-icon size="small">mdi-delete</v-icon>
                                    </v-btn>
                                </v-col>
                            </v-row>
                        </v-list-item>
                    </v-list>
                </v-card-text>

                <v-divider />
                <v-card-actions>
                    <!-- 匯出 / 匯入 -->
                    <v-btn
                        size="small"
                        variant="text"
                        prepend-icon="mdi-export"
                        @click="handleExport"
                    >匯出</v-btn>
                    <v-btn
                        size="small"
                        variant="text"
                        prepend-icon="mdi-import"
                        @click="handleImport"
                    >匯入</v-btn>
                    <v-spacer />
                    <v-btn
                        color="primary"
                        variant="flat"
                        prepend-icon="mdi-plus"
                        @click="startAddCondition"
                    >新增條件</v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>

        <!-- ── 條件編輯 Dialog ─────────────────────────────────────── -->
        <v-dialog v-model="editDialogOpen" max-width="600px" scrollable>
            <v-card v-if="editingConfig">
                <v-card-title class="d-flex align-center">
                    <v-icon class="mr-2">mdi-filter-edit</v-icon>
                    {{ isNewCondition ? '新增自訂條件' : '編輯自訂條件' }}
                    <v-spacer />
                    <v-btn icon variant="text" @click="editDialogOpen = false">
                        <v-icon>mdi-close</v-icon>
                    </v-btn>
                </v-card-title>
                <v-divider />

                <v-card-text style="max-height: 480px">
                    <!-- 顯示名稱 -->
                    <v-text-field
                        v-model="editingConfig.displayName"
                        label="顯示名稱"
                        density="compact"
                        variant="outlined"
                        class="mb-3"
                        placeholder="例：崩壞期間爆發"
                        hide-details
                    />

                    <!-- 分組方式 -->
                    <v-radio-group
                        v-model="editingConfig.groupBy"
                        inline
                        density="compact"
                        class="mb-3"
                        hide-details
                    >
                        <template #label>
                            <span class="text-body-2 mr-2">分組方式</span>
                        </template>
                        <v-radio value="eachEntry" label="每段分組（每次進出條件各算一段）" />
                        <v-radio value="none" label="全部合計" />
                    </v-radio-group>

                    <!-- 條件規則（目前僅支援單一 leaf，巢狀未來擴展） -->
                    <v-divider class="mb-3" />
                    <div class="text-body-2 font-weight-medium mb-2">條件規則</div>

                    <template v-if="editingConfig.filter.type === 'leaf'">
                        <v-select
                            v-model="editingConfig.filter.mode"
                            :items="filterModeOptions"
                            item-title="label"
                            item-value="value"
                            label="觸發模式"
                            density="compact"
                            variant="outlined"
                            class="mb-3"
                            hide-details
                        />

                        <v-autocomplete
                            v-model="editingConfig.filter.conditionId"
                            :items="conditionOptions"
                            item-title="label"
                            item-value="id"
                            :custom-filter="conditionFilter"
                            label="Condition ID"
                            density="compact"
                            variant="outlined"
                            hide-details
                            class="mb-2"
                        />

                        <div class="text-caption text-disabled">
                            目前選擇：
                            <strong>
                                {{ condNameMap[editingConfig.filter.conditionId] ?? '（未知）' }}
                                （#{{ editingConfig.filter.conditionId }}）
                            </strong>
                        </div>
                    </template>

                    <!-- 預覽 -->
                    <v-divider class="mt-3 mb-2" />
                    <div class="text-caption text-disabled">
                        預覽：{{ describeFilter(editingConfig.filter, condNameMap) }}
                    </div>
                </v-card-text>

                <v-divider />
                <v-card-actions>
                    <v-spacer />
                    <v-btn variant="text" @click="editDialogOpen = false">取消</v-btn>
                    <v-btn
                        color="primary"
                        variant="flat"
                        :disabled="!editingConfig.displayName"
                        @click="saveEditingCondition"
                    >
                        {{ isNewCondition ? '新增' : '儲存' }}
                    </v-btn>
                </v-card-actions>
            </v-card>
        </v-dialog>

    </v-container>
</template>

<script lang="ts">
import { defineComponent, inject, computed, ref, watch, type Ref } from "vue";
import { ActorManager, EntityActor } from "@/eventActor";
import { prettyEntityName } from "@/lib/util";
import SummaryDpsChart, { type DpsChartEntity } from "@/components/subComponents/summaryDpsChart.vue";
import EffectiveTimeGantt from "@/components/EffectiveTimeGantt.vue";
import PlayerRenameDialog from "@/components/subComponents/PlayerRenameDialog.vue";
import PlayerSkillCCAnalysis from "@/components/subComponents/PlayerSkillCCAnalysis.vue";
import ConfigImportExport from "@/components/subComponents/ConfigImportExport.vue";
import GanttChart, { type GanttRow } from "@/components/subComponents/GanttChart.vue";
import {
    buildBossSummary,
    sumIntervalDuration,
    type BossSummary,
    type CustomConditionResult,
    type TimeInterval,
} from "@/summaryCollector";
import { getDisplayName } from "@/store";
import {
    customConditionConfigs,
    addCustomCondition,
    updateCustomCondition,
    removeCustomCondition,
    reorderCustomCondition,
    makeLeafFilter,
    makeEmptyConditionConfig,
    describeFilter,
    exportConfigs,
    importConfigs,
    type CustomConditionConfig,
    type ConditionFilterMode,
} from "@/summaryConfig";

// ── 欄位可見性設定 ──────────────────────────────────────────────────────
const PLAYER_COL_DEFS = [
    { key: "totalDamage",   label: "總傷害 / 佔比" },
    { key: "activeDamage",  label: "主動 / 被動" },
    { key: "critRate",      label: "暴擊" },
    { key: "totalDPS",      label: "全場 DPS" },
    { key: "effectiveDPS",  label: "有效 DPS" },
    { key: "individualDPS", label: "個人有效 DPS" },
] as const;

const SKILL_COL_DEFS = [
    { key: "useCount",      label: "使用次數" },
    { key: "usesPerMinute", label: "次/分鐘" },
    { key: "damagePerUse",  label: "平均傷害" },
    { key: "critRate",      label: "暴擊率" },
    { key: "critDmg",       label: "暴擊傷害" },
    { key: "nonCritDmg",    label: "無暴擊傷害" },
    { key: "categoryRatio", label: "佔比" },
] as const;

type PlayerColKey = (typeof PLAYER_COL_DEFS)[number]["key"];
type SkillColKey  = (typeof SKILL_COL_DEFS)[number]["key"];

const loadColPrefs = (storageKey: string, defaults: string[]): string[] => {
    try {
        const raw = localStorage.getItem(storageKey);
        if (raw) {
            const parsed = JSON.parse(raw);
            if (Array.isArray(parsed)) return parsed as string[];
        }
    } catch { /* ignore */ }
    return [...defaults];
};

// ────────────────────────────────────────────────────────────────────────────

export default defineComponent({
    name: "Summary",
    components: { SummaryDpsChart, EffectiveTimeGantt, PlayerRenameDialog, GanttChart, PlayerSkillCCAnalysis, ConfigImportExport },
    setup() {
        const actorManager = inject("actorManager") as Ref<ActorManager>;
        const skillNameMap = inject("skillNameMap") as Ref<Record<number, string>>;
        const raceNameMap  = inject("raceNameMap")  as Ref<Record<number, string>>;
        const condNameMap  = inject("condNameMap")  as Ref<Record<number, string>>;
        const itemNameMap  = inject("itemNameMap")  as Ref<Record<number, string>>;
        const region       = inject("region")       as Ref<string>;

        // ── Boss 選擇器 ─────────────────────────────────────────────
        const selectedBossId  = ref<string>("");
        const quickRaceFilter = ref<number | null>(null);

        const bossOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const opts: { id: string; label: string }[] = [];
            for (const k in map) {
                const e = map[k] as EntityActor;
                if (e.isPC) continue;
                if (e.ownerId) continue;
                if (e.takeDamages.length === 0) continue;
                if (quickRaceFilter.value !== null && e.raceId !== quickRaceFilter.value) continue;
                const name =
                    prettyEntityName(e, raceNameMap) ??
                    raceNameMap.value[e.raceId] ??
                    `Race ${e.raceId}`;
                const firstAt = e.takeDamages.reduce((m, d) => Math.min(m, d.At), Infinity);
                const lastAt  = e.takeDamages.reduce((m, d) => Math.max(m, d.At), -Infinity);
                opts.push({
                    id: k,
                    label: `${name}  ${fmtTime(firstAt)} ～ ${fmtTime(lastAt)}`,
                });
            }
            return opts;
        });

        // ── 快速選擇 Boss ────────────────────────────────────────────
        const QUICK_RACE_IDS = [7603, 7602, 7600, 7601] as const;

        const quickBossOptions = computed(() => {
            const map = actorManager.value.entityMap;
            // 先收集所有合法候選（條件與 bossOptions 一致）
            const candidates = Object.entries(map).filter(
                ([, e]) => !e.isPC && !e.ownerId && e.takeDamages.length > 0,
            );
            return QUICK_RACE_IDS.map((raceId) => {
                // 同 raceId 取最晚一筆傷害時間最大的 entity
                const matched = candidates
                    .filter(([, e]) => e.raceId === raceId)
                    .sort(([, a], [, b]) => {
                        const lastA = a.takeDamages.reduce((m, d) => Math.max(m, d.At), -Infinity);
                        const lastB = b.takeDamages.reduce((m, d) => Math.max(m, d.At), -Infinity);
                        return lastB - lastA;
                    });
                const name = raceNameMap.value[raceId] ?? `Race ${raceId}`;
                return { raceId, name, entityKey: matched[0]?.[0] ?? null };
            });
        });

        const quickSelectBoss = (raceId: number, entityKey: string | null) => {
            // 同一個 race 再按一次 → 取消篩選
            if (quickRaceFilter.value === raceId) {
                quickRaceFilter.value = null;
                return;
            }
            quickRaceFilter.value = raceId;
            // 若目前選取的 entity 不屬於此 race，清掉再自動選最新的
            const map = actorManager.value.entityMap;
            const cur = selectedBossId.value ? map[selectedBossId.value] : null;
            if (!cur || cur.raceId !== raceId) {
                selectedBossId.value = entityKey ?? "";
            }
        };

        // ── 時間段選取 ──────────────────────────────────────────────
        const selectedTimeRange = ref<{ startAt: number; endAt: number } | null>(null);

        // Boss 切換時清除時間段選取
        watch(selectedBossId, () => { selectedTimeRange.value = null; });

        // ── Summary 計算 ────────────────────────────────────────────
        const summary = computed((): BossSummary | null => {
            if (!selectedBossId.value) return null;
            return buildBossSummary(
                selectedBossId.value,
                actorManager.value,
                customConditionConfigs.value,
                selectedTimeRange.value ?? undefined,
            );
        });

        // ── 格式化 ──────────────────────────────────────────────────
        const fmtTime = (ts: number | null | typeof Infinity) => {
            if (ts == null || ts === Infinity || ts === -Infinity) return "—";
            return new Date(ts * 1000).toLocaleTimeString("zh-TW", { hour12: false });
        };

        const fmtDuration = (sec: number) => {
            if (!isFinite(sec) || sec <= 0) return "0s";
            if (sec < 60) return `${sec.toFixed(1)}s`;
            const m = Math.floor(sec / 60);
            const s = Math.round(sec % 60).toString().padStart(2, "0");
            return `${m}m ${s}s`;
        };

        const fmtPct = (ratio: number) => {
            if (!isFinite(ratio)) return "—";
            return `${(ratio * 100).toFixed(1)}%`;
        };

        const fmtDps = (dps: number) => {
            if (!isFinite(dps) || dps <= 0) return "—";
            if (dps >= 1_000_000) return `${(dps / 1_000_000).toFixed(2)}M/s`;
            if (dps >= 1_000)     return `${(dps / 1_000).toFixed(1)}k/s`;
            return `${dps.toFixed(0)}/s`;
        };

        // ── 欄位可見性 ──────────────────────────────────────────────
        const visiblePlayerCols = ref<PlayerColKey[]>(
            loadColPrefs("mabidil_playerColVis", PLAYER_COL_DEFS.map((d) => d.key) as unknown as string[]) as PlayerColKey[],
        );
        const visibleSkillCols = ref<SkillColKey[]>(
            loadColPrefs("mabidil_skillColVis", SKILL_COL_DEFS.map((d) => d.key) as unknown as string[]) as SkillColKey[],
        );

        watch(visiblePlayerCols, (v) => localStorage.setItem("mabidil_playerColVis", JSON.stringify(v)), { deep: true });
        watch(visibleSkillCols,  (v) => localStorage.setItem("mabidil_skillColVis",  JSON.stringify(v)), { deep: true });

        const colSettingsOpen = ref(false);

        const togglePlayerCol = (key: PlayerColKey, checked: boolean) => {
            if (checked && !visiblePlayerCols.value.includes(key)) {
                visiblePlayerCols.value = [...visiblePlayerCols.value, key];
            } else if (!checked) {
                visiblePlayerCols.value = visiblePlayerCols.value.filter((k) => k !== key);
            }
        };
        const toggleSkillCol = (key: SkillColKey, checked: boolean) => {
            if (checked && !visibleSkillCols.value.includes(key)) {
                visibleSkillCols.value = [...visibleSkillCols.value, key];
            } else if (!checked) {
                visibleSkillCols.value = visibleSkillCols.value.filter((k) => k !== key);
            }
        };

        // ── 技能佔比計算 ─────────────────────────────────────────────
        const computeSkillRatios = (player: any, item: any) => {
            const isPassive = !!item.noCritRate;
            const categoryTotal = (player.skillStats as any[])
                .filter((s) => !!s.noCritRate === isPassive)
                .reduce((sum, s) => sum + s.totalDamage, 0);
            return {
                categoryRatio: categoryTotal > 0 ? item.totalDamage / categoryTotal : 0,
                overallRatio:  player.totalDamage > 0 ? item.totalDamage / player.totalDamage : 0,
            };
        };

        // ── Table headers ────────────────────────────────────────────
        const ALL_SKILL_STAT_HEADERS = [
            { title: "技能",      key: "skillId",       sortable: true,  align: "start" as const },
            { title: "使用次數",  key: "useCount",       sortable: true,  align: "end"   as const },
            { title: "次/分鐘",   key: "usesPerMinute",  sortable: true,  align: "end"   as const },
            { title: "總傷害",    key: "totalDamage",    sortable: true,  align: "end"   as const },
            { title: "平均傷害",  key: "damagePerUse",   sortable: true,  align: "end"   as const },
            { title: "暴擊率",    key: "critRate",       sortable: true,  align: "end"   as const },
            { title: "暴擊傷害",  key: "critDmg",        sortable: false, align: "end"   as const },
            { title: "無暴擊傷害",key: "nonCritDmg",     sortable: false, align: "end"   as const },
            { title: "佔比",      key: "categoryRatio",  sortable: false, align: "end"   as const },
        ];

        const skillStatHeaders = computed(() =>
            ALL_SKILL_STAT_HEADERS.filter(
                (h) => h.key === "skillId" || h.key === "totalDamage" || visibleSkillCols.value.includes(h.key as SkillColKey),
            ),
        );

        const skillSeqHeaders = [
            { title: "時間",   key: "at",          sortable: false },
            { title: "技能",   key: "skillId",      sortable: true  },
            { title: "總傷害", key: "totalDamage",  sortable: true  },
            { title: "暴",     key: "isCrit",       sortable: false },
        ];

        // ── Boss CC 覆蓋率 ──────────────────────────────────────────
        const bossCondCoverage = computed(() => {
            if (!summary.value || !selectedBossId.value) return [];
            const boss = actorManager.value.entityMap[selectedBossId.value] as EntityActor | undefined;
            if (!boss) return [];

            const { startAt, endAt, totalDuration, effectiveDuration, invincibleIntervals } = summary.value.session;
            if (totalDuration <= 0) return [];

            const history = boss.conditionHistory;
            const activeMap = new Map<number, { activeSec: number; effectiveActiveSec: number }>();

            for (let i = 0; i < history.length; i++) {
                const segStart = Math.max(history[i].At, startAt);
                const segEnd   = i + 1 < history.length
                    ? Math.min(history[i + 1].At, endAt)
                    : endAt;
                if (segEnd <= segStart) continue;
                const dur = segEnd - segStart;

                // 有效時間 = 此段扣除無敵區間的時長
                const invincOverlap = sumIntervalDuration(
                    invincibleIntervals as TimeInterval[],
                    segStart,
                    segEnd,
                );
                const effectiveDur = Math.max(0, dur - invincOverlap);

                for (const cond of history[i].List) {
                    const entry = activeMap.get(cond.CCId) ?? { activeSec: 0, effectiveActiveSec: 0 };
                    entry.activeSec          += dur;
                    entry.effectiveActiveSec += effectiveDur;
                    activeMap.set(cond.CCId, entry);
                }
            }

            return [...activeMap.entries()]
                .map(([ccId, { activeSec, effectiveActiveSec }]) => ({
                    ccId,
                    name: condNameMap.value[ccId] ?? `#${ccId}`,
                    activeSec,
                    coverageTotal:     activeSec          / totalDuration,
                    coverageEffective: effectiveDuration > 0
                        ? effectiveActiveSec / effectiveDuration
                        : 0,
                }))
                .sort((a, b) => b.coverageEffective - a.coverageEffective);
        });

        // ── Boss UI 狀態持久化 ───────────────────────────────────────
        const BOSS_UI_KEY = "mabidil_bossUI";
        const _bossUI = (() => {
            try { const r = localStorage.getItem(BOSS_UI_KEY); return r ? JSON.parse(r) : {}; }
            catch { return {}; }
        })();
        const saveBossUI = () => {
            try {
                localStorage.setItem(BOSS_UI_KEY, JSON.stringify({
                    bossSkillCollapsed: bossSkillCollapsed.value,
                    bossCondCollapsed:  bossCondCollapsed.value,
                    bossCondTab:        bossCondTab.value,
                    bossCondXMode:      bossCondXMode.value,
                    bossCondIncludeIds: bossCondIncludeIds.value,
                    secOverview:        secCollapsed.value.overview,
                    secDpsChart:        secCollapsed.value.dpsChart,
                    secPlayerOutput:    secCollapsed.value.playerOutput,
                    secCustomCond:      secCollapsed.value.customConditions,
                }));
            } catch { /* ignore */ }
        };

        const bossCondCollapsed    = ref<boolean>(_bossUI.bossCondCollapsed  ?? true);
        const bossCondSettingsOpen = ref(false);
        const bossCondIncludeIds   = ref<number[]>(_bossUI.bossCondIncludeIds ?? []);
        const bossCondTab          = ref<"icons" | "gantt">(_bossUI.bossCondTab  ?? "icons");
        const bossCondXMode        = ref<"elapsed" | "clock">(_bossUI.bossCondXMode ?? "elapsed");

        watch(bossCondCollapsed,  saveBossUI);
        watch(bossCondTab,        saveBossUI);
        watch(bossCondXMode,      saveBossUI);
        watch(bossCondIncludeIds, saveBossUI, { deep: true });

        const bossCondOptions = computed(() =>
            bossCondCoverage.value.map((r) => ({ ccId: r.ccId, name: r.name })),
        );

        const bossCondFilteredCoverage = computed(() => {
            if (!bossCondIncludeIds.value.length) return bossCondCoverage.value;
            const ids = new Set(bossCondIncludeIds.value);
            return bossCondCoverage.value.filter((r) => ids.has(r.ccId));
        });

        const CC_GANTT_COLORS = [
            "#5470c6","#91cc75","#fac858","#ee6666","#73c0de",
            "#3ba272","#fc8452","#9a60b4","#ea7ccc","#00b4d8",
        ];

        const bossCondGanttData = computed(() => {
            if (!summary.value || !selectedBossId.value) return null;
            const boss = actorManager.value.entityMap[selectedBossId.value] as EntityActor | undefined;
            if (!boss) return null;

            const { startAt, endAt, invincibleIntervals } = summary.value.session;

            // 非無敵區間 → 有效輸出時間段
            const invSegs = invincibleIntervals.map((iv) => ({
                start: iv.start,
                end:   iv.end === Infinity ? endAt : iv.end,
            })).sort((a, b) => a.start - b.start);

            const effectiveSegs: { start: number; end: number }[] = [];
            let cur = startAt;
            for (const iv of invSegs) {
                if (iv.start > cur) effectiveSegs.push({ start: cur, end: iv.start });
                cur = Math.max(cur, iv.end);
            }
            if (cur < endAt) effectiveSegs.push({ start: cur, end: endAt });

            // 從 conditionHistory 抽出各 CCId 的活躍區間
            const ccIntervals = new Map<number, { start: number; end: number }[]>();
            const history = boss.conditionHistory;
            for (let i = 0; i < history.length; i++) {
                const segStart = Math.max(history[i].At, startAt);
                const segEnd   = i + 1 < history.length
                    ? Math.min(history[i + 1].At, endAt)
                    : endAt;
                if (segEnd <= segStart) continue;
                for (const cond of history[i].List) {
                    const arr = ccIntervals.get(cond.CCId) ?? [];
                    arr.push({ start: segStart, end: segEnd });
                    ccIntervals.set(cond.CCId, arr);
                }
            }

            // 套用篩選
            const filtered = bossCondFilteredCoverage.value;
            const effectiveDur = summary.value.session.effectiveDuration;

            const rows: GanttRow[] = [
                {
                    label:    "有效輸出",
                    sublabel: fmtPct(effectiveDur / summary.value.session.totalDuration),
                    segments: effectiveSegs,
                    color:    "rgba(99,179,237,0.5)",
                    isHeader: true,
                },
            ];

            filtered.forEach((row, i) => {
                const segs = ccIntervals.get(row.ccId) ?? [];
                rows.push({
                    label:    row.name,
                    sublabel: fmtPct(row.coverageEffective),
                    segments: segs,
                    color:    CC_GANTT_COLORS[i % CC_GANTT_COLORS.length],
                    iconUrl:  `/res/characterconditionimage/${region.value}/${row.ccId}/${row.ccId}.png`,
                });
            });

            return {
                rows,
                timeStart:  startAt,
                timeEnd:    endAt,
                invincible: invSegs,
            };
        });

        const bossSkillHeaders = [
            { title: "技能",     key: "skillId",    sortable: true, align: "start" },
            { title: "使用次數", key: "useCount",   sortable: true, align: "end"   },
            { title: "總命中",   key: "totalHits",  sortable: true, align: "end"   },
            { title: "總傷害",   key: "totalDamage",sortable: true, align: "end"   },
        ] as const;

        const bossSkillCollapsed = ref<boolean>(_bossUI.bossSkillCollapsed ?? true);
        watch(bossSkillCollapsed, saveBossUI);

        // ── 區塊折疊狀態 ─────────────────────────────────────────────
        const secCollapsed = ref({
            overview:         _bossUI.secOverview     ?? false,
            dpsChart:         _bossUI.secDpsChart     ?? false,
            playerOutput:     _bossUI.secPlayerOutput ?? false,
            customConditions: _bossUI.secCustomCond   ?? false,
        });
        watch(secCollapsed, saveBossUI, { deep: true });

        // ── 自訂條件管理 Dialog ──────────────────────────────────────
        const conditionConfigOpen = ref(false);
        const editDialogOpen = ref(false);
        const isNewCondition = ref(false);
        const editingConfig = ref<(Omit<CustomConditionConfig, "id"> & { _id?: string }) | null>(null);
        const editingOriginalId = ref<string | null>(null);

        const filterModeOptions: { value: ConditionFilterMode; label: string }[] = [
            { value: "onHit_targetHas",          label: "傷害時，目標身上有指定 condition" },
            { value: "onHit_attackerHas",         label: "傷害時，攻擊者身上有指定 condition" },
            { value: "duringCondition_target",    label: "指定 condition 在目標身上存在的期間（分段）" },
            { value: "duringCondition_attacker",  label: "指定 condition 在攻擊者身上存在的期間（分段）" },
        ];

        const conditionOptions = computed(() => {
            return Object.entries(condNameMap.value).map(([id, name]) => ({
                id: Number(id),
                label: `${name} (#${id})`,
            }));
        });

        const conditionFilter = (value: string, query: string) => {
            if (!query) return true;
            return value.toLowerCase().includes(query.toLowerCase());
        };

        const startAddCondition = () => {
            isNewCondition.value = true;
            editingOriginalId.value = null;
            editingConfig.value = makeEmptyConditionConfig() as Omit<CustomConditionConfig, "id"> & { _id?: string };
            editDialogOpen.value = true;
        };

        const startEditCondition = (config: CustomConditionConfig) => {
            isNewCondition.value = false;
            editingOriginalId.value = config.id;
            editingConfig.value = JSON.parse(JSON.stringify(config));
            editDialogOpen.value = true;
        };

        const saveEditingCondition = () => {
            if (!editingConfig.value) return;
            if (isNewCondition.value) {
                addCustomCondition(editingConfig.value);
            } else if (editingOriginalId.value) {
                updateCustomCondition(editingOriginalId.value, editingConfig.value);
            }
            editDialogOpen.value = false;
        };

        const handleExport = () => {
            const json = exportConfigs();
            const blob = new Blob([json], { type: "application/json" });
            const url  = URL.createObjectURL(blob);
            const a    = document.createElement("a");
            a.href     = url;
            a.download = "summary_conditions.json";
            a.click();
            URL.revokeObjectURL(url);
        };

        const handleImport = () => {
            const input = document.createElement("input");
            input.type  = "file";
            input.accept = ".json";
            input.onchange = (e) => {
                const file = (e.target as HTMLInputElement).files?.[0];
                if (!file) return;
                const reader = new FileReader();
                reader.onload = (ev) => {
                    const text = ev.target?.result as string;
                    const ok = importConfigs(text);
                    if (!ok) alert("匯入失敗：格式不正確");
                };
                reader.readAsText(file);
            };
            input.click();
        };

        // ── 自訂條件矩陣（段 × 玩家） ──────────────────────────────
        type ConditionMatrix = {
            playerNames: string[];
            rows: { label: string; damages: (number | null)[] }[];
            totals: (number | null)[];
        };

        const buildConditionMatrix = (result: CustomConditionResult): ConditionMatrix => {
            // 收集所有出現的玩家（依傷害降序，維持欄位一致）
            const playerMap = new Map<string, string>(); // entityId → name
            for (const group of result.groups) {
                for (const p of group.players) {
                    if (!playerMap.has(p.entityId)) playerMap.set(p.entityId, p.name);
                }
            }
            const playerIds   = [...playerMap.keys()];
            const playerNames = playerIds.map((id) => getDisplayName(playerMap.get(id)!));

            // 每段一列
            const rows = result.groups.map((group) => {
                const damages: (number | null)[] = playerIds.map((id) => {
                    const p = group.players.find((p) => p.entityId === id);
                    return p ? p.totalDamage : null;
                });
                return {
                    label: `段${group.index}  ${fmtTime(group.startAt)}`,
                    damages,
                };
            });

            // 各玩家合計
            const totals: (number | null)[] = playerIds.map((id) =>
                result.groups.reduce((s, g) => {
                    const p = g.players.find((p) => p.entityId === id);
                    return s + (p?.totalDamage ?? 0);
                }, 0),
            );

            return { playerNames, rows, totals };
        };

        /** 預先算好所有條件的矩陣，避免 template 重複呼叫 */
        const conditionMatrices = computed(() => {
            const results = summary.value?.customConditions ?? [];
            return Object.fromEntries(results.map((r) => [r.configId, buildConditionMatrix(r)]));
        });

        // ── 改名（PlayerRenameDialog 所需的玩家清單） ────────────────
        const renamePlayers = computed(() =>
            (summary.value?.players ?? []).map((p) => ({ entityId: p.entityId, name: p.name }))
        );

        // ── DPS 曲線圖資料 ──────────────────────────────────────────
        const dpsChartEntities = computed((): DpsChartEntity[] => {
            if (!summary.value) return [];
            const bossId = selectedBossId.value;
            const { startAt, endAt } = summary.value.session;
            return summary.value.players.map((p) => {
                const entity = actorManager.value.entityMap[p.entityId] as EntityActor;
                const damages = (entity?.applyDamages ?? []).filter(
                    (d) => d.TargetId === bossId && d.Damage > 0 && d.At >= startAt && d.At <= endAt,
                );
                return { entityId: p.entityId, displayName: getDisplayName(p.name), damages };
            });
        });

        // ── 時間分布 Timeline ────────────────────────────────────────
        type TimelineSeg = { active: boolean; flex: number; startAt: number; endAt: number };

        const buildTimeline = (session: NonNullable<BossSummary>["session"]): TimelineSeg[] => {
            const { startAt, endAt, totalDuration, invincibleIntervals } = session;
            if (totalDuration <= 0) return [];

            const segs: TimelineSeg[] = [];
            let cursor = startAt;

            for (const iv of invincibleIntervals) {
                const ivStart = Math.max(iv.start, startAt);
                const ivEnd   = Math.min(iv.end === Infinity ? endAt : iv.end, endAt);
                if (ivStart >= ivEnd) continue;

                // 無敵前的活躍段
                if (cursor < ivStart) {
                    segs.push({ active: true,  flex: ivStart - cursor, startAt: cursor, endAt: ivStart });
                }
                // 無敵段
                segs.push({ active: false, flex: ivEnd - ivStart, startAt: ivStart, endAt: ivEnd });
                cursor = ivEnd;
            }

            // 最後的活躍段
            if (cursor < endAt) {
                segs.push({ active: true, flex: endAt - cursor, startAt: cursor, endAt: endAt });
            }

            return segs;
        };

        return {
            selectedBossId,
            selectedTimeRange,
            quickRaceFilter,
            bossOptions,
            quickBossOptions,
            quickSelectBoss,
            summary,
            conditionMatrices,
            buildTimeline,
            fmtTime,
            fmtDuration,
            fmtPct,
            fmtDps,
            skillStatHeaders,
            skillSeqHeaders,
            bossCondCoverage,
            bossCondCollapsed,
            bossCondSettingsOpen,
            bossCondIncludeIds,
            bossCondOptions,
            bossCondFilteredCoverage,
            bossCondTab,
            bossCondXMode,
            bossCondGanttData,
            bossSkillHeaders,
            bossSkillCollapsed,
            skillNameMap,
            raceNameMap,
            condNameMap,
            itemNameMap,
            region,
            // 自訂條件
            customConditionConfigs,
            removeCustomCondition,
            reorderCustomCondition,
            describeFilter,
            conditionConfigOpen,
            editDialogOpen,
            isNewCondition,
            editingConfig,
            filterModeOptions,
            conditionOptions,
            conditionFilter,
            startAddCondition,
            startEditCondition,
            saveEditingCondition,
            handleExport,
            handleImport,
            // 改名
            renamePlayers,
            getDisplayName,
            dpsChartEntities,
            computeSkillRatios,
            // 欄位可見性
            PLAYER_COL_DEFS,
            SKILL_COL_DEFS,
            visiblePlayerCols,
            visibleSkillCols,
            colSettingsOpen,
            togglePlayerCol,
            toggleSkillCol,
            secCollapsed,
        };
    },
});
</script>

<style scoped>
.timeline-seg-active {
    cursor: pointer;
    transition: filter 0.1s;
}
.timeline-seg-active:hover {
    filter: brightness(1.3);
}
.timeline-seg-selected {
    outline: 2px solid white;
    outline-offset: -2px;
    filter: brightness(1.4);
}
</style>
