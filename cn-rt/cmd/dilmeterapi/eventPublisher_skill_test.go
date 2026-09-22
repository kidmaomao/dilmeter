package main

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestReplaySkillExecuteCapture(t *testing.T) {
	file := os.Getenv("DILMETER_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_TEST_PCAP to run the raw-capture integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:               ctx,
		LogDir:            t.TempDir(),
		DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}

	wanted := map[uint16]bool{35024: false, 26006: false}
	for {
		select {
		case events := <-eventCh:
			for _, raw := range events {
				action, ok := raw.(*event.EventSkillAction)
				if ok {
					if _, tracked := wanted[action.SkillId]; tracked && !action.IsFallback {
						wanted[action.SkillId] = true
					}
				}
			}
			if wanted[35024] && wanted[26006] {
				return
			}
		case <-ctx.Done():
			t.Fatalf("capture did not publish both direct skill executions: Hydra=%t Smoke=%t", wanted[35024], wanted[26006])
		}
	}
}

func TestReplayMagnumAimCapture(t *testing.T) {
	file := os.Getenv("DILMETER_AIM_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_AIM_TEST_PCAP to verify the captured Magnum target lifecycle")
	}
	replaySkillStateCapture(t, file, map[string]bool{"aim:true": false, "aim:false": false})
}

func TestReplayFinalShotCapture(t *testing.T) {
	file := os.Getenv("DILMETER_FINAL_SHOT_TEST_PCAP")
	if file == "" {
		t.Skip("set DILMETER_FINAL_SHOT_TEST_PCAP to verify the captured Final Shot lifecycle")
	}
	replaySkillStateCapture(t, file, map[string]bool{"active:true": false, "active:false": false})
}

func replaySkillStateCapture(t *testing.T, file string, wanted map[string]bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case events := <-eventCh:
			for _, raw := range events {
				state, ok := raw.(*event.EventSkillState)
				if !ok {
					continue
				}
				key := state.Scope + ":" + strconv.FormatBool(state.Active)
				if _, tracked := wanted[key]; tracked {
					wanted[key] = true
				}
			}
			complete := true
			for _, found := range wanted {
				complete = complete && found
			}
			if complete {
				return
			}
		case <-ctx.Done():
			t.Fatalf("capture did not publish the complete skill state lifecycle: %#v", wanted)
		}
	}
}

