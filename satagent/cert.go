package satagent

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"
	"unfoldedip/sattypes"
)

// TLSCertCheck runs a TLS dial against a target and checks the certificate chain
func (s *satAgent) TLSCertCheck(service sattypes.Service) sattypes.ServiceResult {
	// prepare result set
	var sResult sattypes.ServiceResult
	sResult.Status = sattypes.ServiceUP
	sResult.ServiceID = service.ServiceID

	// build up tls connection with TCP
	conn, err := tls.Dial("tcp", service.ToCheck, nil)
	if err != nil {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = err.Error()
		return sResult
	}

	// split host and port path for hostname verification
	hostPort := strings.Split(service.ToCheck, ":")
	if len(hostPort) == 0 {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = "Hostname could not be parsed"
		return sResult
	}

	// do the verify
	err = conn.VerifyHostname(hostPort[0])
	if err != nil {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = "Hostname verification failed" + err.Error()
		return sResult
	}

	// Now loop and verify the certs, collect all messages inside expendedMessage
	var expandedMessage string
	var timeNow = time.Now().Unix()
	for _, p := range conn.ConnectionState().PeerCertificates {
		tempTime := p
		if p.NotAfter.Unix() <= timeNow {
			expandedMessage += fmt.Sprintf("Expired: Subject %s %v\n", p.Subject, p.NotAfter.Format(time.RFC850))
			sResult.Status = sattypes.ServiceDown
		} else if tempTime.NotAfter.Sub(time.Now()).Hours() <= 168 {
			expandedMessage += fmt.Sprintf("Expiring Soon: Subject %s %v\n", p.Subject, p.NotAfter.Format(time.RFC850))
			sResult.Status = sattypes.ServiceDown
		} else {
			expandedMessage += fmt.Sprintf("Ok: Subject %s %v\n", p.Subject, p.NotAfter.Format(time.RFC850))
		}
	}

	// final concat for the status message (default service up)
	sResult.Message = expandedMessage
	return sResult
}
