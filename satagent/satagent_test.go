package satagent

import (
	"runtime"
	"testing"
	"unfoldedip/sattypes"
)

// Test Service Check HTTP
func TestServiceCheckHTTP(t *testing.T) {
	// empty object
	s := satAgent{}

	// HTTP Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		OwnerID:   0,
		Type:      "http",
		ToCheck:   "https://www.google.com",
		Expected:  "Google",
	}
	result := s.httpCheck(service)

	if result.Status != sattypes.ServiceUP {
		t.Errorf("Status of http check for google.com returned %s", result.Status)
	}
}

// Test Service Check TCP
func TestServiceCheckTCP(t *testing.T) {
	// empty object
	s := satAgent{}

	// TCP Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		Type:      "TCP",
		ToCheck:   "www.google.com:80",
	}
	result := s.tcpCheck(service)

	if result.Status != sattypes.ServiceUP {
		t.Errorf("Status of TCP check for google.com returned %s", result.Status)
	}
}

// Test Service Check Ping
func TestServiceCheckPing(t *testing.T) {
	// empty object
	s := satAgent{}

	// Ping Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		Type:      "PING",
		ToCheck:   "www.google.com",
	}

	// Ping only supported on Linux and Darwin
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		result := s.pingCheck(service)
		if result.Status != sattypes.ServiceUP {
			t.Errorf("Status of Ping check for google.com returned %s", result.Status)
		}
	}

}