func TestReplayBossLaserCapture(t *testing.T) {
	file := os.Getenv("DILMETER_BOSS_MECHANIC_PCAP")
	if file == "" {
		t.Skip("set DILMETER_BOSS_MECHANIC_PCAP to replay the Divine Sword capture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}

	for {
		select {
		case events := <-eventCh:
			for _, raw := range events {
				action, ok := raw.(*event.EventSkillAction)
				if ok && action.SkillId == 52401 && action.IsLocal == false && action.IsFallback == false {
					return
				}
			}
		case <-ctx.Done():
			t.Fatal("capture did not publish an exact 0xafef Divine Sword action")
		}
	}
}

func TestReplayDelayedDamageSubtype319Capture(t *testing.T) {
	file := os.Getenv("DILMETER_PASSIVE_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PASSIVE_PCAP to replay the subtype-319 passive-damage capture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:               ctx,
		LogDir:            t.TempDir(),
		DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}

	const (
		wantAttacker = "4503599631566088"
		wantTarget   = "4767482419252914"
	)
	wantDamage := map[float32]bool{2029: false, 48927: false}
	for {
		select {
		case events := <-eventCh:
			for _, raw := range events {
				damage, ok := raw.(*event.EventDamage)
				if !ok || damage.SkillId != 58100 || !damage.IsDelayed {
					continue
				}
				if damage.Id != wantAttacker || damage.TargetId != wantTarget {
					t.Fatalf("subtype-319 damage attribution = attacker %q target %q, want attacker %q target %q",
						damage.Id, damage.TargetId, wantAttacker, wantTarget)
				}
				if _, expected := wantDamage[damage.Damage]; !expected {
					t.Fatalf("unexpected subtype-319 skill 58100 damage: %v", damage.Damage)
				}
				wantDamage[damage.Damage] = true
			}
			if wantDamage[2029] && wantDamage[48927] {
				return
			}
		case <-ctx.Done():
			t.Fatalf("capture did not publish both subtype-319 skill 58100 hits: damage 2029=%t damage 48927=%t",
				wantDamage[2029], wantDamage[48927])
		}
	}
}

func TestReplayAllPassiveDamageSubtype319Capture(t *testing.T) {
	file := os.Getenv("DILMETER_PASSIVE_ALL_PCAP")
	if file == "" {
		t.Skip("set DILMETER_PASSIVE_ALL_PCAP to replay the subtype-319 all-passive capture")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx:               ctx,
		LogDir:            t.TempDir(),
		DisableCaptureLog: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	eventCh := make(chan []event.IEvent, 1000)
	publisher.addClient(ctx, eventCh)
	if err := reader.OpenFile(file); err != nil {
		t.Fatal(err)
	}

	type aggregate struct {
		count int
		sum   float32
	}
	want := map[uint16]aggregate{
		58009: {count: 3, sum: 5_300_558},
		58100: {count: 2, sum: 107_278},
		58101: {count: 9, sum: 3_424_255},
	}
	got := map[uint16]aggregate{}

	for {
		select {
		case events := <-eventCh:
			for _, raw := range events {
				damage, ok := raw.(*event.EventDamage)
				if !ok {
					continue
				}
				if _, tracked := want[damage.SkillId]; !tracked {
					continue
				}
				if !damage.IsDelayed {
					t.Fatalf("skill %d was not published as delayed damage", damage.SkillId)
				}
				if damage.Id == "" || damage.Id == "0" || damage.TargetId == "" || damage.TargetId == "0" {
					t.Fatalf("skill %d has empty attribution: attacker=%q target=%q", damage.SkillId, damage.Id, damage.TargetId)
				}
				entry := got[damage.SkillId]
				entry.count++
				entry.sum += damage.Damage
				got[damage.SkillId] = entry
			}

			complete := true
			for skillID, expected := range want {
				actual := got[skillID]
				if actual.count > expected.count {
					t.Fatalf("skill %d published %d hits, want %d", skillID, actual.count, expected.count)
				}
				if actual.count != expected.count {
					complete = false
				}
			}
			if complete {
				for skillID, expected := range want {
					actual := got[skillID]
					if actual.sum != expected.sum {
						t.Fatalf("skill %d damage sum = %.0f, want %.0f", skillID, actual.sum, expected.sum)
					}
				}
				return
			}
		case <-ctx.Done():
			t.Fatalf("capture passive damage totals = %#v, want %#v", got, want)
		}
	}
}

func newSkillTestPublisher() *eventPublisher {
	return &eventPublisher{
		entityCache:            make(entityCache),
		recentSkillActionIds:   make(map[uint32]struct{}, 256),
		recentSkillActionOrder: make([]uint32, 0, 256),
	}
}

func TestEffectDelayedDamageMessageCompatibility(t *testing.T) {
	valid := func(effectType uint32) packet.Message {
		return packet.Message{
			packet.NewMessageElemInt(0),
			packet.NewMessageElemInt(effectType),
			packet.NewMessageElemInt(48_927),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemInt(24),
			packet.NewMessageElemLong(4503599631566088),
			packet.NewMessageElemShort(58100),
		}
	}

	for _, effectType := range []uint32{318, 319} {
		if !isEffectDelayedDamageMessage(valid(effectType)) {
			t.Fatalf("EffectDelayed damage subtype %d was rejected", effectType)
		}
	}
	if isEffectDelayedDamageMessage(valid(102)) {
		t.Fatal("non-damage EffectDelayed subtype 102 was accepted")
	}

	badDamage := valid(319)
	badDamage[2] = packet.NewMessageElemFloat(48_927)
	if isEffectDelayedDamageMessage(badDamage) {
		t.Fatal("EffectDelayed packet with non-int damage was accepted")
	}
	badAttacker := valid(319)
	badAttacker[5] = packet.NewMessageElemInt(1)
	if isEffectDelayedDamageMessage(badAttacker) {
		t.Fatal("EffectDelayed packet with non-long attacker was accepted")
	}
	badSkill := valid(319)
	badSkill[6] = packet.NewMessageElemInt(58100)
	if isEffectDelayedDamageMessage(badSkill) {
		t.Fatal("EffectDelayed packet with non-short skill was accepted")
	}
	if isEffectDelayedDamageMessage(valid(319)[:6]) {
		t.Fatal("truncated EffectDelayed packet was accepted")
	}
}

func TestUpdateLocalEntityIdDoesNotSwitchToPet(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.updateLocalEntityId(100)
	if publisher.localEntityReliable {
		t.Fatal("unknown first entity must remain provisional until EntityAppear")
	}

	// An unknown private-stat id must not replace an established player.
	publisher.updateLocalEntityId(200)
	if publisher.localEntityId != 100 {
		t.Fatalf("unknown entity replaced player: got %d", publisher.localEntityId)
	}

	// Once the entity is known as an owned pet, it resolves back to its owner.
	publisher.entityCache[200] = &entityInfoExtend{
		EntityInfo: &packet.EntityInfo{Id: 200, OwnerId: 100},
	}
	publisher.updateLocalEntityId(200)
	if publisher.localEntityId != 100 {
		t.Fatalf("pet replaced player: got %d", publisher.localEntityId)
	}
	if !publisher.localEntityReliable {
		t.Fatal("known pet owner did not confirm the local player identity")
	}

	// A known ownerless entity can establish a new player after character swap.
	publisher.entityCache[300] = &entityInfoExtend{
		EntityInfo: &packet.EntityInfo{Id: 300},
	}
	publisher.updateLocalEntityId(300)
	if publisher.localEntityId != 300 {
		t.Fatalf("ownerless player did not replace old character: got %d", publisher.localEntityId)
	}
	if !publisher.localEntityReliable {
		t.Fatal("known ownerless player was not marked reliable")
	}
}

func TestReconcileLocalEntityInfoFixesPetFirstStartup(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.updateLocalEntityId(200)
	if publisher.localEntityId != 200 {
		t.Fatalf("first private entity was not provisionally selected: got %d", publisher.localEntityId)
	}
	if publisher.localEntityReliable {
		t.Fatal("unknown pet-first identity must remain provisional")
	}

	publisher.reconcileLocalEntityInfo(&packet.EntityInfo{Id: 200, OwnerId: 100})
	if publisher.localEntityId != 100 {
		t.Fatalf("pet-first startup did not resolve to owner: got %d", publisher.localEntityId)
	}
	if !publisher.localEntityReliable {
		t.Fatal("resolved pet owner was not marked reliable")
	}
}

func TestReconcileHiddenOwnedEntityFixesPetFirstStartup(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.updateLocalEntityId(200)
	entity := &packet.EntityInfo{Id: 200, OwnerId: 100, Name: "_hidden_pet"}
	publisher.entityCache.add(entity, time.Now())
	publisher.reconcileLocalEntityInfo(entity)
	if publisher.localEntityId != 100 || !publisher.localEntityReliable {
		t.Fatalf("hidden owned entity was not resolved: id=%d reliable=%t", publisher.localEntityId, publisher.localEntityReliable)
	}
}

func TestReconcileLocalEntityInfoConfirmsOwnerlessPlayer(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.updateLocalEntityId(100)

	id, reliable, changed := publisher.reconcileLocalEntityInfo(&packet.EntityInfo{Id: 100})
	if !changed || id != 100 || !reliable {
		t.Fatalf("ownerless player confirmation = (%d, %t, %t), want (100, true, true)", id, reliable, changed)
	}
}

func TestToEventLocalEntityCarriesReliability(t *testing.T) {
	e := toEventLocalEntity(123, 456, true, false)
	if e.EventId != 11 || e.At != 123 || e.Id != "456" || !e.Reliable || e.Reset {
		t.Fatalf("unexpected local entity event: %#v", e)
	}
}

func TestBeginConnectionEpochResetsOnlyTransientCaptureState(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	publisher.connectionEpoch = 1
	publisher.localEntityId = 100
	publisher.localEntityReliable = true
	publisher.recentSkillActionIds[42] = struct{}{}
	publisher.recentSkillActionOrder = append(publisher.recentSkillActionOrder, 42)
	publisher.entityCache[100] = &entityInfoExtend{
		EntityInfo: &packet.EntityInfo{Id: 100},
		characterConditionMap: map[uint32]*packet.EntityCharacterCondition{
			680: {CCId: 680, DisableAt: 1234},
		},
	}

	publisher.beginConnectionEpoch(2, time.Unix(500, 0))

	if publisher.connectionEpoch != 2 || publisher.localEntityId != 0 || publisher.localEntityReliable {
		t.Fatalf("identity was not reset: epoch=%d id=%d reliable=%t", publisher.connectionEpoch, publisher.localEntityId, publisher.localEntityReliable)
	}
	if len(publisher.entityCache) != 0 {
		t.Fatalf("entity cache retained %d entries", len(publisher.entityCache))
	}
	if len(publisher.recentSkillActionIds) != 0 || len(publisher.recentSkillActionOrder) != 0 {
		t.Fatal("skill-action de-duplication state was not reset")
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("reset published %d events, want 1", len(publisher.pendingEvents))
	}
	resetEvent, ok := publisher.pendingEvents[0].(*event.EventLocalEntity)
	if !ok || !resetEvent.Reset || resetEvent.Id != "0" || resetEvent.At != 500 {
		t.Fatalf("unexpected reset event: %#v", publisher.pendingEvents[0])
	}
}

func TestAcceptSkillActionDeduplicatesOnlyActionId(t *testing.T) {
	publisher := newSkillTestPublisher()
	if !publisher.acceptSkillAction(42) {
		t.Fatal("first action id was rejected")
	}
	if publisher.acceptSkillAction(42) {
		t.Fatal("duplicate action id was accepted")
	}
	if !publisher.acceptSkillAction(43) {
		t.Fatal("different action id was rejected")
	}
	if !publisher.acceptSkillAction(0) || !publisher.acceptSkillAction(0) {
		t.Fatal("zero action ids should not be deduplicated")
	}

	for id := uint32(1000); id < 1256; id++ {
		if !publisher.acceptSkillAction(id) {
			t.Fatalf("new action id %d was rejected", id)
		}
	}
	if !publisher.acceptSkillAction(42) {
		t.Fatal("old action id was not evicted from bounded cache")
	}
}

func TestFindLocalSkillActionAcceptsSelfTargetCombinedRecord(t *testing.T) {
	const localEntityID = uint64(100)
	pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{
		{
			EntityId: localEntityID,
			Type: packet.CombatActionTypeAttacker |
				packet.CombatActionTypeTakeHit |
				packet.CombatActionTypeSkillSuccess,
			SkillId: 59141,
			Attacker: &packet.CombatActionPacketAttackerInfo{
				TargetId: localEntityID,
			},
			Hit: &packet.CombatActionPacketHitInfo{},
		},
	}}

	action := findLocalSkillAction(pack, localEntityID)
	if action == nil {
		t.Fatal("self-target combined attacker/receiver record was rejected")
	}
	if action.SkillId != 59141 {
		t.Fatalf("unexpected skill id: got %d", action.SkillId)
	}
}

func TestFindLocalSkillActionPrefersAttackerOnlyRecord(t *testing.T) {
	const localEntityID = uint64(100)
	combined := &packet.CombatActionPacket{
		EntityId: localEntityID,
		Type:     packet.CombatActionTypeAttacker | packet.CombatActionTypeTakeHit,
		SkillId:  59141,
		Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: localEntityID},
		Hit:      &packet.CombatActionPacketHitInfo{},
	}
	attackerOnly := &packet.CombatActionPacket{
		EntityId: localEntityID,
		Type:     packet.CombatActionTypeAttacker,
		SkillId:  59143,
		Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 200},
	}
	pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{
		combined,
		attackerOnly,
	}}

	if action := findLocalSkillAction(pack, localEntityID); action != attackerOnly {
		t.Fatal("attacker-only record was not preferred")
	}
}

