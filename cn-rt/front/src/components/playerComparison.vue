<template>
    <v-container fluid class="pa-2">

        <!-- ── Mode toggle ─────────────────────────────────────────── -->
        <v-row dense class="mb-3">
            <v-col class="d-flex align-center" style="gap: 8px;">
                <v-btn-toggle v-model="mode" mandatory density="compact" variant="outlined">
                    <v-btn value="multi-fight"  size="small" prepend-icon="mdi-account-clock">多場 × 單人</v-btn>
                    <v-btn value="multi-player" size="small" prepend-icon="mdi-account-multiple">單場 × 多人</v-btn>
                </v-btn-toggle>
                <v-spacer />
                <PlayerRenameDialog :players="renamePlayers" />
            </v-col>
        </v-row>

        <!-- ══════════════════════════════════════════════════════════ -->
        <!-- Mode A: 多場 × 單人                                       -->
        <!-- ══════════════════════════════════════════════════════════ -->
        <template v-if="mode === 'multi-fight'">

            <!-- 選擇器 -->
            <v-row dense class="mb-2">
                <v-col cols="12" sm="4">
                    <v-autocomplete
                        v-model="selectedPlayerName"
                        :items="playerOptions"
                        item-title="label"
                        item-value="name"
                        label="選擇玩家"
                        density="compact"
                        hide-details
                        clearable
                        auto-select-first
                    />
                </v-col>
                <v-col cols="12" sm="4">
                    <v-autocomplete
                        v-model="selectedRaceId"
                        :items="raceOptions"
                        item-title="label"
                        item-value="id"
                        label="選擇 Boss"
                        density="compact"
                        hide-details
                        clearable
                        auto-select-first
                        :disabled="!selectedPlayerName"
                    />
                </v-col>
                <v-col cols="12" sm="4" class="d-flex align-center flex-wrap" style="gap: 6px;">
                    <v-btn
                        v-for="q in quickBossOptions"
                        :key="q.raceId"
                        size="small"
                        :variant="selectedRaceId === q.raceId ? 'flat' : 'tonal'"
                        :color="selectedRaceId === q.raceId ? 'primary' : undefined"
                        :disabled="!selectedPlayerName || !q.available"
                        @click="selectedRaceId = selectedRaceId === q.raceId ? null : q.raceId"
                    >{{ q.name }}</v-btn>
                </v-col>
            </v-row>

            <!-- 提示 -->
            <v-row v-if="!selectedPlayerName || selectedRaceId == null" dense>
                <v-col>
                    <v-alert type="info" variant="tonal" density="compact" icon="mdi-cursor-pointer">
                        請選擇玩家與 Boss 以顯示多場比較
                    </v-alert>
                </v-col>
            </v-row>
            <v-row v-else-if="overviewItems.length === 0" dense>
                <v-col>
                    <v-alert type="warning" variant="tonal" density="compact">無符合條件的場次紀錄</v-alert>
                </v-col>
            </v-row>

            <template v-else>
                <!-- Table 1: 場次總覽 -->
                <v-row dense class="mb-2">
                    <v-col cols="12">
                        <v-card variant="outlined">
                            <v-card-title
                                class="text-subtitle-1 py-2 px-3 d-flex align-center"
                                style="cursor: pointer; user-select: none"
                                @click="collapsedModeAOverview = !collapsedModeAOverview"
                            >
                                場次總覽
                                <v-spacer />
                                <v-icon size="small" class="text-disabled">{{ collapsedModeAOverview ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-card-title>
                            <v-expand-transition>
                            <div v-if="!collapsedModeAOverview">
                            <v-divider />
                            <v-card-text class="pa-0">
                                <v-data-table
                                    :headers="overviewHeaders"
                                    :items="overviewItems"
                                    density="compact"
                                    :items-per-page="-1"
                                    hide-default-footer
                                    :sort-by="[{ key: 'startAt', order: 'desc' }]"
                                >
                                    <template #item.startAt="{ item }">
                                        <span class="text-caption">{{ fmtDateTime(item.startAt) }}</span>
                                        <span class="text-caption text-disabled ml-1">～{{ fmtTime(item.endAt) }}</span>
                                    </template>
                                    <template #item.jobName="{ item }">
                                        <span v-if="item.jobName" class="text-primary">{{ item.jobName }}</span>
                                        <span v-else class="text-disabled">—</span>
                                        <div class="text-caption text-disabled">{{ raceNameMap[item.player.raceId] ?? `Race ${item.player.raceId}` }}</div>
                                    </template>
                                    <template #item.totalDamage="{ item }">
                                        {{ Math.round(item.totalDamage).toLocaleString() }}
                                    </template>
                                    <template #item.partyRatio="{ item }">
                                        {{ fmtPct(item.totalPartyDamage > 0 ? item.totalDamage / item.totalPartyDamage : 0) }}
                                    </template>
                                    <template #item.activeDamage="{ item }">
                                        <div>{{ Math.round(item.totalDamage * item.player.activeDamageRatio).toLocaleString() }}</div>
                                        <div class="text-caption text-disabled">{{ fmtPct(item.player.activeDamageRatio) }}</div>
                                    </template>
                                    <template #item.passiveDamage="{ item }">
                                        <div>{{ Math.round(item.totalDamage * item.player.passiveDamageRatio).toLocaleString() }}</div>
                                        <div class="text-caption text-disabled">{{ fmtPct(item.player.passiveDamageRatio) }}</div>
                                    </template>
                                    <template #item.critRate="{ item }">{{ fmtPct(item.player.critRate) }}</template>
                                    <template #item.totalDPS="{ item }">{{ fmtDps(item.totalDPS) }}</template>
                                    <template #item.effectiveDPS="{ item }">{{ fmtDps(item.effectiveDPS) }}</template>
                                    <template #item.individualEffectiveDPS="{ item }">
                                        <div>{{ fmtDps(item.individualEffectiveDPS) }}</div>
                                        <div class="text-caption text-disabled">
                                            {{ fmtDuration(item.player.individualEffectiveTime) }}
                                            ({{ fmtPct(item.session.effectiveDuration > 0 ? item.player.individualEffectiveTime / item.session.effectiveDuration : 0) }})
                                        </div>
                                    </template>
                                </v-data-table>
                            </v-card-text>
                            </div>
                            </v-expand-transition>
                        </v-card>
                    </v-col>
                </v-row>

                <!-- Table 2: 技能明細（依職業分組） -->
                <v-row v-for="(group, gi) in fightsByJob" :key="group.job" dense class="mb-2">
                    <v-col cols="12">
                        <v-card variant="outlined">
                            <v-card-title
                                class="text-subtitle-1 py-2 px-3 d-flex align-center"
                                style="cursor: pointer; user-select: none"
                                @click="toggleSkillDetailCollapsed(group.job)"
                            >
                                技能明細
                                <span class="text-primary ml-2">{{ group.job }}</span>
                                <span class="text-caption text-disabled ml-2">{{ group.fights.length }} 場</span>
                                <v-spacer />
                                <v-icon size="small" class="text-disabled">{{ collapsedSkillDetail[group.job] ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-card-title>
                            <v-expand-transition>
                            <div v-if="!collapsedSkillDetail[group.job]">
                            <v-divider />
                            <!-- 篩選（僅第一組顯示，避免重複） -->
                            <div v-if="gi === 0" class="d-flex flex-wrap align-center px-3 py-1 skill-filter-bar">
                                <v-checkbox v-model="filterLowRatio" label="略過傷害佔比 <0.2% 的技能" density="compact" hide-details class="flex-grow-0" />
                                <v-checkbox v-model="showActive"     label="顯示主動技能" density="compact" hide-details class="flex-grow-0 ml-3" />
                                <v-checkbox v-model="showPassive"    label="顯示被動技能" density="compact" hide-details class="flex-grow-0 ml-3" />
                            </div>
                            <v-divider v-if="gi === 0" />
                            <v-card-text class="pa-0">
                                <div style="overflow-x: auto">
                                    <table class="skill-cmp-table">
                                        <thead>
                                            <tr>
                                                <th class="cell-skill-name">技能</th>
                                                <th class="cell-metric-label">指標</th>
                                                <th v-for="fight in group.fights" :key="fight.entityKey" class="cell-fight-col">
                                                    {{ fmtDateTime(fight.startAt) }}
                                                </th>
                                                <th class="cell-fight-col cell-avg">平均</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <template v-for="(skillId, si) in group.skillIds" :key="skillId">
                                                <tr
                                                    v-for="(metric, mi) in (group.passiveSkillIds.has(skillId) ? SKILL_METRICS_PASSIVE : SKILL_METRICS)"
                                                    :key="mi"
                                                    :class="{ 'row-skill-sep': si > 0 && mi === 0 }"
                                                >
                                                    <td v-if="mi === 0" :rowspan="(group.passiveSkillIds.has(skillId) ? SKILL_METRICS_PASSIVE : SKILL_METRICS).length" class="cell-skill-name cell-sticky">
                                                        {{ skillNameMap[skillId] ?? `#${skillId}` }}
                                                    </td>
                                                    <td class="cell-metric-label">{{ metric.label }}</td>
                                                    <template v-for="fight in group.fights" :key="fight.entityKey">
                                                        <template v-for="s in [fight.player.skillStats.find(x => x.skillId === skillId)]" :key="0">
                                                            <td class="cell-val">
                                                                <template v-if="metric.key === 'ratio'">{{ s ? fmtPct(fight.player.totalDamage > 0 ? s.totalDamage / fight.player.totalDamage : 0) : '—' }}</template>
                                                                <template v-else-if="metric.key === 'totalDamage'">{{ s ? Math.round(s.totalDamage).toLocaleString() : '—' }}</template>
                                                                <template v-else-if="metric.key === 'useCount'">{{ s ? s.totalHits : '—' }}</template>
                                                                <template v-else-if="metric.key === 'damagePerUse'">{{ s ? Math.round(s.damagePerUse).toLocaleString() : '—' }}</template>
                                                                <template v-else-if="metric.key === 'critRate'">
                                                                    <span v-if="!s || s.noCritRate" class="text-disabled">—</span>
                                                                    <span v-else>{{ fmtPct(s.critRate) }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'critDmg'">
                                                                    <span v-if="!s || s.maxCritDamage == null" class="text-disabled">—</span>
                                                                    <span v-else>{{ Math.round(s.minCritDamage!).toLocaleString() }}<span class="text-disabled"> ~ </span>{{ Math.round(s.maxCritDamage).toLocaleString() }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'nonCritDmg'">
                                                                    <span v-if="!s || s.maxNonCritDamage == null" class="text-disabled">—</span>
                                                                    <span v-else>{{ Math.round(s.minNonCritDamage!).toLocaleString() }}<span class="text-disabled"> ~ </span>{{ Math.round(s.maxNonCritDamage).toLocaleString() }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'triggerSources'">
                                                                    <div v-if="s && s.triggerSources.length" style="line-height: 1.6">
                                                                        <div v-for="src in s.triggerSources" :key="src.skillId" class="d-flex" style="white-space: nowrap; gap: 6px;">
                                                                            <span style="flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis;">{{ skillNameMap[src.skillId] ?? `#${src.skillId}` }}</span>
                                                                            <span class="text-disabled">{{ src.count }}次</span>
                                                                            <span class="text-disabled">{{ fmtPct(s.totalDamage > 0 ? src.damage / s.totalDamage : 0) }}</span>
                                                                        </div>
                                                                    </div>
                                                                    <span v-else class="text-disabled">—</span>
                                                                </template>
                                                            </td>
                                                        </template>
                                                    </template>
                                                    <!-- 平均欄 -->
                                                    <td v-if="metric.key === 'triggerSources'" class="cell-val cell-avg text-disabled">—</td>
                                                    <template v-else v-for="avg in [calcAvg(group.fights.map(f => ({ totalDamage: f.player.totalDamage, skillStats: f.player.skillStats })), skillId, metric.key)]" :key="0">
                                                        <td class="cell-val cell-avg">
                                                            <span v-if="avg === null" class="text-disabled">—</span>
                                                            <span v-else>{{ avg }}</span>
                                                        </td>
                                                    </template>
                                                </tr>
                                            </template>
                                        </tbody>
                                    </table>
                                </div>
                            </v-card-text>
                            </div>
                            </v-expand-transition>
                        </v-card>
                    </v-col>
                </v-row>
            </template>

        </template>

        <!-- ══════════════════════════════════════════════════════════ -->
        <!-- Mode B: 單場 × 多人                                       -->
        <!-- ══════════════════════════════════════════════════════════ -->
        <template v-else>

            <!-- 選擇器 -->
            <v-row dense class="mb-2">
                <v-col cols="12" sm="5">
                    <v-autocomplete
                        v-model="selectedFightKey"
                        :items="fightOptions"
                        item-title="label"
                        item-value="key"
                        label="選擇場次"
                        density="compact"
                        hide-details
                        clearable
                        auto-select-first
                    />
                </v-col>
                <v-col cols="12" sm="3">
                    <v-autocomplete
                        v-model="selectedJob"
                        :items="jobOptions"
                        label="選擇職業"
                        density="compact"
                        hide-details
                        clearable
                        auto-select-first
                        :disabled="!selectedFightKey"
                    />
                </v-col>
                <v-col cols="12" sm="4" class="d-flex align-center flex-wrap" style="gap: 6px;">
                    <v-btn
                        v-for="q in quickBossOptions"
                        :key="q.raceId"
                        size="small"
                        :variant="quickFightRaceId === q.raceId ? 'flat' : 'tonal'"
                        :color="quickFightRaceId === q.raceId ? 'primary' : undefined"
                        :disabled="!q.available"
                        @click="quickSelectFight(q.raceId)"
                    >{{ q.name }}</v-btn>
                </v-col>
            </v-row>

            <!-- 提示 -->
            <v-row v-if="!selectedFightKey || !selectedJob" dense>
                <v-col>
                    <v-alert type="info" variant="tonal" density="compact" icon="mdi-cursor-pointer">
                        請選擇場次與職業以顯示單場比較
                    </v-alert>
                </v-col>
            </v-row>
            <v-row v-else-if="comparedPlayers.length === 0" dense>
                <v-col>
                    <v-alert type="warning" variant="tonal" density="compact">此場次中無該職業的玩家</v-alert>
                </v-col>
            </v-row>

            <template v-else>
                <!-- Table 1: 玩家總覽 -->
                <v-row dense class="mb-2">
                    <v-col cols="12">
                        <v-card variant="outlined">
                            <v-card-title
                                class="text-subtitle-1 py-2 px-3 d-flex align-center"
                                style="cursor: pointer; user-select: none"
                                @click="collapsedModeBOverview = !collapsedModeBOverview"
                            >
                                玩家總覽
                                <span class="text-primary ml-2">{{ selectedJob }}</span>
                                <v-spacer />
                                <v-icon size="small" class="text-disabled">{{ collapsedModeBOverview ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-card-title>
                            <v-expand-transition>
                            <div v-if="!collapsedModeBOverview">
                            <v-divider />
                            <v-card-text class="pa-0">
                                <v-data-table
                                    :headers="modeBOverviewHeaders"
                                    :items="modeBOverviewItems"
                                    density="compact"
                                    :items-per-page="-1"
                                    hide-default-footer
                                    :sort-by="[{ key: 'totalDamage', order: 'desc' }]"
                                >
                                    <template #item.name="{ item }">
                                        <span class="font-weight-medium">{{ getDisplayName(item.name) }}</span>
                                        <div class="text-caption text-disabled">{{ raceNameMap[item.raceId] ?? `Race ${item.raceId}` }}</div>
                                    </template>
                                    <template #item.totalDamage="{ item }">
                                        {{ Math.round(item.totalDamage).toLocaleString() }}
                                    </template>
                                    <template #item.partyRatio="{ item }">
                                        {{ fmtPct(item.totalPartyDamage > 0 ? item.totalDamage / item.totalPartyDamage : 0) }}
                                    </template>
                                    <template #item.activeDamage="{ item }">
                                        <div>{{ Math.round(item.totalDamage * item.activeDamageRatio).toLocaleString() }}</div>
                                        <div class="text-caption text-disabled">{{ fmtPct(item.activeDamageRatio) }}</div>
                                    </template>
                                    <template #item.passiveDamage="{ item }">
                                        <div>{{ Math.round(item.totalDamage * item.passiveDamageRatio).toLocaleString() }}</div>
                                        <div class="text-caption text-disabled">{{ fmtPct(item.passiveDamageRatio) }}</div>
                                    </template>
                                    <template #item.critRate="{ item }">{{ fmtPct(item.critRate) }}</template>
                                    <template #item.totalDPS="{ item }">{{ fmtDps(item.totalDPS) }}</template>
                                    <template #item.effectiveDPS="{ item }">{{ fmtDps(item.effectiveDPS) }}</template>
                                    <template #item.individualEffectiveDPS="{ item }">
                                        <div>{{ fmtDps(item.individualEffectiveDPS) }}</div>
                                        <div class="text-caption text-disabled">
                                            {{ fmtDuration(item.individualEffectiveTime) }}
                                            ({{ fmtPct(modeBSession && modeBSession.effectiveDuration > 0 ? item.individualEffectiveTime / modeBSession.effectiveDuration : 0) }})
                                        </div>
                                    </template>
                                </v-data-table>
                            </v-card-text>
                            </div>
                            </v-expand-transition>
                        </v-card>
                    </v-col>
                </v-row>

                <!-- Table 2: 技能明細 -->
                <v-row dense class="mb-2">
                    <v-col cols="12">
                        <v-card variant="outlined">
                            <v-card-title
                                class="text-subtitle-1 py-2 px-3 d-flex align-center"
                                style="cursor: pointer; user-select: none"
                                @click="collapsedModeBSkill = !collapsedModeBSkill"
                            >
                                技能明細
                                <span class="text-primary ml-2">{{ selectedJob }}</span>
                                <v-spacer />
                                <v-icon size="small" class="text-disabled">{{ collapsedModeBSkill ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                            </v-card-title>
                            <v-expand-transition>
                            <div v-if="!collapsedModeBSkill">
                            <v-divider />
                            <div class="d-flex flex-wrap align-center px-3 py-1 skill-filter-bar">
                                <v-checkbox v-model="filterLowRatio" label="略過傷害佔比 <0.2% 的技能" density="compact" hide-details class="flex-grow-0" />
                                <v-checkbox v-model="showActive"     label="顯示主動技能" density="compact" hide-details class="flex-grow-0 ml-3" />
                                <v-checkbox v-model="showPassive"    label="顯示被動技能" density="compact" hide-details class="flex-grow-0 ml-3" />
                            </div>
                            <v-divider />
                            <v-card-text class="pa-0">
                                <div style="overflow-x: auto">
                                    <table class="skill-cmp-table">
                                        <thead>
                                            <tr>
                                                <th class="cell-skill-name">技能</th>
                                                <th class="cell-metric-label">指標</th>
                                                <th v-for="p in comparedPlayers" :key="p.entityId" class="cell-fight-col">
                                                    {{ getDisplayName(p.name) }}
                                                </th>
                                                <th class="cell-fight-col cell-avg">平均</th>
                                            </tr>
                                        </thead>
                                        <tbody>
                                            <template v-for="(skillId, si) in modeBSkillIds" :key="skillId">
                                                <tr
                                                    v-for="(metric, mi) in (modeBPassiveSkillIds.has(skillId) ? SKILL_METRICS_PASSIVE : SKILL_METRICS)"
                                                    :key="mi"
                                                    :class="{ 'row-skill-sep': si > 0 && mi === 0 }"
                                                >
                                                    <td v-if="mi === 0" :rowspan="(modeBPassiveSkillIds.has(skillId) ? SKILL_METRICS_PASSIVE : SKILL_METRICS).length" class="cell-skill-name cell-sticky">
                                                        {{ skillNameMap[skillId] ?? `#${skillId}` }}
                                                    </td>
                                                    <td class="cell-metric-label">{{ metric.label }}</td>
                                                    <template v-for="p in comparedPlayers" :key="p.entityId">
                                                        <template v-for="s in [p.skillStats.find(x => x.skillId === skillId)]" :key="0">
                                                            <td class="cell-val">
                                                                <template v-if="metric.key === 'ratio'">{{ s ? fmtPct(p.totalDamage > 0 ? s.totalDamage / p.totalDamage : 0) : '—' }}</template>
                                                                <template v-else-if="metric.key === 'totalDamage'">{{ s ? Math.round(s.totalDamage).toLocaleString() : '—' }}</template>
                                                                <template v-else-if="metric.key === 'useCount'">{{ s ? s.totalHits : '—' }}</template>
                                                                <template v-else-if="metric.key === 'damagePerUse'">{{ s ? Math.round(s.damagePerUse).toLocaleString() : '—' }}</template>
                                                                <template v-else-if="metric.key === 'critRate'">
                                                                    <span v-if="!s || s.noCritRate" class="text-disabled">—</span>
                                                                    <span v-else>{{ fmtPct(s.critRate) }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'critDmg'">
                                                                    <span v-if="!s || s.maxCritDamage == null" class="text-disabled">—</span>
                                                                    <span v-else>{{ Math.round(s.minCritDamage!).toLocaleString() }}<span class="text-disabled"> ~ </span>{{ Math.round(s.maxCritDamage).toLocaleString() }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'nonCritDmg'">
                                                                    <span v-if="!s || s.maxNonCritDamage == null" class="text-disabled">—</span>
                                                                    <span v-else>{{ Math.round(s.minNonCritDamage!).toLocaleString() }}<span class="text-disabled"> ~ </span>{{ Math.round(s.maxNonCritDamage).toLocaleString() }}</span>
                                                                </template>
                                                                <template v-else-if="metric.key === 'triggerSources'">
                                                                    <div v-if="s && s.triggerSources.length" style="line-height: 1.6">
                                                                        <div v-for="src in s.triggerSources" :key="src.skillId" class="d-flex" style="white-space: nowrap; gap: 6px;">
                                                                            <span style="flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis;">{{ skillNameMap[src.skillId] ?? `#${src.skillId}` }}</span>
                                                                            <span class="text-disabled">{{ src.count }}次</span>
                                                                            <span class="text-disabled">{{ fmtPct(s.totalDamage > 0 ? src.damage / s.totalDamage : 0) }}</span>
                                                                        </div>
                                                                    </div>
                                                                    <span v-else class="text-disabled">—</span>
                                                                </template>
                                                            </td>
                                                        </template>
                                                    </template>
                                                    <!-- 平均欄 -->
                                                    <td v-if="metric.key === 'triggerSources'" class="cell-val cell-avg text-disabled">—</td>
                                                    <template v-else v-for="avg in [calcAvg(comparedPlayers.map(p => ({ totalDamage: p.totalDamage, skillStats: p.skillStats })), skillId, metric.key)]" :key="0">
                                                        <td class="cell-val cell-avg">
                                                            <span v-if="avg === null" class="text-disabled">—</span>
                                                            <span v-else>{{ avg }}</span>
                                                        </td>
                                                    </template>
                                                </tr>
                                            </template>
                                        </tbody>
                                    </table>
                                </div>
                            </v-card-text>
                            </div>
                            </v-expand-transition>
                        </v-card>
                    </v-col>
                </v-row>
            </template>

        </template>

    </v-container>
</template>

<script lang="ts">
import { defineComponent, inject, computed, ref, onMounted, onUnmounted, type Ref } from "vue";
import { ActorManager, EntityActor } from "@/eventActor";
import { buildBossSummary, type BossSummary, type PlayerSummary, type SkillStat } from "@/summaryCollector";
import { getDisplayName } from "@/store";
import PlayerRenameDialog from "@/components/subComponents/PlayerRenameDialog.vue";

// ── 技能指標定義 ─────────────────────────────────────────────────────────────
const SKILL_METRICS = [
    { key: "ratio",        label: "比例"       },
    { key: "totalDamage",  label: "總傷害"     },
    { key: "useCount",     label: "次數"       },
    { key: "damagePerUse", label: "每擊均傷"    },
    { key: "critRate",     label: "暴擊%"      },
    { key: "critDmg",      label: "暴擊傷害"   },
    { key: "nonCritDmg",   label: "無暴擊傷害" },
] as const;

/** 被動技能不會爆擊，移除爆擊欄位，改以觸發來源列取代 */
const SKILL_METRICS_PASSIVE = [
    { key: "ratio",          label: "比例"       },
    { key: "totalDamage",    label: "總傷害"     },
    { key: "useCount",       label: "次數"       },
    { key: "damagePerUse",   label: "每擊均傷"     },
    { key: "nonCritDmg",     label: "傷害"     },
    { key: "triggerSources", label: "來源"     },
] as const;

// ── Types ────────────────────────────────────────────────────────────────────
type FightEntry = {
    entityKey: string;
    startAt: number;
    endAt: number;
    session: NonNullable<BossSummary>["session"];
    totalPartyDamage: number;
    player: PlayerSummary;
};

type OverviewItem = FightEntry & {
    jobName: string | null;
    totalDamage: number;
    totalDPS: number;
    effectiveDPS: number;
    individualEffectiveDPS: number;
};

type SkillSource = { totalDamage: number; skillStats: SkillStat[] };

// ─────────────────────────────────────────────────────────────────────────────
export default defineComponent({
    name: "PlayerComparison",
    components: { PlayerRenameDialog },
    setup() {
        const actorManager = inject("actorManager") as Ref<ActorManager>;
        const appEvent     = inject("appEvent")     as Ref<EventTarget>;
        const skillNameMap = inject("skillNameMap") as Ref<Record<number, string>>;
        const raceNameMap  = inject("raceNameMap")  as Ref<Record<number, string>>;

        // ── Mode ────────────────────────────────────────────────────
        const mode = ref<"multi-fight" | "multi-player">("multi-fight");

        // ── 改名（PlayerRenameDialog 所需的玩家清單） ────────────────
        const renamePlayers = computed(() =>
            Object.values(actorManager.value.entityMap)
                .filter((e) => (e as EntityActor).isPC && (e as EntityActor).name)
                .map((e) => ({ entityId: e.id, name: (e as EntityActor).name }))
                .sort((a, b) => a.name.localeCompare(b.name))
        );

        // ── 技能篩選 ─────────────────────────────────────────────
        const filterLowRatio = ref(true);
        const showActive     = ref(true);
        const showPassive    = ref(true);

        // ── 區塊折疊狀態 ─────────────────────────────────────────
        const collapsedModeAOverview  = ref(false);
        const collapsedSkillDetail    = ref<Record<string, boolean>>({});
        const collapsedModeBOverview  = ref(false);
        const collapsedModeBSkill     = ref(false);

        const toggleSkillDetailCollapsed = (job: string) => {
            collapsedSkillDetail.value = {
                ...collapsedSkillDetail.value,
                [job]: !collapsedSkillDetail.value[job],
            };
        };

        // ── 共用：快速 Boss 按鈕 ────────────────────────────────────
        const QUICK_RACE_IDS = [7603, 7602, 7600, 7601] as const;

        const quickBossOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const allBossRaceIds = new Set(
                Object.values(map)
                    .filter((e) => !(e as EntityActor).isPC && !(e as EntityActor).ownerId && (e as EntityActor).takeDamages.length > 0)
                    .map((e) => (e as EntityActor).raceId),
            );
            return QUICK_RACE_IDS.map((raceId) => ({
                raceId,
                name: raceNameMap.value[raceId] ?? `Race ${raceId}`,
                available: allBossRaceIds.has(raceId),
            }));
        });

        // ────────────────────────────────────────────────────────────
        // Mode A：多場 × 單人
        // ────────────────────────────────────────────────────────────
        const selectedPlayerName = ref<string>("");
        const selectedRaceId     = ref<number | null>(null);

        const playerOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const bossKeys = new Set(
                Object.entries(map)
                    .filter(([, e]) => !(e as EntityActor).isPC && !(e as EntityActor).ownerId)
                    .map(([k]) => k),
            );
            const damageByName = new Map<string, number>();
            for (const e of Object.values(map)) {
                const ea = e as EntityActor;
                if (!ea.isPC || !ea.name) continue;
                const total = ea.applyDamages
                    .filter((d) => bossKeys.has(d.TargetId))
                    .reduce((s, d) => s + d.Damage, 0);
                if (total > 0) damageByName.set(ea.name, (damageByName.get(ea.name) ?? 0) + total);
            }
            return [...damageByName.entries()]
                .sort((a, b) => b[1] - a[1])
                .map(([name, total]) => ({ name, label: `${getDisplayName(name)}  ${Math.round(total).toLocaleString()}` }));
        });

        const raceOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const seen = new Map<number, string>();
            if (!selectedPlayerName.value) {
                for (const e of Object.values(map)) {
                    const ea = e as EntityActor;
                    if (ea.isPC || ea.ownerId || !ea.takeDamages.length) continue;
                    if (!seen.has(ea.raceId)) seen.set(ea.raceId, raceNameMap.value[ea.raceId] ?? `Race ${ea.raceId}`);
                }
            } else {
                const playerTargets = new Set<string>();
                for (const e of Object.values(map)) {
                    const ea = e as EntityActor;
                    if (!ea.isPC || ea.name !== selectedPlayerName.value) continue;
                    for (const d of ea.applyDamages) playerTargets.add(d.TargetId);
                }
                for (const [key, e] of Object.entries(map)) {
                    const ea = e as EntityActor;
                    if (ea.isPC || ea.ownerId || !ea.takeDamages.length || !playerTargets.has(key)) continue;
                    if (!seen.has(ea.raceId)) seen.set(ea.raceId, raceNameMap.value[ea.raceId] ?? `Race ${ea.raceId}`);
                }
            }
            return [...seen.entries()].map(([id, label]) => ({ id, label }));
        });

        const fightSummaries = computed((): FightEntry[] => {
            if (!selectedPlayerName.value || selectedRaceId.value == null) return [];
            const results: FightEntry[] = [];
            for (const [key, e] of Object.entries(actorManager.value.entityMap)) {
                const ea = e as EntityActor;
                if (ea.isPC || ea.ownerId || ea.raceId !== selectedRaceId.value || !ea.takeDamages.length) continue;
                const summary = buildBossSummary(key, actorManager.value, []);
                if (!summary) continue;
                const player = summary.players.find((p) => p.name === selectedPlayerName.value);
                if (!player) continue;
                results.push({ entityKey: key, startAt: summary.session.startAt, endAt: summary.session.endAt, session: summary.session, totalPartyDamage: summary.effectiveBossDamage, player });
            }
            return results.sort((a, b) => b.startAt - a.startAt);
        });

        const overviewItems = computed((): OverviewItem[] =>
            fightSummaries.value.map((f) => ({
                ...f,
                jobName:                f.player.jobName,
                totalDamage:            f.player.totalDamage,
                totalDPS:               f.player.totalDPS,
                effectiveDPS:           f.player.effectiveDPS,
                individualEffectiveDPS: f.player.individualEffectiveDPS,
            })),
        );

        const fightsByJob = computed(() => {
            const groups = new Map<string, FightEntry[]>();
            for (const f of fightSummaries.value) {
                const job = f.player.jobName ?? "未知職業";
                if (!groups.has(job)) groups.set(job, []);
                groups.get(job)!.push(f);
            }
            return [...groups.entries()].map(([job, fights]) => {
                const skillTotals   = new Map<number, number>();
                const skillIsPassive = new Map<number, boolean>();
                const totalPlayerDamage = fights.reduce((s, f) => s + f.player.totalDamage, 0);
                for (const f of fights) {
                    for (const s of f.player.skillStats) {
                        skillTotals.set(s.skillId, (skillTotals.get(s.skillId) ?? 0) + s.totalDamage);
                        if (!skillIsPassive.has(s.skillId))
                            skillIsPassive.set(s.skillId, s.triggerSources.length > 0);
                    }
                }
                const skillIds = [...skillTotals.entries()]
                    .filter(([id, dmg]) => {
                        if (filterLowRatio.value && totalPlayerDamage > 0 && dmg / totalPlayerDamage < 0.002) return false;
                        const isPassive = skillIsPassive.get(id) ?? false;
                        if (!showPassive.value && isPassive)  return false;
                        if (!showActive.value  && !isPassive) return false;
                        return true;
                    })
                    .sort((a, b) => b[1] - a[1])
                    .map(([id]) => id);
                const passiveSkillIds = new Set(
                    skillIds.filter((id) => skillIsPassive.get(id) ?? false)
                );
                return { job, fights, skillIds, passiveSkillIds };
            });
        });

        const overviewHeaders = [
            { title: "場次",         key: "startAt",               sortable: true,  align: "start" as const },
            { title: "職業",         key: "jobName",               sortable: true,  align: "start" as const },
            { title: "總傷害",       key: "totalDamage",           sortable: true,  align: "end"   as const },
            { title: "隊伍佔比",     key: "partyRatio",            sortable: false, align: "end"   as const },
            { title: "主動傷害",     key: "activeDamage",          sortable: false, align: "end"   as const },
            { title: "被動傷害",     key: "passiveDamage",         sortable: false, align: "end"   as const },
            { title: "暴擊率",       key: "critRate",              sortable: false, align: "end"   as const },
            { title: "全場 DPS",     key: "totalDPS",              sortable: true,  align: "end"   as const },
            { title: "有效 DPS",     key: "effectiveDPS",          sortable: true,  align: "end"   as const },
            { title: "個人有效 DPS", key: "individualEffectiveDPS",sortable: true,  align: "end"   as const },
        ];

        // ────────────────────────────────────────────────────────────
        // Mode B：單場 × 多人
        // ────────────────────────────────────────────────────────────
        const selectedFightKey = ref<string>("");
        const selectedJob      = ref<string>("");

        // 場次清單（所有 Boss entity）
        const fightOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const opts: { key: string; label: string; lastAt: number }[] = [];
            for (const [key, e] of Object.entries(map)) {
                const ea = e as EntityActor;
                if (ea.isPC || ea.ownerId || !ea.takeDamages.length) continue;
                const name    = raceNameMap.value[ea.raceId] ?? `Race ${ea.raceId}`;
                const firstAt = ea.takeDamages.reduce((m, d) => Math.min(m, d.At), Infinity);
                const lastAt  = ea.takeDamages.reduce((m, d) => Math.max(m, d.At), -Infinity);
                opts.push({ key, lastAt, label: `${name}  ${fmtDateTime(firstAt)} ～ ${fmtTime(lastAt)}` });
            }
            return opts.sort((a, b) => b.lastAt - a.lastAt);
        });

        // 選取場次的 summary
        const fightSummaryB = computed(() => {
            if (!selectedFightKey.value) return null;
            return buildBossSummary(selectedFightKey.value, actorManager.value, []);
        });

        // 此場次有哪些職業（null 排除）
        const jobOptions = computed(() => {
            const jobs = new Set<string>();
            for (const p of fightSummaryB.value?.players ?? [])
                if (p.jobName) jobs.add(p.jobName);
            return [...jobs].sort();
        });

        // 選取職業的玩家（null 已排除）
        const comparedPlayers = computed((): PlayerSummary[] => {
            if (!fightSummaryB.value || !selectedJob.value) return [];
            return fightSummaryB.value.players.filter((p) => p.jobName === selectedJob.value);
        });

        const modeBSession = computed(() => fightSummaryB.value?.session ?? null);

        // Mode B overview items
        const modeBOverviewItems = computed(() => {
            const partyTotal = fightSummaryB.value?.totalDamage ?? 0;
            return comparedPlayers.value.map((p) => ({ ...p, totalPartyDamage: partyTotal }));
        });

        // Mode B 技能欄（依總傷害合計降序）
        const modeBSkillIds = computed(() => {
            const totals        = new Map<number, number>();
            const skillIsPassive = new Map<number, boolean>();
            const totalPlayerDamage = comparedPlayers.value.reduce((s, p) => s + p.totalDamage, 0);
            for (const p of comparedPlayers.value) {
                for (const s of p.skillStats) {
                    totals.set(s.skillId, (totals.get(s.skillId) ?? 0) + s.totalDamage);
                    if (!skillIsPassive.has(s.skillId))
                        skillIsPassive.set(s.skillId, s.triggerSources.length > 0);
                }
            }
            return [...totals.entries()]
                .filter(([id, dmg]) => {
                    if (filterLowRatio.value && totalPlayerDamage > 0 && dmg / totalPlayerDamage < 0.002) return false;
                    const isPassive = skillIsPassive.get(id) ?? false;
                    if (!showPassive.value && isPassive)  return false;
                    if (!showActive.value  && !isPassive) return false;
                    return true;
                })
                .sort((a, b) => b[1] - a[1])
                .map(([id]) => id);
        });

        const modeBPassiveSkillIds = computed((): Set<number> => {
            const ids = new Set<number>();
            for (const p of comparedPlayers.value)
                for (const s of p.skillStats)
                    if (s.triggerSources.length > 0) ids.add(s.skillId);
            return ids;
        });

        // Mode B quick select：選最新一場指定 raceId 的 entity
        const quickFightRaceId = computed(() => {
            if (!selectedFightKey.value) return null;
            return (actorManager.value.entityMap[selectedFightKey.value] as EntityActor)?.raceId ?? null;
        });

        const quickSelectFight = (raceId: number) => {
            if (quickFightRaceId.value === raceId) { selectedFightKey.value = ""; return; }
            const map = actorManager.value.entityMap;
            let best = ""; let bestTime = -Infinity;
            for (const [key, e] of Object.entries(map)) {
                const ea = e as EntityActor;
                if (ea.isPC || ea.ownerId || ea.raceId !== raceId || !ea.takeDamages.length) continue;
                const lastAt = ea.takeDamages.reduce((m, d) => Math.max(m, d.At), -Infinity);
                if (lastAt > bestTime) { bestTime = lastAt; best = key; }
            }
            if (best) selectedFightKey.value = best;
        };

        const modeBOverviewHeaders = [
            { title: "名稱",         key: "name",                   sortable: true,  align: "start" as const },
            { title: "總傷害",       key: "totalDamage",            sortable: true,  align: "end"   as const },
            { title: "隊伍佔比",     key: "partyRatio",             sortable: false, align: "end"   as const },
            { title: "主動傷害",     key: "activeDamage",           sortable: false, align: "end"   as const },
            { title: "被動傷害",     key: "passiveDamage",          sortable: false, align: "end"   as const },
            { title: "暴擊率",       key: "critRate",               sortable: true,  align: "end"   as const },
            { title: "全場 DPS",     key: "totalDPS",               sortable: true,  align: "end"   as const },
            { title: "有效 DPS",     key: "effectiveDPS",           sortable: true,  align: "end"   as const },
            { title: "個人有效 DPS", key: "individualEffectiveDPS", sortable: true,  align: "end"   as const },
        ];

        // ────────────────────────────────────────────────────────────
        // 共用：指標平均計算（Mode A / B 共用 SkillSource 介面）
        // ────────────────────────────────────────────────────────────
        const calcAvg = (sources: SkillSource[], skillId: number, metricKey: string): string | null => {
            const stats = sources.map((src) => src.skillStats.find((x) => x.skillId === skillId)).filter(Boolean) as SkillStat[];
            if (!stats.length) return null;

            if (metricKey === "ratio") {
                const vals = sources
                    .map((src) => { const s = src.skillStats.find((x) => x.skillId === skillId); return s && src.totalDamage > 0 ? s.totalDamage / src.totalDamage : null; })
                    .filter((v): v is number => v !== null);
                return vals.length ? fmtPct(vals.reduce((a, b) => a + b, 0) / vals.length) : null;
            }
            if (metricKey === "totalDamage") {
                const vals = stats.map((s) => s.totalDamage);
                return Math.round(vals.reduce((a, b) => a + b, 0) / vals.length).toLocaleString();
            }

            if (metricKey === "useCount") {
                const vals = stats.map((s) => s.totalHits);
                return (vals.reduce((a, b) => a + b, 0) / vals.length).toFixed(1);
            }
            if (metricKey === "damagePerUse") {
                const vals = stats.map((s) => s.damagePerUse);
                return Math.round(vals.reduce((a, b) => a + b, 0) / vals.length).toLocaleString();
            }
            if (metricKey === "critRate") {
                const eligible = stats.filter((s) => !s.noCritRate);
                if (!eligible.length) return null;
                const vals = eligible.map((s) => s.critRate);
                return fmtPct(vals.reduce((a, b) => a + b, 0) / vals.length);
            }
            if (metricKey === "critDmg") {
                const maxVals = stats.map((s) => s.maxCritDamage).filter((v): v is number => v !== null);
                const minVals = stats.map((s) => s.minCritDamage).filter((v): v is number => v !== null);
                if (!maxVals.length) return null;
                const maxStr = Math.round(maxVals.reduce((a, b) => a + b, 0) / maxVals.length).toLocaleString();
                const minStr = Math.round(minVals.reduce((a, b) => a + b, 0) / minVals.length).toLocaleString();
                return `${minStr} ~ ${maxStr}`;
            }
            if (metricKey === "nonCritDmg") {
                const maxVals = stats.map((s) => s.maxNonCritDamage).filter((v): v is number => v !== null);
                const minVals = stats.map((s) => s.minNonCritDamage).filter((v): v is number => v !== null);
                if (!maxVals.length) return null;
                const maxStr = Math.round(maxVals.reduce((a, b) => a + b, 0) / maxVals.length).toLocaleString();
                const minStr = Math.round(minVals.reduce((a, b) => a + b, 0) / minVals.length).toLocaleString();
                return `${minStr} ~ ${maxStr}`;
            }
            return null;
        };

        // ── 格式化 ──────────────────────────────────────────────────
        const fmtDateTime = (ts: number) =>
            new Date(ts * 1000).toLocaleString("zh-TW", { month: "2-digit", day: "2-digit", hour: "2-digit", minute: "2-digit", hour12: false });

        const fmtTime = (ts: number) =>
            new Date(ts * 1000).toLocaleTimeString("zh-TW", { hour12: false });

        const fmtDuration = (sec: number) => {
            if (!isFinite(sec) || sec <= 0) return "0s";
            if (sec < 60) return `${sec.toFixed(1)}s`;
            const m = Math.floor(sec / 60);
            const s = Math.round(sec % 60).toString().padStart(2, "0");
            return `${m}m ${s}s`;
        };

        const fmtPct = (ratio: number) => (!isFinite(ratio) ? "—" : `${(ratio * 100).toFixed(1)}%`);


        const fmtDps = (dps: number) => {
            if (!isFinite(dps) || dps <= 0) return "—";
            if (dps >= 1_000_000) return `${(dps / 1_000_000).toFixed(2)}M/s`;
            if (dps >= 1_000)     return `${(dps / 1_000).toFixed(1)}k/s`;
            return `${dps.toFixed(0)}/s`;
        };

        const clearSelections = () => {
            selectedPlayerName.value = "";
            selectedRaceId.value     = null;
            selectedFightKey.value   = "";
            selectedJob.value        = "";
        };
        onMounted(()   => appEvent.value.addEventListener("clear", clearSelections));
        onUnmounted(() => appEvent.value.removeEventListener("clear", clearSelections));

        return {
            mode,
            // Mode A
            selectedPlayerName, selectedRaceId,
            playerOptions, raceOptions, overviewItems, overviewHeaders, fightsByJob,
            // Mode B
            selectedFightKey, selectedJob,
            fightOptions, jobOptions, comparedPlayers, modeBSession,
            modeBOverviewItems, modeBOverviewHeaders, modeBSkillIds, modeBPassiveSkillIds,
            quickFightRaceId, quickSelectFight,
            // 共用
            quickBossOptions, skillNameMap, raceNameMap, SKILL_METRICS, calcAvg,
            filterLowRatio, showActive, showPassive, getDisplayName, renamePlayers,
            collapsedModeAOverview, collapsedSkillDetail, toggleSkillDetailCollapsed,
            collapsedModeBOverview, collapsedModeBSkill,
            fmtDateTime, fmtTime, fmtDuration, fmtPct, fmtDps,
            SKILL_METRICS_PASSIVE,
        };
    },
});
</script>

<style scoped>
.skill-cmp-table {
    border-collapse: collapse;
    font-size: 0.75rem;
    white-space: nowrap;
}
.skill-cmp-table th,
.skill-cmp-table td {
    border: 1px solid rgba(128, 128, 128, 0.2);
    padding: 4px 8px;
}
.skill-cmp-table thead th {
    background: rgba(128, 128, 128, 0.08);
    font-weight: 600;
    text-align: center;
}
.cell-sticky {
    position: sticky;
    left: 0;
    z-index: 1;
    background: rgb(var(--v-theme-surface));
}
.cell-skill-name {
    min-width: 100px;
    font-weight: 600;
    text-align: left !important;
    vertical-align: middle;
}
.cell-metric-label {
    min-width: 80px;
    color: rgba(var(--v-theme-on-surface), 0.6);
    font-weight: normal;
    text-align: left;
    background: rgb(var(--v-theme-surface));
}
.cell-fight-col { min-width: 130px; font-size: 0.7rem; }
.cell-val       { text-align: right; min-width: 100px; }
.cell-avg {
    background: rgba(128, 128, 128, 0.08) !important;
    font-weight: 600;
    border-left: 2px solid rgba(128, 128, 128, 0.35) !important;
}
.row-skill-sep td {
    border-top: 2px solid rgba(128, 128, 128, 0.35) !important;
}
.skill-cmp-table tbody tr:hover td {
    background: rgba(128, 128, 128, 0.06);
}
.skill-filter-bar {
    background: rgba(128, 128, 128, 0.04);
    gap: 0;
}
</style>
