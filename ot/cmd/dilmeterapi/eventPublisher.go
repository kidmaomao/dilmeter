package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const (
	_MAX_PENDING_EVENTS   = 100
	_EVENT_FLUSH_INTERVAL = 100 * time.Millisecond
)

type eventPublisher struct {
	sync.Mutex

	// non-mutable
	ctx         context.Context
	r           *packet.GameServerPacketReader
	clientMap   map[uint32]*eventClient
	entityCache entityCache

	// mutable
	currentClientId        uint32
	localEntityId          uint64
	localEntityReliable    bool
	recentSkillActionIds   map[uint32]struct{}
	recentSkillActionOrder []uint32
	recentBossSkillAt      map[string]int64
	recentCooldownSignals  map[uint16]uint64
	activeAimSkillID       uint16
	activeAimTargetID      uint64
	combatTargetID         uint64
	finalShotActive        bool
	statCache              map[uint64]map[uint32]float64
	connectionEpoch        uint64
	lastSentEventAt        time.Time
	pendingEvents          []event.IEvent
}

type eventClient struct {
	ctx context.Context
	ch  chan<- []event.IEvent
}

var le = binary.LittleEndian

func newEventPublisher(ctx context.Context, r *packet.GameServerPacketReader) *eventPublisher {
	v := &eventPublisher{
		ctx:         ctx,
		r:           r,
		clientMap:   make(map[uint32]*eventClient),
		entityCache: make(entityCache),

		currentClientId:        1,
		localEntityId:          0,
		localEntityReliable:    false,
		recentSkillActionIds:   make(map[uint32]struct{}, 256),
		recentSkillActionOrder: make([]uint32, 0, 256),
		recentBossSkillAt:      make(map[string]int64, 32),
		recentCooldownSignals:  make(map[uint16]uint64, 8),
		statCache:              make(map[uint64]map[uint32]float64, 64),
		lastSentEventAt:        time.Now(),
		pendingEvents:          make([]event.IEvent, 0, _MAX_PENDING_EVENTS),
	}

	go v.loop()

	return v
}