func TestFindLocalSkillActionRejectsLocalReceiverOnlyRecord(t *testing.T) {
	const localEntityID = uint64(100)
	pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{
		{
			EntityId: localEntityID,
			Type:     packet.CombatActionTypeTakeHit,
			SkillId:  59141,
			Hit:      &packet.CombatActionPacketHitInfo{},
		},
	}}

	if action := findLocalSkillAction(pack, localEntityID); action != nil {
		t.Fatal("receiver-only record was incorrectly treated as a local skill action")
	}
}

func TestFindLocalSkillActionAcceptsNonDamageLifecycleRecord(t *testing.T) {
	const localEntityID = uint64(100)
	for _, skillID := range []uint16{45008, 58005} {
		action := &packet.CombatActionPacket{
			EntityId: localEntityID,
			Type: packet.CombatActionTypeSkillActive |
				packet.CombatActionTypeSkillSuccess |
				packet.CombatActionTypeSkillPlayerCharacter,
			SkillId: skillID,
		}
		pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{action}}
		if got := findLocalSkillAction(pack, localEntityID); got != action {
			t.Fatalf("non-damage skill %d was not recognized", skillID)
		}
	}
}

func TestFindTrackedSkillActionAcceptsOwnedMarionetteAndHydra(t *testing.T) {
	const localEntityID = uint64(100)
	const ownedEntityID = uint64(200)
	entities := entityCache{
		ownedEntityID: {EntityInfo: &packet.EntityInfo{Id: ownedEntityID, OwnerId: localEntityID}},
	}

	for _, skillID := range []uint16{54151, 54156, 59167, 59169, 35024} {
		action := &packet.CombatActionPacket{
			EntityId: ownedEntityID,
			Type:     packet.CombatActionTypeAttacker,
			SkillId:  skillID,
			Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 300},
		}
		pack := &packet.CombatActionPackPacket{Hit: true, SubPackets: []*packet.CombatActionPacket{action}}
		got, owned := findTrackedSkillAction(pack, localEntityID, entities)
		if got != action || !owned {
			t.Fatalf("owned skill %d was not recognized: action=%p owned=%t", skillID, got, owned)
		}
		if !isFallbackSkillAction(pack, got, owned) {
			t.Fatalf("owned skill %d must be a fallback trigger", skillID)
		}
	}
}

