package packet

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"
)

// Opt-in verification against the user's local capture; captures are not shipped.
func TestReplayCrombasBatchAppearances(t *testing.T) {
	path := os.Getenv("DILMETER_HEALER_VISIBILITY_PCAP")
	if path == "" {
		t.Skip("set DILMETER_HEALER_VISIBILITY_PCAP")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	reader, err := NewGameServerPacketReader(&GameServerPacketReaderOpt{Ctx: ctx, LogDir: t.TempDir(), DisableCaptureLog: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if err := reader.OpenFile(path); err != nil {
		t.Fatal(err)
	}
	idle := time.NewTimer(8 * time.Second)
	defer idle.Stop()
	wanted := map[uint64]bool{4503599631431147: true, 4503599631370930: true, 4503599638761237: true}
	seen, lost := 0, 0
	for {
		select {
		case p := <-reader.PacketCh():
			if !idle.Stop() {
				select {
				case <-idle.C:
				default:
				}
			}
			idle.Reset(8 * time.Second)
			if p.Op != OpcodeEntitiesAppear {
				continue
			}
			parsed, err := ParseEntitiesAppearPacket(p)
			if err != nil {
				t.Fatal(err)
			}
			found := map[uint64]bool{}
			for _, entity := range parsed {
				found[entity.Id] = true
			}
			msg := p.Msg[1:]
			for len(msg) >= 3 {
				entry := msg[:3]
				msg = msg[3:]
				if entry[0].Type() != MessageElemTypeShort || entry[0].Data().(uint16) != 16 || entry[2].Type() != MessageElemTypeBin {
					continue
				}
				_, _, sub, err := GamePacketBodyReader(bytes.NewReader(entry[2].Data().([]byte)))
				if err != nil {
					continue
				}
				entity, err := ParseEntityAppearPacket(sub)
				if err != nil || entity == nil || !wanted[entity.Id] {
					continue
				}
				seen++
				if !found[entity.Id] {
					lost++
					t.Logf("lost batch member %s at %s", entity.Name, p.At.In(time.FixedZone("HKT", 8*3600)).Format("15:04:05"))
				}
			}
		case <-idle.C:
			t.Logf("teammate batch appearances=%d missing=%d", seen, lost)
			if seen == 0 || lost != 0 {
				t.Fail()
			}
			return
		case <-ctx.Done():
			t.Fatal("replay timed out")
		}
	}
}
