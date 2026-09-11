//go:build !windows

package pcaputil

import (
	"context"
	"errors"
	"time"

	"github.com/gopacket/gopacket/pcap"
	"gitlab.com/prilus/mabidilmeter/lib/packet"
)

// FindNic iterates all NICs and returns the first one that yields a game packet.
func FindNic() (string, error) {
	packetWaitTime := time.Second * 5

	nics, err := pcap.FindAllDevs()
	if err != nil {
		logger.Println(err)
		return "", err
	}

	for _, nic := range nics {
		ctx, cancel := context.WithCancel(context.Background())

		r, err := packet.NewGameServerPacketReader(&packet.GameServerPacketReaderOpt{
			Ctx: ctx,
		})
		if err != nil {
			logger.Println("FindNic: NewGameServerPacketReader failed:", err, nic.Name)
			cancel()
			continue
		}

		if err := r.OpenNic(nic.Name); err != nil {
			logger.Println("FindNic: OpenNic failed:", err, nic.Name)
			cancel()
			r.Close()
			continue
		}

		found := false
		select {
		case <-time.After(packetWaitTime):
			logger.Println("FindNic: timeout", nic.Name)
		case <-r.PacketCh():
			found = true
			logger.Println("FindNic: success", nic.Name)
		}

		cancel()
		r.Close()

		if found {
			return nic.Name, nil
		}
	}

	err = errors.New("FindNic: no NIC found with game traffic")
	logger.Println(err)
	return "", err
}