func TestFindTrackedSkillActionAcceptsNestedOwnedDeployable(t *testing.T) {
	const localEntityID = uint64(100)
	entities := entityCache{
		200: {EntityInfo: &packet.EntityInfo{Id: 200, OwnerId: localEntityID}},
		300: {EntityInfo: &packet.EntityInfo{Id: 300, OwnerId: 200}},
	}
	action := &packet.CombatActionPacket{
		EntityId: 300, Type: packet.CombatActionTypeAttacker, SkillId: 35024,
		Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 400},
	}
	got, owned := findTrackedSkillAction(&packet.CombatActionPackPacket{Hit: true, SubPackets: []*packet.CombatActionPacket{action}}, localEntityID, entities)
	if got != action || !owned {
		t.Fatalf("nested deployable was not recognized: action=%p owned=%t", got, owned)
	}
}

func TestFindTrackedSkillActionRejectsOrdinaryPetSkill(t *testing.T) {
	const localEntityID = uint64(100)
	const petEntityID = uint64(200)
	entities := entityCache{
		petEntityID: {EntityInfo: &packet.EntityInfo{Id: petEntityID, OwnerId: localEntityID}},
	}
	action := &packet.CombatActionPacket{
		EntityId: petEntityID,
		Type:     packet.CombatActionTypeAttacker,
		SkillId:  12345,
		Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 300},
	}
	pack := &packet.CombatActionPackPacket{SubPackets: []*packet.CombatActionPacket{action}}
	if got, owned := findTrackedSkillAction(pack, localEntityID, entities); got != nil || owned {
		t.Fatalf("ordinary pet skill was tracked: action=%p owned=%t", got, owned)
	}
}