func (t *eventPublisher) loop() {
	debug := false
	eventFlushTicker := time.NewTicker(_EVENT_FLUSH_INTERVAL / 2)

	for {
		select {
		case <-t.ctx.Done():
			return
		case p := <-t.r.PacketCh():
			if p.ConnectionEpoch != 0 && p.ConnectionEpoch != t.connectionEpoch {
				t.beginConnectionEpoch(p.ConnectionEpoch, p.At)
			}

			if debug {
				logger.Printf("packet op %x id %x", p.Op, p.Id)
				for i, msg := range p.Msg {
					logger.Println("* msg", i, msg.Type(), msg.String())
				}
			}
			// Boss mechanism deployment packets use a small family of shared
			// opcodes. Keep their opcode and payload dispatch in one testable path.
			if t.publishBossLaserPacket(p) {
				continue
			}

			switch p.Op {

			// short packet
			case 0:
				continue

			case packet.OpcodeStatUpdatePrivate, packet.OpcodeStatUpdatePublic:
				isPrivate := p.Op == packet.OpcodeStatUpdatePrivate
				// Private stat packets are addressed to the character controlled
				// by this client, but the game also sends them for owned pets. Use
				// the entity owner relation so a pet packet cannot replace the
				// player id and make subsequent player skills disappear.
				if isPrivate {
					if id, reliable, changed := t.updateLocalEntityId(p.Id); changed {
						t.publish(toEventLocalEntity(p.At.Unix(), id, reliable, false))
					}
				}
				stats, err := packet.ParseStatUpdatePacket(p.Msg)
				if err == nil && len(stats) > 0 {
					values := t.filterChangedStats(p.Id, stats)
					if len(values) == 0 {
						continue
					}
					t.publish(&event.EventStatUpdate{
						EventBase: event.EventBase{EventId: event.EventIdStatUpdate, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)},
						Private:   isPrivate,
						Stats:     values,
					})
				}
				continue

			case packet.OpcodeEntityAppear:
				entity, err := packet.ParseEntityAppearPacket(p.Msg)
				if err != nil {
					logger.Println("ParseEntityAppearPacket failed:", err)
					continue
				}

				if entity == nil {
					continue
				}

				t.Lock()
				t.entityCache.add(entity, p.At)
				t.Unlock()

				// Keep hidden/NPC entities in the cache. Player deployables such as
				// Hydra can use an internal name while still carrying an OwnerId that
				// is needed to associate their skill actions with the local player.
				// They remain excluded from the public actor list.
				if id, reliable, changed := t.reconcileLocalEntityInfo(entity); changed {
					t.publish(toEventLocalEntity(p.At.Unix(), id, reliable, false))
				}
				// Deployable skills such as Hydra can announce their successful
				// creation through the entity's initial condition snapshot without a
				// local-player CombatAction record. Inspect the snapshot before hidden
				// entities are filtered from the public actor list.
				for _, condition := range entity.CharacterConditionMap {
					if condition != nil {
						t.publishConditionMetadataSkillAction(p.At, entity.Id, condition.AttackerId, condition.Metadata)
					}
				}
				if len(entity.Name) <= 0 || entity.Name[0] == '_' {
					continue
				}

				e := toEventEntityAppear(p.At.Unix(), entity)
				t.publish(e)

				el := toEventListEntityContains(p.At.Unix(), entity, &t.entityCache)
				for _, e := range el {
					t.publish(e)
				}

				continue

			case packet.OpcodeEntityDisappear:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					continue
				}

				id := p.Msg[0].Data().(uint64)

				t.Lock()
				t.entityCache.disappear(id, p.At)
				t.Unlock()

				e := &event.EventEntityDisappear{
					EventBase: event.EventBase{
						EventId: event.EventIdEntityDisappear,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(id, 10),
					},
				}
				t.publish(e)

				continue

			case packet.OpcodeCreatureBodyUpdate:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeBin {
					logger.Println("invalid packet")
					continue
				}

				b := p.Msg[0].Data().([]byte)

				height := math.Float32frombits(le.Uint32(b[0:]))
				weight := math.Float32frombits(le.Uint32(b[4:]))
				upper := math.Float32frombits(le.Uint32(b[8:]))
				lower := math.Float32frombits(le.Uint32(b[12:]))

				t.entityCache.updateBody(p.Id, height, weight, upper, lower)

				e := &event.EventEntityUpdateBody{
					EventBase: event.EventBase{
						EventId: event.EventIdEntityUpdateBody,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					Height: height,
					Weight: weight,
					Upper:  upper,
					Lower:  lower,
				}

				t.publish(e)

				continue

			case packet.OpcodeEntitiesAppear:
				entities, err := packet.ParseEntitiesAppearPacket(p)
				if err != nil {
					logger.Println("ParseEntitiesAppearPacket failed:", err)
					continue
				}

				for _, entity := range entities {
					t.Lock()
					t.entityCache.add(entity, p.At)
					t.Unlock()
					if id, reliable, changed := t.reconcileLocalEntityInfo(entity); changed {
						t.publish(toEventLocalEntity(p.At.Unix(), id, reliable, false))
					}
					if len(entity.Name) <= 0 || entity.Name[0] == '_' {
						continue
					}

					e := toEventEntityAppear(p.At.Unix(), entity)
					t.publish(e)

					el := toEventListEntityContains(p.At.Unix(), entity, &t.entityCache)
					for _, e := range el {
						t.publish(e)
					}
				}
				continue

			case packet.OpcodeEntitiesDisappear:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeShort {
					logger.Println("invalid packet")
					continue
				}

				count := int(p.Msg[0].Data().(uint16))
				msg := p.Msg[1:]

				now := p.At.Unix()
				for i := 0; i < count; i++ {
					// ttype, id, unk1 (if ttype == 16)
					if len(msg) < 2 ||
						msg[0].Type() != packet.MessageElemTypeShort ||
						msg[1].Type() != packet.MessageElemTypeLong {

						logger.Println("invalid packet")

						for j, m := range p.Msg {
							logger.Println("* msg", j, m.Type(), m.String())
						}

						break
					}

					ttype := msg[0].Data().(uint16)
					id := msg[1].Data().(uint64)

					t.Lock()
					t.entityCache.disappear(id, p.At)
					t.Unlock()

					e := &event.EventEntityDisappear{
						EventBase: event.EventBase{
							EventId: event.EventIdEntityDisappear,
							At:      now,
							Id:      strconv.FormatUint(id, 10),
						},
					}
					t.publish(e)

					msg = msg[2:]

					if ttype == 16 && len(msg) >= 1 {
						msg = msg[1:]
					}
				}
				continue

			case packet.OpcodeEquipmentChanged:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeBin {
					logger.Println("invalid packet", p.Op)
					continue
				}

				b := p.Msg[0].Data().([]byte)
				info, err := packet.EntityItemReader(b)
				if err != nil {
					logger.Println("EntityItemReader failed:", err)
					continue
				}

				if !t.entityCache.addOrUpdateEquipItem(p.Id, info) {
					continue
				}

				e := &event.EventEntityEquipItem{
					EventBase: event.EventBase{
						EventId: event.EventIdEntityEquipItem,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					PocketType: info.PocketType,
					ItemId:     info.ItemId,
					Color1:     fmt.Sprintf("#%06x", info.Color1),
					Color2:     fmt.Sprintf("#%06x", info.Color2),
					Color3:     fmt.Sprintf("#%06x", info.Color3),
					Color5:     fmt.Sprintf("#%06x", info.Color5),
					Color6:     fmt.Sprintf("#%06x", info.Color6),
					Color7:     fmt.Sprintf("#%06x", info.Color7),
				}

				t.publish(e)

				continue

			case packet.OpcodeUnequipment:
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeInt {
					continue
				}

				pocketType := p.Msg[0].Data().(uint32)

				if !t.entityCache.hasEquipItem(p.Id, pocketType) {
					continue
				}

				t.entityCache.unequipItem(p.Id, pocketType)

				e := &event.EventEntityUnequipItem{
					EventBase: event.EventBase{
						EventId: event.EventIdEntityUnequipItem,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					PocketType: pocketType,
				}

				t.publish(e)

				continue

			case packet.OpcodeSetFinisher:
				// set finisher
				if len(p.Msg) < 1 || p.Msg[0].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					continue
				}

				attackerId := p.Msg[0].Data().(uint64)
				attackerIdStr := ""
				if attackerId != 0 {
					attackerIdStr = strconv.FormatUint(attackerId, 10)
				}

				e := &event.EventFinish{
					EventBase: event.EventBase{
						EventId: event.EventIdFinish,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(p.Id, 10),
					},
					AttackerId: attackerIdStr,
				}
				t.publish(e)

				continue

			case packet.OpcodeSkillExecute:
				if err := t.publishSkillExecutePacket(p); err != nil {
					logger.Println("ParseSkillExecutePacket failed:", err)
				}
				continue

			case packet.OpcodeSkillCooldownReset:
				if err := t.publishSkillCooldownResetPacket(p); err != nil {
					logger.Println("ParseSkillCooldownResetPacket failed:", err)
				}
				continue

			case packet.OpcodeSkillCooldownReduce:
				if err := t.publishSkillCooldownReducePacket(p); err != nil {
					logger.Println("ParseSkillCooldownReducePacket failed:", err)
				}
				continue

			case packet.OpcodeSkillListState:
				if err := t.publishAstrologyCooldownPacket(p); err != nil {
					logger.Println("ParseAstrologyCooldownPacket failed:", err)
				}
				continue

			case packet.OpcodeSkillPrepareReady:
				if p.Id != t.localEntityId {
					continue
				}
				if skillID, ok := parsePreparedSkillID(p.Msg); ok && skillID == finalShotSkillID && !t.finalShotActive {
					t.finalShotActive = true
					t.publishSkillState(p, finalShotSkillID, "active", true, 0)
				}
				continue

			case packet.OpcodeSkillPrepareEnd:
				if p.Id != t.localEntityId {
					continue
				}
				if skillID, ok := parseEndedSkillID(p.Msg); ok && skillID == finalShotSkillID && t.finalShotActive {
					t.finalShotActive = false
					t.publishSkillState(p, finalShotSkillID, "active", false, 0)
				}
				continue

			case packet.OpcodeSkillTarget:
				if p.Id != t.localEntityId {
					continue
				}
				active, targetID, skillID, ok := parseSkillTarget(p.Msg)
				if !ok {
					continue
				}
				if active {
					if skillID != magnumShotSkillID {
						continue
					}
					if t.activeAimSkillID == skillID && t.activeAimTargetID == targetID {
						continue
					}
					t.activeAimSkillID = skillID
					t.activeAimTargetID = targetID
					t.publishSkillState(p, skillID, "aim", true, targetID)
					continue
				}
				if t.activeAimSkillID == magnumShotSkillID {
					t.publishSkillState(p, t.activeAimSkillID, "aim", false, t.activeAimTargetID)
					t.activeAimSkillID = 0
					t.activeAimTargetID = 0
				}
				continue

			case packet.OpcodeSetCombatTarget:
				if p.Id != t.localEntityId {
					continue
				}
				targetID, err := packet.ParseSetCombatTargetPacket(p.Msg)
				if err != nil {
					logger.Println("ParseSetCombatTargetPacket failed:", err)
					continue
				}
				if targetID == t.combatTargetID {
					continue
				}
				t.combatTargetID = targetID
				target := ""
				if targetID != 0 {
					target = strconv.FormatUint(targetID, 10)
				}
				t.publish(&event.EventCombatTarget{
					EventBase: event.EventBase{EventId: event.EventIdCombatTarget, At: p.At.Unix(), Id: strconv.FormatUint(t.localEntityId, 10)},
					AtMs:      p.At.UnixMilli(), TargetId: target,
				})
				continue

			case packet.OpCode(0x6d62):
				// 米耶尔环绕球的机制倒计时使用 280 标记，并以步骤 4/5
				// 在短时间内成对确认。映射为仅供提醒使用的内部动作 52402。
				t.publishBossOrbCountdownSignal(p)
				continue

			case packet.OpCode(0x6d66):
				// This packet is only a late confirmation that an orb is still
				// active. It must never stop the countdown.
				t.publishBossOrbLateConfirmation(p)
				continue

			case packet.OpcodeCombatActionPack:
				pack, err := packet.ParseCombatActionPackPacket(p)
				if err != nil {
					logger.Println("ParseCombatActionPackPacket failed:", err)
					continue
				}

				attackerId := uint64(0)
				attackSkillId := uint16(0)
				var fallbackAttacker *packet.CombatActionPacket

				// find attacker
				for i, v := range pack.SubPackets {
					_ = i

					if debug {
						logger.Println("sub packet", i, v.Hit != nil, v.Attacker != nil)
						logger.Printf("base %+v", v)
						if v.Hit != nil {
							logger.Printf("hit %+v", v.Hit)
						}

						if v.Attacker != nil {
							logger.Printf("attacker %+v", v.Attacker)
						}
					}

					// 한패킷에 공격자가 2명 이상 일 수 있을까?
					if v.Hit == nil {
						if fallbackAttacker == nil {
							fallbackAttacker = v
						}
						if v.Attacker != nil {
							// 공격자
							attackerId = v.EntityId
							attackSkillId = v.SkillId
							break
						}
					}
				}
				if attackerId == 0 && fallbackAttacker != nil {
					attackerId = fallbackAttacker.EntityId
					attackSkillId = fallbackAttacker.SkillId
				}
				// A pack may contain one attacker action followed by several hit
				// sub-packets. Publish exactly one server-confirmed action for the
				// local player. CN player actions commonly use type 0x42 and do not
				// set SkillSuccess, so requiring that bit would silently miss them.
				// Prefer the non-hit attacker record so multi-hit skills cannot start
				// the cooldown timer more than once.
				if t.localEntityId != 0 {
					action, ownedSource := findTrackedSkillAction(pack, t.localEntityId, t.entityCache)
					if action != nil && action.SkillId != 0 {
						actionID := action.CombatActionId
						if actionID == 0 {
							actionID = pack.CombatActionId
						}
						if t.acceptSkillAction(actionID) {
							t.publish(&event.EventSkillAction{
								EventBase: event.EventBase{
									EventId: event.EventIdSkillAction,
									At:      p.At.Unix(),
									Id:      strconv.FormatUint(t.localEntityId, 10),
								},
								SkillId:        action.SkillId,
								SubSkillId:     action.SubSkillId,
								CombatActionId: actionID,
								AtMs:           p.At.UnixMilli(),
								SourceId:       strconv.FormatUint(action.EntityId, 10),
								IsFallback:     isFallbackSkillAction(pack, action, ownedSource),
								IsLocal:        true,
							})
						}
					}
				}

				for _, v := range pack.SubPackets {
					if v.Hit == nil {
						continue
					}

					// 방어자
					targetId := v.EntityId
					damage := v.Hit.Damage
					isCritical := v.Hit.Options&0x1 != 0

					e := &event.EventDamage{
						EventBase: event.EventBase{
							EventId: event.EventIdDamage,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(attackerId, 10),
						},
						AtMs:       p.At.UnixMilli(),
						TargetId:   strconv.FormatUint(targetId, 10),
						SkillId:    attackSkillId,
						Damage:     damage,
						IsCritical: isCritical,
					}
					t.publish(e)
				}

				continue

			case packet.OpcodeEffectDelayed:
				// effect delayed, 연공 블래스트 대미지가 이걸로 날라옴
				targetId := p.Id
				if !isEffectDelayedDamageMessage(p.Msg) {
					continue
				}

				if len(p.Msg) < 2 ||
					p.Msg[0].Type() != packet.MessageElemTypeInt ||
					p.Msg[1].Type() != packet.MessageElemTypeInt {

					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}

					logger.Println("invalid packet")
					continue
				}

				delay := p.Msg[0].Data().(uint32)
				ttype := p.Msg[1].Data().(uint32)
				if !isEffectDelayedDamageType(ttype) {
					// 연공 블래스트가 아님
					continue
				}

				_ = delay

				if len(p.Msg) < 7 {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[2].Type() != packet.MessageElemTypeInt {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[5].Type() != packet.MessageElemTypeLong {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}
				if p.Msg[6].Type() != packet.MessageElemTypeShort {
					logger.Println("invalid packet")
					logger.Printf("packet op %x id %x", p.Op, p.Id)
					for i, msg := range p.Msg {
						logger.Println("* msg", i, msg.Type(), msg.String())
					}
					continue
				}

				damage := p.Msg[2].Data().(uint32)
				attackerId := p.Msg[5].Data().(uint64)
				attackSkillId := p.Msg[6].Data().(uint16)
				e := &event.EventDamage{
					EventBase: event.EventBase{
						EventId: event.EventIdDamage,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(attackerId, 10),
					},
					AtMs:      p.At.UnixMilli(),
					TargetId:  strconv.FormatUint(targetId, 10),
					SkillId:   attackSkillId,
					Damage:    float32(damage),
					IsDelayed: true,
				}
				t.publish(e)

				continue

			case packet.OpcodeConditionUpdate:
				// condition update
				cond, err := packet.ParseCharacterConditionPacket(p)
				if err != nil {
					logger.Println("ParseCharacterConditionPacket failed:", err)
					continue
				}

				t.Lock()
				changed := t.entityCache.addCondition(cond)
				t.Unlock()
				if !changed {
					continue
				}

				if !cond.IsEnable {
					e := &event.EventCharacterConditionDisable{
						EventBase: event.EventBase{
							EventId: event.EventIdCharacterConditionDisable,
							At:      p.At.Unix(),
							Id:      strconv.FormatUint(cond.Id, 10),
						},
						CCId: cond.CCId,
					}
					t.publish(e)
					continue
				}

				// Some condition packets carry an explicit caster skill id. Treat that
				// metadata as a cast signal, but never infer a skill from the CC id:
				// CN Debuff ids have changed and the old Hydra/Smoke mappings are stale.
				t.publishConditionMetadataSkillAction(p.At, cond.Id, cond.AttackerId, cond.Metadata)
				t.publishTechniqueConditionSkillAction(p.At, cond)

				attackerId := ""
				if cond.AttackerId != 0 {
					attackerId = strconv.FormatUint(cond.AttackerId, 10)
				}

				e := &event.EventCharacterConditionEnable{
					EventBase: event.EventBase{
						EventId: event.EventIdCharacterConditionEnable,
						At:      p.At.Unix(),
						Id:      strconv.FormatUint(cond.Id, 10),
					},
					CCId:        cond.CCId,
					DisableAt:   cond.DisableAt,
					DisableAtMs: cond.DisableAtMs,
					AttackerId:  attackerId,
					Metadata:    cond.Metadata,
					DurationMs:  cond.DurationMs,
				}
				t.publish(e)

				continue
			}

		case <-eventFlushTicker.C:
			t.publishNow()

		}
	}
}

func (t *eventPublisher) publishSkillState(p *packet.GamePacket, skillID uint16, scope string, active bool, targetID uint64) {
	if p == nil || t.localEntityId == 0 {
		return
	}
	target := ""
	if targetID != 0 {
		target = strconv.FormatUint(targetID, 10)
	}
	t.publish(&event.EventSkillState{
		EventBase: event.EventBase{
			EventId: event.EventIdSkillState,
			At:      p.At.Unix(),
			Id:      strconv.FormatUint(t.localEntityId, 10),
		},
		AtMs: p.At.UnixMilli(), SkillId: skillID, Scope: scope,
		Active: active, TargetId: target,
	})
}

func (t *eventPublisher) publish(e event.IEvent) {
	// blocking이 되면 안된다

	t.Lock()

	sendNowCond1 := time.Since(t.lastSentEventAt) >= _EVENT_FLUSH_INTERVAL
	sendNowCond2 := len(t.pendingEvents) >= _MAX_PENDING_EVENTS
	sendNow := sendNowCond1 || sendNowCond2

	t.pendingEvents = append(t.pendingEvents, e)
	t.Unlock()

	if !sendNow {
		return
	}

	t.publishNow()
}

func (t *eventPublisher) publishNow() {
	t.Lock()
	defer t.Unlock()

	if len(t.pendingEvents) == 0 {
		return
	}

	events := t.pendingEvents
	t.pendingEvents = make([]event.IEvent, 0, _MAX_PENDING_EVENTS)
	t.lastSentEventAt = time.Now()

	for k, c := range t.clientMap {
		select {
		case <-c.ctx.Done():
			delete(t.clientMap, k)
			continue

		default:
			_ = 1
		}

		select {
		case c.ch <- events:
			// write ok
			_ = 1

		default:
			// queue full
			delete(t.clientMap, k)
			logger.Println("queue full... force close socket", k)
			continue
		}
	}
}

func (t *eventPublisher) addClient(ctx context.Context, ch chan<- []event.IEvent) uint32 {
	t.Lock()
	t.currentClientId++
	clientId := t.currentClientId
	t.Unlock()

	events := []event.IEvent(nil)

	t.Lock()
	// Always send an identity state, including the initial unconfirmed zero
	// value, so a reconnect cannot keep using a previous capture's player id.
	events = append(events, toEventLocalEntity(
		time.Now().Unix(),
		t.localEntityId,
		t.localEntityReliable,
		false,
	))
	for _, entity := range t.entityCache {
		e := toEventEntityAppear(entity.appearAt, entity.EntityInfo)

		events = append(events, e)

		for _, cond := range entity.characterConditionMap {
			attackerId := ""
			if cond.AttackerId != 0 {
				attackerId = strconv.FormatUint(cond.AttackerId, 10)
			}

			e := &event.EventCharacterConditionEnable{
				EventBase: event.EventBase{
					EventId: event.EventIdCharacterConditionEnable,
					At:      entity.appearAt,
					Id:      strconv.FormatUint(entity.Id, 10),
				},
				CCId:        cond.CCId,
				DisableAt:   cond.DisableAt,
				DisableAtMs: cond.DisableAtMs,
				AttackerId:  attackerId,
				Metadata:    cond.Metadata,
				DurationMs:  cond.DurationMs,
			}

			events = append(events, e)
		}

		for _, item := range entity.equipItemMap {
			e := &event.EventEntityEquipItem{
				EventBase: event.EventBase{
					EventId: event.EventIdEntityEquipItem,
					At:      entity.appearAt,
					Id:      strconv.FormatUint(entity.Id, 10),
				},
				PocketType: item.PocketType,
				ItemId:     item.ItemId,
				Color1:     fmt.Sprintf("#%06x", item.Color1),
				Color2:     fmt.Sprintf("#%06x", item.Color2),
				Color3:     fmt.Sprintf("#%06x", item.Color3),
				Color5:     fmt.Sprintf("#%06x", item.Color5),
				Color6:     fmt.Sprintf("#%06x", item.Color6),
				Color7:     fmt.Sprintf("#%06x", item.Color7),
			}

			events = append(events, e)
		}
	}
	t.Unlock()

	logger.Println("send initial data", clientId, ", ", len(events), "events")

	if len(events) > 0 {
		ch <- events
	}

	t.Lock()
	t.clientMap[clientId] = &eventClient{
		ctx: ctx,
		ch:  ch,
	}
	t.Unlock()

	return clientId
}

func toEventEntityAppear(now int64, p *packet.EntityInfo) *event.EventEntityAppear {
	ownerId := ""

	if p.OwnerId != 0 {
		ownerId = strconv.FormatUint(p.OwnerId, 10)
	}

	v := &event.EventEntityAppear{
		EventBase: event.EventBase{
			EventId: event.EventIdEntityAppear,
			At:      now,
			Id:      strconv.FormatUint(p.Id, 10),
		},
		Name:      p.Name,
		RaceId:    p.RaceId,
		Height:    p.Height,
		Weight:    p.Weight,
		Upper:     p.Upper,
		Lower:     p.Lower,
		GuildName: p.GuildName,
		OwnerId:   ownerId,
	}

	return v
}

func toEventLocalEntity(now int64, id uint64, reliable bool, reset bool) *event.EventLocalEntity {
	return &event.EventLocalEntity{
		EventBase: event.EventBase{
			EventId: event.EventIdLocalEntity,
			At:      now,
			Id:      strconv.FormatUint(id, 10),
		},
		Reliable: reliable,
		Reset:    reset,
	}
}

// beginConnectionEpoch separates channel sessions. It deliberately preserves
// published combat history, but discards cached entities, local-player
// identity, active conditions and action de-duplication state so the new
// channel's EntityAppear snapshot becomes authoritative.
func (t *eventPublisher) beginConnectionEpoch(epoch uint64, at time.Time) {
	if epoch == 0 || epoch == t.connectionEpoch {
		return
	}

	t.Lock()
	t.connectionEpoch = epoch
	t.entityCache = make(entityCache)
	t.localEntityId = 0
	t.localEntityReliable = false
	t.activeAimSkillID = 0
	t.activeAimTargetID = 0
	t.combatTargetID = 0
	t.finalShotActive = false
	clear(t.recentSkillActionIds)
	clear(t.recentBossSkillAt)
	clear(t.recentCooldownSignals)
	clear(t.statCache)
	t.recentSkillActionOrder = t.recentSkillActionOrder[:0]
	t.Unlock()

	logger.Printf("capture connection epoch changed to %d; transient entity and Buff state reset", epoch)
	t.publish(toEventLocalEntity(at.Unix(), 0, false, true))
}

func toEventListEntityContains(now int64, p *packet.EntityInfo, entityCache *entityCache) []event.IEvent {
	l := []event.IEvent(nil)

	for _, v := range p.CharacterConditionMap {
		if !entityCache.addOrUpdateCondition(p.Id, v) {
			continue
		}

		attackerId := ""
		if v.AttackerId != 0 {
			attackerId = strconv.FormatUint(v.AttackerId, 10)
		}

		e := &event.EventCharacterConditionEnable{
			EventBase: event.EventBase{
				EventId: event.EventIdCharacterConditionEnable,
				At:      now,
				Id:      strconv.FormatUint(p.Id, 10),
			},
			CCId:        v.CCId,
			DisableAt:   v.DisableAt,
			DisableAtMs: v.DisableAtMs,
			AttackerId:  attackerId,
			Metadata:    v.Metadata,
			DurationMs:  v.DurationMs,
		}

		l = append(l, e)
	}

	for _, v := range p.EquipItemMap {
		if !entityCache.addOrUpdateEquipItem(p.Id, v) {
			continue
		}

		e := &event.EventEntityEquipItem{
			EventBase: event.EventBase{
				EventId: event.EventIdEntityEquipItem,
				At:      now,
				Id:      strconv.FormatUint(p.Id, 10),
			},
			PocketType: v.PocketType,
			ItemId:     v.ItemId,
			Color1:     fmt.Sprintf("#%06x", v.Color1),
			Color2:     fmt.Sprintf("#%06x", v.Color2),
			Color3:     fmt.Sprintf("#%06x", v.Color3),
			Color5:     fmt.Sprintf("#%06x", v.Color5),
			Color6:     fmt.Sprintf("#%06x", v.Color6),
			Color7:     fmt.Sprintf("#%06x", v.Color7),
		}

		l = append(l, e)
	}

	for _, pocketType := range entityCache.allEquipItemPockets(p.Id) {
		if p.EquipItemMap[pocketType] != nil {
			continue
		}

		entityCache.unequipItem(p.Id, pocketType)

		e := &event.EventEntityUnequipItem{
			EventBase: event.EventBase{
				EventId: event.EventIdEntityUnequipItem,
				At:      now,
				Id:      strconv.FormatUint(p.Id, 10),
			},
			PocketType: pocketType,
		}

		l = append(l, e)
	}

	return l
}

// updateLocalEntityId resolves private stat packets from owned pets back to
// their player. Unknown ids are allowed to establish the first player, but do
// not replace an already reliable id until their entity relationship is known.
func (t *eventPublisher) updateLocalEntityId(candidate uint64) (uint64, bool, bool) {
	if candidate == 0 {
		return 0, false, false
	}

	t.Lock()

	resolved := candidate
	reliable := false
	if entity := t.entityCache[candidate]; entity != nil && entity.EntityInfo != nil {
		reliable = true
		resolved = resolveEntityOwner(candidate, t.entityCache)
	}

	if t.localEntityId != 0 && !reliable && candidate != t.localEntityId {
		id, confirmed := t.localEntityId, t.localEntityReliable
		t.Unlock()
		return id, confirmed, false
	}
	if resolved == t.localEntityId && reliable == t.localEntityReliable {
		t.Unlock()
		return resolved, reliable, false
	}

	t.localEntityId = resolved
	t.localEntityReliable = reliable
	clear(t.recentSkillActionIds)
	t.recentSkillActionOrder = t.recentSkillActionOrder[:0]
	t.Unlock()
	logger.Printf("local player entity detected: %d (confirmed=%t)", resolved, reliable)
	return resolved, reliable, true
}

// reconcileLocalEntityInfo fixes the startup edge case where a private stat
// packet for a pet arrives before that pet's EntityAppear packet.
func (t *eventPublisher) reconcileLocalEntityInfo(entity *packet.EntityInfo) (uint64, bool, bool) {
	if entity == nil {
		return 0, false, false
	}

	t.Lock()
	if t.localEntityId != entity.Id {
		id, reliable := t.localEntityId, t.localEntityReliable
		t.Unlock()
		return id, reliable, false
	}

	resolved := entity.Id
	if entity.OwnerId != 0 {
		resolved = resolveEntityOwner(entity.OwnerId, t.entityCache)
	}
	if t.localEntityId == resolved && t.localEntityReliable {
		t.Unlock()
		return resolved, true, false
	}

	t.localEntityId = resolved
	t.localEntityReliable = true
	clear(t.recentSkillActionIds)
	t.recentSkillActionOrder = t.recentSkillActionOrder[:0]
	t.Unlock()
	if entity.OwnerId != 0 {
		logger.Printf("local pet entity %d resolved to player %d", entity.Id, entity.OwnerId)
	} else {
		logger.Printf("local player entity confirmed: %d", entity.Id)
	}
	return resolved, true, true
}

// acceptSkillAction rejects only an identical non-zero combat action id. It
// deliberately does not use a time threshold, so real rapid casts and
// condition-triggered refreshes are still counted.
func (t *eventPublisher) acceptSkillAction(actionID uint32) bool {
	if actionID == 0 {
		return true
	}
	if _, exists := t.recentSkillActionIds[actionID]; exists {
		return false
	}

	t.recentSkillActionIds[actionID] = struct{}{}
	t.recentSkillActionOrder = append(t.recentSkillActionOrder, actionID)
	if len(t.recentSkillActionOrder) > 256 {
		oldest := t.recentSkillActionOrder[0]
		delete(t.recentSkillActionIds, oldest)
		t.recentSkillActionOrder = t.recentSkillActionOrder[1:]
	}
	return true
}

// publishSkillExecutePacket turns opcode 27016 into the canonical cooldown
// start event. Local players and owned entities feed the personal cooldown
// tracker; non-player sources are also published as remote actions so selected
// Boss mechanics such as Miel's Divine Spear can be identified.
func (t *eventPublisher) publishSkillExecutePacket(p *packet.GamePacket) error {
	if p == nil {
		return fmt.Errorf("skill execute: nil packet")
	}
	action, err := packet.ParseSkillExecutePacket(p.Msg)
	if err != nil {
		return err
	}
	sourceOwner := resolveEntityOwner(p.Id, t.entityCache)
	isLocal := t.localEntityId != 0 && (p.Id == t.localEntityId || sourceOwner == t.localEntityId)
	if !isLocal {
		entity := t.entityCache[p.Id]
		if entity == nil || entity.EntityInfo == nil || entity.IsUser() {
			return nil
		}
	}
	if !t.acceptSkillAction(action.ActionId) {
		return nil
	}

	eventID := p.Id
	if isLocal {
		eventID = t.localEntityId
	}
	t.publish(&event.EventSkillAction{
		EventBase: event.EventBase{
			EventId: event.EventIdSkillAction,
			At:      p.At.Unix(),
			Id:      strconv.FormatUint(eventID, 10),
		},
		SkillId:        action.SkillId,
		CombatActionId: action.ActionId,
		AtMs:           p.At.UnixMilli(),
		SourceId:       strconv.FormatUint(p.Id, 10),
		IsFallback:     false,
		IsLocal:        isLocal,
	})
	return nil
}

func (t *eventPublisher) publishBossSkillAction(p *packet.GamePacket, skillID uint16, clusterWindow time.Duration, fallback ...bool) {
	if p == nil || p.Id == 0 || skillID == 0 {
		return
	}
	entity := t.entityCache[p.Id]
	if entity != nil && entity.EntityInfo != nil && entity.IsUser() {
		return
	}
	// A single Boss cast can be broadcast by every owned mechanism fragment at
	// the same instant.  Deduplicate by the resolved Boss owner when the owner
	// chain is known, while retaining the actual fragment as the event source so
	// the frontend can still validate its race and ownership.
	dedupSourceID := resolveEntityOwner(p.Id, t.entityCache)
	if dedupSourceID == 0 {
		dedupSourceID = p.Id
	}
	key := fmt.Sprintf("%d:%d", dedupSourceID, skillID)
	atMs := p.At.UnixMilli()
	if t.recentBossSkillAt == nil {
		t.recentBossSkillAt = make(map[string]int64, 32)
	}
	if previous := t.recentBossSkillAt[key]; previous > 0 && atMs-previous < clusterWindow.Milliseconds() {
		return
	}
	t.recentBossSkillAt[key] = atMs
	t.publish(&event.EventSkillAction{
		EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)},
		SkillId:   skillID, AtMs: atMs, SourceId: strconv.FormatUint(p.Id, 10), IsLocal: false,
		IsFallback: len(fallback) > 0 && fallback[0],
	})
}

