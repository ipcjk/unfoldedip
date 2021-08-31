package satagent

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
	"unfoldedip/sattypes"
)

// HTTPCheck runs a HTTP Get query against a target
func (s *satAgent) HTTPCheck(service sattypes.Service) sattypes.ServiceResult {
	var expandedMessage string
	log.Println(s.hello(), "HTTP Check", service.ToCheck, service.ServiceID)

	// prepare result set
	var sResult sattypes.ServiceResult
	sResult.ServiceID = service.ServiceID

	// generate HTTP client
	client := http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Second * 5,
	}

	// add path to server url
	request, err := http.NewRequest("GET", service.ToCheck, nil)
	if err != nil {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = err.Error()
		return sResult
	}

	// set user-agent for identification in logfiles
	request.Header.Set("User-Agent", "unfolded ip monitoring agent")

	// do the request
	resp, err := client.Do(request)
	if err != nil {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = err.Error()
		return sResult
	}

	// Status == 0, service is up, but
	// also examine the HTTP statuscode
	switch resp.StatusCode {
	case http.StatusOK, http.StatusAccepted, http.StatusFound, http.StatusMovedPermanently:
		sResult.Status = sattypes.ServiceUP
	default:
		sResult.Status = sattypes.ServiceDown
	}

	// check if we need to expect a certain result inside the body
	if service.Expected != "" {
		// search body
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			if strings.Contains(string(body), service.Expected) {
				expandedMessage = fmt.Sprintf("Text '%s' found", service.Expected)
			} else {
				sResult.Status = sattypes.ServiceDown
				expandedMessage = fmt.Sprintf("Text '%s' NOT found", service.Expected)
			}
		}
	}

	// close response body
	resp.Body.Close()

	sResult.Message = fmt.Sprintf("HTTP Status: %d (%s) %s", resp.StatusCode, http.StatusText(resp.StatusCode),
		expandedMessage)
	return sResult
}
