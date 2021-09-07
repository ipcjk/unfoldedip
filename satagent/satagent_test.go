package satagent_test

import (
	"net/http"
	"runtime"
	"testing"
	"time"
	"unfoldedip/satagent"
	"unfoldedip/sattypes"
)

// Test Cert
func TestServiceCert(t *testing.T) {
	// empty object
	s := satagent.CreateSatAgent("", "", "", false, sattypes.BaseHandler{})

	// HTTP Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		OwnerID:   0,
		Type:      "tls",
		ToCheck:   "google.com:443",
		Expected:  "Google",
	}
	result := s.TLSCertCheck(service)

	if result.Status != sattypes.ServiceUP {
		t.Errorf("Status of cert check for google.com returned %s: %s", result.Status, result.Message)
	}
}

// Test Service Check HTTP
func TestServiceCheckHTTP(t *testing.T) {
	// empty object
	s := satagent.CreateSatAgent("", "", "", false, sattypes.BaseHandler{})

	// HTTP Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		OwnerID:   0,
		Type:      "http",
		ToCheck:   "https://www.google.com",
		Expected:  "Google",
	}
	result := s.HTTPCheck(service)

	if result.Status != sattypes.ServiceUP {
		t.Errorf("Status of http check for google.com returned %s", result.Status)
	}
}

// Test Service Check TCP
func TestServiceCheckTCP(t *testing.T) {
	// empty object
	s := satagent.CreateSatAgent("", "", "", false, sattypes.BaseHandler{})

	// TCP Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		Type:      "TCP",
		ToCheck:   "www.google.com:80",
	}
	result := s.TCPCheck(service)

	if result.Status != sattypes.ServiceUP {
		t.Errorf("Status of TCP check for google.com returned %s", result.Status)
	}
}

// Test Service Check Ping
func TestServiceCheckPing(t *testing.T) {
	// empty object
	s := satagent.CreateSatAgent("", "", "", false, sattypes.BaseHandler{})

	// Ping Check
	service := sattypes.Service{
		ServiceID: 99,
		Name:      "Mock-Check",
		Type:      "PING",
		ToCheck:   "www.google.com",
	}

	// Ping only supported on Linux and Darwin
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		result := s.PingCheck(service)
		if result.Status != sattypes.ServiceUP {
			t.Errorf("Status of Ping check for google.com returned %s", result.Status)
		}
	}

}

// Test agent thread, run a service check
func TestAgentThread(t *testing.T) {
	// Basehandler
	var BaseHandler sattypes.BaseHandler
	BaseHandler.SatKey = "93812"

	// mocking channel
	var mockChannel = make(chan struct{})

	// Sample OK http, writes something in the mock channel
	http.HandleFunc("/agents/config", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("agent-name") == "agent" {
			mockChannel <- struct{}{}
		}
		writer.WriteHeader(http.StatusOK)
	})
	http.HandleFunc("/agents/results", func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("agent-name") == "agent" {
			mockChannel <- struct{}{}
		}
		writer.WriteHeader(http.StatusOK)
	})

	// start mock HTTP
	go http.ListenAndServe("localhost:55543", nil)

	// create and start thread
	s := satagent.CreateSatAgent("http://localhost:55543", "agent", "location", false, BaseHandler)
	time.Sleep(time.Second * 2)
	go s.Run()

	select {
	case <-mockChannel:
		break
	case <-time.After(4 * time.Second):
		t.Error("Timeout waiting for satagent")
	}
}
