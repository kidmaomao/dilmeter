package packet

import "testing"

func TestHealingHandsTargetIsNotActionID(t *testing.T) {
	const target uint64 = 4503599631566088
	for _, tc := range []struct {
		name string
		msg  Message
		want uint32
	}{
		{"captured target and flags", Message{NewMessageElemShort(59000), NewMessageElemLong(target), NewMessageElemInt(0), NewMessageElemInt(1)}, 0},
		{"other skill keeps token", Message{NewMessageElemShort(35024), NewMessageElemLong(target), NewMessageElemInt(0), NewMessageElemInt(1)}, uint32(target&0xffffffff) ^ uint32(target>>32)},
		{"unverified flags keep token", Message{NewMessageElemShort(59000), NewMessageElemLong(target), NewMessageElemInt(0), NewMessageElemInt(2)}, uint32(target&0xffffffff) ^ uint32(target>>32)},
		{"unverified shape keeps token", Message{NewMessageElemShort(59000), NewMessageElemLong(target)}, uint32(target&0xffffffff) ^ uint32(target>>32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseSkillExecutePacket(tc.msg)
			if err != nil || got.ActionId != tc.want {
				t.Fatalf("action = %#v, error = %v; want action ID %d", got, err, tc.want)
			}
		})
	}
}
