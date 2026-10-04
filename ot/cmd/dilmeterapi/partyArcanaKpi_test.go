package main

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// Rebuild the original typed packets without converting uint64 IDs through
// float64. Only public messages are fed to the teammate path: no local ACKs,
// aim counters or private resource baselines may make this test pass.
func readArcanaReplay(t *testing.T, file string) []*packet.GamePacket {
	t.Helper()
	f, err := os.Open(file)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4096), 16*1024*1024)
	var result []*packet.GamePacket
	for scanner.Scan() {
		var row struct {
			AtMs   int64
			Op     packet.OpCode
			Id     string
			Fields []struct {
				Type  int
				Value json.RawMessage
			}
		}
		if err := json.Unmarshal(scanner.Bytes(), &row); err != nil {
			t.Fatal(err)
		}
		var id uint64
		if err := json.Unmarshal([]byte(row.Id), &id); err != nil {
			t.Fatal(err)
		}
		p := &packet.GamePacket{Id: id, Op: row.Op, At: time.UnixMilli(row.AtMs)}
		for _, field := range row.Fields {
			decode := func(v any) {
				if err := json.Unmarshal(field.Value, v); err != nil {
					t.Fatal(err)
				}
			}
			switch field.Type {
			case 1:
				var v uint8
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemByte(v))
			case 2:
				var v uint16
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemShort(v))
			case 3:
				var v uint32
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemInt(v))
			case 4:
				var v uint64
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemLong(v))
			case 5:
				var v float32
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemFloat(v))
			case 6:
				var v string
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemString(v))
			case 7:
				var v []byte
				decode(&v)
				p.Msg = append(p.Msg, packet.NewMessageElemBin(v))
			default:
				t.Fatalf("unknown field type %d", field.Type)
			}
		}
		result = append(result, p)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestReplayPartyArcanaPublicPackets(t *testing.T) {
	directory := os.Getenv("DILMETER_PARTY_ARCANA_REPLAY_DIR")
	if directory == "" {
		t.Skip("set DILMETER_PARTY_ARCANA_REPLAY_DIR to replay the archived controlled captures")
	}
	for _, name := range []string{"gunner-kpi", "gunner-domain-kpi", "gunner-domain-repeat-kpi", "alchemist-kpi", "dark-0130", "puppeteer-kpi", "fighter-1009"} {
		t.Run(name, func(t *testing.T) {
			p := newPartyStingerTestPublisher()
			const caster = 4503599631566088
			p.entityCache[caster] = &entityInfoExtend{EntityInfo: &packet.EntityInfo{Id: caster, RaceId: 9001}}
			packets := readArcanaReplay(t, filepath.Join(directory, name+"-packets.ndjson"))
			var observed []event.IEvent
			for _, raw := range packets {
				p.lastSentEventAt = time.Now()
				if raw.Op != 37011 && raw.Op != 37013 && raw.Op != 28006 && raw.Op != packet.OpcodeCombatActionPack {
					continue
				}
				p.publishPartyArcanaPacket(raw)
				p.publishDarkMagePacket(raw)
				if raw.Op == packet.OpcodeCombatActionPack {
					pack, err := packet.ParseCombatActionPackPacket(raw)
					if err != nil {
						t.Fatal(err)
					}
					p.observePartyArcanaCombat(raw, pack)
					p.publishPartyCombatAction(raw, pack)
				}
				observed = append(observed, p.pendingEvents...)
				p.pendingEvents = nil
			}
			counts := map[string][]uint32{}
			casts := map[uint16]int{}
			for _, raw := range observed {
				if s, ok := raw.(*event.EventArcanaSignal); ok && s.Complete {
					counts[s.Signal] = append(counts[s.Signal], s.Count)
				}
				if a, ok := raw.(*event.EventSkillAction); ok && !a.IsFallback {
					casts[a.SkillId]++
				}
			}
			t.Logf("public-only teammate replay: samples=%v casts=%v", counts, casts)
			wantSamples := map[string]map[string][]uint32{
				"gunner-kpi":               {"domain-sample": {0, 0}, "sniper-counter": {6}},
				"gunner-domain-kpi":        {"domain-sample": {3}},
				"gunner-domain-repeat-kpi": {"domain-sample": {0, 1}}, // last release is truncated before its hit
				"alchemist-kpi":            {"chemical-sample": {5}},
				"dark-0130":                {"lightning-chain-state": {1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0}},
				"puppeteer-kpi":            {"interlude-sample": {1, 3, 3, 3, 2, 2}},
				"fighter-1009":             {"fighter-combo": {1, 1, 1, 1, 1, 1, 1}, "fighter-spend-end": {0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
			}
			if !reflect.DeepEqual(counts, wantSamples[name]) {
				t.Fatalf("sample mismatch: got %v want %v", counts, wantSamples[name])
			}
			if name == "dark-0130" && !reflect.DeepEqual(casts, map[uint16]int{30102: 87, 59040: 11, 59045: 4}) {
				t.Fatalf("missing or duplicate dark casts: %v", casts)
			}
			if output := os.Getenv("DILMETER_PARTY_ARCANA_EVENTS_DIR"); output != "" {
				file, err := os.Create(filepath.Join(output, name+"-party-signals.ndjson"))
				if err != nil {
					t.Fatal(err)
				}
				defer file.Close()
				encoder := json.NewEncoder(file)
				for _, e := range observed {
					if err := encoder.Encode(e); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestPartyChemicalRequiresMatchingCountAndCaster(t *testing.T) {
	for _, scenario := range []string{"complete", "cancel", "wrong-target", "wrong-count", "truncated", "other-caster"} {
		t.Run(scenario, func(t *testing.T) {
			p := newPartyStingerTestPublisher()
			msg := packet.Message{packet.NewMessageElemInt(850), packet.NewMessageElemInt(924), packet.NewMessageElemLong(200), packet.NewMessageElemInt(350), packet.NewMessageElemInt(150), packet.NewMessageElemInt(5), packet.NewMessageElemInt(150)}
			start := &packet.GamePacket{Id: 101, Op: 37013, At: time.UnixMilli(1000), Msg: msg}
			p.publishPartyArcanaPacket(start)
			p.publishPartyArcanaPacket(start)
			if scenario == "cancel" {
				p.publishPartyArcanaPacket(&packet.GamePacket{Id: 101, Op: 28006, At: time.UnixMilli(1100), Msg: packet.Message{packet.NewMessageElemByte(0)}})
			}
			attacker, target, count := uint64(101), uint64(200), uint8(5)
			if scenario == "wrong-target" {
				target = 201
			}
			if scenario == "other-caster" {
				attacker = 102
			}
			if scenario == "wrong-count" {
				count = 4
			}
			pack := &packet.CombatActionPackPacket{CombatActionId: 7, SubPackets: []*packet.CombatActionPacket{
				{EntityId: attacker, SkillId: 59144, Attacker: &packet.CombatActionPacketAttackerInfo{}},
				{EntityId: target, Hit: &packet.CombatActionPacketHitInfo{Options: packet.CombatActionHitOptionsMultiHit, MultiHitCount: uint32(count)}},
			}}
			if scenario != "truncated" {
				p.observePartyArcanaCombat(&packet.GamePacket{At: time.UnixMilli(1500)}, pack)
				p.observePartyArcanaCombat(&packet.GamePacket{At: time.UnixMilli(1500)}, pack)
			}
			completed := 0
			for _, raw := range p.pendingEvents {
				if s, ok := raw.(*event.EventArcanaSignal); ok && s.Complete {
					completed++
					if s.Id != "101" || s.Count != 5 || s.TargetId != "200" {
						t.Fatalf("bad sample %+v", s)
					}
				}
			}
			want := 0
			if scenario == "complete" {
				want = 1
			}
			if completed != want {
				t.Fatalf("samples=%d want=%d", completed, want)
			}
		})
	}
}

func TestPartyPublicReleaseFlagsAndAckDedup(t *testing.T) {
	p := newPartyStingerTestPublisher()
	for index, skill := range []uint16{20017, 21002, 59026, 59028, 59105, 59106} {
		for _, flags := range []packet.CombatActionType{0x42, 0x72, 0x72} {
			a := &packet.CombatActionPacket{EntityId: 101, SkillId: skill, Type: flags, CombatActionId: uint32(index + 1), Attacker: &packet.CombatActionPacketAttackerInfo{TargetId: 200}}
			p.publishPartyCombatAction(&packet.GamePacket{At: time.UnixMilli(1000 + int64(index))}, &packet.CombatActionPackPacket{CombatActionId: a.CombatActionId, SubPackets: []*packet.CombatActionPacket{a}})
		}
	}
	confirmed, fallback := 0, 0
	for _, raw := range p.pendingEvents {
		if a, ok := raw.(*event.EventSkillAction); ok {
			if a.IsFallback {
				fallback++
			} else {
				confirmed++
			}
		}
	}
	if confirmed != 6 || fallback != 6 {
		t.Fatalf("confirmed=%d fallback=%d", confirmed, fallback)
	}
	p.pendingEvents = nil
	public := &packet.GamePacket{Id: 101, Op: 37013, At: time.UnixMilli(2000), Msg: packet.Message{packet.NewMessageElemInt(500), packet.NewMessageElemInt(818), packet.NewMessageElemInt(2), packet.NewMessageElemLong(200)}}
	p.publishPartyArcanaPacket(public)
	p.publishPartyArcanaPacket(public)
	if err := p.publishSkillExecutePacket(&packet.GamePacket{Id: 101, Op: 27016, At: time.UnixMilli(2005), Msg: packet.Message{packet.NewMessageElemShort(59040), packet.NewMessageElemLong(200)}}); err != nil {
		t.Fatal(err)
	}
	if len(p.pendingEvents) != 1 {
		t.Fatalf("public effect plus ACK double counted: %d", len(p.pendingEvents))
	}
}

func TestPartyArcanaIsolationAndSniperLifecycle(t *testing.T) {
	p := newPartyStingerTestPublisher()
	start := packet.Message{packet.NewMessageElemInt(920), packet.NewMessageElemByte(4), packet.NewMessageElemLong(200), packet.NewMessageElemFloat(5000), packet.NewMessageElemFloat(50), packet.NewMessageElemInt(200), packet.NewMessageElemInt(100), packet.NewMessageElemInt(100), packet.NewMessageElemByte(1)}
	shot := func(phase uint32) packet.Message {
		return packet.Message{packet.NewMessageElemInt(920), packet.NewMessageElemByte(5), packet.NewMessageElemInt(phase), packet.NewMessageElemLong(200), packet.NewMessageElemFloat(100), packet.NewMessageElemFloat(100), packet.NewMessageElemFloat(50)}
	}
	send := func(id uint64, at int64, msg packet.Message) {
		p.publishPartyArcanaPacket(&packet.GamePacket{Id: id, At: time.UnixMilli(at), Op: 37011, Msg: msg})
	}
	for _, id := range []uint64{100, 101, 102, 103, 104, 999} {
		send(id, 1000, start)
	}
	send(101, 1000, start)
	send(101, 1100, shot(6))
	p.publishPartyArcanaPacket(&packet.GamePacket{Id: 102, At: time.UnixMilli(1150), Op: 28006, Msg: packet.Message{packet.NewMessageElemByte(0)}})
	send(102, 1200, shot(7))
	send(101, 1100, shot(7)) // a second shot may share the capture timestamp
	send(101, 1200, shot(7))
	want := map[string]uint32{"101": 2}
	got := map[string]uint32{}
	for _, raw := range p.pendingEvents {
		if s, ok := raw.(*event.EventArcanaSignal); ok && s.Complete {
			got[s.Id] = s.Count
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mixed actors, duplicate, or incomplete lifecycle: %v", got)
	}
	send(101, 2000, start)
	p.beginConnectionEpoch(1, time.UnixMilli(2100))
	p.localEntityId = 100
	send(101, 2200, shot(7))
	if state := p.partyArcana[101]; state != nil && state.sniper != nil {
		t.Fatal("sniper crossed connection reset")
	}
}
