package packet

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/constants"
)

// Opt-in regression for the original, UNSORTED Taiwan capture supplied with
// the 2026-10-05 incident. Never commit the capture or player event data.
// DILMETER_TW_DAMAGE_PCAP selects the capture. DILMETER_TW_DAMAGE_REPORT can
// optionally save aggregate (no player identifiers) diagnostic results.
func TestReplayTaiwanDamageOrdering(t *testing.T) {
	path := os.Getenv("DILMETER_TW_DAMAGE_PCAP")
	if path == "" {
		t.Skip("set DILMETER_TW_DAMAGE_PCAP")
	}
	if err := constants.ConfigureGameServer("210.208.80.0/24", []string{"11022"}); err != nil {
		t.Fatal(err)
	}
	defer constants.ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"})
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
	type result struct {
		Raw              float64
		Hits             int
		HealthLoss       float64
		Healing          float64
		LastHealth       float64
		HasHealth        bool
		Pending          float64
		UnattributedLoss float64
		ClippedRaw       float64
	}
	results := map[uint64]*result{}
	expected := map[uint64]struct {
		raw  float64
		hits int
	}{
		4767482419268160: {1622762074.9208221, 2857},
		4767482419274678: {1824813181.1100502, 3108},
		4767482419285654: {1844716959.184517, 3187},
		4767482419296435: {1710094956.565216, 2939},
		4767482419301072: {1814516023.3636246, 3048},
	}
	for id := range expected {
		results[id] = &result{}
	}
	hit := func(id uint64, damage float64) {
		if r := results[id]; r != nil {
			r.Raw += damage
			r.Hits++
			r.Pending += damage
		}
	}
	idle := time.NewTimer(8 * time.Second)
	defer idle.Stop()
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
			if p.Op == OpcodeCombatActionPack {
				pack, e := ParseCombatActionPackPacket(p)
				if e != nil {
					t.Fatal(e)
				}
				for _, a := range pack.SubPackets {
					if a.Hit != nil {
						hit(a.EntityId, float64(a.Hit.Damage))
					}
				}
			}
			if p.Op == OpcodeEffectDelayed && len(p.Msg) >= 7 && p.Msg[1].Type() == MessageElemTypeInt && p.Msg[2].Type() == MessageElemTypeInt {
				ty := p.Msg[1].Data().(uint32)
				if ty == 318 || ty == 319 {
					hit(p.Id, float64(p.Msg[2].Data().(uint32)))
				}
			}
			if p.Op == OpcodeStatUpdatePublic {
				if r := results[p.Id]; r != nil {
					stats, e := ParseStatUpdatePacket(p.Msg)
					if e != nil {
						t.Fatal(e)
					}
					for _, s := range stats {
						if s.StatId != 28 {
							continue
						}
						if r.HasHealth {
							loss := r.LastHealth - s.Value
							if loss > 0 {
								r.HealthLoss += loss
								r.UnattributedLoss += math.Max(0, loss-r.Pending)
							} else {
								r.Healing -= loss
							}
							r.ClippedRaw += math.Max(0, r.Pending-math.Max(0, loss))
						}
						r.LastHealth = s.Value
						r.HasHealth = true
						r.Pending = 0
					}
				}
			}
		case <-idle.C:
			for id, want := range expected {
				r := results[id]
				t.Logf("boss %d raw=%.3f hits=%d hpLoss=%.3f healing=%.3f lastHP=%.3f", id, r.Raw, r.Hits, r.HealthLoss, r.Healing, r.LastHealth)
				if math.Abs(r.Raw-want.raw) > 1 || r.Hits != want.hits {
					t.Errorf("boss %d: got %.3f/%d want %.3f/%d", id, r.Raw, r.Hits, want.raw, want.hits)
				}
			}
			if reader.parsedCount != 1617253 || reader.parseErrorCount != 4 {
				t.Errorf("packet parsing changed: parsed=%d errors=%d", reader.parsedCount, reader.parseErrorCount)
			}
			if report := os.Getenv("DILMETER_TW_DAMAGE_REPORT"); report != "" {
				b, e := json.MarshalIndent(results, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(report, b, 0600); e != nil {
					t.Fatal(e)
				}
			}
			return
		case <-ctx.Done():
			t.Fatal("capture replay timed out")
		}
	}
}
