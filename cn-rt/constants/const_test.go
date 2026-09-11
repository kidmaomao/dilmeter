//go:build dilmeter_rt

package constants

import (
	"strings"
	"testing"
)

func TestWinDivertGameServerFilterForNetwork(t *testing.T) {
	if err := ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}); err != nil {
		t.Fatal(err)
	}
	filter, err := WinDivertGameServerFilter()
	if err != nil {
		t.Fatal(err)
	}
	wants := []string{
		"inbound and ip and tcp",
		"tcp.PayloadLength > 0",
		"remoteAddr >= 211.147.76.0",
		"remoteAddr <= 211.147.76.255",
		"remotePort == 11020",
		"remotePort == 11021",
		"remotePort == 11023",
	}
	for _, want := range wants {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func TestWinDivertGameServerFilterForActiveConnection(t *testing.T) {
	if err := ConfigureGameServer("61.164.61.0/24", []string{"11020", "11021", "11023"}); err != nil {
		t.Fatal(err)
	}
	UseActiveConnection("61.164.61.42", "11021", "43210")
	filter, err := WinDivertGameServerFilter()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(filter, "remoteAddr == 61.164.61.42") || !strings.Contains(filter, "remotePort == 11021") {
		t.Fatalf("unexpected active filter: %s", filter)
	}
	if strings.Contains(filter, "43210") {
		t.Fatalf("local port must not be part of the route-mode filter: %s", filter)
	}
}
