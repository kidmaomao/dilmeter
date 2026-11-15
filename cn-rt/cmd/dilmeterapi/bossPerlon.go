package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const perlonRaceID uint32 = 193810
const perlonTornadoKey = "perlon-tornado"
const perlonResonanceKey = "perlon-resonance"

// Keep a short dodge warning after startup. This is display retention, not
// extra preparation time or an estimate of the tornado's full lifetime.
const perlonTornadoHoldMs int64 = 2000

// Local penalty feedback repeats every 1002–1037 ms in the capture. There is
// no proven zone-exit packet: expire the evidence instead of guessing from
// movement or allowing an old warning to blink for the rest of the phase.
const perlonAreaWarningFreshMs int64 = 1500

type nativePerlonHealthSettings struct {
	Enabled        bool `json:"enabled"`
	X              int  `json:"x"`
	Y              int  `json:"y"`
	ScalePercent   int  `json:"scalePercent"`
	OpacityPercent int  `json:"opacityPercent"`
}

type nativePerlonRuntime struct {
	BossID                    string
	Wave                      int
	WaveAt                    int64
	SeenPhantoms              map[string]bool
	Mode                      string
	ModeAt, ModeDue           int64
	TornadoAt, TornadoDue     int64
	TornadoTargetID           string
	ReleaseAreaWarningUntilMs int64
}

// 37011/967/1 is the tornado deployment, not a hit. The capture contains
// 63 deployments with a 1000 ms startup and 3000 ms initial lifetime. Later
// 967/4/6 packets move/split the same tornado and must not restart the warning.
func (t *eventPublisher) publishPerlonPacket(p *packet.GamePacket) {
	if p == nil {
		return
	}
	t.observePerlonTarget(p)
	mechanicTargetID := ""
	signal := ""
	duration := int64(0)
	if p.Op == packet.OpCode(37011) {
		m := p.Msg
		types := []packet.MessageElemType{3, 3, 4, 5, 5, 5, 3, 3}
		if len(m) != len(types) {
			return
		}
		for i, typ := range types {
			if m[i].Type() != typ {
				return
			}
		}
		if m[0].Data().(uint32) != 967 || m[1].Data().(uint32) != 1 {
			return
		}
		entity := t.entityCache[p.Id]
		if entity == nil || entity.EntityInfo == nil || entity.RaceId != perlonRaceID {
			return
		}
		duration = int64(m[6].Data().(uint32))
		if duration < 100 || duration > 10000 || m[2].Data().(uint64) == 0 {
			return
		}
		key := "perlon-tornado:" + strconv.FormatUint(m[2].Data().(uint64), 10)
		if t.recentBossSkillAt == nil {
			t.recentBossSkillAt = make(map[string]int64)
		}
		if previous, exists := t.recentBossSkillAt[key]; exists && p.At.UnixMilli()-previous <= 60000 {
			return
		}
		for previous, at := range t.recentBossSkillAt {
			if strings.HasPrefix(previous, "perlon-tornado:") && p.At.UnixMilli()-at > 60000 {
				delete(t.recentBossSkillAt, previous)
			}
		}
		t.recentBossSkillAt[key] = p.At.UnixMilli()
		signal = perlonTornadoKey
		mechanicTargetID = t.perlonDeploymentTarget(p)
	} else if p.Op == packet.OpcodeNotice && p.Id == t.localEntityId {
		m := p.Msg
		if len(m) != 4 || m[0].Type() != 1 || m[1].Type() != 6 || m[2].Type() != 3 || m[3].Type() != 1 {
			return
		}
		text := m[1].Data().(string)
		switch m[0].Data().(uint8) {
		case 3:
			switch {
			case strings.Contains(text, "充满整个空间的黑暗尔格开始吸收共鸣！"):
				signal = "perlon-absorb"
			case strings.Contains(text, "充满整个空间的黑暗尔格开始释放共鸣！"):
				signal = "perlon-release"
			case text == "佩隆躲进了防御结界中。":
				signal = "perlon-shield"
			case text == "佩隆的黑暗尔格开始暴走了！":
				signal = "perlon-rampage"
			default:
				return
			}
		case 4:
			// The general "当心不要卷入..." instruction is sent to everyone.
			// Only these two personal penalty messages prove unsafe occupancy.
			if !strings.Contains(text, "远离他人和地面的尔格释放区域") ||
				!(strings.Contains(text, "尔格正在减少。") || strings.Contains(text, "因尔格不足，生命值正在减少。")) {
				return
			}
			signal = "perlon-release-area-warning"
		default:
			return
		}
	} else {
		return
	}
	// Keep mechanic evidence out of skill-use counters (SkillId remains zero).
	id := strconv.FormatUint(p.Id, 10)
	t.publish(&event.EventSkillAction{EventBase: event.EventBase{EventId: event.EventIdSkillAction, At: p.At.Unix(), Id: id}, AtMs: p.At.UnixMilli(), SourceId: id, MechanicSignal: signal, MechanicDurationMs: duration, MechanicTargetId: mechanicTargetID, IsFallback: true})
	if signal == perlonTornadoKey {
		// The startup is only one second: deliver it together with preceding
		// entity updates without waiting for the ordinary event batch timer.
		t.publishNow()
	}
}