func (t *eventPublisher) publishBossLaserPacket(p *packet.GamePacket) bool {
	if p == nil {
		return false
	}
	switch p.Op {
	case packet.OpCode(0xafe7), packet.OpCode(0xafe8), packet.OpCode(0xafef):
		t.publishBossLaserSignal(p)
		return true
	default:
		return false
	}
}

func (t *eventPublisher) publishBossLaserSignal(p *packet.GamePacket) {
	// These cast opcodes are shared by several shard actions. The reference
	// client identifies the rotating beam only for the exact Divine Sword
	// payload: Short(52401), Byte(0).  0xafef also carries several unrelated
	// 524xx shard skills, so string/value-only matching is unsafe here.
	if p == nil || len(p.Msg) < 2 || p.Msg[0] == nil || p.Msg[1] == nil ||
		p.Msg[0].Type() != packet.MessageElemTypeShort ||
		p.Msg[1].Type() != packet.MessageElemTypeByte ||
		p.Msg[0].Data().(uint16) != 52401 || p.Msg[1].Data().(uint8) != 0 {
		return
	}
	t.publishBossSkillAction(p, 52401, 250*time.Millisecond)
}

func (t *eventPublisher) publishBossOrbCountdownSignal(p *packet.GamePacket) {
	// The mechanism signal has a fixed payload layout: Msg[0] is marker 280
	// and Msg[1] is step 4/5. Looking for these values anywhere in the packet
	// also matches unrelated Boss actions (entrance, Divine Advent, etc.).
	if p == nil || len(p.Msg) < 2 || p.Msg[0] == nil || p.Msg[1] == nil {
		return
	}
	marker := p.Msg[0].String()
	step := p.Msg[1].String()
	if marker != "280" {
		return
	}
	if t.recentBossSkillAt == nil {
		t.recentBossSkillAt = make(map[string]int64, 32)
	}
	atMs := p.At.UnixMilli()
	if atMs <= 0 {
		atMs = time.Now().UnixMilli()
	}
	pairKey := fmt.Sprintf("miel-orb-pair:%d", p.Id)
	if step == "4" {
		t.recentBossSkillAt[pairKey] = atMs
		return
	}
	if step != "5" {
		return
	}
	startedAt := t.recentBossSkillAt[pairKey]
	if startedAt <= 0 || atMs-startedAt > 250 {
		return
	}
	delete(t.recentBossSkillAt, pairKey)
	// This is a derived red-orb signal, not the Boss's real 52402 (Divine
	// Spear) skill execution. Mark it as fallback so the two reminders can be
	// configured independently in the UI.
	t.publishBossSkillAction(p, 52402, 1200*time.Millisecond, true)
}

