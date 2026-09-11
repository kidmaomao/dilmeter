package main

import (
	"context"
	"encoding/json"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
)

const (
	nativeReminderTickInterval       = 100 * time.Millisecond
	nativeDebuffSoundCooldown        = 1800 * time.Millisecond
	nativeDebuffMissingDebounce      = 1200 * time.Millisecond
	nativeBuffSoundExpiryDedupWindow = 1500 * time.Millisecond
	nativeBossMinimumHealth          = 100_000_000
	// Retained for legacy fixtures; phase selection no longer relies on one fixed shard HP value.
	nativeMiel60ShardMaximumHealth          = 63_803_160
	nativeMagnumAimReferenceDistance        = 1000.0
	nativeMagnumAimRangeDistanceFactor      = 7.0
	nativeMagnumAimRangeOffsetSeconds       = 1.0
	nativeMagnumDefaultWeaponRange          = 2200.0
	nativeMagnumRangePerIdentificationLevel = 70.0
	nativeMagnumBestSystemAimRate           = 0.7
)

type nativeReminderSettings struct {
	Buff              nativeBuffSettings          `json:"buff"`
	Debuff            nativeDebuffSettings        `json:"debuff"`
	SkillCooldowns    nativeSkillCooldownSettings `json:"skillCooldowns"`
	BossMechanics     nativeBossMechanicSettings  `json:"bossMechanics"`
	EffectTimers      nativeEffectTimerSettings   `json:"effectTimers"`
	PreferredBossID   string                      `json:"preferredBossId"`
	PreferredBossName string                      `json:"preferredBossName"`
	BossRaceNames     map[uint32]string           `json:"bossRaceNames"`
}

type nativeBuffSettings struct {
	Locked                *bool                     `json:"locked"`
	Volume                int                       `json:"volume"`
	IconSize              int                       `json:"iconSize"`
	DPIPercent            int                       `json:"dpiPercent"`
	OverlayEnabled        bool                      `json:"overlayEnabled"`
	Opacity               int                       `json:"opacity"`
	TimeAdjustmentSeconds int                       `json:"timeAdjustmentSeconds"`
	Rules                 map[uint32]nativeBuffRule `json:"rules"`
}

type nativeBuffRule struct {
	CCID                  uint32 `json:"ccId"`
	Name                  string `json:"name"`
	IconURL               string `json:"iconUrl"`
	OverlayEnabled        bool   `json:"overlayEnabled"`
	DurationMode          string `json:"durationMode"`
	ManualDurationSeconds int    `json:"manualDurationSeconds"`
	FlashEnabled          bool   `json:"flashEnabled"`
	FlashThresholdSeconds int    `json:"flashThresholdSeconds"`
	SoundMode             string `json:"soundMode"`
	SoundThresholdSeconds int    `json:"soundThresholdSeconds"`
	CustomSoundID         string `json:"customSoundId"`
	StackAlertEnabled     bool   `json:"stackAlertEnabled"`
	StackScreenEnabled    bool   `json:"stackScreenEnabled"`
	StackThreshold        int    `json:"stackThreshold"`
	StackX                int    `json:"stackX"`
	StackY                int    `json:"stackY"`
	StackSoundMode        string `json:"stackSoundMode"`
	StackCustomSoundID    string `json:"stackCustomSoundId"`
}

type nativeDebuffSettings struct {
	Volume            int                         `json:"volume"`
	IconSize          int                         `json:"iconSize"`
	OverlayEnabled    bool                        `json:"overlayEnabled"`
	ForcedBossRaceIDs []uint32                    `json:"forcedBossRaceIds"`
	Rules             map[uint32]nativeDebuffRule `json:"rules"`
}

type nativeDebuffRule struct {
	CCID           uint32 `json:"ccId"`
	Name           string `json:"name"`
	IconURL        string `json:"iconUrl"`
	Enabled        bool   `json:"enabled"`
	WarningSeconds int    `json:"warningSeconds"`
	FlashEnabled   bool   `json:"flashEnabled"`
	SoundEnabled   bool   `json:"soundEnabled"`
	SoundMode      string `json:"soundMode"`
	CustomSoundID  string `json:"customSoundId"`
	Order          int    `json:"order"`
}

type nativeSkillCooldownSettings struct {
	Volume      int                                `json:"volume"`
	IconSize    int                                `json:"iconSize"`
	AimReminder nativeAimReminderSettings          `json:"aimReminder"`
	Rules       map[uint16]nativeSkillCooldownRule `json:"rules"`
}

type nativeAimReminderSettings struct {
	Enabled                  bool    `json:"enabled"`
	AlwaysVisible            bool    `json:"alwaysVisible"`
	WeaponRange              float64 `json:"weaponRange"`
	RangeIdentificationLevel int     `json:"rangeIdentificationLevel"`
	CalibrationPercent       float64 `json:"calibrationPercent"`
	ErgSpeedPercent          float64 `json:"ergSpeedPercent"`
	FineTuneSeconds          float64 `json:"fineTuneSeconds"`
	ScalePercent             int     `json:"scalePercent"`
	X                        int     `json:"x"`
	Y                        int     `json:"y"`
	IconURL                  string  `json:"iconUrl"`
}

type nativeSkillCooldownRule struct {
	SkillID                   uint16  `json:"skillId"`
	Name                      string  `json:"name"`
	IconURL                   string  `json:"iconUrl"`
	Enabled                   bool    `json:"enabled"`
	BarOnly                   bool    `json:"barOnly,omitempty"`
	CooldownSeconds           float64 `json:"cooldownSeconds"`
	ShortCooldownSeconds      float64 `json:"shortCooldownSeconds"`
	CumulativeCooldownSeconds float64 `json:"cumulativeCooldownSeconds"`
	AlwaysVisible             bool    `json:"alwaysVisible"`
	OwnerMode                 string  `json:"ownerMode"`
	SoundMode                 string  `json:"soundMode"`
	CustomSoundID             string  `json:"customSoundId"`
	ProgressThresholdPercent  float64 `json:"progressThresholdPercent"`
	X                         int     `json:"x"`
	Y                         int     `json:"y"`
}

type nativeBossMechanicSettings struct {
	Volume                    int                               `json:"volume"`
	X                         int                               `json:"x"`
	Y                         int                               `json:"y"`
	ScalePercent              int                               `json:"scalePercent"`
	Miel60HealthBarEnabled    *bool                             `json:"miel60HealthBarEnabled"`
	Miel60HealthBarX          int                               `json:"miel60HealthBarX"`
	Miel60HealthBarY          int                               `json:"miel60HealthBarY"`
	Miel60HealthBarScale      int                               `json:"miel60HealthBarScalePercent"`
	MielShardHealthPhases     *nativeMielShardHealthPhases      `json:"mielShardHealthPhases"`
	MielShardHealthBarX       int                               `json:"mielShardHealthBarX"`
	MielShardHealthBarY       int                               `json:"mielShardHealthBarY"`
	MielShardHealthBarScale   int                               `json:"mielShardHealthBarScalePercent"`
	MielShardHealthBarOpacity int                               `json:"mielShardHealthBarOpacityPercent"`
	Rules                     map[string]nativeBossMechanicRule `json:"rules"`
}

type nativeMielShardHealthPhases struct {
	Normal80 bool `json:"normal80"`
	Normal60 bool `json:"normal60"`
	Normal40 bool `json:"normal40"`
	Regret80 bool `json:"regret80"`
}

type nativeEffectTimerSettings struct {
	Rules map[string]nativeEffectTimerRule `json:"rules"`
}

