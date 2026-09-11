package main

import (
	"reflect"
	"testing"
)

func TestNormalizeGameServerNetwork(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "single IPv4", input: "203.0.113.25", want: "203.0.113.25/32"},
		{name: "CIDR is canonicalized", input: "211.147.76.99/24", want: "211.147.76.0/24"},
		{name: "whitespace", input: " 61.164.61.0/24 ", want: "61.164.61.0/24"},
		{name: "empty", input: "", wantErr: true},
		{name: "invalid", input: "not-an-ip", wantErr: true},
		{name: "IPv6", input: "2001:db8::1", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := normalizeGameServerNetwork(test.input)
			if test.wantErr {
				if err == nil {
					t.Fatalf("normalizeGameServerNetwork(%q) unexpectedly succeeded: %q", test.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("normalizeGameServerNetwork(%q): %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("normalizeGameServerNetwork(%q) = %q, want %q", test.input, got, test.want)
			}
		})
	}
}

func TestNormalizeGameServerPorts(t *testing.T) {
	got, err := normalizeGameServerPorts([]string{"11020, 11021", "11020；11023"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"11020", "11021", "11023"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeGameServerPorts() = %#v, want %#v", got, want)
	}

	for _, ports := range [][]string{
		nil,
		{""},
		{"0"},
		{"65536"},
		{"abc"},
	} {
		if _, err := normalizeGameServerPorts(ports); err == nil {
			t.Fatalf("normalizeGameServerPorts(%#v) unexpectedly succeeded", ports)
		}
	}
}

func TestConfiguredGameServerFallsBackFromInvalidConfig(t *testing.T) {
	got := configuredGameServer(config{
		GameServerNetwork: "invalid",
		GameServerPorts:   []string{"70000"},
	})
	if got.Network != defaultGameServerNetwork {
		t.Fatalf("network = %q, want default %q", got.Network, defaultGameServerNetwork)
	}
	if !reflect.DeepEqual(got.Ports, defaultGameServerPorts) {
		t.Fatalf("ports = %#v, want default %#v", got.Ports, defaultGameServerPorts)
	}
}
