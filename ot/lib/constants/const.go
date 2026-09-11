package constants

import (
	"fmt"
	"time"
)

var PCAP_GAMESERVER_FILTER = ""

// kr server
// const _SERVER_MABI_KR = "211.218.233.0/24"
// const _PORT_MABI_KR = "11020 or 11021 or 11023"

// China server defaults.
var ServerIP = "211.147.76.0/24"
var ServerSrcPort = "11020 or 11021 or 11023"
var ServerDstPort = ""

func init() {
	RebuildFilter()
}

// RebuildFilter rebuilds the pcap BPF filter from the current settings.
func RebuildFilter() {
	if ServerSrcPort != "" && ServerDstPort != "" {
		PCAP_GAMESERVER_FILTER = fmt.Sprintf("tcp and src net %s and src port (%s) and dst port (%s)", ServerIP, ServerSrcPort, ServerDstPort)
	} else if ServerSrcPort != "" {
		PCAP_GAMESERVER_FILTER = fmt.Sprintf("tcp and src net %s and src port (%s)", ServerIP, ServerSrcPort)
	} else if ServerDstPort != "" {
		PCAP_GAMESERVER_FILTER = fmt.Sprintf("tcp and src net %s and dst port (%s)", ServerIP, ServerDstPort)
	} else {
		PCAP_GAMESERVER_FILTER = fmt.Sprintf("tcp and src net %s", ServerIP)
	}
}

var SERVER_START_AT = time.Now().Unix()
var SERVER_START_AT_STR = time.Now().Format("2006-01-02_15-04-05")