type nativeEffectTimerRule struct {
	Key             string  `json:"key"`
	Enabled         bool    `json:"enabled"`
	SourceType      string  `json:"sourceType"`
	SourceID        uint32  `json:"sourceId"`
	Name            string  `json:"name"`
	IconURL         string  `json:"iconUrl"`
	DurationSeconds float64 `json:"durationSeconds"`
	AlwaysVisible   bool    `json:"alwaysVisible"`
	TargetMode      string  `json:"targetMode"`
	Orientation     string  `json:"orientation"`
	ScalePercent    int     `json:"scalePercent"`
	OpacityPercent  int     `json:"opacityPercent"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
}

type nativeBossMechanicRule struct {
	Key              string   `json:"key"`
	Name             string   `json:"name"`
	BossRaceIDs      []uint32 `json:"bossRaceIds"`
	Trigger          string   `json:"trigger"`
	SkillID          uint16   `json:"skillId"`
	TriggerRaceIDs   []uint32 `json:"triggerRaceIds"`
	Enabled          bool     `json:"enabled"`
	CountdownSeconds float64  `json:"countdownSeconds"`
	ShowCountdown    bool     `json:"showCountdown"`
	SoundMode        string   `json:"soundMode"`
	CustomSoundID    string   `json:"customSoundId"`
	X                int      `json:"x"`
	Y                int      `json:"y"`
}

type nativeReminderCondition struct {
	CCID        uint32
	At          int64
	DisableAt   int64
	DisableAtMs int64
	DurationMs  int64
	Metadata    string
}

type nativeReminderEntity struct {
	ID             string
	Name           string
	OwnerID        string
	RaceID         uint32
	Known          bool
	Active         bool
	AppearedAt     int64
	MaximumHealth  float64
	CurrentHealth  float64
	TotalDamage    float64
	LastDamageAtMs int64
	Conditions     map[uint32]nativeReminderCondition
	RefreshGuard   map[uint32]int64
}

type nativeReminderSoundRequest struct {
	Kind         string
	Volume       int
	SoundID      string
	EnqueuedAtMs int64
	Priority     bool
}

type nativeSkillCooldownRuntime struct {
	UsedAtMs                   int64
	ReadyAtMs                  int64
	ShortReadyAtMs             int64
	AccumulatedReadyAtMs       int64
	AccumulatedCooldownSeconds float64
	CooldownPhase              string
	Generation                 uint64
	PetSkill                   bool
}

type nativeMagnumAimRuntime struct {
	Active             bool
	TargetID           string
	StartedAtMs        int64
	ReadyAtMs          int64
	SpeedMultiplier    float64
	BuffNames          []string
	CalibrationPercent float64
	Generation         uint64
}

type nativeBossMechanicRuntime struct {
	Key                    string
	StartedAtMs            int64
	EndsAtMs               int64
	Generation             uint64
	BossID                 string
	BossRaceID             uint32
	RequiredContacts       int
	CheckpointAtMs         int64
	ContactCount           int
	CheckpointContactCount int
	LastContactAtMs        int64
	CheckpointEvaluated    bool
	ProvisionallyHidden    bool
}

type nativeBuffStackRuntime struct {
	CCID        uint32
	Name        string
	Stack       int
	StartedAtMs int64
	EndsAtMs    int64
	Generation  uint64
}

type nativeEffectTimerRuntime struct {
	StartedAtMs int64
	EndsAtMs    int64
	Generation  uint64
	TargetID    string
}

type nativeReminderRuntime struct {
	ctx              context.Context
	settingsCh       chan nativeReminderSettings
	positionCh       chan nativeReminderPositionUpdate
	eventsCh         chan []event.IEvent
	settingsMu       sync.RWMutex
	settings         nativeReminderSettings
	entities         map[string]*nativeReminderEntity
	localID          string
	selectedTargetID string
	activeBossID     string
	preferredBossID  string

	announcedBuffSounds     map[string]struct{}
	lastBuffSoundExpiryMs   map[uint32]int64
	buffStackAbove          map[uint32]bool
	buffStackMissingSince   map[uint32]int64
	observedDebuffs         map[string]bool
	lastDebuffAppliedAt     map[string]int64
	debuffMissingSinceMs    map[string]int64
	announcedDebuffSounds   map[string]struct{}
	debuffSoundBlockedUntil time.Time
	skillCooldowns          map[uint16]nativeSkillCooldownRuntime
	magnumAim               nativeMagnumAimRuntime
	finalShotActive         bool
	bossMechanics           map[string]nativeBossMechanicRuntime
	buffStackAlerts         map[uint32]nativeBuffStackRuntime
	effectTimers            map[string]nativeEffectTimerRuntime
	announcedSkillSounds    map[string]struct{}
	recentBossMechanicAtMs  map[string]int64
	toahProgressObserved    bool
	toahProgress            float64
	toahThresholdReached    bool
	toahFullChargeReached   bool
	toahGeneration          uint64

	lastBuffStateKey   string
	lastDebuffStateKey string
	lastSkillStateKey  string
	playSound          func(nativeReminderSoundRequest) bool
}

var nativeReminderRuntimeHolder struct {
	sync.RWMutex
	runtime *nativeReminderRuntime
}

var stackCountPattern = regexp.MustCompile(`(?i)(?:^|[^A-Z0-9_])MCSTCT\s*:\s*\d+\s*:\s*(-?\d+)`)

func startNativeReminderRuntime(ctx context.Context, publisher *eventPublisher) *nativeReminderRuntime {
	runtime := &nativeReminderRuntime{
		ctx:                    ctx,
		settingsCh:             make(chan nativeReminderSettings, 4),
		positionCh:             make(chan nativeReminderPositionUpdate, 32),
		eventsCh:               make(chan []event.IEvent, 10000),
		settings:               loadNativeReminderSettings(),
		entities:               make(map[string]*nativeReminderEntity),
		announcedBuffSounds:    make(map[string]struct{}),
		lastBuffSoundExpiryMs:  make(map[uint32]int64),
		buffStackAbove:         make(map[uint32]bool),
		buffStackMissingSince:  make(map[uint32]int64),
		observedDebuffs:        make(map[string]bool),
		lastDebuffAppliedAt:    make(map[string]int64),
		debuffMissingSinceMs:   make(map[string]int64),
		announcedDebuffSounds:  make(map[string]struct{}),
		skillCooldowns:         make(map[uint16]nativeSkillCooldownRuntime),
		bossMechanics:          make(map[string]nativeBossMechanicRuntime),
		buffStackAlerts:        make(map[uint32]nativeBuffStackRuntime),
		effectTimers:           make(map[string]nativeEffectTimerRuntime),
		announcedSkillSounds:   make(map[string]struct{}),
		recentBossMechanicAtMs: make(map[string]int64),
		playSound:              queueNativeReminderSound,
	}
	runtime.preferredBossID = runtime.settings.PreferredBossID
	setNativeReminderOverlaysLocked(nativeReminderSettingsLocked(runtime.settings))
	nativeReminderRuntimeHolder.Lock()
	nativeReminderRuntimeHolder.runtime = runtime
	nativeReminderRuntimeHolder.Unlock()
	publisher.addClient(ctx, runtime.eventsCh)
	go runtime.loop()
	return runtime
}

func (runtime *nativeReminderRuntime) loop() {
	ticker := time.NewTicker(nativeReminderTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-runtime.ctx.Done():
			return
		case settings := <-runtime.settingsCh:
			runtime.settingsMu.Lock()
			runtime.settings = normalizeNativeReminderSettings(settings)
			runtime.settingsMu.Unlock()
			runtime.preferredBossID = runtime.settings.PreferredBossID
			setNativeReminderOverlaysLocked(nativeReminderSettingsLocked(runtime.settings))
			runtime.resetSettingsDependentState()
			runtime.evaluate(time.Now())
		case position := <-runtime.positionCh:
			runtime.settingsMu.Lock()
			changed := applyNativeReminderPosition(&runtime.settings, position)
			settings := runtime.settings
			runtime.settingsMu.Unlock()
			if !changed {
				markNativeReminderDragCommitted(position.Kind, position.ID)
				continue
			}
			if err := saveNativeReminderSettings(settings); err != nil {
				logger.Println("save dragged reminder position failed:", err)
			}
			runtime.lastSkillStateKey = ""
			runtime.evaluate(time.Now())
			markNativeReminderDragCommitted(position.Kind, position.ID)
		case events := <-runtime.eventsCh:
			for _, current := range events {
				runtime.onEvent(current)
			}
			runtime.evaluate(time.Now())
		case now := <-ticker.C:
			runtime.evaluate(now)
		}
	}
}

func (runtime *nativeReminderRuntime) onEvent(current event.IEvent) {
	switch value := current.(type) {
	case *event.EventLocalEntity:
		if value.Reset {
			for _, entity := range runtime.entities {
				entity.Active = false
				entity.Conditions = make(map[uint32]nativeReminderCondition)
				entity.RefreshGuard = make(map[uint32]int64)
			}
			runtime.activeBossID = ""
			runtime.selectedTargetID = ""
			runtime.finalShotActive = false
			runtime.magnumAim.Active = false
			runtime.resetSettingsDependentState()
		}
		runtime.localID = value.Id
	case *event.EventCombatTarget:
		if value.Id == runtime.localID {
			runtime.selectedTargetID = value.TargetId
		}
	case *event.EventEntityAppear:
		entity := runtime.ensureEntity(value.Id)
		entity.ID = value.Id
		entity.Name = value.Name
		entity.OwnerID = value.OwnerId
		entity.RaceID = value.RaceId
		entity.Known = true
		entity.Active = true
		entity.AppearedAt = value.At
	case *event.EventEntityDisappear:
		entity := runtime.ensureEntity(value.Id)
		entity.Active = false
		if value.Id == runtime.selectedTargetID {
			runtime.selectedTargetID = ""
		}
		if value.Id != runtime.localID {
			entity.Conditions = make(map[uint32]nativeReminderCondition)
			entity.RefreshGuard = make(map[uint32]int64)
		}
	case *event.EventFinish:
		runtime.ensureEntity(value.Id).Active = false
		if value.Id == runtime.selectedTargetID {
			runtime.selectedTargetID = ""
		}
	case *event.EventDamage:
		entity := runtime.ensureEntity(value.TargetId)
		entity.TotalDamage += float64(value.Damage)
		atMs := value.AtMs
		if atMs <= 0 {
			atMs = value.At * 1000
		}
		if atMs > entity.LastDamageAtMs {
			entity.LastDamageAtMs = atMs
		}
		runtime.observeSkillDamage(value, atMs)
		runtime.observeBossMechanicContact(value, atMs)
	case *event.EventStatUpdate:
		entity := runtime.ensureEntity(value.Id)
		for _, stat := range value.Stats {
			switch stat.StatId {
			case 28:
				entity.CurrentHealth = stat.Value
			case 30:
				entity.MaximumHealth = stat.Value
			case 198:
				if value.Id == runtime.localID {
					runtime.observeToahProgress(stat.Value, nativeEventAtMs(value.At, 0))
				}
			}
		}
	case *event.EventSkillAction:
		runtime.observeEffectTimerSkill(value)
		if value.IsLocal {
			runtime.observeSkillAction(value)
		} else {
			runtime.observeBossMechanic(value)
		}
	case *event.EventSkillCooldown:
		runtime.observeSkillCooldownAdjustment(value)
	case *event.EventSkillState:
		runtime.observeSkillState(value)
	case *event.EventCharacterConditionEnable:
		entity := runtime.ensureEntity(value.Id)
		if _, exists := entity.Conditions[value.CCId]; exists {
			entity.RefreshGuard[value.CCId] = value.At
		} else {
			delete(entity.RefreshGuard, value.CCId)
		}
		entity.Conditions[value.CCId] = nativeReminderCondition{
			CCID: value.CCId, At: value.At, DisableAt: value.DisableAt,
			DisableAtMs: value.DisableAtMs, DurationMs: value.DurationMs,
			Metadata: value.Metadata,
		}
		runtime.observeEffectTimerCondition(value.Id, value.CCId, nativeEventAtMs(value.At, 0), true)
	case *event.EventCharacterConditionDisable:
		entity := runtime.ensureEntity(value.Id)
		active, exists := entity.Conditions[value.CCId]
		if !exists {
			return
		}
		if refreshAt, guarded := entity.RefreshGuard[value.CCId]; guarded &&
			refreshAt == active.At && value.At >= refreshAt && value.At-refreshAt <= 1 {
			delete(entity.RefreshGuard, value.CCId)
			return
		}
		delete(entity.RefreshGuard, value.CCId)
		delete(entity.Conditions, value.CCId)
		runtime.observeEffectTimerCondition(value.Id, value.CCId, nativeEventAtMs(value.At, 0), false)
	}
}

func (runtime *nativeReminderRuntime) observeEffectTimerSkill(action *event.EventSkillAction) {
	if action == nil {
		return
	}
	targetMode := "self"
	if !action.IsLocal {
		targetMode = "monster"
	}
	for key, rule := range runtime.settings.EffectTimers.Rules {
		if rule.Enabled && rule.SourceType == "skill" && rule.SourceID == uint32(action.SkillId) && rule.TargetMode == targetMode {
			runtime.startEffectTimer(key, rule, nativeEventAtMs(action.At, action.AtMs), action.SourceId)
		}
	}
}

func (runtime *nativeReminderRuntime) observeEffectTimerCondition(entityID string, ccID uint32, atMs int64, enabled bool) {
	entity := runtime.entities[entityID]
	targetMode := "monster"
	if entityID == runtime.localID || (entity != nil && entity.OwnerID == runtime.localID) {
		targetMode = "self"
	}
	for key, rule := range runtime.settings.EffectTimers.Rules {
		if !rule.Enabled || rule.SourceType != "condition" || rule.SourceID != ccID || rule.TargetMode != targetMode {
			continue
		}
		if enabled {
			runtime.startEffectTimer(key, rule, atMs, entityID)
		} else if state, exists := runtime.effectTimers[key]; exists && state.TargetID == entityID {
			delete(runtime.effectTimers, key)
		}
	}
}

func (runtime *nativeReminderRuntime) startEffectTimer(key string, rule nativeEffectTimerRule, atMs int64, targetID string) {
	if atMs <= 0 {
		atMs = time.Now().UnixMilli()
	}
	if runtime.effectTimers == nil {
		runtime.effectTimers = make(map[string]nativeEffectTimerRuntime)
	}
	previous := runtime.effectTimers[key]
	runtime.effectTimers[key] = nativeEffectTimerRuntime{
		StartedAtMs: atMs,
		EndsAtMs:    atMs + int64(math.Max(100, rule.DurationSeconds*1000)),
		Generation:  previous.Generation + 1,
		TargetID:    targetID,
	}
}

func (runtime *nativeReminderRuntime) observeSkillState(state *event.EventSkillState) {
	if state == nil || state.Id == "" || state.Id != runtime.localID {
		return
	}
	if state.Scope == "active" && state.SkillId == finalShotSkillID {
		runtime.finalShotActive = state.Active
		return
	}
	if state.Scope != "aim" || state.SkillId != magnumShotSkillID {
		return
	}
	if !state.Active {
		runtime.magnumAim.Active = false
		return
	}
	settings := runtime.settings.SkillCooldowns.AimReminder
	if !settings.Enabled {
		return
	}
	atMs := nativeEventAtMs(state.At, state.AtMs)
	latikaActive := runtime.localConditionActiveAt(latikaSecretCCID, atMs)
	rapidActive := runtime.localConditionActiveAt(rapidAimCCID, atMs)
	temporaryMultiplier, temporaryBuffNames := nativeMagnumAimSpeed(runtime.finalShotActive, latikaActive, rapidActive)
	weaponRange := clampNativeReminderFloat(settings.WeaponRange, 100, 10000, nativeMagnumDefaultWeaponRange)
	rangeIdentificationLevel := clampNativeReminderInt(settings.RangeIdentificationLevel, 0, 20, 0)
	calibrationPercent := clampNativeReminderFloat(settings.CalibrationPercent, 20, 40, 40)
	ergSpeedPercent := clampNativeReminderFloat(settings.ErgSpeedPercent, 100, 1000, 200)
	fineTuneSeconds := clampNativeReminderFloat(settings.FineTuneSeconds, -10, 10, 0)
	calibration := calibrationPercent / 100
	ergMultiplier := ergSpeedPercent / 100
	effectiveRange := weaponRange + float64(rangeIdentificationLevel)*nativeMagnumRangePerIdentificationLevel
	rangeBasedFullAimSeconds := nativeMagnumAimRangeDistanceFactor*nativeMagnumAimReferenceDistance/effectiveRange + nativeMagnumAimRangeOffsetSeconds
	bestSystemSpan := nativeMagnumBestSystemAimRate - calibration
	calculatedBestSeconds := rangeBasedFullAimSeconds * math.Pow(bestSystemSpan, 2) / ergMultiplier
	adjustedBestSeconds := math.Max(0.01, calculatedBestSeconds+fineTuneSeconds)
	durationMs := int64(math.Round(adjustedBestSeconds*1000/temporaryMultiplier + 1e-9))
	if durationMs < 1 {
		durationMs = 1
	}
	buffNames := append([]string{"弓尔格 " + strconv.Itoa(int(math.Round(ergSpeedPercent))) + "%"}, temporaryBuffNames...)
	runtime.magnumAim = nativeMagnumAimRuntime{
		Active: true, TargetID: state.TargetId, StartedAtMs: atMs,
		ReadyAtMs: atMs + durationMs, SpeedMultiplier: ergMultiplier * temporaryMultiplier,
		BuffNames: buffNames, CalibrationPercent: calibrationPercent,
		Generation: runtime.magnumAim.Generation + 1,
	}
}

func (runtime *nativeReminderRuntime) localConditionActiveAt(ccID uint32, atMs int64) bool {
	entity := runtime.entities[runtime.localID]
	if entity == nil {
		return false
	}
	condition, exists := entity.Conditions[ccID]
	if !exists {
		return false
	}
	expiresAtMs := condition.DisableAtMs
	if expiresAtMs <= 0 && condition.DisableAt > 0 {
		expiresAtMs = condition.DisableAt * 1000
	}
	return expiresAtMs <= 0 || expiresAtMs > atMs
}

func nativeMagnumAimDisplayProgress(startedAtMs, bestAtMs, nowMs int64, calibrationPercent float64) float64 {
	if bestAtMs <= startedAtMs {
		return 0
	}
	calibration := clampNativeReminderFloat(calibrationPercent, 20, 40, 40) / 100
	bestSpan := math.Max(0.0001, nativeMagnumBestSystemAimRate-calibration)
	elapsedRatio := math.Max(0, float64(nowMs-startedAtMs)) / math.Max(1, float64(bestAtMs-startedAtMs))
	systemAim := math.Min(1, calibration+math.Sqrt(elapsedRatio)*bestSpan)
	if systemAim < nativeMagnumBestSystemAimRate {
		return systemAim
	}
	return math.Min(1, systemAim+(1-systemAim)/2)
}

// Rapid is the fallback. Final Shot wins over Rapid, Latika wins over Rapid,
// and Final Shot + Latika multiply with each other.
func nativeMagnumAimSpeed(finalShot, latika, rapid bool) (float64, []string) {
	if finalShot && latika {
		return 9.6, []string{"无影箭", "拉蒂卡秘术"}
	}
	if latika {
		return 4, []string{"拉蒂卡秘术"}
	}
	if finalShot {
		return 2.4, []string{"无影箭"}
	}
	if rapid {
		return 2, []string{"疾速"}
	}
	return 1, nil
}

func (runtime *nativeReminderRuntime) ensureEntity(id string) *nativeReminderEntity {
	if entity := runtime.entities[id]; entity != nil {
		return entity
	}
	entity := &nativeReminderEntity{
		ID: id, Conditions: make(map[uint32]nativeReminderCondition),
		RefreshGuard: make(map[uint32]int64),
	}
	runtime.entities[id] = entity
	return entity
}

func (runtime *nativeReminderRuntime) evaluate(now time.Time) {
	runtime.evaluateBuffs(now)
	runtime.evaluateDebuffs(now)
	runtime.evaluateSkillCooldowns(now)
	runtime.evaluateBossMechanics(now)
	runtime.publishNativeSkillState(now)
}

func (runtime *nativeReminderRuntime) observeSkillAction(action *event.EventSkillAction) {
	if action == nil || action.SkillId == 27012 {
		return
	}
	runtime.observeSkillCooldown(action.SkillId, nativeEventAtMs(action.At, action.AtMs), action.IsFallback, runtime.nativeSkillSourceIsPet(action.SkillId, action.SourceId))
}

func (runtime *nativeReminderRuntime) observeSkillCooldownAdjustment(adjustment *event.EventSkillCooldown) {
	if adjustment == nil || (runtime.localID != "" && adjustment.Id != "" && adjustment.Id != runtime.localID) {
		return
	}
	rule, configured := runtime.settings.SkillCooldowns.Rules[adjustment.SkillId]
	if !configured || !rule.Enabled {
		return
	}
	previous, observed := runtime.skillCooldowns[adjustment.SkillId]
	if !observed {
		return
	}
	atMs := nativeEventAtMs(adjustment.At, adjustment.AtMs)
	if previous.CooldownPhase == "accumulating" {
		if previous.AccumulatedReadyAtMs <= atMs {
			return
		}
		nextReadyAtMs := atMs
		if !adjustment.Reset {
			if adjustment.ReduceMs == 0 {
				return
			}
			nextReadyAtMs = previous.AccumulatedReadyAtMs - int64(adjustment.ReduceMs)
			if nextReadyAtMs < atMs {
				nextReadyAtMs = atMs
			}
		}
		if nextReadyAtMs <= atMs {
			previous.ReadyAtMs = 0
			previous.ShortReadyAtMs = 0
			previous.AccumulatedReadyAtMs = 0
			previous.AccumulatedCooldownSeconds = 0
			previous.CooldownPhase = "idle"
		} else {
			previous.AccumulatedReadyAtMs = nextReadyAtMs
			previous.AccumulatedCooldownSeconds = math.Ceil(math.Max(0, float64(nextReadyAtMs-atMs)/1000)*10) / 10
		}
		runtime.skillCooldowns[adjustment.SkillId] = previous
		return
	}
	if previous.ReadyAtMs <= atMs {
		return
	}
	readyAtMs := atMs
	if !adjustment.Reset {
		if adjustment.ReduceMs == 0 {
			return
		}
		readyAtMs = previous.ReadyAtMs - int64(adjustment.ReduceMs)
		if readyAtMs < atMs {
			readyAtMs = atMs
		}
	}
	previous.ReadyAtMs = readyAtMs
	if readyAtMs <= atMs {
		previous.ShortReadyAtMs = 0
		previous.AccumulatedReadyAtMs = 0
		previous.AccumulatedCooldownSeconds = 0
		previous.CooldownPhase = "idle"
	}
	runtime.skillCooldowns[adjustment.SkillId] = previous
}

func (runtime *nativeReminderRuntime) observeSkillDamage(damage *event.EventDamage, atMs int64) {
	if damage == nil || runtime.localID == "" {
		return
	}
	source := runtime.entities[damage.Id]
	ownedMarionette := source != nil && source.OwnerID == runtime.localID && nativeMarionetteCooldownSkill(damage.SkillId)
	if damage.Id != runtime.localID && !ownedMarionette {
		return
	}
	runtime.observeSkillCooldown(damage.SkillId, atMs, true, false)
}

func (runtime *nativeReminderRuntime) observeSkillCooldown(skillID uint16, usedAtMs int64, fallback bool, petSkills ...bool) {
	rule, exists := runtime.settings.SkillCooldowns.Rules[skillID]
	if !exists || !rule.Enabled || skillID == 27012 {
		return
	}
	if usedAtMs <= 0 {
		usedAtMs = time.Now().UnixMilli()
	}
	previous, observed := runtime.skillCooldowns[skillID]
	duplicateWindowMs := int64(1500)
	if nativeCumulativeCooldownSkill(skillID) {
		duplicateWindowMs = 120
	}
	if observed && nativeAbsInt64(previous.UsedAtMs-usedAtMs) <= duplicateWindowMs {
		return
	}
	if fallback && observed && previous.ReadyAtMs > usedAtMs {
		return
	}
	petSkill := len(petSkills) > 0 && petSkills[0]
	if nativeCumulativeCooldownSkill(skillID) {
		if previous.CooldownPhase == "full" && previous.ReadyAtMs > usedAtMs {
			return
		}
		currentAccumulated := 0.0
		if previous.CooldownPhase == "accumulating" && previous.AccumulatedReadyAtMs > usedAtMs {
			currentAccumulated = math.Max(0, float64(previous.AccumulatedReadyAtMs-usedAtMs)/1000)
		}
		// Accumulated CD is one decaying time pool. Every accepted use adds one
		// complete short-CD amount to the time still left in that pool: 3s on
		// the first use; after 1s, 2+3=5s; after another 1s, 4+3=7s.
		accumulated := math.Min(rule.CumulativeCooldownSeconds, math.Round((currentAccumulated+rule.ShortCooldownSeconds)*10)/10)
		fullCooldown := accumulated >= rule.CumulativeCooldownSeconds
		readyAtMs := usedAtMs
		accumulatedReadyAtMs := int64(0)
		if accumulated > 0 {
			accumulatedReadyAtMs = usedAtMs + int64(math.Round(accumulated*1000))
		}
		phase := "accumulating"
		if fullCooldown {
			readyAtMs = usedAtMs + int64(math.Max(100, rule.CooldownSeconds*1000))
			accumulatedReadyAtMs = 0
			phase = "full"
		}
		runtime.skillCooldowns[skillID] = nativeSkillCooldownRuntime{
			UsedAtMs: usedAtMs, ReadyAtMs: readyAtMs,
			ShortReadyAtMs:             0,
			AccumulatedReadyAtMs:       accumulatedReadyAtMs,
			AccumulatedCooldownSeconds: accumulated,
			CooldownPhase:              phase, Generation: previous.Generation + 1, PetSkill: petSkill,
		}
		return
	}
	runtime.skillCooldowns[skillID] = nativeSkillCooldownRuntime{
		UsedAtMs: usedAtMs, ReadyAtMs: usedAtMs + int64(math.Max(100, rule.CooldownSeconds*1000)),
		CooldownPhase: "full", Generation: previous.Generation + 1, PetSkill: petSkill,
	}
}

func (runtime *nativeReminderRuntime) nativeSkillSourceIsPet(skillID uint16, sourceID string) bool {
	rule := runtime.settings.SkillCooldowns.Rules[skillID]
	switch rule.OwnerMode {
	case "pet":
		return true
	case "player":
		return false
	}
	if sourceID == "" || sourceID == runtime.localID || nativeMarionetteCooldownSkill(skillID) {
		return false
	}
	source := runtime.entities[sourceID]
	return source != nil && source.OwnerID == runtime.localID
}

func (runtime *nativeReminderRuntime) evaluateSkillCooldowns(now time.Time) {
	nowMs := now.UnixMilli()
	for skillID, cooldown := range runtime.skillCooldowns {
		if cooldown.CooldownPhase == "accumulating" {
			resetAtMs := cooldown.ShortReadyAtMs
			if cooldown.AccumulatedCooldownSeconds > 0 && cooldown.AccumulatedReadyAtMs > 0 {
				resetAtMs = cooldown.AccumulatedReadyAtMs
			}
			if resetAtMs > 0 && nowMs >= resetAtMs {
				cooldown.ReadyAtMs = 0
				cooldown.ShortReadyAtMs = 0
				cooldown.AccumulatedReadyAtMs = 0
				cooldown.AccumulatedCooldownSeconds = 0
				cooldown.CooldownPhase = "idle"
				runtime.skillCooldowns[skillID] = cooldown
			} else if cooldown.AccumulatedCooldownSeconds > 0 && cooldown.AccumulatedReadyAtMs > nowMs {
				cooldown.AccumulatedCooldownSeconds = math.Ceil(
					math.Max(0, float64(cooldown.AccumulatedReadyAtMs-nowMs)/1000)*10,
				) / 10
				runtime.skillCooldowns[skillID] = cooldown
			}
			continue
		}
		if cooldown.Generation == 0 || cooldown.ReadyAtMs <= 0 || nowMs < cooldown.ReadyAtMs {
			continue
		}
		key := strconv.FormatUint(uint64(skillID), 10) + ":" + strconv.FormatUint(cooldown.Generation, 10)
		if _, announced := runtime.announcedSkillSounds[key]; announced {
			continue
		}
		runtime.announcedSkillSounds[key] = struct{}{}
		rule, exists := runtime.settings.SkillCooldowns.Rules[skillID]
		if !exists || !rule.Enabled || rule.SoundMode == "none" || nowMs-cooldown.ReadyAtMs > 1400 {
			continue
		}
		runtime.playNativeSkillSound(rule)
	}
	trimNativeStringSet(runtime.announcedSkillSounds, 256)
}

func (runtime *nativeReminderRuntime) observeToahProgress(progress float64, atMs int64) {
	progress = math.Min(100, math.Max(0, progress))
	rule, exists := runtime.settings.SkillCooldowns.Rules[27012]
	threshold := 95.0
	if exists {
		threshold = rule.ProgressThresholdPercent
	}
	if runtime.toahProgressObserved && runtime.toahProgress-progress > 5 {
		runtime.toahThresholdReached = false
	}
	if progress < 100 {
		runtime.toahFullChargeReached = false
	}
	crossedThreshold := progress >= threshold && !runtime.toahThresholdReached
	crossedFullCharge := progress >= 100 && !runtime.toahFullChargeReached
	runtime.toahProgressObserved = true
	runtime.toahProgress = progress
	if crossedThreshold {
		runtime.toahThresholdReached = true
		runtime.toahGeneration++
		if exists && rule.Enabled && rule.SoundMode != "none" {
			runtime.playNativeSkillSound(rule)
		}
	}
	if crossedFullCharge {
		runtime.toahFullChargeReached = true
		runtime.refreshSkillCooldownsFromToah(atMs)
	}
}

func (runtime *nativeReminderRuntime) refreshSkillCooldownsFromToah(atMs int64) {
	for skillID, cooldown := range runtime.skillCooldowns {
		petSkill := cooldown.PetSkill
		switch runtime.settings.SkillCooldowns.Rules[skillID].OwnerMode {
		case "pet":
			petSkill = true
		case "player":
			petSkill = false
		}
		if !nativeRefreshableFromToah(skillID, petSkill) || cooldown.ReadyAtMs <= atMs {
			continue
		}
		cooldown.ReadyAtMs = atMs
		cooldown.ShortReadyAtMs = 0
		cooldown.AccumulatedReadyAtMs = 0
		cooldown.AccumulatedCooldownSeconds = 0
		cooldown.CooldownPhase = "idle"
		runtime.skillCooldowns[skillID] = cooldown
		key := strconv.FormatUint(uint64(skillID), 10) + ":" + strconv.FormatUint(cooldown.Generation, 10)
		runtime.announcedSkillSounds[key] = struct{}{}
	}
}

func (runtime *nativeReminderRuntime) playNativeSkillSound(rule nativeSkillCooldownRule) {
	if rule.SoundMode == "custom" {
		if rule.CustomSoundID != "" {
			runtime.playSound(nativeReminderSoundRequest{Kind: "custom", Volume: runtime.settings.SkillCooldowns.Volume, SoundID: rule.CustomSoundID})
		}
		return
	}
	if rule.SoundMode == "default" {
		runtime.playSound(nativeReminderSoundRequest{Kind: "skill-ready", Volume: runtime.settings.SkillCooldowns.Volume})
	}
}

func (runtime *nativeReminderRuntime) observeBossMechanic(action *event.EventSkillAction) {
	if action == nil || action.MechanicSignal == "miel-orb-late-confirm" {
		return
	}
	if runtime.bossMechanics == nil {
		runtime.bossMechanics = make(map[string]nativeBossMechanicRuntime)
	}
	atMs := nativeEventAtMs(action.At, action.AtMs)
	for key, rule := range runtime.settings.BossMechanics.Rules {
		if !rule.Enabled || rule.SkillID != action.SkillId {
			continue
		}
		if rule.Trigger == "orb-spawn" && !action.IsFallback {
			continue
		}
		if rule.Trigger == "skill-action" && action.IsFallback {
			continue
		}
		boss, matched := runtime.bossMechanicSourceBoss(action, rule)
		if !matched {
			continue
		}
		clusterWindowMs := int64(250)
		if rule.Trigger == "orb-spawn" {
			clusterWindowMs = 1200
		}
		if previous := runtime.recentBossMechanicAtMs[key]; previous > 0 && atMs-previous < clusterWindowMs {
			continue
		}
		runtime.recentBossMechanicAtMs[key] = atMs
		previous := runtime.bossMechanics[key]
		countdownSeconds := rule.CountdownSeconds
		if countdownSeconds <= 0 {
			if key == "miel-orb" {
				countdownSeconds = 20
			} else {
				countdownSeconds = 5
			}
		}
		state := nativeBossMechanicRuntime{
			Key: key, StartedAtMs: atMs, EndsAtMs: atMs + int64(countdownSeconds*1000),
			Generation: previous.Generation + 1,
		}
		if boss != nil {
			state.BossID = boss.ID
			state.BossRaceID = boss.RaceID
		}
		if key == "miel-orb" {
			state.RequiredContacts = 10
			state.CheckpointAtMs = atMs + 11_000
			if state.BossRaceID == 7603 {
				state.RequiredContacts = 8
				state.CheckpointAtMs = atMs + 8_000
			}
		}
		runtime.bossMechanics[key] = state
		if rule.SoundMode == "custom" {
			if rule.CustomSoundID != "" {
				runtime.playSound(nativeReminderSoundRequest{Kind: "custom", Volume: runtime.settings.BossMechanics.Volume, SoundID: rule.CustomSoundID, Priority: true})
			}
		} else if rule.SoundMode == "dedicated" {
			runtime.playSound(nativeReminderSoundRequest{Kind: "boss-" + key, Volume: runtime.settings.BossMechanics.Volume, Priority: true})
		}
	}
}

func (runtime *nativeReminderRuntime) bossMechanicSourceBoss(action *event.EventSkillAction, rule nativeBossMechanicRule) (*nativeReminderEntity, bool) {
	sourceID := action.SourceId
	if sourceID == "" {
		sourceID = action.Id
	}
	source := runtime.entities[sourceID]
	if source != nil {
		if nativeUint32Contains(rule.BossRaceIDs, source.RaceID) {
			return source, true
		}
		if nativeUint32Contains(rule.TriggerRaceIDs, source.RaceID) {
			owner := runtime.entities[source.OwnerID]
			if owner != nil && !nativeUint32Contains(rule.BossRaceIDs, owner.RaceID) {
				return nil, false
			}
			if owner != nil {
				return owner, true
			}
			return runtime.fallbackBossForRaces(rule.BossRaceIDs), true
		}
		if owner := runtime.entities[source.OwnerID]; owner != nil && nativeUint32Contains(rule.BossRaceIDs, owner.RaceID) {
			return owner, true
		}
		return nil, false
	}
	if boss := runtime.fallbackBossForRaces(rule.BossRaceIDs); boss != nil {
		return boss, true
	}
	return nil, false
}

func (runtime *nativeReminderRuntime) fallbackBossForRaces(raceIDs []uint32) *nativeReminderEntity {
	for _, candidateID := range []string{runtime.preferredBossID, runtime.activeBossID} {
		if candidate := runtime.entities[candidateID]; candidate != nil && candidate.Active && nativeUint32Contains(raceIDs, candidate.RaceID) {
			return candidate
		}
	}
	return nil
}

func (runtime *nativeReminderRuntime) observeBossMechanicContact(damage *event.EventDamage, atMs int64) {
	if damage == nil || damage.SkillId != 52407 || damage.Damage >= 10_000 {
		return
	}
	state, active := runtime.bossMechanics["miel-orb"]
	if !active || atMs < state.StartedAtMs || atMs > state.EndsAtMs {
		return
	}
	if state.BossID != "" && damage.Id != state.BossID {
		return
	}
	if state.BossID == "" {
		source := runtime.entities[damage.Id]
		if source == nil || source.RaceID != state.BossRaceID {
			return
		}
	}
	if state.LastContactAtMs > 0 && atMs-state.LastContactAtMs < 120 {
		return
	}
	state.LastContactAtMs = atMs
	state.ContactCount++
	if atMs <= state.CheckpointAtMs {
		state.CheckpointContactCount++
	}
	if state.CheckpointEvaluated && state.ContactCount >= state.RequiredContacts {
		state.ProvisionallyHidden = true
	}
	runtime.bossMechanics["miel-orb"] = state
}

func (runtime *nativeReminderRuntime) evaluateBossMechanics(now time.Time) {
	nowMs := now.UnixMilli()
	for key, state := range runtime.bossMechanics {
		if state.EndsAtMs <= nowMs {
			delete(runtime.bossMechanics, key)
			continue
		}
		if key == "miel-orb" && !state.CheckpointEvaluated && nowMs >= state.CheckpointAtMs {
			state.CheckpointEvaluated = true
			state.ProvisionallyHidden = state.CheckpointContactCount >= state.RequiredContacts
			runtime.bossMechanics[key] = state
		}
	}
}

func nativeEventAtMs(atSeconds, atMs int64) int64 {
	if atMs > 0 {
		return atMs
	}
	if atSeconds > 0 {
		return atSeconds * 1000
	}
	return time.Now().UnixMilli()
}

func nativeAbsInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func nativeMarionetteCooldownSkill(skillID uint16) bool {
	return (skillID >= 54101 && skillID <= 54106) ||
		(skillID >= 54151 && skillID <= 54156) ||
		(skillID >= 59167 && skillID <= 59169)
}

func nativeCumulativeCooldownSkill(skillID uint16) bool {
	return skillID == 59104 || skillID == 59145
}

func nativeRefreshableFromToah(skillID uint16, petSkill bool) bool {
	return !petSkill && skillID != 27012 && (skillID < 58000 || skillID > 58018)
}

func nativeUint32Contains(values []uint32, wanted uint32) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func trimNativeStringSet(values map[string]struct{}, maximum int) {
	for len(values) > maximum {
		for key := range values {
			delete(values, key)
			break
		}
	}
}

func (runtime *nativeReminderRuntime) evaluateBuffs(now time.Time) {
	nowMs := now.UnixMilli()
	items := make([]nativeBuffOverlayItem, 0, len(runtime.settings.Buff.Rules))
	activeRuleIDs := make(map[uint32]bool, len(runtime.settings.Buff.Rules))
	rules := sortedNativeBuffRules(runtime.settings.Buff.Rules)
	for _, rule := range rules {
		condition, active := runtime.localBuffCondition(rule.CCID)
		if active {
			activeRuleIDs[rule.CCID] = true
			expiresAtMs := nativeBuffExpiresAtMs(condition, rule, runtime.settings.Buff.TimeAdjustmentSeconds)
			if rule.SoundMode != "none" && expiresAtMs > nowMs {
				remainingMs := expiresAtMs - nowMs
				if remainingMs <= int64(rule.SoundThresholdSeconds)*1000 {
					key := nativeBuffSoundKey(rule, condition, expiresAtMs)
					if _, announced := runtime.announcedBuffSounds[key]; !announced {
						runtime.announcedBuffSounds[key] = struct{}{}
						// Entity snapshots are replayed after some map transitions.
						// The packet timestamp changes, but the server expiry only
						// drifts by a few milliseconds. Treat that as the same Buff.
						if previousExpiry := runtime.lastBuffSoundExpiryMs[rule.CCID]; previousExpiry > 0 &&
							nativeAbsInt64(previousExpiry-expiresAtMs) <= int64(nativeBuffSoundExpiryDedupWindow/time.Millisecond) {
							continue
						}
						if rule.SoundMode != "custom" || rule.CustomSoundID != "" {
							if runtime.playSound(nativeReminderSoundRequest{Kind: rule.SoundMode, Volume: runtime.settings.Buff.Volume, SoundID: rule.CustomSoundID}) {
								runtime.lastBuffSoundExpiryMs[rule.CCID] = expiresAtMs
							}
						}
					}
				}
			}
			runtime.evaluateBuffStack(rule, condition, nowMs)
			if rule.OverlayEnabled {
				items = append(items, nativeBuffOverlayItemFromCondition(rule, condition, expiresAtMs))
			}
		} else {
			delete(runtime.buffStackAbove, rule.CCID)
			delete(runtime.buffStackMissingSince, rule.CCID)
			if rule.OverlayEnabled {
				items = append(items, nativeBuffOverlayItem{CCID: rule.CCID, Name: rule.Name, IconURL: rule.IconURL, FlashEnabled: rule.FlashEnabled, FlashThresholdSeconds: rule.FlashThresholdSeconds, Active: false})
			}
		}
	}
	for key := range runtime.announcedBuffSounds {
		parts := strings.SplitN(key, ":", 2)
		ccID, _ := strconv.ParseUint(parts[0], 10, 32)
		if !activeRuleIDs[uint32(ccID)] {
			delete(runtime.announcedBuffSounds, key)
		}
	}
	runtime.publishNativeBuffState(now, items)
}

func (runtime *nativeReminderRuntime) evaluateBuffStack(rule nativeBuffRule, condition nativeReminderCondition, nowMs int64) {
	if !rule.StackAlertEnabled {
		delete(runtime.buffStackAbove, rule.CCID)
		delete(runtime.buffStackMissingSince, rule.CCID)
		return
	}
	stack, ok := parseNativeConditionStack(condition.Metadata)
	if !ok {
		missingSince := runtime.buffStackMissingSince[rule.CCID]
		if missingSince == 0 {
			runtime.buffStackMissingSince[rule.CCID] = nowMs
		} else if nowMs-missingSince >= 1500 {
			runtime.buffStackAbove[rule.CCID] = false
		}
		return
	}
	delete(runtime.buffStackMissingSince, rule.CCID)
	above := stack >= rule.StackThreshold
	wasAbove := runtime.buffStackAbove[rule.CCID]
	runtime.buffStackAbove[rule.CCID] = above
	if !above || wasAbove {
		return
	}
	if rule.StackScreenEnabled {
		if runtime.buffStackAlerts == nil {
			runtime.buffStackAlerts = make(map[uint32]nativeBuffStackRuntime)
		}
		previous := runtime.buffStackAlerts[rule.CCID]
		runtime.buffStackAlerts[rule.CCID] = nativeBuffStackRuntime{
			CCID: rule.CCID, Name: rule.Name, Stack: stack,
			StartedAtMs: nowMs, EndsAtMs: nowMs + 3000, Generation: previous.Generation + 1,
		}
	}
	if rule.StackSoundMode != "none" && (rule.StackSoundMode != "custom" || rule.StackCustomSoundID != "") {
		runtime.playSound(nativeReminderSoundRequest{Kind: rule.StackSoundMode, Volume: runtime.settings.Buff.Volume, SoundID: rule.StackCustomSoundID})
	}
}

func (runtime *nativeReminderRuntime) localBuffCondition(ccID uint32) (nativeReminderCondition, bool) {
	if runtime.localID == "" {
		return nativeReminderCondition{}, false
	}
	entity := runtime.entities[runtime.localID]
	if entity == nil || (entity.Known && !nativeReminderPCRace(entity.RaceID)) {
		return nativeReminderCondition{}, false
	}
	condition, ok := entity.Conditions[ccID]
	return condition, ok
}

func (runtime *nativeReminderRuntime) evaluateDebuffs(now time.Time) {
	if !runtime.settings.Debuff.OverlayEnabled {
		runtime.activeBossID = ""
		runtime.observedDebuffs = make(map[string]bool)
		runtime.lastDebuffAppliedAt = make(map[string]int64)
		runtime.debuffMissingSinceMs = make(map[string]int64)
		runtime.announcedDebuffSounds = make(map[string]struct{})
		runtime.publishNativeDebuffState(now, nil, nil)
		return
	}
	boss := runtime.selectDebuffBoss()
	if boss == nil {
		runtime.activeBossID = ""
		runtime.observedDebuffs = make(map[string]bool)
		runtime.lastDebuffAppliedAt = make(map[string]int64)
		runtime.debuffMissingSinceMs = make(map[string]int64)
		runtime.announcedDebuffSounds = make(map[string]struct{})
		runtime.publishNativeDebuffState(now, nil, nil)
		return
	}
	if boss.ID != runtime.activeBossID {
		runtime.activeBossID = boss.ID
		runtime.observedDebuffs = make(map[string]bool)
		runtime.lastDebuffAppliedAt = make(map[string]int64)
		runtime.debuffMissingSinceMs = make(map[string]int64)
		runtime.announcedDebuffSounds = make(map[string]struct{})
	}
	rules := sortedNativeDebuffRules(runtime.settings.Debuff.Rules)
	items := make([]nativeDebuffOverlayItem, 0, len(rules))
	candidates := make([]nativeDebuffSoundCandidate, 0, len(rules))
	currentKeys := make(map[string]struct{})
	nowMs := now.UnixMilli()
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		canonical := canonicalNativeDebuffID(rule.CCID)
		observationKey := boss.ID + ":" + strconv.FormatUint(uint64(canonical), 10)
		condition, active := runtime.findBossDebuffCondition(boss, rule.CCID)
		if active {
			runtime.observedDebuffs[observationKey] = true
			runtime.lastDebuffAppliedAt[observationKey] = condition.At
			delete(runtime.debuffMissingSinceMs, observationKey)
		}
		expiresAtMs := nativeDebuffExpiresAtMs(condition, active, nowMs)
		state := "hidden"
		if !active {
			// Equivalent CN conditions such as 912/913 are replaced using
			// separate remove/add packets. Wait briefly before declaring an
			// already-observed Debuff missing so the replacement can arrive.
			if runtime.observedDebuffs[observationKey] {
				missingSince := runtime.debuffMissingSinceMs[observationKey]
				if missingSince == 0 {
					runtime.debuffMissingSinceMs[observationKey] = nowMs
					continue
				}
				if nowMs-missingSince < int64(nativeDebuffMissingDebounce/time.Millisecond) {
					continue
				}
			}
			state = "missing"
		} else if rule.FlashEnabled && expiresAtMs > nowMs && expiresAtMs-nowMs <= int64(rule.WarningSeconds)*1000 {
			state = "expiring"
		}
		if state == "hidden" {
			continue
		}
		appliedAt := runtime.lastDebuffAppliedAt[observationKey]
		if appliedAt == 0 {
			appliedAt = boss.AppearedAt
		}
		item := nativeDebuffOverlayItem{CCID: rule.CCID, Name: rule.Name, IconURL: rule.IconURL, AppliedAt: appliedAt, WarningSeconds: rule.WarningSeconds, State: state}
		if expiresAtMs > 0 {
			value := float64(expiresAtMs) / 1000
			item.ExpiresAt = &value
		}
		items = append(items, item)
		key := boss.ID + ":" + strconv.FormatUint(uint64(rule.CCID), 10) + ":" + state + ":" + strconv.FormatInt(appliedAt, 10)
		currentKeys[key] = struct{}{}
		if _, announced := runtime.announcedDebuffSounds[key]; announced {
			continue
		}
		runtime.announcedDebuffSounds[key] = struct{}{}
		if rule.SoundEnabled && rule.SoundMode != "none" && (rule.SoundMode != "custom" || rule.CustomSoundID != "") && (state == "expiring" || runtime.observedDebuffs[observationKey]) {
			candidates = append(candidates, nativeDebuffSoundCandidate{Rule: rule})
		}
	}
	for key := range runtime.announcedDebuffSounds {
		if _, current := currentKeys[key]; !current {
			delete(runtime.announcedDebuffSounds, key)
		}
	}
	if len(candidates) > 0 && !now.Before(runtime.debuffSoundBlockedUntil) {
		selected := candidates[0].Rule
		if runtime.playSound(nativeReminderSoundRequest{Kind: selected.SoundMode, Volume: runtime.settings.Debuff.Volume, SoundID: selected.CustomSoundID}) {
			runtime.debuffSoundBlockedUntil = now.Add(nativeDebuffSoundCooldown)
		}
	}
	runtime.publishNativeDebuffState(now, boss, items)
}

type nativeDebuffSoundCandidate struct {
	Rule nativeDebuffRule
}

func (runtime *nativeReminderRuntime) selectDebuffBoss() *nativeReminderEntity {
	forced := make(map[uint32]bool, len(runtime.settings.Debuff.ForcedBossRaceIDs))
	for _, raceID := range runtime.settings.Debuff.ForcedBossRaceIDs {
		forced[raceID] = true
	}
	eligible := func(entity *nativeReminderEntity) bool {
		if entity == nil || !entity.Known || !entity.Active || nativeReminderPCRace(entity.RaceID) {
			return false
		}
		health := entity.MaximumHealth
		if health <= 0 {
			health = entity.TotalDamage
		}
		return health >= nativeBossMinimumHealth || forced[entity.RaceID]
	}
	if preferred := runtime.entities[runtime.preferredBossID]; eligible(preferred) {
		return preferred
	}
	// After the selected target disappears, the fallback is automatic rather
	// than a lasting lock. A later explicit UI selection sends a new preference.
	if runtime.preferredBossID != "" {
		runtime.preferredBossID = ""
	}
	if current := runtime.entities[runtime.activeBossID]; eligible(current) {
		return current
	}
	var best *nativeReminderEntity
	for _, entity := range runtime.entities {
		if !eligible(entity) {
			continue
		}
		if best == nil || nativeBossLess(best, entity) {
			best = entity
		}
	}
	return best
}

func nativeBossLess(left, right *nativeReminderEntity) bool {
	leftHealth, rightHealth := left.MaximumHealth, right.MaximumHealth
	if leftHealth <= 0 {
		leftHealth = left.TotalDamage
	}
	if rightHealth <= 0 {
		rightHealth = right.TotalDamage
	}
	if leftHealth != rightHealth {
		return leftHealth < rightHealth
	}
	return left.LastDamageAtMs < right.LastDamageAtMs
}

func (runtime *nativeReminderRuntime) findBossDebuffCondition(boss *nativeReminderEntity, ccID uint32) (nativeReminderCondition, bool) {
	accepted := equivalentNativeDebuffIDs(ccID)
	latest := nativeReminderCondition{}
	found := false
	consider := func(entity *nativeReminderEntity) {
		if entity == nil || (entity.Known && nativeReminderPCRace(entity.RaceID)) {
			return
		}
		for _, acceptedID := range accepted {
			condition, exists := entity.Conditions[acceptedID]
			if exists && (!found || condition.At > latest.At || (condition.At == latest.At && condition.DisableAtMs > latest.DisableAtMs)) {
				latest, found = condition, true
			}
		}
	}
	consider(boss)
	// A condition reported directly on the selected Boss is authoritative.
	// Boss-owned mechanic entities can carry the same CC ID with their own
	// short lifetime (for example Miel's orbs carry 1165/1166 for ~22.5s).
	// Letting a newer helper snapshot replace the Boss condition produces a
	// false expiry warning while the real Boss Debuff still has minutes left.
	if found {
		return latest, true
	}
	groupIDs := make(map[string]bool)
	for _, entity := range runtime.entities {
		if entity.Known && entity.RaceID == boss.RaceID {
			groupIDs[entity.ID] = true
			consider(entity)
		}
	}
	for _, entity := range runtime.entities {
		if entity.OwnerID == boss.ID || groupIDs[entity.OwnerID] {
			consider(entity)
		}
	}
	return latest, found
}

func (runtime *nativeReminderRuntime) publishNativeBuffState(now time.Time, items []nativeBuffOverlayItem) {
	settings := nativeBuffOverlayMessageSettings{
		Locked: nativeReminderSettingsLocked(runtime.settings), IconSize: runtime.settings.Buff.IconSize, DPIPercent: runtime.settings.Buff.DPIPercent,
		OverlayEnabled: runtime.settings.Buff.OverlayEnabled, Opacity: runtime.settings.Buff.Opacity,
	}
	keyData, _ := json.Marshal(struct {
		Items    []nativeBuffOverlayItem          `json:"items"`
		Settings nativeBuffOverlayMessageSettings `json:"settings"`
	}{items, settings})
	key := string(keyData)
	if key == runtime.lastBuffStateKey {
		return
	}
	runtime.lastBuffStateKey = key
	data, _ := json.Marshal(nativeBuffOverlayMessage{Type: "buff-state", At: float64(now.UnixMilli()) / 1000, Items: items, Settings: settings})
	setBuffOverlayState(data)
}

func (runtime *nativeReminderRuntime) publishNativeDebuffState(now time.Time, boss *nativeReminderEntity, items []nativeDebuffOverlayItem) {
	settings := nativeDebuffOverlayMessageSettings{
		IconSize: runtime.settings.Debuff.IconSize, OverlayEnabled: runtime.settings.Debuff.OverlayEnabled,
		DPIPercent: runtime.settings.Buff.DPIPercent, Opacity: runtime.settings.Buff.Opacity,
	}
	var overlayBoss *nativeDebuffOverlayBoss
	if boss != nil && settings.OverlayEnabled {
		overlayBoss = &nativeDebuffOverlayBoss{EntityID: boss.ID, Name: runtime.nativeBossDisplayName(boss), AppearedAt: boss.AppearedAt}
	}
	keyData, _ := json.Marshal(struct {
		Boss         *nativeDebuffOverlayBoss           `json:"boss"`
		TargetHealth *nativeTargetHealthOverlayItem     `json:"targetHealth"`
		Items        []nativeDebuffOverlayItem          `json:"items"`
		Settings     nativeDebuffOverlayMessageSettings `json:"settings"`
	}{overlayBoss, nil, items, settings})
	key := string(keyData)
	if key == runtime.lastDebuffStateKey {
		return
	}
	runtime.lastDebuffStateKey = key
	data, _ := json.Marshal(nativeDebuffOverlayMessage{Type: "debuff-state", At: float64(now.UnixMilli()) / 1000, Boss: overlayBoss, TargetHealth: nil, Items: items, Settings: settings})
	setDebuffOverlayState(data)
}

func (runtime *nativeReminderRuntime) selectedTargetHealth() *nativeTargetHealthOverlayItem {
	target := runtime.entities[runtime.selectedTargetID]
	phase := runtime.mielShardHealthPhase(target)
	if target == nil || !target.Known || !target.Active || phase == "" {
		return nil
	}
	maximum := target.MaximumHealth
	current := target.CurrentHealth
	if maximum <= 0 || math.IsNaN(maximum) || math.IsInf(maximum, 0) || math.IsNaN(current) || math.IsInf(current, 0) {
		return nil
	}
	current = math.Max(0, math.Min(maximum, current))
	return &nativeTargetHealthOverlayItem{
		EntityID:       target.ID,
		Name:           runtime.nativeTargetDisplayName(target),
		CurrentHealth:  current,
		MaximumHealth:  maximum,
		PhaseLabel:     nativeMielShardPhaseLabel(phase),
		X:              runtime.settings.BossMechanics.MielShardHealthBarX,
		Y:              runtime.settings.BossMechanics.MielShardHealthBarY,
		ScalePercent:   runtime.settings.BossMechanics.MielShardHealthBarScale,
		OpacityPercent: runtime.settings.BossMechanics.MielShardHealthBarOpacity,
	}
}

func (runtime *nativeReminderRuntime) mielShardHealthPhase(target *nativeReminderEntity) string {
	if target == nil || target.MaximumHealth <= 0 || target.CurrentHealth <= 0 {
		return ""
	}
	boss := runtime.entities[target.OwnerID]
	if boss == nil || !boss.Known || !boss.Active ||
		boss.MaximumHealth <= 0 || boss.CurrentHealth <= 0 {
		return ""
	}
	ratio := boss.CurrentHealth / boss.MaximumHealth
	phases := runtime.settings.BossMechanics.MielShardHealthPhases
	if phases == nil {
		return ""
	}
	if boss.RaceID == 7603 && target.RaceID >= 7604 && target.RaceID <= 7606 {
		if ratio <= 0.400001 && phases.Normal40 {
			return "normal40"
		}
		if ratio > 0.400001 && ratio <= 0.600001 && phases.Normal60 {
			return "normal60"
		}
		if ratio > 0.600001 && ratio <= 0.800001 && phases.Normal80 {
			return "normal80"
		}
	}
	if boss.RaceID == 7615 && target.RaceID >= 7616 && target.RaceID <= 7619 && ratio <= 0.800001 && phases.Regret80 {
		return "regret80"
	}
	return ""
}

func nativeMielShardPhaseLabel(phase string) string {
	switch phase {
	case "normal80":
		return "普通 80%"
	case "normal60":
		return "普通 60%"
	case "normal40":
		return "普通 40%"
	case "regret80":
		return "悔恨 80%"
	}
	return ""
}

func (runtime *nativeReminderRuntime) nativeTargetDisplayName(target *nativeReminderEntity) string {
	if target == nil {
		return ""
	}
	switch target.RaceID {
	case 7604, 7605, 7606, 7607, 7608, 7616, 7617, 7618, 7619, 7620:
		return "安乐碎片"
	}
	if target.RaceID == 7600 || target.RaceID == 7601 || target.RaceID == 7602 || target.RaceID == 7603 || target.RaceID == 7615 {
		return runtime.nativeBossDisplayName(target)
	}
	if name := strings.TrimSpace(runtime.settings.BossRaceNames[target.RaceID]); name != "" {
		return name
	}
	name := strings.TrimSpace(target.Name)
	if name != "" && name != target.ID && !strings.HasPrefix(strings.ToLower(name), "unknown:") {
		if _, err := strconv.ParseUint(name, 10, 64); err != nil {
			return name
		}
	}
	if target.RaceID > 0 {
		return "目标 " + strconv.FormatUint(uint64(target.RaceID), 10)
	}
	return "选中目标"
}

func (runtime *nativeReminderRuntime) nativeBossDisplayName(boss *nativeReminderEntity) string {
	if boss == nil {
		return ""
	}
	switch boss.RaceID {
	case 7600, 7601:
		return "枯木的佩塔克"
	case 7602:
		return "布隆塔纳斯"
	case 7603:
		return "雷内恩的米耶尔"
	case 7615:
		return "雷内恩的米耶尔：悔恨"
	}
	if name := strings.TrimSpace(runtime.settings.BossRaceNames[boss.RaceID]); name != "" {
		return name
	}
	if boss.ID == runtime.settings.PreferredBossID {
		if name := strings.TrimSpace(runtime.settings.PreferredBossName); name != "" {
			return name
		}
	}
	name := strings.TrimSpace(boss.Name)
	if name != "" && name != boss.ID && !strings.HasPrefix(strings.ToLower(name), "unknown:") {
		if _, err := strconv.ParseUint(name, 10, 64); err != nil {
			return name
		}
	}
	if boss.RaceID > 0 {
		return "首领 " + strconv.FormatUint(uint64(boss.RaceID), 10)
	}
	return "首领"
}

func (runtime *nativeReminderRuntime) publishNativeSkillState(now time.Time) {
	nowMs := now.UnixMilli()
	items := make([]nativeSkillOverlayItem, 0, len(runtime.settings.SkillCooldowns.Rules))
	for skillID, rule := range runtime.settings.SkillCooldowns.Rules {
		if !rule.Enabled {
			continue
		}
		cooldown := runtime.skillCooldowns[skillID]
		name := strings.TrimSpace(rule.Name)
		if name == "" {
			name = "技能 " + strconv.FormatUint(uint64(skillID), 10)
		}
		petSkill := cooldown.PetSkill
		if rule.OwnerMode == "pet" {
			petSkill = true
		} else if rule.OwnerMode == "player" {
			petSkill = false
		}
		item := nativeSkillOverlayItem{
			SkillID: skillID, Name: name, IconURL: rule.IconURL,
			AlwaysVisible: rule.AlwaysVisible, BarOnly: rule.BarOnly, X: rule.X, Y: rule.Y,
			UsedAtMs: cooldown.UsedAtMs, ReadyAtMs: cooldown.ReadyAtMs,
			ShortReadyAtMs:             cooldown.ShortReadyAtMs,
			AccumulatedReadyAtMs:       cooldown.AccumulatedReadyAtMs,
			AccumulatedCooldownSeconds: cooldown.AccumulatedCooldownSeconds,
			CooldownPhase:              cooldown.CooldownPhase,
			Generation:                 cooldown.Generation, PetSkill: petSkill,
		}
		if nativeCumulativeCooldownSkill(skillID) {
			limit := rule.CumulativeCooldownSeconds
			item.CumulativeCooldownSeconds = &limit
		}
		if skillID == 27012 {
			progress := runtime.toahProgress
			threshold := rule.ProgressThresholdPercent
			observed := runtime.toahProgressObserved
			item.ProgressPercent = &progress
			item.ProgressThresholdPercent = &threshold
			item.ProgressObserved = &observed
			item.Generation = runtime.toahGeneration
		}
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].SkillID < items[j].SkillID })

	var aimReminder *nativeAimReminderOverlayItem
	aimSettings := runtime.settings.SkillCooldowns.AimReminder
	if aimSettings.Enabled && (runtime.magnumAim.Active || aimSettings.AlwaysVisible) {
		aimReminder = &nativeAimReminderOverlayItem{
			Active: runtime.magnumAim.Active, AlwaysVisible: aimSettings.AlwaysVisible,
			X: aimSettings.X, Y: aimSettings.Y, IconURL: aimSettings.IconURL,
			ScalePercent: aimSettings.ScalePercent, Generation: runtime.magnumAim.Generation,
		}
		if runtime.magnumAim.Active {
			aimReminder.StartedAtMs = runtime.magnumAim.StartedAtMs
			aimReminder.ReadyAtMs = runtime.magnumAim.ReadyAtMs
			aimReminder.SpeedMultiplier = runtime.magnumAim.SpeedMultiplier
			aimReminder.BuffNames = append([]string(nil), runtime.magnumAim.BuffNames...)
			aimReminder.CalibrationPercent = runtime.magnumAim.CalibrationPercent
			aimReminder.TargetID = runtime.magnumAim.TargetID
		} else {
			aimReminder.SpeedMultiplier = 1
			aimReminder.BuffNames = []string{}
			aimReminder.CalibrationPercent = aimSettings.CalibrationPercent
		}
	}
	targetHealth := runtime.selectedTargetHealth()
	effectTimers := make([]nativeEffectTimerOverlayItem, 0, len(runtime.settings.EffectTimers.Rules))
	for key, rule := range runtime.settings.EffectTimers.Rules {
		if !rule.Enabled {
			continue
		}
		state, active := runtime.effectTimers[key]
		if active && state.EndsAtMs <= nowMs {
			delete(runtime.effectTimers, key)
			active = false
		}
		if !active && !rule.AlwaysVisible {
			continue
		}
		effectTimers = append(effectTimers, nativeEffectTimerOverlayItem{
			Key: key, Enabled: rule.Enabled, SourceType: rule.SourceType, SourceID: rule.SourceID,
			Name: rule.Name, IconURL: rule.IconURL, DurationSeconds: rule.DurationSeconds,
			AlwaysVisible: rule.AlwaysVisible, TargetMode: rule.TargetMode, Orientation: rule.Orientation,
			ScalePercent: rule.ScalePercent, OpacityPercent: rule.OpacityPercent, X: rule.X, Y: rule.Y,
			StartedAtMs: state.StartedAtMs, EndsAtMs: state.EndsAtMs, Generation: state.Generation, TargetID: state.TargetID,
		})
	}
	sort.Slice(effectTimers, func(i, j int) bool { return effectTimers[i].Key < effectTimers[j].Key })

	mechanics := make([]nativeBossMechanicOverlayItem, 0, len(runtime.bossMechanics))
	for key, state := range runtime.bossMechanics {
		rule, exists := runtime.settings.BossMechanics.Rules[key]
		if !exists || !rule.Enabled || !rule.ShowCountdown || state.EndsAtMs <= nowMs || state.ProvisionallyHidden {
			continue
		}
		icon := "mdi-laser-pointer"
		if key == "miel-orb" {
			icon = "mdi-orbit"
		}
		mechanics = append(mechanics, nativeBossMechanicOverlayItem{
			Key: key, Name: rule.Name, Icon: icon,
			StartedAtMs: state.StartedAtMs, EndsAtMs: state.EndsAtMs, Generation: state.Generation,
			X: rule.X, Y: rule.Y, ScalePercent: runtime.settings.BossMechanics.ScalePercent,
		})
	}
	sort.Slice(mechanics, func(i, j int) bool { return mechanics[i].Key < mechanics[j].Key })

	stackAlerts := make([]nativeBuffStackOverlayItem, 0, len(runtime.buffStackAlerts))
	for ccID, state := range runtime.buffStackAlerts {
		if state.EndsAtMs <= nowMs {
			delete(runtime.buffStackAlerts, ccID)
			continue
		}
		rule := runtime.settings.Buff.Rules[ccID]
		stackAlerts = append(stackAlerts, nativeBuffStackOverlayItem{
			CCID: state.CCID, Name: state.Name, Stack: state.Stack,
			StartedAtMs: state.StartedAtMs, EndsAtMs: state.EndsAtMs, Generation: state.Generation,
			X:            rule.StackX,
			Y:            rule.StackY,
			ScalePercent: runtime.settings.BossMechanics.ScalePercent,
		})
	}
	sort.Slice(stackAlerts, func(i, j int) bool { return stackAlerts[i].CCID < stackAlerts[j].CCID })

	settings := nativeSkillOverlaySettings{
		IconSize: runtime.settings.SkillCooldowns.IconSize, OverlayEnabled: runtime.settings.Buff.OverlayEnabled,
		DPIPercent: runtime.settings.Buff.DPIPercent, Opacity: runtime.settings.Buff.Opacity,
	}
	visible := targetHealth != nil || aimReminder != nil || len(effectTimers) > 0 || len(mechanics) > 0 || len(stackAlerts) > 0
	if !visible {
		for _, item := range items {
			if item.ProgressObserved != nil {
				progress := *item.ProgressPercent
				threshold := *item.ProgressThresholdPercent
				visible = item.AlwaysVisible || (*item.ProgressObserved && progress >= threshold && progress < 100)
			} else {
				visible = item.AlwaysVisible || (item.ReadyAtMs > 0 && nowMs >= item.ReadyAtMs && nowMs-item.ReadyAtMs < 2600)
			}
			if visible {
				break
			}
		}
	}
	keyData, _ := json.Marshal(struct {
		Items        []nativeSkillOverlayItem        `json:"items"`
		AimReminder  *nativeAimReminderOverlayItem   `json:"aimReminder,omitempty"`
		TargetHealth *nativeTargetHealthOverlayItem  `json:"targetHealth,omitempty"`
		EffectTimers []nativeEffectTimerOverlayItem  `json:"effectTimers"`
		Mechanics    []nativeBossMechanicOverlayItem `json:"mechanics"`
		StackAlerts  []nativeBuffStackOverlayItem    `json:"stackAlerts"`
		Settings     nativeSkillOverlaySettings      `json:"settings"`
		Visible      bool                            `json:"visible"`
	}{items, aimReminder, targetHealth, effectTimers, mechanics, stackAlerts, settings, visible})
	key := string(keyData)
	if key == runtime.lastSkillStateKey {
		return
	}
	runtime.lastSkillStateKey = key
	setNativeSkillOverlayActive(visible)
	data, _ := json.Marshal(nativeSkillOverlayMessage{
		Type: "skill-cooldown-state", AtMs: nowMs, Items: items,
		AimReminder: aimReminder, TargetHealth: targetHealth, EffectTimers: effectTimers, Mechanics: mechanics, StackAlerts: stackAlerts, Settings: settings,
	})
	setSkillOverlayState(data)
}

type nativeBuffOverlayItem struct {
	CCID                  uint32   `json:"ccId"`
	Name                  string   `json:"name"`
	IconURL               string   `json:"iconUrl"`
	AppliedAt             int64    `json:"appliedAt"`
	ExpiresAt             *float64 `json:"expiresAt"`
	FlashEnabled          bool     `json:"flashEnabled"`
	FlashThresholdSeconds int      `json:"flashThresholdSeconds"`
	Active                bool     `json:"active"`
}

type nativeBuffOverlayMessageSettings struct {
	Locked         bool `json:"locked"`
	IconSize       int  `json:"iconSize"`
	DPIPercent     int  `json:"dpiPercent"`
	OverlayEnabled bool `json:"overlayEnabled"`
	Opacity        int  `json:"opacity"`
}
type nativeBuffOverlayMessage struct {
	Type     string                           `json:"type"`
	At       float64                          `json:"at"`
	Items    []nativeBuffOverlayItem          `json:"items"`
	Settings nativeBuffOverlayMessageSettings `json:"settings"`
}

type nativeDebuffOverlayItem struct {
	CCID           uint32   `json:"ccId"`
	Name           string   `json:"name"`
	IconURL        string   `json:"iconUrl"`
	AppliedAt      int64    `json:"appliedAt"`
	ExpiresAt      *float64 `json:"expiresAt"`
	WarningSeconds int      `json:"warningSeconds"`
	State          string   `json:"state"`
}
type nativeDebuffOverlayBoss struct {
	EntityID   string `json:"entityId"`
	Name       string `json:"name"`
	AppearedAt int64  `json:"appearedAt"`
}
type nativeTargetHealthOverlayItem struct {
	EntityID       string  `json:"entityId"`
	Name           string  `json:"name"`
	CurrentHealth  float64 `json:"currentHealth"`
	MaximumHealth  float64 `json:"maximumHealth"`
	X              int     `json:"x"`
	Y              int     `json:"y"`
	ScalePercent   int     `json:"scalePercent"`
	OpacityPercent int     `json:"opacityPercent"`
	PhaseLabel     string  `json:"phaseLabel"`
	PreviewEndsAt  int64   `json:"previewExpiresAtMs,omitempty"`
	Generation     uint64  `json:"generation,omitempty"`
}

type nativeEffectTimerOverlayItem struct {
	Key             string  `json:"key"`
	Enabled         bool    `json:"enabled"`
	SourceType      string  `json:"sourceType"`
	SourceID        uint32  `json:"sourceId"`
	Name            string  `json:"name"`
	IconURL         string  `json:"iconUrl"`
	DurationSeconds float64 `json:"durationSeconds"`
	AlwaysVisible   bool    `json:"alwaysVisible"`
	TargetMode      string  `json:"targetMode"`
	Orientation     string  `json:"orientation"`
	ScalePercent    int     `json:"scalePercent"`
	OpacityPercent  int     `json:"opacityPercent"`
	X               int     `json:"x"`
	Y               int     `json:"y"`
	StartedAtMs     int64   `json:"startedAtMs"`
	EndsAtMs        int64   `json:"endsAtMs"`
	Generation      uint64  `json:"generation"`
	TargetID        string  `json:"targetId"`
}
type nativeDebuffOverlayMessageSettings struct {
	IconSize       int  `json:"iconSize"`
	OverlayEnabled bool `json:"overlayEnabled"`
	DPIPercent     int  `json:"dpiPercent"`
	Opacity        int  `json:"opacity"`
}
type nativeDebuffOverlayMessage struct {
	Type         string                             `json:"type"`
	At           float64                            `json:"at"`
	Boss         *nativeDebuffOverlayBoss           `json:"boss"`
	TargetHealth *nativeTargetHealthOverlayItem     `json:"targetHealth"`
	Items        []nativeDebuffOverlayItem          `json:"items"`
	Settings     nativeDebuffOverlayMessageSettings `json:"settings"`
}

type nativeSkillOverlayItem struct {
	SkillID                    uint16   `json:"skillId"`
	Name                       string   `json:"name"`
	IconURL                    string   `json:"iconUrl"`
	AlwaysVisible              bool     `json:"alwaysVisible"`
	BarOnly                    bool     `json:"barOnly,omitempty"`
	X                          int      `json:"x"`
	Y                          int      `json:"y"`
	UsedAtMs                   int64    `json:"usedAtMs"`
	ReadyAtMs                  int64    `json:"readyAtMs"`
	ShortReadyAtMs             int64    `json:"shortReadyAtMs,omitempty"`
	AccumulatedReadyAtMs       int64    `json:"accumulatedReadyAtMs,omitempty"`
	AccumulatedCooldownSeconds float64  `json:"accumulatedCooldownSeconds,omitempty"`
	CooldownPhase              string   `json:"cooldownPhase,omitempty"`
	CumulativeCooldownSeconds  *float64 `json:"cumulativeCooldownSeconds,omitempty"`
	Generation                 uint64   `json:"generation"`
	PetSkill                   bool     `json:"petSkill,omitempty"`
	ProgressPercent            *float64 `json:"progressPercent,omitempty"`
	ProgressThresholdPercent   *float64 `json:"progressThresholdPercent,omitempty"`
	ProgressObserved           *bool    `json:"progressObserved,omitempty"`
}

type nativeAimReminderOverlayItem struct {
	Active             bool     `json:"active"`
	AlwaysVisible      bool     `json:"alwaysVisible"`
	StartedAtMs        int64    `json:"startedAtMs"`
	ReadyAtMs          int64    `json:"readyAtMs"`
	SpeedMultiplier    float64  `json:"speedMultiplier"`
	BuffNames          []string `json:"buffNames"`
	CalibrationPercent float64  `json:"calibrationPercent"`
	TargetID           string   `json:"targetId"`
	X                  int      `json:"x"`
	Y                  int      `json:"y"`
	IconURL            string   `json:"iconUrl"`
	ScalePercent       int      `json:"scalePercent"`
	Generation         uint64   `json:"generation"`
}

type nativeBossMechanicOverlayItem struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	StartedAtMs  int64  `json:"startedAtMs"`
	EndsAtMs     int64  `json:"endsAtMs"`
	Generation   uint64 `json:"generation"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	ScalePercent int    `json:"scalePercent"`
}