func (t *eventPublisher) publishBossOrbLateConfirmation(p *packet.GamePacket) {
	// 0x6d66 is evidence that the orb is still alive, not a completion event.
	// Publish a dedicated mechanic signal so consumers can extend/confirm the
	// current reminder without borrowing a real game SkillId.
	if p == nil || p.Id == 0 {
		return
	}
	atMs := p.At.UnixMilli()
	if atMs <= 0 {
		atMs = time.Now().UnixMilli()
	}
	sourceID := strconv.FormatUint(p.Id, 10)
	t.publish(&event.EventSkillAction{
		EventBase: event.EventBase{
			EventId: event.EventIdSkillAction,
			At:      p.At.Unix(),
			Id:      sourceID,
		},
		AtMs:           atMs,
		MechanicSignal: "miel-orb-late-confirm",
		SourceId:       sourceID,
		IsLocal:        false,
	})
}

var techniqueSkillByCondition = map[uint32]uint16{
	479: 58000, // 坚定意志
	478: 58001, // 超越生命
	487: 58005, // 时间歪曲
	477: 58006, // 快速
	476: 58007, // 要害贯通
	521: 58010, // 洞察之眼
	522: 58013, // 再生之域
	520: 58014, // 力量团聚
	555: 58016, // 阻断
}

