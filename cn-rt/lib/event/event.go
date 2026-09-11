package event

type EventId int16

const (
	EventIdEntityAppear EventId = 1 + iota
	EventIdEntityDisappear
	EventIdDamage
	EventIdCharacterConditionEnable
	EventIdCharacterConditionDisable
	EventIdFinish
	EventIdEntityEquipItem
	EventIdEntityUnequipItem
	EventIdEntityUpdateBody
	EventIdSkillAction   EventId = 10
	EventIdLocalEntity   EventId = 11
	EventIdStatUpdate    EventId = 17
	EventIdSkillState    EventId = 18
	EventIdCombatTarget  EventId = 19
	EventIdSkillCooldown EventId = 20
)

const (
	EventIdMessageBox EventId = -1 - iota
)

type IEvent interface {
	GetEventId() EventId
}

type EventBase struct {
	EventId EventId
	At      int64
	Id      string
}

func (t *EventBase) GetEventId() EventId {
	return t.EventId
}

type EventEntityAppear struct {
	EventBase
	Name      string
	RaceId    uint32
	Height    float32
	Weight    float32
	Upper     float32
	Lower     float32
	GuildName string
	OwnerId   string
}

type EventEntityDisappear struct {
	EventBase
}

type EventDamage struct {
	EventBase
	// AtMs keeps packet precision for mechanics whose repeated hit packets must
	// be grouped. At remains for backwards-compatible NDJSON imports.
	AtMs       int64
	TargetId   string
	SkillId    uint16
	Damage     float32
	IsCritical bool
	IsDelayed  bool
}

type EventCharacterConditionEnable struct {
	EventBase
	CCId        uint32
	DisableAt   int64
	DisableAtMs int64
	AttackerId  string
	Metadata    string
	DurationMs  int64
}

type EventCharacterConditionDisable struct {
	EventBase
	CCId uint32
}

type EventFinish struct {
	EventBase
	AttackerId string
}

type EventEntityEquipItem struct {
	EventBase
	PocketType uint32
	ItemId     uint32
	Color1     string
	Color2     string
	Color3     string
	Color5     string
	Color6     string
	Color7     string
}

type EventEntityUnequipItem struct {
	EventBase
	PocketType uint32
}

type EventEntityUpdateBody struct {
	EventBase
	Height float32
	Weight float32
	Upper  float32
	Lower  float32
}

// EventSkillAction is emitted once for a successful local-player action in a
// CombatActionPack. AtMs retains packet millisecond precision for cooldown
// timers; EventBase.At remains populated for compatibility with existing log
// readers.
type EventSkillAction struct {
	EventBase
	SkillId        uint16
	SubSkillId     uint16
	CombatActionId uint32
	AtMs           int64
	// MechanicSignal identifies a derived monster-mechanic event without
	// borrowing a real in-game SkillId. It is omitted for normal skill actions.
	MechanicSignal string `json:"MechanicSignal,omitempty"`
	// SourceId is the entity that produced the action. It differs from Id when
	// an owned marionette or deployable produced the tracked skill for the local
	// player.
	SourceId string
	// IsFallback marks actions inferred from an owned entity or a damage-phase
	// packet. Consumers must not restart a running cooldown from fallback hits.
	IsFallback bool
	// IsLocal distinguishes player/owned actions from monster mechanics. Older
	// logs omit this field and are treated as local by the frontend.
	IsLocal bool
}

// EventLocalEntity identifies the character controlled by this client.
// Reliable is false while a private-stat packet has only provided an unknown
// entity id; it becomes true after EntityAppear confirms either the player or
// an owned creature's player owner.
type EventLocalEntity struct {
	EventBase
	Reliable bool
	// Reset marks a new game-server connection/channel. Consumers should clear
	// only transient identity and active-condition state while keeping damage
	// history intact.
	Reset bool
}

type EventStatUpdateEntry struct {
	StatId uint32
	Value  float64
}

// EventStatUpdate exposes public entity attributes needed for boss HP and
// progress monitoring. Private marks attributes addressed to this client.
type EventStatUpdate struct {
	EventBase
	Private bool
	Stats   []EventStatUpdateEntry
}

// EventSkillState reports local skill lifecycles that are not successful
// actions/cooldowns. Scope "aim" marks target acquisition for an aimed skill;
// scope "active" marks a sustained skill such as Final Shot being enabled.
type EventSkillState struct {
	EventBase
	AtMs     int64
	SkillId  uint16
	Scope    string
	Active   bool
	TargetId string `json:"TargetId,omitempty"`
}

// EventCombatTarget follows the target explicitly selected by the local
// player. TargetId is empty when the selection is cleared.
type EventCombatTarget struct {
	EventBase
	AtMs     int64
	TargetId string `json:"TargetId,omitempty"`
}

// EventSkillCooldown reports an authoritative server change to one skill's
// current cooldown. Reset is used by opcode 27049; ReduceMs is populated by
// opcode 27069 or a configured SLST astrology critical-hit signal.
type EventSkillCooldown struct {
	EventBase
	AtMs     int64
	SkillId  uint16
	Reset    bool
	ReduceMs uint32
	Signal   string `json:"Signal,omitempty"`
}

type EventMessageBox struct {
	EventBase
	Message string
}
