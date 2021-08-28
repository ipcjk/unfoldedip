package satagent

// satagent contains the code to run the checking client
// that is embedded inside the server, but could also run
// on a remote site

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
	"unfoldedip/sattypes"
)

// satAgent object with all necessary information
type satAgent struct {
	// access key that is loaded from command line
	SatKey string
	// URL for the server
	SatServerURL string
	// name of this client
	SatName string
	// Location of this client
	SatLocation string
	// Fixed location?
	SatLocationOnly bool
	// array with service configuration
	satServices []sattypes.Service
	// mutex to protect satServices array
	satServicesMutex sync.Mutex
	// bool, that shows, if configuration has been loaded
	satServerLoaded bool
	// saving the next service check interval
	serviceInterval map[int64]int
	// different values, when to block and
	// when to refresh configuration
	blockTime             time.Duration
	blockSeconds          int
	refreshSeconds        int
	refreshSecondsDefault int
	// will be used to protect the results array
	// while collecting results before posting
	// to server
	resultsMutex sync.Mutex
	results      []sattypes.ServiceResult
	// debug mode is on?
	debug bool
}

// keepalive
func (s *satAgent) keepalive() {
	// log.Printf("--- satagent %s (%s) alive ---", s.SatName, s.SatLocation)
}

// CreateSatAgent creates a satAgent thread and returns an object
func CreateSatAgent(url, name, location string, locationOnly bool, H sattypes.BaseHandler) *satAgent {
	s := satAgent{}
	s.SatKey = H.SatKey
	s.SatName = name
	s.SatLocation = location
	s.SatLocationOnly = locationOnly
	s.SatServerURL = url + "/agents/"
	s.blockSeconds = 1
	s.blockTime = time.Second * time.Duration(s.blockSeconds)
	s.refreshSecondsDefault = 45
	s.refreshSeconds = s.refreshSecondsDefault
	s.serviceInterval = make(map[int64]int)
	s.debug = H.Debug
	return &s
}

// print prefix for logging
func (s *satAgent) hello() string {
	return fmt.Sprintf("--- satagent %s (%s): ", s.SatName, s.SatLocation)
}

// print startup message of the day
func (s *satAgent) motd() {
	log.Printf("%s Pull tests with access key %s from %s", s.hello(),
		s.SatKey, s.SatServerURL)
}

// pull configuration from server
func (s *satAgent) pullServerConfiguration() error {
	client := http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Second * 20,
	}
	// add path to server url
	request, err := http.NewRequest("GET", s.SatServerURL+"config", nil)
	if err != nil {
		return err
	}

	// set access key / token
	request.Header.Set("agent-key", s.SatKey)
	// transport my and location inside http header
	request.Header.Set("agent-location", s.SatLocation)
	request.Header.Set("agent-name", s.SatName)

	if s.SatLocationOnly {
		request.Header.Set("agent-onlylocation", "YES")
	}

	// do the request
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	// Read response and try to parse
	var agentServices []sattypes.Service
	err = json.NewDecoder(resp.Body).Decode(&agentServices)
	if err != nil {
		log.Println(err)
		return err
	}

	// Close body
	err = resp.Body.Close()
	if err != nil {
		log.Println(err)
	}

	if len(agentServices) == 0 {
		log.Println(s.hello() + "No services found")
		time.Sleep(time.Second * 10)
	}

	// lock configuration by holding satServicesMutex
	// replace live configured services
	// this needs to be tricky,
	// because we need to retain the NextInterval for existing check, else the agent
	// could miss certain intervals, this is very sensitive from performance view
	// set starting next interval for the check
	s.satServicesMutex.Lock()

	for i := range agentServices {
		// service is old, retain old next interval
		if val, ok := s.serviceInterval[agentServices[i].ServiceID]; ok {
			agentServices[i].NextInterval = val
			if s.debug {
				log.Println("Retain", val, "for ", agentServices[i].ServiceID)
			}
		} else {
			// take default
			agentServices[i].NextInterval = agentServices[i].Interval
		}
	}

	// overwrite services
	s.satServices = nil
	s.satServices = agentServices
	s.satServerLoaded = true
	// unlockMutex
	s.satServicesMutex.Unlock()

	// some helpful message
	log.Println(s.hello(), "reloaded / refreshing services", s.satServices)

	return nil
}

