package satanalytics

// satanalytics contains the code to run the analyst
// thread. the analyst thread is reading the satagent events
// from an array or a channel and is then:
// - updating states,
// - pitching the alert messages

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unfoldedip/satsql"
	"unfoldedip/sattypes"
)

type serviceTracking struct {
	state string
	// keep track of maximum 64 results in memory
	// saved as bitset where 1 = ServiceDown, 0 = ServiceUP
	// when stateHistory is non-balanced
	// we will consider a service transition (down=>up, up=>down)
	// for the last 8 bits
	stateHistory uint64
}

// satanalytics object with all necessary information
type satanalytics struct {
	Name          string
	Tracker       map[int64]*serviceTracking
	H             sattypes.BaseHandler
	HasSMTPConfig bool
	ReadMessages  int64
}

// keepalive
func (s *satanalytics) keepalive() {
	log.Printf("--- satanalytics thread %s alive ---", s.Name)
}

// keepalive
func (s *satanalytics) GetReadMessages() int64 {
	return s.ReadMessages
}

// CreateSatAnalytics returns an analytics object
func CreateSatAnalytics(name string, H sattypes.BaseHandler) *satanalytics {
	s := satanalytics{Name: name}
	s.Tracker = make(map[int64]*serviceTracking)
	s.H = H
	if len(H.SMTPConfiguration.SmtpSender) != 0 && len(H.SMTPConfiguration.SmtpUser) != 0 &&
		len(H.SMTPConfiguration.SmtpPassword) != 0 && len(H.SMTPConfiguration.SmtpServer) != 0 {
		s.HasSMTPConfig = true
	}
	return &s
}

// load initial state from database
func (s *satanalytics) load() {

	if s.H.DB == nil {
		log.Println("No database, cant load initial configuration")
		return
	}

	// load defined services from database
	services, err := satsql.ReadServices(s.H, 0, "", false)
	if err != nil {
		log.Println("Cant load initial configuration", err)
		return
	}
	// Walk every service and create a tracker
	for _, service := range services {
		s.Tracker[service.ServiceID] = &serviceTracking{state: service.ServiceState}
	}

}

// Run the analytics thread
func (s *satanalytics) Run() {
	// load initial configuration
	s.load()

	// read / wait for channel messages on results
	// result will be stored and then the "state" of the service
	// will be calculated in kind of "quorom" - decision
	for {
		select {
		case r := <-sattypes.ResultsChannel:
			s.ReadMessages++
			var sendNotification = false
			result := r
			if s.H.Debug && r.Status != sattypes.ServiceUP {
				log.Println("Received from", r.TestNode, r.Message)
			}
			// if this is a new service, it is necessary to allocate some memory for tracking
			if _, ok := s.Tracker[result.ServiceID]; !ok {
				if s.H.Debug {
					log.Println("Create new structure for unknown service")
					log.Println(s.Tracker[result.ServiceID])
				}
				s.Tracker[result.ServiceID] = &serviceTracking{state: ""}
			}

			// shift a 0, if the service is up
			// shift a 1 if the service is down
			if result.Status == sattypes.ServiceDown {
				s.Tracker[result.ServiceID].stateHistory =
					(s.Tracker[result.ServiceID].stateHistory << 1) | 0x1
			} else {
				s.Tracker[result.ServiceID].stateHistory =
					s.Tracker[result.ServiceID].stateHistory << 1
			}

			var changeState bool
			// check if we were down or up for more than four requests
			if (result.Status == sattypes.ServiceDown && s.Tracker[result.ServiceID].stateHistory&0x0F == 0x0F) ||
				(result.Status == sattypes.ServiceUP && s.Tracker[result.ServiceID].stateHistory&0x0F == 0x0) {
				changeState = true
			}
			// possible changeState? From down to up?
			if changeState && result.Status != s.Tracker[result.ServiceID].state {
				s.Tracker[result.ServiceID].state = result.Status
				// and also in persistent in DB
				err := satsql.UpdateServiceState(s.H, result.ServiceID, result.Status)
				if err != nil {
					log.Println(err)
				}
				sendNotification = true
				err = satsql.InsertServiceChange(s.H, result)
				if err != nil {
					log.Println(err)
				}
			}
			// sendNotification
			if sendNotification {
				serviceID := strconv.FormatInt(result.ServiceID, 10)
				service, err := satsql.SelectService(s.H, "service_id", serviceID, 0)
				if err == nil {
					if s.HasSMTPConfig && service.ContactGroup != 0 {
						contactGroups, err := satsql.SelectAlertGroup(s.H, "contact_id", fmt.Sprintf("%d", service.ContactGroup))
						if err == nil {
							emails := strings.Split(contactGroups.Emails, ",")
							for i := range emails {
								go func(recipient string) {
									err := s.H.SMTPConfiguration.SendServiceMail(service, result, recipient)
									if err != nil {
										log.Println("SMTP-failed", err)
										log.Println("Don't have working SMTP-configuration for sending alert")
										log.Println("Service changed up/down", s.Tracker[result.ServiceID], result.Status)
									}
								}(emails[i])
							}

						}
					} else {
						log.Println("Don't have working SMTP-configuration for sending alert")
						log.Println("Service changed up/down", s.Tracker[result.ServiceID], result.Status)
						log.Println(service, result)
					}
				}
				if s.H.Debug {
					log.Println("Service changed up/down", s.Tracker[result.ServiceID], result.Status)
				}
			}

		case <-time.After(time.Second * 10):
			s.keepalive()
		}
	}
}