// Active techniques do not consistently emit the normal skill-execute
// packet. Their authoritative local signal is the technique's self-applied CC.
func (t *eventPublisher) publishTechniqueConditionSkillAction(at time.Time, condition *packet.CharacterConditionPacket) {
	if condition == nil || !condition.IsEnable || condition.Id == 0 || condition.Id != t.localEntityId {
		return
	}
	skillID := techniqueSkillByCondition[condition.CCId]
	if skillID == 0 {
		return
	}
	if t.recentBossSkillAt == nil {
		t.recentBossSkillAt = make(map[string]int64, 32)
	}
	atMs := at.UnixMilli()
	if atMs <= 0 {
		atMs = time.Now().UnixMilli()
	}
	key := fmt.Sprintf("technique:%d", skillID)
	if previous := t.recentBossSkillAt[key]; previous > 0 && atMs-previous < 1000 {
		return
	}
	t.recentBossSkillAt[key] = atMs
	t.publish(&event.EventSkillAction{
		EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: at.Unix(), Id: strconv.FormatUint(t.localEntityId, 10)},
		SkillId:   skillID, AtMs: atMs, SourceId: strconv.FormatUint(condition.Id, 10), IsFallback: false, IsLocal: true,
	})
}

func (t *eventPublisher) filterChangedStats(entityID uint64, stats []packet.StatUpdateEntry) []event.EventStatUpdateEntry {
	t.Lock()
	defer t.Unlock()
	if t.statCache == nil {
		t.statCache = make(map[uint64]map[uint32]float64, 64)
	}
	current := t.statCache[entityID]
	if current == nil {
		current = make(map[uint32]float64, len(stats))
		t.statCache[entityID] = current
	}
	changed := make([]event.EventStatUpdateEntry, 0, len(stats))
	for _, stat := range stats {
		if previous, exists := current[stat.StatId]; exists && previous == stat.Value {
			continue
		}
		current[stat.StatId] = stat.Value
		changed = append(changed, event.EventStatUpdateEntry{StatId: stat.StatId, Value: stat.Value})
	}
	return changed
}