func TestCombatActionSkillSignalsAreCompatibilityFallbacks(t *testing.T) {
	action := &packet.CombatActionPacket{
		EntityId: 100,
		Type: packet.CombatActionTypeAttacker |
			packet.CombatActionTypeSkillPlayerCharacter,
		SkillId:  59025,
		Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 300},
	}
	if !isFallbackSkillAction(&packet.CombatActionPackPacket{Hit: true}, action, false) {
		t.Fatal("continuous damage hit must be fallback")
	}
	if !isFallbackSkillAction(&packet.CombatActionPackPacket{Hit: false}, action, false) {
		t.Fatal("continuous-skill activation must remain a compatibility fallback")
	}

	normal := *action
	normal.SkillId = 45008
	if !isFallbackSkillAction(&packet.CombatActionPackPacket{Hit: false}, &normal, false) {
		t.Fatal("non-damage combat action must remain a compatibility fallback")
	}
}

func TestConditionMetadataAcceptsLocalAttackerAndOwnedEntity(t *testing.T) {
	const localEntityID = uint64(100)
	entities := entityCache{
		200: {EntityInfo: &packet.EntityInfo{Id: 200, OwnerId: localEntityID}},
		300: {EntityInfo: &packet.EntityInfo{Id: 300, OwnerId: 200}},
	}

	if !isLocalTrackedConditionSource(999, localEntityID, localEntityID, entities) {
		t.Fatal("condition applied by the local player was not recognized")
	}
	if !isLocalTrackedConditionSource(300, 0, localEntityID, entities) {
		t.Fatal("condition on a nested owned deployable was not recognized")
	}
	if isLocalTrackedConditionSource(999, 998, localEntityID, entities) {
		t.Fatal("another player's condition was incorrectly recognized")
	}
}

func TestConditionWithoutSkillMetadataDoesNotPublishCooldownAction(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	publisher.publishConditionMetadataSkillAction(time.Now(), 300, 100, "")
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("a CC-only condition incorrectly started a cooldown")
	}
}

func TestConditionMetadataPublishesSmokeScreenCooldownOnce(t *testing.T) {
	const metadata = "MCDCASI:2:26006;MCDCTSI:2:26006;MCDCTROID:8:100;SBT:8:63921829263900;"
	skillID, sourceID, ok := parseConditionSkillMetadata(metadata)
	if !ok || skillID != 26006 || sourceID != 100 {
		t.Fatalf("metadata parse = (%d, %d, %t), want (26006, 100, true)", skillID, sourceID, ok)
	}

	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	at := time.Unix(1_800_000_000, 456_000_000)
	publisher.publishConditionMetadataSkillAction(at, 300, 0, metadata)
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 26006 || action.Id != "100" || action.SourceId != "100" || !action.IsFallback {
		t.Fatalf("unexpected Smoke Screen cooldown action: %#v", publisher.pendingEvents[0])
	}
}

func TestConditionMetadataRejectsPersistentSkillRefreshAndRemoteSource(t *testing.T) {
	const refreshed = "MCDCASI:2:26006;MCDCTSI:2:26006;MCDCTROID:8:100;MCDCPASI:2:26006;"
	if _, _, ok := parseConditionSkillMetadata(refreshed); ok {
		t.Fatal("periodic Smoke Screen refresh was treated as a new cast")
	}

	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	publisher.publishConditionMetadataSkillAction(time.Now(), 300, 0,
		"MCDCASI:2:26006;MCDCTSI:2:26006;MCDCTROID:8:999;")
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("another player's condition metadata started the local cooldown")
	}
}

func TestTechniqueConditionPublishesLocalCooldownActions(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1_800_000_000_123)

	wanted := map[uint32]uint16{
		479: 58000, 478: 58001, 487: 58005, 477: 58006, 476: 58007,
		521: 58010, 517: 58012, 522: 58013, 520: 58014, 555: 58016,
	}
	for ccID, skillID := range wanted {
		publisher.publishTechniqueConditionSkillAction(at, &packet.CharacterConditionPacket{
			Id: 100, IsEnable: true,
			EntityCharacterCondition: packet.EntityCharacterCondition{CCId: ccID},
		})
		got, ok := publisher.pendingEvents[len(publisher.pendingEvents)-1].(*event.EventSkillAction)
		if !ok || got.SkillId != skillID || got.Id != "100" || !got.IsLocal || got.IsFallback {
			t.Fatalf("CC %d produced unexpected technique action: %#v", ccID, got)
		}
	}
	if len(publisher.pendingEvents) != len(wanted) {
		t.Fatalf("published %d technique actions, want %d", len(publisher.pendingEvents), len(wanted))
	}
}

func TestTechniqueConditionRejectsRemoteAndDisable(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	for _, condition := range []*packet.CharacterConditionPacket{
		{Id: 999, IsEnable: true, EntityCharacterCondition: packet.EntityCharacterCondition{CCId: 479}},
		{Id: 100, IsEnable: false, EntityCharacterCondition: packet.EntityCharacterCondition{CCId: 479}},
	} {
		publisher.publishTechniqueConditionSkillAction(time.Now(), condition)
	}
	if len(publisher.pendingEvents) != 0 {
		t.Fatalf("remote/disabled technique published %d actions", len(publisher.pendingEvents))
	}
}