type nativeBuffStackOverlayItem struct {
	CCID         uint32 `json:"ccId"`
	Name         string `json:"name"`
	Stack        int    `json:"stack"`
	StartedAtMs  int64  `json:"startedAtMs"`
	EndsAtMs     int64  `json:"endsAtMs"`
	Generation   uint64 `json:"generation"`
	X            int    `json:"x"`
	Y            int    `json:"y"`
	ScalePercent int    `json:"scalePercent"`
}

type nativeSkillOverlaySettings struct {
	IconSize       int  `json:"iconSize"`
	OverlayEnabled bool `json:"overlayEnabled"`
	DPIPercent     int  `json:"dpiPercent"`
	Opacity        int  `json:"opacity"`
}

type nativeSkillOverlayMessage struct {
	Type         string                          `json:"type"`
	AtMs         int64                           `json:"atMs"`
	Items        []nativeSkillOverlayItem        `json:"items"`
	AimReminder  *nativeAimReminderOverlayItem   `json:"aimReminder,omitempty"`
	TargetHealth *nativeTargetHealthOverlayItem  `json:"targetHealth,omitempty"`
	EffectTimers []nativeEffectTimerOverlayItem  `json:"effectTimers"`
	Mechanics    []nativeBossMechanicOverlayItem `json:"mechanics"`
	StackAlerts  []nativeBuffStackOverlayItem    `json:"stackAlerts"`
	Settings     nativeSkillOverlaySettings      `json:"settings"`
}