// findLocalSkillAction returns the server-confirmed action performed by the
// local player. Most offensive skills use separate attacker and hit records,
// while a skill targeted at the caster can encode both roles in the same
// sub-packet. In that self-target form Hit and Attacker are both non-nil, so it
// must not be discarded merely because it also contains hit/receiver data.
func findLocalSkillAction(pack *packet.CombatActionPackPacket, localEntityID uint64) *packet.CombatActionPacket {
	if pack == nil || localEntityID == 0 {
		return nil
	}

	// Non-damaging skills can be represented by a local lifecycle record without
	// an attacker payload. Prefer that server-confirmed activation when present.
	for _, action := range pack.SubPackets {
		if action != nil && action.EntityId == localEntityID && action.Hit == nil &&
			action.SkillId != 0 && hasSkillLifecycleFlag(action.Type) {
			return action
		}
	}

	// Prefer the conventional attacker-only record when the pack contains one.
	for _, action := range pack.SubPackets {
		if action != nil && action.EntityId == localEntityID &&
			action.Hit == nil && action.Attacker != nil && action.SkillId != 0 {
			return action
		}
	}

	// Self-target support skills may merge the attacker and receiver records.
	for _, action := range pack.SubPackets {
		if action != nil && action.EntityId == localEntityID &&
			action.Attacker != nil && action.SkillId != 0 {
			return action
		}
	}

	// Keep compatibility with packets whose attacker payload could not be
	// decoded but whose type flag still identifies the local action.
	for _, action := range pack.SubPackets {
		if action != nil && action.EntityId == localEntityID && action.Hit == nil &&
			(action.Type&packet.CombatActionTypeAttacker) != 0 && action.SkillId != 0 {
			return action
		}
	}
	for _, action := range pack.SubPackets {
		if action != nil && action.EntityId == localEntityID &&
			(action.Type&packet.CombatActionTypeAttacker) != 0 && action.SkillId != 0 {
			return action
		}
	}

	return nil
}

