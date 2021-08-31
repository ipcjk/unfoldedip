package main_test

import (
	"database/sql"
	"html/template"
	"testing"
	"time"
	"unfoldedip/satanalytics"
	"unfoldedip/sattypes"
)

// Test web application templates
func TestTemplates(t *testing.T) {
	templates := []string{
		"base.html",
		"service_add.html",
		"alertgroups.html", "alertgroup_add.html",
		"profile.html", "register.html", "login.html",
		"services.html", "service_add.html",
	}
	for _, v := range templates {
		temp, err := template.New("test").ParseFiles("templates/" + v)
		if err != nil {
			t.Errorf("Broken template found: %s", v)
			t.Error(err)
		} else {
			t.Logf("template %s OK", v)
			t.Logf(temp.DefinedTemplates())
		}
	}
}

// Test SMTP
func TestSMTPConnection(t *testing.T) {
	var SMTPConfig sattypes.SMTPConfiguration
	SMTPConfig.SmtpServer = "icmp.info:25"
	SMTPConfig.SmtpUser = "unfolded"
	SMTPConfig.SmtpPassword = "jg4u4huru"
	SMTPConfig.SmtpSender = "unfolded@icmp.info"
	SMTPConfig.SendServiceMail(sattypes.Service{ServiceID: 100, Type: "ping", Name: "Test-Service", ServiceState: sattypes.ServiceUP},
		sattypes.ServiceResult{ServiceID: 100, Message: "OK"}, "@")
	SMTPConfig.SendServiceMail(sattypes.Service{ServiceID: 100, Type: "ping", Name: "Test-Service", ServiceState: sattypes.ServiceDown},
		sattypes.ServiceResult{ServiceID: 100, Message: "NOT OK"}, "@")
}

// Test sat analytics thread
func TestAnalyticsThread(t *testing.T) {
	// var error
	var err error
	// Create a basehandler
	var BaseHandler sattypes.BaseHandler

	// open DB
	BaseHandler.DB, err = sql.Open("sqlite", "unfolded-test.sqlite")
	if err != nil {
		t.Errorf("database issue: %s", err)
	}

	// create Channel
	sattypes.ResultsChannel = make(chan sattypes.ServiceResult, 100)
	// send message to analytics
	sattypes.ResultsChannel <- sattypes.ServiceResult{ServiceID: 99, Status: sattypes.ServiceUP, Message: "OK"}
	// create Analytics thread
	satAnalytics := satanalytics.CreateSatAnalytics("main", BaseHandler)

	time.Sleep(time.Second * 1)
	go satAnalytics.Run()
	time.Sleep(time.Second * 3)
	var messagesRead = satAnalytics.GetReadMessages()
	if messagesRead != 1 {
		t.Errorf("Read Messages shall be %d, but it is %d", 1, messagesRead)
	}
	defer BaseHandler.DB.Close()

	tracker := satAnalytics.GetServicesTrack()
	// service shall exist at this time
	var ok bool
	if _, ok = tracker[99]; !ok {
		t.Errorf("Tracker for Service ID %d does not exist", 99)
	}

}
