package packet

import (
	"time"

	"gitlab.com/prilus/mabidilmeter/lib/util"
)

var logger = util.NewLogger("packet")

type GamePacket struct {
	At     time.Time
	Sign   uint8
	Length uint32
	Flag   uint8

	// ConnectionEpoch identifies the capture/TCP session that produced this
	// packet. It changes when the game reconnects or switches channel, allowing
	// consumers to discard live state from the previous server connection
	// without clearing accumulated combat history.
	ConnectionEpoch uint64

	// raw packet
	IsShortPacket bool
	ShortBody     []byte

	// normal packet
	Op  OpCode
	Id  uint64
	Msg Message

	// checksum uint32

	RawPacket []byte
}
