package packet

import "testing"

func TestSpiritAwakeningReady(t *testing.T) {
	for typ := uint16(1); typ <= 20; typ++ {
		got, err := ParseSpiritAwakeningReady(Message{NewMessageElemShort(typ)})
		if err != nil || got != typ {
			t.Fatalf("weapon %d: %d %v", typ, got, err)
		}
	}
	for _, msg := range []Message{nil, {NewMessageElemInt(20)}, {NewMessageElemShort(0)}, {NewMessageElemShort(21)}, {NewMessageElemShort(20), NewMessageElemInt(1)}} {
		if _, err := ParseSpiritAwakeningReady(msg); err == nil {
			t.Fatal("accepted invalid ready notification")
		}
	}
}

func TestSpiritWeaponSnapshotAndDelta(t *testing.T) {
	full := Message{NewMessageElemInt(2), NewMessageElemInt(0xffffffff),
		NewMessageElemShort(20), NewMessageElemShort(9), NewMessageElemShort(143),
		NewMessageElemLong(123), NewMessageElemString("spirit"), NewMessageElemShort(100),
		NewMessageElemInt(0), NewMessageElemInt(0), NewMessageElemShort(100),
		NewMessageElemShort(196), NewMessageElemShort(0), NewMessageElemString(""),
		NewMessageElemString(""), NewMessageElemShort(60), NewMessageElemInt(60000),
		NewMessageElemLong(456), NewMessageElemInt(0)}
	got, err := ParseSpiritWeaponUpdates(full)
	if err != nil || len(got) != 2 || *got[0].WeaponType != 20 || *got[0].Charge != 60000 || got[1].Charge != nil {
		t.Fatalf("snapshot: %+v %v", got, err)
	}
	delta := Message{NewMessageElemInt(2), NewMessageElemInt(0), NewMessageElemInt(1 << 14), NewMessageElemInt(2000)}
	got, err = ParseSpiritWeaponUpdates(delta)
	if err != nil || got[0].Charge != nil || got[1].WeaponType != nil || *got[1].Charge != 2000 {
		t.Fatalf("positional delta: %+v %v", got, err)
	}
	for n := 0; n < len(full); n++ {
		if _, err := ParseSpiritWeaponUpdates(full[:n]); err == nil {
			t.Fatalf("accepted partial snapshot of length %d", n)
		}
	}
	for _, bad := range []Message{
		{NewMessageElemInt(65)},
		{NewMessageElemInt(1), NewMessageElemInt(1 << 16)},
		{NewMessageElemInt(1), NewMessageElemInt(1 << 14), NewMessageElemInt(60001)},
		{NewMessageElemInt(1), NewMessageElemInt(1 << 14), NewMessageElemShort(100)},
		append(append(Message{}, delta...), NewMessageElemInt(0)),
	} {
		if _, err := ParseSpiritWeaponUpdates(bad); err == nil {
			t.Fatalf("accepted malformed update: %+v", bad)
		}
	}
}

func TestSpiritExecuteTargetIsNotActionToken(t *testing.T) {
	for id := uint16(35111); id <= 35130; id++ {
		got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(id), NewMessageElemLong(123)})
		if err != nil || got.ActionId != 0 {
			t.Fatalf("skill %d target used as dedup token: %+v %v", id, got, err)
		}
	}
	// Do not weaken deduplication for unverified multi-field forms.
	got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(35130), NewMessageElemLong(123), NewMessageElemInt(1)})
	if err != nil || got.ActionId != 123 {
		t.Fatal("multi-field token changed")
	}
}