func TestFilterChangedStatsSuppressesExactCopies(t *testing.T) {
	publisher := newSkillTestPublisher()
	stats := []packet.StatUpdateEntry{{StatId: 28, Value: 100}, {StatId: 30, Value: 200}}
	if got := publisher.filterChangedStats(100, stats); len(got) != 2 {
		t.Fatalf("first stat snapshot contained %d changes, want 2", len(got))
	}
	if got := publisher.filterChangedStats(100, stats); len(got) != 0 {
		t.Fatalf("identical stat snapshot contained %d changes, want 0", len(got))
	}
	stats[0].Value = 90
	got := publisher.filterChangedStats(100, stats)
	if len(got) != 1 || got[0].StatId != 28 || got[0].Value != 90 {
		t.Fatalf("changed stat snapshot = %#v, want only HP=90", got)
	}
}

func TestSkillExecutePacketPublishesHydraCooldown(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	at := time.Unix(1_800_000_100, 126_000_000)
	packet_ := &packet.GamePacket{
		At: at,
		Id: 100,
		Msg: packet.Message{
			packet.NewMessageElemShort(35024),
			packet.NewMessageElemLong(3458777829178426275),
			packet.NewMessageElemInt(0),
			packet.NewMessageElemInt(1),
		},
	}
	if err := publisher.publishSkillExecutePacket(packet_); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 35024 || action.Id != "100" || action.SourceId != "100" || action.IsFallback {
		t.Fatalf("unexpected Hydra cooldown action: %#v", publisher.pendingEvents[0])
	}
	if action.AtMs != at.UnixMilli() || action.CombatActionId == 0 {
		t.Fatalf("Hydra timing/token was not retained: %#v", action)
	}

	if err := publisher.publishSkillExecutePacket(packet_); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("duplicate execution token published twice")
	}
}

func TestSkillExecutePacketAcceptsOwnedSourceAndRejectsRemoteSource(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	publisher.entityCache[200] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 200, OwnerId: 100}}

	owned := &packet.GamePacket{
		At:  time.Now(),
		Id:  200,
		Msg: packet.Message{packet.NewMessageElemShort(26006), packet.NewMessageElemInt(0)},
	}
	if err := publisher.publishSkillExecutePacket(owned); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("owned Smoke Screen execution was not published")
	}

	remote := &packet.GamePacket{
		At:  time.Now(),
		Id:  999,
		Msg: packet.Message{packet.NewMessageElemShort(26006), packet.NewMessageElemInt(0)},
	}
	if err := publisher.publishSkillExecutePacket(remote); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatal("remote skill execution started the local cooldown")
	}
}

func TestSkillExecutePacketPublishesMielDivineSpear(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 100
	publisher.lastSentEventAt = time.Now()
	const bossID = uint64(7603001)
	publisher.entityCache[bossID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: bossID, RaceId: 7603}}

	packet_ := &packet.GamePacket{
		At:  time.UnixMilli(1800000300123),
		Id:  bossID,
		Msg: packet.Message{packet.NewMessageElemShort(52402), packet.NewMessageElemInt(987654)},
	}
	if err := publisher.publishSkillExecutePacket(packet_); err != nil {
		t.Fatal(err)
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 52402 || action.SourceId != "7603001" || action.IsLocal || action.IsFallback {
		t.Fatalf("unexpected Divine Spear action: %#v", publisher.pendingEvents[0])
	}
}

func TestPublishBossSkillActionMarksRemoteAndDeduplicates(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.Unix(1_800_000_200, 250_000_000)
	packet_ := &packet.GamePacket{At: at, Id: 7603001}
	publisher.publishBossSkillAction(packet_, 52401, 250*time.Millisecond)
	publisher.publishBossSkillAction(packet_, 52401, 250*time.Millisecond)
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("published event count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 52401 || action.SourceId != "7603001" || action.IsLocal {
		t.Fatalf("unexpected Boss mechanic action: %#v", publisher.pendingEvents[0])
	}
	if action.AtMs != at.UnixMilli() {
		t.Fatalf("Boss mechanic timestamp = %d, want %d", action.AtMs, at.UnixMilli())
	}
}

func divineSwordDeployPacket(at time.Time, sourceID uint64) *packet.GamePacket {
	return &packet.GamePacket{
		At: at, Op: packet.OpCode(0xafef), Id: sourceID,
		Msg: packet.Message{
			packet.NewMessageElemShort(52401),
			packet.NewMessageElemByte(0),
			packet.NewMessageElemFloat(25154),
			packet.NewMessageElemFloat(500),
			packet.NewMessageElemFloat(36743),
			packet.NewMessageElemFloat(1),
		},
	}
}

func TestPublishBossLaserPacketDispatchesAFEFDivineSword(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1700000000000)
	const shardID = uint64(7604001)

	if !publisher.publishBossLaserPacket(divineSwordDeployPacket(at, shardID)) {
		t.Fatal("0xafef was not dispatched as a Boss laser packet")
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("Divine Sword beam signal count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 52401 || action.SourceId != "7604001" || action.IsLocal || action.IsFallback {
		t.Fatalf("unexpected Divine Sword beam action: %#v", publisher.pendingEvents[0])
	}
	if publisher.publishBossLaserPacket(&packet.GamePacket{At: at, Op: packet.OpCode(0xbeef), Id: shardID}) {
		t.Fatal("unrelated opcode was dispatched as a Boss laser packet")
	}
}