func nativeBuffOverlayItemFromCondition(rule nativeBuffRule, condition nativeReminderCondition, expiresAtMs int64) nativeBuffOverlayItem {
	item := nativeBuffOverlayItem{CCID: rule.CCID, Name: rule.Name, IconURL: rule.IconURL, AppliedAt: condition.At, FlashEnabled: rule.FlashEnabled, FlashThresholdSeconds: rule.FlashThresholdSeconds, Active: true}
	if expiresAtMs > 0 {
		value := float64(expiresAtMs) / 1000
		item.ExpiresAt = &value
	}
	return item
}

func nativeBuffExpiresAtMs(condition nativeReminderCondition, rule nativeBuffRule, adjustmentSeconds int) int64 {
	adjustmentMs := int64(adjustmentSeconds) * 1000
	if rule.DurationMode == "manual" {
		return condition.At*1000 + int64(rule.ManualDurationSeconds)*1000 + adjustmentMs
	}
	if condition.DisableAtMs > condition.At*1000 && condition.DisableAtMs-condition.At*1000 <= int64(7*24*time.Hour/time.Millisecond) {
		return condition.DisableAtMs + adjustmentMs
	}
	if condition.DisableAt > condition.At && condition.DisableAt-condition.At <= int64(7*24*time.Hour/time.Second) {
		return condition.DisableAt*1000 + adjustmentMs
	}
	if condition.DurationMs >= 100 && condition.DurationMs <= int64(7*24*time.Hour/time.Millisecond) {
		return condition.At*1000 + condition.DurationMs + adjustmentMs
	}
	return 0
}

