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