func TestPublishBossLaserSignalRequiresExactDivineSwordPayload(t *testing.T) {
	invalidMessages := []packet.Message{
		nil,
		{packet.NewMessageElemShort(52401)},
		{packet.NewMessageElemShort(52402), packet.NewMessageElemByte(0)},
		{packet.NewMessageElemShort(52401), packet.NewMessageElemByte(1)},
		{packet.NewMessageElemInt(52401), packet.NewMessageElemByte(0)},
		{packet.NewMessageElemShort(52401), packet.NewMessageElemShort(0)},
		{packet.NewMessageElemShort(52409), packet.NewMessageElemByte(1)},
	}
	for i, msg := range invalidMessages {
		publisher := newSkillTestPublisher()
		publisher.lastSentEventAt = time.Now()
		if !publisher.publishBossLaserPacket(&packet.GamePacket{
			At: time.UnixMilli(1700000000000), Op: packet.OpCode(0xafef), Id: 7604001, Msg: msg,
		}) {
			t.Fatalf("0xafef invalid payload %d bypassed the Boss laser dispatcher", i)
		}
		if len(publisher.pendingEvents) != 0 {
			t.Fatalf("invalid Divine Sword payload %d published %d alerts", i, len(publisher.pendingEvents))
		}
	}
}

func TestPublishBossLaserPacketKeepsLegacyOpcodes(t *testing.T) {
	for _, op := range []packet.OpCode{0xafe7, 0xafe8, 0xafef} {
		publisher := newSkillTestPublisher()
		publisher.lastSentEventAt = time.Now()
		p := divineSwordDeployPacket(time.UnixMilli(1700000000000), 7604001)
		p.Op = op
		if !publisher.publishBossLaserPacket(p) || len(publisher.pendingEvents) != 1 {
			t.Fatalf("opcode %#x published %d Divine Sword alerts, want 1", op, len(publisher.pendingEvents))
		}
	}
}

func TestPublishBossLaserPacketDeduplicatesSiblingFragmentsByBossOwner(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	const bossID = uint64(7603001)
	const unrelatedBossID = uint64(7615001)
	shards := []uint64{7604001, 7605001, 7606001, 7607001}
	publisher.entityCache[bossID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: bossID, RaceId: 7603}}
	for i, shardID := range shards {
		publisher.entityCache[shardID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{
			Id: shardID, RaceId: uint32(7604 + i), OwnerId: bossID,
		}}
	}
	publisher.entityCache[unrelatedBossID] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: unrelatedBossID, RaceId: 7615}}
	publisher.entityCache[7616001] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: 7616001, RaceId: 7616, OwnerId: unrelatedBossID}}

	at := time.UnixMilli(1786633420616)
	for _, shardID := range shards {
		publisher.publishBossLaserPacket(divineSwordDeployPacket(at, shardID))
	}
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("simultaneous sibling shard casts published %d alerts, want 1", len(publisher.pendingEvents))
	}
	first := publisher.pendingEvents[0].(*event.EventSkillAction)
	if first.SourceId != "7604001" {
		t.Fatalf("deduplicated cast source = %q, want first actual shard", first.SourceId)
	}

	// A different Boss owner must not be hidden by the first Boss's cluster.
	publisher.publishBossLaserPacket(divineSwordDeployPacket(at.Add(50*time.Millisecond), 7616001))
	if len(publisher.pendingEvents) != 2 {
		t.Fatalf("different Boss owner cast count = %d, want 2", len(publisher.pendingEvents))
	}
	// The same Boss may start another real cast once the cluster window ends.
	publisher.publishBossLaserPacket(divineSwordDeployPacket(at.Add(251*time.Millisecond), shards[1]))
	if len(publisher.pendingEvents) != 3 {
		t.Fatalf("next Divine Sword cast count = %d, want 3", len(publisher.pendingEvents))
	}
}

func TestPublishBossLaserPacketKeepsPerSourceDedupWhenOwnerUnknown(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1700000000000)

	publisher.publishBossLaserPacket(divineSwordDeployPacket(at, 7604001))
	publisher.publishBossLaserPacket(divineSwordDeployPacket(at, 7605001))
	if len(publisher.pendingEvents) != 2 {
		t.Fatalf("ownerless fragments published %d alerts, want one per source", len(publisher.pendingEvents))
	}
}

