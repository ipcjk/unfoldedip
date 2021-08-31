package satagent

import (
	"log"
	"net"
	"time"
	"unfoldedip/sattypes"
)

// TCPCheck checks a service for a successful tcp connection
func (s *satAgent) TCPCheck(service sattypes.Service) sattypes.ServiceResult {
	log.Println(s.hello(), "TCP Check", service.ToCheck, service.ServiceID)

	// prepare result set
	var sResult sattypes.ServiceResult
	sResult.ServiceID = service.ServiceID

	// generate TCP connection with timeout
	tcpDialer := net.Dialer{Timeout: time.Second * 5}

	conn, err := tcpDialer.Dial("tcp", service.ToCheck)
	if err != nil {
		sResult.Status = sattypes.ServiceDown
		sResult.Message = err.Error()
		return sResult
	}

	err = conn.Close()
	if err != nil {
		log.Println(err)
	}

	sResult.Status = sattypes.ServiceUP
	sResult.Message = "TCP OK"
	return sResult
}