// Called after the ordinary entity/stat update. Settings changes do not reset
// the observed encounter; death, a new boss and connection resets do.
func (r *nativeReminderRuntime) observePerlonEvent(raw event.IEvent) {
	c := &r.perlon
	switch v := raw.(type) {
	case *event.EventLocalEntity:
		if v.Reset || c.BossID != "" && (r.entities[c.BossID] == nil || !r.entities[c.BossID].Active) {
			*c = nativePerlonRuntime{}
		}
	case *event.EventEntityAppear:
		if v.RaceId == perlonRaceID && c.BossID != v.Id {
			*c = nativePerlonRuntime{BossID: v.Id, SeenPhantoms: make(map[string]bool)}
		}
		if v.RaceId >= 193811 && v.RaceId <= 193813 && c.BossID != "" && !c.SeenPhantoms[v.Id] {
			if c.SeenPhantoms == nil {
				c.SeenPhantoms = make(map[string]bool)
			}
			c.SeenPhantoms[v.Id] = true
			if c.WaveAt == 0 || v.At-c.WaveAt > 10 {
				c.Wave = min(2, c.Wave+1)
				if boss := r.entities[c.BossID]; boss != nil && boss.HealthKnown && boss.MaximumHealth > 0 && boss.CurrentHealth/boss.MaximumHealth < 0.55 {
					c.Wave = 2
				}
				c.WaveAt = v.At
				c.Mode = ""
				c.ModeAt = 0
				c.ModeDue = 0
				c.ReleaseAreaWarningUntilMs = 0
			}
		}
	case *event.EventEntityDisappear:
		if v.Id == c.BossID || v.Id == r.localID {
			*c = nativePerlonRuntime{}
		}
	case *event.EventFinish:
		if v.Id == r.localID || v.Id == c.BossID {
			c.ReleaseAreaWarningUntilMs = 0
		}
	case *event.EventSkillAction:
		boss := r.entities[c.BossID]
		if boss == nil || !boss.Active || boss.Defeated || boss.RaceID != perlonRaceID {
			return
		}
		at := nativeEventAtMs(v.At, v.AtMs)
		if v.MechanicSignal == perlonTornadoKey && v.Id == c.BossID && v.MechanicDurationMs > 0 {
			if at <= c.TornadoAt {
				return
			}
			c.TornadoAt, c.TornadoDue = at, at+v.MechanicDurationMs
			c.TornadoTargetID = v.MechanicTargetId
			if r.perlonTornadoEnabled() {
				r.playPerlonSound(perlonTornadoKey)
			}
			return
		}
		if v.Id != r.localID {
			return
		}
		switch v.MechanicSignal {
		case "perlon-release-area-warning":
			if local := r.entities[r.localID]; local != nil && local.Defeated {
				return
			}
			if c.Mode == "perlon-release" && at >= c.ModeAt {
				c.ReleaseAreaWarningUntilMs = max(c.ReleaseAreaWarningUntilMs, at+perlonAreaWarningFreshMs)
			}
		case "perlon-shield":
			// The 60% shield pauses resonance; the 25% shield does not.
			if c.Wave <= 1 {
				c.Mode = ""
				c.ModeAt = 0
				c.ModeDue = 0
				c.ReleaseAreaWarningUntilMs = 0
			}
		case "perlon-rampage":
			c.Mode = ""
			c.ModeAt = 0
			c.ModeDue = 0
			c.ReleaseAreaWarningUntilMs = 0
		case "perlon-absorb", "perlon-release":
			if at <= c.ModeAt || v.MechanicSignal == c.Mode && at-c.ModeAt < 1000 {
				return
			}
			period := int64(0)
			if c.Wave == 1 {
				period = 15000
				if c.ModeAt == 0 {
					period = 13000
				}
			}
			if c.Wave >= 2 {
				period = 10000
				if c.ModeAt == 0 {
					period = 8000
				}
			}
			// Mid-encounter attachment has no phase anchor: show the mode but
			// leave timing unknown until an actual interval is observed.
			if c.Wave == 0 && c.ModeAt > 0 && at-c.ModeAt >= 8000 && at-c.ModeAt <= 16000 {
				period = at - c.ModeAt
			}
			c.Mode, c.ModeAt, c.ModeDue = v.MechanicSignal, at, 0
			c.ReleaseAreaWarningUntilMs = 0
			if period > 0 {
				c.ModeDue = at + period
			}
			r.playPerlonSound(perlonResonanceKey)
		}
	}
}