func TestPublishBossOrbCountdownSignalRequiresConfirmedPair(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1700000000000)
	start := &packet.GamePacket{
		At: at, Id: 7603001,
		Msg: packet.Message{packet.NewMessageElemShort(280), packet.NewMessageElemByte(4)},
	}
	confirm := &packet.GamePacket{
		At: at.Add(100 * time.Millisecond), Id: 7603001,
		Msg: packet.Message{packet.NewMessageElemShort(280), packet.NewMessageElemByte(5)},
	}
	publisher.publishBossOrbCountdownSignal(start)
	if len(publisher.pendingEvents) != 0 {
		t.Fatal("unconfirmed orb signal published an alert")
	}
	publisher.publishBossOrbCountdownSignal(confirm)
	if len(publisher.pendingEvents) != 1 {
		t.Fatalf("confirmed orb signal count = %d, want 1", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[0].(*event.EventSkillAction)
	if !ok || action.SkillId != 52402 || action.IsLocal || !action.IsFallback {
		t.Fatalf("unexpected orb mechanism action: %#v", publisher.pendingEvents[0])
	}
}

func TestPublishBossOrbCountdownSignalRejectsMarkerAndStepsOutsideFixedFields(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1700000000000)
	// Unrelated Boss packets may contain 280 and 4/5 in later fields. They are
	// not the red-orb marker/step pair and must not start a countdown.
	publisher.publishBossOrbCountdownSignal(&packet.GamePacket{
		At: at, Id: 7603001,
		Msg: packet.Message{
			packet.NewMessageElemShort(999),
			packet.NewMessageElemByte(1),
			packet.NewMessageElemShort(280),
			packet.NewMessageElemByte(4),
		},
	})
	publisher.publishBossOrbCountdownSignal(&packet.GamePacket{
		At: at.Add(100 * time.Millisecond), Id: 7603001,
		Msg: packet.Message{
			packet.NewMessageElemShort(999),
			packet.NewMessageElemByte(1),
			packet.NewMessageElemShort(280),
			packet.NewMessageElemByte(5),
		},
	})
	if len(publisher.pendingEvents) != 0 {
		t.Fatalf("unrelated payload published %d red-orb alerts", len(publisher.pendingEvents))
	}
}

func TestPublishBossOrbLateConfirmationUsesDedicatedMechanicSignal(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.lastSentEventAt = time.Now()
	at := time.UnixMilli(1_700_000_000_000)
	publisher.publishBossOrbCountdownSignal(&packet.GamePacket{
		At: at, Id: 7603001,
		Msg: packet.Message{packet.NewMessageElemShort(280), packet.NewMessageElemByte(4)},
	})
	publisher.publishBossOrbCountdownSignal(&packet.GamePacket{
		At: at.Add(100 * time.Millisecond), Id: 7603001,
		Msg: packet.Message{packet.NewMessageElemShort(280), packet.NewMessageElemByte(5)},
	})
	publisher.publishBossOrbLateConfirmation(&packet.GamePacket{At: at.Add(13 * time.Second), Id: 7603001})
	if len(publisher.pendingEvents) != 2 {
		t.Fatalf("late confirmation published %d events, want start plus confirmation", len(publisher.pendingEvents))
	}
	action, ok := publisher.pendingEvents[1].(*event.EventSkillAction)
	if !ok {
		t.Fatalf("late confirmation event type = %T, want *event.EventSkillAction", publisher.pendingEvents[1])
	}
	if action.EventId != event.EventIdSkillAction {
		t.Fatalf("late confirmation EventId = %d, want %d", action.EventId, event.EventIdSkillAction)
	}
	if action.MechanicSignal != "miel-orb-late-confirm" {
		t.Fatalf("late confirmation MechanicSignal = %q", action.MechanicSignal)
	}
	if action.SkillId != 0 {
		t.Fatalf("late confirmation reused real SkillId %d", action.SkillId)
	}
	if action.AtMs != at.Add(13*time.Second).UnixMilli() {
		t.Fatalf("late confirmation AtMs = %d, want %d", action.AtMs, at.Add(13*time.Second).UnixMilli())
	}
	if action.SourceId != "7603001" || action.IsLocal {
		t.Fatalf("unexpected late confirmation source/local fields: %#v", action)
	}
}

func TestParseMagnumAimAndFinalShotLifecyclePackets(t *testing.T) {
	active, targetID, skillID, ok := parseSkillTarget(packet.Message{
		packet.NewMessageElemByte(1),
		packet.NewMessageElemLong(0x12345678),
		packet.NewMessageElemShort(magnumShotSkillID),
		packet.NewMessageElemByte(0),
	})
	if !ok || !active || targetID != 0x12345678 || skillID != magnumShotSkillID {
		t.Fatalf("aim start = active:%t target:%x skill:%d ok:%t", active, targetID, skillID, ok)
	}
	active, _, _, ok = parseSkillTarget(packet.Message{packet.NewMessageElemByte(0)})
	if !ok || active {
		t.Fatalf("aim clear = active:%t ok:%t", active, ok)
	}
	if skillID, ok = parsePreparedSkillID(packet.Message{packet.NewMessageElemShort(finalShotSkillID)}); !ok || skillID != finalShotSkillID {
		t.Fatalf("Final Shot ready = skill:%d ok:%t", skillID, ok)
	}
	if skillID, ok = parseEndedSkillID(packet.Message{
		packet.NewMessageElemByte(0), packet.NewMessageElemByte(1),
		packet.NewMessageElemByte(1), packet.NewMessageElemShort(finalShotSkillID),
	}); !ok || skillID != finalShotSkillID {
		t.Fatalf("Final Shot end = skill:%d ok:%t", skillID, ok)
	}
}