func nativeDebuffExpiresAtMs(condition nativeReminderCondition, active bool, nowMs int64) int64 {
	if !active {
		return 0
	}
	if condition.DisableAtMs > nowMs && condition.DisableAtMs-nowMs < int64(24*time.Hour/time.Millisecond) {
		return condition.DisableAtMs
	}
	if condition.DisableAt*1000 > nowMs && condition.DisableAt*1000-nowMs < int64(24*time.Hour/time.Millisecond) {
		return condition.DisableAt * 1000
	}
	return 0
}

func nativeBuffSoundKey(rule nativeBuffRule, condition nativeReminderCondition, expiresAtMs int64) string {
	return strconv.FormatUint(uint64(rule.CCID), 10) + ":" + strconv.FormatInt(condition.At, 10) + ":" + strconv.FormatInt(expiresAtMs, 10) + ":" + rule.SoundMode + ":" + rule.CustomSoundID + ":" + strconv.Itoa(rule.SoundThresholdSeconds)
}

func parseNativeConditionStack(metadata string) (int, bool) {
	match := stackCountPattern.FindStringSubmatch(metadata)
	if len(match) != 2 {
		return 0, false
	}
	value, err := strconv.Atoi(match[1])
	if err != nil || value < 0 || value > 999 {
		return 0, false
	}
	return value, true
}

