package main

import "testing"

func TestBurstTaiwanNamesDoNotChangeSkillIdentity(t *testing.T) {
	settings := normalizeNativeBurstSettings(nativeBurstSettings{Rules: map[uint16]nativeBurstRule{
		59005: {Name: "崩壞的波動", CCID: 99, CastSeconds: 3},
		58014: {Name: "力量團聚", CCID: 99, CooldownSeconds: 999, Ready: nativeBurstDisplay{Enabled: true}},
	}})
	if got := settings.Rules[59005]; got.Name != "崩壞的波動" || got.SkillID != 59005 || got.CCID != 803 || got.CastSeconds != 3 {
		t.Fatalf("localized collapse changed identity/settings: %+v", got)
	}
	if got := settings.Rules[58014]; got.Name != "力量團聚" || got.CCID != 516 || got.CooldownSeconds != 0 || got.Ready.Enabled {
		t.Fatalf("localized power changed identity/timing: %+v", got)
	}
	settings.Rules[59005] = nativeBurstRule{Name: "力量團聚"}
	if got := normalizeNativeBurstSettings(settings).Rules[59005]; got.Name != "崩坏波动" || got.SkillID != 59005 {
		t.Fatalf("a mismatched label must not change the source skill: %+v", got)
	}
}
