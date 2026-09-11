<template>
    <v-container fluid class="pa-2">
        <!-- Entity selector -->
        <v-row dense class="mb-2">
            <v-col cols="12" sm="5">
                <v-autocomplete
                    v-model="selectedEntityId"
                    :items="entityOptions"
                    item-title="label"
                    item-value="id"
                    :custom-filter="
                        (value, query) =>
                            !query ||
                            value.toLowerCase().includes(query.toLowerCase())
                    "
                    label="Select Monster"
                    density="compact"
                    hide-details
                    clearable
                    auto-select-first
                />
            </v-col>
            <v-col cols="12" sm="7" class="d-flex align-center flex-wrap" style="gap:6px;">
                <!-- 全部 -->
                <v-btn
                    size="small"
                    :variant="quickRaceFilter === null ? 'flat' : 'tonal'"
                    :color="quickRaceFilter === null ? 'primary' : undefined"
                    @click="quickSelectRace(null)"
                >全部</v-btn>
                <!-- 各 race 快速按鈕 -->
                <v-btn
                    v-for="q in quickRaceOptions"
                    :key="q.raceId"
                    size="small"
                    :variant="quickRaceFilter === q.raceId ? 'flat' : 'tonal'"
                    :color="quickRaceFilter === q.raceId ? 'primary' : undefined"
                    :disabled="!q.entityKey"
                    @click="quickSelectRace(q.raceId)"
                >
                    {{ q.name }}
                </v-btn>
            </v-col>
        </v-row>

        <!-- 未選擇怪物時的提示 -->
        <v-row v-if="!selectedEntityId" dense class="mb-2">
            <v-col cols="12">
                <v-alert type="info" variant="tonal" density="compact" icon="mdi-cursor-pointer">
                    請從上方選擇怪物 / Boss 以顯示分析資料
                </v-alert>
            </v-col>
        </v-row>

        <v-row v-if="selectedEntityId" dense>
            <!-- Section 1: Skill usage -->
            <v-col cols="12" md="6">
                <v-card variant="outlined">
                    <v-card-title
                        class="text-subtitle-1 py-2 px-3 d-flex align-center"
                        style="cursor: pointer"
                        @click="collapsed.skill = !collapsed.skill"
                    >
                        技能使用紀錄
                        <v-spacer />
                        <v-icon>{{
                            collapsed.skill
                                ? "mdi-chevron-down"
                                : "mdi-chevron-up"
                        }}</v-icon>
                    </v-card-title>
                    <template v-if="!collapsed.skill">
                        <v-divider />
                        <v-data-table
                            :headers="skillHeaders"
                            :items="skillRows"
                            density="compact"
                            :items-per-page="20"
                            no-data-text="No data"
                        >
                            <template #item.skillName="{ item }">
                                <span
                                    :style="{
                                        color: getMabiNameColor(item.skillName),
                                    }"
                                >
                                    {{ item.skillName }}
                                </span>
                            </template>
                            <template #item.firstAt="{ item }">
                                {{ fmtTime(item.firstAt) }}
                            </template>
                            <template #item.lastAt="{ item }">
                                {{ fmtTime(item.lastAt) }}
                            </template>
                        </v-data-table>
                    </template>
                </v-card>
            </v-col>

            <!-- Section 2: HP change timeline -->
            <v-col cols="12" md="6">
                <v-card variant="outlined">
                    <v-card-title
                        class="text-subtitle-1 py-2 px-3 d-flex align-center"
                        style="cursor: pointer"
                        @click="collapsed.hp = !collapsed.hp"
                    >
                        HP 變動紀錄
                        <v-tooltip location="bottom" @click.stop>
                            <template #activator="{ props }">
                                <v-icon
                                    v-bind="props"
                                    size="small"
                                    icon="mdi-information-outline"
                                    class="ml-1"
                                />
                            </template>
                            以傷害事件推算，連續傷害間隔 &lt;{{ gapThreshold }}s
                            視為同一區段；活躍段命中 &lt;{{ minActiveHits }}
                            次視為無變動
                        </v-tooltip>
                        <v-spacer />
                        <!-- Export button -->
                        <v-btn
                            v-if="hpSegments.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-export"
                            class="mr-1"
                            @click.stop="copyTable"
                            >複製</v-btn
                        >
                        <v-btn
                            v-if="hpSegments.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-download"
                            @click.stop="downloadTable"
                            >CSV</v-btn
                        >
                        <v-icon class="ml-1">{{
                            collapsed.hp ? "mdi-chevron-down" : "mdi-chevron-up"
                        }}</v-icon>
                    </v-card-title>
                    <template v-if="!collapsed.hp">
                        <v-divider />

                        <!-- Controls row -->
                        <v-sheet
                            class="pa-2 d-flex flex-wrap align-center"
                            style="gap: 12px"
                        >
                            <!-- 模式切換 -->
                            <v-btn-toggle
                                v-model="segmentMode"
                                mandatory
                                density="compact"
                                rounded="lg"
                                color="primary"
                            >
                                <v-btn value="damage" size="small"
                                    >傷害間隔</v-btn
                                >
                                <v-btn value="damage+invinc" size="small"
                                    >傷害+無敵過濾</v-btn
                                >
                                <v-btn value="invinc" size="small"
                                    >只用無敵狀態</v-btn
                                >
                            </v-btn-toggle>

                            <!-- 傷害間隔模式下才有意義的參數 -->
                            <template v-if="segmentMode !== 'invinc'">
                                <v-text-field
                                    v-model.number="gapThreshold"
                                    type="number"
                                    label="間隔閾值(秒)"
                                    density="compact"
                                    hide-details
                                    min="1"
                                    max="60"
                                    style="max-width: 120px"
                                />
                                <v-text-field
                                    v-model.number="minActiveHits"
                                    type="number"
                                    label="最小命中次數"
                                    density="compact"
                                    hide-details
                                    min="1"
                                    max="20"
                                    style="max-width: 120px"
                                />
                            </template>
                            <v-text-field
                                v-model.number="minDamage"
                                type="number"
                                label="最小傷害門檻"
                                density="compact"
                                hide-details
                                min="0"
                                style="max-width: 120px"
                            />
                            <v-text-field
                                v-if="segmentMode !== 'damage'"
                                v-model.number="invincBridgeGap"
                                type="number"
                                label="無敵橋接(秒)"
                                density="compact"
                                hide-details
                                min="0"
                                max="30"
                                style="max-width: 120px"
                            />
                            <v-text-field
                                v-model.number="minSegmentDuration"
                                type="number"
                                label="最小段時長(秒)"
                                density="compact"
                                hide-details
                                min="0"
                                style="max-width: 120px"
                            />
                            <v-autocomplete
                                v-model="excludedSkillIds"
                                :items="bossReceivedSkillOptions"
                                item-title="name"
                                item-value="id"
                                label="排除技能（不計入活躍）"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 220px; max-width: 400px"
                            />
                        </v-sheet>
                        <v-divider />

                        <!-- Summary bar -->
                        <v-sheet
                            class="pa-2 d-flex flex-wrap"
                            style="gap: 16px; font-size: 0.85em"
                        >
                            <span>
                                <v-icon size="small" color="error"
                                    >mdi-heart-pulse</v-icon
                                >
                                活躍：<strong>{{
                                    fmtDuration(hpSummary.activeTotal)
                                }}</strong>
                                （{{ hpSummary.sessions }} 段）
                            </span>
                            <span>
                                <v-icon size="small" color="grey"
                                    >mdi-heart-outline</v-icon
                                >
                                無變動：<strong>{{
                                    fmtDuration(hpSummary.idleTotal)
                                }}</strong>
                                （{{ hpSummary.idleCount }} 段）
                            </span>
                            <span>
                                總計：<strong>{{
                                    fmtDuration(hpSummary.total)
                                }}</strong>
                            </span>
                        </v-sheet>

                        <!-- Progress bar overview -->
                        <v-sheet v-if="hpSegments.length" class="px-2 pb-1">
                            <div
                                style="
                                    height: 12px;
                                    display: flex;
                                    border-radius: 4px;
                                    overflow: hidden;
                                "
                            >
                                <div
                                    v-for="(seg, i) in hpBarSegments"
                                    :key="i"
                                    :style="{
                                        flex: seg.duration,
                                        background: seg.active
                                            ? '#ef5350'
                                            : '#e0e0e0',
                                    }"
                                />
                            </div>
                            <div
                                class="d-flex justify-space-between"
                                style="font-size: 0.75em; color: grey"
                            >
                                <span>{{ fmtTime(hpSummary.startAt) }}</span>
                                <span>{{ fmtTime(hpSummary.endAt) }}</span>
                            </div>
                            <!-- HP% markers：僅選中單一怪物時顯示 -->
                            <div
                                v-if="selectedEntityId && hpBarMarkers.length"
                                style="
                                    position: relative;
                                    height: 18px;
                                    margin-top: 2px;
                                "
                            >
                                <span
                                    v-for="(m, i) in hpBarMarkers"
                                    :key="i"
                                    :style="{
                                        position: 'absolute',
                                        left: `${m.left}%`,
                                        transform: 'translateX(-50%)',
                                        fontSize: '0.7em',
                                        color:
                                            m.hpPct === 0 ? '#ef5350' : '#aaa',
                                        whiteSpace: 'nowrap',
                                    }"
                                    >{{ m.hpPct }}%</span
                                >
                            </div>
                            <!-- 活躍/無變動比例 -->
                            <div
                                v-if="selectedEntityId"
                                class="d-flex flex-wrap mt-1"
                                style="
                                    gap: 12px;
                                    font-size: 0.75em;
                                    color: #aaa;
                                "
                            >
                                <span>
                                    <span style="color: #ef5350">■</span>
                                    活躍
                                    {{
                                        hpSummary.total > 0
                                            ? (
                                                  (hpSummary.activeTotal /
                                                      hpSummary.total) *
                                                  100
                                              ).toFixed(1)
                                            : 0
                                    }}%
                                </span>
                                <span>
                                    <span style="color: #e0e0e0">■</span>
                                    無變動
                                    {{
                                        hpSummary.total > 0
                                            ? (
                                                  (hpSummary.idleTotal /
                                                      hpSummary.total) *
                                                  100
                                              ).toFixed(1)
                                            : 0
                                    }}%
                                </span>
                                <span>
                                    總時長 {{ fmtDuration(hpSummary.total) }}
                                </span>
                            </div>
                        </v-sheet>
                        <v-divider v-if="hpSegments.length" />

                        <!-- Segment table -->
                        <v-data-table
                            :headers="hpHeaders"
                            :items="hpSegments"
                            density="compact"
                            :items-per-page="20"
                            no-data-text="No data"
                        >
                            <template #item.type="{ item }">
                                <v-chip
                                    :color="item.active ? 'error' : 'default'"
                                    size="x-small"
                                    variant="flat"
                                >
                                    {{ item.active ? "活躍" : "無變動" }}
                                </v-chip>
                            </template>
                            <template #item.startAt="{ item }">
                                {{ fmtTime(item.startAt) }}
                            </template>
                            <template #item.endAt="{ item }">
                                {{ fmtTime(item.endAt) }}
                            </template>
                            <template #item.duration="{ item }">
                                {{ fmtDuration(item.duration) }}
                            </template>
                            <template #item.hits="{ item }">
                                <template v-if="item.active">
                                    <v-btn
                                        size="x-small"
                                        variant="tonal"
                                        density="compact"
                                        @click="openDetail(item)"
                                        >{{ item.hits }}</v-btn
                                    >
                                </template>
                                <template v-else>
                                    <v-btn
                                        v-if="
                                            getBossSkillsDuring(
                                                item.startAt,
                                                item.endAt,
                                            ).length
                                        "
                                        size="x-small"
                                        variant="tonal"
                                        density="compact"
                                        color="warning"
                                        @click="openIdleDetail(item)"
                                        >{{
                                            getBossSkillsDuring(
                                                item.startAt,
                                                item.endAt,
                                            ).length
                                        }}
                                        技能</v-btn
                                    >
                                    <span v-else class="text-grey">—</span>
                                </template>
                            </template>
                        </v-data-table>
                    </template>
                </v-card>
            </v-col>
        </v-row>

        <!-- Section 3: Boss skill sequence -->
        <v-row dense class="mt-2">
            <v-col cols="12">
                <v-card variant="outlined">
                    <v-card-title
                        class="text-subtitle-1 py-2 px-3 d-flex align-center"
                        style="cursor: pointer"
                        @click="collapsed.bossSkill = !collapsed.bossSkill"
                    >
                        BOSS 技能時間軸
                        <span class="text-caption text-grey ml-2">
                            {{
                                bossSkillFiltered.length ===
                                bossSkillSequence.length
                                    ? `（共 ${bossSkillSequence.length} 筆）`
                                    : `（顯示 ${bossSkillFiltered.length} / ${bossSkillSequence.length} 筆）`
                            }}
                        </span>
                        <v-spacer />
                        <v-btn
                            v-if="bossSkillSequence.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-export"
                            class="mr-1"
                            @click.stop="copySkillSequence"
                            >複製</v-btn
                        >
                        <v-btn
                            v-if="bossSkillSequence.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-download"
                            @click.stop="downloadSkillSequence"
                            >CSV</v-btn
                        >
                        <v-icon class="ml-1">{{
                            collapsed.bossSkill
                                ? "mdi-chevron-down"
                                : "mdi-chevron-up"
                        }}</v-icon>
                    </v-card-title>
                    <template v-if="!collapsed.bossSkill">
                        <v-divider />
                        <!-- Filter bar -->
                        <v-sheet
                            class="pa-2 d-flex flex-wrap align-center"
                            style="gap: 10px"
                        >
                            <!-- 時間範圍：兩欄並排，寬度固定 -->
                            <div class="d-flex align-center" style="gap: 6px">
                                <v-text-field
                                    v-model="bossSkillTimeFrom"
                                    type="text"
                                    label="從"
                                    placeholder="HH:MM:SS"
                                    density="compact"
                                    hide-details
                                    clearable
                                    style="width: 120px"
                                />
                                <span class="text-grey">～</span>
                                <v-text-field
                                    v-model="bossSkillTimeTo"
                                    type="text"
                                    label="至"
                                    placeholder="HH:MM:SS"
                                    density="compact"
                                    hide-details
                                    clearable
                                    style="width: 120px"
                                />
                            </div>
                            <v-text-field
                                v-model.number="consecGap"
                                type="number"
                                label="連擊間隔(秒)"
                                density="compact"
                                hide-details
                                min="0"
                                max="10"
                                style="width: 110px"
                            />
                            <v-autocomplete
                                v-model="bossSkillFilterIds"
                                :items="bossSkillSequenceOptions"
                                item-title="name"
                                item-value="id"
                                label="技能篩選"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 200px; max-width: 380px"
                            />
                            <v-btn
                                v-if="
                                    bossSkillTimeFrom ||
                                    bossSkillTimeTo ||
                                    bossSkillFilterIds.length
                                "
                                size="x-small"
                                variant="text"
                                icon="mdi-close"
                                @click="
                                    bossSkillTimeFrom = '';
                                    bossSkillTimeTo = '';
                                    bossSkillFilterIds = [];
                                "
                            />
                        </v-sheet>
                        <v-divider />
                        <v-data-table
                            :headers="bossSkillHeaders"
                            :items="bossSkillFiltered"
                            density="compact"
                            :items-per-page="50"
                            no-data-text="No data"
                        >
                            <template #item.at="{ item }">
                                <span v-if="item.hits > 1" class="text-caption">
                                    {{ fmtTime(item.at) }}<br />
                                    <span class="text-grey"
                                        >～ {{ fmtTime(item.endAt) }}</span
                                    >
                                </span>
                                <span v-else>{{ fmtTime(item.at) }}</span>
                            </template>
                            <template #item.skillName="{ item }">
                                <span
                                    :style="{
                                        color: getMabiNameColor(item.skillName),
                                    }"
                                >
                                    {{ item.skillName }}
                                </span>
                            </template>
                            <template #item.hits="{ item }">
                                <v-chip
                                    v-if="item.hits > 1"
                                    size="x-small"
                                    color="primary"
                                    variant="flat"
                                    >{{ item.hits }} hit</v-chip
                                >
                                <span v-else class="text-grey">1</span>
                            </template>
                            <template #item.targetNames="{ item }">
                                <span v-if="item.targetNames.length === 1">
                                    {{ item.targetNames[0] }}
                                </span>
                                <span
                                    v-else
                                    class="d-flex flex-wrap"
                                    style="gap: 4px"
                                >
                                    <v-chip
                                        v-for="name in item.targetNames"
                                        :key="name"
                                        size="x-small"
                                        variant="tonal"
                                        color="info"
                                        >{{ name }}</v-chip
                                    >
                                </span>
                            </template>
                            <template #item.damage="{ item }">
                                {{ Math.round(item.damage).toLocaleString() }}
                            </template>
                            <template #item.isCrit="{ item }">
                                <v-icon
                                    v-if="item.isCrit"
                                    size="small"
                                    color="warning"
                                    >mdi-star</v-icon
                                >
                                <span v-else>—</span>
                            </template>
                            <template #item.hpPhase="{ item }">
                                <span
                                    :style="{
                                        color:
                                            item.hpPhase === '機制'
                                                ? '#ef5350'
                                                : '#aaa',
                                    }"
                                >
                                    {{ item.hpPhase }}
                                </span>
                            </template>
                        </v-data-table>
                    </template>
                </v-card>
            </v-col>
        </v-row>

        <!-- Section 4: Player damage log -->
        <v-row dense class="mt-2">
            <v-col cols="12">
                <v-card variant="outlined">
                    <v-card-title
                        class="text-subtitle-1 py-2 px-3 d-flex align-center"
                        style="cursor: pointer"
                        @click="collapsed.playerDmg = !collapsed.playerDmg"
                    >
                        玩家對 BOSS 傷害紀錄
                        <span class="text-caption text-grey ml-2">
                            {{
                                playerDmgWithTriggers.length ===
                                playerDamageLog.length
                                    ? `（共 ${playerDamageLog.length} 筆）`
                                    : `（顯示 ${playerDmgWithTriggers.length} / ${playerDamageLog.length} 筆）`
                            }}
                        </span>
                        <v-spacer />
                        <v-btn
                            v-if="playerDamageLog.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-export"
                            class="mr-1"
                            @click.stop="copyPlayerDmg"
                            >複製</v-btn
                        >
                        <v-btn
                            v-if="playerDamageLog.length"
                            size="x-small"
                            variant="tonal"
                            prepend-icon="mdi-download"
                            @click.stop="downloadPlayerDmg"
                            >CSV</v-btn
                        >
                        <v-icon class="ml-1">{{
                            collapsed.playerDmg
                                ? "mdi-chevron-down"
                                : "mdi-chevron-up"
                        }}</v-icon>
                    </v-card-title>
                    <template v-if="!collapsed.playerDmg">
                        <v-divider />
                        <!-- Toolbar -->
                        <v-sheet class="px-3 py-1 d-flex align-center" style="gap:8px;">
                            <v-btn
                                size="small"
                                :variant="showCritMark ? 'flat' : 'outlined'"
                                :color="showCritMark ? 'warning' : undefined"
                                prepend-icon="mdi-star"
                                @click="showCritMark = !showCritMark"
                            >標示爆擊</v-btn>
                        </v-sheet>
                        <v-divider />
                        <!-- Filter bar -->
                        <v-sheet
                            class="pa-2 d-flex flex-wrap align-center"
                            style="gap: 10px"
                        >
                            <!-- 時間範圍：兩欄並排，寬度固定 -->
                            <div class="d-flex align-center" style="gap: 6px">
                                <v-text-field
                                    v-model="playerDmgTimeFrom"
                                    type="text"
                                    label="從"
                                    placeholder="HH:MM:SS"
                                    density="compact"
                                    hide-details
                                    clearable
                                    style="width: 120px"
                                />
                                <span class="text-grey">～</span>
                                <v-text-field
                                    v-model="playerDmgTimeTo"
                                    type="text"
                                    label="至"
                                    placeholder="HH:MM:SS"
                                    density="compact"
                                    hide-details
                                    clearable
                                    style="width: 120px"
                                />
                            </div>
                            <v-checkbox
                                v-model="filterOutPets"
                                label="排除寵物"
                                density="compact"
                                hide-details
                                class="flex-grow-0"
                            />
                            <v-autocomplete
                                v-model="playerDmgFilterPlayers"
                                :items="playerNameOptions"
                                item-title="name"
                                item-value="id"
                                label="玩家篩選"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 180px; max-width: 320px"
                            />
                            <v-autocomplete
                                v-model="playerDmgFilterIds"
                                :items="playerSkillOptions"
                                item-title="name"
                                item-value="id"
                                label="技能篩選"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 200px; max-width: 380px"
                            />
                            <v-autocomplete
                                v-model="playerCCFilter"
                                :items="playerCCOptions"
                                item-value="value"
                                item-title="title"
                                label="玩家狀態篩選"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 180px; max-width: 300px"
                            >
                                <template #chip="{ item, props: cp }">
                                    <v-chip v-bind="cp" size="x-small">
                                        <img
                                            width="16"
                                            height="16"
                                            :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`"
                                            style="
                                                vertical-align: middle;
                                                margin-right: 3px;
                                            "
                                        />
                                        {{
                                            condNameMap[item.raw.value] ??
                                            item.raw.value
                                        }}
                                    </v-chip>
                                </template>
                                <template #item="{ item, props: ip }">
                                    <v-list-item v-bind="ip" :title="undefined">
                                        <div
                                            class="d-flex align-center"
                                            style="gap: 6px"
                                        >
                                            <img
                                                width="20"
                                                height="20"
                                                :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`"
                                                style="border-radius: 2px"
                                            />
                                            <span class="text-caption">{{
                                                item.raw.title
                                            }}</span>
                                        </div>
                                    </v-list-item>
                                </template>
                            </v-autocomplete>
                            <v-autocomplete
                                v-model="bossCCFilter"
                                :items="bossCCOptions"
                                item-value="value"
                                item-title="title"
                                label="BOSS狀態篩選"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 180px; max-width: 300px"
                            >
                                <template #chip="{ item, props: cp }">
                                    <v-chip v-bind="cp" size="x-small">
                                        <img
                                            width="16"
                                            height="16"
                                            :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`"
                                            style="
                                                vertical-align: middle;
                                                margin-right: 3px;
                                            "
                                        />
                                        {{
                                            condNameMap[item.raw.value] ??
                                            item.raw.value
                                        }}
                                    </v-chip>
                                </template>
                                <template #item="{ item, props: ip }">
                                    <v-list-item v-bind="ip" :title="undefined">
                                        <div
                                            class="d-flex align-center"
                                            style="gap: 6px"
                                        >
                                            <img
                                                width="20"
                                                height="20"
                                                :src="`/res/characterconditionimage/${region}/${item.raw.value}/${item.raw.value}.png`"
                                                style="border-radius: 2px"
                                            />
                                            <span class="text-caption">{{
                                                item.raw.title
                                            }}</span>
                                        </div>
                                    </v-list-item>
                                </template>
                            </v-autocomplete>
                            <v-btn
                                size="x-small"
                                variant="tonal"
                                prepend-icon="mdi-tune"
                                @click="ccConfigOpen = true"
                                >欄位設定</v-btn
                            >
                            <v-btn
                                v-if="
                                    playerDmgTimeFrom ||
                                    playerDmgTimeTo ||
                                    playerDmgFilterIds.length ||
                                    playerDmgFilterPlayers.length ||
                                    playerCCFilter.length ||
                                    bossCCFilter.length
                                "
                                size="x-small"
                                variant="text"
                                icon="mdi-close"
                                @click="
                                    playerDmgTimeFrom = '';
                                    playerDmgTimeTo = '';
                                    playerDmgFilterIds = [];
                                    playerDmgFilterPlayers = [];
                                    playerCCFilter = [];
                                    bossCCFilter = [];
                                "
                            />
                        </v-sheet>
                        <v-divider />

                        <!-- 被動技能設定 -->
                        <v-sheet class="px-3 py-2">
                            <div
                                class="d-flex align-center"
                                style="cursor: pointer; gap: 6px"
                                @click="passiveConfigOpen = !passiveConfigOpen"
                            >
                                <v-icon size="small">{{
                                    passiveConfigOpen
                                        ? "mdi-chevron-up"
                                        : "mdi-chevron-down"
                                }}</v-icon>
                                <span class="text-caption"
                                    >被動技能觸發設定</span
                                >
                                <v-chip
                                    v-if="passiveRules.length"
                                    size="x-small"
                                    color="primary"
                                    variant="tonal"
                                    >{{ passiveRules.length }} 條規則</v-chip
                                >
                            </div>
                            <v-expand-transition>
                                <div v-if="passiveConfigOpen" class="mt-2">
                                    <div
                                        v-for="rule in passiveRules"
                                        :key="rule.id"
                                        class="d-flex flex-wrap align-center mb-2"
                                        style="gap: 8px"
                                    >
                                        <v-autocomplete
                                            v-model="rule.skillId"
                                            :items="playerSkillOptions"
                                            item-title="name"
                                            item-value="id"
                                            label="被動技能"
                                            density="compact"
                                            hide-details
                                            :error="rule.skillId === 0"
                                            style="
                                                min-width: 160px;
                                                max-width: 220px;
                                            "
                                        />
                                        <v-tooltip
                                            v-if="rule.skillId === 0"
                                            text="請選擇被動技能，否則規則不會生效"
                                            location="top"
                                        >
                                            <template
                                                #activator="{ props: tp }"
                                            >
                                                <v-icon
                                                    v-bind="tp"
                                                    color="warning"
                                                    size="small"
                                                    >mdi-alert</v-icon
                                                >
                                            </template>
                                        </v-tooltip>
                                        <v-text-field
                                            v-model.number="rule.ratioMin"
                                            type="number"
                                            label="傷害比例下限%"
                                            density="compact"
                                            hide-details
                                            min="0"
                                            style="max-width: 110px"
                                        />
                                        <v-text-field
                                            v-model.number="rule.ratioMax"
                                            type="number"
                                            label="傷害比例上限%"
                                            density="compact"
                                            hide-details
                                            min="0"
                                            style="max-width: 110px"
                                        />
                                        <v-text-field
                                            v-model.number="rule.lookback"
                                            type="number"
                                            label="往前幾秒"
                                            density="compact"
                                            hide-details
                                            min="1"
                                            max="30"
                                            style="max-width: 80px"
                                        />
                                        <v-btn
                                            size="x-small"
                                            icon="mdi-delete"
                                            variant="text"
                                            color="error"
                                            @click="removePassiveRule(rule.id)"
                                        />
                                    </div>
                                    <div
                                        class="d-flex flex-wrap"
                                        style="gap: 8px; margin-top: 4px"
                                    >
                                        <v-btn
                                            size="x-small"
                                            variant="tonal"
                                            prepend-icon="mdi-plus"
                                            @click="addPassiveRule()"
                                            >新增規則</v-btn
                                        >
                                        <v-btn
                                            size="x-small"
                                            variant="tonal"
                                            color="warning"
                                            @click="addPassivePreset('flash')"
                                            >+ 閃焰 (40~49%)</v-btn
                                        >
                                        <v-btn
                                            size="x-small"
                                            variant="tonal"
                                            color="error"
                                            @click="addPassivePreset('burst')"
                                            >+ 爆破 (44~56%)</v-btn
                                        >
                                        <v-btn
                                            size="x-small"
                                            variant="tonal"
                                            color="info"
                                            @click="addPassivePreset('chain')"
                                            >+ 連續攻擊 (190~240%)</v-btn
                                        >
                                    </div>
                                </div>
                            </v-expand-transition>
                        </v-sheet>
                        <v-divider />

                        <v-data-table
                            :headers="playerDmgHeaders"
                            :items="playerDmgWithTriggers"
                            density="compact"
                            :items-per-page="50"
                            no-data-text="No data"
                        >
                            <template #item.at="{ item }">
                                {{ fmtTime24(item.at) }}
                            </template>
                            <template #item.playerName="{ item }">
                                <span
                                    :style="{
                                        color: getMabiNameColor(
                                            item.playerName,
                                        ),
                                    }"
                                >
                                    {{ item.playerName }}
                                </span>
                                <span
                                    v-if="item.petName"
                                    class="text-caption text-grey ml-1"
                                >
                                    (寵: {{ item.petName }})
                                </span>
                            </template>
                            <template #item.skillName="{ item }">
                                <span
                                    class="d-flex align-center flex-wrap"
                                    style="gap: 4px"
                                >
                                    <span
                                        :style="{
                                            color: getMabiNameColor(
                                                item.skillName,
                                            ),
                                        }"
                                        >{{ item.skillName }}</span
                                    >
                                    <template v-if="item.trigger">
                                        <span class="text-grey text-caption"
                                            >from</span
                                        >
                                        <span
                                            :style="{
                                                color: getMabiNameColor(
                                                    item.trigger.skillName,
                                                ),
                                            }"
                                            >{{ item.trigger.skillName }}</span
                                        >
                                        <v-chip
                                            size="x-small"
                                            variant="tonal"
                                            color="secondary"
                                            >{{
                                                item.trigger.ratio.toFixed(1)
                                            }}%</v-chip
                                        >
                                    </template>
                                    <span
                                        v-else-if="item.isPassive"
                                        class="text-grey text-caption"
                                        >（未匹配觸發）</span
                                    >
                                </span>
                            </template>
                            <template #item.damage="{ item }">
                                <span class="d-flex align-center justify-end" style="gap:4px;">
                                    <v-icon
                                        v-if="item.isCrit && showCritMark"
                                        size="x-small"
                                        color="warning"
                                    >mdi-star</v-icon>
                                    <span :style="item.isCrit && showCritMark ? { color: 'rgb(var(--v-theme-warning))' } : {}">
                                        {{ Math.round(item.damage).toLocaleString() }}
                                    </span>
                                </span>
                            </template>
                            <template #item.conditions="{ item }">
                                <div
                                    class="d-flex flex-wrap align-center"
                                    style="gap: 2px; min-width: 24px"
                                >
                                    <img
                                        v-for="ccId in item.conditions.filter(
                                            (id: number) =>
                                                playerVisibleCCIds.includes(id),
                                        )"
                                        :key="ccId"
                                        width="20"
                                        height="20"
                                        :src="`/res/characterconditionimage/${region}/${ccId}/${ccId}.png`"
                                        :title="`${ccId} ${condNameMap[ccId] ?? ''}`"
                                        style="border-radius: 2px"
                                    />
                                    <span
                                        v-if="
                                            !item.conditions.filter(
                                                (id: number) =>
                                                    playerVisibleCCIds.includes(
                                                        id,
                                                    ),
                                            ).length
                                        "
                                        class="text-grey"
                                        >—</span
                                    >
                                </div>
                            </template>
                            <template #item.targetConditions="{ item }">
                                <div
                                    class="d-flex flex-wrap align-center"
                                    style="gap: 2px; min-width: 24px"
                                >
                                    <img
                                        v-for="ccId in item.targetConditions.filter(
                                            (id: number) =>
                                                bossVisibleCCIds.includes(id),
                                        )"
                                        :key="ccId"
                                        width="20"
                                        height="20"
                                        :src="`/res/characterconditionimage/${region}/${ccId}/${ccId}.png`"
                                        :title="`${ccId} ${condNameMap[ccId] ?? ''}`"
                                        style="border-radius: 2px"
                                    />
                                    <span
                                        v-if="
                                            !item.targetConditions.filter(
                                                (id: number) =>
                                                    bossVisibleCCIds.includes(
                                                        id,
                                                    ),
                                            ).length
                                        "
                                        class="text-grey"
                                        >—</span
                                    >
                                </div>
                            </template>
                        </v-data-table>
                    </template>
                </v-card>
            </v-col>
        </v-row>

        <!-- Section 5: 傷害散點圖 -->
        <v-row dense class="mt-2">
            <v-col cols="12">
                <v-card variant="outlined">
                    <v-card-title
                        class="text-subtitle-1 py-2 px-3 d-flex align-center"
                        style="cursor: pointer"
                        @click="collapsed.scatter = !collapsed.scatter"
                    >
                        傷害散點圖
                        <v-spacer />
                        <v-icon>{{ collapsed.scatter ? 'mdi-chevron-down' : 'mdi-chevron-up' }}</v-icon>
                    </v-card-title>
                    <template v-if="!collapsed.scatter">
                        <v-divider />
                        <v-sheet class="pa-2 d-flex flex-wrap align-center" style="gap: 10px">
                            <v-autocomplete
                                v-model="scatterFilterPlayers"
                                :items="playerNameOptions"
                                item-title="name"
                                item-value="id"
                                label="角色"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 200px; max-width: 340px"
                            />
                            <v-autocomplete
                                v-model="scatterFilterSkillIds"
                                :items="scatterSkillOptions"
                                item-title="name"
                                item-value="id"
                                label="技能"
                                density="compact"
                                hide-details
                                multiple
                                chips
                                closable-chips
                                clearable
                                style="min-width: 200px; max-width: 340px"
                            />
                            <v-btn-toggle
                                v-model="scatterCritMode"
                                mandatory
                                density="compact"
                                color="primary"
                                rounded="lg"
                            >
                                <v-btn value="all" size="small">全部</v-btn>
                                <v-btn value="non-crit" size="small">非暴</v-btn>
                                <v-btn value="crit" size="small">暴擊</v-btn>
                            </v-btn-toggle>
                            <v-select
                                v-model="scatterMaWindow"
                                :items="scatterMaOptions"
                                item-title="label"
                                item-value="value"
                                label="移動平均"
                                density="compact"
                                hide-details
                                style="max-width: 110px"
                            />
                        </v-sheet>
                        <v-divider />
                        <div ref="scatterChartDom" style="height: 300px" />
                    </template>
                </v-card>
            </v-col>
        </v-row>

        <!-- CC 欄位設定 dialog -->
        <v-dialog v-model="ccConfigOpen" max-width="680">
            <v-card>
                <v-card-title class="text-subtitle-1 py-2 px-4"
                    >狀態欄位設定</v-card-title
                >
                <v-divider />
                <v-tabs v-model="ccConfigTab" density="compact">
                    <v-tab value="player">玩家</v-tab>
                    <v-tab value="boss">怪物</v-tab>
                </v-tabs>
                <v-divider />
                <v-card-text class="pt-3">
                    <div class="d-flex" style="gap: 8px; height: 320px">
                        <!-- Left：不顯示 -->
                        <v-card
                            variant="outlined"
                            class="d-flex flex-column"
                            style="flex: 1; min-width: 0"
                        >
                            <div
                                class="px-2 pt-1 pb-1 d-flex align-center"
                                style="gap: 4px"
                            >
                                <v-text-field
                                    v-model="ccLeftSearch"
                                    density="compact"
                                    hide-details
                                    clearable
                                    placeholder="搜尋"
                                    prepend-inner-icon="mdi-magnify"
                                    style="flex: 1"
                                />
                            </div>
                            <div
                                class="px-2 pb-1 d-flex align-center justify-space-between"
                                style="gap: 4px; min-height: 28px"
                            >
                                <span class="text-caption text-grey"
                                    >不顯示</span
                                >
                                <div class="d-flex" style="gap: 4px">
                                    <v-btn
                                        size="x-small"
                                        variant="text"
                                        @click="ccSelectAllLeft"
                                        :disabled="!ccLeftFiltered.length"
                                        >全選</v-btn
                                    >
                                    <v-btn
                                        v-if="ccDualLeftSel.length"
                                        size="x-small"
                                        variant="text"
                                        @click="ccDualLeftSel = []"
                                        >清除選取</v-btn
                                    >
                                </div>
                            </div>
                            <v-divider />
                            <div style="overflow-y: auto; flex: 1">
                                <div
                                    v-for="ccId in ccLeftFiltered"
                                    :key="ccId"
                                    class="d-flex align-center px-2 py-1"
                                    style="
                                        gap: 6px;
                                        cursor: pointer;
                                        user-select: none;
                                    "
                                    :style="
                                        ccDualLeftSel.includes(ccId)
                                            ? {
                                                  background:
                                                      'rgba(var(--v-theme-primary),0.15)',
                                              }
                                            : {}
                                    "
                                    @click="ccToggleSel(ccDualLeftSel, ccId)"
                                >
                                    <v-checkbox-btn
                                        :model-value="
                                            ccDualLeftSel.includes(ccId)
                                        "
                                        density="compact"
                                        @click.stop="
                                            ccToggleSel(ccDualLeftSel, ccId)
                                        "
                                    />
                                    <img
                                        width="20"
                                        height="20"
                                        :src="`/res/characterconditionimage/${region}/${ccId}/${ccId}.png`"
                                        style="
                                            border-radius: 2px;
                                            flex-shrink: 0;
                                        "
                                    />
                                    <span class="text-caption">{{
                                        condNameMap[ccId] ?? `CC ${ccId}`
                                    }}</span>
                                    <span class="text-caption text-grey"
                                        >({{ ccId }})</span
                                    >
                                </div>
                                <div
                                    v-if="!ccLeftFiltered.length"
                                    class="text-caption text-grey text-center pa-3"
                                >
                                    {{
                                        ccLeftSearch
                                            ? "無符合結果"
                                            : "（全部顯示中）"
                                    }}
                                </div>
                            </div>
                        </v-card>

                        <!-- Move buttons：← 移到左（隱藏）、→ 移到右（顯示） -->
                        <div
                            class="d-flex flex-column justify-center align-center"
                            style="flex: 0 0 36px; gap: 6px"
                        >
                            <v-btn
                                icon="mdi-chevron-left"
                                size="small"
                                variant="tonal"
                                :disabled="!ccDualRightSel.length"
                                @click="ccMoveToHidden"
                            />
                            <v-btn
                                icon="mdi-chevron-right"
                                size="small"
                                variant="tonal"
                                :disabled="!ccDualLeftSel.length"
                                @click="ccMoveToVisible"
                            />
                        </div>

                        <!-- Right：顯示 -->
                        <v-card
                            variant="outlined"
                            class="d-flex flex-column"
                            style="flex: 1; min-width: 0"
                        >
                            <div
                                class="px-2 pt-1 pb-1 d-flex align-center"
                                style="gap: 4px"
                            >
                                <v-text-field
                                    v-model="ccRightSearch"
                                    density="compact"
                                    hide-details
                                    clearable
                                    placeholder="搜尋"
                                    prepend-inner-icon="mdi-magnify"
                                    style="flex: 1"
                                />
                            </div>
                            <div
                                class="px-2 pb-1 d-flex align-center justify-space-between"
                                style="gap: 4px; min-height: 28px"
                            >
                                <span class="text-caption text-grey">顯示</span>
                                <div class="d-flex" style="gap: 4px">
                                    <v-btn
                                        size="x-small"
                                        variant="text"
                                        @click="ccSelectAllRight"
                                        :disabled="!ccRightFiltered.length"
                                        >全選</v-btn
                                    >
                                    <v-btn
                                        v-if="ccDualRightSel.length"
                                        size="x-small"
                                        variant="text"
                                        @click="ccDualRightSel = []"
                                        >清除選取</v-btn
                                    >
                                    <v-btn
                                        v-if="
                                            (ccConfigTab === 'player'
                                                ? playerHiddenCCs
                                                : bossHiddenCCs
                                            ).length
                                        "
                                        size="x-small"
                                        variant="text"
                                        color="primary"
                                        @click="ccResetVisible"
                                        >重設全顯示</v-btn
                                    >
                                </div>
                            </div>
                            <v-divider />
                            <div style="overflow-y: auto; flex: 1">
                                <div
                                    v-for="ccId in ccRightFiltered"
                                    :key="ccId"
                                    class="d-flex align-center px-2 py-1"
                                    style="
                                        gap: 6px;
                                        cursor: pointer;
                                        user-select: none;
                                    "
                                    :style="
                                        ccDualRightSel.includes(ccId)
                                            ? {
                                                  background:
                                                      'rgba(var(--v-theme-primary),0.15)',
                                              }
                                            : {}
                                    "
                                    @click="ccToggleSel(ccDualRightSel, ccId)"
                                >
                                    <v-checkbox-btn
                                        :model-value="
                                            ccDualRightSel.includes(ccId)
                                        "
                                        density="compact"
                                        @click.stop="
                                            ccToggleSel(ccDualRightSel, ccId)
                                        "
                                    />
                                    <img
                                        width="20"
                                        height="20"
                                        :src="`/res/characterconditionimage/${region}/${ccId}/${ccId}.png`"
                                        style="
                                            border-radius: 2px;
                                            flex-shrink: 0;
                                        "
                                    />
                                    <span class="text-caption">{{
                                        condNameMap[ccId] ?? `CC ${ccId}`
                                    }}</span>
                                    <span class="text-caption text-grey"
                                        >({{ ccId }})</span
                                    >
                                </div>
                                <div
                                    v-if="!ccRightFiltered.length"
                                    class="text-caption text-grey text-center pa-3"
                                >
                                    {{
                                        ccRightSearch
                                            ? "無符合結果"
                                            : "（全部隱藏）"
                                    }}
                                </div>
                            </div>
                        </v-card>
                    </div>
                </v-card-text>
                <v-card-actions>
                    <v-spacer />
                    <v-btn
                        size="small"
                        variant="text"
                        @click="ccConfigOpen = false"
                        >關閉</v-btn
                    >
                </v-card-actions>
            </v-card>
        </v-dialog>

        <!-- Detail dialog：技能命中詳情 -->
        <v-dialog v-model="detailOpen" max-width="520">
            <v-card>
                <v-card-title class="text-subtitle-1">
                    技能命中詳情
                    <span
                        v-if="detailSegment"
                        class="text-caption text-grey ml-2"
                    >
                        {{ fmtTime(detailSegment.startAt) }} ～
                        {{ fmtTime(detailSegment.endAt) }} （{{
                            fmtDuration(detailSegment.duration)
                        }}）
                    </span>
                </v-card-title>
                <v-divider />
                <v-data-table
                    :headers="detailHeaders"
                    :items="detailRows"
                    density="compact"
                    :items-per-page="-1"
                    hide-default-footer
                    no-data-text="No data"
                >
                    <template #item.skillName="{ item }">
                        <span
                            :style="{ color: getMabiNameColor(item.skillName) }"
                        >
                            {{ item.skillName }}
                        </span>
                    </template>
                    <template #item.avgDamage="{ item }">
                        {{ item.avgDamage.toFixed(0) }}
                    </template>
                    <template #item.totalDamage="{ item }">
                        {{ Math.round(item.totalDamage).toLocaleString() }}
                    </template>
                </v-data-table>
                <v-card-actions>
                    <v-spacer />
                    <v-btn variant="text" @click="detailOpen = false"
                        >關閉</v-btn
                    >
                </v-card-actions>
            </v-card>
        </v-dialog>

        <!-- Idle detail dialog：無變動期間 BOSS 技能 -->
        <v-dialog v-model="idleDetailOpen" max-width="560">
            <v-card>
                <v-card-title class="text-subtitle-1">
                    無變動期間 BOSS 技能
                    <span
                        v-if="idleDetailSegment"
                        class="text-caption text-grey ml-2"
                    >
                        {{ fmtTime(idleDetailSegment.startAt) }} ～
                        {{ fmtTime(idleDetailSegment.endAt) }}（{{
                            fmtDuration(idleDetailSegment.duration)
                        }}）
                    </span>
                </v-card-title>
                <v-divider />
                <v-data-table
                    :headers="idleDetailHeaders"
                    :items="idleDetailRows"
                    density="compact"
                    :items-per-page="-1"
                    hide-default-footer
                    no-data-text="No data"
                >
                    <template #item.skillName="{ item }">
                        <span
                            :style="{ color: getMabiNameColor(item.skillName) }"
                        >
                            {{ item.skillName }}
                        </span>
                    </template>
                    <template #item.avgDamage="{ item }">
                        {{ item.avgDamage.toFixed(0) }}
                    </template>
                    <template #item.totalDamage="{ item }">
                        {{ Math.round(item.totalDamage).toLocaleString() }}
                    </template>
                </v-data-table>
                <v-card-actions>
                    <v-spacer />
                    <v-btn variant="text" @click="idleDetailOpen = false"
                        >關閉</v-btn
                    >
                </v-card-actions>
            </v-card>
        </v-dialog>

        <!-- Copy snackbar -->
        <v-snackbar v-model="snackbar" :timeout="2000" location="bottom right">
            已複製到剪貼簿
        </v-snackbar>
    </v-container>
</template>

<script lang="ts">
import { defineComponent, inject, computed, ref, watch, nextTick, onMounted, onUnmounted, type Ref } from "vue";
import { ActorManager, EntityActor } from "@/eventActor";
import { getMabiNameColor, prettyEntityName } from "@/lib/util";
import { getDisplayName } from "@/store";
import highcharts from "highcharts";
import type { Options, SeriesOptionsType } from "highcharts";

type Seg = {
    active: boolean;
    startAt: number;
    endAt: number;
    duration: number;
    hits: number;
    damage: number;
};

export default defineComponent({
    name: "BattleLog",
    setup() {
        const actorManager = inject("actorManager") as Ref<ActorManager>;
        const appEvent    = inject("appEvent") as Ref<EventTarget>;
        const skillNameMap = inject("skillNameMap") as Ref<
            Record<number, string>
        >;
        const raceNameMap = inject("raceNameMap") as Ref<
            Record<number, string>
        >;
        const timeRangeMin = inject("timeRangeMin") as Ref<number | null>;
        const timeRangeMax = inject("timeRangeMax") as Ref<number | null>;
        const region = inject("region") as Ref<string>;
        const condNameMap = inject("condNameMap") as Ref<
            Record<number, string>
        >;

        // ── 各區塊 collapse 狀態 ───────────────────────────────────────
        const collapsed = ref({
            skill: false,
            hp: false,
            bossSkill: false,
            playerDmg: false,
            scatter: true,
        });

        // ── 可調控制項 ─────────────────────────────────────────────────
        // 'damage' | 'damage+invinc' | 'invinc'
        const segmentMode = ref<"damage" | "damage+invinc" | "invinc">(
            "damage",
        );
        const gapThreshold = ref(3); // 秒：超過此間隔分段
        const minActiveHits = ref(3); // 低於此命中次數的活躍段視為無變動
        const minDamage = ref(0); // 低於此傷害的命中不計入活躍判斷
        const invincBridgeGap = ref(5); // 兩段無敵間距 ≤ N 秒視為同一段
        const minSegmentDuration = ref(0); // 活躍段短於此秒數轉為無變動（0=關閉）
        const excludedSkillIds = ref<number[]>([]); // 這些技能的命中不計入活躍判斷
        const excludeInvincible = computed(
            () => segmentMode.value !== "damage",
        );

        const INVINCIBLE_CCIDS = new Set([277, 494]);

        /** 根據 conditionHistory 建構無敵區間列表 */
        const buildInvincibleIntervals = (
            entity: import("@/eventActor").EntityActor,
        ) => {
            const intervals: { start: number; end: number }[] = [];
            let invStart: number | null = null;
            for (const state of entity.conditionHistory) {
                const has = state.List.some((c) =>
                    INVINCIBLE_CCIDS.has(c.CCId),
                );
                if (has && invStart === null) {
                    invStart = state.At;
                } else if (!has && invStart !== null) {
                    intervals.push({ start: invStart, end: state.At });
                    invStart = null;
                }
            }
            if (invStart !== null) {
                intervals.push({ start: invStart, end: Infinity });
            }
            return intervals;
        };

        const isDuringInvincible = (
            at: number,
            intervals: { start: number; end: number }[],
        ) => intervals.some((iv) => at >= iv.start && at <= iv.end);

        /** 橋接間隔 ≤ bridgeGap 秒的相鄰無敵段，補足封包遺失造成的斷點 */
        const bridgeInvincibleIntervals = (
            entity: import("@/eventActor").EntityActor,
            bridgeGap: number,
        ) => {
            const ivs = buildInvincibleIntervals(entity);
            if (ivs.length < 2 || bridgeGap <= 0) return ivs;
            const merged = [{ ...ivs[0] }];
            for (let i = 1; i < ivs.length; i++) {
                const last = merged[merged.length - 1];
                const curr = ivs[i];
                if (curr.start - last.end <= bridgeGap) {
                    last.end = Math.max(last.end, curr.end);
                } else {
                    merged.push({ ...curr });
                }
            }
            return merged;
        };

        const selectedEntityId  = ref<string>("");
        const quickRaceFilter   = ref<number | null>(null);

        const clearSelection = () => { selectedEntityId.value = ""; };
        onMounted(()   => appEvent.value.addEventListener("clear", clearSelection));
        onUnmounted(() => appEvent.value.removeEventListener("clear", clearSelection));

        // ── helpers ────────────────────────────────────────────────────
        const fmtTime = (ts: number | null) => {
            if (ts == null) return "—";
            return new Date(ts * 1000).toLocaleTimeString();
        };

        const fmtTime24 = (ts: number | null) => {
            if (ts == null) return "—";
            return new Date(ts * 1000).toLocaleTimeString('zh-TW', { hour12: false });
        };

        const fmtDuration = (sec: number) => {
            if (sec < 60) return `${sec.toFixed(1)}s`;
            const m = Math.floor(sec / 60);
            const s = (sec % 60).toFixed(0).padStart(2, "0");
            return `${m}m ${s}s`;
        };

        const inRange = (at: number) => {
            if (timeRangeMin.value != null && at < timeRangeMin.value)
                return false;
            if (timeRangeMax.value != null && at > timeRangeMax.value)
                return false;
            return true;
        };

        // BOSS 實際被使用的技能清單（用於排除技能下拉選項）
        const bossReceivedSkillOptions = computed(() => {
            const seen = new Map<number, string>();
            for (const entity of selectedEntities.value) {
                for (const d of entity.takeDamages) {
                    if (!seen.has(d.SkillId)) {
                        seen.set(
                            d.SkillId,
                            skillNameMap.value[d.SkillId] ||
                                `unknown:${d.SkillId}`,
                        );
                    }
                }
            }
            return [...seen.entries()]
                .map(([id, name]) => ({ id, name }))
                .sort((a, b) => a.name.localeCompare(b.name));
        });

        // ── entity selector（怪物限定）─────────────────────────────────
        const ALL_ENTITY_ID = "__ALL__";

        const entityOptions = computed(() => {
            const map = actorManager.value.entityMap;
            const opts: { id: string; label: string }[] = [];
            for (const k in map) {
                const e = map[k];
                if (e.isPC) continue;
                if (e.ownerId) continue;
                if (quickRaceFilter.value !== null && e.raceId !== quickRaceFilter.value) continue;
                const inRangeHits = e.takeDamages.filter((d) => inRange(d.At));
                if (inRangeHits.length < minActiveHits.value) continue;
                const totalDmg = inRangeHits.reduce((s, d) => s + d.Damage, 0);
                const firstAt = inRangeHits.reduce((m, d) => Math.min(m, d.At), Infinity);
                const lastAt  = inRangeHits.reduce((m, d) => Math.max(m, d.At), -Infinity);
                const name = prettyEntityName(e, raceNameMap) ?? `${e.raceId}`;
                opts.push({
                    id: k,
                    label: `${name}  ${fmtTime(firstAt)} ~ ${fmtTime(lastAt)}  (${Math.round(totalDmg).toLocaleString()})`,
                });
            }
            if (opts.length > 1)
                opts.unshift({ id: ALL_ENTITY_ID, label: `（全部怪物，共 ${opts.length} 隻）` });
            return opts;
        });

        const selectedEntities = computed((): EntityActor[] => {
            if (!selectedEntityId.value) return [];
            const map = actorManager.value.entityMap;
            if (selectedEntityId.value === ALL_ENTITY_ID)
                return entityOptions.value
                    .filter(o => o.id !== ALL_ENTITY_ID)
                    .map(o => map[o.id])
                    .filter((e): e is EntityActor => !!e);
            const e = map[selectedEntityId.value];
            return e ? [e] : [];
        });

        // 快速選擇：依 raceId 找到第一個符合的 entityKey
        const QUICK_RACE_IDS = [7603, 7602, 7600, 7601] as const;
        const quickRaceOptions = computed(() => {
            const map = actorManager.value.entityMap;
            return QUICK_RACE_IDS.map(raceId => {
                const entry = Object.entries(map).find(([, e]) => e.raceId === raceId);
                const name = raceNameMap.value[raceId] ?? `Race ${raceId}`;
                return { raceId, name, entityKey: entry?.[0] ?? null };
            });
        });
        const quickSelectRace = (raceId: number | null) => {
            // null = 全部；同一個 race 再按一次也等同切回全部
            quickRaceFilter.value = (raceId !== null && quickRaceFilter.value === raceId) ? null : raceId;
            // filter 改變後，若目前選取的 entity 不在新選項內就清掉
            const map = actorManager.value.entityMap;
            const cur = selectedEntityId.value ? map[selectedEntityId.value] : null;
            if (cur && quickRaceFilter.value !== null && cur.raceId !== quickRaceFilter.value) {
                selectedEntityId.value = "";
            }
        };

        // ── Section 1: Skill usage ──────────────────────────────────────
        const skillHeaders = [
            { title: "技能", key: "skillName", sortable: true },
            { title: "次數", key: "count", sortable: true },
            { title: "首次", key: "firstAt", sortable: false },
            { title: "最後", key: "lastAt", sortable: false },
        ];

        const skillRows = computed(() => {
            const CONSEC = consecGap.value;

            // 收集各技能的命中時間點
            const hitsBySkill: Record<number, number[]> = {};
            for (const entity of selectedEntities.value) {
                for (const d of entity.applyDamages) {
                    if (!inRange(d.At)) continue;
                    if (!hitsBySkill[d.SkillId]) hitsBySkill[d.SkillId] = [];
                    hitsBySkill[d.SkillId].push(d.At);
                }
            }

            // 對每個技能，將連續命中（間隔 ≤ CONSEC 秒）合成一次使用
            return Object.entries(hitsBySkill)
                .map(([sid, times]) => {
                    times.sort((a, b) => a - b);
                    let uses = 1;
                    for (let i = 1; i < times.length; i++) {
                        if (times[i] - times[i - 1] > CONSEC) uses++;
                    }
                    return {
                        skillId: +sid,
                        skillName: skillNameMap.value[+sid] || `unknown:${sid}`,
                        count: uses,
                        firstAt: times[0],
                        lastAt: times[times.length - 1],
                    };
                })
                .sort((a, b) => b.count - a.count);
        });

        // ── Section 2: HP change segments ──────────────────────────────
        /** 回傳 BOSS 在指定時間範圍內使用的技能（含使用次數），供無變動段顯示 */
        const getBossSkillsDuring = (
            startAt: number,
            endAt: number,
        ): { name: string; count: number }[] => {
            const skillCounts = new Map<string, number>();
            for (const entity of selectedEntities.value) {
                for (const d of entity.applyDamages) {
                    if (d.At < startAt || d.At > endAt) continue;
                    const name =
                        skillNameMap.value[d.SkillId] || `unknown:${d.SkillId}`;
                    skillCounts.set(name, (skillCounts.get(name) ?? 0) + 1);
                }
            }
            return [...skillCounts.entries()]
                .map(([name, count]) => ({ name, count }))
                .sort((a, b) => b.count - a.count);
        };
        const hpHeaders = [
            { title: "狀態", key: "type", sortable: false },
            { title: "開始", key: "startAt", sortable: false },
            { title: "結束", key: "endAt", sortable: false },
            { title: "時長", key: "duration", sortable: true },
            { title: "傷害次數", key: "hits", sortable: true },
        ];

        /** 合併相鄰無變動段（共用工具） */
        const mergeAdjacentIdle = (segs: Seg[]): Seg[] => {
            const result: Seg[] = [];
            for (const s of segs) {
                const prev = result[result.length - 1];
                if (prev && !prev.active && !s.active) {
                    prev.endAt = s.endAt;
                    prev.duration = prev.endAt - prev.startAt;
                } else {
                    result.push({ ...s });
                }
            }
            return result;
        };

        const hpSegments = computed((): Seg[] => {
            const MIN_DMG = minDamage.value;
            const mode = segmentMode.value;
            const BRIDGE = invincBridgeGap.value;
            const MIN_DUR = minSegmentDuration.value;
            const EXCL_SKILLS = new Set(excludedSkillIds.value);

            /** 將活躍段短於 MIN_DUR 的轉成無變動並合併 */
            const applyMinDuration = (segs: Seg[]): Seg[] => {
                if (MIN_DUR <= 0) return segs;
                return mergeAdjacentIdle(
                    segs.map((s) =>
                        s.active && s.duration < MIN_DUR
                            ? { ...s, active: false, hits: 0, damage: 0 }
                            : s,
                    ),
                );
            };

            // ── 模式：只用無敵狀態 ────────────────────────────────────────
            if (mode === "invinc") {
                // 收集所有選中 entity 的無敵區間（橋接 + 合併後去重）
                const rawIntervals: { start: number; end: number }[] = [];
                for (const entity of selectedEntities.value) {
                    rawIntervals.push(
                        ...bridgeInvincibleIntervals(
                            entity as import("@/eventActor").EntityActor,
                            BRIDGE,
                        ),
                    );
                }
                if (!rawIntervals.length) return [];

                rawIntervals.sort((a, b) => a.start - b.start);

                // 合併重疊區間
                const merged: { start: number; end: number }[] = [
                    rawIntervals[0],
                ];
                for (let i = 1; i < rawIntervals.length; i++) {
                    const cur = rawIntervals[i];
                    const last = merged[merged.length - 1];
                    if (cur.start <= last.end) {
                        last.end = Math.max(last.end, cur.end);
                    } else {
                        merged.push({ ...cur });
                    }
                }

                // 收集全部命中（用於在活躍段計算 hits/damage）
                const allHits: { At: number; Damage: number }[] = [];
                for (const entity of selectedEntities.value) {
                    for (const d of entity.takeDamages) {
                        if (!inRange(d.At)) continue;
                        if (d.Damage < MIN_DMG) continue;
                        if (EXCL_SKILLS.size && EXCL_SKILLS.has(d.SkillId))
                            continue;
                        allHits.push({ At: d.At, Damage: d.Damage });
                    }
                }
                if (!allHits.length) return [];
                allHits.sort((a, b) => a.At - b.At);

                const timelineStart = allHits[0].At;
                const timelineEnd = allHits[allHits.length - 1].At;

                const segs: Seg[] = [];
                let cursor = timelineStart;
                let hitPtr = 0;

                const countHits = (from: number, to: number) => {
                    let h = 0,
                        dmg = 0;
                    for (let i = hitPtr; i < allHits.length; i++) {
                        if (allHits[i].At > to) break;
                        if (allHits[i].At >= from) {
                            h++;
                            dmg += allHits[i].Damage;
                        }
                    }
                    // advance hitPtr
                    while (hitPtr < allHits.length && allHits[hitPtr].At <= to)
                        hitPtr++;
                    return { h, dmg };
                };

                for (const iv of merged) {
                    const ivStart = Math.max(iv.start, timelineStart);
                    const ivEnd = Math.min(
                        iv.end === Infinity ? timelineEnd : iv.end,
                        timelineEnd,
                    );

                    if (ivStart > cursor) {
                        // 無敵前的活躍段
                        const { h, dmg } = countHits(cursor, ivStart);
                        segs.push({
                            active: true,
                            startAt: cursor,
                            endAt: ivStart,
                            duration: ivStart - cursor,
                            hits: h,
                            damage: dmg,
                        });
                    }
                    if (ivEnd > ivStart) {
                        segs.push({
                            active: false,
                            startAt: ivStart,
                            endAt: ivEnd,
                            duration: ivEnd - ivStart,
                            hits: 0,
                            damage: 0,
                        });
                    }
                    cursor = ivEnd;
                }

                // 最後一段活躍
                if (cursor < timelineEnd) {
                    const { h, dmg } = countHits(cursor, timelineEnd);
                    segs.push({
                        active: true,
                        startAt: cursor,
                        endAt: timelineEnd,
                        duration: timelineEnd - cursor,
                        hits: h,
                        damage: dmg,
                    });
                }

                return applyMinDuration(
                    mergeAdjacentIdle(
                        segs.filter((s) => s.duration > 0 || s.active),
                    ),
                );
            }

            // ── 模式：傷害間隔 / 傷害+無敵過濾 ──────────────────────────
            const GAP = gapThreshold.value;
            const MIN_HITS = minActiveHits.value;
            const useFilter = mode === "damage+invinc";

            const hits: { At: number; Damage: number }[] = [];
            for (const entity of selectedEntities.value) {
                const invIntervals = useFilter
                    ? bridgeInvincibleIntervals(
                          entity as import("@/eventActor").EntityActor,
                          BRIDGE,
                      )
                    : [];
                for (const d of entity.takeDamages) {
                    if (!inRange(d.At)) continue;
                    if (d.Damage < MIN_DMG) continue;
                    if (EXCL_SKILLS.size && EXCL_SKILLS.has(d.SkillId))
                        continue;
                    if (useFilter && isDuringInvincible(d.At, invIntervals))
                        continue;
                    hits.push({ At: d.At, Damage: d.Damage });
                }
            }
            if (hits.length < MIN_HITS) return [];

            hits.sort((a, b) => a.At - b.At);

            const segs: Seg[] = [];
            let sessionStart = hits[0].At;
            let sessionEnd = hits[0].At;
            let sessionHits = 1;
            let sessionDamage = hits[0].Damage;

            const pushActive = (s: number, e: number, h: number, dmg: number) =>
                segs.push({
                    active: true,
                    startAt: s,
                    endAt: e,
                    duration: e - s,
                    hits: h,
                    damage: dmg,
                });
            const pushIdle = (s: number, e: number) =>
                segs.push({
                    active: false,
                    startAt: s,
                    endAt: e,
                    duration: e - s,
                    hits: 0,
                    damage: 0,
                });

            for (let i = 1; i < hits.length; i++) {
                const gap = hits[i].At - hits[i - 1].At;
                if (gap <= GAP) {
                    sessionEnd = hits[i].At;
                    sessionHits++;
                    sessionDamage += hits[i].Damage;
                } else {
                    pushActive(
                        sessionStart,
                        sessionEnd,
                        sessionHits,
                        sessionDamage,
                    );
                    pushIdle(sessionEnd, hits[i].At);
                    sessionStart = hits[i].At;
                    sessionEnd = hits[i].At;
                    sessionHits = 1;
                    sessionDamage = hits[i].Damage;
                }
            }
            pushActive(sessionStart, sessionEnd, sessionHits, sessionDamage);

            // 命中次數不足的活躍段轉為無變動，再合併相鄰無變動段，再過濾最小時長
            const raw = segs.map((s) =>
                s.active && s.hits < MIN_HITS
                    ? { ...s, active: false, hits: 0, damage: 0 }
                    : s,
            );
            return applyMinDuration(mergeAdjacentIdle(raw));
        });

        const hpSummary = computed(() => {
            const segs = hpSegments.value;
            if (!segs.length) {
                return {
                    activeTotal: 0,
                    idleTotal: 0,
                    total: 0,
                    sessions: 0,
                    idleCount: 0,
                    startAt: null as number | null,
                    endAt: null as number | null,
                };
            }
            let activeTotal = 0,
                idleTotal = 0,
                sessions = 0,
                idleCount = 0;
            for (const s of segs) {
                if (s.active) {
                    activeTotal += s.duration;
                    sessions++;
                } else {
                    idleTotal += s.duration;
                    idleCount++;
                }
            }
            return {
                activeTotal,
                idleTotal,
                total: segs[segs.length - 1].endAt - segs[0].startAt,
                sessions,
                idleCount,
                startAt: segs[0].startAt,
                endAt: segs[segs.length - 1].endAt,
            };
        });

        const hpBarSegments = computed(() => {
            const segs = hpSegments.value;
            if (!segs.length) return [];
            const totalDuration = segs.reduce((sum, s) => sum + s.duration, 0);
            const minFlex = totalDuration > 0 ? totalDuration / 100 : 0.5;
            return segs.map((s) => ({
                duration: Math.max(s.duration, minFlex),
                active: s.active,
            }));
        });

        const hpBarMarkers = computed(() => {
            if (!selectedEntityId.value) return [];
            const segs = hpSegments.value;
            if (!segs.length) return [];
            const totalDmg = segs.reduce((s, seg) => s + seg.damage, 0);
            const totalTime = hpSummary.value.total;
            if (totalDmg === 0 || totalTime === 0) return [];
            const startTime = segs[0].startAt;
            let cumDmg = 0;
            const markers: { left: number; hpPct: number }[] = [
                { left: 0, hpPct: 100 },
            ];
            for (const seg of segs) {
                if (!seg.active) continue;
                cumDmg += seg.damage;
                const hpPct = Math.max(
                    0,
                    Math.round(100 - (cumDmg / totalDmg) * 100),
                );
                const left = Math.min(
                    100,
                    ((seg.endAt - startTime) / totalTime) * 100,
                );
                markers.push({ left, hpPct });
            }
            return markers;
        });

        // ── Section 3: Boss skill sequence ─────────────────────────────
        const consecGap = ref(2); // 連擊判定間隔（秒），同技能連續使用在此秒數內視為一組

        const bossSkillHeaders = [
            { title: "時間", key: "at", sortable: false },
            { title: "技能", key: "skillName", sortable: true },
            { title: "連擊", key: "hits", sortable: true },
            { title: "目標", key: "targetNames", sortable: false },
            { title: "總傷害", key: "damage", sortable: true },
            { title: "暴擊", key: "isCrit", sortable: true },
            { title: "HP 階段", key: "hpPhase", sortable: false },
        ];

        const bossSkillSequence = computed(() => {
            const entityMap = actorManager.value.entityMap;
            const segs = hpSegments.value;
            const CONSEC = consecGap.value;

            const getPhase = (at: number): string => {
                for (const s of segs) {
                    if (at >= s.startAt && at <= s.endAt)
                        return s.active ? "活躍" : "機制";
                }
                return "—";
            };

            // Step 1：同時間同技能合併（AoE）
            const groupMap = new Map<
                string,
                {
                    at: number;
                    endAt: number; // 此組最後一次命中的時間
                    skillId: number;
                    skillName: string;
                    hits: number; // 連擊數（初始 1）
                    targetNames: string[];
                    damage: number;
                    isCrit: boolean;
                    hpPhase: string;
                }
            >();

            for (const entity of selectedEntities.value) {
                for (const d of entity.applyDamages) {
                    if (!inRange(d.At)) continue;

                    const targetEntity = entityMap[d.TargetId];
                    const targetName = targetEntity
                        ? (prettyEntityName(targetEntity, raceNameMap) ??
                          d.TargetId)
                        : d.TargetId;

                    const key = `${d.At}|${d.SkillId}`;
                    const existing = groupMap.get(key);
                    if (existing) {
                        existing.damage += d.Damage;
                        if (d.IsCritical) existing.isCrit = true;
                        if (!existing.targetNames.includes(targetName))
                            existing.targetNames.push(targetName);
                    } else {
                        groupMap.set(key, {
                            at: d.At,
                            endAt: d.At,
                            skillId: d.SkillId,
                            skillName:
                                skillNameMap.value[d.SkillId] ||
                                `unknown:${d.SkillId}`,
                            hits: 1,
                            targetNames: [targetName],
                            damage: d.Damage,
                            isCrit: d.IsCritical,
                            hpPhase: getPhase(d.At),
                        });
                    }
                }
            }

            const sorted = [...groupMap.values()].sort((a, b) => a.at - b.at);

            // Step 2：連擊合併 — 同技能、間隔 ≤ CONSEC 秒的相鄰紀錄合成一筆
            const result: typeof sorted = [];
            for (const row of sorted) {
                const last = result[result.length - 1];
                if (
                    last &&
                    last.skillId === row.skillId &&
                    row.at - last.endAt <= CONSEC
                ) {
                    last.hits++;
                    last.endAt = row.at;
                    last.damage += row.damage;
                    if (row.isCrit) last.isCrit = true;
                    for (const t of row.targetNames)
                        if (!last.targetNames.includes(t))
                            last.targetNames.push(t);
                } else {
                    result.push({ ...row });
                }
            }

            return result;
        });

        const buildSkillSeqTsv = () => {
            const header = [
                "時間",
                "結束",
                "連擊",
                "技能",
                "目標",
                "總傷害",
                "暴擊",
                "HP階段",
            ].join("\t");
            const rows = bossSkillSequence.value.map((r) =>
                [
                    fmtTime(r.at),
                    r.hits > 1 ? fmtTime(r.endAt) : "",
                    r.hits,
                    r.skillName,
                    r.targetNames.join(" / "),
                    Math.round(r.damage),
                    r.isCrit ? "Y" : "",
                    r.hpPhase,
                ].join("\t"),
            );
            return [header, ...rows].join("\n");
        };

        const copySkillSequence = async () => {
            try {
                await navigator.clipboard.writeText(buildSkillSeqTsv());
                snackbar.value = true;
            } catch {
                /* ignore */
            }
        };

        const downloadSkillSequence = () => {
            const header = [
                "時間",
                "結束",
                "連擊",
                "技能",
                "目標",
                "總傷害",
                "暴擊",
                "HP階段",
            ].join(",");
            const rows = bossSkillSequence.value.map((r) =>
                [
                    fmtTime(r.at),
                    r.hits > 1 ? fmtTime(r.endAt) : "",
                    r.hits,
                    `"${r.skillName}"`,
                    `"${r.targetNames.join(" / ")}"`,
                    Math.round(r.damage),
                    r.isCrit ? "Y" : "",
                    r.hpPhase,
                ].join(","),
            );
            const csv = "\uFEFF" + [header, ...rows].join("\n");
            const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = "boss_skills.csv";
            a.click();
            URL.revokeObjectURL(url);
        };

        // ── Timeline filters (Section 3 & 4) ───────────────────────────
        /** 將 "HH:MM" 字串轉換成當日對應的 Unix 秒數（以 refAt 所在日期為基準） */
        const parseTimeStr = (
            timeStr: string,
            refAt: number,
        ): number | null => {
            if (!timeStr || !refAt) return null;
            const parts = timeStr.split(":").map(Number);
            const d = new Date(refAt * 1000);
            d.setHours(parts[0] ?? 0, parts[1] ?? 0, parts[2] ?? 0, 0);
            return d.getTime() / 1000;
        };

        // Section 3 filters
        const bossSkillTimeFrom = ref("");
        const bossSkillTimeTo = ref("");
        const bossSkillFilterIds = ref<number[]>([]);

        const bossSkillSequenceOptions = computed(() => {
            const seen = new Map<number, string>();
            for (const r of bossSkillSequence.value)
                if (!seen.has(r.skillId)) seen.set(r.skillId, r.skillName);
            return [...seen.entries()]
                .map(([id, name]) => ({ id, name }))
                .sort((a, b) => a.name.localeCompare(b.name));
        });

        const bossSkillFiltered = computed(() => {
            let rows = bossSkillSequence.value;
            const refAt = rows[0]?.at;
            if (bossSkillTimeFrom.value) {
                const from = parseTimeStr(bossSkillTimeFrom.value, refAt);
                if (from != null) rows = rows.filter((r) => r.at >= from);
            }
            if (bossSkillTimeTo.value) {
                const to = parseTimeStr(bossSkillTimeTo.value, refAt);
                if (to != null) rows = rows.filter((r) => r.at <= to);
            }
            if (bossSkillFilterIds.value.length) {
                const ids = new Set(bossSkillFilterIds.value);
                rows = rows.filter((r) => ids.has(r.skillId));
            }
            return rows;
        });

        // Section 4 filters
        const playerDmgTimeFrom = ref("");
        const playerDmgTimeTo = ref("");
        const playerDmgFilterIds = ref<number[]>([]); // 技能
        const playerDmgFilterPlayers = ref<string[]>([]); // 玩家 ID
        const filterOutPets = ref(true); // 預設排除寵物
        const playerCCFilter = ref<number[]>([]); // 玩家狀態 filter
        const bossCCFilter = ref<number[]>([]); // BOSS 狀態 filter

        // CC display config (Dual Listbox dialog)
        const ccConfigOpen = ref(false);
        const ccConfigTab = ref<"player" | "boss">("player");
        const playerHiddenCCs = ref<number[]>([]); // 移到左側（隱藏）的玩家 CC
        const bossHiddenCCs = ref<number[]>([]); // 移到左側（隱藏）的 BOSS CC
        const ccDualLeftSel = ref<number[]>([]); // 左側選中
        const ccDualRightSel = ref<number[]>([]); // 右側選中
        const ccLeftSearch = ref("");
        const ccRightSearch = ref("");
        watch(ccConfigTab, () => {
            ccDualLeftSel.value = [];
            ccDualRightSel.value = [];
            ccLeftSearch.value = "";
            ccRightSearch.value = "";
        });

        const playerSkillOptions = computed(() => {
            const seen = new Map<number, string>();
            for (const r of playerDamageLog.value)
                if (!seen.has(r.skillId)) seen.set(r.skillId, r.skillName);
            return [...seen.entries()]
                .map(([id, name]) => ({ id, name }))
                .sort((a, b) => a.name.localeCompare(b.name));
        });

        // CCIds that appear in the current data
        const allPlayerCCIds = computed(() => {
            const ids = new Set<number>();
            for (const r of playerDamageLog.value)
                for (const id of r.conditions) ids.add(id);
            return [...ids].sort((a, b) => a - b);
        });
        const allBossCCIds = computed(() => {
            const ids = new Set<number>();
            for (const r of playerDamageLog.value)
                for (const id of r.targetConditions) ids.add(id);
            return [...ids].sort((a, b) => a - b);
        });
        // Autocomplete option objects for CC filter dropdowns
        const playerCCOptions = computed(() =>
            allPlayerCCIds.value.map((id) => ({
                value: id,
                title: `${id} ${condNameMap.value[id] ?? ""}`.trim(),
            })),
        );
        const bossCCOptions = computed(() =>
            allBossCCIds.value.map((id) => ({
                value: id,
                title: `${id} ${condNameMap.value[id] ?? ""}`.trim(),
            })),
        );

        // Right side of dual listbox (visible)
        const playerVisibleCCIds = computed(() =>
            allPlayerCCIds.value.filter(
                (id) => !playerHiddenCCs.value.includes(id),
            ),
        );
        const bossVisibleCCIds = computed(() =>
            allBossCCIds.value.filter(
                (id) => !bossHiddenCCs.value.includes(id),
            ),
        );
        // Left side of dual listbox (hidden / moved out)
        const playerInvisibleCCIds = computed(() =>
            allPlayerCCIds.value.filter((id) =>
                playerHiddenCCs.value.includes(id),
            ),
        );
        const bossInvisibleCCIds = computed(() =>
            allBossCCIds.value.filter((id) => bossHiddenCCs.value.includes(id)),
        );

        const ccToggleSel = (arr: number[], id: number) => {
            const idx = arr.indexOf(id);
            if (idx >= 0) arr.splice(idx, 1);
            else arr.push(id);
        };
        const ccMoveToHidden = () => {
            const hidden =
                ccConfigTab.value === "player"
                    ? playerHiddenCCs
                    : bossHiddenCCs;
            const toAdd = ccDualRightSel.value.filter(
                (id) => !hidden.value.includes(id),
            );
            hidden.value = [...hidden.value, ...toAdd];
            ccDualRightSel.value = [];
        };
        const ccMoveToVisible = () => {
            const hidden =
                ccConfigTab.value === "player"
                    ? playerHiddenCCs
                    : bossHiddenCCs;
            const remove = new Set(ccDualLeftSel.value);
            hidden.value = hidden.value.filter((id) => !remove.has(id));
            ccDualLeftSel.value = [];
        };
        const ccResetVisible = () => {
            if (ccConfigTab.value === "player") playerHiddenCCs.value = [];
            else bossHiddenCCs.value = [];
        };

        const ccMatchSearch = (ccId: number, query: string) => {
            if (!query) return true;
            const q = query.toLowerCase();
            return (
                String(ccId).includes(q) ||
                (condNameMap.value[ccId] ?? "").toLowerCase().includes(q)
            );
        };
        const ccLeftFiltered = computed(() => {
            const list =
                ccConfigTab.value === "player"
                    ? playerInvisibleCCIds.value
                    : bossInvisibleCCIds.value;
            return list.filter((id) => ccMatchSearch(id, ccLeftSearch.value));
        });
        const ccRightFiltered = computed(() => {
            const list =
                ccConfigTab.value === "player"
                    ? playerVisibleCCIds.value
                    : bossVisibleCCIds.value;
            return list.filter((id) => ccMatchSearch(id, ccRightSearch.value));
        });
        const ccSelectAllLeft = () => {
            ccDualLeftSel.value = [...ccLeftFiltered.value];
        };
        const ccSelectAllRight = () => {
            ccDualRightSel.value = [...ccRightFiltered.value];
        };

        const playerNameOptions = computed(() => {
            const seen = new Map<string, string>(); // id → displayName
            for (const r of playerDamageLog.value) {
                if (filterOutPets.value && r.isPet) continue;
                if (!seen.has(r.playerId)) seen.set(r.playerId, r.playerName);
            }
            return [...seen.entries()]
                .map(([id, name]) => ({ id, name }))
                .sort((a, b) => a.name.localeCompare(b.name));
        });

        const playerDmgFiltered = computed(() => {
            let rows = playerDamageLog.value;
            const refAt = rows[0]?.at;
            if (playerDmgTimeFrom.value) {
                const from = parseTimeStr(playerDmgTimeFrom.value, refAt);
                if (from != null) rows = rows.filter((r) => r.at >= from);
            }
            if (playerDmgTimeTo.value) {
                const to = parseTimeStr(playerDmgTimeTo.value, refAt);
                if (to != null) rows = rows.filter((r) => r.at <= to);
            }
            if (playerDmgFilterIds.value.length) {
                const ids = new Set(playerDmgFilterIds.value);
                rows = rows.filter((r) => ids.has(r.skillId));
            }
            if (filterOutPets.value) {
                rows = rows.filter((r) => !r.isPet);
            }
            if (playerDmgFilterPlayers.value.length) {
                const pids = new Set(playerDmgFilterPlayers.value);
                rows = rows.filter((r) => pids.has(r.playerId));
            }
            if (playerCCFilter.value.length) {
                rows = rows.filter((r) =>
                    playerCCFilter.value.some((id) =>
                        r.conditions.includes(id),
                    ),
                );
            }
            if (bossCCFilter.value.length) {
                rows = rows.filter((r) =>
                    bossCCFilter.value.some((id) =>
                        r.targetConditions.includes(id),
                    ),
                );
            }
            return rows;
        });

        const playerDmgWithTriggers = computed(() => {
            const rules = passiveRules.value;
            const filteredRows = playerDmgFiltered.value;
            const allRows = playerDamageLog.value; // 全量資料供 lookback 用

            type TriggerInfo = { skillName: string; at: number; ratio: number };
            type RowWithTrigger = (typeof filteredRows)[number] & {
                trigger: TriggerInfo | null;
                isPassive: boolean;
            };

            if (!rules.length) {
                return filteredRows.map(
                    (r): RowWithTrigger => ({
                        ...r,
                        trigger: null,
                        isPassive: false,
                    }),
                );
            }

            // 以 skillId 為 key，只取有指定 skillId (非 0) 的規則
            const ruleMap = new Map(
                rules.filter((r) => r.skillId !== 0).map((r) => [r.skillId, r]),
            );

            return filteredRows.map((row): RowWithTrigger => {
                const rule = ruleMap.get(row.skillId);
                if (!rule) return { ...row, trigger: null, isPassive: false };

                // 同一玩家、在 lookback 秒內、非同技能的記錄（由新到舊找第一個符合比例的）
                const candidates = allRows.filter(
                    (r) =>
                        r.playerId === row.playerId &&
                        r.at < row.at &&
                        r.skillId !== row.skillId &&
                        row.at - r.at <= rule.lookback,
                );

                let trigger: TriggerInfo | null = null;
                for (let i = candidates.length - 1; i >= 0; i--) {
                    const prev = candidates[i];
                    if (prev.damage === 0) continue;
                    const ratio = (row.damage / prev.damage) * 100;
                    if (ratio >= rule.ratioMin && ratio <= rule.ratioMax) {
                        trigger = {
                            skillName: prev.skillName,
                            at: prev.at,
                            ratio,
                        };
                        break;
                    }
                }
                return { ...row, trigger, isPassive: true };
            });
        });

        // ── Section 4: Player damage log ───────────────────────────────

        // 被動技能規則
        type PassiveRule = {
            id: string;
            skillId: number;
            ratioMin: number; // % 例如 40
            ratioMax: number; // % 例如 48
            lookback: number; // 往前查幾秒
        };

        const passiveRules = ref<PassiveRule[]>([]);
        const passiveConfigOpen = ref(false);

        let _ruleSeq = 0;

        const addPassiveRule = () => {
            passiveRules.value.push({
                id: `r${_ruleSeq++}`,
                skillId: 0,
                ratioMin: 40,
                ratioMax: 48,
                lookback: 5,
            });
        };
        const addPassivePreset = (type: "flash" | "burst" | "chain") => {
            const nameHint =
                type === "flash"
                    ? "閃"
                    : type === "burst"
                      ? "爆破"
                      : "連續攻擊";
            const preset =
                type === "flash"
                    ? { ratioMin: 40, ratioMax: 49, lookback: 5 }
                    : type === "burst"
                      ? { ratioMin: 44, ratioMax: 56, lookback: 5 }
                      : { ratioMin: 190, ratioMax: 240, lookback: 5 };
            // 自動比對技能名稱
            const match = playerSkillOptions.value.find((s) =>
                s.name.includes(nameHint),
            );
            const skillId = match?.id ?? 0;
            passiveRules.value.push({
                id: `r${_ruleSeq++}`,
                skillId,
                ...preset,
            });
            passiveConfigOpen.value = true;
        };
        const removePassiveRule = (id: string) => {
            passiveRules.value = passiveRules.value.filter((r) => r.id !== id);
        };

        const showCritMark = ref(true);
        const playerDmgHeaders = computed(() => [
            { title: "時間",     key: "at",               sortable: false },
            { title: "玩家",     key: "playerName",        sortable: true  },
            { title: "技能",     key: "skillName",         sortable: true  },
            { title: "傷害",     key: "damage",            sortable: true, align: "end" as const },
            { title: "玩家狀態", key: "conditions",        sortable: false },
            { title: "BOSS狀態", key: "targetConditions",  sortable: false },
        ]);

        const playerDamageLog = computed(() => {
            const entityMap = actorManager.value.entityMap;
            const rows: {
                at: number;
                playerId: string;
                playerName: string;
                petName: string;
                isPet: boolean;
                skillId: number;
                skillName: string;
                damage: number;
                isCrit: boolean;
                conditions: number[];
                targetConditions: number[];
            }[] = [];

            for (const entity of selectedEntities.value) {
                for (const d of entity.takeDamages) {
                    if (!inRange(d.At)) continue;

                    // 用 ownerId 判斷是否為寵物攻擊
                    const attacker = entityMap[d.Id];
                    const isPet = !!attacker?.ownerId;
                    const owner = isPet
                        ? entityMap[attacker.ownerId]
                        : undefined;

                    const playerName = getDisplayName(isPet
                        ? (prettyEntityName(owner, raceNameMap) ??
                          attacker.ownerId)
                        : attacker
                          ? (prettyEntityName(attacker, raceNameMap) ?? d.Id)
                          : d.Id);

                    const playerId = isPet ? attacker.ownerId : d.Id;

                    const petName = isPet
                        ? (prettyEntityName(attacker, raceNameMap) ?? d.Id)
                        : "";

                    rows.push({
                        at: d.At,
                        playerId,
                        playerName,
                        petName,
                        isPet,
                        skillId: d.SkillId,
                        skillName:
                            skillNameMap.value[d.SkillId] ||
                            `unknown:${d.SkillId}`,
                        damage: d.Damage,
                        isCrit: d.IsCritical,
                        conditions: d.Conditions.map((c) => c.CCId),
                        targetConditions: d.TargetConditions.map((c) => c.CCId),
                    });
                }
            }

            rows.sort((a, b) => a.at - b.at);
            return rows;
        });

        const buildPlayerDmgTsv = () => {
            const header = [
                "時間",
                "玩家",
                "寵物",
                "技能",
                "傷害",
                "暴擊",
            ].join("\t");
            const rows = playerDamageLog.value.map((r) =>
                [
                    fmtTime(r.at),
                    r.playerName,
                    r.petName,
                    r.skillName,
                    Math.round(r.damage),
                    r.isCrit ? "Y" : "",
                ].join("\t"),
            );
            return [header, ...rows].join("\n");
        };

        // 第一次有資料時自動加入三個預設規則（閃燄 / 爆破 / 連續攻擊）
        // ※ 必須放在 playerDamageLog（被 playerSkillOptions 引用）宣告之後，
        //    否則 watch 設置時會立即對 playerSkillOptions.value 求值，觸發 TDZ
        let _rulesInitialized = false;
        watch(playerSkillOptions, (opts) => {
            if (_rulesInitialized || !opts.length) return;
            _rulesInitialized = true;
            const presets: [string, number, number][] = [
                ["閃",    40,  49],   // 閃燄系
                ["爆破",  44,  56],   // 爆破
                ["連續攻擊", 190, 240], // 連續攻擊
            ];
            for (const [hint, ratioMin, ratioMax] of presets) {
                const match = opts.find((s) => s.name.includes(hint));
                if (!match) continue;
                passiveRules.value.push({
                    id: `r${_ruleSeq++}`,
                    skillId: match.id,
                    ratioMin,
                    ratioMax,
                    lookback: 5,
                });
            }
        });

        const copyPlayerDmg = async () => {
            try {
                await navigator.clipboard.writeText(buildPlayerDmgTsv());
                snackbar.value = true;
            } catch {
                /* ignore */
            }
        };

        const downloadPlayerDmg = () => {
            const header = [
                "時間",
                "玩家",
                "寵物",
                "技能",
                "傷害",
                "暴擊",
            ].join(",");
            const rows = playerDamageLog.value.map((r) =>
                [
                    `"${fmtTime(r.at)}"`,
                    `"${r.playerName}"`,
                    `"${r.petName}"`,
                    `"${r.skillName}"`,
                    Math.round(r.damage),
                    r.isCrit ? "Y" : "",
                ].join(","),
            );
            const csv = "\uFEFF" + [header, ...rows].join("\n");
            const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = "player_damage.csv";
            a.click();
            URL.revokeObjectURL(url);
        };

        // ── Detail dialog ───────────────────────────────────────────────
        const detailOpen = ref(false);
        const detailSegment = ref<Seg | null>(null);

        const detailHeaders = [
            { title: "技能", key: "skillName", sortable: true },
            { title: "命中", key: "count", sortable: true },
            { title: "暴擊", key: "crits", sortable: true },
            { title: "平均傷", key: "avgDamage", sortable: true },
            { title: "總傷", key: "totalDamage", sortable: true },
        ];

        const detailRows = computed(() => {
            const seg = detailSegment.value;
            if (!seg) return [];
            const skillMap: Record<
                number,
                { count: number; damage: number; crits: number }
            > = {};
            for (const entity of selectedEntities.value) {
                for (const d of entity.takeDamages) {
                    if (d.At < seg.startAt || d.At > seg.endAt) continue;
                    if (!inRange(d.At)) continue;
                    const sid = ((d as any).SkillId as number) ?? 0;
                    if (!skillMap[sid])
                        skillMap[sid] = { count: 0, damage: 0, crits: 0 };
                    skillMap[sid].count++;
                    skillMap[sid].damage += d.Damage;
                    if ((d as any).IsCritical) skillMap[sid].crits++;
                }
            }
            return Object.entries(skillMap)
                .map(([sid, v]) => ({
                    skillId: +sid,
                    skillName: skillNameMap.value[+sid] || `unknown:${sid}`,
                    count: v.count,
                    crits: v.crits,
                    avgDamage: v.count > 0 ? v.damage / v.count : 0,
                    totalDamage: v.damage,
                }))
                .sort((a, b) => b.totalDamage - a.totalDamage);
        });

        const openDetail = (seg: Seg) => {
            detailSegment.value = seg;
            detailOpen.value = true;
        };

        // ── Idle detail dialog（無變動期間 BOSS 技能）──────────────────
        const idleDetailOpen = ref(false);
        const idleDetailSegment = ref<Seg | null>(null);

        const idleDetailHeaders = [
            { title: "技能", key: "skillName", sortable: true },
            { title: "次數", key: "count", sortable: true },
            { title: "平均傷", key: "avgDamage", sortable: true },
            { title: "總傷", key: "totalDamage", sortable: true },
        ];

        const idleDetailRows = computed(() => {
            const seg = idleDetailSegment.value;
            if (!seg) return [];
            const skillMap: Record<
                number,
                { name: string; count: number; damage: number }
            > = {};
            for (const entity of selectedEntities.value) {
                for (const d of entity.applyDamages) {
                    if (d.At < seg.startAt || d.At > seg.endAt) continue;
                    if (!skillMap[d.SkillId]) {
                        skillMap[d.SkillId] = {
                            name:
                                skillNameMap.value[d.SkillId] ||
                                `unknown:${d.SkillId}`,
                            count: 0,
                            damage: 0,
                        };
                    }
                    skillMap[d.SkillId].count++;
                    skillMap[d.SkillId].damage += d.Damage;
                }
            }
            return Object.values(skillMap)
                .map((v) => ({
                    skillName: v.name,
                    count: v.count,
                    avgDamage: v.count > 0 ? v.damage / v.count : 0,
                    totalDamage: v.damage,
                }))
                .sort((a, b) => b.totalDamage - a.totalDamage);
        });

        const openIdleDetail = (seg: Seg) => {
            idleDetailSegment.value = seg;
            idleDetailOpen.value = true;
        };

        // ── Export ──────────────────────────────────────────────────────
        const snackbar = ref(false);

        const buildTsvRows = () => {
            const header = ["狀態", "開始", "結束", "時長(s)", "傷害次數"].join(
                "\t",
            );
            const rows = hpSegments.value.map((s) =>
                [
                    s.active ? "活躍" : "無變動",
                    fmtTime(s.startAt),
                    fmtTime(s.endAt),
                    s.duration.toFixed(1),
                    s.active ? s.hits : "",
                ].join("\t"),
            );
            return [header, ...rows].join("\n");
        };

        const copyTable = async () => {
            try {
                await navigator.clipboard.writeText(buildTsvRows());
                snackbar.value = true;
            } catch {
                // fallback
            }
        };

        const downloadTable = () => {
            const header = ["狀態", "開始", "結束", "時長(s)", "傷害次數"].join(
                ",",
            );
            const rows = hpSegments.value.map((s) =>
                [
                    s.active ? "活躍" : "無變動",
                    fmtTime(s.startAt),
                    fmtTime(s.endAt),
                    s.duration.toFixed(1),
                    s.active ? s.hits : "",
                ].join(","),
            );
            const csv = "\uFEFF" + [header, ...rows].join("\n");
            const blob = new Blob([csv], { type: "text/csv;charset=utf-8;" });
            const url = URL.createObjectURL(blob);
            const a = document.createElement("a");
            a.href = url;
            a.download = "hp_timeline.csv";
            a.click();
            URL.revokeObjectURL(url);
        };

        // ── Section 5: 傷害散點圖 ───────────────────────────────────────
        const scatterFilterPlayers = ref<string[]>([]);
        const scatterFilterSkillIds = ref<number[]>([]);
        const scatterCritMode = ref<"all" | "non-crit" | "crit">("all");
        const scatterMaWindow = ref(0);
        const scatterMaOptions = [
            { label: "關閉", value: 0 },
            { label: "15s",  value: 15 },
            { label: "30s",  value: 30 },
            { label: "60s",  value: 60 },
            { label: "2min", value: 120 },
        ];
        const scatterChartDom = ref<HTMLElement | undefined>(undefined);
        let scatterChart: Highcharts.Chart | undefined;

        const scatterSkillOptions = computed(() => {
            const selected = new Set(scatterFilterPlayers.value);
            const seen = new Map<number, string>();
            for (const r of playerDamageLog.value) {
                if (selected.size > 0 && !selected.has(r.playerId)) continue;
                if (!seen.has(r.skillId)) seen.set(r.skillId, r.skillName);
            }
            return [...seen.entries()]
                .map(([id, name]) => ({ id, name }))
                .sort((a, b) => a.name.localeCompare(b.name));
        });

        const scatterSeriesData = computed(() => {
            const selectedPlayers = new Set(scatterFilterPlayers.value);
            const selectedSkills = new Set(scatterFilterSkillIds.value);
            const critMode = scatterCritMode.value;

            const filtered = playerDamageLog.value.filter((r) => {
                if (selectedPlayers.size > 0 && !selectedPlayers.has(r.playerId)) return false;
                if (selectedSkills.size > 0 && !selectedSkills.has(r.skillId)) return false;
                if (critMode === "non-crit" && r.isCrit) return false;
                if (critMode === "crit" && !r.isCrit) return false;
                return true;
            });

            const groups = new Map<string, {
                label: string;
                points: { x: number; y: number; isCrit: boolean }[];
            }>();
            for (const r of filtered) {
                const key = `${r.playerId}::${r.skillId}`;
                if (!groups.has(key)) {
                    groups.set(key, { label: `${r.playerName} · ${r.skillName}`, points: [] });
                }
                groups.get(key)!.points.push({ x: r.at * 1000, y: r.damage, isCrit: r.isCrit });
            }

            return [...groups.values()].map((g, idx) => ({
                ...g,
                color: SCATTER_PALETTE[idx % SCATTER_PALETTE.length],
            }));
        });

        const rebuildScatterChart = () => {
            if (!scatterChartDom.value) return;
            scatterChart?.destroy();

            const showAll = scatterCritMode.value === "all";
            const maWindowMs = scatterMaWindow.value * 1000;
            const series: SeriesOptionsType[] = [];

            scatterSeriesData.value.forEach(({ label, color, points }, idx) => {
                const seriesId = `scatter_${idx}`;
                series.push({
                    type: "scatter",
                    id: seriesId,
                    name: label,
                    color,
                    data: points.map((p) => ({
                        x: p.x,
                        y: p.y,
                        marker: (showAll && p.isCrit)
                            ? { symbol: "circle", radius: 3, fillColor: "transparent", lineWidth: 1.5, lineColor: color }
                            : { symbol: "circle", radius: 3 },
                    })),
                } as SeriesOptionsType);

                if (maWindowMs > 0 && points.length > 0) {
                    const sorted = [...points].sort((a, b) => a.x - b.x);
                    const maData: [number, number][] = sorted.map((p) => {
                        const inWindow = sorted.filter((pp) => pp.x >= p.x - maWindowMs && pp.x <= p.x);
                        const avg = inWindow.reduce((s, pp) => s + pp.y, 0) / inWindow.length;
                        return [p.x, avg];
                    });
                    series.push({
                        type: "line",
                        name: `${label} MA${scatterMaWindow.value}s`,
                        linkedTo: seriesId,
                        color,
                        dashStyle: "ShortDash",
                        lineWidth: 2,
                        marker: { enabled: false },
                        data: maData,
                    } as SeriesOptionsType);
                }
            });

            const options: Options = {
                title:   { text: "" },
                credits: { enabled: false },
                time:    { timezone: Intl.DateTimeFormat().resolvedOptions().timeZone },
                chart: {
                    type: "scatter",
                    animation: false,
                    zooming: { type: "xy" },
                    margin: [8, 12, 36, 56],
                    backgroundColor: "transparent",
                },
                xAxis: {
                    type: "datetime",
                    labels: { style: { fontSize: "10px" } },
                },
                yAxis: {
                    title: { text: "傷害", style: { fontSize: "10px" } },
                    labels: {
                        style: { fontSize: "10px" },
                        formatter() {
                            const v = this.value as number;
                            if (v >= 1_000_000) return `${(v / 1_000_000).toFixed(1)}M`;
                            if (v >= 1_000) return `${(v / 1_000).toFixed(0)}k`;
                            return String(v);
                        },
                    },
                    min: 0,
                },
                tooltip: {
                    formatter() {
                        const time = new Date(this.x as number).toLocaleTimeString("zh-TW", { hour12: false });
                        return `<span style="font-size:10px">${time}</span><br>`
                            + `<span style="color:${this.color}">●</span> ${this.series.name}: <b>${Math.round(this.y ?? 0).toLocaleString()}</b>`;
                    },
                },
                legend: {
                    enabled: true,
                    itemStyle: { fontSize: "10px", fontWeight: "normal" },
                },
                plotOptions: {
                    scatter: { turboThreshold: 10000 },
                },
                series,
            };

            scatterChart = highcharts.chart(scatterChartDom.value, options);
        };

        watch(
            [scatterSeriesData, scatterMaWindow, () => collapsed.value.scatter],
            () => nextTick(rebuildScatterChart),
        );

        onUnmounted(() => scatterChart?.destroy());

        return {
            region,
            condNameMap,
            collapsed,
            segmentMode,
            gapThreshold,
            minActiveHits,
            minDamage,
            invincBridgeGap,
            minSegmentDuration,
            excludedSkillIds,
            bossReceivedSkillOptions,
            selectedEntityId,
            entityOptions,
            quickRaceOptions,
            quickSelectRace,
            quickRaceFilter,
            skillHeaders,
            skillRows,
            hpHeaders,
            hpSegments,
            hpSummary,
            hpBarSegments,
            hpBarMarkers,
            detailOpen,
            detailSegment,
            detailHeaders,
            detailRows,
            openDetail,
            snackbar,
            copyTable,
            downloadTable,
            consecGap,
            bossSkillHeaders,
            bossSkillSequence,
            bossSkillFiltered,
            bossSkillSequenceOptions,
            bossSkillTimeFrom,
            bossSkillTimeTo,
            bossSkillFilterIds,
            copySkillSequence,
            downloadSkillSequence,
            playerDmgHeaders,
            showCritMark,
            fmtTime24,
            playerDamageLog,
            playerDmgFiltered,
            playerDmgWithTriggers,
            playerSkillOptions,
            playerNameOptions,
            playerDmgTimeFrom,
            playerDmgTimeTo,
            playerDmgFilterIds,
            playerDmgFilterPlayers,
            filterOutPets,
            playerCCFilter,
            bossCCFilter,
            allPlayerCCIds,
            allBossCCIds,
            playerCCOptions,
            bossCCOptions,
            playerVisibleCCIds,
            bossVisibleCCIds,
            playerInvisibleCCIds,
            bossInvisibleCCIds,
            ccConfigOpen,
            ccConfigTab,
            ccDualLeftSel,
            ccDualRightSel,
            ccToggleSel,
            ccMoveToHidden,
            ccMoveToVisible,
            ccResetVisible,
            ccLeftSearch,
            ccRightSearch,
            ccLeftFiltered,
            ccRightFiltered,
            ccSelectAllLeft,
            ccSelectAllRight,
            playerHiddenCCs,
            bossHiddenCCs,
            copyPlayerDmg,
            downloadPlayerDmg,
            passiveRules,
            passiveConfigOpen,
            addPassiveRule,
            addPassivePreset,
            removePassiveRule,
            getBossSkillsDuring,
            idleDetailOpen,
            idleDetailSegment,
            idleDetailHeaders,
            idleDetailRows,
            openIdleDetail,
            getMabiNameColor,
            fmtTime,
            fmtDuration,
            // Section 5
            scatterFilterPlayers,
            scatterFilterSkillIds,
            scatterCritMode,
            scatterMaWindow,
            scatterMaOptions,
            scatterSkillOptions,
            scatterChartDom,
        };
    },
});

const SCATTER_PALETTE = [
    "#5470c6","#91cc75","#fac858","#ee6666",
    "#73c0de","#3ba272","#fc8452","#9a60b4","#ea7ccc",
];
</script>
