package constants

import (
	"strings"
	"testing"
)

func TestConfigureGameServerAndMatch(t *testing.T) {
	t.Cleanup(func() {
		if err := ConfigureGameServer(defaultGameServerNetwork, defaultGameServerPorts); err != nil {
			t.Fatal(err)
		}
	})

	ports := []string{"11020", "11021", "11023"}
	if err := ConfigureGameServer("61.164.61.0/24", ports); err != nil {
		t.Fatal(err)
	}
	if !MatchesConfiguredGameServer("61.164.61.42", "11021") {
		t.Fatal("expected 亚特 connection to match")
	}
	if MatchesConfiguredGameServer("211.147.76.42", "11021") {
		t.Fatal("伊鲁夏 connection must not match while 亚特 is selected")
	}
	if MatchesConfiguredGameServer("61.164.61.42", "80") {
		t.Fatal("unexpected port matched")
	}

	filter := GameServerFilter()
	if !strings.Contains(filter, "src net 61.164.61.0/24") || !strings.Contains(filter, "11023") {
		t.Fatalf("unexpected filter: %s", filter)
	}
}

func TestConfigureGameServerRejectsInvalidValues(t *testing.T) {
	for _, test := range []struct {
		network string
		ports   []string
	}{
		{network: "not-a-network", ports: []string{"11020"}},
		{network: "2001:db8::/64", ports: []string{"11020"}},
		{network: "211.147.76.0/24", ports: nil},
		{network: "211.147.76.0/24", ports: []string{"0"}},
		{network: "211.147.76.0/24", ports: []string{"65536"}},
		{network: "211.147.76.0/24", ports: []string{"not-a-port"}},
	} {
		if err := ConfigureGameServer(test.network, test.ports); err == nil {
			t.Fatalf("ConfigureGameServer(%q, %#v) unexpectedly succeeded", test.network, test.ports)
		}
	}
}

func TestUseActiveConnectionNarrowsFilter(t *testing.T) {
	t.Cleanup(func() {
		if err := ConfigureGameServer(defaultGameServerNetwork, defaultGameServerPorts); err != nil {
			t.Fatal(err)
		}
	})

	if err := ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}); err != nil {
		t.Fatal(err)
	}
	UseActiveConnection("211.147.76.88", "11020", "53001")
	filter := GameServerFilter()
	for _, expected := range []string{"src net 211.147.76.88", "src port ( 11020 )", "dst port 53001"} {
		if !strings.Contains(filter, expected) {
			t.Fatalf("filter %q does not contain %q", filter, expected)
		}
	}
}
