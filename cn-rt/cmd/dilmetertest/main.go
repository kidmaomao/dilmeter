package main

import (
	"context"
	"fmt"
	"os"

	"gitlab.com/prilus/mabidilmeter/lib/packet"
	"gitlab.com/prilus/mabidilmeter/lib/pcaputil"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

var logger = util.NewLogger("dilmeter")

func main() {
	nicName := ""

	if len(os.Args) > 1 {
		nicName = os.Args[1]
	}

	if nicName == "" {
		found, err := pcaputil.FindNic()
		if err != nil {
			logger.Fatalln("FindNic failed:", err)
		}

		nicName = found
	}

	logger.Println("nicName:", nicName)

	entityMap := make(map[uint64]*packet.EntityInfo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
		Ctx: ctx,
	})
	if err != nil {
		logger.Fatalln("NewGameServerPacketReader failed:", err)
	}

	if _, err := os.Stat(nicName); err == nil {
		// pcap file
		if err := r.OpenFile(nicName); err != nil {
			logger.Fatalln("OpenFile failed:", err)
		}
	} else if err := r.OpenNic(nicName); err != nil {
		logger.Fatalln("OpenNic failed:", err)
	}

	for p := range r.PacketCh() {
		debug := false

		switch p.Op {

		// short packet
		case 0:
			continue

		case packet.OpcodeEntityAppear:
			// entity appears
			entity, err := packet.ParseEntityAppearPacket(p.Msg)
			if err != nil {
				logger.Println("ParseEntityAppearPacket failed:", err)
				continue
			}

			if entity != nil {
				entityMap[entity.Id] = entity
			}

			continue

		case packet.OpcodeEntityDisappear:
			// entity disappears
			// @TODO: 처리 로직 필요
			continue

		case packet.OpcodeCreatureBodyUpdate:
			// creature body update
			continue

		case packet.OpcodeItemAppear:
			// item appears
			continue

		case packet.OpcodeItemDisappear:
			// item disappears
			continue

		case packet.OpcodeChat:
			// chat
			continue

		case packet.OpcodeNotice:
			// notice
			continue

		case packet.OpcodeUnknownWarp:
			// unknown warp
			continue

		case packet.OpcodeEntitiesAppear:
			// entities appear
			entities, err := packet.ParseEntitiesAppearPacket(p)
			if err != nil {
				logger.Println("ParseEntitiesAppearPacket failed:", err)
				continue
			}

			for _, entity := range entities {
				entityMap[entity.Id] = entity
			}
			continue

		case packet.OpcodeEntitiesDisappear:
			// entities disappear
			// @TODO: 처리 로직 필요
			continue

		case packet.OpcodeIsNowDead:
			// is now dead
			continue

		case packet.OpcodeItemDurabilityUpdate:
			// item durability update
			continue

		case packet.OpcodeForceWalk:
			// force walk
			continue

		case packet.OpcodeFlying:
			// flying
			continue

		case packet.OpcodeChangeStanceRes:
			// change stance res
			// 내 전투/일상 상태 변경 완료
			continue

		case packet.OpcodeChangeStance:
			// change stance
			// 전투/일상 상태 변경
			continue

		case packet.OpcodeStatUpdatePrivate:
			// stat update private
			// 내 상태 업데이트
			continue

		case packet.OpcodeStatUpdatePublic:
			// stat update public
			continue

		case 0x7534:
			// entity 관련일듯? byte만 잔뜩
			continue

		case packet.OpcodeCombatTargetUpdate:
			// combat target update
			// 일상 상태로 변경시 리셋 날라옴
			continue

		case packet.OpcodeSetCombatTarget:
			// set combat target
			// ?
			continue

		case packet.OpcodeSetFinisher:
			// set finisher
			// id 몬스터 -> msg[0] 막타
			continue

		case packet.OpcodeSetFinisher2:
			// set finisher2
			continue

		case packet.OpcodeCombatActionPack:
			// combatactions
			pack, err := packet.ParseCombatActionPackPacket(p)
			if err != nil {
				logger.Println("ParseCombatActionPackPacket failed:", err)
				continue
			}

			attackerName := ""
			attackSkillId := uint16(0)
			targetName := ""
			damage := float32(0)

			for i, v := range pack.SubPackets {
				_ = i
				// logger.Println("sub packet", i, v.Hit != nil, v.Attacker != nil)
				// logger.Printf("base %+v", v)
				// if v.Hit != nil {
				// 	logger.Printf("hit %+v", v.Hit)
				// }

				// if v.Attacker != nil {
				// 	logger.Printf("attacker %+v", v.Attacker)
				// }

				if v.Hit == nil {
					// 공격자
					attackerName = fmt.Sprintf("entityId:%x", v.EntityId)
					if entity := entityMap[v.EntityId]; entity != nil {
						attackerName = entity.Name
					}
					attackSkillId = v.SkillId
				} else {
					// 방어자
					targetName = fmt.Sprintf("entityId:%x", v.EntityId)
					if entity := entityMap[v.EntityId]; entity != nil {
						targetName = entity.Name
					}

					damage = v.Hit.Damage
				}

			}

			logger.Println("*", attackerName, "->", targetName, "damage", damage, "skill", attackSkillId)

			continue

		case packet.OpcodeCombatAttackRes:
			// combat attack res
			continue

		case packet.OpcodeEffectDelayed:
			// effect
			continue

		case packet.OpcodeConditionUpdate:
			// condition update
			continue

		case 0xa43c:
			// party window update
			continue

		case 0xaf63:
			// 지정 pc 관련 패킷
			continue

		case 0x1d4c3:
			// ngs
			continue

		case packet.OpcodeWalking:
			// walk
			continue

		case packet.OpcodeRunning:
			// run
			continue

		case packet.OpcodeEffect:
			// effect
			continue

		case packet.OpcodeSharpMind:
			// sharp mind
			continue
		}

		if debug {
			logger.Printf("packet op %s id %x", p.Op, p.Id)
			for i, msg := range p.Msg {
				logger.Println("* msg", i, msg.Type(), msg.String())
			}
		}
	}
}
