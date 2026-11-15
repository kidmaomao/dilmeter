package main

import (
	"testing"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

func TestPerlonAreaNoticeRequiresPersonalPenalty(t *testing.T) {
	p := newSkillTestPublisher()
	p.localEntityId = 10
	p.lastSentEventAt = time.Now()
	penalty := "<color=5><outlinecolor=2>尔格正在减少。\n远离他人和地面的尔格释放区域，防御尔格吸收吧。</outlineColor></color>"
	for _, tc := range []struct {
		id   uint64
		kind uint8
		text string
	}{
		{11, 4, penalty}, // Another player is never evidence about this client.
		{10, 3, penalty},
		{10, 4, "当心不要卷入他人或地面的尔格释放区域。\n"},
		{10, 4, "尔格正在减少。利用他人或地面的尔格释放区域来防御尔格吸收。"},
	} {
		p.publishPerlonPacket(&packet.GamePacket{Op: packet.OpcodeNotice, Id: tc.id, At: time.UnixMilli(100000), Msg: packet.Message{packet.NewMessageElemByte(tc.kind), packet.NewMessageElemString(tc.text), packet.NewMessageElemInt(5000), packet.NewMessageElemByte(1)}})
	}
	if len(p.pendingEvents) != 0 {
		t.Fatal("general instruction or another player triggered an area alert")
	}
	for i, text := range []string{penalty, "<color=5><outlinecolor=2>因尔格不足，生命值正在减少。\n远离他人和地面的尔格释放区域，抵挡伤害吧。</outlineColor></color>"} {
		p.publishPerlonPacket(&packet.GamePacket{Op: packet.OpcodeNotice, Id: 10, At: time.UnixMilli(100000 + int64(i)*1010), Msg: packet.Message{packet.NewMessageElemByte(4), packet.NewMessageElemString(text), packet.NewMessageElemInt(5000), packet.NewMessageElemByte(1)}})
	}
	if len(p.pendingEvents) != 2 {
		t.Fatalf("both resource-loss and HP-loss variants must work: %d", len(p.pendingEvents))
	}
	for _, raw := range p.pendingEvents {
		if v := raw.(*event.EventSkillAction); v.Id != "10" || v.MechanicSignal != "perlon-release-area-warning" || v.SkillId != 0 {
			t.Fatalf("bad evidence %+v", v)
		}
	}
}

func TestPerlonTraditionalChineseNotices(t *testing.T) {
	publisher := newSkillTestPublisher()
	publisher.localEntityId = 10
	publisher.lastSentEventAt = time.Now()
	for _, notice := range []struct {
		kind uint8
		text string
		want string
	}{
		{3, "充滿整個空間的黑暗爾格開始吸收共鳴！", "perlon-absorb"},
		{3, "充滿整個空間的黑暗爾格開始釋放共鳴！", "perlon-release"},
		{3, "佩隆躲進了防禦結界中。", "perlon-shield"},
		{3, "佩隆的黑暗爾格開始暴走了！", "perlon-rampage"},
		{4, "爾格正在減少。遠離他人和地面的爾格釋放區域。", "perlon-release-area-warning"},
	} {
		publisher.pendingEvents = nil
		publisher.publishPerlonPacket(&packet.GamePacket{
			Op: packet.OpcodeNotice, Id: 10, At: time.UnixMilli(100000),
			Msg: packet.Message{packet.NewMessageElemByte(notice.kind), packet.NewMessageElemString(notice.text), packet.NewMessageElemInt(5000), packet.NewMessageElemByte(1)},
		})
		if len(publisher.pendingEvents) != 1 {
			t.Fatalf("unrecognized notice: %s", notice.text)
		}
		if got := publisher.pendingEvents[0].(*event.EventSkillAction).MechanicSignal; got != notice.want {
			t.Fatalf("notice %q: got %s, want %s", notice.text, got, notice.want)
		}
	}
}

func TestPerlonAreaWarningFreshnessAndPhaseIsolation(t *testing.T) {
	var sounds []nativeReminderSoundRequest
	r := perlonTestRuntime(&sounds)
	r.onEvent(liveTestAppear("self", "玩家", 10000, 99))
	r.onEvent(liveTestAppear("boss", "佩隆", perlonRaceID, 100))
	signal := func(id, key string, at int64) {
		r.onEvent(&event.EventSkillAction{EventBase: event.EventBase{Id: id, At: at / 1000}, AtMs: at, MechanicSignal: key})
	}
	card := func(at int64) nativeBossMechanicOverlayItem {
		t.Helper()
		items := r.perlonOverlay(time.UnixMilli(at))
		if len(items) != 1 {
			t.Fatalf("want resonance card, got %+v", items)
		}
		return items[0]
	}
	signal("self", "perlon-release", 100000)
	initial := card(100010)
	if initial.ResonanceMode != "release" || initial.AreaWarningUntilMs != 0 {
		t.Fatal("switch notification alone must not blink")
	}
	signal("other", "perlon-release-area-warning", 100050)
	if card(100100).AreaWarningUntilMs != 0 {
		t.Fatal("another player's penalty caused a warning")
	}
	signal("self", "perlon-release-area-warning", 100100)
	if got := card(100101); got.AreaWarningUntilMs != 101600 || got.Generation != initial.Generation {
		t.Fatalf("warning restarted the phase or has the wrong deadline: %+v", got)
	}
	signal("self", "perlon-release-area-warning", 101120)
	if card(101620).AreaWarningUntilMs != 102620 {
		t.Fatal("continued occupancy must renew the warning")
	}
	if card(102620).AreaWarningUntilMs != 0 {
		t.Fatal("warning survived stale server evidence")
	}
	signal("self", "perlon-release-area-warning", 103000)
	signal("self", "perlon-absorb", 103100)
	if got := card(103101); got.ResonanceMode != "absorb" || got.AreaWarningUntilMs != 0 {
		t.Fatal("moon inherited sunlight danger")
	}
	signal("self", "perlon-release-area-warning", 103110)
	if card(103120).AreaWarningUntilMs != 0 {
		t.Fatal("absorb phase accepted a release-area warning")
	}
	signal("self", "perlon-release", 104000)
	signal("self", "perlon-release-area-warning", 103900)
	if card(104001).AreaWarningUntilMs != 0 {
		t.Fatal("old phase evidence leaked into the next release")
	}
	signal("self", "perlon-release-area-warning", 104100)
	r.onEvent(&event.EventFinish{EventBase: event.EventBase{Id: "self", At: 104}})
	if card(104101).AreaWarningUntilMs != 0 {
		t.Fatal("death left an occupancy warning")
	}
	signal("self", "perlon-release-area-warning", 104110)
	if card(104111).AreaWarningUntilMs != 0 {
		t.Fatal("dead player resumed blinking")
	}
	if len(sounds) != 3 {
		t.Fatalf("penalty ticks must not repeatedly play audio: sounds=%d", len(sounds))
	}
}

func TestPerlonResonancePreviewVariants(t *testing.T) {
	preserveBossPreviewState(t)
	for _, tc := range []struct {
		mode        string
		warning     bool
		wantMode    string
		wantWarning bool
	}{
		{"", false, "absorb", false}, {"release", false, "release", false}, {"release", true, "release", true}, {"absorb", true, "absorb", false},
	} {
		setBossMechanicPreview(bossMechanicPreviewRequest{Key: perlonResonanceKey, DurationSeconds: 8, ResonanceMode: tc.mode, AreaWarning: tc.warning, ScalePercent: 145, OpacityPercent: 65}, 100000)
		setSkillOverlayStateAt([]byte(emptySkillOverlayState), 102000)
		items := readBurstPreviewState(t).Mechanics
		if len(items) != 1 || items[0].ResonanceMode != tc.wantMode || items[0].ScalePercent != 145 || items[0].OpacityPercent != 65 ||
			(items[0].AreaWarningUntilMs > 102000) != tc.wantWarning {
			t.Fatalf("preview state %+v", items)
		}
		setSkillOverlayStateAt([]byte(emptySkillOverlayState), 108000)
		if len(readBurstPreviewState(t).Mechanics) != 0 {
			t.Fatal("expired preview lingered")
		}
	}
}
