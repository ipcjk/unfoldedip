package ping_test

import (
	"log"
	"testing"
	"unfoldedip/ping"
)

// Test Ping Check for IPv4
func TestPingIPv4(t *testing.T) {
	p, err := ping.CreatePing("127.0.0.1", 3)
	if err != nil {
		log.Fatal(err)
	}
	stats, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	if stats.SendPkts != stats.RcvdPkts {
		t.Errorf("Send %d packets, but only received %d", stats.SendPkts, stats.RcvdPkts)
	}

	if stats.ErrPkts != 0 {
		t.Errorf("%d error packets on localhost ping", stats.ErrPkts)
	}
}

// Test Ping Check for IPv6 localhost address
func TestPingIPv6(t *testing.T) {
	p, err := ping.CreatePing("::1", 3)
	if err != nil {
		log.Fatal(err)
	}
	stats, err := p.Run()
	if err != nil {
		log.Fatal(err)
	}

	if stats.SendPkts != stats.RcvdPkts {
		t.Errorf("Send %d packets, but only received %d", stats.SendPkts, stats.RcvdPkts)
	}

	if stats.ErrPkts != 0 {
		t.Errorf("%d error packets on localhost ping", stats.ErrPkts)
	}
}
