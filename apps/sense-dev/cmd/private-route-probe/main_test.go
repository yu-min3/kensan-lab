package main

import (
	"context"
	"net"
	"testing"
)

func TestAddressValidationRejectsLoopbackOrUnroutableTargets(t *testing.T) {
	for _, tc := range []struct {
		ip     string
		family int
		want   bool
	}{
		{"192.168.0.113", 4, true}, {"127.0.0.1", 4, false}, {"0.0.0.0", 4, false}, {"8.8.8.8", 4, false},
		{"2001:4860:4860::8888", 6, true}, {"::1", 6, false}, {"fe80::1", 6, false}, {"fd00::1", 6, false},
	} {
		if got := validIP(tc.ip, tc.family); got != tc.want {
			t.Fatalf("%s family %d: got %t", tc.ip, tc.family, got)
		}
	}
}

func TestDirectRefusalIsDifferentFromReachable(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if got := classify(context.Background(), address).Status; got != "reachable" {
		t.Fatalf("open listener: %s", got)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if got := classify(context.Background(), address).Status; got != "direct_refused" {
		t.Fatalf("closed listener: %s", got)
	}
}