func (r *nativeReminderRuntime) playPerlonSound(key string) {
	rule, ok := r.settings.BossMechanics.Rules[key]
	if !ok || !rule.Enabled {
		return
	}
	if rule.SoundMode == "custom" && rule.CustomSoundID != "" {
		r.playSound(nativeReminderSoundRequest{Kind: "custom", SoundID: rule.CustomSoundID, Volume: r.settings.BossMechanics.Volume, Priority: true, Tornado: key == perlonTornadoKey})
	} else if rule.SoundMode == "dedicated" {
		r.playSound(nativeReminderSoundRequest{Kind: "skill-ready", Volume: r.settings.BossMechanics.Volume, Priority: true, Tornado: key == perlonTornadoKey})
	}
}

func (r *nativeReminderRuntime) perlonOverlay(now time.Time) []nativeBossMechanicOverlayItem {
	c := r.perlon
	boss := r.entities[c.BossID]
	if boss == nil || !boss.Active || boss.Defeated {
		return nil
	}
	at := now.UnixMilli()
	items := []nativeBossMechanicOverlayItem{}
	add := func(key, name string, start, end int64, unknown bool) {
		rule := r.settings.BossMechanics.Rules[key]
		if !rule.Enabled || !rule.ShowCountdown {
			return
		}
		item := nativeBossMechanicOverlayItem{Key: key, Name: name, Icon: "mdi-weather-windy", StartedAtMs: start, EndsAtMs: end, Generation: uint64(start), TimingUnknown: unknown, X: rule.X, Y: rule.Y, ScalePercent: rule.ScalePercent, OpacityPercent: rule.OpacityPercent}
		if key == perlonTornadoKey {
			item.TornadoStartsAtMs = c.TornadoDue
			item.TornadoTarget = r.perlonTornadoAudience()
		}
		if key == perlonResonanceKey {
			item.ResonanceMode = strings.TrimPrefix(c.Mode, "perlon-")
			if c.Mode == "perlon-release" && c.ReleaseAreaWarningUntilMs > at {
				item.AreaWarningUntilMs = c.ReleaseAreaWarningUntilMs
			}
		}
		items = append(items, item)
	}
	if c.TornadoAt > 0 && at >= c.TornadoAt && c.TornadoDue+perlonTornadoHoldMs > at && r.perlonTornadoEnabled() {
		add(perlonTornadoKey, "尔格龙卷风", c.TornadoAt, c.TornadoDue+perlonTornadoHoldMs, false)
	}
	if c.Mode != "" && at >= c.ModeAt && at-c.ModeAt < 30000 {
		name := "吸收 · 靠近区域"
		if c.Mode == "perlon-release" {
			name = "释放 · 远离区域"
		}
		unknown := c.ModeDue == 0 || c.ModeDue <= at
		end := c.ModeDue
		if unknown {
			end = c.ModeAt + 30000
		}
		add(perlonResonanceKey, name, c.ModeAt, end, unknown)
	}
	return items
}

func (r *nativeReminderRuntime) selectedPerlonHealth(target *nativeReminderEntity) *nativeTargetHealthOverlayItem {
	s := r.settings.BossMechanics.PerlonHealth
	boss := r.entities[r.perlon.BossID]
	if !s.Enabled || boss == nil || !boss.Active || boss.Defeated || target == nil || !target.Known || !target.Active || target.Defeated || !target.HealthKnown || target.RaceID < 193811 || target.RaceID > 193813 || target.AppearedAt < boss.AppearedAt {
		return nil
	}
	current, maximum := target.CurrentHealth, target.MaximumHealth
	if current <= 0 || maximum <= 0 || math.IsNaN(current) || math.IsNaN(maximum) || math.IsInf(current, 0) || math.IsInf(maximum, 0) {
		return nil
	}
	phase := "幻影机制"
	if r.perlon.Wave == 1 {
		phase = "90% 幻影"
	} else if r.perlon.Wave >= 2 {
		phase = "50% 幻影"
	}
	return &nativeTargetHealthOverlayItem{Kind: "perlon-phantom", EntityID: target.ID, Name: "佩隆的幻影", PhaseLabel: phase, CurrentHealth: math.Min(current, maximum), MaximumHealth: maximum, X: s.X, Y: s.Y, ScalePercent: s.ScalePercent, OpacityPercent: s.OpacityPercent}
}