func nativeReminderPCRace(raceID uint32) bool {
	switch raceID {
	case 8001, 8002, 9001, 9002, 10001, 10002:
		return true
	}
	return false
}

func equivalentNativeDebuffIDs(ccID uint32) []uint32 {
	switch ccID {
	case 912, 913:
		return []uint32{912, 913}
	case 392, 504:
		return []uint32{392, 504}
	}
	return []uint32{ccID}
}

func canonicalNativeDebuffID(ccID uint32) uint32 {
	switch ccID {
	case 912, 913:
		return 912
	case 392, 504:
		return 392
	}
	return ccID
}

func sortedNativeBuffRules(rules map[uint32]nativeBuffRule) []nativeBuffRule {
	result := make([]nativeBuffRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, rule)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CCID < result[j].CCID })
	return result
}

func sortedNativeDebuffRules(rules map[uint32]nativeDebuffRule) []nativeDebuffRule {
	result := make([]nativeDebuffRule, 0, len(rules))
	for _, rule := range rules {
		result = append(result, rule)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Order != result[j].Order {
			return result[i].Order < result[j].Order
		}
		return result[i].CCID < result[j].CCID
	})
	return result
}

func normalizeNativeReminderSettings(settings nativeReminderSettings) nativeReminderSettings {
	legacyVisualSettings := settings.Buff.Opacity == 0
	locked := true
	if settings.Buff.Locked != nil {
		locked = *settings.Buff.Locked
	}
	settings.Buff.Locked = &locked
	settings.PreferredBossID = strings.TrimSpace(settings.PreferredBossID)
	settings.PreferredBossName = strings.TrimSpace(settings.PreferredBossName)
	if settings.BossRaceNames == nil {
		settings.BossRaceNames = make(map[uint32]string)
	}
	for raceID, name := range settings.BossRaceNames {
		settings.BossRaceNames[raceID] = strings.TrimSpace(name)
	}
	settings.Buff.Volume = clampNativeReminderInt(settings.Buff.Volume, 0, 100, 80)
	settings.Buff.IconSize = clampNativeReminderInt(settings.Buff.IconSize, 16, 80, 20)
	settings.Buff.DPIPercent = clampNativeReminderInt(settings.Buff.DPIPercent, 0, 500, 0)
	settings.Buff.Opacity = clampNativeReminderInt(settings.Buff.Opacity, 20, 100, 100)
	if legacyVisualSettings {
		settings.Buff.OverlayEnabled = true
	}
	settings.Buff.TimeAdjustmentSeconds = clampNativeReminderInt(settings.Buff.TimeAdjustmentSeconds, -3600, 3600, 0)
	if settings.Buff.Rules == nil {
		settings.Buff.Rules = make(map[uint32]nativeBuffRule)
	}
	for id, rule := range settings.Buff.Rules {
		rule.CCID = id
		rule.DurationMode = nativeDurationMode(rule.DurationMode)
		rule.ManualDurationSeconds = clampNativeReminderInt(rule.ManualDurationSeconds, 1, 86400, 60)
		rule.FlashThresholdSeconds = clampNativeReminderInt(rule.FlashThresholdSeconds, 1, 3600, 10)
		rule.SoundThresholdSeconds = clampNativeReminderInt(rule.SoundThresholdSeconds, 1, 3600, 5)
		rule.StackThreshold = clampNativeReminderInt(rule.StackThreshold, 1, 99, 5)
		rule.StackX = clampNativeReminderInt(rule.StackX, -32000, 32000, 850)
		rule.StackY = clampNativeReminderInt(rule.StackY, -32000, 32000, 280)
		rule.SoundMode = nativeSoundMode(rule.SoundMode)
		rule.StackSoundMode = nativeSoundMode(rule.StackSoundMode)
		settings.Buff.Rules[id] = rule
	}
	settings.Debuff.Volume = clampNativeReminderInt(settings.Debuff.Volume, 0, 100, 80)
	settings.Debuff.IconSize = clampNativeReminderInt(settings.Debuff.IconSize, 16, 80, 30)
	if settings.Debuff.Rules == nil {
		settings.Debuff.Rules = make(map[uint32]nativeDebuffRule)
	}
	for id, rule := range settings.Debuff.Rules {
		rule.CCID = id
		rule.WarningSeconds = clampNativeReminderInt(rule.WarningSeconds, 1, 3600, 20)
		rule.SoundMode = nativeSoundMode(rule.SoundMode)
		settings.Debuff.Rules[id] = rule
	}
	settings.SkillCooldowns.Volume = clampNativeReminderInt(settings.SkillCooldowns.Volume, 0, 100, settings.Buff.Volume)
	settings.SkillCooldowns.IconSize = clampNativeReminderInt(settings.SkillCooldowns.IconSize, 24, 96, 48)
	settings.SkillCooldowns.AimReminder.WeaponRange = clampNativeReminderFloat(settings.SkillCooldowns.AimReminder.WeaponRange, 100, 10000, nativeMagnumDefaultWeaponRange)
	settings.SkillCooldowns.AimReminder.RangeIdentificationLevel = clampNativeReminderInt(settings.SkillCooldowns.AimReminder.RangeIdentificationLevel, 0, 20, 0)
	settings.SkillCooldowns.AimReminder.CalibrationPercent = clampNativeReminderFloat(settings.SkillCooldowns.AimReminder.CalibrationPercent, 20, 40, 40)
	settings.SkillCooldowns.AimReminder.ErgSpeedPercent = clampNativeReminderFloat(settings.SkillCooldowns.AimReminder.ErgSpeedPercent, 100, 1000, 200)
	settings.SkillCooldowns.AimReminder.FineTuneSeconds = clampNativeReminderFloat(settings.SkillCooldowns.AimReminder.FineTuneSeconds, -10, 10, 0)
	settings.SkillCooldowns.AimReminder.ScalePercent = clampNativeReminderInt(settings.SkillCooldowns.AimReminder.ScalePercent, 50, 200, 100)
	settings.SkillCooldowns.AimReminder.X = clampNativeReminderInt(settings.SkillCooldowns.AimReminder.X, -32000, 32000, 600)
	settings.SkillCooldowns.AimReminder.Y = clampNativeReminderInt(settings.SkillCooldowns.AimReminder.Y, -32000, 32000, 180)
	if settings.SkillCooldowns.Rules == nil {
		settings.SkillCooldowns.Rules = make(map[uint16]nativeSkillCooldownRule)
	}
	for id, rule := range settings.SkillCooldowns.Rules {
		rule.SkillID = id
		rule.CooldownSeconds = clampNativeReminderFloat(rule.CooldownSeconds, 0.1, 86400, 30)
		rule.ShortCooldownSeconds = clampNativeReminderFloat(rule.ShortCooldownSeconds, 0.1, 86400, 1)
		rule.CumulativeCooldownSeconds = clampNativeReminderFloat(rule.CumulativeCooldownSeconds, 0.1, 86400, 10)
		rule.ProgressThresholdPercent = clampNativeReminderFloat(rule.ProgressThresholdPercent, 1, 100, 95)
		rule.SoundMode = nativeSkillSoundMode(rule.SoundMode)
		rule.OwnerMode = nativeSkillOwnerMode(rule.OwnerMode)
		rule.X = clampNativeReminderInt(rule.X, -32000, 32000, 600)
		rule.Y = clampNativeReminderInt(rule.Y, -32000, 32000, 180)
		settings.SkillCooldowns.Rules[id] = rule
	}
	settings.BossMechanics.Volume = clampNativeReminderInt(settings.BossMechanics.Volume, 0, 100, 80)
	settings.BossMechanics.X = clampNativeReminderInt(settings.BossMechanics.X, -32000, 32000, 850)
	settings.BossMechanics.Y = clampNativeReminderInt(settings.BossMechanics.Y, -32000, 32000, 280)
	settings.BossMechanics.ScalePercent = clampNativeReminderInt(settings.BossMechanics.ScalePercent, 50, 200, 100)
	settings.BossMechanics.Miel60HealthBarX = clampNativeReminderInt(settings.BossMechanics.Miel60HealthBarX, -32000, 32000, 750)
	settings.BossMechanics.Miel60HealthBarY = clampNativeReminderInt(settings.BossMechanics.Miel60HealthBarY, -32000, 32000, 280)
	settings.BossMechanics.Miel60HealthBarScale = clampNativeReminderInt(settings.BossMechanics.Miel60HealthBarScale, 50, 200, 100)
	miel60HealthBarEnabled := true
	if settings.BossMechanics.Miel60HealthBarEnabled != nil {
		miel60HealthBarEnabled = *settings.BossMechanics.Miel60HealthBarEnabled
	}
	settings.BossMechanics.Miel60HealthBarEnabled = &miel60HealthBarEnabled
	if settings.BossMechanics.MielShardHealthPhases == nil {
		settings.BossMechanics.MielShardHealthPhases = &nativeMielShardHealthPhases{Normal60: miel60HealthBarEnabled}
		settings.BossMechanics.MielShardHealthBarX = settings.BossMechanics.Miel60HealthBarX
		settings.BossMechanics.MielShardHealthBarY = settings.BossMechanics.Miel60HealthBarY
		settings.BossMechanics.MielShardHealthBarScale = settings.BossMechanics.Miel60HealthBarScale
	}
	settings.BossMechanics.MielShardHealthBarX = clampNativeReminderInt(settings.BossMechanics.MielShardHealthBarX, -32000, 32000, 750)
	settings.BossMechanics.MielShardHealthBarY = clampNativeReminderInt(settings.BossMechanics.MielShardHealthBarY, -32000, 32000, 280)
	settings.BossMechanics.MielShardHealthBarScale = clampNativeReminderInt(settings.BossMechanics.MielShardHealthBarScale, 50, 200, 100)
	settings.BossMechanics.MielShardHealthBarOpacity = clampNativeReminderInt(settings.BossMechanics.MielShardHealthBarOpacity, 20, 100, 100)
	if settings.BossMechanics.Rules == nil {
		settings.BossMechanics.Rules = make(map[string]nativeBossMechanicRule)
	}
	for key, rule := range settings.BossMechanics.Rules {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		rule.Key = key
		rule.Name = strings.TrimSpace(rule.Name)
		countdownFallback := 5.0
		if key == "miel-orb" {
			countdownFallback = 20
		}
		rule.CountdownSeconds = clampNativeReminderFloat(rule.CountdownSeconds, 1, 120, countdownFallback)
		rule.X = clampNativeReminderInt(rule.X, -32000, 32000, settings.BossMechanics.X)
		rule.Y = clampNativeReminderInt(rule.Y, -32000, 32000, settings.BossMechanics.Y)
		rule.SoundMode = nativeBossSoundMode(rule.SoundMode)
		if rule.Trigger != "orb-spawn" {
			rule.Trigger = "skill-action"
		}
		settings.BossMechanics.Rules[key] = rule
	}
	if settings.EffectTimers.Rules == nil {
		settings.EffectTimers.Rules = make(map[string]nativeEffectTimerRule)
	}
	for storedKey, rule := range settings.EffectTimers.Rules {
		key := strings.TrimSpace(storedKey)
		if key == "" || rule.SourceID == 0 {
			delete(settings.EffectTimers.Rules, storedKey)
			continue
		}
		if key != storedKey {
			delete(settings.EffectTimers.Rules, storedKey)
		}
		rule.Key = key
		rule.Name = strings.TrimSpace(rule.Name)
		if rule.Name == "" {
			rule.Name = "效果 " + strconv.FormatUint(uint64(rule.SourceID), 10)
		}
		if rule.SourceType != "skill" {
			rule.SourceType = "condition"
		}
		if rule.TargetMode != "monster" {
			rule.TargetMode = "self"
		}
		if rule.Orientation != "vertical" {
			rule.Orientation = "horizontal"
		}
		rule.DurationSeconds = clampNativeReminderFloat(rule.DurationSeconds, 0.1, 86400, 10)
		rule.ScalePercent = clampNativeReminderInt(rule.ScalePercent, 50, 200, 100)
		rule.OpacityPercent = clampNativeReminderInt(rule.OpacityPercent, 20, 100, 100)
		rule.X = clampNativeReminderInt(rule.X, -32000, 32000, 600)
		rule.Y = clampNativeReminderInt(rule.Y, -32000, 32000, 260)
		settings.EffectTimers.Rules[key] = rule
	}
	return settings
}

