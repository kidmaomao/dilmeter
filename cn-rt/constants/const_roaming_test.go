package constants

import (
	"strings"
	"testing"
)

func TestGameServerRoamingFilterFollowsConfiguredDirectConnections(t *testing.T) {
	if err := ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}); err != nil {
		t.Fatal(err)
	}
	UseActiveConnection("211.147.76.32", "11021", "43210")

	filter, roaming := GameServerRoamingFilter()
	if !roaming {
		t.Fatal("configured direct connection should use a channel-roaming filter")
	}
	if !strings.Contains(filter, "src net 211.147.76.0/24") || strings.Contains(filter, "dst port 43210") {
		t.Fatalf("unexpected roaming filter: %s", filter)
	}
}

func TestGameServerRoamingFilterKeepsProxyConnectionExact(t *testing.T) {
	if err := ConfigureGameServer("211.147.76.0/24", []string{"11020", "11021", "11023"}); err != nil {
		t.Fatal(err)
	}
	UseActiveConnection("127.0.0.1", "18000", "43210")

	filter, roaming := GameServerRoamingFilter()
	if roaming {
		t.Fatal("proxy connection outside configured server range must stay exact")
	}
	for _, want := range []string{"src host 127.0.0.1", "src port 18000", "dst port 43210"} {
		if !strings.Contains(filter, want) {
			t.Fatalf("filter %q does not contain %q", filter, want)
		}
	}
}

func TestActiveConnectionClearedByServerReconfiguration(t *testing.T) {
	if err := ConfigureGameServer("211.147.76.0/24", []string{"11020"}); err != nil {
		t.Fatal(err)
	}
	UseActiveConnection("211.147.76.32", "11020", "43210")
	if _, _, _, ok := ActiveConnection(); !ok {
		t.Fatal("active connection was not stored")
	}
	if err := ConfigureGameServer("61.164.61.0/24", []string{"11021"}); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := ActiveConnection(); ok {
		t.Fatal("server reconfiguration retained a stale active connection")
	}
}
