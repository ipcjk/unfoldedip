package satanalytics

// satanalytics contains the code to run the analyst
// thread. the analyst thread is reading the satagent events
// from an array or a channel and is then:
// - updating states,
// - pitching the alert messages

import (
	"fmt"
	"log"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unfoldedip/satsql"
	"unfoldedip/sattypes"
)

type serviceTracking struct {
	// currentState as string
	state string
	// keep track of maximum 64 results in memory
	// saved as bitset where 1 = ServiceDown, 0 = ServiceUP
	// when stateHistory is non-balanced
	// we will consider a service transition (down=>up, up=>down)
	// for the last 8 bits
	stateHistory uint64
	lastSeen     time.Time
}

type agentTracking struct {
	lastSeen time.Time
}

// satanalytics object with all necessary information
type satanalytics struct {
	Name              string
	Tracker           map[int64]*serviceTracking
	TrackerMutex      sync.Mutex
	AgentTracker      map[string]*agentTracking
	AgentTrackerMutex sync.Mutex
	H                 sattypes.BaseHandler
	HasSMTPConfig     bool
	ReadMessages      int64
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
		s.Tracker[service.ServiceID] = &serviceTracking{state: service.ServiceState, lastSeen: time.Now()}
	}

	// Load agent nodes
	//agents, err := satsql.ReadAgents(s.H)
	//if err != nil && err != sql.ErrNoRows {
	//	log.Println("Cant load initial configuration", err)
	//	return
	//}
	// Walk every service and create a tracker
	// this is optimistic currently, because we update lastSeen with current time and not with the actual
	// value from the database (sqlite issue)
	//for _, agent := range agents {
	//	s.AgentTracker[agent.SatAgentID] = &agentTracking{lastSeen: time.Now()}
	//}

}

// The dead node detection
// will send mail to admin, if a node did not contact the server for a long period
func (s *satanalytics) deadNodeSwitch() {
	s.AgentTrackerMutex.Lock()

	for i := range s.AgentTracker {
		// everything over 60 minutes is suspicious
		if int(time.Since(s.AgentTracker[i].lastSeen)/time.Second) > 3600 {
			// FIXME, send mail to $x  or raise alert
			log.Println("Node with ID ", i, "seems to be dead ")
		}
	}

	s.AgentTrackerMutex.Unlock()
}

// The dead service thread
// will signal to the golang channel, if a service check has not been seen for a  long period
func (s *satanalytics) deadServiceSwitch() {

	// quickly lock the tracker
	s.TrackerMutex.Lock()

	for i := range s.Tracker {
		// everything over 10 minutes is suspicious
		if int(time.Since(s.Tracker[i].lastSeen)/time.Second) > 600 {
			// pushing a message down the channel?
			// attention, fixme, this could be a deadlock if the channel is FULl?
			sattypes.ResultsChannel <- sattypes.ServiceResult{
				ServiceID:   i,
				Status:      sattypes.ServiceUnknown,
				Message:     "Service is stalled, not any status received in the last 600 seconds",
				Time:        time.Now(),
				TestNode:    "analytics",
				RapidChange: true,
			}
		}
	}

	// Unlock tracker mutex again
	s.TrackerMutex.Unlock()

}

// Run the analytics thread
func (s *satanalytics) Run() {
	// load initial configuration
	s.load()

	// read / wait for channel messages on results
	// result will be stored and then the "state" of the service
	// will be calculated in kind of "quorom" - decision
	idleTimer := time.NewTicker(time.Second * 10)
	for {
		select {
		case r := <-sattypes.ResultsChannel:
			idleTimer.Reset(time.Second * 10)
			s.ReadMessages++
			var sendNotification = false
			if s.H.Debug && r.Status != sattypes.ServiceUP {
				log.Println("Received from", r.TestNode, r.Message)
			}
			// if this is a new service, it is necessary to allocate some memory for tracking
			if _, ok := s.Tracker[r.ServiceID]; !ok {
				if s.H.Debug {
					log.Println("Create new structure for unknown service")
					log.Println(s.Tracker[r.ServiceID])
				}
				s.TrackerMutex.Lock()
				s.Tracker[r.ServiceID] = &serviceTracking{state: ""}
				s.TrackerMutex.Unlock()
			}

			// update lastseen attribute to "now"
			s.Tracker[r.ServiceID].lastSeen = time.Now()
			err := satsql.UpdateServiceLastSeenNow(s.H, r.ServiceID)
			if err != nil {
				log.Println(err)
			}

			// shift a 0, if the service is up
			// shift a 1 if the service is down
			if r.Status == sattypes.ServiceDown {
				s.Tracker[r.ServiceID].stateHistory =
					(s.Tracker[r.ServiceID].stateHistory << 1) | 0x1
			} else if r.Status == sattypes.ServiceUP {
				s.Tracker[r.ServiceID].stateHistory =
					s.Tracker[r.ServiceID].stateHistory << 1
			}

			var changeState bool
			// check if we were down or up for more than four requests
			if (r.Status == sattypes.ServiceDown && s.Tracker[r.ServiceID].stateHistory&0x0F == 0x0F) ||
				(r.Status == sattypes.ServiceUP && s.Tracker[r.ServiceID].stateHistory&0x0F == 0x0) {
				changeState = true
			}

			// possible changeState? From down to up?
			// or RapidChange Event? For example, when hitting a stalled service
			if changeState && r.Status != s.Tracker[r.ServiceID].state || r.RapidChange {
				s.Tracker[r.ServiceID].state = r.Status
				// and also in persistent in DB
				err := satsql.UpdateServiceState(s.H, r.ServiceID, r.Status)
				if err != nil {
					log.Println(err)
				}
				sendNotification = true
				err = satsql.InsertServiceChange(s.H, r)
				if err != nil {
					log.Println(err)
				}
			}
			// sendNotification
			if sendNotification {
				serviceID := strconv.FormatInt(r.ServiceID, 10)
				service, err := satsql.SelectService(s.H, "service_id", serviceID, 0)
				if err == nil {
					if s.HasSMTPConfig && service.ContactGroup != 0 {
						contactGroups, err := satsql.SelectAlertGroup(s.H, "contact_id", fmt.Sprintf("%d", service.ContactGroup))
						if err == nil {
							emails := strings.Split(contactGroups.Emails, ",")
							for i := range emails {
								go func(recipient string) {
									err := s.H.SMTPConfiguration.SendServiceMail(service, r, recipient)
									if err != nil {
										log.Println("SMTP-failed", err)
										log.Println("Don't have working SMTP-configuration for sending alert")
										log.Println("Service changed up/down", s.Tracker[r.ServiceID], r.Status)
									}
								}(emails[i])
							}

						}
					} else {
						log.Println("Don't have working SMTP-configuration for sending alert")
						log.Println("Service changed up/down", s.Tracker[r.ServiceID], r.Status)
						log.Println(service, r)
					}
				}
				if s.H.Debug {
					log.Println("Service changed up/down", s.Tracker[r.ServiceID], r.Status)
				}
			}
		case <-idleTimer.C:
			runtime.GC()
			// Do other work, like searching zombie services
			s.deadServiceSwitch()
			s.keepalive()
		}
	}
}

// Return tracking information for debugging
func (s *satanalytics) GetServicesTrack() map[int64]*serviceTracking {
	return s.Tracker
}