func nativeReminderSettingsLocked(settings nativeReminderSettings) bool {
	return settings.Buff.Locked == nil || *settings.Buff.Locked
}

func nativeDurationMode(value string) string {
	if value == "manual" {
		return "manual"
	}
	return "auto"
}
func nativeSoundMode(value string) string {
	switch value {
	case "electronic", "voice", "custom":
		return value
	}
	return "none"
}
func nativeSkillSoundMode(value string) string {
	switch value {
	case "default", "custom":
		return value
	}
	return "none"
}
func nativeSkillOwnerMode(value string) string {
	switch value {
	case "player", "pet":
		return value
	}
	return "auto"
}
func nativeBossSoundMode(value string) string {
	switch value {
	case "dedicated", "custom":
		return value
	}
	return "none"
}
func clampNativeReminderInt(value, min, max, fallback int) int {
	if value < min || value > max {
		return fallback
	}
	return value
}
func clampNativeReminderFloat(value, min, max, fallback float64) float64 {
	if math.IsNaN(value) || math.IsInf(value, 0) || value < min || value > max {
		return fallback
	}
	return value
}

func (runtime *nativeReminderRuntime) resetSettingsDependentState() {
	runtime.announcedBuffSounds = make(map[string]struct{})
	runtime.buffStackAbove = make(map[uint32]bool)
	runtime.buffStackMissingSince = make(map[uint32]int64)
	runtime.buffStackAlerts = make(map[uint32]nativeBuffStackRuntime)
	runtime.announcedDebuffSounds = make(map[string]struct{})
	runtime.debuffMissingSinceMs = make(map[string]int64)
	runtime.recentBossMechanicAtMs = make(map[string]int64)
	runtime.bossMechanics = make(map[string]nativeBossMechanicRuntime)
	runtime.effectTimers = make(map[string]nativeEffectTimerRuntime)
	runtime.toahProgressObserved = false
	runtime.toahProgress = 0
	runtime.toahThresholdReached = false
	runtime.toahFullChargeReached = false
	runtime.toahGeneration = 0
	runtime.lastBuffStateKey = ""
	runtime.lastDebuffStateKey = ""
	runtime.lastSkillStateKey = ""
}

