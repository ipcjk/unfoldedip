package satagent

import (
	"log"
	"unfoldedip/ping"
	"unfoldedip/sattypes"
)

// PingCheck runs icmp ping echo against a target
func (s *satAgent) PingCheck(service sattypes.Service) sattypes.ServiceResult {
	log.Println(s.hello(), "Ping Check", service.ToCheck)

	var r sattypes.ServiceResult = sattypes.ServiceResult{ServiceID: service.ServiceID}
	r.Status = sattypes.ServiceDown

	pinger, err := ping.CreatePing(service.ToCheck, 5)
	if err != nil {
		r.Message = err.Error()
		return r
	}

	// Start the pinger
	stats, err := pinger.Run()
	if err != nil {
		log.Println(err)
		r.Message = err.Error()
		return r
	}

	// no mismatch between send and recv?
	if stats.SendPkts != 0 && stats.RcvdPkts == stats.SendPkts {
		r.Status = sattypes.ServiceUP
		// todo, could also check ping avg time against expected time
	}
	r.Message = stats.ToString()

	return r
}
