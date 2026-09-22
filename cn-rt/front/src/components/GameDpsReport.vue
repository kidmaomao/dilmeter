<template>
    <div v-if="isSharePreview" class="share-preview-page">
        <img v-if="sharePreviewUrl" :src="sharePreviewUrl" alt="导出图片预览" />
        <span v-else>正在生成导出图片…</span>
    </div>
    <div v-else class="report-page" :class="{ 'design-preview': isDesignPreview }">
        <div v-if="!isDesignPreview || showPreviewControls" class="report-controls">
            <label class="control-field">
                <span class="control-label">
                    战斗目标
                    <span
                        class="control-help"
                        tabindex="0"
                        title="这里显示 Boss 的真实最大血量（客户端 Stat30），统一保留两位小数。"
                        aria-label="Boss 血量说明"
                    ><v-icon icon="mdi-help-circle-outline" size="14" /></span>
                </span>
                <select
                    :value="selectedBossId"
                    :disabled="isFileLoading || (bossOptions.length === 0 && archivedBossOptions.length === 0 && !isRecordReplay)"
                    @change="handleBattleTargetSelected"
                >
                    <option v-if="bossOptions.length === 0" value="">尚无战斗数据</option>
                    <option v-if="isRecordReplay" value="live:">返回实时监测</option>
                    <option v-for="boss in bossOptions" :key="boss.entityId" :value="boss.entityId">{{ boss.label }}</option>
                    <optgroup v-if="archivedBossOptions.length" label="此前场次 · 选中后加载">
                        <option v-for="boss in archivedBossOptions" :key="boss.value" :value="boss.value">{{ boss.label }}</option>
                    </optgroup>
                </select>
            </label>
            <label class="only-boss-switch" title="只列出客户端观察到的最大血量或累计承伤达到 1 亿的目标">
                <input v-model="onlyBossTargets" type="checkbox" />
                仅看 Boss
            </label>
            <label v-if="dpsVisible" class="control-field">
                <span class="control-label">
                    分享角色
                    <span
                        class="control-help"
                        tabindex="0"
                        title="Boss 进入机制时的溢伤和被动伤害也会计入 DPS；因此累计承伤与角色占比可能超过 Boss 血量和 100%。"
                        aria-label="DPS 溢伤统计说明"
                    ><v-icon icon="mdi-help-circle-outline" size="14" /></span>
                </span>
                <select v-model="selectedPlayerId" :disabled="playerOptions.length === 0">
                    <option v-if="playerOptions.length === 0" value="">尚无角色数据</option>
                    <option v-for="player in playerOptions" :key="player.entityId" :value="player.entityId">{{ player.label }}</option>
                </select>
            </label>
            <button
                v-if="dpsVisible"
                type="button"
                class="game-button team-chart-button"
                :disabled="!(summary?.players.length) && !isDesignPreview"
                @click="teamChartOpen = true"
            >
                <v-icon icon="mdi-chart-areaspline" size="15" />团队绘图
            </button>
            <button class="game-button" :disabled="isStandalone || isFileLoading" @click="emit('refresh-info')">
                <v-icon icon="mdi-refresh" size="15" />刷新信息
            </button>
            <button class="game-button" :disabled="isFileLoading" @click="emit('view-records')">
                <v-icon icon="mdi-history" size="15" />查看记录
            </button>
            <button v-if="dpsVisible" class="game-button" :disabled="!summary && !isDesignPreview" @click="saveBattleRecord">
                <v-icon icon="mdi-download" size="15" />导出日志
            </button>
            <button v-if="dpsVisible" class="game-button" :disabled="!reportPlayer" @click="copyShareImage">
                <v-icon icon="mdi-content-copy" size="15" />复制图片
            </button>
            <button class="game-button" @click="clearReportData">
                <v-icon icon="mdi-delete-sweep" size="15" />清空数据
            </button>
        </div>

        <div v-if="notice && (!isDesignPreview || showPreviewControls)" class="notice-line" :class="`notice-${noticeType}`">
            {{ notice }}
            <button aria-label="关闭提示" @click="notice = ''"><v-icon icon="mdi-close" size="14" /></button>
        </div>

        <section class="combat-window" aria-label="详细战斗统计">
            <header class="window-titlebar">
                <v-icon icon="mdi-sword-cross" size="15" />
                <span>详细战斗统计</span>
                <v-icon class="window-close" icon="mdi-close-box-outline" size="17" />
            </header>

            <div v-if="dpsVisible" class="top-stat-grid">
                <article v-for="metric in metrics" :key="metric.label" class="top-stat">
                    <div class="top-stat-label">{{ metric.label }}</div>
                    <div class="top-stat-value">{{ metric.value }}</div>
                </article>
            </div>

            <div class="table-frame">
                <nav class="combat-tabs" aria-label="技能分类">
                    <button
                        v-for="tab in visibleCombatTabs"
                        :key="tab.id"
                        type="button"
                        class="combat-tab"
                        :class="{ active: activeTab === tab.id }"
                        :aria-current="activeTab === tab.id ? 'page' : undefined"
                        @click="activeTab = tab.id"
                    >{{ tab.label }}</button>
                </nav>
                <div v-if="activeTab === 'summary'" class="combat-summary-panel">
                    <h2>战斗汇总</h2>
                    <dl class="combat-summary-table">
                        <div v-for="item in combatSummaryRows" :key="item.label" class="combat-summary-row">
                            <dt>{{ item.label }}</dt>
                            <dd>{{ item.value }}</dd>
                        </div>
                    </dl>

                    <section class="condition-panel" aria-label="状态效果">
                        <header class="condition-panel-header">
                            <button
                                v-if="conditionTarget === 'ally'"
                                type="button"
                                class="condition-settings-button"
                                :class="{ active: playerBuffSettingsOpen }"
                                @click="togglePlayerBuffSettings"
                            >
                                <v-icon icon="mdi-tune-variant" size="13" />管理 Buff
                            </button>
                            <div class="condition-panel-title">
                                <strong>{{ conditionTarget === "ally" ? "玩家 Buff 覆盖率" : "怪物 Debuff 覆盖率" }}</strong>
                                <span>{{ conditionTargetLabel }}</span>
                                <span v-if="conditionTarget === 'monster' && bossHealthText" class="boss-health-readout">生命 {{ bossHealthText }}</span>
                            </div>
                            <div class="condition-target-toggle" role="radiogroup" aria-label="状态效果对象">
                                <label :class="{ active: conditionTarget === 'ally' }">
                                    <input v-model="conditionTarget" type="radio" value="ally" />
                                    我方
                                </label>
                                <label :class="{ active: conditionTarget === 'monster' }">
                                    <input v-model="conditionTarget" type="radio" value="monster" />
                                    怪物
                                </label>
                            </div>
                        </header>

                        <div v-if="conditionTarget === 'ally' && playerBuffSettingsOpen" class="player-buff-settings">
                            <div class="player-buff-settings-tip">
                                搜索任意状态名称或 CC ID 添加；下方分类可删除已有项目。设置和收藏会保存在本机。
                            </div>
                            <div class="player-buff-search-row">
                                <v-icon icon="mdi-magnify" size="15" />
                                <input
                                    v-model.trim="playerBuffInput"
                                    type="search"
                                    aria-label="搜索 Buff 名称或 CC ID"
                                    placeholder="搜索 Buff 名称或 CC ID"
                                    @input="playerBuffInputError = ''"
                                    @keyup.enter="addPlayerBuff"
                                />
                                <button type="button" @click="addPlayerBuff">添加首项</button>
                                <button type="button" @click="resetPlayerBuffs">恢复默认</button>
                            </div>
                            <div v-if="playerBuffInput && playerBuffSearchResults.length" class="player-buff-search-results">
                                <button
                                    v-for="buff in playerBuffSearchResults"
                                    :key="buff.id"
                                    type="button"
                                    :disabled="configuredPlayerBuffIdSet.has(buff.id)"
                                    @click="addPlayerBuffById(buff.id)"
                                >
                                    <span class="condition-icon-wrap compact">
                                        <span>CC</span>
                                        <img
                                            :src="conditionIconUrl(buff.id)"
                                            :alt="`${buff.name}状态图标`"
                                            @error="hideMissingConditionIcon"
                                        />
                                    </span>
                                    <span><strong>{{ buff.name }}</strong><small>CC {{ buff.id }}</small></span>
                                    <v-icon
                                        :icon="configuredPlayerBuffIdSet.has(buff.id) ? 'mdi-check' : 'mdi-plus'"
                                        size="14"
                                    />
                                </button>
                            </div>
                            <div v-else-if="playerBuffInput" class="player-buff-search-empty">
                                {{ playerBuffInputError || "没有匹配的状态" }}
                            </div>

                            <details v-for="group in playerBuffGroups" :key="group.name" class="player-buff-manage-group">
                                <summary>
                                    <span>{{ group.name }}</span>
                                    <small>{{ selectedBuffCount(group) }}/{{ group.items.length }}</small>
                                    <v-icon icon="mdi-chevron-down" size="15" />
                                </summary>
                                <div class="player-buff-chip-list">
                                    <button
                                        v-for="buff in group.items"
                                        :key="buff.id"
                                        type="button"
                                        class="player-buff-chip"
                                        :class="{ selected: configuredPlayerBuffIdSet.has(buff.id) }"
                                        :title="buff.detail || buff.name"
                                        @click="togglePlayerBuff(buff.id)"
                                    >
                                        <span>{{ configuredPlayerBuffIdSet.has(buff.id) ? "✓" : "+" }}</span>
                                        {{ buff.name }} <small>{{ buff.id }}</small>
                                    </button>
                                </div>
                            </details>
                            <details v-if="customPlayerBuffs.length" class="player-buff-manage-group">
                                <summary>
                                    <span>自定义</span>
                                    <small>{{ customPlayerBuffs.length }}</small>
                                    <v-icon icon="mdi-chevron-down" size="15" />
                                </summary>
                                <div class="player-buff-chip-list">
                                    <button
                                        v-for="buff in customPlayerBuffs"
                                        :key="buff.id"
                                        type="button"
                                        class="player-buff-chip selected"
                                        @click="togglePlayerBuff(buff.id)"
                                    >
                                        <span>×</span>{{ buff.name }} <small>{{ buff.id }}</small>
                                    </button>
                                </div>
                            </details>
                        </div>

                        <div v-if="selectedConditionRow && reportSession" class="condition-timeline-panel">
                            <header>
                                <div class="condition-timeline-heading">
                                    <div class="condition-icon-wrap">
                                        <span>CC</span>
                                        <img
                                            :src="selectedConditionRow.iconUrl"
                                            :alt="`${selectedConditionRow.name}状态图标`"
                                            @error="hideMissingConditionIcon"
                                        />
                                    </div>
                                    <div>
                                        <strong>{{ selectedConditionRow.name }}</strong>
                                        <span>覆盖 {{ fmtPct(selectedConditionRow.coverage) }} · 生效 {{ fmtCoverageDuration(selectedConditionRow.activeSeconds) }}</span>
                                    </div>
                                </div>
                                <button type="button" aria-label="关闭时间轴" @click="selectedConditionId = null">
                                    <v-icon icon="mdi-close" size="15" />
                                </button>
                            </header>
                            <div ref="conditionTimelineElement" class="condition-timeline-row">
                                <div class="condition-timeline-bars">
                                    <span
                                        v-for="(segment, index) in selectedConditionRow.segments"
                                        :key="index"
                                        class="condition-timeline-segment"
                                        :style="conditionSegmentStyle(segment)"
                                        :title="`${fmtBattleElapsed(segment.start)} - ${fmtBattleElapsed(segment.end)}`"
                                    />
                                    <i
                                        v-for="tick in conditionTimelineScale.gridTicks"
                                        :key="tick.seconds"
                                        :class="`condition-timeline-${tick.kind}-tick`"
                                        :style="{ left: `${tick.pct}%` }"
                                    />
                                </div>
                                <div class="condition-timeline-axis">
                                    <i
                                        v-for="tick in conditionTimelineScale.gridTicks"
                                        :key="`axis-${tick.seconds}`"
                                        :class="`condition-timeline-${tick.kind}-tick`"
                                        :style="{ left: `${tick.pct}%` }"
                                        aria-hidden="true"
                                    />
                                    <span
                                        v-for="tick in conditionTimelineScale.labelTicks"
                                        :key="`label-${tick.seconds}`"
                                        :style="{ left: `${tick.pct}%` }"
                                    >{{ fmtCoverageDuration(tick.seconds) }}</span>
                                </div>
                            </div>
                        </div>

                        <template v-if="conditionTarget === 'ally' && conditionRows.length">
                            <section class="favorite-condition-section">
                                <header>
                                    <v-icon icon="mdi-star" size="14" />
                                    <strong>收藏</strong>
                                    <span>{{ favoriteConditionRows.length ? "收藏的 Buff 固定显示" : "点击 Buff 右上角星标固定显示" }}</span>
                                </header>
                                <div v-if="favoriteConditionRows.length" class="condition-grid favorite-condition-grid">
                                    <article
                                        v-for="condition in favoriteConditionRows"
                                        :key="`favorite-${condition.entityId}-${condition.ccId}`"
                                        class="condition-card"
                                        :class="{ selected: selectedConditionId === condition.ccId }"
                                        :title="`${condition.name}：覆盖 ${fmtPct(condition.coverage)}`"
                                    >
                                        <button
                                            type="button"
                                            class="condition-favorite-button active"
                                            :aria-label="`取消收藏 ${condition.name}`"
                                            title="星星只固定在 Buff 汇总中；桌面图标请使用铃铛提醒"
                                            @click="toggleFavoriteCondition(condition.ccId)"
                                        ><v-icon icon="mdi-star" size="16" /></button>
                                        <button type="button" class="condition-alert-button" :class="{ active: isBuffAlertEnabled(condition.ccId) }" :aria-label="`设置 ${condition.name} 提醒`" title="添加到桌面 Buff 提醒" @click="openBuffAlertEditor(condition.ccId)">
                                            <v-icon icon="mdi-bell-outline" size="15" />
                                        </button>
                                        <button type="button" class="condition-card-main" @click="selectCondition(condition.ccId)">
                                            <span class="condition-icon-wrap">
                                                <span>CC</span>
                                                <img :src="condition.iconUrl" :alt="`${condition.name}状态图标`" @error="hideMissingConditionIcon" />
                                            </span>
                                            <strong class="condition-coverage">{{ fmtPct(condition.coverage) }}</strong>
                                            <span class="condition-name">{{ condition.name }}</span>
                                            <small>CC {{ condition.ccId }}</small>
                                        </button>
                                    </article>
                                </div>
                            </section>

                            <div class="condition-category-list">
                                <details v-for="group in playerConditionGroups" :key="group.name" class="condition-category">
                                    <summary>
                                        <span>{{ group.name }}</span>
                                        <small>{{ group.activeCount }} 个生效 / {{ group.rows.length }} 个</small>
                                        <v-icon icon="mdi-chevron-down" size="16" />
                                    </summary>
                                    <div class="condition-grid">
                                        <article
                                            v-for="condition in group.rows"
                                            :key="`${group.name}-${condition.entityId}-${condition.ccId}`"
                                            class="condition-card"
                                            :class="{ selected: selectedConditionId === condition.ccId }"
                                            :title="`${condition.name}：覆盖 ${fmtPct(condition.coverage)}`"
                                        >
                                            <button
                                                type="button"
                                                class="condition-favorite-button"
                                                :aria-label="`收藏 ${condition.name}`"
                                                title="星星只固定在 Buff 汇总中；桌面图标请使用铃铛提醒"
                                                @click="toggleFavoriteCondition(condition.ccId)"
                                            ><v-icon icon="mdi-star-outline" size="16" /></button>
                                            <button type="button" class="condition-alert-button" :class="{ active: isBuffAlertEnabled(condition.ccId) }" :aria-label="`设置 ${condition.name} 提醒`" title="添加到桌面 Buff 提醒" @click="openBuffAlertEditor(condition.ccId)">
                                                <v-icon icon="mdi-bell-outline" size="15" />
                                            </button>
                                            <button type="button" class="condition-card-main" @click="selectCondition(condition.ccId)">
                                                <span class="condition-icon-wrap">
                                                    <span>CC</span>
                                                    <img :src="condition.iconUrl" :alt="`${condition.name}状态图标`" @error="hideMissingConditionIcon" />
                                                </span>
                                                <strong class="condition-coverage">{{ fmtPct(condition.coverage) }}</strong>
                                                <span class="condition-name">{{ condition.name }}</span>
                                                <small>CC {{ condition.ccId }}</small>
                                            </button>
                                        </article>
                                    </div>
                                </details>
                            </div>
                        </template>
                        <div v-else-if="conditionTarget === 'monster' && conditionRows.length" class="condition-grid">
                            <article
                                v-for="condition in conditionRows"
                                :key="`${condition.entityId}-${condition.ccId}`"
                                class="condition-card"
                                :class="{ selected: selectedConditionId === condition.ccId }"
                                :title="`${condition.name}：覆盖 ${fmtPct(condition.coverage)}`"
                            >
                                <button type="button" class="condition-alert-button" :class="{ active: isDebuffAlertEnabled(condition.ccId) }" :aria-label="`设置 ${condition.name} Debuff 提醒`" title="监控即将失效或遗漏的 Debuff" @click="toggleDebuffAlert(condition.ccId)">
                                    <v-icon :icon="isDebuffAlertEnabled(condition.ccId) ? 'mdi-bell' : 'mdi-bell-outline'" size="15" />
                                </button>
                                <button type="button" class="condition-card-main" @click="selectCondition(condition.ccId)">
                                    <span class="condition-icon-wrap">
                                        <span>CC</span>
                                        <img :src="condition.iconUrl" :alt="`${condition.name}状态图标`" @error="hideMissingConditionIcon" />
                                    </span>
                                    <strong class="condition-coverage">{{ fmtPct(condition.coverage) }}</strong>
                                    <span class="condition-name">{{ condition.name }}</span>
                                    <small>CC {{ condition.ccId }}</small>
                                </button>
                            </article>
                        </div>
                        <div v-else class="condition-empty">
                            <v-icon icon="mdi-shield-off-outline" size="25" />
                            <span>{{ conditionEmptyText }}</span>
                            <button
                                v-if="conditionTarget === 'ally' && effectivePlayerBuffIds.length === 0"
                                type="button"
                                @click="resetPlayerBuffs"
                            >恢复默认</button>
                        </div>

                    </section>
                </div>

                <template v-else-if="activeTab === 'attack'">
                    <div class="skill-head skill-grid">
                        <span>技能名</span>
                        <span>累计伤害</span>
                        <span>每秒伤害</span>
                        <span>伤害占比</span>
                        <span>最大伤害</span>
                        <span>使用次数</span>
                        <span>暴击发动次数</span>
                    </div>

                    <div v-if="skillRows.length" class="skill-scroll">
                        <div v-for="skill in skillRows" :key="skill.skillId" class="skill-row skill-grid">
                            <div class="skill-progress-track" aria-hidden="true">
                                <span :style="{ width: `${skill.barRatio * 100}%` }" />
                            </div>
                            <div class="skill-name-cell">
                                <img
                                    class="skill-icon"
                                    :src="skill.iconUrl"
                                    :alt="`${skill.name}技能图标`"
                                    @error="useFallbackIcon"
                                />
                                <span class="skill-name-text">{{ skill.name }}</span>
                            </div>
                            <span>{{ fmtGameNumber(skill.totalDamage) }}</span>
                            <span>{{ fmtGameNumber(skill.dps) }}</span>
                            <span>{{ fmtPct(skill.ratio) }}</span>
                            <span>{{ fmtGameNumber(skill.maxDamage) }}</span>
                            <span>{{ skill.totalHits || "-" }}</span>
                            <span>{{ skill.critHits }}({{ skill.noCritRate ? "—" : fmtPct(skill.critRate) }})</span>
                        </div>
                    </div>

                    <div v-else class="empty-combat-state">
                        <v-icon icon="mdi-sword-cross" size="30" />
                        <strong>等待战斗数据</strong>
                        <span>进入战斗并对首领造成伤害后，攻击技能会显示在这里。</span>
                    </div>
                </template>

                <div v-else class="reminder-settings-page">
                    <section class="reminder-profile-toolbar" aria-label="提醒方案与时间微调">
                        <div class="reminder-profile-heading">
                            <v-icon icon="mdi-folder-cog-outline" size="17" />
                            <div>
                                <strong>提醒方案</strong>
                                <span>保存多套 Buff、Debuff、瞄准提醒与技能 CD 配置，切换职业时可直接套用。</span>
                            </div>
                        </div>
                        <label>
                            当前方案
                            <select v-model="selectedReminderProfileId" @change="switchReminderProfile">
                                <option v-for="profile in reminderProfileStore.profiles" :key="profile.id" :value="profile.id">
                                    {{ profile.name }}
                                </option>
                            </select>
                        </label>
                        <label>
                            方案名称
                            <input
                                :value="activeReminderProfile?.name || ''"
                                type="text"
                                maxlength="32"
                                @input="renameActiveReminderProfile"
                            />
                        </label>
                        <button type="button" class="profile-action" @click="duplicateReminderProfile">
                            <v-icon icon="mdi-content-copy" size="13" />复制为新方案
                        </button>
                        <button
                            type="button"
                            class="profile-action danger"
                            :disabled="reminderProfileStore.profiles.length <= 1"
                            @click="deleteActiveReminderProfile"
                        >
                            <v-icon icon="mdi-delete-outline" size="13" />删除方案
                        </button>
                        <div class="buff-time-adjustment">
                            <span>Buff 时间整体微调</span>
                            <button type="button" aria-label="减少一秒" @click="adjustBuffTime(-1)">−</button>
                            <input
                                v-model.number="buffOverlaySettings.timeAdjustmentSeconds"
                                type="number"
                                min="-3600"
                                max="3600"
                                step="1"
                                inputmode="numeric"
                                @input="markBuffAlertSettingsDirty"
                            />
                            <button type="button" aria-label="增加一秒" @click="adjustBuffTime(1)">+</button>
                            <small>秒</small>
                        </div>
                    </section>
                    <section class="buff-alert-settings" aria-label="Buff 提醒设置">
                        <header>
                            <div>
                                <strong>桌面 Buff 提醒（可同时监控多个）</strong>
                                <span>图标位置使用固定屏幕坐标；右侧可直接调整图标大小、音量和坐标。</span>
                            </div>
                            <span v-if="reminderSettingsDirty" class="buff-alert-unsaved">有未保存修改</span>
                            <slot name="reminder-settings-actions" :dirty="reminderSettingsDirty" />
                        </header>
                        <div class="buff-alert-picker">
                            <label for="reminder-buff-alert-target">搜索桌面提醒</label>
                            <input
                                id="reminder-buff-alert-target"
                                v-model.trim="buffAlertInput"
                                type="search"
                                placeholder="输入 Buff 名称或 CC ID"
                                autocomplete="off"
                                @keyup.enter="addFirstBuffAlertSearchResult"
                            />
                            <button type="button" class="buff-alert-add" :disabled="!buffAlertSearchResults.length" @click="addFirstBuffAlertSearchResult">
                                <v-icon icon="mdi-plus" size="13" />添加
                            </button>
                        </div>
                        <div v-if="buffAlertInput && buffAlertSearchResults.length" class="buff-alert-search-results">
                            <button
                                v-for="buff in buffAlertSearchResults"
                                :key="buff.id"
                                type="button"
                                :disabled="Boolean(buffOverlaySettings.rules[buff.id])"
                                @click="addBuffAlertRule(buff.id)"
                            >
                                <span class="condition-icon-wrap compact">
                                    <span>CC</span>
                                    <img :src="conditionIconUrl(buff.id)" :alt="`${buff.name}图标`" @error="hideMissingConditionIcon" />
                                </span>
                                <span><strong>{{ buff.name }}</strong><small>CC {{ buff.id }}</small></span>
                                <v-icon :icon="buffOverlaySettings.rules[buff.id] ? 'mdi-check' : 'mdi-plus'" size="13" />
                            </button>
                        </div>
                        <div v-if="configuredBuffAlertRules.length" class="buff-alert-rule-list">
                            <div v-for="rule in configuredBuffAlertRules" :key="rule.ccId" class="buff-alert-editor">
                                <div class="buff-alert-identity">
                                    <span class="condition-icon-wrap compact">
                                        <span>CC</span>
                                        <img :src="conditionIconUrl(rule.ccId)" :alt="`${conditionDisplayName(rule.ccId)}图标`" @error="hideMissingConditionIcon" />
                                    </span>
                                    <strong>{{ conditionDisplayName(rule.ccId) }}</strong>
                                    <small>CC {{ rule.ccId }}</small>
                                </div>
                                <template v-if="isStackAlertCondition(rule.ccId)">
                                    <span v-if="isStackOnlyBuffAlertCondition(rule.ccId)" class="buff-alert-mode-note">
                                        {{ rule.ccId === 1080 ? "层数型 Buff" : "Boss 机制负面状态" }} · 达到设定层数时提醒
                                    </span>
                                    <div class="buff-stack-alert-controls">
                                        <label class="buff-stack-alert-toggle">
                                            <input v-model="rule.stackAlertEnabled" type="checkbox" @change="markBuffAlertSettingsDirty" />
                                            层数提醒
                                        </label>
                                        <label v-if="rule.stackAlertEnabled" class="buff-stack-alert-threshold">
                                            达到
                                            <input v-model.number="rule.stackThreshold" type="number" min="1" max="99" step="1" @change="markBuffAlertSettingsDirty" />
                                            层
                                        </label>
                                        <label v-if="rule.stackAlertEnabled">
                                            <input v-model="rule.stackScreenEnabled" type="checkbox" @change="markBuffAlertSettingsDirty" />
                                            屏幕闪烁
                                        </label>
                                        <span v-if="rule.stackAlertEnabled && rule.stackScreenEnabled" class="buff-stack-alert-coordinates">
                                            <span>提示位置</span>
                                            <label>X <input v-model.number="rule.stackX" type="number" min="-32000" max="32000" step="1" @change="markBuffAlertSettingsDirty" /></label>
                                            <label>Y <input v-model.number="rule.stackY" type="number" min="-32000" max="32000" step="1" @change="markBuffAlertSettingsDirty" /></label>
                                        </span>
                                        <label v-if="rule.stackAlertEnabled">
                                            层数音效
                                            <select v-model="rule.stackSoundMode" @change="markBuffAlertSettingsDirty">
                                                <option value="none">不提示</option>
                                                <option value="electronic">内置电子音</option>
                                                <option value="voice">晓晓语音</option>
                                                <option value="custom">自定义音效</option>
                                            </select>
                                        </label>
                                        <label v-if="rule.stackAlertEnabled && rule.stackSoundMode === 'custom'" class="buff-alert-file-picker">
                                            <input
                                                type="file"
                                                accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav"
                                                @change="uploadCustomBuffStackSound(rule, $event)"
                                            />
                                            <span>{{ rule.stackCustomSoundName || "选择 MP3 / WAV" }}</span>
                                        </label>
                                        <button v-if="rule.stackAlertEnabled && rule.stackSoundMode === 'custom'" type="button" class="local-tts-open" @click="openLocalTTS('buff-stack', rule.ccId, conditionDisplayName(rule.ccId))">
                                            <v-icon icon="mdi-account-voice" size="13" />本地TTS
                                        </button>
                                        <button
                                            v-if="rule.stackAlertEnabled && rule.stackScreenEnabled"
                                            type="button"
                                            class="buff-alert-preview"
                                            @click="previewBuffStackAlert(rule)"
                                        >预览位置与动画</button>
                                    </div>
                                </template>
                                <template v-if="!isStackOnlyBuffAlertCondition(rule.ccId)">
                                    <label>
                                        <input v-model="rule.overlayEnabled" type="checkbox" @change="onBuffOverlayRuleChanged(rule)" />
                                        显示图标
                                    </label>
                                    <label>
                                        时长来源
                                        <select v-model="rule.durationMode" @change="markBuffAlertSettingsDirty">
                                            <option value="auto">自动读取</option>
                                            <option value="manual">手动固定时长</option>
                                        </select>
                                    </label>
                                    <label v-if="rule.durationMode === 'manual'">
                                        固定时长（秒）
                                        <input v-model.number="rule.manualDurationSeconds" type="number" min="1" max="86400" @change="markBuffAlertSettingsDirty" />
                                    </label>
                                    <label>
                                        <input v-model="rule.flashEnabled" type="checkbox" @change="markBuffAlertSettingsDirty" />
                                        到期前闪烁
                                    </label>
                                    <label>
                                        闪烁提前（秒）
                                        <input v-model.number="rule.flashThresholdSeconds" type="number" min="1" max="3600" @change="markBuffAlertSettingsDirty" />
                                    </label>
                                    <label>
                                        音效
                                        <select v-model="rule.soundMode" @change="onBuffSoundModeChanged(rule)">
                                            <option value="none">不提示</option>
                                            <option value="electronic">欢快电子音</option>
                                            <option value="voice">晓晓：音乐要结束了</option>
                                            <option value="custom">自定义音效</option>
                                        </select>
                                    </label>
                                    <label v-if="rule.soundMode === 'custom'" class="buff-alert-file-picker">
                                        <input
                                            type="file"
                                            accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav"
                                            @change="uploadCustomBuffSound(rule, $event)"
                                        />
                                        <span>{{ rule.customSoundName || "选择 MP3 / WAV" }}</span>
                                    </label>
                                    <button v-if="rule.soundMode === 'custom'" type="button" class="local-tts-open" @click="openLocalTTS('buff', rule.ccId, conditionDisplayName(rule.ccId))">
                                        <v-icon icon="mdi-account-voice" size="13" />本地TTS
                                    </button>
                                    <label v-if="rule.soundMode !== 'none'">
                                        音效提前（秒）
                                        <input v-model.number="rule.soundThresholdSeconds" type="number" min="1" max="3600" @change="markBuffAlertSettingsDirty" />
                                    </label>
                                    <button
                                        v-if="rule.soundMode !== 'none'"
                                        type="button"
                                        class="buff-alert-preview"
                                        :disabled="rule.soundMode === 'custom' && !rule.customSoundId"
                                        @click="previewBuffAlertSound(rule.soundMode, true, rule.customSoundId)"
                                    >试听</button>
                                </template>
                                <button type="button" class="buff-alert-remove" @click="removeBuffAlertRule(rule.ccId)">
                                    <v-icon icon="mdi-delete-outline" size="13" />删除
                                </button>
                                <span class="buff-alert-runtime-status">{{ buffAlertRuntimeStatus(rule) }}</span>
                            </div>
                        </div>
                        <p v-else>尚未添加提醒。添加后会在这里逐项显示，多个项目可同时启用。</p>
                    </section>

                    <section class="debuff-alert-settings" aria-label="Boss Debuff 提醒设置">
                        <header>
                            <div>
                                <strong>Boss Debuff 提醒</strong>
                                <span>已配置的图标平时常亮；监测到当前怪物拥有对应 CC 后立即隐藏，状态移除后重新显示。</span>
                            </div>
                            <button type="button" class="common-debuff-button" @click="addCommonDebuffAlerts">
                                <v-icon icon="mdi-playlist-plus" size="14" />常用debuff
                            </button>
                            <slot name="debuff-reminder-settings-actions" />
                        </header>
                        <div class="debuff-alert-toolbar">
                            <div class="buff-alert-picker debuff-alert-picker">
                                <label for="reminder-debuff-alert-target">搜索提醒</label>
                                <input
                                    id="reminder-debuff-alert-target"
                                    v-model.trim="debuffAlertInput"
                                    type="search"
                                    placeholder="输入 Debuff 名称或 CC ID"
                                    autocomplete="off"
                                    @keyup.enter="addFirstDebuffAlertSearchResult"
                                />
                                <button type="button" class="buff-alert-add" :disabled="!debuffAlertSearchResults.length" @click="addFirstDebuffAlertSearchResult">
                                    <v-icon icon="mdi-plus" size="13" />添加
                                </button>
                            </div>
                        </div>
                        <div v-if="debuffAlertInput && debuffAlertSearchResults.length" class="buff-alert-search-results">
                            <button
                                v-for="debuff in debuffAlertSearchResults"
                                :key="debuff.id"
                                type="button"
                                :disabled="Boolean(debuffAlertSettings.rules[debuff.id])"
                                @click="addDebuffAlertRule(debuff.id)"
                            >
                                <span class="condition-icon-wrap compact"><span>CC</span><img :src="conditionIconUrl(debuff.id)" :alt="`${debuff.name}图标`" @error="hideMissingConditionIcon" /></span>
                                <span><strong>{{ debuff.name }}</strong><small>CC {{ debuff.id }}</small></span>
                                <v-icon :icon="debuffAlertSettings.rules[debuff.id] ? 'mdi-check' : 'mdi-plus'" size="13" />
                            </button>
                        </div>
                        <section class="debuff-boss-manager" aria-label="自定义 Debuff Boss 种族">
                            <div class="debuff-boss-manager-copy">
                                <strong>自定义 Boss 种族</strong>
                                <span>这些种族即使血量低于 1 亿，也可作为唯一的 Debuff 监测主体。</span>
                            </div>
                            <div class="buff-alert-picker debuff-boss-picker">
                                <label for="reminder-debuff-boss-race">搜索种族</label>
                                <input
                                    id="reminder-debuff-boss-race"
                                    v-model.trim="debuffBossRaceInput"
                                    type="search"
                                    placeholder="输入 Boss 名称或种族 ID"
                                    autocomplete="off"
                                    @keyup.enter="addFirstForcedDebuffBossRace"
                                />
                                <button type="button" class="buff-alert-add" :disabled="!debuffBossRaceSearchResults.length" @click="addFirstForcedDebuffBossRace">
                                    <v-icon icon="mdi-plus" size="13" />添加
                                </button>
                            </div>
                            <div v-if="debuffBossRaceInput && debuffBossRaceSearchResults.length" class="buff-alert-search-results debuff-boss-search-results">
                                <button
                                    v-for="race in debuffBossRaceSearchResults"
                                    :key="race.id"
                                    type="button"
                                    @click="addForcedDebuffBossRace(race.id)"
                                >
                                    <v-icon icon="mdi-skull-scan-outline" size="17" />
                                    <span><strong>{{ race.name }}</strong><small>种族 ID {{ race.id }}</small></span>
                                    <v-icon icon="mdi-plus" size="13" />
                                </button>
                            </div>
                            <div v-if="configuredForcedDebuffBossRaces.length" class="debuff-boss-race-list">
                                <span v-for="race in configuredForcedDebuffBossRaces" :key="race.id">
                                    <b>{{ race.name }}</b>
                                    <small>ID {{ race.id }}</small>
                                    <button type="button" :aria-label="`删除 ${race.name}`" @click="removeForcedDebuffBossRace(race.id)"><v-icon icon="mdi-close" size="13" /></button>
                                </span>
                            </div>
                            <p v-else>未添加自定义种族；默认只监测估算血量达到 1 亿的目标。</p>
                        </section>
                        <div v-if="configuredDebuffAlertRules.length" class="debuff-alert-rule-list">
                            <div v-for="(rule, index) in configuredDebuffAlertRules" :key="rule.ccId" class="debuff-alert-rule">
                                <div class="debuff-alert-identity">
                                    <span class="condition-icon-wrap compact"><span>CC</span><img :src="conditionIconUrl(rule.ccId)" :alt="`${conditionDisplayName(rule.ccId)}图标`" @error="hideMissingConditionIcon" /></span>
                                    <strong>{{ conditionDisplayName(rule.ccId) }}</strong>
                                    <small>CC {{ rule.ccId }}</small>
                                </div>
                                <div class="debuff-order-controls" aria-label="调整图标顺序">
                                    <button type="button" :disabled="index === 0" :aria-label="`上移 ${conditionDisplayName(rule.ccId)}`" title="向前移动" @click="moveDebuffAlertRule(rule.ccId, -1)"><v-icon icon="mdi-chevron-up" size="15" /></button>
                                    <button type="button" :disabled="index === configuredDebuffAlertRules.length - 1" :aria-label="`下移 ${conditionDisplayName(rule.ccId)}`" title="向后移动" @click="moveDebuffAlertRule(rule.ccId, 1)"><v-icon icon="mdi-chevron-down" size="15" /></button>
                                </div>
                                <label class="debuff-warning-toggle">
                                    <input v-model="rule.flashEnabled" type="checkbox" @change="markDebuffAlertSettingsDirty" />
                                    到期前闪烁
                                </label>
                                <label class="debuff-warning-seconds">
                                    闪烁提前（秒）
                                    <input v-model.number="rule.warningSeconds" type="number" min="1" max="3600" step="1" :disabled="!rule.flashEnabled" @change="markDebuffAlertSettingsDirty" />
                                </label>
                                <label class="debuff-sound-choice">
                                    音效
                                    <select :value="rule.soundEnabled ? rule.soundMode : 'none'" @change="onDebuffSoundSelectionChanged(rule, $event)">
                                        <option value="none">不提示</option>
                                        <option value="electronic">内置电子音</option>
                                        <option value="voice">晓晓语音</option>
                                        <option value="custom">自定义音效</option>
                                    </select>
                                </label>
                                <label v-if="rule.soundEnabled && rule.soundMode === 'custom'" class="buff-alert-file-picker">
                                    <input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav" @change="uploadCustomDebuffSound(rule, $event)" />
                                    <span>{{ rule.customSoundName || "选择 MP3 / WAV" }}</span>
                                </label>
                                <button v-if="rule.soundEnabled && rule.soundMode === 'custom'" type="button" class="local-tts-open" @click="openLocalTTS('debuff', rule.ccId, conditionDisplayName(rule.ccId))">
                                    <v-icon icon="mdi-account-voice" size="13" />本地TTS
                                </button>
                                <button
                                    v-if="rule.soundEnabled"
                                    type="button"
                                    class="buff-alert-preview"
                                    :disabled="rule.soundMode === 'custom' && !rule.customSoundId"
                                    @click="previewDebuffSound(rule, true)"
                                >试听</button>
                                <button type="button" class="buff-alert-remove" @click="removeDebuffAlert(rule.ccId)"><v-icon icon="mdi-delete-outline" size="13" />删除</button>
                            </div>
                        </div>
                        <p v-else>尚未监控 Debuff。可在上方搜索名称或 CC ID，也可在“综合 → 怪物”的状态卡片右上角点击铃铛添加。</p>
                    </section>

                    <section class="boss-mechanic-settings" aria-label="Boss 特殊机制提醒设置">
                        <header>
                            <div>
                                <strong>Boss 特殊机制提醒</strong>
                                <span>从怪物技能执行或机制实体生成信号开始倒计时；可分别选择声音和屏幕中央数字提示。</span>
                            </div>
                            <label class="boss-mechanic-volume">
                                音量
                                <input v-model.number="bossMechanicSettings.volume" type="number" min="0" max="100" step="1" @input="markBossMechanicSettingsDirty" />
                                <span>%</span>
                            </label>
                            <label class="boss-mechanic-display-setting">
                                大小
                                <input v-model.number="bossMechanicSettings.scalePercent" type="number" min="50" max="200" step="5" @input="markBossMechanicSettingsDirty" />
                                <span>%</span>
                            </label>
                            <span v-if="bossMechanicSettingsDirty" class="boss-mechanic-unsaved">有未保存修改</span>
                            <button
                                type="button"
                                class="skill-cooldown-save"
                                :class="{ 'needs-save': bossMechanicSettingsDirty, saved: bossMechanicSettingsSaved }"
                                @click="persistBossMechanicSettings"
                            >
                                <v-icon :icon="bossMechanicSettingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" size="13" />
                                {{ bossMechanicSettingsSaved ? "已保存" : "保存设定" }}
                            </button>
                        </header>
                        <div class="boss-mechanic-rule-list">
                            <div v-for="rule in configuredBossMechanicRules" :key="rule.key" class="boss-mechanic-rule">
                                <div class="boss-mechanic-identity">
                                    <v-icon :icon="rule.trigger === 'orb-spawn' ? 'mdi-orbit' : 'mdi-laser-pointer'" size="24" />
                                    <strong>{{ rule.name }}</strong>
                                    <small>{{ rule.trigger === "orb-spawn" ? "米耶尔环绕球生成信号" : `怪物技能 ${rule.skillId} 执行信号` }}</small>
                                </div>
                                <label><input v-model="rule.enabled" type="checkbox" @change="markBossMechanicSettingsDirty" />启用提醒</label>
                                <label>
                                    倒计时（秒）
                                    <input v-model.number="rule.countdownSeconds" type="number" min="1" max="120" step="0.1" @input="markBossMechanicSettingsDirty" />
                                </label>
                                <label><input v-model="rule.showCountdown" type="checkbox" @change="markBossMechanicSettingsDirty" />屏幕中央倒计时</label>
                                <label class="boss-mechanic-rule-coordinate">
                                    提示坐标
                                    <b>X</b>
                                    <input v-model.number="rule.x" type="number" min="-32000" max="32000" step="1" @input="markBossMechanicSettingsDirty" />
                                    <b>Y</b>
                                    <input v-model.number="rule.y" type="number" min="-32000" max="32000" step="1" @input="markBossMechanicSettingsDirty" />
                                </label>
                                <label>
                                    音效
                                    <select v-model="rule.soundMode" @change="markBossMechanicSettingsDirty">
                                        <option value="none">不提示</option>
                                        <option value="dedicated">专属机制音效</option>
                                        <option value="custom">自定义 / 本地TTS</option>
                                    </select>
                                </label>
                                <label v-if="rule.soundMode === 'custom'" class="buff-alert-file-picker">
                                    <input type="file" accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav" @change="uploadCustomBossMechanicSound(rule, $event)" />
                                    <span>{{ rule.customSoundName || "选择 MP3 / WAV" }}</span>
                                </label>
                                <button v-if="rule.soundMode === 'custom'" type="button" class="local-tts-open" @click="openLocalTTS('boss', rule.key, rule.name)">
                                    <v-icon icon="mdi-account-voice" size="13" />本地TTS
                                </button>
                                <button type="button" class="buff-alert-preview" :disabled="rule.soundMode === 'custom' && !rule.customSoundId" @click="previewBossMechanicRule(rule.key)">测试提醒</button>
                                <span class="buff-alert-runtime-status">{{ bossMechanicRuntimeStatus(rule.key) }}</span>
                            </div>
                            <div class="boss-mechanic-rule boss-mechanic-health-rule">
                                <div class="boss-mechanic-identity">
                                    <v-icon icon="mdi-heart-pulse" size="24" />
                                    <strong>安乐碎片机制</strong>
                                    <small>选中对应阶段的机制碎片时显示放大血条；各阶段可独立开启</small>
                                </div>
                                <div class="miel-shard-phase-options" aria-label="显示阶段">
                                    <label><input v-model="bossMechanicSettings.mielShardHealthPhases.normal80" type="checkbox" @change="markBossMechanicSettingsDirty" />普通米耶尔 80%</label>
                                    <label><input v-model="bossMechanicSettings.mielShardHealthPhases.normal60" type="checkbox" @change="markBossMechanicSettingsDirty" />普通米耶尔 60%</label>
                                    <label><input v-model="bossMechanicSettings.mielShardHealthPhases.normal40" type="checkbox" @change="markBossMechanicSettingsDirty" />普通米耶尔 40%</label>
                                    <label><input v-model="bossMechanicSettings.mielShardHealthPhases.regret80" type="checkbox" @change="markBossMechanicSettingsDirty" />米耶尔：悔恨 80%</label>
                                </div>
                                <label class="boss-mechanic-rule-coordinate" title="放大血条左上角的屏幕坐标；解锁覆盖层后也可以拖动预览血条">
                                    血条坐标
                                    <b>X</b>
                                    <input v-model.number="bossMechanicSettings.mielShardHealthBarX" type="number" min="-32000" max="32000" step="1" @input="markBossMechanicSettingsDirty" />
                                    <b>Y</b>
                                    <input v-model.number="bossMechanicSettings.mielShardHealthBarY" type="number" min="-32000" max="32000" step="1" @input="markBossMechanicSettingsDirty" />
                                </label>
                                <label title="只缩放放大血条，不影响环绕球与射线提醒">
                                    大小
                                    <input v-model.number="bossMechanicSettings.mielShardHealthBarScalePercent" type="number" min="50" max="200" step="5" @input="markBossMechanicSettingsDirty" />
                                    <span>%</span>
                                </label>
                                <label>
                                    透明度
                                    <input v-model.number="bossMechanicSettings.mielShardHealthBarOpacityPercent" type="number" min="20" max="100" step="5" @input="markBossMechanicSettingsDirty" />
                                    <span>%</span>
                                </label>
                                <button type="button" class="buff-alert-preview" @click="previewMielShardHealthBar">预览血条</button>
                                <span class="buff-alert-runtime-status">{{ mielShardHealthBarPreview ? "正在预览 5 秒，可解锁后拖动" : "不影响环绕球与射线提醒" }}</span>
                            </div>
                        </div>
                    </section>

                    <section class="boss-mechanic-settings effect-timer-settings" aria-label="伤害增益效果计时条设置">
                        <header>
                            <div>
                                <strong>伤害增益效果计时条</strong>
                                <span>技能执行或角色状态出现时开始倒数；读条随剩余时间缩短，到期自动消失。</span>
                            </div>
                            <span v-if="effectTimerSettingsDirty" class="boss-mechanic-unsaved">有未保存修改</span>
                            <button type="button" class="skill-cooldown-save" :class="{ 'needs-save': effectTimerSettingsDirty, saved: effectTimerSettingsSaved }" @click="persistEffectTimerSettings">
                                <v-icon :icon="effectTimerSettingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" size="13" />
                                {{ effectTimerSettingsSaved ? "已保存" : "保存设定" }}
                            </button>
                            <button type="button" class="buff-alert-preview" @click="addEffectTimerRule"><v-icon icon="mdi-plus" size="13" />添加计时条</button>
                        </header>
                        <div v-if="configuredEffectTimerRules.length" class="boss-mechanic-rule-list">
                            <div v-for="rule in configuredEffectTimerRules" :key="rule.key" class="boss-mechanic-rule effect-timer-rule">
                                <div class="boss-mechanic-identity">
                                    <v-icon :icon="rule.sourceType === 'skill' ? 'mdi-sword-cross' : 'mdi-timer-sand'" size="24" />
                                    <input v-model.trim="rule.name" :list="rule.sourceType === 'skill' ? 'effect-timer-skill-options' : 'effect-timer-condition-options'" type="text" maxlength="80" aria-label="技能或状态名称" @input="markEffectTimerSettingsDirty" @change="resolveEffectTimerSourceFromName(rule)" />
                                    <small>{{ rule.sourceType === "skill" ? "技能" : "状态" }} ID {{ rule.sourceId }}</small>
                                </div>
                                <label><input v-model="rule.enabled" type="checkbox" @change="markEffectTimerSettingsDirty" />启用</label>
                                <label>触发来源<select v-model="rule.sourceType" @change="resolveEffectTimerRuleName(rule); markEffectTimerSettingsDirty()"><option value="skill">技能</option><option value="condition">Character Condition</option></select></label>
                                <label>技能 / 状态 ID<input v-model.number="rule.sourceId" type="number" min="1" step="1" @change="resolveEffectTimerRuleName(rule); markEffectTimerSettingsDirty()" /></label>
                                <label>作用对象<select v-model="rule.targetMode" @change="markEffectTimerSettingsDirty"><option value="self">自身</option><option value="monster">怪物</option></select></label>
                                <label>持续时间（秒）<input v-model.number="rule.durationSeconds" type="number" min="0.1" max="86400" step="0.1" @input="markEffectTimerSettingsDirty" /></label>
                                <label><input v-model="rule.alwaysVisible" type="checkbox" @change="markEffectTimerSettingsDirty" />到期后一直显示空条</label>
                                <label>方向<select v-model="rule.orientation" @change="markEffectTimerSettingsDirty"><option value="horizontal">横向</option><option value="vertical">纵向</option></select></label>
                                <label class="boss-mechanic-rule-coordinate">坐标 <b>X</b><input v-model.number="rule.x" type="number" min="-32000" max="32000" step="1" @input="markEffectTimerSettingsDirty" /><b>Y</b><input v-model.number="rule.y" type="number" min="-32000" max="32000" step="1" @input="markEffectTimerSettingsDirty" /></label>
                                <label>大小<input v-model.number="rule.scalePercent" type="number" min="50" max="200" step="5" @input="markEffectTimerSettingsDirty" /><span>%</span></label>
                                <label>透明度<input v-model.number="rule.opacityPercent" type="number" min="20" max="100" step="5" @input="markEffectTimerSettingsDirty" /><span>%</span></label>
                                <button type="button" class="buff-alert-preview" @click="previewEffectTimer(rule)">预览</button>
                                <button type="button" class="buff-alert-remove" @click="removeEffectTimerRule(rule.key)"><v-icon icon="mdi-delete-outline" size="13" />删除</button>
                            </div>
                        </div>
                        <p v-else>尚未配置计时条。添加后可自由选择技能或 Character Condition 作为触发来源。</p>
                        <datalist id="effect-timer-skill-options"><option v-for="item in allSkillDefinitions" :key="item.id" :value="item.name">技能 {{ item.id }}</option></datalist>
                        <datalist id="effect-timer-condition-options"><option v-for="item in allConditionDefinitions" :key="item.id" :value="item.name">CC {{ item.id }}</option></datalist>
                    </section>

                    <section class="aim-reminder-settings" aria-label="穿心箭瞄准提醒设置">
                        <header>
                            <div>
                                <strong>瞄准提醒</strong>
                                <span>穿心箭锁定目标后显示独立紧凑读条；无需在技能 CD 中添加穿心箭。</span>
                            </div>
                            <div class="aim-reminder-header-actions">
                                <span v-if="aimReminderSettingsDirty" class="aim-reminder-unsaved">有未保存修改</span>
                                <button
                                    type="button"
                                    class="aim-reminder-save skill-cooldown-save"
                                    :class="{ 'needs-save': aimReminderSettingsDirty, saved: aimReminderSettingsSaved }"
                                    :disabled="aimReminderSettingsSaving"
                                    @click="persistAimReminderSettings"
                                >
                                    <v-icon :icon="aimReminderSettingsSaving ? 'mdi-loading' : aimReminderSettingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" :class="{ 'mdi-spin': aimReminderSettingsSaving }" size="13" />
                                    {{ aimReminderSettingsSaving ? "保存中" : aimReminderSettingsSaved ? "已保存" : "保存设定" }}
                                </button>
                            </div>
                        </header>
                        <div class="aim-reminder-controls">
                            <label class="aim-reminder-toggle">
                                <input v-model="skillCooldownSettings.aimReminder.enabled" type="checkbox" @change="markAimReminderSettingsDirty" />
                                启用瞄准提醒
                            </label>
                            <label class="aim-reminder-toggle" title="未使用穿心箭时也保留 0% 的紧凑提示；开始瞄准后自动进入读条">
                                <input v-model="skillCooldownSettings.aimReminder.alwaysVisible" type="checkbox" :disabled="!skillCooldownSettings.aimReminder.enabled" @change="markAimReminderSettingsDirty" />
                                未瞄准时一直显示
                            </label>
                            <label title="填写武器面板的基础射程；可从候选武器中选择，也可以直接输入其他武器射程">
                                武器基础射程
                                <input
                                    v-model.number="skillCooldownSettings.aimReminder.weaponRange"
                                    list="magnum-weapon-range-presets"
                                    type="number"
                                    min="100"
                                    max="10000"
                                    step="10"
                                    @input="markAimReminderSettingsDirty"
                                />
                                <datalist id="magnum-weapon-range-presets">
                                    <option value="2200">毁灭弓 2200</option>
                                    <option value="2000">释魂弓 2000</option>
                                    <option value="2100">释魂弩 2100</option>
                                </datalist>
                            </label>
                            <label title="鉴定射程会与武器基础射程相加，每级增加 70">
                                鉴定射程
                                <select v-model.number="skillCooldownSettings.aimReminder.rangeIdentificationLevel" @change="markAimReminderSettingsDirty">
                                    <option
                                        v-for="option in magnumRangeIdentificationOptions"
                                        :key="option.level"
                                        :value="option.level"
                                    >
                                        {{ option.level === 0 ? "无鉴定" : `${option.level}级（+${option.bonus}）` }}
                                    </option>
                                </select>
                            </label>
                            <output class="aim-reminder-effective-range">
                                <span>合计射程</span>
                                <strong>{{ magnumAimEffectiveRangeText() }}</strong>
                            </output>
                            <label>
                                瞄准校准
                                <input
                                    v-model.number="skillCooldownSettings.aimReminder.calibrationPercent"
                                    type="number"
                                    min="20"
                                    max="40"
                                    step="1"
                                    @input="markAimReminderSettingsDirty"
                                />
                                %
                            </label>
                            <label>
                                尔格瞄准加成
                                <input
                                    v-model.number="skillCooldownSettings.aimReminder.ergSpeedPercent"
                                    type="number"
                                    min="100"
                                    max="1000"
                                    step="10"
                                    @input="markAimReminderSettingsDirty"
                                />
                                %
                            </label>
                            <label>
                                延迟微调
                                <input
                                    v-model.number="skillCooldownSettings.aimReminder.fineTuneSeconds"
                                    type="number"
                                    min="-10"
                                    max="10"
                                    step="0.01"
                                    @input="markAimReminderSettingsDirty"
                                />
                                秒
                            </label>
                            <output class="aim-reminder-calculated-time">
                                <span>85%最佳时间</span>
                                <strong>{{ magnumAimCalculatedBestText() }}</strong>
                                <small>秒</small>
                            </output>
                            <label>
                                整体大小
                                <input
                                    v-model.number="skillCooldownSettings.aimReminder.scalePercent"
                                    type="number"
                                    min="50"
                                    max="200"
                                    step="5"
                                    @input="markAimReminderSettingsDirty"
                                />
                                %
                            </label>
                            <div class="aim-reminder-coordinates" title="以整个桌面左上角为 (0, 0)，坐标表示长条提示左上角">
                                <span>提示左上角</span>
                                <label>X <input v-model.number="skillCooldownSettings.aimReminder.x" type="number" min="-32000" max="32000" @input="markAimReminderSettingsDirty" /></label>
                                <label>Y <input v-model.number="skillCooldownSettings.aimReminder.y" type="number" min="-32000" max="32000" @input="markAimReminderSettingsDirty" /></label>
                            </div>
                            <button type="button" class="aim-reminder-preview" :disabled="!skillCooldownSettings.aimReminder.enabled" @click="previewAimReminder">
                                <v-icon icon="mdi-play-circle-outline" size="14" />预览进度与最佳提示
                            </button>
                        </div>
                        <p>合计射程＝武器基础射程＋鉴定等级×70。固定距离改版后的基础瞄准时间按 7×1000÷合计射程＋1 秒计算，再叠加瞄准校准、尔格与临时加速，得出系统 70%（画面显示 85%）的最佳射击时间；网络与画面延迟因人而异，请先预览或实战测试，再用“延迟微调”校正。无影箭、拉蒂卡秘术和疾速会自动识别并换算。</p>
                    </section>

                    <section class="skill-cooldown-settings" aria-label="技能冷却完成提醒设置">
                        <header>
                            <div>
                                <strong>技能 CD 好了提示</strong>
                                <span>每个技能可设置自己的屏幕坐标；螺旋爆裂与蓄势突击每次使用都会把一份短时 CD 加入累计 CD，达到上限后进入技能 CD。</span>
                            </div>
                            <div class="skill-cooldown-header-actions">
                                <label class="skill-cooldown-icon-size" for="skill-cooldown-icon-size">
                                    图标大小
                                    <input
                                        id="skill-cooldown-icon-size"
                                        v-model.number="skillCooldownSettings.iconSize"
                                        type="number"
                                        min="24"
                                        max="96"
                                        step="1"
                                        inputmode="numeric"
                                        aria-describedby="skill-cooldown-icon-size-unit"
                                        @input="markSkillCooldownSettingsDirty"
                                    />
                                    <span id="skill-cooldown-icon-size-unit">px</span>
                                </label>
                                <span v-if="skillCooldownSettingsDirty" class="skill-cooldown-unsaved">有未保存修改</span>
                                <button
                                    type="button"
                                    class="skill-cooldown-save"
                                    :class="{ 'needs-save': skillCooldownSettingsDirty, saved: skillCooldownSettingsSaved }"
                                    :disabled="skillCooldownSettingsSaving"
                                    @click="persistSkillCooldownSettings"
                                >
                                    <v-icon :icon="skillCooldownSettingsSaving ? 'mdi-loading' : skillCooldownSettingsSaved ? 'mdi-check' : 'mdi-content-save-outline'" :class="{ 'mdi-spin': skillCooldownSettingsSaving }" size="13" />
                                    {{ skillCooldownSettingsSaving ? "保存中" : skillCooldownSettingsSaved ? "已保存" : "保存设定" }}
                                </button>
                            </div>
                        </header>
                        <div class="buff-alert-picker skill-cooldown-picker">
                            <label for="skill-cooldown-target">搜索技能提醒</label>
                            <input
                                id="skill-cooldown-target"
                                v-model.trim="skillCooldownInput"
                                type="search"
                                placeholder="输入技能名称或技能 ID"
                                autocomplete="off"
                                @keyup.enter="addFirstSkillCooldownSearchResult"
                            />
                            <button type="button" class="buff-alert-add" :disabled="!skillCooldownSearchResults.length" @click="addFirstSkillCooldownSearchResult">
                                <v-icon icon="mdi-plus" size="13" />添加
                            </button>
                        </div>
                        <div v-if="skillCooldownInput && skillCooldownSearchResults.length" class="buff-alert-search-results skill-cooldown-search-results">
                            <button
                                v-for="skill in skillCooldownSearchResults"
                                :key="skill.id"
                                type="button"
                                :disabled="Boolean(skillCooldownSettings.rules[skill.id] && !skillCooldownSettings.rules[skill.id].barOnly)"
                                @click="addSkillCooldownRule(skill.id)"
                            >
                                <span class="skill-alert-icon compact">
                                    <span>技能</span>
                                    <img :src="skillIconUrl(skill.id)" :alt="`${skill.name}图标`" @error="hideMissingConditionIcon" />
                                </span>
                                <span><strong>{{ skill.name }}</strong><small>ID {{ skill.id }}</small></span>
                                <v-icon :icon="skillCooldownSettings.rules[skill.id] && !skillCooldownSettings.rules[skill.id].barOnly ? 'mdi-check' : 'mdi-plus'" size="13" />
                            </button>
                        </div>
                        <div v-if="configuredSkillCooldownRules.length" class="skill-cooldown-rule-list">
                            <div v-for="rule in configuredSkillCooldownRules" :key="rule.skillId" class="skill-cooldown-editor">
                                <div class="buff-alert-identity">
                                    <span class="skill-alert-icon compact">
                                        <span>技能</span>
                                        <img :src="skillIconUrl(rule.skillId)" :alt="`${skillDisplayName(rule.skillId)}图标`" @error="hideMissingConditionIcon" />
                                    </span>
                                    <strong>{{ skillDisplayName(rule.skillId) }}</strong>
                                    <small>ID {{ rule.skillId }}</small>
                                </div>
                                <p v-if="builtinSkillCooldownRuleDescription(rule.skillId)" class="skill-cooldown-builtin-rule">
                                    <v-icon icon="mdi-timer-sync-outline" size="14" />
                                    {{ builtinSkillCooldownRuleDescription(rule.skillId) }}检测到对应包信号时自动更新当前剩余 CD。
                                </p>
                                <label>
                                    <input v-model="rule.enabled" type="checkbox" @change="markSkillCooldownSettingsDirty" />
                                    启用提醒
                                </label>
                                <label v-if="rule.skillId === DORCHA_MASTERY_SKILL_ID">
                                    多尔卡少于
                                    <input v-model.number="rule.quantityThreshold" type="number" min="1" max="15" step="1" @change="markSkillCooldownSettingsDirty" />
                                    <span>点时弹框</span>
                                </label>
                                <label v-else-if="rule.skillId === TOAH_SPIRIT_SKILL_ID">
                                    提示进度
                                    <input
                                        v-model.number="rule.progressThresholdPercent"
                                        type="number"
                                        min="1"
                                        max="100"
                                        step="1"
                                        @change="markSkillCooldownSettingsDirty"
                                    />
                                    <span>%</span>
                                </label>
                                <div
                                    v-else-if="isCumulativeCooldownSkill(rule.skillId)"
                                    class="skill-cooldown-cumulative-fields"
                                    title="每次使用都把完整短时 CD 加入当前剩余的累计 CD；例如短时 CD 为 3 秒时，第一下为 3 秒，1 秒后第二下为 2+3=5 秒；累计值持续回落，达到上限后进入技能 CD"
                                >
                                    <label>短时 CD <input v-model.number="rule.shortCooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @change="markSkillCooldownSettingsDirty" /> 秒</label>
                                    <label>累计 CD <input v-model.number="rule.cumulativeCooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @change="markSkillCooldownSettingsDirty" /> 秒</label>
                                    <label>技能 CD <input v-model.number="rule.cooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @change="markSkillCooldownSettingsDirty" /> 秒</label>
                                </div>
                                <label v-else>
                                    技能 CD（秒）
                                    <input v-model.number="rule.cooldownSeconds" type="number" min="0.1" max="86400" step="0.1" @change="markSkillCooldownSettingsDirty" />
                                </label>
                                <label v-if="rule.skillId !== TOAH_SPIRIT_SKILL_ID && rule.skillId !== DORCHA_MASTERY_SKILL_ID" title="宠物技能不会被托亚灵满充刷新">
                                    技能归属
                                    <select v-model="rule.ownerMode" @change="markSkillCooldownSettingsDirty">
                                        <option value="auto">自动识别</option>
                                        <option value="player">角色技能</option>
                                        <option value="pet">宠物技能</option>
                                    </select>
                                </label>
                                <div class="skill-cooldown-coordinates" title="以整个桌面左上角为 (0, 0)，坐标表示技能图标左上角">
                                    <span>{{ rule.skillId === DORCHA_MASTERY_SKILL_ID ? "弹框左上角" : "图标左上角" }}</span>
                                    <label>X <input v-model.number="rule.x" type="number" min="-32000" max="32000" @input="markSkillCooldownSettingsDirty" /></label>
                                    <label>Y <input v-model.number="rule.y" type="number" min="-32000" max="32000" @input="markSkillCooldownSettingsDirty" /></label>
                                </div>
                                <label v-if="rule.skillId === DORCHA_MASTERY_SKILL_ID" title="同时缩放多尔卡弹框、文字和数字，默认 100%">
                                    窗口大小
                                    <input v-model.number="rule.scalePercent" aria-label="多尔卡精通窗口大小百分比" type="number" min="50" max="200" step="5" @input="markSkillCooldownSettingsDirty" />
                                    <span>%</span>
                                </label>
                                <label>
                                    {{ rule.skillId === DORCHA_MASTERY_SKILL_ID ? "提示音效" : "完成音效" }}
                                    <select v-model="rule.soundMode" @change="onSkillCooldownSoundModeChanged(rule)">
                                        <option value="default">默认提示音</option>
                                        <option value="none">不提示</option>
                                        <option value="custom">自定义音效</option>
                                    </select>
                                </label>
                                <label v-if="rule.soundMode === 'custom'" class="buff-alert-file-picker">
                                    <input
                                        type="file"
                                        accept=".mp3,.wav,audio/mpeg,audio/wav,audio/x-wav"
                                        @change="uploadCustomSkillCooldownSound(rule, $event)"
                                    />
                                    <span>{{ rule.customSoundName || "选择 MP3 / WAV" }}</span>
                                </label>
                                <button v-if="rule.soundMode === 'custom'" type="button" class="local-tts-open" @click="openLocalTTS('skill', rule.skillId, skillDisplayName(rule.skillId))">
                                    <v-icon icon="mdi-account-voice" size="13" />本地TTS
                                </button>
                                <button
                                    v-if="rule.soundMode !== 'none'"
                                    type="button"
                                    class="buff-alert-preview"
                                    :disabled="rule.soundMode === 'custom' && !rule.customSoundId"
                                    @click="previewSkillCooldownSound(rule, true)"
                                >试听</button>
                                <label :title="rule.skillId === DORCHA_MASTERY_SKILL_ID ? '始终显示当前多尔卡数量；未勾选时仅在数量不足时显示' : '勾选后，冷却期间灰色显示并显示倒计时；完成后恢复彩色'">
                                    <input v-model="rule.alwaysVisible" type="checkbox" @change="markSkillCooldownSettingsDirty" />
                                    {{ rule.skillId === DORCHA_MASTERY_SKILL_ID ? "数量一直显示" : rule.skillId === TOAH_SPIRIT_SKILL_ID ? "能量槽一直显示" : "技能图标一直显示" }}
                                </label>
                                <button type="button" class="buff-alert-preview" @click="previewSkillCooldown(rule.skillId)">预览位置与动画</button>
                                <button type="button" class="buff-alert-remove" @click="removeSkillCooldownRule(rule.skillId)">
                                    <v-icon icon="mdi-delete-outline" size="13" />删除
                                </button>
                                <span class="buff-alert-runtime-status">{{ skillCooldownRuntimeStatus(rule.skillId) }}</span>
                            </div>
                        </div>
                        <p v-else>尚未添加技能。先搜索技能，再填写该技能在你当前装备与状态下的实际 CD 秒数。</p>
                    </section>

                </div>
            </div>
        </section>

        <div v-if="localTtsDialogOpen" class="team-chart-backdrop local-tts-backdrop" @click.self="localTtsDialogOpen = false">
            <section class="local-tts-dialog" role="dialog" aria-modal="true" aria-labelledby="local-tts-title">
                <header>
                    <div><v-icon icon="mdi-account-voice" size="18" /><strong id="local-tts-title">Windows 本地 TTS</strong></div>
                    <button type="button" aria-label="关闭本地TTS" @click="localTtsDialogOpen = false"><v-icon icon="mdi-close-box-outline" size="18" /></button>
                </header>
                <label>
                    提醒文字
                    <textarea v-model="localTtsText" maxlength="120" rows="3" placeholder="输入要朗读的内容"></textarea>
                    <small>{{ Array.from(localTtsText).length }}/120</small>
                </label>
                <div class="local-tts-fields">
                    <label>
                        系统语音
                        <select v-model="localTtsVoice" :disabled="localTtsVoicesLoading || !localTtsVoices.length">
                            <option v-if="!localTtsVoices.length" value="">{{ localTtsVoicesLoading ? "正在读取…" : "未找到系统语音" }}</option>
                            <option v-for="voice in localTtsVoices" :key="voice.name" :value="voice.name">{{ voice.description || voice.name }}（{{ voice.culture }}）</option>
                        </select>
                        <small>使用 Windows 已安装语音</small>
                    </label>
                    <label>
                        语速
                        <input v-model.number="localTtsRate" type="number" min="-5" max="5" step="1" />
                        <small>-5 慢　0 正常　5 快</small>
                    </label>
                </div>
                <div v-if="localTtsStatus" class="local-tts-status" :class="{ error: localTtsError }">{{ localTtsStatus }}</div>
                <footer>
                    <span>应用目标：{{ localTtsTarget?.label || "—" }}</span>
                    <button type="button" class="game-button" @click="localTtsDialogOpen = false">取消</button>
                    <button type="button" class="game-button confirm" :disabled="localTtsPending || !localTtsText.trim() || !localTtsVoice" @click="generateAndApplyLocalTTS">
                        <v-icon icon="mdi-play-circle-outline" size="14" />{{ localTtsPending ? "生成中…" : "生成、应用并试听" }}
                    </button>
                </footer>
            </section>
        </div>

        <div v-if="teamChartOpen" class="team-chart-backdrop" @click.self="teamChartOpen = false">
            <section class="team-chart-dialog" role="dialog" aria-modal="true" aria-labelledby="team-chart-title">
                <header class="team-chart-titlebar">
                    <div>
                        <v-icon icon="mdi-chart-areaspline" size="17" />
                        <span id="team-chart-title">团队绘图</span>
                    </div>
                    <button type="button" aria-label="关闭团队绘图" @click="teamChartOpen = false">
                        <v-icon icon="mdi-close-box-outline" size="18" />
                    </button>
                </header>
                <div class="team-chart-toolbar">
                    <div class="team-chart-context">
                        <strong>{{ bossLabel }}</strong>
                        <span>{{ reportSession ? `${fmtBattleClock(reportSession.startAt)} ~ ${fmtBattleClock(reportSession.endAt)}` : "尚无时间范围" }}</span>
                    </div>
                    <div class="team-chart-view-switch" role="radiogroup" aria-label="团队绘图类型">
                        <button type="button" role="radio" :aria-checked="teamChartView === 'damage'" :class="{ active: teamChartView === 'damage' }" @click="teamChartView = 'damage'">
                            <v-icon icon="mdi-chart-areaspline" size="13" />累计输出
                        </button>
                        <button type="button" role="radio" :aria-checked="teamChartView === 'dps'" :class="{ active: teamChartView === 'dps' }" @click="teamChartView = 'dps'">
                            <v-icon icon="mdi-chart-line" size="13" />实时 DPS
                        </button>
                        <button type="button" role="radio" :aria-checked="teamChartView === 'skills'" :class="{ active: teamChartView === 'skills' }" @click="teamChartView = 'skills'">
                            <v-icon icon="mdi-timeline-clock-outline" size="13" />技能时间轴
                        </button>
                    </div>
                    <span class="team-chart-privacy"><v-icon icon="mdi-shield-account-outline" size="14" />默认仅显示阿尔卡纳职业</span>
                    <button
                        type="button"
                        class="game-button"
                        :class="{ active: teamUseIds }"
                        @click="requestTeamIdentityToggle"
                    >{{ teamUseIds ? "隐藏队员" : "显示队员" }}</button>
                </div>
                <TeamCumulativeChart
                    v-if="teamChartView !== 'skills' && reportSession && teamChartPlayers.length"
                    :mode="teamChartView"
                    :players="teamChartPlayers"
                    :start-at="reportSession.startAt"
                    :end-at="reportSession.endAt"
                />
                <TeamSkillTimeline
                    v-else-if="teamChartView === 'skills' && reportSession && teamSkillTimelinePlayers.length"
                    :players="teamSkillTimelinePlayers"
                    :personal-player-id="selectedPlayerId"
                    :allow-player-selection="teamUseIds"
                    :start-at="reportSession.startAt"
                    :end-at="reportSession.endAt"
                    @update:personal-player-id="selectedPlayerId = $event"
                />
                <div v-else class="team-chart-empty">尚无可绘制的团队{{ teamChartView === "skills" ? "技能" : "伤害" }}记录。</div>
                <footer>
                    <span>{{ teamChartView === "damage"
                        ? "折线斜率反映阶段输出速度；各层高度叠加后，顶部即全团累计伤害。"
                        : teamChartView === "dps" ? "粗线为全团 DPS，彩色细线为各成员 DPS；悬停查看该时间段的数值，拖选可放大，点击图例可隐藏曲线。"
                        : "按所选队员的技能总伤害分轨；打开“显示队员”后可切换查看其他队员。这里只展示实际造成伤害的施放时间。" }}</span>
                    <button type="button" class="game-button" @click="teamChartOpen = false">关闭</button>
                </footer>
            </section>
        </div>

        <div v-if="teamIdentityWarningOpen" class="team-chart-backdrop privacy-warning-backdrop" @click.self="teamIdentityWarningOpen = false">
            <section class="team-privacy-dialog" role="alertdialog" aria-modal="true" aria-labelledby="team-privacy-title">
                <header><v-icon icon="mdi-account-alert-outline" size="24" /><strong id="team-privacy-title">显示队员前请确认</strong></header>
                <p>队员角色名属于队友的识别信息。生成或分享带角色名的团队图表前，请征得队友同意。</p>
                <div>
                    <button type="button" class="game-button" @click="teamIdentityWarningOpen = false">取消</button>
                    <button type="button" class="game-button confirm" @click="confirmTeamIdentityDisplay">我已征得同意，显示队员</button>
                </div>
            </section>
        </div>
    </div>
</template>

<script setup lang="ts">
import { computed, inject, onMounted, onUnmounted, ref, shallowRef, toRefs, watch, type Ref } from "vue";
import type { DamageCollectorManager } from "@/actionCollector";
import {
    BATTLE_RECORD_LOADED_EVENT,
    battleRecordFilename,
    createBattleRecord,
    type BattleRecordSelection,
} from "@/battleRecord";
import type { ActorManager, EntityActor, EntityCondition, EntityDamage } from "@/eventActor";
import TeamCumulativeChart, { type TeamChartPlayer } from "@/components/TeamCumulativeChart.vue";
import TeamSkillTimeline, {
    type SkillTimelinePlayer,
    type SkillTimelineUse,
} from "@/components/TeamSkillTimeline.vue";
import { findLocalBuffCondition } from "@/buffOwnership";
import { isMarionetteDamageSkill } from "@/ownedDamagePolicy";
import {
    buildBossSummary,
    type BossSummary,
    type BossFightSession,
    type PlayerSummary,
    type SkillStat,
} from "@/summaryCollector";
import { getDisplayName, resourceNameVersion } from "@/store";
import type { LiveBattleCatalog, LiveBattleTarget } from "@/liveBattles";
import { BOSS_NAME_OVERRIDES, bossDisplayName } from "@/bossDisplay";
import {
    DORCHA_MASTERY_SKILL_ID, DORCHA_STAT_ID, normalizeDorchaThreshold,
    formatDorchaQuantity, initialDorchaQuantityRuntime, applyDorchaQuantityObservation, dorchaQuantityOverlayItem,
} from "@/dorcha";
import { normalizeSkillDisplayName } from "@/skillDisplay";
import { eventIdCharacterConditionDisable, eventIdCharacterConditionEnable, eventIdSkillAction, type eventCharacterConditionDisable, type eventCharacterConditionEnable, type eventDamage, type eventSkillAction, type eventSkillCooldown, type eventSkillState } from "@/protocols";
import { CURRENT_HEALTH_STAT_ID, MAXIMUM_HEALTH_STAT_ID } from "@/effectiveDamage";
import { conditions as fallbackConditionNames } from "@/data/condition";
import { skills as fallbackSkillNames } from "@/data/skill";
import {
    BUFF_OVERLAY_CHANNEL,
    isStackOnlyBuffAlertCondition,
    loadBuffOverlaySettings,
    makeBuffAlertRule,
    parseConditionStack,
    resolveBuffExpiresAt,
    saveBuffOverlaySettings,
    type BuffAlertRule,
    type BuffSoundMode,
    type BuffOverlayItem,
    type BuffOverlayMessage,
} from "@/buffAlert";
import {
    DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
    DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS,
    DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
    DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
    DEFAULT_MAGNUM_WEAPON_RANGE,
    DEFAULT_TOAH_PROGRESS_THRESHOLD,
    FINAL_SHOT_SKILL_ID,
    LATIKA_SECRET_CC_ID,
    MAGNUM_RANGE_PER_IDENTIFICATION_LEVEL,
    MAGNUM_SHOT_SKILL_ID,
    RAPID_AIM_CC_ID,
    SKILL_ACTION_EVENT,
    SKILL_COOLDOWN_ADJUSTMENT_EVENT,
    SKILL_DAMAGE_EVENT,
    SKILL_STATE_EVENT,
    TOAH_SPIRIT_SKILL_ID,
    TOAH_SPIRIT_STAT_ID,
    applyToahSpiritProgressObservation,
    applySkillCooldownAdjustment,
    applySkillCooldownObservation,
    builtinSkillCooldownRuleDescription,
    calculateMagnumAimDisplayProgress,
    calculateMagnumAimTiming,
    initialToahSpiritProgressRuntime,
    isCumulativeCooldownSkill,
    isLocalCooldownDamage,
    SKILL_COOLDOWN_CHANNEL,
    loadSkillCooldownSettings,
    makeSkillCooldownRule,
    saveSkillCooldownSettings,
    settleSkillCooldownRuntime,
    shouldRefreshSkillCooldownFromToah,
    summarizeMagnumAimSamples,
    type MagnumAimSample,
    type SkillCooldownRule,
    type SkillCooldownOverlayMessage,
    type SkillCooldownRuntime,
    type TargetHealthBarOverlayItem,
    type ToahSpiritProgressRuntime,
} from "@/skillCooldown";
import { loadSkillBarSettings } from "@/skillBar";
import {
    EFFECT_TIMER_CONDITION_EVENT,
    loadEffectTimerSettings,
    makeEffectTimerRule,
    normalizeEffectTimerSettings,
    saveEffectTimerSettings,
    type EffectTimerRule,
    type EffectTimerRuntime,
} from "@/effectTimer";
import { buildConditionTimelineScale } from "@/conditionTimeline";
import { formatBattleTargetTimeRange } from "@/battleTargetTime";
import {
    lockBattleTargetSelection,
    reconcileBattleTargetSelection,
    selectDamageBattleTarget,
} from "@/battleTargetSelection";
import {
    cloneReminderSnapshot,
    loadReminderProfileStore,
    makeReminderProfile,
    saveReminderProfileStore,
    type ReminderProfileSnapshot,
} from "@/reminderProfiles";
import {
    canonicalDebuffId,
    DEFAULT_DEBUFF_ALERT_IDS,
    DEBUFF_SOUND_MERGE_COOLDOWN_MS,
    DEBUFF_OVERLAY_CHANNEL,
    equivalentDebuffDisplayName,
    equivalentDebuffIds,
    evaluateDebuffAlert,
    loadDebuffAlertSettings,
    makeDebuffAlertRule,
    saveDebuffAlertSettings,
    selectDebuffBossCandidate,
    selectDebuffSoundCandidate,
    type DebuffAlertRule,
    type DebuffOverlayItem,
    type DebuffOverlayMessage,
} from "@/debuffAlert";
import { selectMainReportPlayer } from "@/localPlayerSelection";
import {
    advanceMielOrbRuntime,
    BOSS_MECHANIC_EVENT,
    createMielOrbRuntime,
    isMielOrbVisible,
    loadBossMechanicAlertSettings,
    normalizeBossMechanicAlertSettings,
    observeMielOrbContact,
    observeMielOrbLateConfirmation,
    saveBossMechanicAlertSettings,
    type BossMechanicAlertRule,
    type MielOrbRuntime,
} from "@/bossMechanicAlert";

const props = withDefaults(defineProps<{
    isFileLoading?: boolean;
    isStandalone?: boolean;
    dpsVisible?: boolean;
    battleCatalog?: LiveBattleCatalog | null;
    loadedSessionKey?: string;
    isRecordReplay?: boolean;
}>(), {
    isFileLoading: false,
    isStandalone: false,
    dpsVisible: true,
    battleCatalog: null,
    loadedSessionKey: "",
    isRecordReplay: false,
});
const { isFileLoading, isStandalone, dpsVisible, isRecordReplay } = toRefs(props);
const emit = defineEmits<{
    (event: "refresh-info"): void;
    (event: "view-records"): void;
    (event: "clear-data"): void;
    (event: "select-archived-session", target: LiveBattleTarget): void;
    (event: "return-live"): void;
}>();

type NoticeType = "success" | "warning" | "error" | "info";
type LocalTtsTargetKind = "buff" | "buff-stack" | "debuff" | "skill" | "boss";
type LocalTtsTarget = { kind: LocalTtsTargetKind; id: number | string; label: string };
type LocalTtsVoice = { name: string; culture: string; gender: string; description: string };
type CombatTab = "summary" | "attack" | "alerts";
type BossMechanicRuntimeState = {
    startedAtMs: number;
    endsAtMs: number;
    generation: number;
    bossId: string;
    bossRaceId: number;
    mielOrb?: MielOrbRuntime;
};
type ConditionTarget = "ally" | "monster";
type SkillRow = SkillStat & {
    name: string;
    ratio: number;
    dps: number;
    maxDamage: number;
    barRatio: number;
    iconUrl: string;
};
type ConditionSegment = {
    start: number;
    end: number;
};
type ConditionCoverageRow = {
    entityId: string;
    ccId: number;
    name: string;
    iconUrl: string;
    coverage: number;
    activeSeconds: number;
    segments: ConditionSegment[];
};
type PlayerBuffDefinition = {
    id: number;
    name: string;
    detail?: string;
};
type PlayerBuffGroup = {
    name: string;
    items: PlayerBuffDefinition[];
};
type PlayerConditionGroup = {
    name: string;
    rows: ConditionCoverageRow[];
    activeCount: number;
};

const actorManager = inject<Ref<ActorManager>>("actorManager")!;
const dcManager = inject<Ref<DamageCollectorManager>>("dcManager")!;
const appEvent = inject<Ref<EventTarget>>("appEvent")!;
const raceNameMap = inject<Ref<Record<number, string>>>("raceNameMap")!;
const skillNameMap = inject<Ref<Record<number, string>>>("skillNameMap")!;
const condNameMap = inject<Ref<Record<number, string>>>("condNameMap")!;
const region = inject<Ref<string>>("region")!;

const PLAYER_BUFF_STORAGE_KEY = "dilmetercn.playerBuffIds.v1";
const PLAYER_BUFF_FAVORITES_STORAGE_KEY = "dilmetercn.playerBuffFavoriteIds.v1";
const ONLY_BOSS_TARGETS_STORAGE_KEY = "dilmetercn.onlyBossTargets.v1";
const HIDDEN_PLAYER_BUFF_IDS = new Set([951]);
const PLAYER_BUFF_ID_MIGRATIONS = new Map([
    [906, 1150],
    [951, 1159],
]);
const FALLBACK_CONDITION_NAME_MAP = fallbackConditionNames as Record<string, string>;
const FALLBACK_SKILL_NAME_MAP = fallbackSkillNames as Record<string, string>;
const CONDITION_DISPLAY_NAME_OVERRIDES: Record<number, string> = {
    392: "闪电风暴 / 雷霆咆哮",
    504: "闪电风暴 / 雷霆咆哮",
    511: "状态支援：锐利",
    512: "状态支援：再生",
    513: "状态支援：迅速",
    514: "状态支援：隐身",
    515: "状态支援：逆转",
    912: "喵咪的奔袭",
    913: "喵咪的奔袭",
    1080: "米耶尔的羽毛",
    1098: "湛蓝内伤",
    1153: "分裂的能量",
};
const STACK_ALERT_CC_IDS = new Set([1080, 1098, 1153]);
const BASE_PLAYER_BUFF_GROUPS: PlayerBuffGroup[] = [
    {
        name: "音乐类",
        items: [
            { id: 680, name: "战争序曲" },
            { id: 192, name: "活跃进行曲" },
            { id: 193, name: "行进曲" },
        ],
    },
    {
        name: "药水类",
        items: [
            { id: 63, name: "攻击力增加" },
            { id: 177, name: "魔法攻击强化" },
            { id: 1150, name: "炼金术伤害增加" },
        ],
    },
    {
        name: "技能类",
        items: [
            { id: 1120, name: "赐予自然力", detail: "元素骑士" },
            { id: 874, name: "生命的温度", detail: "圣光颂唱者" },
            { id: 800, name: "活力之歌", detail: "圣光颂唱者" },
            { id: 796, name: "守护祝福", detail: "圣光颂唱者" },
            { id: 938, name: "魔法穿刺", detail: "黑魔导士" },
            { id: 887, name: "远距离战术系列攻击伤害增加", detail: "流星弓手" },
            { id: 1123, name: "负载转移", detail: "圣盾骑士 / 枪炮师" },
            { id: 975, name: "骑士枪冲刺骑乘效果(未骑乘)", detail: "爆裂骑士枪" },
            { id: 1159, name: "斗士之怒", detail: "狂怒斗士" },
            { id: 1144, name: "催化协同", detail: "禁术炼金术师；由实际记录判断作用对象" },
            { id: 1145, name: "催化协同", detail: "禁术炼金术师；由实际记录判断作用对象" },
            { id: 1146, name: "元素融合", detail: "禁术炼金术师" },
            { id: 1161, name: "灵线织网", detail: "旋律操纵师" },
            { id: 1033, name: "星星", detail: "占星术" },
            { id: 1023, name: "月亮", detail: "占星术" },
            { id: 1016, name: "物理伤害增加", detail: "占星术" },
            { id: 1045, name: "魔法伤害增加", detail: "占星术" },
            { id: 1046, name: "炼金术伤害增加", detail: "占星术" },
        ],
    },
    {
        name: "特技类",
        items: [
            { id: 511, name: "状态支援：锐利" },
            { id: 512, name: "状态支援：再生" },
            { id: 513, name: "状态支援：迅速" },
            { id: 514, name: "状态支援：隐身" },
            { id: 515, name: "状态支援：逆转" },
            { id: 479, name: "坚定意志" },
            { id: 476, name: "致命穿透" },
            { id: 612, name: "刻印：逆光剑" },
        ],
    },
    {
        name: "套装类",
        items: [
            { id: 1035, name: "日" },
            { id: 1036, name: "月" },
            { id: 1037, name: "月蚀" },
            { id: 891, name: "不屈" },
            { id: 978, name: "执行" },
            { id: 934, name: "祈祷" },
        ],
    },
    {
        name: "宠物类",
        items: [
            { id: 1225, name: "超燃咚咚" },
            { id: 645, name: "净化之浪" },
            { id: 914, name: "喵咪的恩赐 914" },
            { id: 915, name: "喵咪的恩赐 915" },
            { id: 717, name: "狐克斯的幸运" },
        ],
    },
    {
        name: "种族技",
        items: [
            { id: 406, name: "拉蒂卡瞬步" },
            { id: 407, name: "拉蒂卡秘术" },
            { id: 408, name: "拉蒂卡冲击" },
        ],
    },
];

const previewParams = new URLSearchParams(window.location.search);
const isDesignPreview = import.meta.env.DEV && previewParams.has("preview");
const showPreviewControls = isDesignPreview && previewParams.has("controls");
const isSharePreview = isDesignPreview && previewParams.has("share");
const previewAimSummaryEnabled = isDesignPreview && previewParams.get("aimSummary") === "1";
const sharePreviewUrl = ref("");
const selectedBossId = ref("");
const manuallyLockedBossId = ref("");
const onlyBossTargets = ref(loadOnlyBossTargets());
const reminderTargetId = ref("");
const selectedPlayerId = ref("");
const activeTab = ref<CombatTab>("attack");
const conditionTarget = ref<ConditionTarget>("ally");
const selectedConditionId = ref<number | null>(null);
const conditionTimelineElement = ref<HTMLElement | null>(null);
const conditionTimelineWidth = ref(900);
const playerBuffSettingsOpen = ref(false);
const playerBuffInput = ref("");
const playerBuffInputError = ref("");
const storedPlayerBuffIds = loadStoredPlayerBuffIds();
const playerBuffUsesDefaults = ref(storedPlayerBuffIds === null);
const playerBuffIds = ref<number[]>(storedPlayerBuffIds ?? []);
const playerBuffFavoriteIds = ref<number[]>(loadStoredPlayerBuffFavoriteIds());
const buffOverlaySettings = ref(loadBuffOverlaySettings());
const buffAlertSettingsDirty = ref(false);
const debuffAlertSettings = ref(loadDebuffAlertSettings());
const debuffAlertSettingsDirty = ref(false);
const skillCooldownSettings = ref(loadSkillCooldownSettings());
const magnumRangeIdentificationOptions = Array.from({ length: 21 }, (_, level) => ({
    level,
    bonus: level * MAGNUM_RANGE_PER_IDENTIFICATION_LEVEL,
}));
const skillCooldownRuntime = ref<Record<number, SkillCooldownRuntime>>({});
const magnumAimPreview = ref<{
    active: boolean;
    startedAtMs: number;
    readyAtMs: number;
    speedMultiplier: number;
    buffNames: string[];
    calibrationPercent: number;
    generation: number;
} | null>(null);
type MagnumAimCycle = {
    startedAtMs: number;
    readyAtMs: number;
    calibrationPercent: number;
    targetId: string;
    endedAtMs?: number;
};
const activeMagnumAimCycle = ref<MagnumAimCycle | null>(null);
const recentMagnumAimCycle = ref<MagnumAimCycle | null>(null);
const magnumAimSamples = ref<MagnumAimSample[]>([]);
const finalShotActive = ref(false);
let lastMagnumAimActionKey = "";
const toahSpiritProgressRuntime = ref<ToahSpiritProgressRuntime>(initialToahSpiritProgressRuntime());
const dorchaQuantityRuntime = ref(initialDorchaQuantityRuntime());
let dorchaPreviewUntilMs = 0;
const bossMechanicSettings = ref(loadBossMechanicAlertSettings());
const bossMechanicSettingsDirty = ref(false);
const bossMechanicSettingsSaved = ref(false);
const bossMechanicRuntime = ref<Record<string, BossMechanicRuntimeState>>({});
const mielShardHealthBarPreview = ref<{ endsAtMs: number; generation: number } | null>(null);
const effectTimerSettings = ref(loadEffectTimerSettings());
const effectTimerRuntime = ref<Record<string, EffectTimerRuntime>>({});
const effectTimerSettingsDirty = ref(false);
const effectTimerSettingsSaved = ref(false);
const buffStackAlertRuntime = ref<Record<number, { stack: number; startedAtMs: number; endsAtMs: number; generation: number }>>({});
const skillCooldownInput = ref("");
const skillCooldownSettingsDirty = ref(false);
const skillCooldownSettingsSaved = ref(false);
const skillCooldownSettingsSaving = ref(false);
const aimReminderSettingsDirty = ref(false);
const aimReminderSettingsSaved = ref(false);
const aimReminderSettingsSaving = ref(false);
const reminderProfileSettingsDirty = ref(false);
const skillCooldownClock = ref(Date.now());
const activeBuffAudio = new Set<HTMLAudioElement>();
const buffAlertInput = ref("");
const debuffAlertInput = ref("");
const debuffBossRaceInput = ref("");
const reminderProfileStore = ref(loadReminderProfileStore({
    buffRules: buffOverlaySettings.value.rules,
    debuffSettings: debuffAlertSettings.value,
    aimReminder: skillCooldownSettings.value.aimReminder,
    skillRules: skillCooldownSettings.value.rules,
    bossMechanicSettings: bossMechanicSettings.value,
    effectTimerSettings: effectTimerSettings.value,
    playerBuffIds: playerBuffIds.value,
    playerBuffFavoriteIds: playerBuffFavoriteIds.value,
    playerBuffUsesDefaults: playerBuffUsesDefaults.value,
}));
const selectedReminderProfileId = ref(reminderProfileStore.value.activeProfileId);
const notice = ref("");
const noticeType = ref<NoticeType>("success");
const localTtsDialogOpen = ref(false);
const localTtsVoices = ref<LocalTtsVoice[]>([]);
const localTtsVoicesLoading = ref(false);
const localTtsText = ref("");
const localTtsVoice = ref("");
const localTtsRate = ref(0);
const localTtsPending = ref(false);
const localTtsStatus = ref("");
const localTtsError = ref(false);
const localTtsTarget = ref<LocalTtsTarget | null>(null);
const teamChartOpen = ref(false);
const teamChartView = ref<"damage" | "dps" | "skills">("damage");
const teamUseIds = ref(false);
const teamIdentityWarningOpen = ref(false);
let buffOverlayChannel: BroadcastChannel | undefined;
let debuffOverlayChannel: BroadcastChannel | undefined;
let skillOverlayChannel: BroadcastChannel | undefined;
let buffOverlayTimer: number | undefined;
let nativeReminderDragTimer: number | undefined;
let nativeReminderDragSequence = 0;
let nativeReminderTickListener: EventListener | undefined;
let lastReminderPublishAtMs = 0;
let buffOverlayStateKey = "";
let debuffOverlayStateKey = "";
let skillOverlayStateKey = "";
let bossMechanicSavedTimer: number | undefined;
let mielShardHealthBarPreviewTimer: number | undefined;
let effectTimerSavedTimer: number | undefined;
let skillCooldownSavedTimer: number | undefined;
let aimReminderSavedTimer: number | undefined;
let magnumAimPreviewTimer: number | undefined;
let toahSpiritPreviewUntilMs = 0;
const announcedBuffSounds = new Set<string>();
const buffStackAboveThreshold = new Map<number, boolean>();
const buffStackMissingSince = new Map<number, number>();
const announcedDebuffSounds = new Set<string>();
const observedDebuffApplications = new Set<string>();
let debuffSoundPlaying = false;
let debuffSoundBlockedUntilMs = 0;
let nativeReminderSettingsSyncTimer: number | undefined;
let nativeReminderSettingsSyncQueue: Promise<void> = Promise.resolve();
let summaryRefreshTimer: number | undefined;
let lastSummaryEventVersion = -1;
const announcedSkillReadySounds = new Set<string>();
const recentBossMechanicAt = new Map<string, number>();
const SKILL_READY_SOUND_WINDOW_MS = 1400;
let conditionTimelineResizeObserver: ResizeObserver | undefined;
const combatTabs: Array<{ id: CombatTab; label: string }> = [
    { id: "summary", label: "综合" },
    { id: "attack", label: "攻击技能" },
    { id: "alerts", label: "提醒设置" },
];
const visibleCombatTabs = computed(() => dpsVisible.value
    ? combatTabs
    : combatTabs.filter((tab) => tab.id === "alerts"));
const PREVIEW_BOSS_TOTAL_DAMAGE = 1_037_202_447;

function loadOnlyBossTargets(): boolean {
    try { return localStorage.getItem(ONLY_BOSS_TARGETS_STORAGE_KEY) !== "false"; }
    catch { return true; }
}

function saveOnlyBossTargets(enabled: boolean): void {
    try { localStorage.setItem(ONLY_BOSS_TARGETS_STORAGE_KEY, enabled ? "true" : "false"); }
    catch { /* keep the current session preference */ }
}

const previewSession: BossFightSession = {
    bossEntityId: "preview-boss",
    startAt: 1784640000,
    endAt: 1784640158,
    totalDuration: 158,
    invincibleIntervals: [],
    effectiveDuration: 158,
    inactiveDuration: 0,
};

const previewPlayer: PlayerSummary = {
    entityId: "preview-player",
    name: "示例角色",
    raceId: 10001,
    totalDamage: 392_535_875,
    totalDPS: 2_484_404,
    effectiveDPS: 2_484_404,
    individualEffectiveTime: 158,
    individualEffectiveDPS: 2_484_404,
    critRate: 0.557,
    passiveDamageRatio: 0.042,
    activeDamageRatio: 0.958,
    jobName: "黑魔导士",
    weapons: [],
    skillStats: [
        previewSkill(59040, "火焰·闪电", 178_440_000, 394, 394, 215, 164_082_310),
        previewSkill(59041, "闪电链", 63_971_109, 203, 203, 113, 6_070_100),
        previewSkill(59042, "爆裂箭", 61_449_777, 14, 14, 10, 6_568_103),
        previewSkill(59060, "烈焰旋涡", 47_087_170, 5, 5, 4, 11_736_047),
        previewSkill(59061, "冰雪风暴", 22_385_594, 3, 3, 3, 11_421_533),
        previewSkill(59064, "耀斑", 19_187_891, 41, 41, 0, 3_415_024),
        previewSkill(58009, "连击", 7_338, 7, 7, 3, 2_599, true),
        previewSkill(58100, "轰击", 186, 1, 1, 1, 186, true),
        previewSkill(58101, "爆闪", 2, 2, 2, 0, 1, true),
    ],
};

function previewSkill(
    skillId: number,
    name: string,
    totalDamage: number,
    useCount: number,
    totalHits: number,
    critHits: number,
    maxDamage: number,
    noCritRate = false,
): SkillStat & { previewName: string } {
    return {
        skillId,
        previewName: name,
        useCount,
        usesPerMinute: 0,
        totalDamage,
        critHits,
        totalHits,
        hitsPerUse: useCount > 0 ? totalHits / useCount : 0,
        damagePerUse: totalHits > 0 ? totalDamage / totalHits : 0,
        critRate: totalHits > 0 ? critHits / totalHits : 0,
        noCritRate,
        maxCritDamage: maxDamage,
        minCritDamage: null,
        maxNonCritDamage: Math.round(maxDamage * 0.7),
        minNonCritDamage: null,
        triggerSources: [],
    };
}

const bossOptions = computed(() => {
    resourceNameVersion.value;
    const forcedBossRaceIds = new Set(debuffAlertSettings.value.forcedBossRaceIds);
    const items = Object.values(actorManager.value.entityMap)
        .filter((entity) => !entity.isPC && (
            entity.totalTakeDamage > 0 || entity.id === reminderTargetId.value || forcedBossRaceIds.has(entity.raceId)
        ))
        .map((entity) => {
            // EntityActor already maintains the target's accumulated damage.
            // Re-scanning the complete session damage history whenever any
            // actor changes made long sessions progressively more expensive.
            const totalDamage = entity.totalTakeDamage;
            const maximumHealth = Number(entity.statMap?.[30]);
            const trueMaximumHealth = Number.isFinite(maximumHealth) && maximumHealth > 0
                ? maximumHealth
                : 0;
            const estimatedHealth = trueMaximumHealth || totalDamage;
            const firstDamageAt = entity.takeDamages[0]?.At ?? 0;
            const lastDamageAt = entity.takeDamages.at(-1)?.At ?? 0;
            const battleTime = firstDamageAt > 0 && lastDamageAt >= firstDamageAt
                ? formatBattleTargetTimeRange({ startAt: firstDamageAt, endAt: lastDamageAt })
                : "";
            return {
                entityId: entity.id,
                raceId: entity.raceId,
                totalDamage,
                estimatedHealth,
                lastDamageAt,
                active: actorManager.value.activeEntityMap[entity.id] !== false && !entity.finisherId,
                label: dpsVisible.value
                    ? `${bossName(entity)} · ${trueMaximumHealth > 0 ? fmtBossHealth(trueMaximumHealth) : "血量未知"}${battleTime ? ` · ${battleTime}` : ""}`
                    : `${bossName(entity)}${battleTime ? ` · ${battleTime}` : ""}`,
            };
        })
        .filter((entity) => !onlyBossTargets.value
            || entity.estimatedHealth >= 100_000_000
            || forcedBossRaceIds.has(entity.raceId))
        .sort((a, b) => b.estimatedHealth - a.estimatedHealth || b.lastDamageAt - a.lastDamageAt);
    return items.length || !isDesignPreview
        ? items
        : [{
            entityId: "preview-boss",
            raceId: 7603,
            totalDamage: PREVIEW_BOSS_TOTAL_DAMAGE,
            estimatedHealth: PREVIEW_BOSS_TOTAL_DAMAGE,
            lastDamageAt: previewSession.endAt,
            active: true,
            label: `样例首领 · 3.93亿 · ${formatBattleTargetTimeRange(previewSession)}`,
        }];
});

const archivedBossOptions = computed(() => {
    resourceNameVersion.value;
    const forced = new Set(debuffAlertSettings.value.forcedBossRaceIds);
    return (props.battleCatalog?.targets ?? [])
        .filter((target) => target.sessionKey !== props.loadedSessionKey)
        .filter((target) => !onlyBossTargets.value || forced.has(target.raceId)
            || Math.max(target.maximumHealth, target.totalDamage) >= 100_000_000)
        .map((target) => ({
            target,
            value: `history:${target.sessionKey}:${target.entityId}`,
            label: `${bossName(target)} · ${formatBattleTargetTimeRange({ startAt: target.startedAt, endAt: target.endedAt })}`,
        }));
});

const debuffBossTarget = computed(() => selectDebuffBossCandidate(
    bossOptions.value,
    selectedBossId.value,
    undefined,
    debuffAlertSettings.value.forcedBossRaceIds,
));
const nativeBossRaceNameKey = computed(() => [...new Set(bossOptions.value.map((boss) => boss.raceId))]
    .sort((a, b) => a - b)
    .map((raceId) => `${raceId}:${debuffBossRaceDisplayName(raceId)}`)
    .join("|"));

watch(onlyBossTargets, (enabled) => saveOnlyBossTargets(enabled));
watch(resourceNameVersion, () => scheduleNativeReminderSettingsSync());
watch(nativeBossRaceNameKey, () => scheduleNativeReminderSettingsSync());

watch(dpsVisible, (visible) => {
    if (!visible) activeTab.value = "alerts";
}, { immediate: true });

watch(bossOptions, (items) => {
    const next = reconcileBattleTargetSelection({
        selectedId: selectedBossId.value,
        manuallyLockedId: manuallyLockedBossId.value,
    }, items);
    manuallyLockedBossId.value = next.manuallyLockedId;
    selectedBossId.value = next.selectedId;
}, { immediate: true });

function handleBattleTargetSelected(event: Event) {
    const targetId = (event.currentTarget as HTMLSelectElement | null)?.value ?? "";
    if (targetId === "live:") {
        (event.currentTarget as HTMLSelectElement).value = selectedBossId.value;
        emit("return-live");
        return;
    }
    const archived = archivedBossOptions.value.find((item) => item.value === targetId);
    if (archived) {
        (event.currentTarget as HTMLSelectElement).value = selectedBossId.value;
        emit("select-archived-session", archived.target);
        return;
    }
    const next = lockBattleTargetSelection(targetId);
    manuallyLockedBossId.value = next.manuallyLockedId;
    selectedBossId.value = next.selectedId;
}

watch(selectedBossId, (next, previous) => {
    scheduleNativeReminderSettingsSync();
    if (!previous) return;
    // Selecting another report target can happen when a mechanism entity
    // leaves the client's area of interest.  It is not proof that the active
    // red orb disappeared, so Boss mechanism lifecycles must survive it.
    // A newly selected 7603/7615 Boss entity with a different id, however,
    // identifies a retry/phase replacement and starts a fresh lifecycle.
    const activeOrb = bossMechanicRuntime.value["miel-orb"];
    const nextBoss = next ? actorManager.value.entityMap[next] as EntityActor | undefined : undefined;
    if (activeOrb?.bossId
        && nextBoss
        && [7603, 7615].includes(Number(nextBoss.raceId))
        && nextBoss.id !== activeOrb.bossId) {
        delete bossMechanicRuntime.value["miel-orb"];
        recentBossMechanicAt.delete("miel-orb");
    }
    announcedDebuffSounds.clear();
    observedDebuffApplications.clear();
    publishDebuffOverlayState();
    publishSkillCooldownOverlayState();
});

const summary = shallowRef<BossSummary | null>(null);

function refreshBossSummary(force = false) {
    if (!dpsVisible.value || !selectedBossId.value || selectedBossId.value === "preview-boss") {
        summary.value = null;
        lastSummaryEventVersion = actorManager.value.eventVersion;
        return;
    }
    if (document.hidden || isFileLoading.value || activeTab.value === "alerts") return;
    const revision = actorManager.value.eventVersion;
    if (!force && revision === lastSummaryEventVersion) return;
    summary.value = buildBossSummary(selectedBossId.value, actorManager.value, []);
    lastSummaryEventVersion = revision;
}

watch([selectedBossId, dpsVisible, activeTab, isFileLoading], () => refreshBossSummary(true), { immediate: true });

function selectedBossMaximumHealth(): number {
    const entity = actorManager.value.entityMap[selectedBossId.value] as EntityActor | undefined;
    const maximumHealth = Number(entity?.statMap?.[30]);
    return Number.isFinite(maximumHealth) && maximumHealth > 0 ? maximumHealth : 0;
}

const playerOptions = computed(() => {
    const bossDamage = selectedBossMaximumHealth() || summary.value?.effectiveBossDamage || 0;
    const players = summary.value?.players ?? [];
    const localId = actorManager.value.localEntityId;
    const preferred = selectMainReportPlayer(players, localId, selectedPlayerId.value);
    const visiblePlayers = teamUseIds.value
        ? [...players].sort((a, b) => Number(b.entityId === localId) - Number(a.entityId === localId) || b.totalDamage - a.totalDamage)
        : preferred ? [preferred] : [];
    const items = visiblePlayers.map((player) => ({
        entityId: player.entityId,
        label: `${getDisplayName(player.name)} · ${fmtCompact(player.totalDamage)}（${fmtPct(bossDamage > 0 ? player.totalDamage / bossDamage : 0)}）`,
    }));
    if (items.length || !isDesignPreview) return items;
    const previewPlayers = previewTeamChartPlayers(true);
    return (teamUseIds.value ? previewPlayers : previewPlayers.slice(0, 1)).map((player) => {
        const totalDamage = player.damages.reduce((sum, damage) => sum + damage.Damage, 0);
        return {
            entityId: player.entityId,
            label: `${player.label} · ${fmtCompact(totalDamage)}（${fmtPct(totalDamage / PREVIEW_BOSS_TOTAL_DAMAGE)}）`,
        };
    });
});

watch(playerOptions, (items) => {
    if (items.some((item) => item.entityId === selectedPlayerId.value)) return;
    selectedPlayerId.value = items[0]?.entityId ?? "";
}, { immediate: true });

const selectedPlayer = computed<PlayerSummary | undefined>(() =>
    summary.value?.players.find((player) => player.entityId === selectedPlayerId.value),
);
const reportPlayer = computed(() => selectedPlayer.value || (isDesignPreview ? previewPlayer : undefined));
const reportSession = computed(() => summary.value?.session || (isDesignPreview ? previewSession : undefined));
const selectedBoss = computed(() => actorManager.value.entityMap[selectedBossId.value] as EntityActor | undefined);
const bossHealthText = computed(() => {
    const current = Number(selectedBoss.value?.statMap?.[28]);
    const maximum = Number(selectedBoss.value?.statMap?.[30]);
    if (!Number.isFinite(current) || !Number.isFinite(maximum) || maximum <= 0 || current < 0) return "";
    return `${fmtCompact(current)} / ${fmtCompact(maximum)}（${fmtPct(Math.min(1, Math.max(0, current / maximum)))}）`;
});
const bossLabel = computed(() => selectedBoss.value
    ? bossName(selectedBoss.value)
    : isDesignPreview ? "样例首领" : "尚未选择目标",
);
const playerLabel = computed(() => reportPlayer.value ? getDisplayName(reportPlayer.value.name) : "");
const reportBossTotalDamage = computed(() => selectedBossMaximumHealth() || summary.value?.effectiveBossDamage
    || (isDesignPreview ? PREVIEW_BOSS_TOTAL_DAMAGE : 0));
const playerContributionRate = computed(() => {
    const total = reportBossTotalDamage.value;
    return total > 0 && reportPlayer.value ? reportPlayer.value.totalDamage / total : 0;
});
const teamChartPlayers = computed<TeamChartPlayer[]>(() => {
    if (isDesignPreview && !summary.value) return previewTeamChartPlayers(teamUseIds.value);
    const currentSummary = summary.value;
    if (!currentSummary) return [];
    const baseLabels = currentSummary.players.map((player) => teamUseIds.value
        ? getDisplayName(player.name)
        : player.jobName || "尚未识别职业",
    );
    const totals = new Map<string, number>();
    for (const label of baseLabels) totals.set(label, (totals.get(label) ?? 0) + 1);
    const indexes = new Map<string, number>();
    return currentSummary.players.map((player, index) => {
        const baseLabel = baseLabels[index];
        const order = (indexes.get(baseLabel) ?? 0) + 1;
        indexes.set(baseLabel, order);
        const label = (totals.get(baseLabel) ?? 0) > 1 ? `${baseLabel} ${order}` : baseLabel;
        const rawDamages = (actorManager.value.entityMap[player.entityId] as EntityActor | undefined)?.applyDamages ?? [];
        const damages = rawDamages
            .filter((damage) => damage.Id === player.entityId
                && damage.TargetId === selectedBossId.value
                && damage.Damage > 0
                && damage.At >= currentSummary.session.startAt
                && damage.At <= currentSummary.session.endAt)
            .map((damage): EntityDamage => ({
                ...damage,
                Conditions: [],
                TargetConditions: [],
                PetId: "",
            }));
        return { entityId: player.entityId, label, damages };
    }).filter((player) => player.damages.length > 0);
});

const teamSkillTimelinePlayers = computed<SkillTimelinePlayer[]>(() => {
    if (isDesignPreview && !summary.value) {
        return previewTeamChartPlayers(teamUseIds.value).map((player) => ({
            entityId: player.entityId,
            label: player.label,
            uses: player.damages.map((damage) => ({
                at: damage.At,
                skillId: damage.SkillId,
                name: skillDisplayName(damage.SkillId),
                iconUrl: skillIconUrl(damage.SkillId),
                damage: damage.Damage,
            })),
        }));
    }
    const currentSummary = summary.value;
    if (!currentSummary) return [];
    const labels = timelinePlayerLabels(currentSummary.players);
    const startAt = currentSummary.session.startAt;
    const endAt = currentSummary.session.endAt;
    return currentSummary.players.map((player, index) => {
        const uses: SkillTimelineUse[] = [];
        const rawDamages = (actorManager.value.entityMap[player.entityId] as EntityActor | undefined)?.applyDamages ?? [];
        for (const damage of rawDamages) {
            if (
                damage.Id !== player.entityId
                || damage.TargetId !== selectedBossId.value
                || damage.SkillId <= 0
                || damage.Damage <= 0
                || Boolean(damage.PetId)
                || damage.At < startAt
                || damage.At > endAt
            ) continue;
            uses.push({
                at: damage.AtMs && damage.AtMs > 0 ? damage.AtMs / 1000 : damage.At,
                skillId: damage.SkillId,
                name: skillDisplayName(damage.SkillId),
                iconUrl: skillIconUrl(damage.SkillId),
                damage: damage.Damage,
            });
        }
        return { entityId: player.entityId, label: labels[index], uses };
    }).filter((player) => player.uses.length > 0);
});

function timelinePlayerLabels(players: PlayerSummary[]): string[] {
    const baseLabels = players.map((player) => teamUseIds.value
        ? getDisplayName(player.name)
        : player.jobName || "尚未识别职业",
    );
    const totals = new Map<string, number>();
    for (const label of baseLabels) totals.set(label, (totals.get(label) ?? 0) + 1);
    const indexes = new Map<string, number>();
    return baseLabels.map((label) => {
        const order = (indexes.get(label) ?? 0) + 1;
        indexes.set(label, order);
        return (totals.get(label) ?? 0) > 1 ? `${label} ${order}` : label;
    });
}

function previewTeamChartPlayers(useIds: boolean): TeamChartPlayer[] {
    const samples = [
        { id: "preview-player", name: "示例角色", job: "黑魔导士", total: 392_535_875, spikes: [0.18, 0.15, 0.13, 0.11, 0.09, 0.08, 0.07, 0.06, 0.05, 0.04, 0.025, 0.015] },
        { id: "preview-player-2", name: "队友甲", job: "元素骑士", total: 351_240_000, spikes: [0.08, 0.12, 0.17, 0.09, 0.18, 0.15, 0.11, 0.10] },
        { id: "preview-player-3", name: "队友乙", job: "圣光颂唱者", total: 293_426_572, spikes: [0.15, 0.08, 0.11, 0.16, 0.10, 0.12, 0.14, 0.14] },
    ];
    return samples.map((sample) => ({
        entityId: sample.id,
        label: useIds ? sample.name : sample.job,
        damages: sample.spikes.map((ratio, index): EntityDamage => ({
            Id: sample.id,
            At: previewSession.startAt + (index + 1) * previewSession.totalDuration / sample.spikes.length,
            TargetId: previewSession.bossEntityId,
            SkillId: 59040 + index,
            Damage: Math.round(sample.total * ratio),
            IsCritical: index % 2 === 0,
            IsDelayed: false,
            Conditions: [],
            TargetConditions: [],
            PetId: "",
        })),
    }));
}

function requestTeamIdentityToggle() {
    if (teamUseIds.value) {
        teamUseIds.value = false;
        return;
    }
    teamIdentityWarningOpen.value = true;
}

function confirmTeamIdentityDisplay() {
    teamUseIds.value = true;
    teamIdentityWarningOpen.value = false;
}

function fmtBattleClock(timestamp: number): string {
    const date = new Date(timestamp * 1000);
    return [date.getHours(), date.getMinutes(), date.getSeconds()]
        .map((value) => String(value).padStart(2, "0"))
        .join(":");
}

const conditionActor = computed<EntityActor | undefined>(() => {
    if (conditionTarget.value === "monster") return selectedBoss.value;
    return actorManager.value.entityMap[selectedPlayerId.value] as EntityActor | undefined;
});

const conditionTargetLabel = computed(() => {
    const actor = conditionActor.value;
    if (!actor && isDesignPreview) return conditionTarget.value === "ally" ? "示例角色" : "示例首领";
    if (!actor) return conditionTarget.value === "ally" ? "尚未选择角色" : "尚未选择怪物";
    return conditionTarget.value === "ally" ? getDisplayName(actor.name) : bossName(actor);
});

const magicCircleBuffs = computed<PlayerBuffDefinition[]>(() => {
    resourceNameVersion.value;
    return Object.entries(condNameMap.value)
        .map(([id, name]) => ({ id: Number(id), name: toSimplified(cleanName(name)) }))
        .filter((item) => item.id >= 10014 && item.id <= 10138 && /魔法阵|魔法陣/.test(item.name))
        .sort((a, b) => a.id - b.id);
});

const playerBuffGroups = computed<PlayerBuffGroup[]>(() => {
    const groups = BASE_PLAYER_BUFF_GROUPS.map((group) => ({
        name: group.name,
        items: group.items.map((item) => ({ ...item })),
    }));
    if (magicCircleBuffs.value.length) {
        groups.splice(3, 0, { name: "魔纹师（魔法阵）", items: magicCircleBuffs.value });
    }
    return groups;
});

const defaultPlayerBuffIds = computed(() => [
    ...new Set(playerBuffGroups.value.flatMap((group) => group.items.map((item) => item.id))),
]);
const effectivePlayerBuffIds = computed(() =>
    (playerBuffUsesDefaults.value ? defaultPlayerBuffIds.value : playerBuffIds.value)
        .filter((id) => !HIDDEN_PLAYER_BUFF_IDS.has(id)),
);
const configuredPlayerBuffIdSet = computed(() => new Set(effectivePlayerBuffIds.value));
const defaultPlayerBuffDefinitionMap = computed(() => new Map(
    playerBuffGroups.value.flatMap((group) => group.items).map((item) => [item.id, item]),
));
const customPlayerBuffs = computed(() => effectivePlayerBuffIds.value
    .filter((id) => !defaultPlayerBuffDefinitionMap.value.has(id))
    .map((id) => ({ id, name: conditionDisplayName(id) }))
    .sort((a, b) => a.id - b.id),
);
const allConditionDefinitions = computed<PlayerBuffDefinition[]>(() => {
    resourceNameVersion.value;
    const definitions = new Map<number, PlayerBuffDefinition>();
    for (const group of playerBuffGroups.value) {
        for (const item of group.items) definitions.set(item.id, item);
    }
    for (const [rawId, rawName] of Object.entries(fallbackConditionNames)) {
        const id = Number(rawId);
        if (!Number.isInteger(id) || id < 0 || HIDDEN_PLAYER_BUFF_IDS.has(id) || definitions.has(id)) continue;
        definitions.set(id, { id, name: toSimplified(cleanName(rawName)) || `状态 ${id}` });
    }
    for (const [rawId, rawName] of Object.entries(condNameMap.value)) {
        const id = Number(rawId);
        if (!Number.isInteger(id) || id < 0 || HIDDEN_PLAYER_BUFF_IDS.has(id)) continue;
        const name = toSimplified(cleanName(rawName)) || `状态 ${id}`;
        if (!defaultPlayerBuffDefinitionMap.value.has(id)) definitions.set(id, { id, name });
    }
    return [...definitions.values()].sort((a, b) => a.id - b.id);
});
const playerBuffSearchResults = computed<PlayerBuffDefinition[]>(() => {
    const query = toSimplified(playerBuffInput.value.trim()).toLowerCase();
    if (!query) return [];
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (!Number.isInteger(id) || id < 0 || HIDDEN_PLAYER_BUFF_IDS.has(id)) return [];
        return [allConditionDefinitions.value.find((item) => item.id === id) ?? { id, name: `状态 ${id}` }];
    }
    return allConditionDefinitions.value
        .filter((item) => item.name.toLowerCase().includes(query))
        .slice(0, 20);
});

const configuredBuffAlertRules = computed<BuffAlertRule[]>(() =>
    Object.values(buffOverlaySettings.value.rules)
        .sort((a, b) => conditionDisplayName(a.ccId).localeCompare(conditionDisplayName(b.ccId), "zh-CN") || a.ccId - b.ccId),
);
const buffAlertSearchResults = computed<PlayerBuffDefinition[]>(() => {
    const query = toSimplified(buffAlertInput.value.trim()).toLowerCase();
    if (!query) return [];
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (!Number.isInteger(id) || id < 0 || HIDDEN_PLAYER_BUFF_IDS.has(id)) return [];
        return [allConditionDefinitions.value.find((item) => item.id === id) ?? { id, name: `状态 ${id}` }];
    }
    return allConditionDefinitions.value
        .filter((item) => item.name.toLowerCase().includes(query))
        .slice(0, 12);
});
const debuffAlertSearchResults = computed<PlayerBuffDefinition[]>(() => {
    const query = toSimplified(debuffAlertInput.value.trim()).toLowerCase();
    if (!query) return [];
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (!Number.isInteger(id) || id < 0) return [];
        return [allConditionDefinitions.value.find((item) => item.id === id) ?? { id, name: `状态 ${id}` }];
    }
    return allConditionDefinitions.value
        .filter((item) => item.name.toLowerCase().includes(query))
        .slice(0, 12);
});
const allDebuffBossRaceDefinitions = computed<Array<{ id: number; name: string }>>(() => {
    resourceNameVersion.value;
    return Object.entries(raceNameMap.value)
        .map(([rawId]) => Number(rawId))
        .filter((id) => Number.isInteger(id) && id > 0)
        .map((id) => ({ id, name: debuffBossRaceDisplayName(id) }))
        .sort((a, b) => a.name.localeCompare(b.name, "zh-CN") || a.id - b.id);
});
const debuffBossRaceSearchResults = computed<Array<{ id: number; name: string }>>(() => {
    const query = toSimplified(debuffBossRaceInput.value.trim()).toLowerCase();
    if (!query) return [];
    const configured = new Set(debuffAlertSettings.value.forcedBossRaceIds);
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (!Number.isInteger(id) || id <= 0 || configured.has(id)) return [];
        return [allDebuffBossRaceDefinitions.value.find((item) => item.id === id)
            ?? { id, name: `种族 ${id}` }];
    }
    return allDebuffBossRaceDefinitions.value
        .filter((item) => !configured.has(item.id) && item.name.toLowerCase().includes(query))
        .slice(0, 12);
});
const configuredForcedDebuffBossRaces = computed(() => debuffAlertSettings.value.forcedBossRaceIds
    .map((id) => ({ id, name: debuffBossRaceDisplayName(id) })));
const configuredSkillCooldownRules = computed<SkillCooldownRule[]>(() =>
    Object.values(skillCooldownSettings.value.rules)
        .filter((rule) => !rule.barOnly)
        .sort((a, b) => skillDisplayName(a.skillId).localeCompare(skillDisplayName(b.skillId), "zh-CN") || a.skillId - b.skillId),
);
const configuredBossMechanicRules = computed<BossMechanicAlertRule[]>(() =>
    Object.values(bossMechanicSettings.value.rules),
);
const configuredEffectTimerRules = computed<EffectTimerRule[]>(() =>
    Object.values(effectTimerSettings.value.rules).sort((a, b) => a.name.localeCompare(b.name, "zh-CN") || a.key.localeCompare(b.key)),
);
const allSkillDefinitions = computed<Array<{ id: number; name: string }>>(() => {
    resourceNameVersion.value;
    const definitions = new Map<number, string>();
    for (const [rawId, rawName] of Object.entries(FALLBACK_SKILL_NAME_MAP)) {
        const id = Number(rawId);
        if (Number.isInteger(id) && id > 0) {
            const name = toSimplified(cleanName(rawName));
            definitions.set(id, normalizeSkillDisplayName(id, name) || `技能 ${id}`);
        }
    }
    for (const [rawId, rawName] of Object.entries(skillNameMap.value)) {
        const id = Number(rawId);
        if (Number.isInteger(id) && id > 0) {
            const name = toSimplified(cleanName(rawName));
            definitions.set(id, normalizeSkillDisplayName(id, name) || definitions.get(id) || `技能 ${id}`);
        }
    }
    return [...definitions].map(([id, name]) => ({ id, name }));
});
const skillCooldownSearchResults = computed<Array<{ id: number; name: string }>>(() => {
    const query = toSimplified(skillCooldownInput.value.trim()).toLowerCase();
    if (!query) return [];
    if (/^\d+$/.test(query)) {
        const id = Number(query);
        if (!Number.isInteger(id) || id <= 0) return [];
        return [{ id, name: skillDisplayName(id) }];
    }
    return allSkillDefinitions.value
        .filter((item) => item.name.toLowerCase().includes(query))
        .sort((a, b) => a.name.localeCompare(b.name, "zh-CN") || a.id - b.id)
        .slice(0, 12);
});
const reminderSettingsDirty = computed(() =>
    buffAlertSettingsDirty.value || debuffAlertSettingsDirty.value || skillCooldownSettingsDirty.value
        || aimReminderSettingsDirty.value || bossMechanicSettingsDirty.value || effectTimerSettingsDirty.value || reminderProfileSettingsDirty.value,
);
const configuredDebuffAlertRules = computed(() => {
    const seen = new Set<number>();
    return Object.values(debuffAlertSettings.value.rules)
        .sort((a, b) => a.order - b.order || a.ccId - b.ccId)
        .filter((rule) => {
            const canonical = canonicalDebuffId(rule.ccId);
            if (seen.has(canonical)) return false;
            seen.add(canonical);
            return true;
        });
});
const activeReminderProfile = computed(() => reminderProfileStore.value.profiles
    .find((profile) => profile.id === selectedReminderProfileId.value));

const conditionRows = computed<ConditionCoverageRow[]>(() => {
    resourceNameVersion.value;
    const actor = conditionActor.value;
    const session = reportSession.value;
    if (!actor || !session) return isDesignPreview ? previewConditionRows() : [];

    if (conditionTarget.value === "ally") {
        return effectivePlayerBuffIds.value
            .map((ccId) => buildConditionCoverageRow(actor, ccId, session.startAt, session.endAt))
            .sort((a, b) => b.coverage - a.coverage || a.ccId - b.ccId);
    }

    const observedIds = new Set<number>();
    for (let i = 0; i < actor.conditionHistory.length; i++) {
        const state = actor.conditionHistory[i];
        const nextAt = actor.conditionHistory[i + 1]?.At ?? session.endAt;
        if (state.At >= session.endAt || nextAt <= session.startAt) continue;
        for (const condition of state.List) {
            if (isLikelyMonsterDebuff(actor, condition)) observedIds.add(canonicalDebuffId(condition.CCId));
        }
    }
    return [...observedIds]
        .map((ccId) => buildConditionCoverageRow(actor, ccId, session.startAt, session.endAt))
        .filter((row) => row.activeSeconds > 0)
        .sort((a, b) => b.coverage - a.coverage || a.ccId - b.ccId);
});

const conditionEmptyText = computed(() => {
    if (conditionTarget.value === "monster") return "本场未记录到怪物 Debuff";
    if (effectivePlayerBuffIds.value.length === 0) return "尚未配置玩家 Buff";
    if (!conditionActor.value || !reportSession.value) return "尚无战斗记录，Buff 配置仍已保留";
    return "本场没有记录到已配置的玩家 Buff";
});

const favoriteConditionIdSet = computed(() => new Set(
    playerBuffFavoriteIds.value.filter((id) => configuredPlayerBuffIdSet.value.has(id)),
));
const favoriteConditionRows = computed(() => conditionRows.value
    .filter((row) => favoriteConditionIdSet.value.has(row.ccId))
    .sort((a, b) => b.coverage - a.coverage || a.ccId - b.ccId),
);
const playerConditionGroups = computed<PlayerConditionGroup[]>(() => {
    if (conditionTarget.value !== "ally") return [];
    const rowById = new Map(conditionRows.value.map((row) => [row.ccId, row]));
    const groups: PlayerConditionGroup[] = playerBuffGroups.value.map((group) => {
        const rows = group.items
            .filter((item) => configuredPlayerBuffIdSet.value.has(item.id) && !favoriteConditionIdSet.value.has(item.id))
            .map((item) => rowById.get(item.id))
            .filter((row): row is ConditionCoverageRow => Boolean(row));
        return {
            name: group.name,
            rows,
            activeCount: rows.filter((row) => row.activeSeconds > 0).length,
        };
    });
    const customRows = customPlayerBuffs.value
        .filter((item) => !favoriteConditionIdSet.value.has(item.id))
        .map((item) => rowById.get(item.id))
        .filter((row): row is ConditionCoverageRow => Boolean(row));
    if (customRows.length) {
        groups.push({
            name: "自定义",
            rows: customRows,
            activeCount: customRows.filter((row) => row.activeSeconds > 0).length,
        });
    }
    return groups.filter((group) => group.rows.length > 0);
});

const selectedConditionRow = computed(() =>
    conditionRows.value.find((row) => row.ccId === selectedConditionId.value),
);
const conditionTimelineScale = computed(() => {
    const duration = reportSession.value?.totalDuration ?? 0;
    return buildConditionTimelineScale(duration, conditionTimelineWidth.value);
});

watch(conditionTimelineElement, (element) => {
    conditionTimelineResizeObserver?.disconnect();
    if (!element) return;
    conditionTimelineWidth.value = element.getBoundingClientRect().width;
    conditionTimelineResizeObserver?.observe(element);
});

watch(conditionTarget, () => {
    selectedConditionId.value = null;
    playerBuffInputError.value = "";
});
watch(conditionRows, (rows) => {
    if (selectedConditionId.value !== null && !rows.some((row) => row.ccId === selectedConditionId.value)) {
        selectedConditionId.value = null;
    }
});

function buildConditionCoverageRow(
    actor: EntityActor,
    ccId: number,
    startAt: number,
    endAt: number,
): ConditionCoverageRow {
    const equivalentIds = conditionTarget.value === "monster" ? equivalentDebuffIds(ccId) : [ccId];
    const segments = collectConditionSegments(actor, equivalentIds, startAt, endAt);
    const activeSeconds = segments.reduce((sum, segment) => sum + segment.end - segment.start, 0);
    const duration = Math.max(0, endAt - startAt);
    return {
        entityId: actor.id,
        ccId,
        name: conditionTarget.value === "monster"
            ? equivalentDebuffDisplayName(ccId) ?? conditionDisplayName(ccId)
            : conditionDisplayName(ccId),
        iconUrl: conditionIconUrl(ccId),
        coverage: duration > 0 ? Math.min(1, activeSeconds / duration) : 0,
        activeSeconds,
        segments,
    };
}

function collectConditionSegments(
    actor: EntityActor,
    ccIds: number | number[],
    startAt: number,
    endAt: number,
): ConditionSegment[] {
    const segments: ConditionSegment[] = [];
    const acceptedIds = new Set(Array.isArray(ccIds) ? ccIds : [ccIds]);
    const history = actor.conditionHistory;
    for (let i = 0; i < history.length; i++) {
        const segmentStart = Math.max(startAt, history[i].At);
        const condition = history[i].List.find((item) => acceptedIds.has(item.CCId));
        if (!condition) continue;
        const expiresAt = resolveBuffExpiresAt(condition);
        const segmentEnd = Math.min(endAt, history[i + 1]?.At ?? endAt, expiresAt ?? endAt);
        if (segmentEnd <= segmentStart) continue;
        const previous = segments.at(-1);
        if (previous && Math.abs(previous.end - segmentStart) < 0.001) {
            previous.end = segmentEnd;
        } else {
            segments.push({ start: segmentStart, end: segmentEnd });
        }
    }
    return segments;
}

function isLikelyMonsterDebuff(target: EntityActor, condition: EntityCondition) {
    if (!condition.AttackerId || condition.AttackerId === "0") return true;
    if (condition.AttackerId === target.id) return false;
    let source = actorManager.value.entityMap[condition.AttackerId] as EntityActor | undefined;
    if (!source) return true;
    if (source.ownerId) {
        source = actorManager.value.entityMap[source.ownerId] as EntityActor | undefined;
    }
    return source?.isPC ?? true;
}

function previewConditionRows(): ConditionCoverageRow[] {
    const session = previewSession;
    const samples = conditionTarget.value === "ally"
        ? new Map<number, { name: string; ranges: number[][] }>([
            { ccId: 680, name: "战争序曲", ranges: [[0, 47], [62, 136]] },
            { ccId: 938, name: "魔法穿刺", ranges: [[18, 79], [102, 158]] },
            { ccId: 63, name: "攻击力增加", ranges: [[0, 92]] },
            { ccId: 887, name: "远距离战术系列攻击伤害增加", ranges: [] },
        ].map((sample) => [sample.ccId, { name: sample.name, ranges: sample.ranges }]))
        : new Map<number, { name: string; ranges: number[][] }>([
            { ccId: 1145, name: "催化协同", ranges: [[12, 58], [81, 137]] },
            { ccId: 323, name: "防御弱化", ranges: [[21, 111]] },
            { ccId: 948, name: "攻击削弱", ranges: [[47, 93], [121, 158]] },
        ].map((sample) => [sample.ccId, { name: sample.name, ranges: sample.ranges }]));
    const ids = conditionTarget.value === "ally"
        ? effectivePlayerBuffIds.value
        : [...samples.keys()];
    return ids.map((ccId) => {
        const sample = samples.get(ccId) ?? { name: conditionDisplayName(ccId), ranges: [] };
        const segments = sample.ranges.map(([start, end]) => ({
            start: session.startAt + start,
            end: session.startAt + end,
        }));
        const activeSeconds = segments.reduce((sum, segment) => sum + segment.end - segment.start, 0);
        return {
            entityId: conditionTarget.value === "ally" ? "preview-player" : "preview-boss",
            ccId,
            name: sample.name,
            iconUrl: conditionIconUrl(ccId),
            coverage: activeSeconds / session.totalDuration,
            activeSeconds,
            segments,
        };
    }).sort((a, b) => b.coverage - a.coverage);
}

function conditionDisplayName(ccId: number) {
    const equivalentName = equivalentDebuffDisplayName(ccId);
    if (equivalentName) return equivalentName;
    const overrideName = CONDITION_DISPLAY_NAME_OVERRIDES[ccId];
    if (overrideName) return overrideName;
    const resourceName = toSimplified(cleanName(condNameMap.value[ccId]));
    const fallbackName = toSimplified(cleanName(FALLBACK_CONDITION_NAME_MAP[String(ccId)]));
    return resourceName || defaultPlayerBuffDefinitionMap.value.get(ccId)?.name || fallbackName || `状态 ${ccId}`;
}

function conditionIconUrl(ccId: number) {
    return `/res/characterconditionimage/${region.value}/${ccId}/${ccId}.png`;
}

function selectCondition(ccId: number) {
    selectedConditionId.value = selectedConditionId.value === ccId ? null : ccId;
}

function conditionSegmentStyle(segment: ConditionSegment) {
    const session = reportSession.value;
    if (!session || session.totalDuration <= 0) return { left: "0%", width: "0%" };
    const left = Math.max(0, Math.min(100, (segment.start - session.startAt) / session.totalDuration * 100));
    const width = Math.max(0.2, Math.min(100 - left, (segment.end - segment.start) / session.totalDuration * 100));
    return { left: `${left}%`, width: `${width}%` };
}

function fmtBattleElapsed(timestamp: number) {
    const startAt = reportSession.value?.startAt ?? timestamp;
    return fmtCoverageDuration(Math.max(0, timestamp - startAt));
}

function materializePlayerBuffDefaults() {
    if (!playerBuffUsesDefaults.value) return;
    playerBuffIds.value = [...defaultPlayerBuffIds.value];
    playerBuffUsesDefaults.value = false;
    reminderProfileSettingsDirty.value = true;
}

function togglePlayerBuffSettings() {
    playerBuffSettingsOpen.value = !playerBuffSettingsOpen.value;
}

function togglePlayerBuff(ccId: number) {
    if (HIDDEN_PLAYER_BUFF_IDS.has(ccId)) return;
    materializePlayerBuffDefaults();
    const ids = new Set(playerBuffIds.value);
    if (ids.has(ccId)) {
        ids.delete(ccId);
        playerBuffFavoriteIds.value = playerBuffFavoriteIds.value.filter((id) => id !== ccId);
        savePlayerBuffFavoriteIds();
    } else {
        ids.add(ccId);
    }
    playerBuffIds.value = [...ids].sort((a, b) => a - b);
    savePlayerBuffIds();
    reminderProfileSettingsDirty.value = true;
}

function addPlayerBuff() {
    playerBuffInputError.value = "";
    const result = playerBuffSearchResults.value[0];
    if (!result) {
        playerBuffInputError.value = "未找到匹配状态，请检查名称或 CC ID。";
        return;
    }
    addPlayerBuffById(result.id);
}

function addPlayerBuffById(ccId: number) {
    if (!Number.isInteger(ccId) || ccId < 0 || HIDDEN_PLAYER_BUFF_IDS.has(ccId)) return;
    materializePlayerBuffDefaults();
    if (!playerBuffIds.value.includes(ccId)) {
        playerBuffIds.value = [...playerBuffIds.value, ccId].sort((a, b) => a - b);
        savePlayerBuffIds();
        reminderProfileSettingsDirty.value = true;
    }
    playerBuffInput.value = "";
    playerBuffInputError.value = "";
}

function selectedBuffCount(group: PlayerBuffGroup) {
    return group.items.filter((item) => configuredPlayerBuffIdSet.value.has(item.id)).length;
}

function toggleFavoriteCondition(ccId: number) {
    if (!configuredPlayerBuffIdSet.value.has(ccId) || HIDDEN_PLAYER_BUFF_IDS.has(ccId)) return;
    const ids = new Set(playerBuffFavoriteIds.value);
    if (ids.has(ccId)) ids.delete(ccId);
    else ids.add(ccId);
    playerBuffFavoriteIds.value = [...ids].sort((a, b) => a - b);
    savePlayerBuffFavoriteIds();
    reminderProfileSettingsDirty.value = true;
}

function addBuffAlertRule(selectedId: number) {
    const ccId = Number(selectedId);
    if (!Number.isInteger(ccId) || ccId < 0) return;
    if (!buffOverlaySettings.value.rules[ccId]) {
        buffOverlaySettings.value.rules[ccId] = makeBuffAlertRule(ccId);
    }
    if (!isStackOnlyBuffAlertCondition(ccId)) {
        buffOverlaySettings.value.rules[ccId].overlayEnabled = true;
    }
    buffAlertInput.value = "";
    markBuffAlertSettingsDirty();
    publishBuffOverlayState();
}

function addFirstBuffAlertSearchResult() {
    const result = buffAlertSearchResults.value.find((item) => !buffOverlaySettings.value.rules[item.id]);
    if (result) addBuffAlertRule(result.id);
}

function openBuffAlertEditor(ccId: number) {
    addBuffAlertRule(ccId);
    activeTab.value = "alerts";
}

function isBuffAlertEnabled(ccId: number) {
    const rule = buffOverlaySettings.value.rules[ccId];
    return Boolean(isStackOnlyBuffAlertCondition(ccId) ? rule?.stackAlertEnabled : rule?.overlayEnabled);
}

function isDebuffAlertEnabled(ccId: number) {
    return equivalentDebuffIds(ccId).some((id) => Boolean(debuffAlertSettings.value.rules[id]?.enabled));
}

function toggleDebuffAlert(ccId: number) {
    const equivalentIds = equivalentDebuffIds(ccId);
    if (equivalentIds.some((id) => debuffAlertSettings.value.rules[id])) {
        for (const id of equivalentIds) delete debuffAlertSettings.value.rules[id];
    }
    else {
        const canonical = canonicalDebuffId(ccId);
        const rule = makeDebuffAlertRule(canonical);
        rule.order = nextDebuffAlertOrder();
        debuffAlertSettings.value.rules[canonical] = rule;
    }
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function addDebuffAlertRule(selectedId: number) {
    const ccId = canonicalDebuffId(Number(selectedId));
    if (!Number.isInteger(ccId) || ccId < 0) return;
    if (!debuffAlertSettings.value.rules[ccId]) {
        const rule = makeDebuffAlertRule(ccId);
        rule.order = nextDebuffAlertOrder();
        debuffAlertSettings.value.rules[ccId] = rule;
    }
    debuffAlertSettings.value.rules[ccId].enabled = true;
    debuffAlertInput.value = "";
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function addCommonDebuffAlerts() {
    let added = 0;
    for (const ccId of DEFAULT_DEBUFF_ALERT_IDS) {
        if (equivalentDebuffIds(ccId).some((id) => Boolean(debuffAlertSettings.value.rules[id]))) continue;
        const rule = makeDebuffAlertRule(ccId);
        rule.order = nextDebuffAlertOrder();
        debuffAlertSettings.value.rules[ccId] = rule;
        added += 1;
    }
    if (added > 0) {
        markDebuffAlertSettingsDirty();
        publishDebuffOverlayState();
        showNotice(`已补齐 ${added} 个常用 Debuff；现有设定未被覆盖。`, "success");
    } else {
        showNotice("常用 Debuff 已全部添加。", "info");
    }
}

function addFirstDebuffAlertSearchResult() {
    const result = debuffAlertSearchResults.value.find((item) => !debuffAlertSettings.value.rules[item.id]);
    if (result) addDebuffAlertRule(result.id);
}

function debuffBossRaceDisplayName(raceId: number) {
    const override = BOSS_NAME_OVERRIDES[raceId];
    if (override) return override;
    const raw = toSimplified(cleanName(raceNameMap.value[raceId]));
    const name = raw.replace(new RegExp(`\\s+${raceId}$`), "").trim();
    return name || `种族 ${raceId}`;
}

function addForcedDebuffBossRace(raceId: number) {
    const id = Number(raceId);
    if (!Number.isInteger(id) || id <= 0) return;
    debuffAlertSettings.value.forcedBossRaceIds = [
        ...new Set([...debuffAlertSettings.value.forcedBossRaceIds, id]),
    ];
    debuffBossRaceInput.value = "";
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function addFirstForcedDebuffBossRace() {
    const result = debuffBossRaceSearchResults.value[0];
    if (result) addForcedDebuffBossRace(result.id);
}

function removeForcedDebuffBossRace(raceId: number) {
    debuffAlertSettings.value.forcedBossRaceIds = debuffAlertSettings.value.forcedBossRaceIds
        .filter((id) => id !== raceId);
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function removeDebuffAlert(ccId: number) {
    delete debuffAlertSettings.value.rules[ccId];
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function nextDebuffAlertOrder() {
    const orders = Object.values(debuffAlertSettings.value.rules).map((rule) => Number(rule.order) || 0);
    return orders.length ? Math.max(...orders) + 1 : 0;
}

function moveDebuffAlertRule(ccId: number, direction: -1 | 1) {
    const rules = configuredDebuffAlertRules.value;
    const index = rules.findIndex((rule) => rule.ccId === ccId);
    const targetIndex = index + direction;
    if (index < 0 || targetIndex < 0 || targetIndex >= rules.length) return;
    const target = rules[targetIndex];
    const current = rules[index];
    const currentOrder = current.order;
    current.order = target.order;
    target.order = currentOrder;
    markDebuffAlertSettingsDirty();
    publishDebuffOverlayState();
}

function markDebuffAlertSettingsDirty() {
    debuffAlertSettingsDirty.value = true;
    reminderProfileSettingsDirty.value = true;
}

function persistBuffAlertSettings(): Promise<boolean> {
    buffOverlaySettings.value.volume = Math.min(100, Math.max(0, Math.round(Number(buffOverlaySettings.value.volume) || 0)));
    buffOverlaySettings.value.timeAdjustmentSeconds = Math.min(3600, Math.max(-3600, Math.round(Number(buffOverlaySettings.value.timeAdjustmentSeconds) || 0)));
    for (const rule of Object.values(buffOverlaySettings.value.rules)) {
        rule.manualDurationSeconds = Math.min(86400, Math.max(1, Math.round(Number(rule.manualDurationSeconds) || 60)));
        rule.flashThresholdSeconds = Math.min(3600, Math.max(1, Math.round(Number(rule.flashThresholdSeconds) || 10)));
        rule.soundThresholdSeconds = Math.min(3600, Math.max(1, Math.round(Number(rule.soundThresholdSeconds) || 5)));
        rule.stackThreshold = Math.min(99, Math.max(1, Math.round(Number(rule.stackThreshold) || 5)));
    }
    saveBuffOverlaySettings(buffOverlaySettings.value);
    skillCooldownSettings.value.iconSize = Math.min(96, Math.max(24, Math.round(Number(skillCooldownSettings.value.iconSize) || 48)));
    skillCooldownSettings.value.aimReminder.weaponRange = Math.min(10000, Math.max(100, Math.round(
        Number(skillCooldownSettings.value.aimReminder.weaponRange) || DEFAULT_MAGNUM_WEAPON_RANGE,
    )));
    skillCooldownSettings.value.aimReminder.rangeIdentificationLevel = Math.min(20, Math.max(0, Math.round(
        Number(skillCooldownSettings.value.aimReminder.rangeIdentificationLevel) || DEFAULT_MAGNUM_RANGE_IDENTIFICATION_LEVEL,
    )));
    skillCooldownSettings.value.aimReminder.calibrationPercent = Math.min(40, Math.max(20, Math.round(
        Number(skillCooldownSettings.value.aimReminder.calibrationPercent) || DEFAULT_MAGNUM_AIM_CALIBRATION_PERCENT,
    )));
    skillCooldownSettings.value.aimReminder.ergSpeedPercent = Math.min(1000, Math.max(100, Math.round(
        Number(skillCooldownSettings.value.aimReminder.ergSpeedPercent) || DEFAULT_MAGNUM_ERG_AIM_SPEED_PERCENT,
    )));
    skillCooldownSettings.value.aimReminder.fineTuneSeconds = Math.min(10, Math.max(-10, Math.round(
        (Number(skillCooldownSettings.value.aimReminder.fineTuneSeconds) || DEFAULT_MAGNUM_AIM_FINE_TUNE_SECONDS) * 100,
    ) / 100));
    skillCooldownSettings.value.aimReminder.scalePercent = Math.min(200, Math.max(50, Math.round(
        Number(skillCooldownSettings.value.aimReminder.scalePercent) || 100,
    )));
    skillCooldownSettings.value.aimReminder.x = Math.min(32000, Math.max(-32000, Math.round(
        Number(skillCooldownSettings.value.aimReminder.x) || 0,
    )));
    skillCooldownSettings.value.aimReminder.y = Math.min(32000, Math.max(-32000, Math.round(
        Number(skillCooldownSettings.value.aimReminder.y) || 0,
    )));
    for (const rule of Object.values(skillCooldownSettings.value.rules)) {
        rule.cooldownSeconds = Math.min(86400, Math.max(0.1, Math.round((Number(rule.cooldownSeconds) || 30) * 10) / 10));
        rule.shortCooldownSeconds = Math.min(86400, Math.max(0.1,
            Math.round((Number(rule.shortCooldownSeconds) || 1) * 10) / 10));
        rule.cumulativeCooldownSeconds = Math.min(86400, Math.max(0.1,
            Math.round((Number(rule.cumulativeCooldownSeconds) || 10) * 10) / 10));
        rule.progressThresholdPercent = Math.min(100, Math.max(1, Math.round(
            Number(rule.progressThresholdPercent) || DEFAULT_TOAH_PROGRESS_THRESHOLD,
        )));
        rule.x = Math.min(32000, Math.max(-32000, Math.round(Number(rule.x) || 0)));
        rule.y = Math.min(32000, Math.max(-32000, Math.round(Number(rule.y) || 0)));
    }
    saveSkillCooldownSettings(skillCooldownSettings.value);
    debuffAlertSettings.value.iconSize = Math.min(80, Math.max(16, Math.round(Number(debuffAlertSettings.value.iconSize) || 30)));
    debuffAlertSettings.value.volume = Math.min(100, Math.max(0, Math.round(Number(debuffAlertSettings.value.volume) || 0)));
    saveDebuffAlertSettings(debuffAlertSettings.value);
    saveBossMechanicAlertSettings(bossMechanicSettings.value);
    saveEffectTimerSettings(effectTimerSettings.value);
    saveCurrentReminderProfile();
    void fetch("/api/buff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ locked: buffOverlaySettings.value.locked }),
    }).catch(() => undefined);
    buffAlertSettingsDirty.value = false;
    skillCooldownSettingsDirty.value = false;
    aimReminderSettingsDirty.value = false;
    debuffAlertSettingsDirty.value = false;
    bossMechanicSettingsDirty.value = false;
    effectTimerSettingsDirty.value = false;
    reminderProfileSettingsDirty.value = false;
    const nativeSync = scheduleNativeReminderSettingsSync(true);
    publishBuffOverlayState();
    publishDebuffOverlayState();
    publishSkillCooldownOverlayState();
    return nativeSync;
}

async function persistSkillCooldownSettings() {
    if (skillCooldownSettingsSaving.value) return;
    skillCooldownSettingsSaving.value = true;
    let nativeSynced = false;
    try {
        nativeSynced = await persistBuffAlertSettings();
    } catch {
        // Local storage can fail when the WebView profile is unavailable.
    } finally {
        skillCooldownSettingsSaving.value = false;
    }
    if (!nativeSynced) {
        skillCooldownSettingsDirty.value = true;
        skillCooldownSettingsSaved.value = false;
        showNotice("设定已保存在本机，但桌面提醒同步失败，请再点一次保存。", "warning");
        return;
    }
    skillCooldownSettingsSaved.value = true;
    if (skillCooldownSavedTimer !== undefined) window.clearTimeout(skillCooldownSavedTimer);
    skillCooldownSavedTimer = window.setTimeout(() => (skillCooldownSettingsSaved.value = false), 1600);
    showNotice("技能 CD 提醒设定已保存并生效。", "success");
}

function markAimReminderSettingsDirty() {
    aimReminderSettingsDirty.value = true;
    aimReminderSettingsSaved.value = false;
    reminderProfileSettingsDirty.value = true;
    publishSkillCooldownOverlayState();
}

async function persistAimReminderSettings() {
    if (aimReminderSettingsSaving.value) return;
    aimReminderSettingsSaving.value = true;
    let nativeSynced = false;
    try {
        nativeSynced = await persistBuffAlertSettings();
    } catch {
        // Keep the button retryable when WebView storage is unavailable.
    } finally {
        aimReminderSettingsSaving.value = false;
    }
    if (!nativeSynced) {
        aimReminderSettingsDirty.value = true;
        aimReminderSettingsSaved.value = false;
        showNotice("设定已保存在本机，但桌面提醒同步失败，请再点一次保存。", "warning");
        return;
    }
    aimReminderSettingsSaved.value = true;
    if (aimReminderSavedTimer !== undefined) window.clearTimeout(aimReminderSavedTimer);
    aimReminderSavedTimer = window.setTimeout(() => (aimReminderSettingsSaved.value = false), 1600);
    showNotice("瞄准提醒设定已保存并生效。", "success");
}

function markBossMechanicSettingsDirty() {
    bossMechanicSettingsDirty.value = true;
    bossMechanicSettingsSaved.value = false;
    reminderProfileSettingsDirty.value = true;
    publishSkillCooldownOverlayState(Boolean(mielShardHealthBarPreview.value));
}

function persistBossMechanicSettings() {
    saveBossMechanicAlertSettings(bossMechanicSettings.value);
    saveCurrentReminderProfile();
    bossMechanicSettingsDirty.value = false;
    reminderProfileSettingsDirty.value = false;
    bossMechanicSettingsSaved.value = true;
    if (bossMechanicSavedTimer !== undefined) window.clearTimeout(bossMechanicSavedTimer);
    bossMechanicSavedTimer = window.setTimeout(() => (bossMechanicSettingsSaved.value = false), 1600);
    scheduleNativeReminderSettingsSync(true);
    publishSkillCooldownOverlayState();
    showNotice("Boss 特殊机制提醒设定已保存。", "success");
}

function bossMechanicRuntimeStatus(key: string) {
    const runtime = bossMechanicRuntime.value[key];
    if (!runtime || runtime.endsAtMs <= skillCooldownClock.value) return "等待捕捉机制信号";
    if (runtime.mielOrb && !isMielOrbVisible(runtime.mielOrb, skillCooldownClock.value)) {
        return `已按 ${runtime.mielOrb.requiredContacts} 次触碰反馈确认安全`;
    }
    const remaining = Math.max(0, runtime.endsAtMs - skillCooldownClock.value) / 1000;
    return `已触发，剩余 ${remaining >= 10 ? Math.ceil(remaining) : Math.ceil(remaining * 10) / 10} 秒`;
}

function previewBossMechanicRule(key: string) {
    triggerBossMechanic(key, Date.now(), true);
}

function previewMielShardHealthBar() {
    const nowMs = Date.now();
    const generation = (mielShardHealthBarPreview.value?.generation ?? 0) + 1;
    mielShardHealthBarPreview.value = { endsAtMs: nowMs + 5_000, generation };
    if (mielShardHealthBarPreviewTimer !== undefined) window.clearTimeout(mielShardHealthBarPreviewTimer);
    publishSkillCooldownOverlayState(true);
    mielShardHealthBarPreviewTimer = window.setTimeout(() => {
        mielShardHealthBarPreview.value = null;
        mielShardHealthBarPreviewTimer = undefined;
        publishSkillCooldownOverlayState(true);
    }, 5_000);
}

function addEffectTimerRule() {
    const rule = makeEffectTimerRule(configuredEffectTimerRules.value.length);
    effectTimerSettings.value.rules[rule.key] = rule;
    markEffectTimerSettingsDirty();
}

function removeEffectTimerRule(key: string) {
    delete effectTimerSettings.value.rules[key];
    delete effectTimerRuntime.value[key];
    markEffectTimerSettingsDirty();
}

function resolveEffectTimerRuleName(rule: EffectTimerRule) {
    const sourceId = Math.max(1, Math.round(Number(rule.sourceId) || 1));
    rule.sourceId = sourceId;
    rule.name = rule.sourceType === "skill" ? skillDisplayName(sourceId) : conditionDisplayName(sourceId);
}

function resolveEffectTimerSourceFromName(rule: EffectTimerRule) {
    const name = rule.name.trim().toLowerCase();
    const definitions = rule.sourceType === "skill" ? allSkillDefinitions.value : allConditionDefinitions.value;
    const match = definitions.find((item) => item.name.trim().toLowerCase() === name);
    if (match) rule.sourceId = match.id;
    markEffectTimerSettingsDirty();
}

function markEffectTimerSettingsDirty() {
    effectTimerSettingsDirty.value = true;
    effectTimerSettingsSaved.value = false;
    reminderProfileSettingsDirty.value = true;
    publishSkillCooldownOverlayState(true);
}

function persistEffectTimerSettings() {
    saveEffectTimerSettings(effectTimerSettings.value);
    saveCurrentReminderProfile();
    effectTimerSettingsDirty.value = false;
    reminderProfileSettingsDirty.value = false;
    effectTimerSettingsSaved.value = true;
    if (effectTimerSavedTimer !== undefined) window.clearTimeout(effectTimerSavedTimer);
    effectTimerSavedTimer = window.setTimeout(() => (effectTimerSettingsSaved.value = false), 1600);
    scheduleNativeReminderSettingsSync(true);
    publishSkillCooldownOverlayState(true);
    showNotice("伤害增益效果计时条设定已保存。", "success");
}

function previewEffectTimer(rule: EffectTimerRule) {
    triggerEffectTimer(rule, Date.now(), "preview", true);
}

function markBuffAlertSettingsDirty() {
    buffAlertSettingsDirty.value = true;
    reminderProfileSettingsDirty.value = true;
}

function currentReminderSnapshot(): ReminderProfileSnapshot {
    return cloneReminderSnapshot({
        buffRules: buffOverlaySettings.value.rules,
        debuffSettings: debuffAlertSettings.value,
        aimReminder: skillCooldownSettings.value.aimReminder,
        skillRules: Object.fromEntries(Object.entries(skillCooldownSettings.value.rules)
            .filter(([, rule]) => !rule.barOnly)),
        bossMechanicSettings: bossMechanicSettings.value,
        effectTimerSettings: effectTimerSettings.value,
        playerBuffIds: playerBuffIds.value,
        playerBuffFavoriteIds: playerBuffFavoriteIds.value,
        playerBuffUsesDefaults: playerBuffUsesDefaults.value,
    });
}

function mergeSkillBarCooldownRules(rules: Record<number, SkillCooldownRule>): Record<number, SkillCooldownRule> {
    const merged = Object.fromEntries(Object.entries(rules).filter(([, rule]) => !rule.barOnly)) as Record<number, SkillCooldownRule>;
    const bar = loadSkillBarSettings();
    bar.slots.forEach((slot, index) => {
        if (slot.skillId <= 0 || merged[slot.skillId]) return;
        const rule = makeSkillCooldownRule(slot.skillId, bar.x + index * 4, bar.y + bar.iconSize + 12);
        rule.enabled = true;
        rule.barOnly = true;
        rule.alwaysVisible = false;
        rule.soundMode = "none";
        rule.cooldownSeconds = slot.cooldownSeconds;
        merged[slot.skillId] = rule;
    });
    return merged;
}

function saveCurrentReminderProfile(profileId = reminderProfileStore.value.activeProfileId) {
    const profile = reminderProfileStore.value.profiles.find((item) => item.id === profileId);
    if (!profile) return;
    Object.assign(profile, currentReminderSnapshot(), { updatedAt: Date.now() });
    reminderProfileStore.value.activeProfileId = profile.id;
    saveReminderProfileStore(reminderProfileStore.value);
}

function applyReminderProfile(profileId: string) {
    const profile = reminderProfileStore.value.profiles.find((item) => item.id === profileId);
    if (!profile) return;
    const snapshot = cloneReminderSnapshot(profile);
    buffOverlaySettings.value.rules = snapshot.buffRules;
    debuffAlertSettings.value = snapshot.debuffSettings;
    skillCooldownSettings.value.aimReminder = snapshot.aimReminder;
    skillCooldownSettings.value.rules = mergeSkillBarCooldownRules(Object.fromEntries(Object.entries(snapshot.skillRules).map(([rawId, rawRule]) => {
        const skillId = Number(rawId);
        const rule = rawRule as SkillCooldownRule;
        const defaults = makeSkillCooldownRule(skillId, Number(rule.x) || 600, Number(rule.y) || 180);
        const normalizedRule = { ...rule } as SkillCooldownRule & {
            aimAssistEnabled?: boolean;
            aimBaseSeconds?: number;
        };
        delete normalizedRule.aimAssistEnabled;
        delete normalizedRule.aimBaseSeconds;
        return [skillId, {
            ...defaults,
            ...normalizedRule,
        }];
    })));
    bossMechanicSettings.value = normalizeBossMechanicAlertSettings(snapshot.bossMechanicSettings);
    effectTimerSettings.value = normalizeEffectTimerSettings(snapshot.effectTimerSettings);
    playerBuffIds.value = snapshot.playerBuffIds;
    playerBuffFavoriteIds.value = snapshot.playerBuffFavoriteIds;
    playerBuffUsesDefaults.value = snapshot.playerBuffUsesDefaults;
    skillCooldownRuntime.value = {};
    magnumAimPreview.value = null;
    toahSpiritProgressRuntime.value = initialToahSpiritProgressRuntime();
    dorchaQuantityRuntime.value = initialDorchaQuantityRuntime();
    dorchaPreviewUntilMs = 0;
    toahSpiritPreviewUntilMs = 0;
    announcedSkillReadySounds.clear();
    announcedBuffSounds.clear();
    buffStackAboveThreshold.clear();
    buffStackMissingSince.clear();
    buffStackAlertRuntime.value = {};
    effectTimerRuntime.value = {};
    announcedDebuffSounds.clear();
    observedDebuffApplications.clear();
    saveBuffOverlaySettings(buffOverlaySettings.value);
    saveDebuffAlertSettings(debuffAlertSettings.value);
    saveSkillCooldownSettings(skillCooldownSettings.value);
    saveBossMechanicAlertSettings(bossMechanicSettings.value);
    saveEffectTimerSettings(effectTimerSettings.value);
    if (playerBuffUsesDefaults.value) {
        try { localStorage.removeItem(PLAYER_BUFF_STORAGE_KEY); } catch { /* ignore */ }
    } else {
        savePlayerBuffIds();
    }
    savePlayerBuffFavoriteIds();
    buffAlertSettingsDirty.value = false;
    debuffAlertSettingsDirty.value = false;
    skillCooldownSettingsDirty.value = false;
    aimReminderSettingsDirty.value = false;
    bossMechanicSettingsDirty.value = false;
    effectTimerSettingsDirty.value = false;
    bossMechanicSettingsSaved.value = false;
    effectTimerSettingsSaved.value = false;
    skillCooldownSettingsSaved.value = false;
    aimReminderSettingsSaved.value = false;
    reminderProfileSettingsDirty.value = false;
    scheduleNativeReminderSettingsSync(true);
    publishBuffOverlayState();
    publishDebuffOverlayState();
    publishSkillCooldownOverlayState();
}

function switchReminderProfile() {
    const nextId = selectedReminderProfileId.value;
    const previousId = reminderProfileStore.value.activeProfileId;
    if (nextId === previousId) return;
    saveCurrentReminderProfile(previousId);
    reminderProfileStore.value.activeProfileId = nextId;
    saveReminderProfileStore(reminderProfileStore.value);
    applyReminderProfile(nextId);
    showNotice(`已切换提醒方案：${activeReminderProfile.value?.name || "未命名方案"}`, "success");
}

function renameActiveReminderProfile(event: Event) {
    const profile = activeReminderProfile.value;
    if (!profile) return;
    profile.name = (event.target as HTMLInputElement).value
        .replace(/[\u0000-\u001f\u007f]/g, "")
        .slice(0, 32);
    reminderProfileSettingsDirty.value = true;
}

function duplicateReminderProfile() {
    saveCurrentReminderProfile();
    const current = activeReminderProfile.value;
    const baseName = (current?.name || "提醒方案").replace(/\s+副本(?:\s+\d+)?$/, "");
    let index = 2;
    let name = `${baseName} 副本`;
    const names = new Set(reminderProfileStore.value.profiles.map((profile) => profile.name));
    while (names.has(name)) name = `${baseName} 副本 ${index++}`;
    const profile = makeReminderProfile(name, currentReminderSnapshot());
    reminderProfileStore.value.profiles.push(profile);
    reminderProfileStore.value.activeProfileId = profile.id;
    selectedReminderProfileId.value = profile.id;
    saveReminderProfileStore(reminderProfileStore.value);
    reminderProfileSettingsDirty.value = false;
    showNotice(`已新建提醒方案：${profile.name}`, "success");
}

function deleteActiveReminderProfile() {
    const profiles = reminderProfileStore.value.profiles;
    if (profiles.length <= 1) return;
    const index = profiles.findIndex((profile) => profile.id === selectedReminderProfileId.value);
    if (index < 0) return;
    const [removed] = profiles.splice(index, 1);
    const next = profiles[Math.min(index, profiles.length - 1)];
    reminderProfileStore.value.activeProfileId = next.id;
    selectedReminderProfileId.value = next.id;
    saveReminderProfileStore(reminderProfileStore.value);
    applyReminderProfile(next.id);
    showNotice(`已删除提醒方案：${removed.name}`, "info");
}

function adjustBuffTime(delta: number) {
    const current = Math.round(Number(buffOverlaySettings.value.timeAdjustmentSeconds) || 0);
    buffOverlaySettings.value.timeAdjustmentSeconds = Math.min(3600, Math.max(-3600, current + delta));
    markBuffAlertSettingsDirty();
    publishBuffOverlayState();
}

function onBuffOverlayRuleChanged(_rule: BuffAlertRule) {
    markBuffAlertSettingsDirty();
    publishBuffOverlayState();
}

function onBuffSoundModeChanged(rule: BuffAlertRule) {
    markBuffAlertSettingsDirty();
    if (rule.soundMode !== "none" && (rule.soundMode !== "custom" || rule.customSoundId)) {
        void previewBuffAlertSound(rule.soundMode, true, rule.customSoundId);
    }
}

async function loadLocalTTSVoices() {
    if (localTtsVoices.value.length || localTtsVoicesLoading.value) return;
    localTtsVoicesLoading.value = true;
    localTtsStatus.value = "";
    localTtsError.value = false;
    try {
        const response = await fetch("/api/local_tts", { cache: "no-store" });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const result = await response.json() as { voices?: LocalTtsVoice[] };
        localTtsVoices.value = Array.isArray(result.voices) ? result.voices.filter((voice) => Boolean(voice.name)) : [];
        const preferred = localTtsVoices.value.find((voice) => voice.culture.toLowerCase() === "zh-cn")
            ?? localTtsVoices.value.find((voice) => voice.culture.toLowerCase().startsWith("zh"))
            ?? localTtsVoices.value[0];
        localTtsVoice.value = preferred?.name ?? "";
        if (!preferred) throw new Error("Windows 没有安装可用的系统语音");
    } catch (error) {
        localTtsError.value = true;
        localTtsStatus.value = `读取系统语音失败：${String(error)}`;
    } finally {
        localTtsVoicesLoading.value = false;
    }
}

function openLocalTTS(kind: LocalTtsTargetKind, id: number | string, label: string) {
    localTtsTarget.value = { kind, id, label };
    localTtsText.value = kind === "debuff"
        ? `${label}即将失效`
        : kind === "skill"
            ? `${label}可以使用了`
            : kind === "boss"
                ? `注意${label}`
                : kind === "buff-stack"
                    ? `${label}层数已达到`
                    : `${label}要结束了`;
    localTtsRate.value = 0;
    localTtsStatus.value = "";
    localTtsError.value = false;
    localTtsDialogOpen.value = true;
    void loadLocalTTSVoices();
}

async function generateAndApplyLocalTTS() {
    const target = localTtsTarget.value;
    if (!target || !localTtsText.value.trim() || !localTtsVoice.value || localTtsPending.value) return;
    localTtsPending.value = true;
    localTtsStatus.value = "正在使用 Windows 系统语音生成 WAV…";
    localTtsError.value = false;
    try {
        const response = await fetch("/api/local_tts", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ text: localTtsText.value, voice: localTtsVoice.value, rate: localTtsRate.value }),
        });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const generated = await response.json() as { soundId?: string; displayName?: string; voice?: string };
        if (!generated.soundId) throw new Error("生成结果缺少音效编号");
        applyLocalTTSSound(target, generated.soundId, generated.displayName || "本地TTS.wav");
        const volume = target.kind === "debuff"
            ? debuffAlertSettings.value.volume
            : target.kind === "boss"
                ? bossMechanicSettings.value.volume
                : buffOverlaySettings.value.volume;
        await previewBuffAlertSound("custom", false, generated.soundId, volume);
        localTtsStatus.value = `已生成并应用：${generated.displayName || "本地TTS.wav"}`;
        showNotice(`已为“${target.label}”应用本地 TTS。请记得保存设定。`, "success");
    } catch (error) {
        localTtsError.value = true;
        localTtsStatus.value = `生成失败：${String(error)}`;
    } finally {
        localTtsPending.value = false;
    }
}

function applyLocalTTSSound(target: LocalTtsTarget, soundId: string, displayName: string) {
    if (target.kind === "buff" || target.kind === "buff-stack") {
        const rule = buffOverlaySettings.value.rules[Number(target.id)];
        if (!rule) throw new Error("对应 Buff 提醒已经被删除");
        if (target.kind === "buff-stack") {
            rule.stackSoundMode = "custom";
            rule.stackCustomSoundId = soundId;
            rule.stackCustomSoundName = displayName;
        } else {
            rule.soundMode = "custom";
            rule.customSoundId = soundId;
            rule.customSoundName = displayName;
        }
        markBuffAlertSettingsDirty();
        return;
    }
    if (target.kind === "debuff") {
        const rule = debuffAlertSettings.value.rules[Number(target.id)];
        if (!rule) throw new Error("对应 Debuff 提醒已经被删除");
        rule.soundEnabled = true;
        rule.soundMode = "custom";
        rule.customSoundId = soundId;
        rule.customSoundName = displayName;
        markDebuffAlertSettingsDirty();
        return;
    }
    if (target.kind === "skill") {
        const rule = skillCooldownSettings.value.rules[Number(target.id)];
        if (!rule) throw new Error("对应技能提醒已经被删除");
        rule.soundMode = "custom";
        rule.customSoundId = soundId;
        rule.customSoundName = displayName;
        markSkillCooldownSettingsDirty();
        return;
    }
    const rule = bossMechanicSettings.value.rules[String(target.id)];
    if (!rule) throw new Error("对应 Boss 机制提醒已经被删除");
    rule.soundMode = "custom";
    rule.customSoundId = soundId;
    rule.customSoundName = displayName;
    markBossMechanicSettingsDirty();
}

async function uploadCustomBuffSound(rule: BuffAlertRule, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    const extension = file.name.split(".").pop()?.toLowerCase();
    if (extension !== "mp3" && extension !== "wav") {
        showNotice("自定义音效仅支持 MP3 或 WAV 文件。", "warning");
        return;
    }
    if (file.size <= 0 || file.size > 10 * 1024 * 1024) {
        showNotice("自定义音效文件需小于 10 MB。", "warning");
        return;
    }
    try {
        const body = new FormData();
        body.append("file", file, file.name);
        const response = await fetch("/api/buff_sound/upload", { method: "POST", body });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const uploaded = await response.json() as { soundId?: string; displayName?: string };
        if (!uploaded.soundId) throw new Error("上传结果缺少音效编号");
        rule.customSoundId = uploaded.soundId;
        rule.customSoundName = uploaded.displayName || file.name;
        rule.soundMode = "custom";
        markBuffAlertSettingsDirty();
        showNotice(`已保存自定义音效：${rule.customSoundName}`, "success");
        await previewBuffAlertSound("custom", false, rule.customSoundId);
    } catch (error) {
        showNotice(`自定义音效保存失败：${String(error)}`, "warning");
    }
}

async function uploadCustomBuffStackSound(rule: BuffAlertRule, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    const extension = file.name.split(".").pop()?.toLowerCase();
    if (extension !== "mp3" && extension !== "wav") {
        showNotice("自定义音效仅支持 MP3 或 WAV 文件。", "warning");
        return;
    }
    if (file.size <= 0 || file.size > 10 * 1024 * 1024) {
        showNotice("自定义音效文件需小于 10 MB。", "warning");
        return;
    }
    try {
        const body = new FormData();
        body.append("file", file, file.name);
        const response = await fetch("/api/buff_sound/upload", { method: "POST", body });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const uploaded = await response.json() as { soundId?: string; displayName?: string };
        if (!uploaded.soundId) throw new Error("上传结果缺少音效编号");
        rule.stackCustomSoundId = uploaded.soundId;
        rule.stackCustomSoundName = uploaded.displayName || file.name;
        rule.stackSoundMode = "custom";
        markBuffAlertSettingsDirty();
        showNotice(`已保存层数提示音：${rule.stackCustomSoundName}`, "success");
        await previewBuffAlertSound("custom", false, rule.stackCustomSoundId);
    } catch (error) {
        showNotice(`层数提示音保存失败：${String(error)}`, "warning");
    }
}

function onDebuffSoundModeChanged(rule: DebuffAlertRule) {
    markDebuffAlertSettingsDirty();
    if (rule.soundEnabled && (rule.soundMode !== "custom" || rule.customSoundId)) {
        void previewDebuffSound(rule, true);
    }
}

function onDebuffSoundSelectionChanged(rule: DebuffAlertRule, event: Event) {
    const value = (event.currentTarget as HTMLSelectElement).value;
    if (value === "none") {
        rule.soundEnabled = false;
    } else {
        rule.soundEnabled = true;
        rule.soundMode = value === "voice" || value === "custom" ? value : "electronic";
    }
    onDebuffSoundModeChanged(rule);
}

async function uploadCustomDebuffSound(rule: DebuffAlertRule, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    const extension = file.name.split(".").pop()?.toLowerCase();
    if (extension !== "mp3" && extension !== "wav") {
        showNotice("自定义音效仅支持 MP3 或 WAV 文件。", "warning");
        return;
    }
    if (file.size <= 0 || file.size > 10 * 1024 * 1024) {
        showNotice("自定义音效文件需小于 10 MB。", "warning");
        return;
    }
    try {
        const body = new FormData();
        body.append("file", file, file.name);
        const response = await fetch("/api/buff_sound/upload", { method: "POST", body });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const uploaded = await response.json() as { soundId?: string; displayName?: string };
        if (!uploaded.soundId) throw new Error("上传结果缺少音效编号");
        rule.customSoundId = uploaded.soundId;
        rule.customSoundName = uploaded.displayName || file.name;
        rule.soundMode = "custom";
        rule.soundEnabled = true;
        markDebuffAlertSettingsDirty();
        showNotice(`已保存 Debuff 提示音：${rule.customSoundName}`, "success");
        await previewDebuffSound(rule, false);
    } catch (error) {
        showNotice(`Debuff 音效保存失败：${String(error)}`, "warning");
    }
}

async function previewDebuffSound(rule: DebuffAlertRule, showSuccess = false) {
    if (!rule.soundEnabled || rule.soundMode === "none") return;
    await previewBuffAlertSound(
        rule.soundMode,
        showSuccess,
        rule.soundMode === "custom" ? rule.customSoundId : "",
        debuffAlertSettings.value.volume,
    );
}

function onSkillCooldownSoundModeChanged(rule: SkillCooldownRule) {
    markSkillCooldownSettingsDirty();
    if (rule.soundMode !== "none" && (rule.soundMode !== "custom" || rule.customSoundId)) {
        void previewSkillCooldownSound(rule, false);
    }
}

async function uploadCustomSkillCooldownSound(rule: SkillCooldownRule, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    const extension = file.name.split(".").pop()?.toLowerCase();
    if (extension !== "mp3" && extension !== "wav") {
        showNotice("自定义音效仅支持 MP3 或 WAV 文件。", "warning");
        return;
    }
    if (file.size <= 0 || file.size > 10 * 1024 * 1024) {
        showNotice("自定义音效文件需小于 10 MB。", "warning");
        return;
    }
    try {
        const body = new FormData();
        body.append("file", file, file.name);
        const response = await fetch("/api/buff_sound/upload", { method: "POST", body });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const uploaded = await response.json() as { soundId?: string; displayName?: string };
        if (!uploaded.soundId) throw new Error("上传结果缺少音效编号");
        rule.customSoundId = uploaded.soundId;
        rule.customSoundName = uploaded.displayName || file.name;
        rule.soundMode = "custom";
        markSkillCooldownSettingsDirty();
        showNotice(`已保存技能完成音效：${rule.customSoundName}`, "success");
        await previewSkillCooldownSound(rule, false);
    } catch (error) {
        showNotice(`技能完成音效保存失败：${String(error)}`, "warning");
    }
}

async function uploadCustomBossMechanicSound(rule: BossMechanicAlertRule, event: Event) {
    const input = event.target as HTMLInputElement;
    const file = input.files?.[0];
    input.value = "";
    if (!file) return;
    const extension = file.name.split(".").pop()?.toLowerCase();
    if (extension !== "mp3" && extension !== "wav") {
        showNotice("自定义音效仅支持 MP3 或 WAV 文件。", "warning");
        return;
    }
    if (file.size <= 0 || file.size > 10 * 1024 * 1024) {
        showNotice("自定义音效文件需小于 10 MB。", "warning");
        return;
    }
    try {
        const body = new FormData();
        body.append("file", file, file.name);
        const response = await fetch("/api/buff_sound/upload", { method: "POST", body });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        const uploaded = await response.json() as { soundId?: string; displayName?: string };
        if (!uploaded.soundId) throw new Error("上传结果缺少音效编号");
        rule.customSoundId = uploaded.soundId;
        rule.customSoundName = uploaded.displayName || file.name;
        rule.soundMode = "custom";
        markBossMechanicSettingsDirty();
        showNotice(`已保存 Boss 机制提示音：${rule.customSoundName}`, "success");
        await previewBuffAlertSound("custom", false, rule.customSoundId, bossMechanicSettings.value.volume);
    } catch (error) {
        showNotice(`Boss 机制音效保存失败：${String(error)}`, "warning");
    }
}

function removeBuffAlertRule(ccId: number) {
    delete buffOverlaySettings.value.rules[ccId];
    buffStackAboveThreshold.delete(ccId);
    buffStackMissingSince.delete(ccId);
    const runtime = { ...buffStackAlertRuntime.value };
    delete runtime[ccId];
    buffStackAlertRuntime.value = runtime;
    markBuffAlertSettingsDirty();
    publishBuffOverlayState();
}

function reloadBuffAlertSettings() {
    buffOverlaySettings.value = loadBuffOverlaySettings();
    buffAlertSettingsDirty.value = false;
    scheduleNativeReminderSettingsSync(true);
}

function reloadDebuffAlertSettings() {
    debuffAlertSettings.value = loadDebuffAlertSettings();
    debuffAlertSettingsDirty.value = false;
    scheduleNativeReminderSettingsSync(true);
    publishDebuffOverlayState();
}

function reloadExternalSkillCooldownSettings(event: Event) {
    const incoming = (event as CustomEvent).detail;
    if (incoming === skillCooldownSettings.value) return;
    skillCooldownSettings.value = loadSkillCooldownSettings();
    skillCooldownSettingsDirty.value = false;
    scheduleNativeReminderSettingsSync(true);
    publishSkillCooldownOverlayState(true);
}

interface NativeReminderDragState {
    sequence: number;
    kind: "buff" | "debuff" | "skill" | "aim" | "mechanic" | "target-health" | "effect" | "stack" | "";
    id: string;
    x: number;
    y: number;
    dragging: boolean;
    locked: boolean;
}

async function pollNativeReminderDragState() {
    if (isStandalone.value || buffOverlaySettings.value.locked) return;
    try {
        const response = await fetch("/api/reminder_overlay/drag_state", { cache: "no-store" });
        if (!response.ok) return;
        const state = await response.json() as NativeReminderDragState;
        const sequence = Math.round(Number(state.sequence) || 0);
        if (sequence <= nativeReminderDragSequence) return;
        nativeReminderDragSequence = sequence;
        const x = Math.min(32000, Math.max(-32000, Math.round(Number(state.x) || 0)));
        const y = Math.min(32000, Math.max(-32000, Math.round(Number(state.y) || 0)));
        let changed = false;
        if (state.kind === "skill") {
            const rule = skillCooldownSettings.value.rules[Number(state.id)];
            if (rule) {
                rule.x = x;
                rule.y = y;
                changed = true;
                if (!state.dragging) saveSkillCooldownSettings(skillCooldownSettings.value);
            }
        } else if (state.kind === "aim") {
            skillCooldownSettings.value.aimReminder.x = x;
            skillCooldownSettings.value.aimReminder.y = y;
            changed = true;
            if (!state.dragging) saveSkillCooldownSettings(skillCooldownSettings.value);
        } else if (state.kind === "mechanic") {
            const rule = bossMechanicSettings.value.rules[state.id];
            if (rule) {
                rule.x = x;
                rule.y = y;
                changed = true;
                if (!state.dragging) saveBossMechanicAlertSettings(bossMechanicSettings.value);
            }
        } else if (state.kind === "target-health") {
            bossMechanicSettings.value.mielShardHealthBarX = x;
            bossMechanicSettings.value.mielShardHealthBarY = y;
            changed = true;
            if (!state.dragging) saveBossMechanicAlertSettings(bossMechanicSettings.value);
        } else if (state.kind === "effect") {
            const rule = effectTimerSettings.value.rules[state.id];
            if (rule) {
                rule.x = x;
                rule.y = y;
                changed = true;
                if (!state.dragging) saveEffectTimerSettings(effectTimerSettings.value);
            }
        } else if (state.kind === "stack") {
            const rule = buffOverlaySettings.value.rules[Number(state.id)];
            if (rule) {
                rule.stackX = x;
                rule.stackY = y;
                changed = true;
                if (!state.dragging) saveBuffOverlaySettings(buffOverlaySettings.value);
            }
        }
        if (changed && !state.dragging) {
            scheduleNativeReminderSettingsSync(true);
        }
    } catch {
        // Native dragging is an optional desktop facility.
    }
}

function scheduleNativeReminderSettingsSync(immediate = false): Promise<boolean> {
    if (isStandalone.value) return Promise.resolve(true);
    if (nativeReminderSettingsSyncTimer !== undefined) {
        window.clearTimeout(nativeReminderSettingsSyncTimer);
        nativeReminderSettingsSyncTimer = undefined;
    }
    const sync = () => {
        nativeReminderSettingsSyncTimer = undefined;
        void syncNativeReminderSettings();
    };
    if (immediate) return syncNativeReminderSettings();
    nativeReminderSettingsSyncTimer = window.setTimeout(sync, 80);
    return Promise.resolve(true);
}

async function syncNativeReminderSettings(): Promise<boolean> {
    if (isStandalone.value) return true;
    const forcedBossRaceIds = Array.isArray(debuffAlertSettings.value.forcedBossRaceIds)
        ? debuffAlertSettings.value.forcedBossRaceIds
        : [];
    const buffRules = Object.fromEntries(Object.values(buffOverlaySettings.value.rules).map((rule) => [rule.ccId, {
        ...rule,
        name: conditionDisplayName(rule.ccId),
        iconUrl: conditionIconUrl(rule.ccId),
    }]));
    const debuffRules = Object.fromEntries(Object.values(debuffAlertSettings.value.rules).map((rule) => [rule.ccId, {
        ...rule,
        name: conditionDisplayName(rule.ccId),
        iconUrl: conditionIconUrl(rule.ccId),
    }]));
    const skillRules = Object.fromEntries(Object.values(skillCooldownSettings.value.rules).map((rule) => [rule.skillId, {
        ...rule,
        name: skillDisplayName(rule.skillId),
        iconUrl: skillIconUrl(rule.skillId),
    }]));
    const nativeBossRaceIds = new Set<number>([
        ...bossOptions.value.map((boss) => boss.raceId),
        ...forcedBossRaceIds,
        ...Object.values(bossMechanicSettings.value.rules).flatMap((rule) => rule.bossRaceIds),
    ]);
    const bossRaceNames = Object.fromEntries([...nativeBossRaceIds]
        .filter((raceId) => Number.isInteger(raceId) && raceId > 0)
        .map((raceId) => [raceId, debuffBossRaceDisplayName(raceId)]));
    const requestBody = JSON.stringify({
        preferredBossId: selectedBossId.value,
        preferredBossName: selectedBoss.value ? bossName(selectedBoss.value) : "",
        bossRaceNames,
        buff: {
            locked: buffOverlaySettings.value.locked,
            volume: buffOverlaySettings.value.volume,
            iconSize: buffOverlaySettings.value.iconSize,
            dpiPercent: buffOverlaySettings.value.dpiPercent,
            overlayEnabled: buffOverlaySettings.value.overlayEnabled,
            opacity: buffOverlaySettings.value.opacity,
            timeAdjustmentSeconds: buffOverlaySettings.value.timeAdjustmentSeconds,
            rules: buffRules,
        },
        debuff: {
            volume: debuffAlertSettings.value.volume,
            iconSize: debuffAlertSettings.value.iconSize,
            overlayEnabled: debuffAlertSettings.value.overlayEnabled,
            forcedBossRaceIds,
            rules: debuffRules,
        },
        skillCooldowns: {
            // Skill completion uses the common reminder volume in the
            // existing settings UI, but its timing is native-owned.
            volume: buffOverlaySettings.value.volume,
            iconSize: skillCooldownSettings.value.iconSize,
            aimReminder: {
                ...skillCooldownSettings.value.aimReminder,
                iconUrl: skillIconUrl(MAGNUM_SHOT_SKILL_ID),
            },
            rules: skillRules,
        },
        bossMechanics: {
            ...bossMechanicSettings.value,
        },
        effectTimers: {
            rules: Object.fromEntries(Object.values(effectTimerSettings.value.rules).map((rule) => [rule.key, {
                ...rule,
                iconUrl: rule.sourceType === "skill" ? skillIconUrl(rule.sourceId) : conditionIconUrl(rule.sourceId),
            }])),
        },
    });
    const request = nativeReminderSettingsSyncQueue.then(async () => {
        let failure: unknown = new Error("desktop reminder settings were rejected");
        for (let attempt = 0; attempt < 2; attempt += 1) {
            try {
                const response = await fetch("/api/reminder_runtime/settings", {
                    method: "PUT",
                    headers: { "Content-Type": "application/json" },
                    body: requestBody,
                });
                if (response.ok) return;
                failure = new Error(`desktop reminder settings were rejected (${response.status})`);
            } catch (error) {
                failure = error;
            }
            if (attempt === 0) await new Promise<void>((resolve) => window.setTimeout(resolve, 120));
        }
        throw failure;
    });
    // Keep every write ordered. An older startup sync can no longer arrive
    // after a newer button save and restore stale desktop rules.
    nativeReminderSettingsSyncQueue = request.catch(() => undefined);
    try {
        await request;
        return true;
    } catch {
        return false;
    }
}

function addSkillCooldownRule(selectedId: number) {
    const skillId = Number(selectedId);
    if (!Number.isInteger(skillId) || skillId <= 0) return;
    if (!skillCooldownSettings.value.rules[skillId]) {
        const index = Object.keys(skillCooldownSettings.value.rules).length;
        const horizontalStep = skillCooldownSettings.value.iconSize + 32;
        const verticalStep = skillCooldownSettings.value.iconSize + 48;
        skillCooldownSettings.value.rules[skillId] = makeSkillCooldownRule(
            skillId,
            600 + (index % 4) * horizontalStep,
            180 + Math.floor(index / 4) * verticalStep,
        );
    }
    skillCooldownSettings.value.rules[skillId].enabled = true;
    skillCooldownSettings.value.rules[skillId].barOnly = false;
    skillCooldownInput.value = "";
    markSkillCooldownSettingsDirty();
    publishSkillCooldownOverlayState();
}

function addFirstSkillCooldownSearchResult() {
    const result = skillCooldownSearchResults.value.find((item) => {
        const rule = skillCooldownSettings.value.rules[item.id];
        return !rule || rule.barOnly;
    });
    if (result) addSkillCooldownRule(result.id);
}

function removeSkillCooldownRule(skillId: number) {
    const barSlot = loadSkillBarSettings().slots.find((slot) => slot.skillId === skillId);
    if (barSlot) {
        const barRule = makeSkillCooldownRule(skillId);
        barRule.enabled = true;
        barRule.barOnly = true;
        barRule.alwaysVisible = false;
        barRule.soundMode = "none";
        barRule.cooldownSeconds = barSlot.cooldownSeconds;
        skillCooldownSettings.value.rules[skillId] = barRule;
    } else {
        delete skillCooldownSettings.value.rules[skillId];
        delete skillCooldownRuntime.value[skillId];
    }
    for (const key of announcedSkillReadySounds) {
        if (key.startsWith(`${skillId}:`)) announcedSkillReadySounds.delete(key);
    }
    markSkillCooldownSettingsDirty();
    publishSkillCooldownOverlayState();
}

function markSkillCooldownSettingsDirty() {
    skillCooldownSettingsDirty.value = true;
    skillCooldownSettingsSaved.value = false;
    reminderProfileSettingsDirty.value = true;
    publishSkillCooldownOverlayState();
}

function handleBossMechanicEvent(event: Event) {
    const detail = (event as CustomEvent<eventSkillAction>).detail;
    if (!detail) return;
    if (detail.EventId !== eventIdSkillAction) return;
    const action = detail as eventSkillAction;
    if (action.IsLocal !== false) return;
    if (action.MechanicSignal === "miel-orb-late-confirm") {
        const runtime = bossMechanicRuntime.value["miel-orb"];
        if (!runtime?.mielOrb) return;
        const nextOrb = observeMielOrbLateConfirmation(
            runtime.mielOrb,
            Number(action.AtMs) || detail.At * 1000,
        );
        if (nextOrb === runtime.mielOrb) return;
        bossMechanicRuntime.value["miel-orb"] = { ...runtime, mielOrb: nextOrb };
        publishSkillCooldownOverlayState();
        return;
    }
    const sourceId = action.SourceId || action.Id;
    const source = actorManager.value.entityMap[sourceId] as EntityActor | undefined;
    for (const rule of configuredBossMechanicRules.value) {
        if (!rule.enabled || rule.skillId !== action.SkillId) continue;
        if (rule.trigger === "orb-spawn" && action.IsFallback !== true) continue;
        if (rule.trigger === "skill-action" && action.IsFallback === true) continue;
        if (!bossMechanicSourceMatchesRule(source, rule)) continue;
        triggerBossMechanic(
            rule.key,
            Number(action.AtMs) || detail.At * 1000,
            false,
            bossMechanicBossForSource(source, rule),
        );
    }
}

function bossMechanicBossForSource(
    source: EntityActor | undefined,
    rule: BossMechanicAlertRule,
): EntityActor | undefined {
    if (source && rule.bossRaceIds.includes(Number(source.raceId))) return source;
    if (source?.ownerId) {
        const owner = actorManager.value.entityMap[source.ownerId] as EntityActor | undefined;
        if (owner && rule.bossRaceIds.includes(Number(owner.raceId))) return owner;
    }
    const selected = selectedBoss.value;
    return selected && rule.bossRaceIds.includes(Number(selected.raceId)) ? selected : undefined;
}

function bossMechanicSourceMatchesRule(source: EntityActor | undefined, rule: BossMechanicAlertRule): boolean {
    if (source) {
        if (rule.bossRaceIds.includes(Number(source.raceId))) return true;
        if (rule.triggerRaceIds.includes(Number(source.raceId))) {
            const owner = source.ownerId
                ? actorManager.value.entityMap[source.ownerId] as EntityActor | undefined
                : undefined;
            return !owner || rule.bossRaceIds.includes(Number(owner.raceId));
        }
        if (source.ownerId) {
            const owner = actorManager.value.entityMap[source.ownerId] as EntityActor | undefined;
            if (owner && rule.bossRaceIds.includes(Number(owner.raceId))) return true;
        }
        return false;
    }
    return rule.bossRaceIds.includes(Number(selectedBoss.value?.raceId) || 0);
}

function triggerBossMechanic(
    key: string,
    startedAtMs: number,
    force = false,
    sourceBoss?: EntityActor,
) {
    const rule = bossMechanicSettings.value.rules[key];
    if (!rule || (!force && !rule.enabled)) return;
    const atMs = Number.isFinite(startedAtMs) && startedAtMs > 0 ? startedAtMs : Date.now();
    const clusterWindowMs = rule.trigger === "orb-spawn" ? 1200 : 250;
    const previousAt = recentBossMechanicAt.get(key) ?? 0;
    if (!force && atMs - previousAt < clusterWindowMs) return;
    recentBossMechanicAt.set(key, atMs);
    const previous = bossMechanicRuntime.value[key];
    const boss = sourceBoss ?? bossMechanicBossForSource(undefined, rule);
    const bossRaceId = Number(boss?.raceId) || 0;
    const mielOrb = key === "miel-orb"
        ? createMielOrbRuntime(
            atMs,
            bossRaceId,
            boss?.statMap?.[28],
            boss?.statMap?.[30],
            rule.countdownSeconds,
        )
        : undefined;
    bossMechanicRuntime.value[key] = {
        startedAtMs: atMs,
        endsAtMs: atMs + Math.max(1, Number(rule.countdownSeconds) || 1) * 1000,
        generation: (previous?.generation ?? 0) + 1,
        bossId: boss?.id ?? "",
        bossRaceId,
        mielOrb,
    };
    // Desktop live events are sounded by the native runtime. A forced settings
    // preview remains explicit user interaction and may still use this path.
    if ((isStandalone.value || force)
        && rule.soundMode !== "none"
        && (rule.soundMode !== "custom" || rule.customSoundId)) {
        void playBossMechanicSound(rule.key, rule.soundMode, rule.customSoundId);
    }
    publishSkillCooldownOverlayState(force);
}

async function playBossMechanicSound(
    mechanicKey: string,
    mode: "dedicated" | "custom",
    soundId = "",
) {
    const volume = Math.min(100, Math.max(0, Math.round(Number(bossMechanicSettings.value.volume) || 0)));
    if (volume <= 0) return;
    try {
        const kind = mode === "custom" ? "custom" : `boss-${mechanicKey}`;
        const response = await fetch("/api/buff_sound", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ kind, soundId: mode === "custom" ? soundId : "", volume }),
        });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
    } catch (error) {
        showNotice(`Boss 机制音效播放失败：${String(error)}`, "warning");
    }
}

function localDorchaQuantity(): number | undefined {
    const id = actorManager.value.localEntityId;
    if (!id) return undefined;
    const value = actorManager.value.entityMap[id]?.statMap?.[DORCHA_STAT_ID]
        ?? actorManager.value.pendingStatMap[id]?.[DORCHA_STAT_ID];
    return typeof value === "number" && Number.isFinite(value) && value >= 0 && value <= 15 ? value : undefined;
}

function refreshDorchaQuantity(atMs: number) {
    if (dorchaPreviewUntilMs > atMs) return;
    const previous = dorchaQuantityRuntime.value;
    const next = applyDorchaQuantityObservation(previous, localDorchaQuantity(),
        skillCooldownSettings.value.rules[DORCHA_MASTERY_SKILL_ID]?.quantityThreshold, atMs);
    dorchaQuantityRuntime.value = next;
    if (next.generation > previous.generation) {
        skillCooldownRuntime.value[DORCHA_MASTERY_SKILL_ID] = {
            usedAtMs: next.triggeredAtMs, readyAtMs: next.triggeredAtMs, generation: next.generation,
        };
    }
}

function localToahSpiritProgress(): number | undefined {
    const localEntityId = actorManager.value.localEntityId;
    if (!localEntityId) return undefined;
    const actor = actorManager.value.entityMap[localEntityId] as EntityActor | undefined;
    const value = actor?.statMap?.[TOAH_SPIRIT_STAT_ID]
        ?? actorManager.value.pendingStatMap[localEntityId]?.[TOAH_SPIRIT_STAT_ID];
    const numeric = Number(value);
    return Number.isFinite(numeric) ? numeric : undefined;
}

function refreshTrackedSkillCooldownsFromToah(atMs: number) {
    for (const [rawSkillId, runtime] of Object.entries(skillCooldownRuntime.value)) {
        const skillId = Number(rawSkillId);
        if (!shouldRefreshSkillCooldownFromToah(skillId, runtime.petSkill === true) || runtime.readyAtMs <= atMs) continue;
        const generation = runtime.generation + 1;
        skillCooldownRuntime.value[skillId] = {
            ...runtime,
            readyAtMs: atMs,
            shortReadyAtMs: 0,
            accumulatedReadyAtMs: 0,
            accumulatedCooldownSeconds: 0,
            cooldownPhase: "idle",
            generation,
        };
        // The Toah reminder is the single audible cue. Mark each refreshed
        // timer announced so several custom CD sounds cannot overlap at 100%.
        announcedSkillReadySounds.add(`${skillId}:${generation}`);
    }
}

function refreshToahSpiritProgress(atMs: number) {
    if (toahSpiritPreviewUntilMs > atMs) return;
    const previous = toahSpiritProgressRuntime.value;
    const rule = skillCooldownSettings.value.rules[TOAH_SPIRIT_SKILL_ID];
    const next = applyToahSpiritProgressObservation(
        previous,
        localToahSpiritProgress(),
        rule?.progressThresholdPercent ?? DEFAULT_TOAH_PROGRESS_THRESHOLD,
        atMs,
    );
    toahSpiritProgressRuntime.value = next;

    if (next.generation > previous.generation) {
        skillCooldownRuntime.value[TOAH_SPIRIT_SKILL_ID] = {
            usedAtMs: next.triggeredAtMs,
            readyAtMs: next.triggeredAtMs,
            generation: next.generation,
        };
    }
    if (next.fullChargeGeneration > previous.fullChargeGeneration) {
        refreshTrackedSkillCooldownsFromToah(atMs);
    }
}

function handleSkillAction(event: Event) {
    const action = (event as CustomEvent<eventSkillAction>).detail;
    if (!action) return;
    const effectTargetMode = action.IsLocal === false ? "monster" : "self";
    for (const rule of configuredEffectTimerRules.value) {
        if (rule.enabled && rule.sourceType === "skill" && rule.sourceId === action.SkillId && rule.targetMode === effectTargetMode) {
            triggerEffectTimer(rule, Number(action.AtMs) || Number(action.At) * 1000, action.SourceId || action.Id);
        }
    }
    if (action.IsLocal === false) return;
    if (action.SkillId === MAGNUM_SHOT_SKILL_ID) observeMagnumAimShot(action);
    // Toah Spirit uses the authoritative Stat 198 gauge rather than a fixed
    // cooldown. The execute packet must not replace its progress reminder with
    // an ordinary countdown.
    if (action.SkillId === TOAH_SPIRIT_SKILL_ID || action.SkillId === DORCHA_MASTERY_SKILL_ID) return;
    const rule = skillCooldownSettings.value.rules[action.SkillId];
    if (!rule?.enabled) return;
    const previous = skillCooldownRuntime.value[action.SkillId];
    const next = applySkillCooldownObservation(rule, previous, action);
    if (next === previous) return;
    skillCooldownRuntime.value[action.SkillId] = {
        ...next,
        petSkill: skillActionIsPet(rule, action),
    };
    publishSkillCooldownOverlayState();
}

function handleEffectTimerCondition(event: Event) {
    const detail = (event as CustomEvent<eventCharacterConditionEnable | eventCharacterConditionDisable>).detail;
    if (!detail || (detail.EventId !== eventIdCharacterConditionEnable && detail.EventId !== eventIdCharacterConditionDisable)) return;
    const actor = actorManager.value.entityMap[detail.Id] as EntityActor | undefined;
    const selfTarget = detail.Id === actorManager.value.localEntityId || actor?.ownerId === actorManager.value.localEntityId;
    const targetMode = selfTarget ? "self" : "monster";
    for (const rule of configuredEffectTimerRules.value) {
        if (!rule.enabled || rule.sourceType !== "condition" || rule.sourceId !== detail.CCId || rule.targetMode !== targetMode) continue;
        if (detail.EventId === eventIdCharacterConditionEnable) {
            const enable = detail as eventCharacterConditionEnable;
            triggerEffectTimer(rule, Number(enable.At) * 1000, enable.Id);
        } else if (effectTimerRuntime.value[rule.key]?.targetId === detail.Id) {
            delete effectTimerRuntime.value[rule.key];
        }
    }
    publishSkillCooldownOverlayState();
}

function triggerEffectTimer(rule: EffectTimerRule, startedAtMs: number, targetId: string, force = false) {
    if (!force && !rule.enabled) return;
    const atMs = Number.isFinite(startedAtMs) && startedAtMs > 0 ? startedAtMs : Date.now();
    const previous = effectTimerRuntime.value[rule.key];
    effectTimerRuntime.value[rule.key] = {
        startedAtMs: atMs,
        endsAtMs: atMs + Math.max(100, Number(rule.durationSeconds) * 1000),
        generation: (previous?.generation ?? 0) + 1,
        targetId,
    };
    publishSkillCooldownOverlayState(force);
}

function handleSkillCooldownAdjustment(event: Event) {
    const adjustment = (event as CustomEvent<eventSkillCooldown>).detail;
    if (!adjustment || adjustment.SkillId === DORCHA_MASTERY_SKILL_ID) return;
    const rule = skillCooldownSettings.value.rules[adjustment.SkillId];
    if (!rule?.enabled) return;
    const previous = skillCooldownRuntime.value[adjustment.SkillId];
    const next = applySkillCooldownAdjustment(previous, adjustment);
    if (!next || next === previous) return;
    skillCooldownRuntime.value[adjustment.SkillId] = next;
    publishSkillCooldownOverlayState();
}

function handleSkillState(event: Event) {
    const state = (event as CustomEvent<eventSkillState>).detail;
    if (!state || state.Id !== actorManager.value.localEntityId) return;
    if (state.Scope === "active" && state.SkillId === FINAL_SHOT_SKILL_ID) {
        finalShotActive.value = state.Active;
        return;
    }
    if (state.Scope !== "aim" || state.SkillId !== MAGNUM_SHOT_SKILL_ID) return;
    const atMs = Number(state.AtMs) > 0 ? Number(state.AtMs) : Number(state.At) * 1000;
    if (!state.Active) {
        if (activeMagnumAimCycle.value) {
            recentMagnumAimCycle.value = { ...activeMagnumAimCycle.value, endedAtMs: atMs };
            activeMagnumAimCycle.value = null;
        }
        return;
    }
    if (!skillCooldownSettings.value.aimReminder.enabled) return;
    const timing = calculateMagnumAimTiming(skillCooldownSettings.value.aimReminder, {
        finalShot: finalShotActive.value,
        latikaSecret: Boolean(findLocalBuffCondition(actorManager.value, LATIKA_SECRET_CC_ID)),
        rapid: Boolean(findLocalBuffCondition(actorManager.value, RAPID_AIM_CC_ID)),
    });
    activeMagnumAimCycle.value = {
        startedAtMs: atMs,
        readyAtMs: atMs + timing.durationMs,
        calibrationPercent: skillCooldownSettings.value.aimReminder.calibrationPercent,
        targetId: state.TargetId ?? "",
    };
    recentMagnumAimCycle.value = null;
}

function observeMagnumAimShot(action: eventSkillAction) {
    if (!skillCooldownSettings.value.aimReminder.enabled) return;
    const atMs = Number(action.AtMs) > 0 ? Number(action.AtMs) : Number(action.At) * 1000;
    const actionKey = `${atMs}:${Number(action.CombatActionId) || 0}`;
    if (actionKey === lastMagnumAimActionKey) return;
    const active = activeMagnumAimCycle.value;
    const recent = recentMagnumAimCycle.value;
    const cycle = active && atMs >= active.startedAtMs
        ? active
        : recent && atMs >= recent.startedAtMs && atMs <= (recent.endedAtMs ?? atMs) + 2500
            ? recent
            : null;
    if (!cycle) return;
    const rate = calculateMagnumAimDisplayProgress(
        cycle.startedAtMs,
        cycle.readyAtMs,
        atMs,
        cycle.calibrationPercent,
    );
    magnumAimSamples.value = [...magnumAimSamples.value, { atMs, rate }].slice(-512);
    lastMagnumAimActionKey = actionKey;
    activeMagnumAimCycle.value = null;
    recentMagnumAimCycle.value = null;
}

function skillActionIsPet(rule: SkillCooldownRule, action: eventSkillAction): boolean {
    if (rule.ownerMode === "pet") return true;
    if (rule.ownerMode === "player") return false;
    const sourceId = action.SourceId || action.Id;
    if (!sourceId || sourceId === actorManager.value.localEntityId || isMarionetteDamageSkill(action.SkillId)) return false;
    const source = actorManager.value.entityMap[sourceId] as EntityActor | undefined;
    return source?.ownerId === actorManager.value.localEntityId;
}

function handleSkillDamage(event: Event) {
    const damage = (event as CustomEvent<eventDamage>).detail;
    if (!damage) return;
    const damageAtMs = Number(damage.AtMs) > 0 ? Number(damage.AtMs) : Number(damage.At) * 1000;
    const target = actorManager.value.entityMap[damage.TargetId] as EntityActor | undefined;
    if (target && !target.isPC) {
        reminderTargetId.value = target.id;
        const next = selectDamageBattleTarget({
            selectedId: selectedBossId.value,
            manuallyLockedId: manuallyLockedBossId.value,
        }, target.id, bossOptions.value.some((item) =>
            item.entityId === selectedBossId.value && item.active !== false));
        manuallyLockedBossId.value = next.manuallyLockedId;
        selectedBossId.value = next.selectedId;
    }
    observeActiveMielOrbContact(damage, damageAtMs);
    const rule = skillCooldownSettings.value.rules[damage.SkillId];
    if (!rule?.enabled) return;
    const source = actorManager.value.entityMap[damage.Id] as EntityActor | undefined;
    if (!isLocalCooldownDamage(
        damage.SkillId,
        damage.Id,
        actorManager.value.localEntityId,
        source?.ownerId ?? "",
    )) return;
    const previous = skillCooldownRuntime.value[damage.SkillId];
    const next = applySkillCooldownObservation(rule, previous, {
        At: damage.At,
        AtMs: damageAtMs,
        IsFallback: true,
    });
    if (next === previous) return;
    skillCooldownRuntime.value[damage.SkillId] = { ...next, petSkill: false };
    publishSkillCooldownOverlayState();
}

function observeActiveMielOrbContact(damage: eventDamage, atMs: number) {
    if (damage.SkillId !== 52407) return;
    const damageAmount = Number(damage.Damage);
    if (!Number.isFinite(damageAmount) || damageAmount >= 10_000) return;
    const runtime = bossMechanicRuntime.value["miel-orb"];
    if (!runtime?.mielOrb || runtime.endsAtMs <= atMs) return;
    if (runtime.bossId && damage.Id !== runtime.bossId) return;
    if (!runtime.bossId) {
        const source = actorManager.value.entityMap[damage.Id] as EntityActor | undefined;
        if (!source || Number(source.raceId) !== runtime.bossRaceId) return;
    }
    const nextOrb = observeMielOrbContact(runtime.mielOrb, atMs);
    if (nextOrb === runtime.mielOrb) return;
    bossMechanicRuntime.value["miel-orb"] = { ...runtime, mielOrb: nextOrb };
    publishSkillCooldownOverlayState();
}

function previewAimReminder() {
    const settings = skillCooldownSettings.value.aimReminder;
    if (!settings.enabled) return;
    const now = Date.now();
    const timing = calculateMagnumAimTiming(settings, {});
    magnumAimPreview.value = {
        active: true,
        startedAtMs: now,
        readyAtMs: now + timing.durationMs,
        speedMultiplier: timing.speedMultiplier,
        buffNames: timing.buffNames,
        calibrationPercent: settings.calibrationPercent,
        generation: (magnumAimPreview.value?.generation ?? 0) + 1,
    };
    if (magnumAimPreviewTimer !== undefined) window.clearTimeout(magnumAimPreviewTimer);
    magnumAimPreviewTimer = window.setTimeout(() => {
        magnumAimPreview.value = null;
        publishSkillCooldownOverlayState(true);
    }, timing.fullDurationMs + 1600);
    publishSkillCooldownOverlayState(true);
    showNotice("正在预览计算后的瞄准曲线；系统 70% 显示为 85% 最佳射击点。", "info");
}

function magnumAimCalculatedBestText() {
    return (calculateMagnumAimTiming(skillCooldownSettings.value.aimReminder, {}).durationMs / 1000).toFixed(3);
}

function magnumAimEffectiveRangeText() {
    return String(calculateMagnumAimTiming(skillCooldownSettings.value.aimReminder, {}).effectiveRange);
}

function previewSkillCooldown(skillId: number) {
    const previous = skillCooldownRuntime.value[skillId];
    const rule = skillCooldownSettings.value.rules[skillId];
    const now = Date.now();
    if (skillId === DORCHA_MASTERY_SKILL_ID) {
        const threshold = normalizeDorchaThreshold(rule?.quantityThreshold);
        dorchaPreviewUntilMs = now + 3600;
        dorchaQuantityRuntime.value = {
            observed: true, quantity: Math.max(0, threshold - 1), below: true, threshold,
            triggeredAtMs: now, generation: dorchaQuantityRuntime.value.generation + 1,
        };
        publishSkillCooldownOverlayState(true);
        if (rule) void previewSkillCooldownSound(rule, false);
        showNotice("正在预览多尔卡数量不足弹框，并试听已配置的提示音。", "info");
        return;
    }
    if (skillId === TOAH_SPIRIT_SKILL_ID) {
        const generation = (previous?.generation ?? toahSpiritProgressRuntime.value.generation) + 1;
        const threshold = Math.min(100, Math.max(1, Math.round(
            Number(rule?.progressThresholdPercent) || DEFAULT_TOAH_PROGRESS_THRESHOLD,
        )));
        toahSpiritPreviewUntilMs = now + 3600;
        toahSpiritProgressRuntime.value = {
            observed: true,
            progressPercent: threshold,
            thresholdPercent: threshold,
            thresholdReached: true,
            fullChargeReached: threshold >= 100,
            triggeredAtMs: now + 900,
            generation,
            fullChargeGeneration: toahSpiritProgressRuntime.value.fullChargeGeneration,
        };
        skillCooldownRuntime.value[skillId] = {
            usedAtMs: now - 1000,
            readyAtMs: now + 900,
            generation,
        };
        publishSkillCooldownOverlayState(true);
        if (rule) window.setTimeout(() => void previewSkillCooldownSound(rule, false), 900);
        showNotice("将在设定坐标预览一次托亚灵能量达标提示，并同步试听已配置音效。", "info");
        return;
    }
    skillCooldownRuntime.value[skillId] = {
        usedAtMs: now - 1000,
        // Leave a brief visible cooldown phase so the preview demonstrates
        // both the sweep/countdown state and the completion bounce.
        readyAtMs: now + 900,
        shortReadyAtMs: isCumulativeCooldownSkill(skillId) ? now + 300 : undefined,
        accumulatedCooldownSeconds: isCumulativeCooldownSkill(skillId)
            ? Number(rule?.cumulativeCooldownSeconds) || 10
            : undefined,
        cooldownPhase: "full",
        generation: (previous?.generation ?? 0) + 1,
    };
    publishSkillCooldownOverlayState(true);
    if (rule) window.setTimeout(() => void previewSkillCooldownSound(rule, false), 900);
    showNotice("将在设定坐标播放一次技能 CD 完成动画，并同步试听已配置音效。", "info");
}

function skillCooldownRuntimeStatus(skillId: number) {
    if (skillId === DORCHA_MASTERY_SKILL_ID) {
        const value = dorchaPreviewUntilMs > skillCooldownClock.value ? dorchaQuantityRuntime.value.quantity : localDorchaQuantity();
        if (value === undefined) return "等待捕捉自身多尔卡数量";
        const threshold = normalizeDorchaThreshold(skillCooldownSettings.value.rules[skillId]?.quantityThreshold);
        return `当前 ${formatDorchaQuantity(value)} / 15 · 少于 ${threshold} 点时提示`;
    }
    if (skillId === TOAH_SPIRIT_SKILL_ID) {
        const runtime = toahSpiritProgressRuntime.value;
        if (!runtime.observed) return "等待捕捉托亚灵能量";
        const progress = Math.round(runtime.progressPercent * 10) / 10;
        const threshold = skillCooldownSettings.value.rules[skillId]?.progressThresholdPercent
            ?? DEFAULT_TOAH_PROGRESS_THRESHOLD;
        return `当前 ${progress}% · 达到 ${threshold}% 时提示`;
    }
    const runtime = skillCooldownRuntime.value[skillId];
    if (!runtime && !actorManager.value.localEntityId) return "尚未确认本机角色；切换一次地图后再试";
    if (!runtime && isMarionetteDamageSkill(skillId)) return "等待捕捉本机人偶的首次成功技能流程";
    if (!runtime) return "等待捕捉本机角色的技能成功执行";
    const ownerLabel = runtime.petSkill ? "宠物技能 · " : "";
    const remaining = Math.max(0, runtime.readyAtMs - skillCooldownClock.value) / 1000;
    if (runtime.cooldownPhase === "accumulating") {
        const rule = skillCooldownSettings.value.rules[skillId];
        const accumulated = Math.round((Number(runtime.accumulatedCooldownSeconds) || 0) * 10) / 10;
        const limit = Math.round((Number(rule?.cumulativeCooldownSeconds) || 0) * 10) / 10;
        const shortAdd = Math.round((Number(rule?.shortCooldownSeconds) || 0) * 10) / 10;
        return `${ownerLabel}仍可使用 · 每次增加 ${shortAdd} 秒 · 累计 CD ${accumulated}/${limit} 秒（持续回落）`;
    }
    if (runtime.cooldownPhase === "idle" && isCumulativeCooldownSkill(skillId)) return `${ownerLabel}初始状态 · 可使用`;
    if (remaining > 0) return `${ownerLabel}冷却中，预计 ${remaining >= 10 ? Math.ceil(remaining) : Math.ceil(remaining * 10) / 10} 秒后完成`;
    return `${ownerLabel}CD 已完成`;
}

async function migrateLegacySkillOverlayPosition() {
    if (skillCooldownSettings.value.coordinateVersion >= 2) return;
    let baseX = 600;
    let baseY = 180;
    try {
        const response = await fetch("/api/skill_overlay/position", { cache: "no-store" });
        if (response.ok) {
            const position = await response.json() as { x?: number; y?: number };
            if (Number.isFinite(Number(position.x))) baseX = Math.round(Number(position.x));
            if (Number.isFinite(Number(position.y))) baseY = Math.round(Number(position.y));
        }
    } catch {
        // Standalone preview uses the original default coordinate.
    }
    const horizontalStep = skillCooldownSettings.value.iconSize + 32;
    const verticalStep = skillCooldownSettings.value.iconSize + 48;
    Object.values(skillCooldownSettings.value.rules).forEach((rule, index) => {
        rule.x = Math.min(32000, Math.max(-32000, baseX + (index % 4) * horizontalStep));
        rule.y = Math.min(32000, Math.max(-32000, baseY + Math.floor(index / 4) * verticalStep));
    });
    skillCooldownSettings.value.coordinateVersion = 2;
    saveSkillCooldownSettings(skillCooldownSettings.value);
    publishSkillCooldownOverlayState();
}

function publishSkillCooldownOverlayState(forceDesktopPreview = false) {
    const atMs = Date.now();
    skillCooldownClock.value = atMs;
    settleCumulativeSkillCooldowns(atMs);
    // The desktop overlay is authoritative native state. Only an explicit
    // user preview may publish one synthetic frame from the renderer.
    if (!isStandalone.value && !forceDesktopPreview) return;
    refreshToahSpiritProgress(atMs);
    refreshDorchaQuantity(atMs);
    announceCompletedSkillCooldowns(atMs);
    const activeOrbRuntime = bossMechanicRuntime.value["miel-orb"];
    if (activeOrbRuntime?.mielOrb) {
        const nextOrb = advanceMielOrbRuntime(activeOrbRuntime.mielOrb, atMs);
        if (nextOrb !== activeOrbRuntime.mielOrb) {
            bossMechanicRuntime.value["miel-orb"] = { ...activeOrbRuntime, mielOrb: nextOrb };
        }
    }
    const items = Object.values(skillCooldownSettings.value.rules)
        .filter((rule) => rule.enabled && rule.skillId !== DORCHA_MASTERY_SKILL_ID)
        .map((rule) => {
            const runtime = skillCooldownRuntime.value[rule.skillId] ?? { usedAtMs: 0, readyAtMs: 0, generation: 0 };
            const item = {
                skillId: rule.skillId,
                name: skillDisplayName(rule.skillId),
                iconUrl: skillIconUrl(rule.skillId),
                alwaysVisible: rule.alwaysVisible,
                barOnly: rule.barOnly === true,
                x: Math.min(32000, Math.max(-32000, Math.round(Number(rule.x) || 0))),
                y: Math.min(32000, Math.max(-32000, Math.round(Number(rule.y) || 0))),
                ...runtime,
            };
            if (rule.skillId === TOAH_SPIRIT_SKILL_ID) {
                return {
                    ...item,
                    progressPercent: toahSpiritProgressRuntime.value.progressPercent,
                    progressThresholdPercent: rule.progressThresholdPercent,
                    progressObserved: toahSpiritProgressRuntime.value.observed,
                };
            }
            if (isCumulativeCooldownSkill(rule.skillId)) {
                return {
                    ...item,
                    cumulativeCooldownSeconds: rule.cumulativeCooldownSeconds,
                };
            }
            return item;
        });
    const aimSettings = skillCooldownSettings.value.aimReminder;
    const activeAimPreview = magnumAimPreview.value?.active ? magnumAimPreview.value : undefined;
    const aimReminder = aimSettings.enabled && (activeAimPreview || aimSettings.alwaysVisible)
        ? {
            active: Boolean(activeAimPreview),
            alwaysVisible: Boolean(aimSettings.alwaysVisible),
            startedAtMs: activeAimPreview?.startedAtMs ?? 0,
            readyAtMs: activeAimPreview?.readyAtMs ?? 0,
            speedMultiplier: activeAimPreview?.speedMultiplier ?? 1,
            buffNames: activeAimPreview?.buffNames ?? [],
            calibrationPercent: activeAimPreview?.calibrationPercent
                ?? skillCooldownSettings.value.aimReminder.calibrationPercent,
            targetId: activeAimPreview ? "preview" : "",
            x: Math.min(32000, Math.max(-32000, Math.round(Number(aimSettings.x) || 0))),
            y: Math.min(32000, Math.max(-32000, Math.round(Number(aimSettings.y) || 0))),
            iconUrl: skillIconUrl(MAGNUM_SHOT_SKILL_ID),
            scalePercent: Math.min(200, Math.max(50, Math.round(Number(aimSettings.scalePercent) || 100))),
            generation: activeAimPreview?.generation ?? 0,
        }
        : undefined;
    const activeHealthPreview = mielShardHealthBarPreview.value?.endsAtMs && mielShardHealthBarPreview.value.endsAtMs > atMs
        ? mielShardHealthBarPreview.value
        : undefined;
    const targetHealth: TargetHealthBarOverlayItem | undefined = activeHealthPreview
        ? {
            entityId: "miel-shard-health-preview",
            name: "安乐碎片（预览）",
            phaseLabel: "普通 60%",
            currentHealth: 38_281_896,
            maximumHealth: 63_803_160,
            x: Math.min(32000, Math.max(-32000, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarX) || 0))),
            y: Math.min(32000, Math.max(-32000, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarY) || 0))),
            scalePercent: Math.min(200, Math.max(50, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarScalePercent) || 100))),
            opacityPercent: Math.min(100, Math.max(20, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarOpacityPercent) || 100))),
            previewExpiresAtMs: activeHealthPreview.endsAtMs,
            generation: activeHealthPreview.generation,
        }
        : selectedMielShardTargetHealth();
    const effectTimers = configuredEffectTimerRules.value.flatMap((rule) => {
        if (!rule.enabled) return [];
        const runtime = effectTimerRuntime.value[rule.key];
        if (!runtime && !rule.alwaysVisible) return [];
        return [{
            ...rule,
            ...(runtime ?? { startedAtMs: 0, endsAtMs: 0, generation: 0, targetId: "" }),
            iconUrl: rule.sourceType === "skill" ? skillIconUrl(rule.sourceId) : conditionIconUrl(rule.sourceId),
        }];
    });
    const mechanics = configuredBossMechanicRules.value
        .filter((rule) => rule.enabled && rule.showCountdown)
        .flatMap((rule) => {
            const runtime = bossMechanicRuntime.value[rule.key];
            if (!runtime || runtime.endsAtMs <= atMs) return [];
            if (runtime.mielOrb && !isMielOrbVisible(runtime.mielOrb, atMs)) return [];
            return [{
                key: rule.key,
                name: rule.name,
                icon: rule.trigger === "orb-spawn" ? "mdi-orbit" : "mdi-laser-pointer",
                // BroadcastChannel cannot structured-clone Vue's nested
                // reactive proxy (mielOrb).  The overlay only needs these
                // primitive lifecycle fields, so keep its payload plain.
                startedAtMs: runtime.startedAtMs,
                endsAtMs: runtime.endsAtMs,
                generation: runtime.generation,
                x: Math.min(32000, Math.max(-32000, Math.round(Number(rule.x) || 0))),
                y: Math.min(32000, Math.max(-32000, Math.round(Number(rule.y) || 0))),
                scalePercent: Math.min(200, Math.max(50, Math.round(Number(bossMechanicSettings.value.scalePercent) || 100))),
            }];
        });
    const stackAlerts = Object.entries(buffStackAlertRuntime.value)
        .filter(([, runtime]) => runtime.endsAtMs > atMs)
        .map(([rawCcId, runtime]) => {
            const ccId = Number(rawCcId);
            const rule = buffOverlaySettings.value.rules[ccId];
            return {
                ccId,
                name: conditionDisplayName(ccId),
                ...runtime,
                x: Math.min(32000, Math.max(-32000, Math.round(Number(rule?.stackX ?? 850)))),
                y: Math.min(32000, Math.max(-32000, Math.round(Number(rule?.stackY ?? 280)))),
                scalePercent: Math.min(200, Math.max(50, Math.round(Number(bossMechanicSettings.value.scalePercent) || 100))),
            };
        });
    const quantityAlert = dorchaQuantityOverlayItem(dorchaQuantityRuntime.value, skillCooldownSettings.value.rules[DORCHA_MASTERY_SKILL_ID]);
    const message: SkillCooldownOverlayMessage = {
        type: "skill-cooldown-state",
        atMs,
        items,
        aimReminder,
        targetHealth,
        effectTimers,
        mechanics,
        stackAlerts: quantityAlert ? [...stackAlerts, quantityAlert] : stackAlerts,
        settings: {
            iconSize: skillCooldownSettings.value.iconSize,
            overlayEnabled: buffOverlaySettings.value.overlayEnabled,
            dpiPercent: buffOverlaySettings.value.dpiPercent,
            opacity: buffOverlaySettings.value.opacity,
        },
    };
    if (skillOverlayChannel) {
        try {
            skillOverlayChannel.postMessage(message);
        } catch {
            // A future reactive field must not interrupt the main report UI.
            // JSON materializes a plain fallback while the native state API
            // below continues to update the desktop overlay independently.
            try {
                skillOverlayChannel.postMessage(JSON.parse(JSON.stringify(message)) as SkillCooldownOverlayMessage);
            } catch { /* the native overlay endpoint remains available */ }
        }
    }
    const stateKey = JSON.stringify({ items, aimReminder, targetHealth, effectTimers, mechanics, stackAlerts, settings: message.settings });
    if (stateKey === skillOverlayStateKey) return;
    skillOverlayStateKey = stateKey;
    void fetch("/api/skill_overlay/state", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(message),
    }).catch(() => undefined);
}

function selectedMielShardTargetHealth(): TargetHealthBarOverlayItem | undefined {
    const selectedTargetId = actorManager.value.selectedTargetId;
    const selectedTarget = selectedTargetId
        ? actorManager.value.entityMap[selectedTargetId] as EntityActor | undefined
        : undefined;
    const selectedCurrentHealth = selectedTarget?.statMap[CURRENT_HEALTH_STAT_ID];
    const selectedMaximumHealth = selectedTarget?.statMap[MAXIMUM_HEALTH_STAT_ID];
    const selectedTargetOwner = selectedTarget?.ownerId
        ? actorManager.value.entityMap[selectedTarget.ownerId] as EntityActor | undefined
        : undefined;
    const ownerCurrentHealth = selectedTargetOwner?.statMap[CURRENT_HEALTH_STAT_ID];
    const ownerMaximumHealth = selectedTargetOwner?.statMap[MAXIMUM_HEALTH_STAT_ID];
    const ownerRatio = Number(ownerCurrentHealth) / Number(ownerMaximumHealth);
    let phase: "normal80" | "normal60" | "normal40" | "regret80" | undefined;
    if (selectedTargetOwner?.raceId === 7603 && selectedTarget && selectedTarget.raceId >= 7604 && selectedTarget.raceId <= 7606) {
        if (ownerRatio <= 0.400001) phase = "normal40";
        else if (ownerRatio <= 0.600001) phase = "normal60";
        else if (ownerRatio <= 0.800001) phase = "normal80";
    } else if (selectedTargetOwner?.raceId === 7615 && selectedTarget && selectedTarget.raceId >= 7616 && selectedTarget.raceId <= 7619 && ownerRatio <= 0.800001) {
        phase = "regret80";
    }
    const recognized = selectedTarget
        && phase
        && selectedTargetOwner
        && bossMechanicSettings.value.mielShardHealthPhases[phase]
        && Number.isFinite(selectedCurrentHealth)
        && selectedCurrentHealth! > 0
        && actorManager.value.activeEntityMap[selectedTargetOwner.id] === true
        && Number.isFinite(ownerCurrentHealth)
        && Number.isFinite(ownerMaximumHealth)
        && ownerCurrentHealth! > 0
        && ownerMaximumHealth! > 0;
    if (!recognized || !selectedTarget || actorManager.value.activeEntityMap[selectedTarget.id] !== true || selectedTarget.isPC) {
        return undefined;
    }
    return {
        entityId: selectedTarget.id,
        name: targetHealthName(selectedTarget),
        phaseLabel: phase === "normal80" ? "普通 80%" : phase === "normal60" ? "普通 60%" : phase === "normal40" ? "普通 40%" : "悔恨 80%",
        currentHealth: Math.max(0, Math.min(selectedMaximumHealth!, selectedCurrentHealth!)),
        maximumHealth: selectedMaximumHealth!,
        x: Math.min(32000, Math.max(-32000, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarX) || 0))),
        y: Math.min(32000, Math.max(-32000, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarY) || 0))),
        scalePercent: Math.min(200, Math.max(50, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarScalePercent) || 100))),
        opacityPercent: Math.min(100, Math.max(20, Math.round(Number(bossMechanicSettings.value.mielShardHealthBarOpacityPercent) || 100))),
    };
}

function settleCumulativeSkillCooldowns(atMs: number) {
    for (const rule of Object.values(skillCooldownSettings.value.rules)) {
        const previous = skillCooldownRuntime.value[rule.skillId];
        const next = settleSkillCooldownRuntime(rule, previous, atMs);
        if (next && next !== previous) skillCooldownRuntime.value[rule.skillId] = next;
    }
}

function announceCompletedSkillCooldowns(atMs: number) {
    const readyRules: SkillCooldownRule[] = [];
    for (const [skillId, runtime] of Object.entries(skillCooldownRuntime.value)) {
        if (runtime.generation <= 0 || runtime.readyAtMs <= 0 || atMs < runtime.readyAtMs) continue;
        if (runtime.cooldownPhase === "accumulating") continue;
        const key = `${skillId}:${runtime.generation}`;
        if (announcedSkillReadySounds.has(key)) continue;
        announcedSkillReadySounds.add(key);
        const rule = skillCooldownSettings.value.rules[Number(skillId)];
        if (rule?.enabled && rule.soundMode !== "none" && atMs - runtime.readyAtMs <= SKILL_READY_SOUND_WINDOW_MS) {
            readyRules.push(rule);
        }
    }
    while (announcedSkillReadySounds.size > 256) {
        const oldest = announcedSkillReadySounds.values().next().value as string | undefined;
        if (!oldest) break;
        announcedSkillReadySounds.delete(oldest);
    }
    // The desktop host announces readiness even while WebView is suspended.
    // Standalone/replay pages keep the browser implementation for previews.
    if (isStandalone.value) {
        for (const rule of readyRules) void playSkillReadySound(rule);
    }
}

async function previewSkillCooldownSound(rule: SkillCooldownRule, showSuccess: boolean) {
    if (rule.soundMode === "none") {
        if (showSuccess) showNotice("该技能已设为不播放完成音效。", "info");
        return;
    }
    if (rule.soundMode === "custom" && !rule.customSoundId) {
        if (showSuccess) showNotice("请先选择 MP3 或 WAV 音效文件。", "warning");
        return;
    }
    const played = await playSkillReadySound(rule, showSuccess);
    if (played && showSuccess) showNotice("已播放技能 CD 完成音效。", "success");
}

async function playSkillReadySound(
    rule: Pick<SkillCooldownRule, "soundMode" | "customSoundId">,
    showError = false,
): Promise<boolean> {
    const volume = Math.min(100, Math.max(0, Math.round(Number(buffOverlaySettings.value.volume) || 0)));
    if (volume <= 0 || rule.soundMode === "none") {
        if (showError && volume <= 0) showNotice("提醒音量当前为 0%。", "warning");
        return false;
    }
    const custom = rule.soundMode === "custom";
    if (custom && !rule.customSoundId) return false;
    try {
        const response = await fetch("/api/buff_sound", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                kind: custom ? "custom" : "skill-ready",
                soundId: custom ? rule.customSoundId : "",
                volume,
            }),
        });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        return true;
    } catch (error) {
        // Standalone browser previews do not have the native Windows audio endpoint.
        if (!custom) {
            try {
                const audio = new Audio("/audio/skill-ready-pop.mp3");
                audio.volume = volume / 100;
                activeBuffAudio.add(audio);
                const cleanup = () => activeBuffAudio.delete(audio);
                audio.addEventListener("ended", cleanup, { once: true });
                audio.addEventListener("error", cleanup, { once: true });
                await audio.play();
                return true;
            } catch {
                // Fall through to the optional user-facing error below.
            }
        }
        if (showError) showNotice(`技能完成音效播放失败：${String(error)}`, "warning");
        return false;
    }
}

async function previewBuffAlertSound(sound: Exclude<BuffSoundMode, "none">, showSuccess = false, soundId = "", volumeOverride?: number) {
    try {
        const volume = Math.min(100, Math.max(0, Math.round(Number(volumeOverride ?? buffOverlaySettings.value.volume) || 0)));
        if (volume <= 0) {
            if (showSuccess) showNotice("提醒音量当前为 0%。", "warning");
            return;
        }
        // Always use the native audio endpoint first. HTMLAudio playback is
        // paused/throttled with a minimized WebView, while the native player
        // remains independent of the window visibility state.
        const response = await fetch("/api/buff_sound", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ kind: sound, soundId, volume }),
        });
        if (!response.ok) throw new Error((await response.text()).trim() || `HTTP ${response.status}`);
        if (showSuccess) showNotice("已播放提醒音效。", "success");
    } catch (error) {
        showNotice(`音效播放失败：${error}`, "warning");
    }
}

type ActiveBuffCondition = {
    actor: EntityActor | null;
    actorName: string;
    condition: EntityCondition;
};

function findActiveBuffCondition(ccId: number): ActiveBuffCondition | null {
    const match = findLocalBuffCondition(actorManager.value, ccId);
    if (!match) return null;
    const actor = match.actor as EntityActor | null;
    return {
        actor,
        actorName: actor ? getDisplayName(actor.name) : "当前角色",
        condition: match.condition as EntityCondition,
    };
}

function buffAlertRuntimeStatus(rule: BuffAlertRule) {
    const active = findActiveBuffCondition(rule.ccId);
    if (!active) return isStackOnlyBuffAlertCondition(rule.ccId) ? "等待识别状态层数" : "等待识别此 Buff";
    const stack = parseConditionStack(active.condition.Metadata);
    if (isStackOnlyBuffAlertCondition(rule.ccId)) {
        return stack === null
            ? `已识别：${active.actorName}；未读取到层数`
            : `已识别：${active.actorName}；当前 ${stack} 层 / ${rule.stackThreshold} 层提醒`;
    }
    const stackText = stack === null ? "" : `；当前 ${stack} 层`;
    if (rule.durationMode === "auto" && resolveBuffExpiresAt(active.condition, rule, buffOverlaySettings.value.timeAdjustmentSeconds) === null) {
        return `已识别：${active.actorName}${stackText}；不能识别持续时间`;
    }
    return `已识别：${active.actorName}${stackText}`;
}

function isStackAlertCondition(ccId: number) {
    return STACK_ALERT_CC_IDS.has(ccId);
}

function previewBuffStackAlert(rule: BuffAlertRule) {
    const nowMs = Date.now();
    const threshold = Math.min(99, Math.max(1, Math.round(Number(rule.stackThreshold) || 1)));
    const previous = buffStackAlertRuntime.value[rule.ccId];
    buffStackAlertRuntime.value[rule.ccId] = {
        stack: threshold,
        startedAtMs: nowMs,
        endsAtMs: nowMs + 3000,
        generation: (previous?.generation ?? 0) + 1,
    };
    publishSkillCooldownOverlayState(true);
    if (rule.stackSoundMode !== "none" && (rule.stackSoundMode !== "custom" || rule.stackCustomSoundId)) {
        void previewBuffAlertSound(rule.stackSoundMode, false, rule.stackCustomSoundId);
    }
    showNotice("将在设定坐标预览一次层数提醒动画，并同步试听已配置音效。", "info");
}

function processBuffStackAlerts(nowMs: number, allowSound = true) {
    for (const rule of Object.values(buffOverlaySettings.value.rules)) {
        if (!isStackAlertCondition(rule.ccId) || !rule.stackAlertEnabled) {
            buffStackAboveThreshold.delete(rule.ccId);
            buffStackMissingSince.delete(rule.ccId);
            continue;
        }
        const active = findActiveBuffCondition(rule.ccId);
        const stack = parseConditionStack(active?.condition.Metadata);
        if (stack === null) {
            const missingSince = buffStackMissingSince.get(rule.ccId) ?? nowMs;
            buffStackMissingSince.set(rule.ccId, missingSince);
            if (nowMs - missingSince >= 1500) buffStackAboveThreshold.set(rule.ccId, false);
            continue;
        }
        buffStackMissingSince.delete(rule.ccId);
        const threshold = Math.min(99, Math.max(1, Math.round(Number(rule.stackThreshold) || 1)));
        const above = stack >= threshold;
        const wasAbove = buffStackAboveThreshold.get(rule.ccId) ?? false;
        buffStackAboveThreshold.set(rule.ccId, above);
        if (!above || wasAbove) continue;

        const previous = buffStackAlertRuntime.value[rule.ccId];
        if (rule.stackScreenEnabled) {
            buffStackAlertRuntime.value[rule.ccId] = {
                stack,
                startedAtMs: nowMs,
                endsAtMs: nowMs + 3000,
                generation: (previous?.generation ?? 0) + 1,
            };
        }
        if (allowSound && rule.stackSoundMode !== "none" && (rule.stackSoundMode !== "custom" || rule.stackCustomSoundId)) {
            void previewBuffAlertSound(rule.stackSoundMode, false, rule.stackCustomSoundId);
        }
    }
}

function processBuffSoundAlerts() {
    const now = Date.now() / 1000;
    for (const rule of Object.values(buffOverlaySettings.value.rules)) {
        if (rule.soundMode === "none") continue;
        const active = findActiveBuffCondition(rule.ccId);
        if (!active) continue;
        const condition = active.condition;
        const expiresAt = resolveBuffExpiresAt(condition, rule, buffOverlaySettings.value.timeAdjustmentSeconds);
        if (expiresAt === null) continue;
        const remaining = expiresAt - now;
        if (remaining <= 0 || remaining > rule.soundThresholdSeconds) continue;
        const key = `${rule.ccId}:${condition.At}:${expiresAt}:${rule.soundMode}:${rule.customSoundId}:${rule.soundThresholdSeconds}`;
        if (announcedBuffSounds.has(key)) continue;
        announcedBuffSounds.add(key);
        if (rule.soundMode === "custom" && !rule.customSoundId) continue;
        void previewBuffAlertSound(rule.soundMode, false, rule.customSoundId);
    }
}

function publishBuffOverlayState() {
    // Desktop reminders are produced by the Go runtime directly from packet
    // events. Keeping this renderer path active would duplicate sounds and
    // allow a suspended WebView to overwrite the authoritative native state.
    if (!isStandalone.value) return;
    processBuffStackAlerts(Date.now(), true);
    processBuffSoundAlerts();
    const now = Date.now() / 1000;
    const items: BuffOverlayItem[] = [];
    for (const rule of Object.values(buffOverlaySettings.value.rules)) {
        if (!rule.overlayEnabled) continue;
        const active = findActiveBuffCondition(rule.ccId);
        if (active) {
            const condition = active.condition;
            items.push({
                ccId: rule.ccId,
                name: conditionDisplayName(rule.ccId),
                iconUrl: conditionIconUrl(rule.ccId),
                appliedAt: condition.At,
                expiresAt: resolveBuffExpiresAt(condition, rule, buffOverlaySettings.value.timeAdjustmentSeconds),
                flashEnabled: rule.flashEnabled,
                flashThresholdSeconds: rule.flashThresholdSeconds,
                active: true,
            });
        } else {
            items.push({
                ccId: rule.ccId,
                name: conditionDisplayName(rule.ccId),
                iconUrl: conditionIconUrl(rule.ccId),
                appliedAt: 0,
                expiresAt: null,
                flashEnabled: rule.flashEnabled,
                flashThresholdSeconds: rule.flashThresholdSeconds,
                active: false,
            });
        }
    }
    const message: BuffOverlayMessage = {
        type: "buff-state",
        at: now,
        items,
        settings: {
            locked: buffOverlaySettings.value.locked,
            iconSize: buffOverlaySettings.value.iconSize,
            dpiPercent: buffOverlaySettings.value.dpiPercent,
            overlayEnabled: buffOverlaySettings.value.overlayEnabled,
            opacity: buffOverlaySettings.value.opacity,
        },
    };
    buffOverlayChannel?.postMessage(message);
    const stateKey = JSON.stringify({ items, settings: message.settings });
    if (stateKey !== buffOverlayStateKey) {
        buffOverlayStateKey = stateKey;
        void fetch("/api/buff_overlay/state", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(message),
        }).catch(() => undefined);
    }
}

function publishDebuffOverlayState() {
    if (!isStandalone.value) return;
    const now = Date.now() / 1000;
    const items: DebuffOverlayItem[] = [];
    const target = debuffBossTarget.value;
    const boss = target
        ? actorManager.value.entityMap[target.entityId] as EntityActor | undefined
        : undefined;
    const appearedAt = boss
        ? boss.takeDamages.reduce((first, damage) => damage.Damage > 0 && (first === 0 || damage.At < first) ? damage.At : first, 0)
        : 0;
    const overlayBoss = debuffAlertSettings.value.overlayEnabled && boss ? {
        entityId: boss.id,
        name: bossName(boss),
        appearedAt,
    } : null;

    // Debuff reminders only arm for one active target whose estimated health
    // is at least 100 million.  This prevents ordinary monsters and parallel
    // adds from repeatedly rearming missing/expiring sounds.
    if (overlayBoss && boss) {
        for (const rule of configuredDebuffAlertRules.value) {
            if (!rule.enabled) continue;
            const condition = findActiveBossDebuffCondition(rule.ccId, boss);
            if (condition) observedDebuffApplications.add(`${boss.id}:${canonicalDebuffId(rule.ccId)}`);
            const expiresAt = rule.flashEnabled ? resolveDebuffExpiresAt(condition, now) : null;
            const decision = evaluateDebuffAlert(Boolean(condition), expiresAt, now, rule.warningSeconds);
            if (decision === "hidden") continue;
            items.push({
                ccId: rule.ccId,
                name: conditionDisplayName(rule.ccId),
                iconUrl: conditionIconUrl(rule.ccId),
                appliedAt: condition?.At ?? appearedAt,
                expiresAt,
                warningSeconds: rule.warningSeconds,
                state: decision,
            });
        }
    }

    processDebuffSoundAlerts(items, overlayBoss?.entityId ?? "");
    const message: DebuffOverlayMessage = {
        type: "debuff-state",
        at: now,
        boss: overlayBoss,
        targetHealth: null,
        items,
        settings: {
            iconSize: debuffAlertSettings.value.iconSize,
            overlayEnabled: debuffAlertSettings.value.overlayEnabled,
            dpiPercent: buffOverlaySettings.value.dpiPercent,
            opacity: buffOverlaySettings.value.opacity,
        },
    };
    debuffOverlayChannel?.postMessage(message);
    const stateKey = JSON.stringify({ boss: overlayBoss, items, settings: message.settings });
    if (stateKey !== debuffOverlayStateKey) {
        debuffOverlayStateKey = stateKey;
        void fetch("/api/debuff_overlay/state", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(message),
        }).catch(() => undefined);
    }
}

function findActiveBossDebuffCondition(ccId: number, boss?: EntityActor): EntityCondition | undefined {
    if (!boss) return undefined;
    const acceptedIds = equivalentDebuffIds(ccId);
    for (const equivalentId of acceptedIds) {
        const direct = boss.conditionMap[equivalentId];
        if (direct) return direct;
        const pendingDirect = actorManager.value.pendingConditionMap[boss.id]?.[equivalentId];
        if (pendingDirect) return pendingDirect;
    }

    // Multipart bosses may attach a Debuff to a body/part or a Boss-owned
    // helper.  Never scan unrelated damaged monsters: doing so made parallel
    // targets hide/rearm one another's reminders and replay their sounds.
    const ownedPartIds = Object.values(actorManager.value.entityMap)
        .filter((actor) => actor.ownerId === boss.id || Boolean(boss.group.entityMap[actor.ownerId]))
        .map((actor) => actor.id);
    const candidateIds = new Set<string>([
        boss.id,
        ...Object.keys(boss.group.entityMap),
        ...ownedPartIds,
    ]);
    let latest: EntityCondition | undefined;
    for (const id of candidateIds) {
        const actor = actorManager.value.entityMap[id];
        if (actor?.isPC) continue;
        for (const equivalentId of acceptedIds) {
            const condition = actor?.conditionMap[equivalentId] ?? actorManager.value.pendingConditionMap[id]?.[equivalentId];
            if (condition && (!latest || condition.At > latest.At)) latest = condition;
        }
    }
    return latest;
}

function resolveDebuffExpiresAt(condition: EntityCondition | undefined, now: number): number | null {
    if (!condition) return null;
    const millisecondExpiry = Number(condition.DisableAtMs) / 1000;
    if (Number.isFinite(millisecondExpiry) && millisecondExpiry > now && millisecondExpiry - now < 86400) return millisecondExpiry;
    const secondExpiry = Number(condition.DisableAt);
    if (Number.isFinite(secondExpiry) && secondExpiry > now && secondExpiry - now < 86400) return secondExpiry;
    return null;
}

function processDebuffSoundAlerts(items: DebuffOverlayItem[], bossId: string) {
    const currentKeys = new Set<string>();
    if (!bossId) {
        announcedDebuffSounds.clear();
        observedDebuffApplications.clear();
        return;
    }
    const candidates: Array<{
        item: DebuffOverlayItem;
        rule: DebuffAlertRule;
        state: "expiring" | "missing";
        hasBeenApplied: boolean;
        soundEnabled: boolean;
        soundMode: DebuffAlertRule["soundMode"];
        customSoundId: string;
    }> = [];
    for (const item of items) {
        const key = `${bossId}:${item.ccId}:${item.state}:${item.appliedAt}`;
        currentKeys.add(key);
        if (announcedDebuffSounds.has(key)) continue;
        announcedDebuffSounds.add(key);
        const rule = debuffAlertSettings.value.rules[item.ccId];
        if (!rule || debuffAlertSettings.value.volume <= 0) continue;
        candidates.push({
            item,
            rule,
            state: item.state,
            hasBeenApplied: observedDebuffApplications.has(`${bossId}:${canonicalDebuffId(item.ccId)}`),
            soundEnabled: rule.soundEnabled,
            soundMode: rule.soundMode,
            customSoundId: rule.customSoundId,
        });
    }
    for (const key of announcedDebuffSounds) {
        if (!currentKeys.has(key)) announcedDebuffSounds.delete(key);
    }
    const nowMs = Date.now();
    const selected = selectDebuffSoundCandidate(
        candidates,
        nowMs,
        debuffSoundBlockedUntilMs,
        debuffSoundPlaying,
    );
    if (!selected) return;
    // Every item in this publish cycle has already been marked announced. New
    // expiries arriving while playback/cooldown is active are intentionally
    // dropped rather than queued and replayed one by one.
    debuffSoundPlaying = true;
    debuffSoundBlockedUntilMs = nowMs + DEBUFF_SOUND_MERGE_COOLDOWN_MS;
    void previewDebuffSound(selected.rule, false).finally(() => {
        debuffSoundPlaying = false;
        debuffSoundBlockedUntilMs = Math.max(
            debuffSoundBlockedUntilMs,
            Date.now() + DEBUFF_SOUND_MERGE_COOLDOWN_MS,
        );
    });
}

function resetPlayerBuffs() {
    playerBuffUsesDefaults.value = true;
    playerBuffIds.value = [];
    playerBuffInput.value = "";
    playerBuffInputError.value = "";
    playerBuffFavoriteIds.value = [];
    try { localStorage.removeItem(PLAYER_BUFF_STORAGE_KEY); } catch { /* ignore */ }
    try { localStorage.removeItem(PLAYER_BUFF_FAVORITES_STORAGE_KEY); } catch { /* ignore */ }
    reminderProfileSettingsDirty.value = true;
}

function loadStoredPlayerBuffIds(): number[] | null {
    try {
        const raw = localStorage.getItem(PLAYER_BUFF_STORAGE_KEY);
        if (!raw) return null;
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return null;
        const ids = [...new Set(parsed
            .filter((id): id is number => Number.isInteger(id) && id >= 0)
            .map((id) => PLAYER_BUFF_ID_MIGRATIONS.get(id) ?? id)
            .filter((id) => !HIDDEN_PLAYER_BUFF_IDS.has(id)))];
        // An explicitly saved empty list is a valid user choice. Restoring
        // defaults here would make removed/customized Buff entries appear to
        // reset whenever the component is opened again.
        return ids;
    } catch {
        return null;
    }
}

function savePlayerBuffIds() {
    try { localStorage.setItem(PLAYER_BUFF_STORAGE_KEY, JSON.stringify(playerBuffIds.value)); }
    catch { /* ignore */ }
}

function loadStoredPlayerBuffFavoriteIds(): number[] {
    try {
        const raw = localStorage.getItem(PLAYER_BUFF_FAVORITES_STORAGE_KEY);
        if (!raw) return [];
        const parsed = JSON.parse(raw);
        if (!Array.isArray(parsed)) return [];
        return [...new Set(parsed
            .filter((id): id is number => Number.isInteger(id) && id >= 0)
            .map((id) => PLAYER_BUFF_ID_MIGRATIONS.get(id) ?? id)
            .filter((id) => !HIDDEN_PLAYER_BUFF_IDS.has(id)))];
    } catch {
        return [];
    }
}

function savePlayerBuffFavoriteIds() {
    try { localStorage.setItem(PLAYER_BUFF_FAVORITES_STORAGE_KEY, JSON.stringify(playerBuffFavoriteIds.value)); }
    catch { /* ignore */ }
}

const skillRows = computed<SkillRow[]>(() => {
    resourceNameVersion.value;
    const player = reportPlayer.value;
    const duration = reportSession.value?.totalDuration ?? 0;
    if (!player) return [];

    const rows = player.skillStats
        .filter((skill) => skill.totalDamage > 0)
        .map((skill) => ({
            ...skill,
            name: (skill as SkillStat & { previewName?: string }).previewName || skillName(skill.skillId),
            ratio: player.totalDamage > 0 ? skill.totalDamage / player.totalDamage : 0,
            dps: duration > 0 ? skill.totalDamage / duration : 0,
            maxDamage: Math.max(skill.maxCritDamage ?? 0, skill.maxNonCritDamage ?? 0),
            barRatio: 0,
            iconUrl: skillIconUrl(skill.skillId),
        }))
        .sort((a, b) => b.totalDamage - a.totalDamage);

    const firstDamage = rows[0]?.totalDamage || 1;
    return rows.map((row, index) => ({
        ...row,
        barRatio: index === 0 ? 1 : Math.max(0, Math.min(1, row.totalDamage / firstDamage)),
    }));
});

const metrics = computed(() => {
    const player = reportPlayer.value;
    const session = reportSession.value;
    const totalCritHits = player?.skillStats.reduce((sum, skill) => sum + skill.critHits, 0) ?? 0;
    return [
        { label: "战斗时间", value: session ? fmtDurationLong(session.totalDuration) : "00:00:00" },
        { label: "累计伤害", value: player ? fmtNumber(player.totalDamage) : "0" },
        { label: "每秒伤害", value: player ? fmtNumber(player.totalDPS) : "0" },
        { label: "暴击次数", value: fmtNumber(totalCritHits) },
    ];
});

const combatSummaryRows = computed(() => {
    const player = reportPlayer.value;
    if (!player) return [];
    const totalHits = player.skillStats.reduce((sum, skill) => sum + skill.totalHits, 0);
    const critHits = player.skillStats.reduce((sum, skill) => sum + skill.critHits, 0);
    const maxDamage = player.skillStats.reduce(
        (maximum, skill) => Math.max(maximum, skill.maxCritDamage ?? 0, skill.maxNonCritDamage ?? 0),
        0,
    );
    const traitDamage = player.skillStats
        .filter((skill) => skill.skillId === 58009)
        .reduce((sum, skill) => sum + skill.totalDamage, 0);
    const stardustDamage = player.skillStats
        .filter((skill) => skill.skillId === 58100 || skill.skillId === 58101)
        .reduce((sum, skill) => sum + skill.totalDamage, 0);
    const activeDamage = Math.max(0, player.totalDamage - traitDamage - stardustDamage);
    const ratio = (damage: number) => player.totalDamage > 0
        ? damage / player.totalDamage
        : 0;
    const rows = [
        { label: "技能伤害（对首领贡献）", value: `${fmtNumber(player.totalDamage)} (${fmtPct(playerContributionRate.value)})` },
        { label: "最大单次伤害", value: fmtNumber(maxDamage) },
        { label: "攻击次数", value: fmtNumber(totalHits) },
        { label: "暴击次数（概率）", value: `${fmtNumber(critHits)} (${fmtPct(player.critRate)})` },
        { label: "主动技能伤害（占比）", value: `${fmtNumber(activeDamage)} (${fmtPct(ratio(activeDamage))})` },
        { label: "特性伤害（连击，占比）", value: `${fmtNumber(traitDamage)} (${fmtPct(ratio(traitDamage))})` },
        { label: "星尘伤害（轰击、爆闪，占比）", value: `${fmtNumber(stardustDamage)} (${fmtPct(ratio(stardustDamage))})` },
        { label: "推测职业", value: player.jobName || "尚未识别" },
    ];
    const localEntityId = actorManager.value.localEntityId;
    const aimReminderConfigured = isDesignPreview
        ? previewAimSummaryEnabled
        : skillCooldownSettings.value.aimReminder.enabled && Boolean(localEntityId) && player.entityId === localEntityId;
    if (aimReminderConfigured) {
        const session = reportSession.value;
        const aimSummary = isDesignPreview && !summary.value
            ? { count: 18, averageRate: 0.876 }
            : session
                ? summarizeMagnumAimSamples(magnumAimSamples.value, session.startAt, session.endAt)
                : undefined;
        rows.splice(rows.length - 1, 0, {
            label: "穿心平均瞄准率",
            value: aimSummary ? fmtPct(aimSummary.averageRate) : "—",
        });
    }
    return rows;
});

function saveBattleRecord() {
    if (isDesignPreview && !summary.value) {
        showNotice("本场日志已导出，包含 1 名角色。", "success");
        return;
    }
    if (!summary.value || !selectedBossId.value) return;
    try {
        const record = createBattleRecord(
            actorManager.value,
            dcManager.value,
            selectedBossId.value,
            selectedPlayerId.value,
            { bossName: bossLabel.value },
        );
        const blob = new Blob([JSON.stringify(record, null, 2)], { type: "application/json;charset=utf-8" });
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = url;
        link.download = battleRecordFilename(record);
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 1000);
        showNotice(`本场日志已导出，包含 ${record.battle.playerCount} 名角色。`, "success");
    } catch (error) {
        showNotice(`导出日志失败：${error}`, "error");
    }
}

function clearReportData() {
    magnumAimSamples.value = [];
    activeMagnumAimCycle.value = null;
    recentMagnumAimCycle.value = null;
    lastMagnumAimActionKey = "";
    emit("clear-data");
}

function handleBattleRecordLoaded(event: Event) {
    const detail = (event as CustomEvent<BattleRecordSelection>).detail;
    if (!detail) return;
    manuallyLockedBossId.value = detail.bossEntityId;
    selectedBossId.value = detail.bossEntityId;
    selectedPlayerId.value = detail.playerEntityId;
    showNotice("历史记录已载入，可切换角色并重新生成分享图。", "success");
}

onMounted(async () => {
    conditionTimelineResizeObserver = new ResizeObserver((entries) => {
        const width = entries[0]?.contentRect.width;
        if (width && Math.abs(width - conditionTimelineWidth.value) >= 1) {
            conditionTimelineWidth.value = width;
        }
    });
    if (conditionTimelineElement.value) {
        conditionTimelineResizeObserver.observe(conditionTimelineElement.value);
    }
    appEvent.value.addEventListener(BATTLE_RECORD_LOADED_EVENT, handleBattleRecordLoaded);
    window.addEventListener("dilmeter-buff-alert-settings", reloadBuffAlertSettings as EventListener);
    window.addEventListener("dilmeter-debuff-alert-settings", reloadDebuffAlertSettings as EventListener);
    window.addEventListener("dilmeter-skill-cooldown-settings", reloadExternalSkillCooldownSettings as EventListener);
    window.addEventListener("dilmeter-save-settings-request", persistBuffAlertSettings as EventListener);
    window.addEventListener(SKILL_ACTION_EVENT, handleSkillAction as EventListener);
    window.addEventListener(EFFECT_TIMER_CONDITION_EVENT, handleEffectTimerCondition as EventListener);
    window.addEventListener(SKILL_COOLDOWN_ADJUSTMENT_EVENT, handleSkillCooldownAdjustment as EventListener);
    window.addEventListener(SKILL_DAMAGE_EVENT, handleSkillDamage as EventListener);
    window.addEventListener(SKILL_STATE_EVENT, handleSkillState as EventListener);
    window.addEventListener(BOSS_MECHANIC_EVENT, handleBossMechanicEvent as EventListener);
    document.addEventListener("visibilitychange", publishReminderStates);
    nativeReminderTickListener = (() => {
        // Always coalesce against the renderer's current clock. Older WebView2
        // runtimes can deliver queued native ticks in a burst after restore;
        // replaying their historical timestamps would bypass the rate limiter.
        publishReminderStates(Date.now(), false);
    }) as EventListener;
    window.addEventListener("dilmeter-native-tick", nativeReminderTickListener);
    try {
        buffOverlayChannel = new BroadcastChannel(BUFF_OVERLAY_CHANNEL);
    } catch {
        buffOverlayChannel = undefined;
    }
    try {
        debuffOverlayChannel = new BroadcastChannel(DEBUFF_OVERLAY_CHANNEL);
    } catch {
        debuffOverlayChannel = undefined;
    }
    try {
        skillOverlayChannel = new BroadcastChannel(SKILL_COOLDOWN_CHANNEL);
    } catch {
        skillOverlayChannel = undefined;
    }
    applyReminderProfile(reminderProfileStore.value.activeProfileId);
    scheduleNativeReminderSettingsSync(true);
    buffOverlayTimer = window.setInterval(() => {
        publishReminderStates(Date.now(), false);
    }, 200);
    nativeReminderDragTimer = window.setInterval(() => {
        void pollNativeReminderDragState();
    }, 100);
    summaryRefreshTimer = window.setInterval(() => refreshBossSummary(), 750);
    void fetch("/api/buff_overlay", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ locked: buffOverlaySettings.value.locked }),
    }).catch(() => undefined);
    void migrateLegacySkillOverlayPosition();
    publishBuffOverlayState();
    publishDebuffOverlayState();
    publishSkillCooldownOverlayState();
    if (isSharePreview) {
        sharePreviewUrl.value = URL.createObjectURL(await createShareBlob());
    }
});
onUnmounted(() => {
    conditionTimelineResizeObserver?.disconnect();
    appEvent.value.removeEventListener(BATTLE_RECORD_LOADED_EVENT, handleBattleRecordLoaded);
    window.removeEventListener("dilmeter-buff-alert-settings", reloadBuffAlertSettings as EventListener);
    window.removeEventListener("dilmeter-debuff-alert-settings", reloadDebuffAlertSettings as EventListener);
    window.removeEventListener("dilmeter-skill-cooldown-settings", reloadExternalSkillCooldownSettings as EventListener);
    window.removeEventListener("dilmeter-save-settings-request", persistBuffAlertSettings as EventListener);
    window.removeEventListener(SKILL_ACTION_EVENT, handleSkillAction as EventListener);
    window.removeEventListener(EFFECT_TIMER_CONDITION_EVENT, handleEffectTimerCondition as EventListener);
    window.removeEventListener(SKILL_COOLDOWN_ADJUSTMENT_EVENT, handleSkillCooldownAdjustment as EventListener);
    window.removeEventListener(SKILL_DAMAGE_EVENT, handleSkillDamage as EventListener);
    window.removeEventListener(SKILL_STATE_EVENT, handleSkillState as EventListener);
    window.removeEventListener(BOSS_MECHANIC_EVENT, handleBossMechanicEvent as EventListener);
    document.removeEventListener("visibilitychange", publishReminderStates);
    if (nativeReminderTickListener) {
        window.removeEventListener("dilmeter-native-tick", nativeReminderTickListener);
        nativeReminderTickListener = undefined;
    }
    buffOverlayChannel?.close();
    debuffOverlayChannel?.close();
    skillOverlayChannel?.close();
    if (buffOverlayTimer !== undefined) window.clearInterval(buffOverlayTimer);
    if (nativeReminderDragTimer !== undefined) window.clearInterval(nativeReminderDragTimer);
    if (summaryRefreshTimer !== undefined) window.clearInterval(summaryRefreshTimer);
    if (bossMechanicSavedTimer !== undefined) window.clearTimeout(bossMechanicSavedTimer);
    if (mielShardHealthBarPreviewTimer !== undefined) window.clearTimeout(mielShardHealthBarPreviewTimer);
    if (effectTimerSavedTimer !== undefined) window.clearTimeout(effectTimerSavedTimer);
    if (skillCooldownSavedTimer !== undefined) window.clearTimeout(skillCooldownSavedTimer);
    if (aimReminderSavedTimer !== undefined) window.clearTimeout(aimReminderSavedTimer);
    if (magnumAimPreviewTimer !== undefined) window.clearTimeout(magnumAimPreviewTimer);
    if (nativeReminderSettingsSyncTimer !== undefined) window.clearTimeout(nativeReminderSettingsSyncTimer);
    if (isStandalone.value) {
        void fetch("/api/buff_overlay/state", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ type: "buff-state", at: Date.now() / 1000, items: [] }),
        }).catch(() => undefined);
        void fetch("/api/debuff_overlay/state", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ type: "debuff-state", at: Date.now() / 1000, boss: null, targetHealth: null, items: [], settings: { iconSize: 30 } }),
        }).catch(() => undefined);
    }
    if (isStandalone.value) {
        void fetch("/api/skill_overlay/state", {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ type: "skill-cooldown-state", atMs: Date.now(), items: [], settings: { iconSize: 48 } }),
        }).catch(() => undefined);
    }
    if (sharePreviewUrl.value) URL.revokeObjectURL(sharePreviewUrl.value);
});

function publishReminderStates(atMs: number | Event = Date.now(), force = true) {
    // A native heartbeat also invokes this path while the main WebView is
    // minimized. Coalesce it with the normal 200 ms browser interval so a
    // visible window does not double the state publishing work.
    const tickMs = typeof atMs === "number" ? atMs : Date.now();
    if (!force && tickMs - lastReminderPublishAtMs < 175) return;
    lastReminderPublishAtMs = tickMs;
    publishBuffOverlayState();
    publishDebuffOverlayState();
    publishSkillCooldownOverlayState();
}

async function copyShareImage() {
    try {
        const blob = await createShareBlob();
        const ClipboardItemCtor = (window as unknown as { ClipboardItem?: typeof ClipboardItem }).ClipboardItem;
        if (!ClipboardItemCtor || !navigator.clipboard?.write) throw new Error("当前系统不支持直接复制图片");
        await navigator.clipboard.write([new ClipboardItemCtor({ "image/png": blob })]);
        showNotice("图片已复制，可以直接粘贴分享。", "success");
    } catch (error) {
        showNotice(`${error}，已改为下载 PNG。`, "warning");
        await saveShareImage();
    }
}

async function saveShareImage() {
    if (!reportPlayer.value) return;
    try {
        const blob = await createShareBlob();
        const url = URL.createObjectURL(blob);
        const link = document.createElement("a");
        link.href = url;
        link.download = safeFilename(`${bossLabel.value}_${playerLabel.value}_战斗统计.png`);
        link.click();
        window.setTimeout(() => URL.revokeObjectURL(url), 1000);
        showNotice("PNG 图片已保存。", "success");
    } catch (error) {
        showNotice(`图片生成失败：${error}`, "error");
    }
}

async function createShareBlob(): Promise<Blob> {
    if (!reportPlayer.value || !reportSession.value) throw new Error("暂无可分享的战斗数据");
    const width = 1200;
    const height = 666;
    const scale = 2;
    const canvas = document.createElement("canvas");
    canvas.width = width * scale;
    canvas.height = height * scale;
    const ctx = canvas.getContext("2d");
    if (!ctx) throw new Error("无法创建图片画布");
    ctx.scale(scale, scale);

    await document.fonts?.load("15px 'Material Design Icons'");
    const icons = await Promise.all(skillRows.value.slice(0, 10).map((skill) => loadImage(skill.iconUrl)));
    drawShareCanvas(ctx, width, height, icons);
    return new Promise((resolve, reject) => canvas.toBlob(
        (blob) => blob ? resolve(blob) : reject(new Error("PNG 编码失败")),
        "image/png",
    ));
}

function drawShareCanvas(ctx: CanvasRenderingContext2D, width: number, height: number, icons: Array<HTMLImageElement | null>) {
    const player = reportPlayer.value!;
    const session = reportSession.value!;
    ctx.fillStyle = "#070707";
    ctx.fillRect(0, 0, width, height);
    ctx.strokeStyle = "#545454";
    ctx.strokeRect(1, 1, width - 2, height - 2);

    const titleGradient = ctx.createLinearGradient(0, 0, 0, 27);
    titleGradient.addColorStop(0, "#2a2a2a");
    titleGradient.addColorStop(1, "#101010");
    ctx.fillStyle = titleGradient;
    ctx.fillRect(3, 3, width - 6, 25);
    ctx.fillStyle = "#686868";
    ctx.font = "15px 'Material Design Icons'";
    ctx.fillText(String.fromCodePoint(0xF0787), 8, 20);
    ctx.fillStyle = "#e7e7e7";
    ctx.font = "700 13px 'Microsoft YaHei', 'Malgun Gothic', sans-serif";
    ctx.fillText("详细战斗统计", 27, 20);
    ctx.fillStyle = "#777777";
    ctx.font = "16px 'Material Design Icons'";
    ctx.fillText(String.fromCodePoint(0xF0158), width - 21, 21);
    ctx.fillStyle = "#030303";
    ctx.fillRect(3, 28, width - 6, 2);

    const statX = 25;
    const statGap = 6;
    const statWidth = (width - 50 - statGap * 3) / 4;
    const statValues = [
        fmtDurationLong(session.totalDuration),
        fmtNumber(player.totalDamage),
        fmtNumber(player.totalDPS),
        fmtNumber(player.skillStats.reduce((sum, skill) => sum + skill.critHits, 0)),
    ];
    ["战斗时间", "累计伤害", "每秒伤害", "暴击次数"].forEach((label, index) => {
        const x = statX + index * (statWidth + statGap);
        ctx.fillStyle = "#f0f0f0";
        ctx.font = "700 14px 'Microsoft YaHei', sans-serif";
        ctx.textAlign = "center";
        ctx.fillText(label, x + statWidth / 2, 57);
        const valueGradient = ctx.createLinearGradient(0, 67, 0, 96);
        valueGradient.addColorStop(0, "#2c2c2c");
        valueGradient.addColorStop(1, "#171717");
        ctx.fillStyle = valueGradient;
        ctx.fillRect(x, 67, statWidth, 29);
        ctx.strokeStyle = "#505050";
        ctx.strokeRect(x + 0.5, 67.5, statWidth - 1, 28);
        ctx.fillStyle = "#ffffff";
        ctx.font = "13px 'Microsoft YaHei', sans-serif";
        ctx.fillText(statValues[index], x + statWidth / 2, 87);
    });
    ctx.textAlign = "left";

    ctx.fillStyle = "#111111";
    ctx.fillRect(9, 104, width - 18, height - 113);
    ctx.strokeStyle = "#555555";
    ctx.strokeRect(9.5, 104.5, width - 19, height - 114);
    const tabs = [
        { label: "综合", width: 51, active: false },
        { label: "攻击技能", width: 65, active: true },
        { label: "提醒设置", width: 65, active: false },
    ];
    let tabX = 17;
    tabs.forEach((tab) => {
        const tabGradient = ctx.createLinearGradient(0, 109, 0, 132);
        if (tab.active) {
            tabGradient.addColorStop(0, "#5d5d5d");
            tabGradient.addColorStop(1, "#242424");
        } else {
            tabGradient.addColorStop(0, "#303030");
            tabGradient.addColorStop(1, "#101010");
        }
        ctx.fillStyle = tabGradient;
        ctx.fillRect(tabX, 109, tab.width, 23);
        ctx.strokeStyle = tab.active ? "#888888" : "#444444";
        ctx.lineWidth = 1;
        ctx.strokeRect(tabX + 0.5, 109.5, tab.width - 1, 22);
        ctx.fillStyle = tab.active ? "#ffffff" : "#d5d5d5";
        ctx.font = "700 11px 'Microsoft YaHei', sans-serif";
        ctx.textAlign = "center";
        ctx.fillText(tab.label, tabX + tab.width / 2, 125);
        tabX += tab.width + 4;
    });
    ctx.strokeStyle = "#3f3f3f";
    ctx.beginPath();
    ctx.moveTo(17, 134.5);
    ctx.lineTo(width - 17, 134.5);
    ctx.stroke();

    const columns = [17, 237, 423, 569, 715, 861, 1007, 1183];
    const headers = ["技能名", "累计伤害", "每秒伤害", "伤害占比", "最大伤害", "使用次数", "暴击发动次数"];
    const headerGradient = ctx.createLinearGradient(0, 138, 0, 163);
    headerGradient.addColorStop(0, "#2e2e2e");
    headerGradient.addColorStop(1, "#151515");
    ctx.fillStyle = headerGradient;
    headers.forEach((header, index) => {
        const x = columns[index];
        const w = columns[index + 1] - x - 5;
        ctx.fillRect(x, 138, w, 25);
        ctx.strokeStyle = "#555555";
        ctx.strokeRect(x + 0.5, 138.5, w - 1, 24);
        ctx.fillStyle = "#f2f2f2";
        ctx.font = "700 11px 'Microsoft YaHei', sans-serif";
        ctx.textAlign = "center";
        ctx.fillText(header, x + w / 2, 155);
        ctx.fillStyle = headerGradient;
    });

    const rows = skillRows.value.slice(0, 10);
    rows.forEach((skill, index) => {
        const y = 169 + index * 44;
        const rowGradient = ctx.createLinearGradient(0, y, 0, y + 39);
        rowGradient.addColorStop(0, "#c3c3c3");
        rowGradient.addColorStop(0.5, "#a9a9a9");
        rowGradient.addColorStop(1, "#8e8e8e");
        ctx.fillStyle = rowGradient;
        ctx.fillRect(17, y, width - 34, 39);
        ctx.strokeStyle = "#070707";
        ctx.lineWidth = 3;
        ctx.strokeRect(17.5, y + 0.5, width - 35, 38);

        const iconX = 20;
        const progressX = 52;
        const progressWidth = (width - progressX - 17) * skill.barRatio;
        const greenGradient = ctx.createLinearGradient(0, y, 0, y + 39);
        greenGradient.addColorStop(0, "#9df30c");
        greenGradient.addColorStop(0.55, "#7ed803");
        greenGradient.addColorStop(1, "#66b900");
        ctx.fillStyle = greenGradient;
        ctx.fillRect(progressX, y + 3, progressWidth, 33);

        if (icons[index]) {
            ctx.drawImage(icons[index]!, iconX, y + 4, 31, 31);
            ctx.strokeStyle = "#131313";
            ctx.lineWidth = 1;
            ctx.strokeRect(iconX - 0.5, y + 3.5, 32, 32);
        }

        const values = [
            skill.name,
            fmtGameNumber(skill.totalDamage),
            fmtGameNumber(skill.dps),
            fmtPct(skill.ratio),
            fmtGameNumber(skill.maxDamage),
            String(skill.totalHits || "-"),
            `${skill.critHits}(${skill.noCritRate ? "—" : fmtPct(skill.critRate)})`,
        ];
        values.forEach((value, valueIndex) => {
            const start = columns[valueIndex];
            const end = columns[valueIndex + 1];
            ctx.textAlign = "center";
            ctx.font = "600 12px 'Microsoft YaHei', sans-serif";
            const text = valueIndex === 0 ? ellipsize(ctx, value, end - start - 44) : value;
            const x = valueIndex === 0 ? start + 36 + (end - start - 36) / 2 : start + (end - start) / 2;
            ctx.lineJoin = "round";
            ctx.miterLimit = 2;
            ctx.strokeStyle = "rgba(52,52,52,.96)";
            ctx.lineWidth = 1.1;
            ctx.strokeText(text, x, y + 25);
            ctx.fillStyle = "#f8f8f8";
            ctx.fillText(text, x, y + 25);
        });
    });
    ctx.textAlign = "left";
}

function loadImage(src: string): Promise<HTMLImageElement | null> {
    return new Promise((resolve) => {
        const image = new Image();
        image.onload = () => resolve(image);
        image.onerror = () => resolve(null);
        image.src = src;
    });
}

function useFallbackIcon(event: Event) {
    const image = event.currentTarget as HTMLImageElement;
    if (image.dataset.fallback === "1") return;
    image.dataset.fallback = "1";
    image.src = skillIconUrl(10001);
}

function hideMissingConditionIcon(event: Event) {
    (event.currentTarget as HTMLImageElement).style.display = "none";
}

function skillIconUrl(skillId: number) {
    return `/skill-icons/${skillId}.png`;
}

function fmtCoverageDuration(seconds: number) {
    const value = Math.max(0, Math.round(seconds));
    const hours = Math.floor(value / 3600);
    const minutes = Math.floor((value % 3600) / 60);
    const secs = value % 60;
    if (hours > 0) return `${hours}:${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
    return `${String(minutes).padStart(2, "0")}:${String(secs).padStart(2, "0")}`;
}

function bossName(entity: Pick<EntityActor, "raceId" | "name">) {
    return bossDisplayName(entity, raceNameMap.value);
}

function targetHealthName(entity: EntityActor) {
    if ([7604, 7605, 7606, 7607, 7608, 7616, 7617, 7618, 7619, 7620].includes(entity.raceId)) {
        return "安乐碎片";
    }
    return toSimplified(cleanName(raceNameMap.value[entity.raceId]) || cleanName(entity.name))
        || `目标 ${entity.raceId}`;
}

function skillName(skillId: number) {
    const resourceName = toSimplified(cleanName(skillNameMap.value[skillId] || FALLBACK_SKILL_NAME_MAP[String(skillId)]));
    return normalizeSkillDisplayName(skillId, resourceName) || `技能 ${skillId}`;
}

function skillDisplayName(skillId: number) {
    return skillName(skillId);
}

function cleanName(name: string | undefined) {
    return name ? name.replace(/\s+\d+$/, "").trim() : "";
}

function toSimplified(text: string) {
    const map: Record<string, string> = {
        "連": "连", "續": "续", "擊": "击", "閃": "闪", "護": "护", "轉": "转", "輪": "轮", "迴": "回",
        "龍": "龙", "雙": "双", "槍": "枪", "夢": "梦", "喚": "唤", "劍": "剑",
        "戰": "战", "鬥": "斗", "風": "风", "闇": "暗", "聖": "圣", "靈": "灵",
        "術": "术", "彈": "弹", "衝": "冲", "範": "范", "圍": "围", "傷": "伤",
        "強": "强", "體": "体", "輕": "轻", "復": "复", "藥": "药", "賦": "赋",
        "詠": "咏", "禱": "祷", "絕": "绝", "對": "对", "稱": "称", "號": "号",
        "標": "标", "記": "记", "減": "减", "緩": "缓", "暈": "晕", "敵": "敌",
        "騎": "骑", "寵": "宠", "鍛": "锻", "煉": "炼", "鍊": "炼", "製": "制",
        "採": "采", "釣": "钓", "魚": "鱼", "藝": "艺", "樂": "乐", "詩": "诗",
        "進": "进", "階": "阶", "變": "变", "詛": "诅", "陣": "阵", "與": "与",
        "為": "为", "損": "损", "讓": "让", "發": "发", "時": "时", "間": "间",
    };
    return text.replace(/[^\x00-\x7F]/g, (char) => map[char] ?? char);
}

function safeFilename(value: string) {
    return value.replace(/[\\/:*?"<>|]/g, "_");
}

function showNotice(message: string, type: NoticeType) {
    notice.value = message;
    noticeType.value = type;
}

function fmtNumber(value: number) {
    return Math.round(value || 0).toLocaleString("zh-CN");
}

function fmtCompact(value: number) {
    const absolute = Math.abs(value || 0);
    if (absolute >= 100_000_000) return `${(value / 100_000_000).toFixed(2)}亿`;
    if (absolute >= 10_000) return `${(value / 10_000).toFixed(1)}万`;
    return fmtNumber(value);
}

function fmtBossHealth(value: number) {
    return `${((Number.isFinite(value) ? value : 0) / 100_000_000).toFixed(2)}亿`;
}

function fmtGameNumber(value: number) {
    const number = Math.max(0, Math.round(value || 0));
    if (number >= 100_000_000) {
        const yi = Math.floor(number / 100_000_000);
        const wan = Math.floor((number % 100_000_000) / 10_000);
        const rest = number % 10_000;
        return `${yi}亿 ${wan}万${rest ? ` ${rest}` : ""}`;
    }
    if (number >= 10_000) {
        const wan = Math.floor(number / 10_000);
        const rest = number % 10_000;
        return `${wan}万${rest ? ` ${rest}` : ""}`;
    }
    return fmtNumber(number);
}

function fmtPct(value: number) {
    return `${((value || 0) * 100).toFixed(2)}%`;
}

function fmtDurationLong(seconds: number) {
    const total = Math.max(0, Math.round(seconds || 0));
    const hours = Math.floor(total / 3600);
    const minutes = Math.floor((total % 3600) / 60);
    const remainder = total % 60;
    return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(remainder).padStart(2, "0")}`;
}

function ellipsize(ctx: CanvasRenderingContext2D, value: string, maxWidth: number) {
    if (ctx.measureText(value).width <= maxWidth) return value;
    let text = value;
    while (text.length && ctx.measureText(`${text}…`).width > maxWidth) text = text.slice(0, -1);
    return `${text}…`;
}
</script>

<style scoped>
.share-preview-page {
    display: flex;
    align-items: flex-start;
    justify-content: center;
    min-height: 100vh;
    color: #dddddd;
    background: #202020;
}

.share-preview-page img {
    display: block;
    width: min(1200px, 100vw);
    height: auto;
}

.share-preview-page span {
    padding: 24px;
    font: 13px "Microsoft YaHei", sans-serif;
}

.report-page {
    min-width: 850px;
    padding: 7px;
    color: #e8e8e8;
    background: #050505;
    font-family: "Microsoft YaHei", "Malgun Gothic", Arial, sans-serif;
}

.report-page.design-preview {
    min-width: 0;
    height: 100vh;
    padding: 0;
    overflow: hidden;
}

.design-preview .combat-window {
    height: 100vh;
    min-height: 0;
}

.report-controls {
    display: flex;
    align-items: end;
    gap: 8px;
    padding: 8px 9px;
    margin-bottom: 7px;
    background: linear-gradient(#292929, #111111);
    border: 1px solid #4e4e4e;
    box-shadow: inset 0 0 0 1px #090909;
}

.control-field {
    display: grid;
    gap: 4px;
    min-width: 230px;
    color: #d3d3d3;
    font-size: 12px;
    font-weight: 600;
}

.control-label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
}

.control-help {
    display: inline-grid;
    place-items: center;
    color: #b8d9ee;
    cursor: help;
    outline: none;
}

.control-help:hover,
.control-help:focus-visible {
    color: #eefaff;
    filter: drop-shadow(0 0 3px rgba(131, 207, 255, .72));
}

.control-field select {
    height: 32px;
    padding: 0 8px;
    color: #f0f0f0;
    background: #171717;
    border: 1px solid #5b5b5b;
    border-radius: 0;
    outline: none;
    font-size: 13px;
    font-weight: 600;
}

.control-field select:focus {
    border-color: #9a9a9a;
}

.only-boss-switch {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    height: 32px;
    color: #d9f7b5;
    font-size: 12px;
    font-weight: 700;
    white-space: nowrap;
}

.only-boss-switch input {
    accent-color: #7bd528;
}

.team-chart-button {
    min-width: 118px;
    margin-left: auto;
    border-color: #6f8f42;
}

.team-chart-button:not(:disabled):hover {
    border-color: #a1d65c;
    box-shadow: inset 0 0 0 1px #405925, 0 0 8px rgba(139, 210, 57, 0.2);
}

.game-button {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 5px;
    height: 32px;
    padding: 0 12px;
    color: #ececec;
    background: linear-gradient(#444444, #1b1b1b);
    border: 1px solid #656565;
    box-shadow: inset 0 0 0 1px #171717;
    font-size: 12px;
    font-weight: 700;
    cursor: pointer;
    white-space: nowrap;
}

.game-button.active,
.game-button.confirm {
    border-color: #8fbf50;
    background: linear-gradient(#55742e, #253515);
}

.reminder-profile-toolbar {
    display: grid;
    grid-template-columns: minmax(210px, 1.2fr) minmax(150px, 0.7fr) minmax(170px, 0.8fr) auto auto minmax(260px, 1fr);
    align-items: end;
    gap: 8px;
    padding: 9px;
    margin-bottom: 8px;
    color: #e7e7e7;
    background: linear-gradient(#343434, #202020);
    border: 1px solid #5d5d5d;
    box-shadow: inset 0 0 0 1px #161616;
}

.reminder-profile-heading {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    padding-bottom: 2px;
}

.reminder-profile-heading :deep(.v-icon) { color: #9fe152; }
.reminder-profile-heading div { display: grid; gap: 2px; min-width: 0; }
.reminder-profile-heading strong { font-size: 13px; }
.reminder-profile-heading span { overflow: hidden; color: #b7b7b7; font-size: 10px; text-overflow: ellipsis; white-space: nowrap; }

.reminder-profile-toolbar > label {
    display: grid;
    gap: 3px;
    color: #d5d5d5;
    font-size: 10px;
}

.reminder-profile-toolbar select,
.reminder-profile-toolbar input {
    min-width: 0;
    height: 29px;
    padding: 0 7px;
    color: #f1f1f1;
    background: #171717;
    border: 1px solid #5d5d5d;
    outline: none;
    font-size: 11px;
}

.profile-action,
.buff-time-adjustment button {
    height: 29px;
    padding: 0 9px;
    color: #ededed;
    background: linear-gradient(#424242, #222);
    border: 1px solid #666;
    font-size: 11px;
    font-weight: 600;
    cursor: pointer;
}

.profile-action { display: inline-flex; align-items: center; justify-content: center; gap: 4px; white-space: nowrap; }
.profile-action.danger { border-color: #795454; }
.profile-action:disabled { opacity: 0.4; cursor: default; }

.buff-time-adjustment {
    display: grid;
    grid-template-columns: minmax(120px, 1fr) 29px 62px 29px 22px;
    align-items: center;
    gap: 4px;
    min-width: 0;
    color: #d9d9d9;
    font-size: 11px;
}

.buff-time-adjustment input { width: 62px; padding: 0 4px; text-align: center; }
.buff-time-adjustment button { padding: 0; color: #baff68; font-size: 18px; }
.buff-time-adjustment small { color: #ababab; }

.team-chart-backdrop {
    position: fixed;
    z-index: 10020;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    background: rgba(0, 0, 0, 0.72);
}

.team-chart-dialog {
    width: min(1080px, calc(100vw - 48px));
    max-height: calc(100vh - 48px);
    overflow: auto;
    color: #ededed;
    background: #111;
    border: 1px solid #6a6a6a;
    box-shadow: 0 14px 52px rgba(0, 0, 0, 0.75), inset 0 0 0 1px #050505;
}

.team-chart-titlebar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    height: 34px;
    padding: 0 10px;
    background: linear-gradient(#292929, #101010);
    border-bottom: 1px solid #030303;
    box-shadow: inset 0 -1px #444;
    font-size: 12px;
    font-weight: 700;
}

.team-chart-titlebar > div { display: flex; align-items: center; gap: 6px; }
.team-chart-titlebar > div :deep(.v-icon) { color: #9be34a; }
.team-chart-titlebar > button { color: #a5a5a5; background: transparent; border: 0; cursor: pointer; }

.team-chart-toolbar {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 10px;
    min-height: 48px;
    padding: 7px 10px;
    background: #222;
    border-bottom: 1px solid #4b4b4b;
}

.team-chart-context { display: grid; gap: 2px; min-width: 210px; margin-right: auto; }
.team-chart-context strong { font-size: 12px; }
.team-chart-context span { color: #b8b8b8; font-size: 10px; }
.team-chart-view-switch { display: flex; align-items: center; gap: 4px; }
.team-chart-view-switch button { display: inline-flex; align-items: center; gap: 4px; height: 25px; padding: 0 9px; color: #d8d8d8; background: linear-gradient(#3d3d3d, #202020); border: 1px solid #5c5c5c; font-size: 10px; cursor: pointer; }
.team-chart-view-switch button.active { color: #f4ffe5; border-color: #91ca4d; box-shadow: inset 0 0 0 1px #415d20; }
.team-chart-privacy { display: inline-flex; align-items: center; gap: 4px; color: #b9d88e; font-size: 11px; }
.team-chart-dialog :deep(.team-chart-component) { padding: 10px; }
.team-chart-dialog :deep(.skill-timeline-component) { padding: 10px; }
.team-chart-dialog footer { display: flex; align-items: center; justify-content: space-between; gap: 12px; padding: 8px 10px; color: #aaa; border-top: 1px solid #454545; font-size: 10px; }
.team-chart-empty { display: grid; place-items: center; min-height: 300px; color: #888; }

.privacy-warning-backdrop { z-index: 10030; }
.team-privacy-dialog { width: min(460px, calc(100vw - 40px)); padding: 18px; color: #ededed; background: linear-gradient(#2b2b2b, #171717); border: 1px solid #777; box-shadow: 0 14px 42px rgba(0, 0, 0, 0.75); }
.team-privacy-dialog header { display: flex; align-items: center; gap: 9px; font-size: 14px; }
.team-privacy-dialog header :deep(.v-icon) { color: #ffc34d; }
.team-privacy-dialog p { margin: 14px 0 18px; color: #cfcfcf; font-size: 12px; line-height: 1.7; }
.team-privacy-dialog > div { display: flex; justify-content: flex-end; gap: 8px; }

.game-button:hover:not(:disabled) {
    background: linear-gradient(#585858, #242424);
}

.game-button:disabled {
    color: #666666;
    cursor: default;
}

.notice-line {
    display: flex;
    align-items: center;
    min-height: 34px;
    padding: 0 9px;
    margin-bottom: 7px;
    border: 1px solid #646464;
    background: #1b1b1b;
    font-size: 13px;
    font-weight: 600;
}

.notice-line button {
    margin-left: auto;
    color: inherit;
    background: transparent;
    border: 0;
    cursor: pointer;
}

.notice-success { color: #a8e878; }
.notice-warning { color: #f4d17a; }
.notice-error { color: #ff8d8d; }
.notice-info { color: #bfcbd2; }

.combat-window {
    box-sizing: border-box;
    height: calc(100vh - 111px);
    min-height: 540px;
    overflow: hidden;
    color: #efefef;
    background: #090909;
    border: 1px solid #505050;
    box-shadow: inset 0 0 0 2px #151515, 0 0 0 1px #000000;
}

.window-titlebar {
    display: flex;
    align-items: center;
    gap: 5px;
    height: 27px;
    padding: 0 5px;
    color: #dedede;
    background: linear-gradient(#2d2d2d, #101010);
    border-bottom: 2px solid #030303;
    font-size: 12px;
    font-weight: 700;
    text-shadow: 0 1px #000000;
}

.window-titlebar :deep(.v-icon) {
    color: #5d5d5d;
}

.window-close {
    margin-left: auto;
}

.top-stat-grid {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 5px;
    padding: 12px 24px 6px;
    background: #242424;
}

.top-stat {
    min-width: 0;
    text-align: center;
}

.top-stat-label {
    height: 28px;
    color: #f1f1f1;
    font-size: 14px;
    font-weight: 700;
    line-height: 28px;
    text-shadow: 0 2px 1px #000000;
}

.top-stat-value {
    height: 30px;
    color: #ffffff;
    background: linear-gradient(#2b2b2b, #171717);
    border: 1px solid #4c4c4c;
    box-shadow: inset 0 0 0 1px #202020;
    font-size: 13px;
    line-height: 28px;
    text-shadow: 0 1px 1px #000000;
}

.table-frame {
    height: calc(100% - 103px);
    padding: 6px 7px 8px;
    background: #111111;
    border: 1px solid #515151;
    box-shadow: inset 0 0 0 1px #050505;
}

.combat-tabs {
    display: flex;
    align-items: end;
    gap: 4px;
    height: 26px;
    padding: 3px 0 0;
    border-bottom: 1px solid #3f3f3f;
}

.combat-tab {
    box-sizing: border-box;
    min-width: 51px;
    height: 23px;
    padding: 0 9px;
    color: #d7d7d7;
    background: linear-gradient(#303030, #101010);
    border: 1px solid #454545;
    border-radius: 0;
    box-shadow: inset 0 0 0 1px #090909;
    font: 700 10px/21px "Microsoft YaHei", sans-serif;
    text-shadow: 0 1px #000000;
    opacity: 1;
    cursor: pointer;
}

.combat-tab:hover:not(.active) {
    background: linear-gradient(#404040, #171717);
    border-color: #606060;
}

.combat-tab.active {
    color: #ffffff;
    background: linear-gradient(#626262, #242424);
    border-color: #888888;
}

.combat-summary-panel {
    height: calc(100% - 27px);
    padding: 10px 9px;
    overflow-y: auto;
    color: #f0f0f0;
    background: #5d5b5a;
}

.combat-summary-panel h2 {
    margin: 0 0 8px;
    font-size: 15px;
    font-weight: 700;
    text-shadow: 0 1px 1px #202020;
}

.combat-summary-table {
    margin: 0;
    border-top: 1px solid #424140;
    border-left: 1px solid #424140;
}

.combat-summary-row {
    display: grid;
    grid-template-columns: 1fr 1fr;
    min-height: 39px;
    border-bottom: 1px solid #424140;
}

.combat-summary-row dt,
.combat-summary-row dd {
    display: flex;
    align-items: center;
    margin: 0;
    padding: 0 11px;
    border-right: 1px solid #424140;
    font-size: 13px;
    text-shadow: 0 1px 1px #2a2a2a;
}

.combat-summary-row dd {
    justify-content: flex-end;
    font-weight: 700;
}

.condition-panel {
    margin-top: 12px;
    border: 1px solid #424140;
    background: #4f4d4c;
}

.condition-panel-header {
    display: flex;
    align-items: center;
    min-height: 38px;
    padding: 5px 8px 5px 11px;
    background: linear-gradient(#696766, #555352);
    border-bottom: 1px solid #3d3c3b;
}

.condition-panel-title {
    display: flex;
    align-items: baseline;
    gap: 9px;
    min-width: 0;
}

.condition-panel-title strong {
    color: #ffffff;
    font-size: 14px;
    text-shadow: 0 1px 1px #202020;
}

.condition-panel-title span {
    overflow: hidden;
    color: #dedede;
    font-size: 11px;
    text-overflow: ellipsis;
    white-space: nowrap;
}
.condition-panel-title .boss-health-readout { color: #dfffb8; font-weight: 700; }

.condition-target-toggle {
    display: flex;
    gap: 3px;
    margin-left: auto;
}

.condition-target-toggle label {
    min-width: 50px;
    height: 24px;
    padding: 0 9px;
    color: #d7d7d7;
    background: linear-gradient(#333333, #151515);
    border: 1px solid #454545;
    box-shadow: inset 0 0 0 1px #0b0b0b;
    font-size: 11px;
    font-weight: 700;
    line-height: 22px;
    text-align: center;
    text-shadow: 0 1px #000000;
    cursor: pointer;
}

.condition-target-toggle label.active {
    color: #ffffff;
    background: linear-gradient(#6b6b6b, #282828);
    border-color: #999999;
}

.condition-target-toggle input {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
}

.condition-settings-button {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 24px;
    margin-right: 8px;
    padding: 0 8px;
    color: #d7d7d7;
    background: linear-gradient(#333333, #151515);
    border: 1px solid #454545;
    box-shadow: inset 0 0 0 1px #0b0b0b;
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
}

.condition-settings-button.active {
    color: #ffffff;
    border-color: #8f8f8f;
    background: linear-gradient(#606060, #292929);
}

.player-buff-settings {
    padding: 8px 9px;
    background: #444241;
    border-bottom: 1px solid #333231;
}

.player-buff-settings-tip {
    margin-bottom: 7px;
    color: #d3d3d3;
    font-size: 10px;
}

.buff-alert-settings {
    margin-bottom: 8px;
    padding: 8px;
    color: #eeeeee;
    background: linear-gradient(#3c4137, #2c2f29);
    border: 1px solid #657951;
    box-shadow: inset 0 0 0 1px rgba(190, 236, 132, .09);
    font-size: 10px;
}

.reminder-settings-page {
    box-sizing: border-box;
    height: calc(100% - 27px);
    padding: 8px;
    overflow-y: auto;
    color: #eeeeee;
    background: #565452;
}

.reminder-settings-page .buff-alert-settings {
    margin: 0;
}

.aim-reminder-settings {
    margin-top: 8px;
    padding: 9px;
    color: var(--ui-theme-text);
    background: linear-gradient(135deg, var(--ui-theme-surface), var(--ui-theme-control));
    border: 1px solid var(--ui-theme-border);
    box-shadow: inset 0 0 0 1px rgba(var(--ui-color-rgb), .1), 0 2px 7px rgba(0, 0, 0, .16);
    font-size: 10px;
}

.aim-reminder-settings > header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    margin-bottom: 8px;
    padding-bottom: 7px;
    border-bottom: 1px solid var(--ui-theme-border);
}

.aim-reminder-settings > header > div:first-child {
    display: grid;
    flex: 1 1 420px;
    gap: 2px;
}

.aim-reminder-settings > header strong {
    color: var(--ui-color-accent);
    font-size: 12px;
}

.aim-reminder-settings > header span,
.aim-reminder-settings > p {
    color: var(--ui-theme-muted);
    font-size: 9px;
}

.aim-reminder-header-actions,
.aim-reminder-controls,
.aim-reminder-coordinates,
.aim-reminder-coordinates > label,
.aim-reminder-controls > label {
    display: flex;
    align-items: center;
}

.aim-reminder-header-actions,
.aim-reminder-controls {
    flex-wrap: wrap;
    gap: 7px 12px;
}

.aim-reminder-controls {
    padding: 8px;
    color: var(--ui-theme-text);
    background: var(--ui-theme-raised);
    border: 1px solid var(--ui-theme-border);
}

.aim-reminder-controls > label,
.aim-reminder-coordinates,
.aim-reminder-coordinates > label {
    gap: 5px;
    white-space: nowrap;
}

.aim-reminder-toggle {
    color: var(--ui-color-accent);
    font-weight: 800;
}

.aim-reminder-controls input[type="number"],
.aim-reminder-controls select {
    width: 68px;
    height: 25px;
    padding: 0 5px;
    color: var(--ui-theme-text);
    background: var(--ui-theme-control);
    border: 1px solid var(--ui-theme-border);
}

.aim-reminder-controls select {
    width: 128px;
    padding: 0 4px;
}

.aim-reminder-effective-range {
    display: inline-flex;
    align-items: baseline;
    gap: 4px;
    min-height: 25px;
    padding: 4px 7px;
    color: var(--ui-theme-muted);
    background: var(--ui-theme-control);
    border: 1px solid var(--ui-theme-border);
    white-space: nowrap;
}

.aim-reminder-effective-range strong {
    color: var(--ui-theme-text);
    font-size: 11px;
    font-variant-numeric: tabular-nums;
}

.aim-reminder-calculated-time {
    display: inline-flex;
    align-items: baseline;
    gap: 4px;
    min-height: 25px;
    padding: 4px 8px;
    color: var(--ui-theme-text);
    background: rgba(var(--ui-color-rgb), .12);
    border: 1px solid var(--ui-color-accent);
    box-shadow: inset 0 0 8px rgba(var(--ui-color-rgb), .08);
    white-space: nowrap;
}

.aim-reminder-calculated-time span,
.aim-reminder-calculated-time small {
    color: var(--ui-theme-muted);
    font-size: 9px;
}

.aim-reminder-calculated-time strong {
    color: var(--ui-color-accent);
    font-size: 13px;
    font-variant-numeric: tabular-nums;
}

.aim-reminder-coordinates {
    padding: 4px 7px;
    background: var(--ui-theme-control);
    border: 1px solid var(--ui-theme-border);
}

.aim-reminder-coordinates > span {
    color: var(--ui-theme-muted);
    font-size: 9px;
    font-weight: 700;
}

.aim-reminder-preview {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    min-height: 25px;
    padding: 4px 9px;
    border: 1px solid var(--ui-theme-border);
    font-size: 9px;
    font-weight: 800;
}

.aim-reminder-preview {
    color: var(--ui-theme-text);
    background: var(--ui-theme-raised);
    border-color: var(--ui-color-accent);
}

.aim-reminder-preview:hover:not(:disabled) {
    box-shadow: 0 0 9px rgba(var(--ui-color-rgb), .72);
    filter: brightness(1.08);
}

.aim-reminder-preview:disabled { cursor: not-allowed; filter: grayscale(.7); opacity: .48; }
.aim-reminder-unsaved { color: var(--ui-color-accent) !important; font-weight: 800; }
.aim-reminder-settings > p { margin: 7px 1px 0; }

.skill-cooldown-settings {
    margin-top: 8px;
    padding: 8px;
    color: #eeeeee;
    background: linear-gradient(#373e40, #292e30);
    border: 1px solid #587279;
    box-shadow: inset 0 0 0 1px rgba(137, 224, 244, .08);
    font-size: 10px;
}

.debuff-alert-settings {
    margin-top: 8px;
    padding: 9px;
    color: #ededeb;
    background: linear-gradient(180deg, #292b27, #20221f);
    border: 1px solid #59614e;
    box-shadow: inset 0 1px rgba(255, 255, 255, .04), inset 0 0 0 1px rgba(0, 0, 0, .5);
    font-size: 10px;
}
.debuff-alert-settings > header { display: flex; flex-wrap: wrap; align-items: center; gap: 8px 14px; margin-bottom: 8px; padding-bottom: 7px; border-bottom: 1px solid #42473d; }
.debuff-alert-settings > header > div:first-child { display: grid; gap: 2px; margin-right: auto; }
.debuff-alert-settings > header strong { color: #ffcfbf; font-size: 12px; }
.debuff-alert-settings > header span, .debuff-alert-settings > p { color: #c9ccc4; font-size: 9px; }
.common-debuff-button {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 25px;
    padding: 0 9px;
    color: #efffdc;
    background: linear-gradient(#4e633a, #29371f);
    border: 1px solid #83a95c;
    font-size: 10px;
    font-weight: 700;
    white-space: nowrap;
}
.common-debuff-button:hover { filter: brightness(1.13); }
.debuff-alert-settings input[type="number"] { width: 54px; height: 25px; padding: 0 5px; color: #fff; background: #171916; border: 1px solid #59614e; }
.debuff-alert-settings select { height: 25px; padding: 0 5px; color: #fff; background: #171916; border: 1px solid #59614e; }
.debuff-alert-settings select:disabled { color: #777d73; background: #252724; border-color: #41463d; }
.debuff-alert-toolbar { display: grid; grid-template-columns: minmax(280px, 1fr); gap: 7px; margin-bottom: 7px; }
.debuff-alert-picker { min-width: 0; margin: 0; }
.debuff-boss-manager { display: grid; gap: 6px; margin: 8px 0; padding: 7px; background: #222520; border: 1px solid #454c3e; }
.debuff-boss-manager-copy { display: flex; flex-wrap: wrap; align-items: baseline; gap: 5px 10px; }
.debuff-boss-manager-copy strong { color: #dfffb9; font-size: 10px; }
.debuff-boss-manager-copy span, .debuff-boss-manager > p { margin: 0; color: #aeb5a8; font-size: 9px; }
.debuff-boss-picker { margin: 0; }
.debuff-boss-search-results { margin: 0; }
.debuff-boss-race-list { display: flex; flex-wrap: wrap; gap: 4px; }
.debuff-boss-race-list > span { display: inline-flex; align-items: center; gap: 5px; min-height: 26px; padding: 2px 3px 2px 7px; color: #eef5e8; background: #171916; border: 1px solid #57604d; }
.debuff-boss-race-list small { color: #aeb5a8; font-size: 8px; }
.debuff-boss-race-list button { display: grid; place-items: center; width: 21px; height: 21px; padding: 0; color: #ffc5b8; background: #30231f; border: 1px solid #694b43; }
.debuff-alert-rule-list { display: grid; gap: 4px; }
.debuff-alert-rule { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; min-height: 43px; padding: 6px; background: #1b1d1a; border: 1px solid #42473d; }
.debuff-alert-identity { display: grid; flex: 1 1 220px; grid-template-columns: 28px minmax(0, 1fr); align-items: center; gap: 1px 7px; min-width: 150px; }
.debuff-alert-identity .condition-icon-wrap { grid-row: 1 / 3; }
.debuff-alert-identity strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.debuff-alert-identity small { color: #adb3a7; font-size: 8px; }
.debuff-order-controls { display: inline-flex; gap: 2px; }
.debuff-order-controls button { display: grid; place-items: center; width: 24px; height: 24px; padding: 0; color: #e6e9e2; background: linear-gradient(#464941, #282a26); border: 1px solid #62685a; cursor: pointer; }
.debuff-order-controls button:disabled { color: #666b62; border-color: #3c4038; cursor: default; }
.debuff-warning-toggle, .debuff-warning-seconds, .debuff-sound-toggle, .debuff-sound-choice { display: inline-flex; align-items: center; gap: 5px; white-space: nowrap; }
.debuff-sound-choice select { min-width: 110px; }
@media (max-width: 1080px) {
    .debuff-alert-toolbar { grid-template-columns: 1fr; }
}

.boss-mechanic-settings {
    margin-top: 8px;
    padding: 8px;
    color: #eeeeee;
    background: #292d2f;
    border: 1px solid #6c6652;
    box-shadow: inset 0 0 0 1px rgba(255, 220, 142, .06);
    font-size: 10px;
}

.boss-mechanic-settings > header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 8px 12px;
    margin-bottom: 7px;
    padding-bottom: 7px;
    border-bottom: 1px solid #4a493f;
}

.boss-mechanic-settings > header > div:first-child { display: grid; gap: 2px; margin-right: auto; }
.boss-mechanic-settings > header strong { color: #ffe0a7; font-size: 12px; }
.boss-mechanic-settings > header small { color: #251a09; background: #e4b965; padding: 1px 4px; }
.boss-mechanic-settings > header span { color: #c9ccc4; font-size: 9px; }
.boss-mechanic-volume,
.boss-mechanic-display-setting { display: inline-flex; align-items: center; gap: 5px; white-space: nowrap; }
.boss-mechanic-volume input,
.boss-mechanic-display-setting input,
.boss-mechanic-rule input[type="number"] { width: 54px; height: 25px; padding: 0 5px; color: #fff; background: #171916; border: 1px solid #6c6652; }
.boss-mechanic-coordinate-setting input { width: 62px; }
.boss-mechanic-coordinate-setting b,
.boss-mechanic-rule-coordinate b { color: #c9ccc4; font-size: 9px; }
.boss-mechanic-rule-coordinate input { width: 62px !important; }
.local-tts-open {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 25px;
    padding: 0 8px;
    color: #e9ffd4;
    background: linear-gradient(#4d6339, #293820);
    border: 1px solid #83aa59;
}
.local-tts-open:hover { filter: brightness(1.14); }
.local-tts-backdrop { z-index: 80; }
.local-tts-dialog {
    width: min(620px, calc(100vw - 36px));
    color: #f1f3ed;
    background: #242722;
    border: 1px solid #818676;
    box-shadow: 0 16px 54px rgba(0, 0, 0, .72);
}
.local-tts-dialog > header {
    display: flex;
    align-items: center;
    justify-content: space-between;
    min-height: 36px;
    padding: 0 10px;
    background: linear-gradient(#20231f, #10120f);
    border-bottom: 1px solid #4f5549;
}
.local-tts-dialog > header > div { display: flex; align-items: center; gap: 7px; color: #d9ffaf; }
.local-tts-dialog > header button { color: #c9cec4; }
.local-tts-dialog > label,
.local-tts-fields label { display: grid; align-content: start; align-self: start; grid-template-rows: auto 29px auto; gap: 5px; color: #dfe5d8; font-size: 11px; }
.local-tts-dialog > label { margin: 10px 12px; }
.local-tts-dialog textarea,
.local-tts-dialog select,
.local-tts-dialog input {
    box-sizing: border-box;
    width: 100%;
    color: #fff;
    background: #121410;
    border: 1px solid #727865;
    outline: none;
}
.local-tts-dialog textarea { min-height: 72px; padding: 8px; resize: vertical; }
.local-tts-dialog select,
.local-tts-dialog input { height: 29px; padding: 0 7px; }
.local-tts-dialog textarea:focus,
.local-tts-dialog select:focus,
.local-tts-dialog input:focus { border-color: #a8e66d; box-shadow: 0 0 0 1px rgba(168, 230, 109, .24); }
.local-tts-dialog label small { justify-self: end; color: #9ca794; font-size: 9px; }
.local-tts-fields { display: grid; grid-template-columns: minmax(0, 1fr) 150px; align-items: start; gap: 10px; margin: 0 12px 10px; }
.local-tts-status { margin: 0 12px 10px; padding: 7px 9px; color: #c8ff95; background: #182014; border-left: 3px solid #8fd254; }
.local-tts-status.error { color: #ffd0c7; background: #2b1815; border-left-color: #e86d58; }
.local-tts-dialog > footer { display: flex; align-items: center; justify-content: flex-end; gap: 7px; padding: 9px 12px; background: #191b18; border-top: 1px solid #484d42; }
.local-tts-dialog > footer span { margin-right: auto; color: #b7c0b0; font-size: 10px; }
.boss-mechanic-unsaved { color: #b9f875 !important; font-weight: 700; }
.boss-mechanic-rule-list { display: grid; gap: 4px; }
.boss-mechanic-rule { display: flex; flex-wrap: wrap; align-items: center; gap: 7px; min-height: 46px; padding: 6px; background: #1b1d1a; border: 1px solid #4a493f; }
.boss-mechanic-rule > label { display: inline-flex; align-items: center; gap: 5px; white-space: nowrap; }
.boss-mechanic-rule select { height: 25px; padding: 0 5px; color: #fff; background: #171916; border: 1px solid #6c6652; }
.boss-mechanic-identity { display: grid; flex: 1 1 260px; grid-template-columns: 30px minmax(0, 1fr); gap: 1px 7px; align-items: center; }
.boss-mechanic-identity .v-icon { grid-row: 1 / 3; color: #ffd27a; }
.boss-mechanic-identity small { color: #adb3a7; font-size: 8px; }
.boss-mechanic-identity input[type="text"] { min-width: 130px; height: 25px; padding: 0 7px; color: #fff; background: #171916; border: 1px solid #6c6652; }
.miel-shard-phase-options { display: flex; flex: 1 1 430px; flex-wrap: wrap; align-items: center; gap: 5px 10px; }
.miel-shard-phase-options > label { display: inline-flex; align-items: center; gap: 4px; white-space: nowrap; }
.effect-timer-settings { border-color: #527b83; box-shadow: inset 0 0 0 1px rgba(108, 230, 255, .07); }
.effect-timer-settings > header strong { color: #9eebff; }
.effect-timer-rule { border-color: #3f6269; }

.skill-cooldown-settings > header {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 10px;
    margin-bottom: 8px;
}

.skill-cooldown-settings > header > div:first-child {
    display: grid;
    gap: 2px;
}

.skill-cooldown-settings > header strong {
    color: #c8f6ff;
    font-size: 12px;
}

.skill-cooldown-settings > header strong small {
    display: inline-block;
    margin-left: 4px;
    padding: 1px 4px;
    color: #102025;
    background: #87d7e7;
    border-radius: 2px;
    font-size: 8px;
    vertical-align: 1px;
}

.skill-cooldown-settings > header span,
.skill-cooldown-settings > p {
    color: #c2cfd1;
    font-size: 9px;
}

.skill-cooldown-header-actions {
    display: inline-flex !important;
    align-items: center;
    gap: 7px !important;
    margin-left: auto;
    white-space: nowrap;
}

.skill-cooldown-header-actions .skill-cooldown-unsaved {
    color: var(--ui-color-accent);
    font-size: 10px;
    font-weight: 700;
}

.skill-cooldown-icon-size {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: #e7edf0;
    font-size: 10px;
    font-weight: 700;
}

.skill-cooldown-icon-size input[type="number"] {
    width: 56px;
    height: 27px;
    text-align: center;
}

.skill-cooldown-icon-size span {
    color: #b9c8cc;
    font-size: 9px;
    font-weight: 400;
}

.skill-cooldown-save {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 4px;
    height: 27px;
    padding: 0 11px;
    color: var(--ui-theme-on-accent);
    background: var(--ui-color-accent);
    border: 1px solid var(--ui-theme-border);
    box-shadow: inset 0 1px 0 rgba(255, 255, 255, .16);
    font-size: 10px;
    font-weight: 700;
    cursor: pointer;
}

.skill-cooldown-save:hover:not(:disabled) {
    box-shadow: 0 0 9px rgba(var(--ui-color-rgb), .72), inset 0 1px 0 rgba(255, 255, 255, .2);
    filter: brightness(1.08);
}

.skill-cooldown-save.needs-save {
    animation: settings-save-attention 1.05s ease-in-out infinite;
}

.skill-cooldown-save.saved {
    box-shadow: 0 0 8px rgba(var(--ui-color-rgb), .62), inset 0 1px 0 rgba(255, 255, 255, .2);
    filter: saturate(.65) brightness(1.08);
}

.skill-cooldown-save:disabled { cursor: wait; filter: grayscale(.45); opacity: .7; }

@keyframes settings-save-attention {
    0%, 100% { border-color: var(--ui-theme-border); box-shadow: 0 0 0 rgba(var(--ui-color-rgb), 0), inset 0 1px 0 rgba(255, 255, 255, .14); }
    50% { border-color: var(--ui-color-accent); box-shadow: 0 0 9px rgba(var(--ui-color-rgb), .82), inset 0 1px 0 rgba(255, 255, 255, .22); }
}

.buff-alert-settings > header .buff-alert-unsaved {
    margin-left: 0;
    color: #b9f875;
    font-size: 10px;
    font-weight: 700;
    white-space: nowrap;
}

.buff-alert-settings > header {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 12px;
    margin-bottom: 8px;
}

.buff-alert-settings > header > div {
    display: grid;
    gap: 2px;
}

.buff-alert-settings > header strong {
    color: #dcffb5;
    font-size: 12px;
}

.buff-alert-settings > header span,
.buff-alert-settings > p {
    color: #c7cdbf;
    font-size: 9px;
}

.buff-alert-settings > header > .buff-alert-actions {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    margin-left: auto;
    white-space: nowrap;
}

.buff-alert-lock {
    display: flex;
    align-items: center;
    gap: 5px;
    margin-left: auto;
    white-space: nowrap;
}

.buff-alert-picker {
    display: grid;
    grid-template-columns: 110px minmax(220px, 1fr) auto;
    align-items: center;
    gap: 7px;
    padding: 6px;
    background: #282b25;
    border: 1px solid #20221e;
}

.buff-alert-add,
.buff-alert-remove {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 3px;
    height: 25px;
    padding: 0 9px;
    color: #eeeeee;
    background: linear-gradient(#4a4a4a, #242424);
    border: 1px solid #696969;
    font-size: 10px;
    cursor: pointer;
}

.buff-alert-runtime-status {
    flex: 1 0 100%;
    padding: 3px 6px;
    color: #d7e8c7;
    background: rgba(12, 15, 11, .42);
    border-left: 2px solid #80ad4d;
    font-size: 9px;
}

.buff-alert-mode-note {
    flex: 1 0 100%;
    padding: 3px 6px;
    color: #ffd9ad;
    background: rgba(73, 42, 19, .48);
    border-left: 2px solid #e09a4b;
    font-size: 9px;
}

.buff-alert-add:disabled {
    color: #777777;
    cursor: default;
}

.buff-alert-rule-list,
.skill-cooldown-rule-list {
    display: grid;
    gap: 6px;
    margin-top: 6px;
}

.buff-alert-settings select,
.buff-alert-settings input[type="number"],
.buff-alert-settings input[type="search"],
.skill-cooldown-settings select,
.skill-cooldown-settings input[type="number"],
.skill-cooldown-settings input[type="search"] {
    height: 25px;
    padding: 0 6px;
    color: #ffffff;
    background: #1d1f1b;
    border: 1px solid #6b765f;
    outline: none;
    font-size: 10px;
}

.buff-alert-settings input[type="search"],
.skill-cooldown-settings input[type="search"] {
    width: 100%;
}

.buff-alert-picker input[type="search"] {
    box-sizing: border-box;
    width: 100%;
    min-width: 0;
    height: 25px;
    padding: 0 6px;
    color: #ffffff;
    background-color: #1d1f1b;
    border: 1px solid #6b765f;
    border-radius: 0;
    outline: 0;
    box-shadow: none;
    font: inherit;
    appearance: none;
    -webkit-appearance: none;
}

.buff-alert-picker input[type="search"]:focus {
    background-color: #20241d;
    border-color: #9bb77d;
    box-shadow: inset 0 0 0 1px rgba(155, 183, 125, .2);
}

.buff-alert-picker input[type="search"]::-webkit-search-decoration,
.buff-alert-picker input[type="search"]::-webkit-search-cancel-button,
.buff-alert-picker input[type="search"]::-webkit-search-results-button,
.buff-alert-picker input[type="search"]::-webkit-search-results-decoration {
    display: none;
    -webkit-appearance: none;
}

.buff-alert-search-results {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(190px, 1fr));
    gap: 4px;
    max-height: 150px;
    padding: 6px;
    overflow-y: auto;
    background: #242622;
    border: 1px solid #3c4435;
    border-top: 0;
}

.buff-alert-search-results > button {
    display: grid;
    grid-template-columns: 25px minmax(0, 1fr) 16px;
    align-items: center;
    gap: 7px;
    min-width: 0;
    min-height: 34px;
    padding: 4px 6px;
    color: #eeeeee;
    background: linear-gradient(#484b43, #34372f);
    border: 1px solid #5f6a53;
    text-align: left;
    cursor: pointer;
}

.buff-alert-search-results > button:disabled {
    color: #8d9486;
    cursor: default;
}

.buff-alert-search-results > button > span:nth-child(2) {
    display: grid;
    min-width: 0;
}

.buff-alert-search-results strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.buff-alert-search-results small {
    color: #aeb5a6;
    font-size: 8px;
}

.buff-alert-editor,
.skill-cooldown-editor {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 6px;
    padding: 7px;
    background: #33362f;
    border: 1px solid #242620;
}

.skill-cooldown-builtin-rule {
    flex: 1 0 100%;
    display: flex;
    align-items: flex-start;
    gap: 5px;
    margin: 0;
    padding: 6px 8px;
    border: 1px solid color-mix(in srgb, var(--ui-color-accent) 35%, #242620);
    background: color-mix(in srgb, var(--ui-color-accent) 10%, #11130f);
    color: #d9e8d1;
    font-size: 9px;
    line-height: 1.45;
}

.skill-cooldown-builtin-rule .v-icon {
    flex: 0 0 auto;
    margin-top: 1px;
    color: var(--ui-color-accent);
}

.buff-alert-editor > label,
.skill-cooldown-editor > label {
    display: flex;
    align-items: center;
    gap: 5px;
    white-space: nowrap;
}

.skill-cooldown-cumulative-fields {
    display: inline-flex;
    flex-wrap: wrap;
    align-items: center;
    gap: 5px;
    padding: 4px 6px;
    color: #ffe4ad;
    background: rgba(52, 35, 15, .58);
    border: 1px solid #79623b;
}

.skill-cooldown-cumulative-fields::before {
    color: #ffc765;
    content: "累计冷却";
    font-size: 9px;
    font-weight: 800;
}

.skill-cooldown-cumulative-fields > label {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    white-space: nowrap;
}

.skill-cooldown-cumulative-fields input[type="number"] {
    width: 62px;
}

.skill-cooldown-coordinates,
.buff-stack-alert-coordinates {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    padding: 3px 5px;
    color: #c7d6d9;
    background: rgba(10, 15, 17, .48);
    border: 1px solid #566d72;
    white-space: nowrap;
}

.skill-cooldown-coordinates > span,
.buff-stack-alert-coordinates > span {
    color: #a9c8cf;
    font-size: 9px;
    font-weight: 700;
}

.skill-cooldown-coordinates > label,
.buff-stack-alert-coordinates > label {
    display: inline-flex;
    align-items: center;
    gap: 3px;
    font-size: 9px;
    font-weight: 700;
}

.skill-cooldown-coordinates input[type="number"],
.buff-stack-alert-coordinates input[type="number"] {
    width: 62px;
    min-width: 62px !important;
    max-width: 62px !important;
}

.buff-stack-alert-controls {
    display: flex;
    flex: 1 1 620px;
    flex-wrap: wrap;
    justify-content: flex-end;
    align-items: center;
    gap: 6px;
    margin-left: auto;
}

.buff-stack-alert-controls > label {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    white-space: nowrap;
}

.buff-stack-alert-controls > label select,
.buff-stack-alert-controls > label input[type="number"] {
    min-width: 70px;
    max-width: 120px;
}

.buff-alert-editor > label select,
.buff-alert-editor > label input[type="number"],
.skill-cooldown-editor > label input[type="number"] {
    min-width: 70px;
    max-width: 120px;
}

.buff-alert-identity {
    display: grid;
    flex: 1 1 170px;
    grid-template-columns: 25px minmax(0, 1fr);
    align-items: center;
    gap: 2px 7px;
    min-width: 0;
}

.buff-alert-identity .condition-icon-wrap,
.buff-alert-identity .skill-alert-icon {
    grid-row: 1 / 3;
}

.buff-alert-identity strong {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.buff-alert-identity small {
    color: #aeb5a6;
    font-size: 8px;
}

.buff-alert-preview {
    height: 25px;
    padding: 0 9px;
    color: #ffffff;
    background: linear-gradient(#637c43, #354524);
    border: 1px solid #8eaf61;
    cursor: pointer;
}

.buff-alert-preview,
.local-tts-open {
    color: var(--ui-theme-text);
    background: var(--ui-theme-raised);
    border-color: var(--ui-color-accent);
}

.buff-alert-preview:hover:not(:disabled),
.local-tts-open:hover:not(:disabled),
.aim-reminder-preview:hover:not(:disabled) {
    background: var(--ui-theme-control);
    box-shadow: 0 0 7px rgba(var(--ui-color-rgb), .46);
    filter: brightness(1.08);
}

.buff-alert-preview:disabled {
    color: #8c9288;
    background: #30332e;
    border-color: #555b50;
    cursor: not-allowed;
}

.buff-alert-file-picker {
    position: relative;
    max-width: 190px;
    height: 25px;
    padding: 0 9px;
    overflow: hidden;
    color: #efffda;
    background: linear-gradient(#52673a, #2f3d23);
    border: 1px solid #789650;
    cursor: pointer;
}

.buff-alert-file-picker input[type="file"] {
    position: absolute;
    width: 1px;
    height: 1px;
    opacity: 0;
    pointer-events: none;
}

.buff-alert-file-picker span {
    max-width: 165px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.buff-alert-remove {
    color: #ffd6d6;
    border-color: #825454;
}

.skill-alert-icon {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 31px;
    height: 31px;
    overflow: hidden;
    color: #8b989b;
    background: #172025;
    border: 1px solid #101719;
    box-shadow: 0 0 0 1px #72888d;
    font-size: 7px;
    font-weight: 700;
}

.skill-alert-icon img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: cover;
}

.skill-alert-icon.compact {
    width: 23px;
    height: 23px;
}

.player-buff-search-row {
    display: flex;
    align-items: center;
    gap: 5px;
    padding: 7px;
    background: #353433;
    border: 1px solid #292827;
}

.player-buff-search-row > :deep(.v-icon) {
    color: #a9d875;
}

.player-buff-search-row input {
    flex: 1;
    min-width: 180px;
    height: 27px;
    padding: 0 8px;
    color: #ffffff;
    background: #202020;
    border: 1px solid #696969;
    outline: none;
    font-size: 10px;
}

.player-buff-search-row input:focus {
    border-color: #9ccc64;
}

.player-buff-search-row button {
    height: 27px;
    padding: 0 9px;
    color: #eeeeee;
    background: linear-gradient(#444444, #202020);
    border: 1px solid #666666;
    font-size: 10px;
    cursor: pointer;
}

.player-buff-search-results {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
    gap: 4px;
    max-height: 158px;
    padding: 6px;
    overflow-y: auto;
    background: #2f2e2d;
    border: 1px solid #242322;
    border-top: 0;
}

.player-buff-search-results > button {
    display: grid;
    grid-template-columns: 25px minmax(0, 1fr) 16px;
    align-items: center;
    gap: 7px;
    min-width: 0;
    min-height: 34px;
    padding: 4px 6px;
    color: #eeeeee;
    background: linear-gradient(#4b4a49, #383736);
    border: 1px solid #5e5c5a;
    text-align: left;
    cursor: pointer;
}

.player-buff-search-results > button:hover:not(:disabled) {
    border-color: #91c653;
}

.player-buff-search-results > button:disabled {
    color: #989898;
    cursor: default;
}

.player-buff-search-results > button > span:nth-child(2) {
    display: grid;
    min-width: 0;
}

.player-buff-search-results strong {
    overflow: hidden;
    font-size: 10px;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.player-buff-search-results small {
    color: #b2b2b2;
    font-size: 8px;
}

.player-buff-search-empty {
    padding: 7px 9px;
    color: #ffd178;
    background: #333231;
    border: 1px solid #292827;
    border-top: 0;
    font-size: 9px;
}

.player-buff-manage-group {
    border-top: 1px solid #343332;
}

.player-buff-manage-group:first-of-type {
    margin-top: 7px;
}

.player-buff-manage-group > summary {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto 18px;
    align-items: center;
    min-height: 29px;
    padding: 0 7px 0 9px;
    color: #f0f0f0;
    background: linear-gradient(#565452, #41403f);
    border: 1px solid #323130;
    font-size: 10px;
    font-weight: 700;
    list-style: none;
    cursor: pointer;
}

.player-buff-manage-group > summary::-webkit-details-marker,
.condition-category > summary::-webkit-details-marker {
    display: none;
}

.player-buff-manage-group > summary small {
    color: #c7c7c7;
    font-size: 9px;
    font-weight: 400;
}

.player-buff-manage-group > summary :deep(.v-icon),
.condition-category > summary :deep(.v-icon) {
    transition: transform .16s ease;
}

.player-buff-manage-group[open] > summary :deep(.v-icon),
.condition-category[open] > summary :deep(.v-icon) {
    transform: rotate(180deg);
}

.player-buff-chip-list {
    display: flex;
    flex-wrap: wrap;
    gap: 4px;
    padding: 6px 8px;
    background: #393837;
    border: 1px solid #302f2e;
    border-top: 0;
}

.player-buff-chip {
    height: 23px;
    padding: 0 6px;
    color: #bcbcbc;
    background: #292929;
    border: 1px solid #555555;
    font-size: 9px;
    cursor: pointer;
}

.player-buff-chip span {
    display: inline-block;
    width: 10px;
    color: #979797;
}

.player-buff-chip small {
    color: #8f8f8f;
    font-size: 8px;
}

.player-buff-chip.selected {
    color: #ffffff;
    background: linear-gradient(#4d642a, #283516);
    border-color: #87b847;
}

.player-buff-chip.selected span {
    color: #a9ee52;
}

.condition-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(92px, 1fr));
    gap: 6px;
    padding: 8px;
}

.favorite-condition-section {
    border-bottom: 1px solid #393837;
}

.favorite-condition-section > header {
    display: flex;
    align-items: center;
    gap: 5px;
    min-height: 27px;
    padding: 0 9px;
    color: #f2f2f2;
    background: #464442;
    font-size: 10px;
}

.favorite-condition-section > header :deep(.v-icon) {
    color: #ffd34f;
}

.favorite-condition-section > header span {
    color: #c8c8c8;
    font-size: 9px;
}

.favorite-condition-grid {
    background: #555351;
}

.condition-category-list {
    padding: 7px 8px 8px;
}

.condition-category {
    border-bottom: 1px solid #323130;
}

.condition-category > summary {
    display: grid;
    grid-template-columns: minmax(0, 1fr) auto 20px;
    align-items: center;
    min-height: 34px;
    padding: 0 8px 0 11px;
    color: #f4f4f4;
    background: linear-gradient(#63615f, #4c4a48);
    border: 1px solid #393836;
    box-shadow: inset 0 1px rgba(255, 255, 255, .08);
    font-size: 11px;
    font-weight: 700;
    list-style: none;
    text-shadow: 0 1px #252525;
    cursor: pointer;
}

.condition-category > summary:hover {
    background: linear-gradient(#6d6b68, #514f4d);
}

.condition-category > summary small {
    color: #d2d2d2;
    font-size: 9px;
    font-weight: 400;
}

.condition-category > .condition-grid {
    background: #484644;
    border: 1px solid #393836;
    border-top: 0;
}

.condition-card {
    position: relative;
    box-sizing: border-box;
    min-width: 0;
    min-height: 94px;
    overflow: hidden;
    color: #eeeeee;
    background: linear-gradient(#6e6c6b, #5a5857);
    border: 1px solid #363534;
    box-shadow: inset 0 0 0 1px #7d7b79;
    font-family: "Microsoft YaHei", sans-serif;
}

.condition-card:hover,
.condition-card.selected,
.condition-card:focus-within {
    background: linear-gradient(#777572, #5e5c59);
    border-color: #9bd35b;
    box-shadow: inset 0 0 0 1px #a5d86a, 0 0 3px rgba(137, 205, 57, .6);
    outline: none;
}

.condition-card-main {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    width: 100%;
    min-height: 92px;
    padding: 6px 5px 5px;
    color: inherit;
    background: transparent;
    border: 0;
    font: inherit;
    cursor: pointer;
}

.condition-card-main:focus {
    outline: none;
}

.condition-favorite-button {
    position: absolute;
    top: 3px;
    right: 3px;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 21px;
    height: 21px;
    padding: 0;
    color: #d4d4d4;
    background: rgba(35, 35, 35, .82);
    border: 1px solid #77736c;
    cursor: pointer;
}

.condition-alert-button {
    position: absolute;
    z-index: 2;
    top: 3px;
    left: 3px;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 21px;
    height: 21px;
    padding: 0;
    color: #d4d4d4;
    background: rgba(35, 35, 35, .82);
    border: 1px solid #77736c;
    cursor: pointer;
}

.condition-alert-button:hover,
.condition-alert-button.active {
    color: #dfffaf;
    background: #2e3925;
    border-color: #91c955;
    box-shadow: 0 0 4px rgba(144, 219, 72, .75);
}

.condition-favorite-button:hover,
.condition-favorite-button.active {
    color: #ffd34f;
    border-color: #d2b84c;
    background: #31302c;
}

.condition-icon-wrap {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 31px;
    height: 31px;
    color: #878787;
    background: #202020;
    border: 1px solid #151515;
    box-shadow: 0 0 0 1px #8a8a8a;
    font-size: 9px;
    font-weight: 700;
}

.condition-icon-wrap img {
    position: absolute;
    inset: 0;
    width: 100%;
    height: 100%;
    object-fit: contain;
}

.condition-icon-wrap.compact {
    width: 23px;
    height: 23px;
    font-size: 7px;
}

.condition-coverage {
    margin-top: 5px;
    color: #d9ffad;
    font-size: 12px;
    font-variant-numeric: tabular-nums;
    text-shadow: 0 1px 1px #222222;
}

.condition-name {
    width: 100%;
    margin-top: 2px;
    overflow: hidden;
    color: #ffffff;
    font-size: 10px;
    text-align: center;
    text-overflow: ellipsis;
    text-shadow: 0 1px #242424;
    white-space: nowrap;
}

.condition-card-main > small {
    margin-top: 1px;
    color: #c0c0c0;
    font-size: 8px;
}

.condition-timeline-panel {
    margin: 0 8px 8px;
    padding: 7px 9px 8px;
    background: #343332;
    border: 1px solid #242424;
    box-shadow: inset 0 0 0 1px #595755;
}

.condition-timeline-panel > header {
    display: flex;
    align-items: center;
    margin-bottom: 8px;
}

.condition-timeline-heading {
    display: flex;
    align-items: center;
    gap: 8px;
}

.condition-timeline-heading .condition-icon-wrap {
    width: 27px;
    height: 27px;
}

.condition-timeline-heading > div:last-child {
    display: flex;
    flex-direction: column;
}

.condition-timeline-heading strong {
    color: #ffffff;
    font-size: 11px;
}

.condition-timeline-heading span {
    margin-top: 2px;
    color: #cfcfcf;
    font-size: 9px;
}

.condition-timeline-panel > header > button {
    margin-left: auto;
    color: #d0d0d0;
    background: transparent;
    border: 0;
    cursor: pointer;
}

.condition-timeline-row {
    padding: 0 3px;
}

.condition-timeline-bars {
    position: relative;
    height: 25px;
    background: linear-gradient(#202020, #2c2c2c);
    border: 1px solid #171717;
    box-shadow: inset 0 0 0 1px #4b4b4b;
}

.condition-timeline-bars > i {
    position: absolute;
    top: 0;
    bottom: 0;
    width: 1px;
    pointer-events: none;
}

.condition-timeline-bars > i.condition-timeline-major-tick {
    background: rgba(210, 210, 210, .24);
}

.condition-timeline-bars > i.condition-timeline-minor-tick {
    top: 5px;
    background: rgba(190, 190, 190, .12);
}

.condition-timeline-segment {
    position: absolute;
    z-index: 1;
    top: 4px;
    bottom: 4px;
    min-width: 2px;
    background: linear-gradient(#a5ef3d, #6fbe08);
    border: 1px solid #4b8700;
    box-shadow: inset 0 1px rgba(255, 255, 255, .35);
}

.condition-timeline-axis {
    position: relative;
    height: 18px;
    color: #c4c4c4;
    font-size: 8px;
}

.condition-timeline-axis > i {
    position: absolute;
    top: 0;
    width: 1px;
    pointer-events: none;
}

.condition-timeline-axis > i.condition-timeline-major-tick {
    height: 3px;
    background: rgba(220, 220, 220, .42);
}

.condition-timeline-axis > i.condition-timeline-minor-tick {
    height: 2px;
    background: rgba(190, 190, 190, .2);
}

.condition-timeline-axis span {
    position: absolute;
    top: 3px;
    transform: translateX(-50%);
    white-space: nowrap;
}

.condition-timeline-axis span:first-child {
    transform: none;
}

.condition-timeline-axis span:last-child {
    transform: translateX(-100%);
}

.condition-empty {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 7px;
    min-height: 76px;
    color: #c6c6c6;
    font-size: 11px;
}

.condition-empty button {
    height: 25px;
    padding: 0 9px;
    color: #f1f1f1;
    background: linear-gradient(#4d4d4d, #282828);
    border: 1px solid #707070;
    font-size: 10px;
    cursor: pointer;
}

@media (max-width: 820px) {
    .condition-panel-header {
        flex-wrap: wrap;
        gap: 5px;
    }

    .condition-target-toggle {
        margin-left: 0;
    }

    .condition-panel-title {
        flex: 1;
    }

    .player-buff-search-row {
        flex-wrap: wrap;
    }

    .player-buff-search-row input {
        min-width: calc(100% - 24px);
    }
}

.skill-grid {
    display: grid;
    grid-template-columns: 219fr 187fr 146fr 146fr 146fr 147fr 145fr;
    align-items: center;
}

.skill-head {
    gap: 5px;
    height: 30px;
    padding: 4px 1px;
    margin-left: 0;
    margin-right: 28px;
}

.skill-head > span {
    height: 25px;
    color: #efefef;
    background: linear-gradient(#313131, #161616);
    border: 1px solid #545454;
    box-shadow: inset 0 0 0 1px #0c0c0c;
    font-size: 10px;
    font-weight: 700;
    line-height: 23px;
    text-align: center;
    text-shadow: 0 1px #000000;
}

.skill-head > span:nth-child(2) {
    color: #ffffff;
    background: linear-gradient(#f0f0f0, #767676);
    border-color: #e2e2e2;
    text-shadow: 0 1px 2px #000000;
}

.skill-scroll {
    height: calc(100% - 57px);
    padding-left: 0;
    padding-right: 14px;
    overflow-y: scroll;
    scrollbar-color: #5b5b5b #151515;
    scrollbar-width: thin;
}

.skill-scroll::-webkit-scrollbar { width: 14px; }
.skill-scroll::-webkit-scrollbar-track { background: #151515; border: 1px solid #555555; }
.skill-scroll::-webkit-scrollbar-thumb { background: #4f4f4f; border: 2px solid #1a1a1a; }

.skill-row {
    position: relative;
    min-height: 40px;
    margin-bottom: 3px;
    overflow: hidden;
    color: #f8f8f8;
    background: linear-gradient(#f5f8f9, #e2e8eb 55%, #cbd4d9);
    border: 3px solid #121516;
    outline: 1px solid #748087;
    box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.58);
    font-size: 12px;
    font-weight: 600;
    text-align: center;
    text-shadow:
        -1px -1px 0 #0b0b0b,
        0 -1px 0 #0b0b0b,
        1px -1px 0 #0b0b0b,
        -1px 0 0 #0b0b0b,
        1px 0 0 #0b0b0b,
        -1px 1px 0 #0b0b0b,
        0 1px 0 #0b0b0b,
        1px 1px 0 #0b0b0b;
}

.skill-progress-track {
    position: absolute;
    z-index: 0;
    top: 2px;
    right: 2px;
    bottom: 2px;
    left: 34px;
}

.skill-progress-track span {
    display: block;
    height: 100%;
    background: linear-gradient(#a8f328, #82dc04 55%, #60b900);
    box-shadow:
        inset 0 1px rgba(255, 255, 255, 0.38),
        inset 0 -1px rgba(39, 99, 0, 0.34),
        1px 0 rgba(39, 99, 0, 0.62);
    transition: width 220ms ease-out;
}

.skill-row > *:not(.skill-progress-track) {
    position: relative;
    z-index: 1;
}

.skill-row > span,
.skill-name-text {
    color: #f8f8f8;
    -webkit-text-stroke: 0.35px #0b0b0b;
    paint-order: stroke fill;
}

.skill-name-cell {
    display: grid;
    grid-template-columns: 36px 1fr;
    align-items: center;
    min-width: 0;
    height: 100%;
    text-align: center;
}

.skill-icon {
    width: 30px;
    height: 30px;
    margin-left: 1px;
    object-fit: contain;
    background: #111111;
    border: 1px solid #171717;
    box-shadow: 0 0 0 1px #777777;
    image-rendering: auto;
}

.skill-name-text {
    min-width: 0;
    padding: 0 5px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
}

.empty-combat-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 7px;
    height: calc(100% - 61px);
    color: #6f6f6f;
    font-size: 11px;
}

.empty-combat-state strong {
    color: #a9a9a9;
    font-size: 13px;
}

.recovery-state {
    height: calc(100% - 27px);
}

@media (max-width: 1050px) {
    .report-controls {
        flex-wrap: wrap;
    }

    .team-chart-button { margin-left: 0; }
    .reminder-profile-toolbar { grid-template-columns: repeat(2, minmax(220px, 1fr)); }

    .combat-window {
        height: calc(100vh - 144px);
    }

    .skill-grid {
        min-width: 850px;
    }
}
</style>
