package packet

import "time"

type iGamePacketPayload interface {
	Type() string
}

var _ iGamePacketPayload = (*gamePacketDataPayload)(nil)

type gamePacketDataPayload struct {
	relSeq uint32
	data   []byte
	at     time.Time
}

func (p *gamePacketDataPayload) Type() string {
	return "gamePacketDataPayload"
}
