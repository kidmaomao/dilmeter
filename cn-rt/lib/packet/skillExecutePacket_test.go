package packet

import "testing"

func TestParseSkillExecutePacket(t *testing.T) {
	tests := []struct {
		name  string
		msg   Message
		skill uint16
	}{
		{
			name: "deployable long token",
			msg: Message{
				NewMessageElemShort(35024),
				NewMessageElemLong(3458777829178426275),
				NewMessageElemInt(0),
				NewMessageElemInt(1),
			},
			skill: 35024,
		},
		{
			name: "ordinary int action",
			msg: Message{
				NewMessageElemShort(20018),
				NewMessageElemInt(0),
			},
			skill: 20018,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSkillExecutePacket(tc.msg)
			if err != nil {
				t.Fatal(err)
			}
			if got.SkillId != tc.skill {
				t.Fatalf("skill id = %d, want %d", got.SkillId, tc.skill)
			}
		})
	}
}

func TestParseSkillExecutePacketRejectsInvalidMessage(t *testing.T) {
	if _, err := ParseSkillExecutePacket(Message{NewMessageElemInt(35024)}); err == nil {
		t.Fatal("expected invalid skill id type to fail")
	}
}

func TestThunderExecuteFlagsAreNotActionIds(t *testing.T) {
	msg := Message{NewMessageElemShort(30102), NewMessageElemInt(100), NewMessageElemInt(1)}
	got, err := ParseSkillExecutePacket(msg)
	if err != nil || got.ActionId != 0 {
		t.Fatalf("Thunder mode flag treated as execution id: %#v %v", got, err)
	}
	msg[2] = NewMessageElemInt(0)
	got, err = ParseSkillExecutePacket(msg)
	if err != nil || got.ActionId != 100 {
		t.Fatal("unverified Thunder form lost its action id")
	}
	msg[0], msg[2] = NewMessageElemShort(20018), NewMessageElemInt(1)
	got, err = ParseSkillExecutePacket(msg)
	if err != nil || got.ActionId != 100 {
		t.Fatal("unrelated integer action changed")
	}
}

func TestGunnerExecuteTargetIsNotDedupToken(t *testing.T) {
	for _, skill := range []uint16{54302, 59121, 59122, 59123} {
		got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(skill), NewMessageElemLong(4767482420927774)})
		if err != nil || got.ActionId != 0 {
			t.Fatalf("skill %d target treated as token: %#v, %v", skill, got, err)
		}
	}
	token := uint64(4767482420927774)
	got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(59121), NewMessageElemLong(token), NewMessageElemInt(0), NewMessageElemInt(1)})
	if err != nil || got.ActionId != 0 {
		t.Fatal("heavy target with flags treated as execution token")
	}
	got, err = ParseSkillExecutePacket(Message{NewMessageElemShort(35024), NewMessageElemLong(token), NewMessageElemInt(0), NewMessageElemInt(1)})
	if err != nil || got.ActionId != uint32(token)^uint32(token>>32) {
		t.Fatal("deployable execution token changed")
	}
}

func TestAlchemyExecuteTargetIsNotDedupToken(t *testing.T) {
	for _, skill := range []uint16{59144, 59145} {
		got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(skill), NewMessageElemLong(4767482421021708), NewMessageElemInt(0), NewMessageElemInt(1), NewMessageElemShort(2)})
		if err != nil || got.ActionId != 0 {
			t.Fatalf("alchemy skill %d suppressed by target deduplication: %#v %v", skill, got, err)
		}
	}
}

func TestPuppetAct7TargetIsNotDedupToken(t *testing.T) {
	msg := Message{NewMessageElemShort(54105), NewMessageElemLong(4767482421050579), NewMessageElemInt(0), NewMessageElemInt(1), NewMessageElemFloat(8147), NewMessageElemFloat(8533), NewMessageElemFloat(-0.89), NewMessageElemFloat(0.45)}
	got, err := ParseSkillExecutePacket(msg)
	if err != nil || got.ActionId != 0 {
		t.Fatalf("Act 7 target suppresses later charges: %#v %v", got, err)
	}
	msg[4] = NewMessageElemInt(8147)
	got, err = ParseSkillExecutePacket(msg)
	if err != nil || got.ActionId == 0 {
		t.Fatal("unverified packet form loses token deduplication")
	}
}

func TestFighterTargetIsNotDedupToken(t *testing.T) {
	for _, skill := range []uint16{24101, 24201, 24301, 59181, 59182, 59180, 59185, 59186, 59187} {
		msg := Message{NewMessageElemShort(skill), NewMessageElemLong(4767482419126404)}
		if skill == 59180 || skill >= 59185 {
			msg = append(msg, NewMessageElemInt(0), NewMessageElemInt(1))
		}
		got, err := ParseSkillExecutePacket(msg)
		if err != nil || got.ActionId != 0 {
			t.Fatalf("fighter %d suppressed by target token: %#v %v", skill, got, err)
		}
	}
}

func TestArcheryExecuteFlagsAreNotActionIds(t *testing.T) {
	for _, skill := range []uint16{21001, 21002} {
		msg := Message{NewMessageElemShort(skill), NewMessageElemInt(520), NewMessageElemInt(1)}
		got, err := ParseSkillExecutePacket(msg)
		if err != nil || got.ActionId != 0 {
			t.Fatalf("archery %d mode flag used as action id: %#v %v", skill, got, err)
		}
		msg[2] = NewMessageElemInt(0)
		got, err = ParseSkillExecutePacket(msg)
		if err != nil || got.ActionId != 520 {
			t.Fatal("unverified archery form lost its action id")
		}
	}
	got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(20018), NewMessageElemInt(520), NewMessageElemInt(1)})
	if err != nil || got.ActionId != 520 {
		t.Fatal("unrelated integer action changed")
	}
}

func TestStingerExecuteTargetIsNotDedupToken(t *testing.T) {
	for _, skill := range []uint16{59060, 59061, 59062, 59063, 59064} {
		msg := Message{NewMessageElemShort(skill), NewMessageElemLong(4767482419822679), NewMessageElemInt(0), NewMessageElemInt(1), NewMessageElemShort(0), NewMessageElemShort(1), NewMessageElemShort(3)}
		got, err := ParseSkillExecutePacket(msg)
		if err != nil || got.ActionId != 0 {
			t.Fatalf("stinger %d target used as token: %#v %v", skill, got, err)
		}
		msg[4] = NewMessageElemInt(0)
		got, err = ParseSkillExecutePacket(msg)
		if err != nil || got.ActionId == 0 {
			t.Fatal("unverified elemental field loses token deduplication")
		}
	}
	for _, skill := range []uint16{21014, 59064} {
		got, err := ParseSkillExecutePacket(Message{NewMessageElemShort(skill), NewMessageElemLong(4767482419822679), NewMessageElemInt(0), NewMessageElemInt(1)})
		if err != nil || got.ActionId != 0 {
			t.Fatalf("archery %d target form used as token: %#v %v", skill, got, err)
		}
	}
}