// findTrackedSkillAction extends direct local-player detection to the small
// set of owned entities whose damage is intentionally credited to the player.
// Normal pet actions remain excluded.
func findTrackedSkillAction(pack *packet.CombatActionPackPacket, localEntityID uint64, entities entityCache) (*packet.CombatActionPacket, bool) {
	if action := findLocalSkillAction(pack, localEntityID); action != nil {
		return action, false
	}
	if pack == nil || localEntityID == 0 {
		return nil, false
	}

	for _, action := range pack.SubPackets {
		if !isUsableSkillActionRecord(action) || !isOwnedCooldownSkill(action.SkillId) {
			continue
		}
		entity := entities[action.EntityId]
		if entity != nil && entity.EntityInfo != nil && resolveEntityOwner(action.EntityId, entities) == localEntityID {
			return action, true
		}
	}
	return nil, false
}

// resolveEntityOwner follows deployable -> puppet/pet -> player chains. Some
// puppet and alchemy objects are one ownership hop deeper than normal pets;
// requiring a direct OwnerId made their successful actions disappear even
// though their damage could later be credited to the player.
func resolveEntityOwner(entityID uint64, entities entityCache) uint64 {
	resolved := entityID
	visited := make(map[uint64]struct{}, 4)
	for depth := 0; depth < 8; depth++ {
		if resolved == 0 {
			return entityID
		}
		if _, exists := visited[resolved]; exists {
			return resolved
		}
		visited[resolved] = struct{}{}
		entity := entities[resolved]
		if entity == nil || entity.EntityInfo == nil || entity.OwnerId == 0 {
			return resolved
		}
		resolved = entity.OwnerId
	}
	return resolved
}

