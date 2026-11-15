package main

import (
	"context"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestHealerPartyRosterTracksSlotsAndKeepsLocalRow(t *testing.T) {
	runtime, _ := healerFixture(t)
	h := runtime.healer
	h.settings = healerDungeonSettings()
	runtime.onEvent(&event.EventLocalEntity{EventBase: event.EventBase{Id: "100", At: 101}, Reliable: true})
	runtime.onEvent(liveTestAppear("100", "本人", 9001, 101))
	runtime.onEvent(liveTestAppear("200", "队长", 10001, 101))
	runtime.onEvent(healerHP("100", 101, 710, 1000))
	healerTick(runtime, 101000)
	if rows := h.overlaySnapshot(101000).Groups[0].Rows; len(rows) != 1 || rows[0].Key != "self" || rows[0].Health.Value != "71%" {
		t.Fatalf("local row must appear before any shield or party: %+v", rows)
	}

	publisher, batches := perlonPacketTestPublisher()
	publisher.localEntityId, publisher.localEntityReliable = 100, true
	deliver := func(at int64, id uint64, op packet.OpCode, msg packet.Message) {
		publisher.publishHealerParty(&packet.GamePacket{At: time.Unix(at, 0), Id: id, Op: op, Msg: msg})
		publisher.publishNow()
		for len(batches) > 0 {
			for _, value := range <-batches {
				runtime.onEvent(value)
			}
		}
	}
	member := func(id uint64, slot uint32, name string) packet.Message {
		return packet.Message{
			packet.NewMessageElemInt(slot), packet.NewMessageElemLong(id), packet.NewMessageElemString(name),
			packet.NewMessageElemByte(1), packet.NewMessageElemInt(4016), packet.NewMessageElemInt(40000), packet.NewMessageElemInt(38416),
		}
	}
	reply := packet.Message{
		packet.NewMessageElemByte(1), packet.NewMessageElemLong(300), packet.NewMessageElemString("队伍"), packet.NewMessageElemLong(200),
		packet.NewMessageElemByte(0), packet.NewMessageElemInt(0), packet.NewMessageElemInt(0), packet.NewMessageElemLong(0),
		packet.NewMessageElemInt(8), packet.NewMessageElemInt(0), packet.NewMessageElemInt(2),
		packet.NewMessageElemString(""), packet.NewMessageElemString(""), packet.NewMessageElemInt(2),
	}
	reply = append(reply, member(100, 2, "本人")...)
	reply = append(reply, packet.NewMessageElemByte(0), packet.NewMessageElemInt(100))
	reply = append(reply, member(200, 1, "队长")...)
	deliver(102, 100, healerPartyJoinReply, reply)
	healerTick(runtime, 102000)
	rows := h.overlaySnapshot(102000).Groups[0].Rows
	if len(rows) != 2 || rows[0].Name != "队长" || rows[0].PartySlot != 1 || rows[1].Key != "self" || rows[1].PartySlot != 2 {
		t.Fatalf("party roster was not displayed in game slot order: %+v", rows)
	}
	if len(h.state.TemporaryMembers) != 1 || h.state.TemporaryMembers[0].ID != "200" {
		t.Fatalf("automatic template did not follow the known party: %+v", h.state.TemporaryMembers)
	}
	// The party's life reading can arrive before any direct life update.
	status := append(packet.Message{packet.NewMessageElemLong(200)}, member(200, 1, "队长")...)
	status = append(status, packet.NewMessageElemByte(0), packet.NewMessageElemInt(83), packet.NewMessageElemInt(100))
	deliver(102, 100, healerPartySlotUpdate, status)
	healerTick(runtime, 102100)
	if rows := h.overlaySnapshot(102100).Groups[0].Rows; rows[0].Health.Value != "83%" {
		t.Fatalf("party's life percent was unavailable before the first full life update: %+v", rows)
	}
	// View disappearance is independent of party membership.
	runtime.onEvent(&event.EventEntityDisappear{EventBase: event.EventBase{Id: "200", At: 103}})
	healerTick(runtime, 103000)
	if rows := h.overlaySnapshot(103000).Groups[0].Rows; len(rows) != 2 || rows[0].Health.Value != "83%" {
		t.Fatalf("out-of-view party member lost their slot: %+v", rows)
	}
	deliver(104, 0, healerPartyMemberLeave, packet.Message{packet.NewMessageElemLong(200)})
	healerTick(runtime, 104000)
	if rows := h.overlaySnapshot(104000).Groups[0].Rows; len(rows) != 1 || rows[0].PartySlot != 2 || len(h.state.TemporaryMembers) != 0 {
		t.Fatalf("departed member survived in the party roster: %+v", rows)
	}
	deliver(105, 100, healerPartyLeaveReply, packet.Message{packet.NewMessageElemByte(1)})
	healerTick(runtime, 105000)
	if rows := h.overlaySnapshot(105000).Groups[0].Rows; len(rows) != 1 || rows[0].PartySlot != 0 || rows[0].Key != "self" {
		t.Fatalf("leaving the party should retain only the player: %+v", rows)
	}
}

func TestHealerPartyCaptureReplay(t *testing.T) {
	path := os.Getenv("DILMETER_PARTY_ORDER_PCAP")
	if path == "" {
		t.Skip("set DILMETER_PARTY_ORDER_PCAP to the 2026-11-15 party capture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reader, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	publisher := newEventPublisher(ctx, reader)
	batches := make(chan []event.IEvent, 64)
	publisher.addClient(ctx, batches)
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}

	runtime, _ := healerFixture(t)
	h := runtime.healer
	h.settings = healerDungeonSettings()
	var at int64
	idle := time.NewTimer(5 * time.Second)
	defer idle.Stop()
	var sawOrderedParty, sawSelfWithoutShield bool
	for {
		select {
		case <-ctx.Done():
			t.Fatal("party capture replay timed out")
		case <-idle.C:
			if !sawOrderedParty || !sawSelfWithoutShield {
				t.Fatalf("capture did not display the game's two ordered slots and self without a shield. party=%v self=%v",
					sawOrderedParty, sawSelfWithoutShield)
			}
			return
		case batch := <-batches:
			idle.Reset(5 * time.Second)
			for _, value := range batch {
				base := value.(interface{ GetEventBase() *event.EventBase }).GetEventBase()
				if base.Sequence == 0 {
					continue
				}
				at = max(at, base.At*1000)
				runtime.onEvent(value)
			}
			healerTick(runtime, at)
			frame := h.overlaySnapshot(at)
			if len(frame.Groups) != 1 {
				continue
			}
			rows := frame.Groups[0].Rows
			if len(rows) == 2 && rows[0].PartySlot == 1 && rows[0].Key == "self" && rows[1].PartySlot == 2 &&
				rows[1].Name != rows[0].Name && len(h.state.TemporaryMembers) == 1 {
				sawOrderedParty = true
			}
			for _, row := range rows {
				if row.Key == "self" && row.Curtain == "" && row.Health != nil {
					sawSelfWithoutShield = true
				}
			}
		}
	}
}
