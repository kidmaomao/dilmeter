package main

import (
	"math"
	"strconv"
	"strings"
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/event"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

const darkEnergySkillID uint16 = 59047
const holyEnergySkillID uint16 = 59085

func isEnergySkill(id uint16) bool { return id == darkEnergySkillID || id == holyEnergySkillID }

// 135688 is a counted property update, not a flat stat packet. Walk complete
// entries so unrelated keys and removals cannot be mistaken for a gauge.
func parseDarkEnergy(msg packet.Message) (float64, bool, bool) {
	if len(msg) < 2 || msg[0].Type() != packet.MessageElemTypeByte || msg[1].Type() != packet.MessageElemTypeShort {
		return 0, false, false
	}
	count := int(msg[1].Data().(uint16))
	cursor := 2
	var value float64
	found, active := false, false
	for i := 0; i < count; i++ {
		if cursor+2 > len(msg) || msg[cursor].Type() != packet.MessageElemTypeByte || msg[cursor+1].Type() != packet.MessageElemTypeString {
			return 0, false, false
		}
		present := msg[cursor].Data().(uint8)
		key := msg[cursor+1].Data().(string)
		cursor += 2
		if present == 0 {
			if key == "MECLIDVA" {
				value, found, active = 0, true, false
			}
			continue
		}
		if present != 1 || cursor+2 > len(msg) || msg[cursor].Type() != packet.MessageElemTypeByte {
			return 0, false, false
		}
		if key == "MECLIDVA" {
			if msg[cursor].Data().(uint8) != 3 || msg[cursor+1].Type() != packet.MessageElemTypeInt {
				return 0, false, false
			}
			value = float64(msg[cursor+1].Data().(uint32))
			if value > 100 {
				return 0, false, false
			}
			found, active = true, true
		}
		cursor += 2
	}
	return value, active, found
}

func (t *eventPublisher) publishDarkEnergyPacket(p *packet.GamePacket) {
	if !t.acceptsLocalCooldownPacket(p) {
		return
	}
	percent, active, ok := parseDarkEnergy(p.Msg)
	if !ok {
		return
	}
	t.publish(&event.EventSkillEnergy{EventBase: event.EventBase{EventId: event.EventIdSkillEnergy, At: p.At.Unix(), Id: strconv.FormatUint(p.Id, 10)}, SkillId: darkEnergySkillID, Percent: percent, Active: active})
}

func holyEnergyPercent(metadata string) (float64, bool) {
	for _, part := range strings.Split(metadata, ";") {
		if !strings.HasPrefix(part, "MCNPGV:f:") {
			continue
		}
		value, err := strconv.ParseFloat(strings.TrimPrefix(part, "MCNPGV:f:"), 64)
		return value, err == nil && !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 100
	}
	// The initial 1179 packet precedes its first increment and omits MCNPGV.
	return 0, strings.Contains(metadata, "MCNPGIPT:f:")
}

func (runtime *nativeReminderRuntime) energyState(skillID uint16, nowMs int64) (percent float64, observed, active bool) {
	if skillID == darkEnergySkillID {
		return runtime.darkEnergy.Percent, runtime.darkEnergy.Active, runtime.darkEnergy.Active
	}
	if skillID == holyEnergySkillID && runtime.localConditionActiveAt(1179, nowMs) {
		percent, observed = holyEnergyPercent(runtime.entities[runtime.localID].Conditions[1179].Metadata)
		return percent, observed, true
	}
	return 0, false, false
}

func (runtime *nativeReminderRuntime) evaluateEnergySkills(now time.Time) {
	if runtime.energyReady == nil {
		runtime.energyReady = make(map[uint16]bool)
	}
	if runtime.energyGeneration == nil {
		runtime.energyGeneration = make(map[uint16]uint64)
	}
	for _, id := range []uint16{darkEnergySkillID, holyEnergySkillID} {
		rule, exists := runtime.settings.SkillCooldowns.Rules[id]
		progress, observed, active := runtime.energyState(id, now.UnixMilli())
		cd := runtime.skillCooldowns[id]
		ready := exists && rule.Enabled && active && observed && progress >= rule.ProgressThresholdPercent &&
			(id != darkEnergySkillID || (cd.UsedAtMs > 0 && cd.ReadyAtMs <= now.UnixMilli()))
		if ready && !runtime.energyReady[id] {
			runtime.energyGeneration[id]++
			if rule.SoundMode != "none" {
				runtime.playNativeSkillSound(rule)
			}
		}
		runtime.energyReady[id] = ready
	}
}
