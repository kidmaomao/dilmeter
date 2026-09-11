package pcaputil

import (
	"fmt"

	"github.com/gopacket/gopacket/pcap"
	"gitlab.com/prilus/mabidilmeter/lib/util"
)

var logger = util.NewLogger("pcaputil")

type NicInfo struct {
	Idx         int
	Name        string
	Description string
	Ip          string
}

func NicList() ([]*NicInfo, error) {
	nics, err := pcap.FindAllDevs()
	if err != nil {
		return nil, fmt.Errorf("FindAllDevs failed: %w", err)
	}

	list := make([]*NicInfo, 0, len(nics))
	for i, nic := range nics {
		ipStr := "unknownAddress"
		if len(nic.Addresses) > 0 {
			ipStr = nic.Addresses[0].IP.String()
		}
		list = append(list, &NicInfo{
			Idx:         i,
			Name:        nic.Name,
			Description: nic.Description,
			Ip:          ipStr,
		})
	}
	return list, nil
}

func (t *NicInfo) String() string {
	return fmt.Sprintf("idx: %d, name: %s, description: %s, ip: %s", t.Idx, t.Name, t.Description, t.Ip)
}
