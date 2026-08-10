package main

import (
	"syscall"
	"testing"
)

func TestIsLoopbackAddr(t *testing.T) {
	cases := []struct {
		addr string
		want bool
		err  bool
	}{
		{"127.0.0.1:8765", true, false},
		{"localhost:8765", true, false},
		{"[::1]:8765", true, false},
		{":8765", false, false},        // wildcard → all interfaces, NOT loopback
		{"0.0.0.0:8765", false, false}, // explicit wildcard
		{"192.168.1.10:8765", false, false},
		{"example.com:8765", false, false}, // hostname → treated as exposed
		{"garbage", false, true},           // no port → error
	}
	for _, c := range cases {
		got, err := isLoopbackAddr(c.addr)
		if (err != nil) != c.err {
			t.Errorf("isLoopbackAddr(%q) err=%v; wantErr=%v", c.addr, err, c.err)
			continue
		}
		if err == nil && got != c.want {
			t.Errorf("isLoopbackAddr(%q) = %v; want %v", c.addr, got, c.want)
		}
	}
}

func TestShutdownSignalsIncludeSIGTERM(t *testing.T) {
	var hasInt, hasTerm bool
	for _, s := range shutdownSignals {
		if s == syscall.SIGINT {
			hasInt = true
		}
		if s == syscall.SIGTERM {
			hasTerm = true
		}
	}
	if !hasTerm {
		t.Error("shutdownSignals must include SIGTERM (Docker/K8s/systemd stop with it)")
	}
	if !hasInt {
		t.Error("shutdownSignals should include SIGINT")
	}
}