func nativeReminderSettingsPath() string { return filepath.Join(appDataDir(), "reminder-runtime.json") }

func loadNativeReminderSettings() nativeReminderSettings {
	data, err := os.ReadFile(nativeReminderSettingsPath())
	if err != nil {
		return normalizeNativeReminderSettings(nativeReminderSettings{})
	}
	var settings nativeReminderSettings
	if json.Unmarshal(data, &settings) != nil {
		return normalizeNativeReminderSettings(nativeReminderSettings{})
	}
	return normalizeNativeReminderSettings(settings)
}

func saveNativeReminderSettings(settings nativeReminderSettings) error {
	data, err := json.MarshalIndent(normalizeNativeReminderSettings(settings), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(nativeReminderSettingsPath(), data, 0644)
}

func handleNativeReminderSettings(w http.ResponseWriter, request *http.Request) {
	nativeReminderRuntimeHolder.RLock()
	runtime := nativeReminderRuntimeHolder.runtime
	nativeReminderRuntimeHolder.RUnlock()
	if runtime == nil {
		http.Error(w, "reminder runtime unavailable", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	switch request.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		runtime.settingsMu.RLock()
		_ = json.NewEncoder(w).Encode(runtime.settings)
		runtime.settingsMu.RUnlock()
	case http.MethodPut:
		var settings nativeReminderSettings
		decoder := json.NewDecoder(io.LimitReader(request.Body, 512*1024))
		if err := decoder.Decode(&settings); err != nil {
			http.Error(w, "invalid reminder settings", http.StatusBadRequest)
			return
		}
		settings = normalizeNativeReminderSettings(settings)
		if err := saveNativeReminderSettings(settings); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		select {
		case runtime.settingsCh <- settings:
		default:
			select {
			case <-runtime.settingsCh:
			default:
			}
			runtime.settingsCh <- settings
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		w.Header().Set("Allow", "GET, PUT")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