// EffectDelayed subtype 319 uses the same verified damage layout as subtype
// 318. Other subtypes carry unrelated effect payloads and must stay excluded.
func isEffectDelayedDamageType(effectType uint32) bool {
	return effectType == 318 || effectType == 319
}

func isEffectDelayedDamageMessage(msg packet.Message) bool {
	if len(msg) < 7 {
		return false
	}
	return msg[0].Type() == packet.MessageElemTypeInt &&
		msg[1].Type() == packet.MessageElemTypeInt &&
		isEffectDelayedDamageType(msg[1].Data().(uint32)) &&
		msg[2].Type() == packet.MessageElemTypeInt &&
		msg[5].Type() == packet.MessageElemTypeLong &&
		msg[6].Type() == packet.MessageElemTypeShort
}

func isUsableSkillActionRecord(action *packet.CombatActionPacket) bool {
	if action == nil || action.SkillId == 0 {
		return false
	}
	if action.Attacker != nil {
		return true
	}
	return action.Hit == nil && hasSkillLifecycleFlag(action.Type)
}

func hasSkillLifecycleFlag(actionType packet.CombatActionType) bool {
	const lifecycleFlags = packet.CombatActionTypeSkillActive |
		packet.CombatActionTypeSkillSuccess |
		packet.CombatActionTypeSkillPlayerCharacter
	return actionType&lifecycleFlags != 0
}

func isOwnedCooldownSkill(skillID uint16) bool {
	return (skillID >= 54101 && skillID <= 54106) ||
		(skillID >= 54151 && skillID <= 54156) ||
		(skillID >= 59167 && skillID <= 59169) ||
		skillID == 35024
}

func isLocalTrackedConditionSource(entityID, attackerID, localEntityID uint64, entities entityCache) bool {
	if localEntityID == 0 {
		return false
	}
	for _, id := range []uint64{attackerID, entityID} {
		if id != 0 && (id == localEntityID || resolveEntityOwner(id, entities) == localEntityID) {
			return true
		}
	}
	return false
}

func parseConditionSkillMetadata(metadata string) (skillID uint16, sourceID uint64, ok bool) {
	if metadata == "" {
		return 0, 0, false
	}

	values := make(map[string]string)
	for _, rawField := range strings.Split(metadata, ";") {
		parts := strings.SplitN(rawField, ":", 3)
		if len(parts) != 3 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[2])
	}

	// MCDCPASI is present on the periodic re-application pulses emitted by
	// persistent fields such as Smoke Screen. Those pulses must not restart a
	// cooldown that belongs to the original cast.
	if values["MCDCPASI"] != "" {
		return 0, 0, false
	}

	var rawSkill string
	for _, key := range []string{"MCDCASI", "MCDCTSI"} {
		if values[key] != "" {
			rawSkill = values[key]
			break
		}
	}
	parsedSkill, err := strconv.ParseUint(rawSkill, 10, 16)
	if err != nil || parsedSkill == 0 {
		return 0, 0, false
	}
	if rawSource := values["MCDCTROID"]; rawSource != "" {
		sourceID, _ = strconv.ParseUint(rawSource, 10, 64)
	}
	return uint16(parsedSkill), sourceID, true
}

func (t *eventPublisher) publishConditionMetadataSkillAction(at time.Time, entityID, attackerID uint64, metadata string) {
	skillID, metadataSourceID, tracked := parseConditionSkillMetadata(metadata)
	if !tracked {
		return
	}
	// The condition owner can be the enemy target. Attribute the cast only
	// through the explicit metadata source (or AttackerId), never from its CC id.
	if !isLocalTrackedConditionSource(metadataSourceID, attackerID, t.localEntityId, t.entityCache) {
		return
	}

	sourceID := metadataSourceID
	if sourceID == 0 {
		sourceID = entityID
	}
	if sourceID == 0 {
		sourceID = attackerID
	}
	t.publish(&event.EventSkillAction{
		EventBase: event.EventBase{
			EventId: event.EventIdSkillAction,
			At:      at.Unix(),
			Id:      strconv.FormatUint(t.localEntityId, 10),
		},
		SkillId:    skillID,
		AtMs:       at.UnixMilli(),
		SourceId:   strconv.FormatUint(sourceID, 10),
		IsFallback: true,
		IsLocal:    true,
	})
}

func isContinuousDamageSkill(skillID uint16) bool {
	switch skillID {
	case 35013, 59025, 59004:
		return true
	default:
		return false
	}
}

func isFallbackSkillAction(_ *packet.CombatActionPackPacket, _ *packet.CombatActionPacket, _ bool) bool {
	// OpcodeSkillExecute is now the canonical server-confirmed cooldown signal.
	// Combat actions remain available as a compatibility fallback for servers
	// or skills that do not emit opcode 27016, but must never push a running
	// cooldown forward after the authoritative execution has been observed.
	return true
}

func newMessageBoxEvent(message string) *event.EventMessageBox {
	return &event.EventMessageBox{
		EventBase: event.EventBase{
			EventId: event.EventIdMessageBox,
			At:      time.Now().Unix(),
			Id:      "0",
		},
		Message: message,
	}
}