// postResults sends the array of results back to the client
func (s *satAgent) postResults() {

	// copy results, then nil/empty the slice, unlock the mutex
	s.resultsMutex.Lock()
	localResults := s.results
	s.results = nil
	s.resultsMutex.Unlock()

	if len(localResults) == 0 {
		log.Println("Nothing to post back")
		return
	}

	// debug prints
	if s.debug {
		log.Println(s.hello(), "POST ", localResults)
	}

	/* post back to satserver */
	client := http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Second * 10,
	}

	/* generate encoder for result */
	b := new(bytes.Buffer)
	err := json.NewEncoder(b).Encode(localResults)
	if err != nil {
		log.Println(err)
		return
	}

	// add path to server url
	// add json
	request, err := http.NewRequest("POST", s.SatServerURL+"results", b)
	if err != nil {
		log.Println(err)
		return
	}

	// set access key / token
	request.Header.Set("agent-key", s.SatKey)
	// transport my and location inside http header
	request.Header.Set("agent-location", s.SatLocation)
	request.Header.Set("agent-name", s.SatName)

	// set type to json
	request.Header.Set("Content-Type", "application/json")

	// do the request
	resp, err := client.Do(request)
	if err != nil {
		log.Println(err)
		return
	}
	err = resp.Body.Close()
	if err != nil {
		log.Println(err)
	}
}

// runServiceCheck decides which service function is to be called
func (s *satAgent) runServiceCheck(service sattypes.Service) {
	var result sattypes.ServiceResult
	if service.Type == "http" {
		result = s.httpCheck(service)
	} else if service.Type == "ping" {
		result = s.pingCheck(service)
	} else if service.Type == "tcp" {
		result = s.tcpCheck(service)
	} else {
		log.Println("Unknown check", result)
	}

	result.TestNode = s.SatLocation
	result.Time = time.Now()

	// add result to local array
	// could also be a channel, that holds the mutex (better performance?)
	s.resultsMutex.Lock()
	s.results = append(s.results, result)
	s.resultsMutex.Unlock()

}

// Run the satagent thread, got called from outside in a non-blocking go thread function
func (s *satAgent) Run() {
	// print hello
	s.motd()
	// sleep for a while, then pull initial configuration
	for !s.satServerLoaded {
		err := s.pullServerConfiguration()
		if err != nil {
			log.Printf("%s Connection to web panel %s\n", s.hello(), err)
			log.Printf("%s Retrying connection\n", s.hello())
			time.Sleep(time.Second * 2)
		} else {
			log.Printf("%s retrieved configuration", s.hello())
		}
	}

	// busy loop, installing timer for waiting for checks
	idleTimer := time.NewTicker(s.blockTime)
	for {
		// send some fancy message, that this thread is running
		s.keepalive()
		select {
		// wait a specific time called blocktime then run pending serviceChecks
		case <-idleTimer.C:
			idleTimer.Reset(s.blockTime)
			// lock mutex as we are working on it
			// for every service that is stored in our slice
			s.satServicesMutex.Lock()
			for i, service := range s.satServices {
				localService := service
				// decrement next check interval with the number of blocked and waited seconds
				s.satServices[i].NextInterval -= s.blockSeconds
				// save next interval for possible configuration change
				s.serviceInterval[s.satServices[i].ServiceID] = s.satServices[i].NextInterval
				// if we reached our threshold, we need to run the check
				if s.satServices[i].NextInterval <= 0 {
					if s.debug {
						log.Println(s.hello(), "service check due", s.satServices[i].ServiceID,
							s.satServices[i].NextInterval)
					}
					// reset the service interval for next check
					s.satServices[i].NextInterval = s.satServices[i].Interval
					// start service check in a go thread, copy the object
					// runServiceCheck will write the result into the results slice
					go s.runServiceCheck(localService)
				}
			}
			s.satServicesMutex.Unlock()
			// try to refresh our configuration around every refreshSecondsDefault
			s.refreshSeconds -= s.blockSeconds
			if s.refreshSeconds <= 0 {
				s.refreshSeconds = s.refreshSecondsDefault
				err := s.pullServerConfiguration()
				if err != nil {
					log.Println(err)
				}
			}
			if len(s.results) >= 1 {
				if s.debug {
					log.Println("Size of results to send back to home is", len(s.results))
				}
				go s.postResults()
			}
		}
	}
}
