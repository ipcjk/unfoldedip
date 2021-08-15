package ping

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
)

const darwin_pktstats = "%d packets transmitted, %d packets received, %f%% packet loss"
const darwin_pktdupstats = "%d packets transmitted, %d packets received, +%d duplicates, %f%% packet loss"
const darwin_rttstats = "round-trip min/avg/max/stddev = %f/%f/%f/%f ms"
const darwin_rttstats6 = "round-trip min/avg/max/std-dev = %f/%f/%f/%f ms"
const linux_pktstats = "%d packets transmitted, %d received, %f%% packet loss, time"
const linux_pktstats_error = "%d packets transmitted, %d received, +%d errors, %f%% packet loss, time"
const linux_pktdupstats = "%d packets transmitted, %d received, +%d duplicates, %f%% packet loss, time"
const linux_rttstats = "rtt min/avg/max/mdev = %f/%f/%f/%f ms"

// Pinger
type Pinger struct {
	ExecPath  string
	Hostname  string
	IPVersion int
	OS        string
	ToSend    int
	wout      *bytes.Buffer
	werr      *bytes.Buffer
}

// Pinger stats
type pingStats struct {
	SendPkts, RcvdPkts, DuplPkts, ErrPkts int
	Loss                                  float32
	RttMin, RttAvg, RttMax, StdDev        float32
}

// ToString returns pingStats as pretty formatted string
func (ps pingStats) ToString() string {
	return fmt.Sprintf("%d/%d/%d%d transmitted/received/duplicates/errors, "+
		"%.2f%%loss, rtt %.2f min/ %.2f avg/ %.2f max/ %.2f stddev", ps.SendPkts, ps.RcvdPkts, ps.DuplPkts,
		ps.ErrPkts, ps.Loss, ps.RttMin, ps.RttAvg, ps.RttMax, ps.StdDev)
}

// CreatePing creates and returns Ping object
func CreatePing(hostname string, toSendPkts int) (*Pinger, error) {

	// new object init
	var p = Pinger{ToSend: toSendPkts, Hostname: hostname, IPVersion: 4, OS: runtime.GOOS}
	// creates some buffer
	p.wout = new(bytes.Buffer)
	p.werr = new(bytes.Buffer)

	// check if hostname is an ipv4|6 address
	// so we can redirect to darwin and the ping6 tool
	// todo if hostname is only showing to ipv6, we also need to consider
	// flagging to ipv6 for the bsd/darwin tool
	if net.ParseIP(hostname).To4() == nil {
		if net.ParseIP(hostname).To16() != nil {
			p.IPVersion = 6
		}
	}

	// depending on OS type, we need to select certain kind of pings
	switch p.OS {
	case "linux":
		// Linux ping has dual capabilities (ipv4 and ipv6)
		p.ExecPath = "/bin/ping"
	case "darwin":
		// darwin has a non compatible ping version for ipv6 (freebsd)
		p.ExecPath = "/bin/ping"
		// if ipv6, we need to switch
		if p.IPVersion == 6 {
			p.ExecPath = "/sbin/ping6"
		} else {
			p.ExecPath = "/sbin/ping"
		}
	default:
		return &Pinger{}, fmt.Errorf("Unsupported OS for ping check: %s", runtime.GOOS)
	}

	// return pinger
	return &p, nil

}

// CreatePing creates and returns Ping object
func (p *Pinger) Run() (pingStats, error) {
	// error and return value
	var err error
	var stats pingStats

	// slice for saving parameters to the ping program
	var pingArgs []string

	// add arguments
	pingArgs = append(pingArgs, "-c", fmt.Sprintf("%d", p.ToSend), p.Hostname)

	// run command
	cmd := exec.Command(p.ExecPath, pingArgs[0:]...)
	cmd.Stdout = p.wout
	cmd.Stderr = p.werr

	// execute ping
	if err = cmd.Run(); err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			// 1 or 2 could be the exit code for ping
			// when host in not reachable
			if exitError.ExitCode() != 1 && exitError.ExitCode() != 2 {
				return stats, err
			}
		}
	}

	// analyzer will work for each OS on the buffers
	//wout and werr and returns a statistics object
	stats, err = p.analyzer()
	if err != nil {
		return stats, err
	}

	return stats, nil
}

func (p *Pinger) analyzer() (pingStats, error) {
	// search and save in this strings
	var pktStats, rttStats string
	var st pingStats

	// read line by line
	scanner := bufio.NewScanner(p.wout)
	for scanner.Scan() {
		line := scanner.Text()
		log.Println(line)
		// waiting for the statistic line to appear
		// could also be regular expression
		if strings.Contains(line, "packets transmitted") {
			pktStats = line
		} else if strings.Contains(line, "rtt") || strings.Contains(line, "round-trip") {
			rttStats = line
		}
	}

	// nothing found? return error
	if len(rttStats) == 0 && len(pktStats) == 0 {
		return pingStats{}, fmt.Errorf("No statistics gathered")
	}

	// begin to parse for different OS
	var pktFormat, rttFormat, pktDuplFormat, pktErrFormat string
	switch p.OS {
	case "darwin":
		if p.IPVersion == 6 {
			pktFormat, pktDuplFormat, pktErrFormat, rttFormat = darwin_pktstats, darwin_pktdupstats, linux_pktstats_error, darwin_rttstats6
			break
		}
		pktFormat, pktDuplFormat, pktErrFormat, rttFormat = darwin_pktstats, linux_pktdupstats, linux_pktstats_error, darwin_rttstats
	case "linux":
		pktFormat, pktDuplFormat, pktErrFormat, rttFormat = linux_pktstats, linux_pktdupstats, linux_pktstats_error, linux_rttstats
	default:
		return st, fmt.Errorf("dont know what to do for %s", p.OS)
	}

	// work on packet line
	// todo, rewrite as regular expression scanner
	if strings.Contains(pktStats, "duplicates") {
		_, err := fmt.Sscanf(pktStats, pktDuplFormat, &st.SendPkts, &st.RcvdPkts, &st.DuplPkts, &st.Loss)
		if err != nil {
			log.Println("At this point, ignore this error", err, "pktstats", pktStats)
		}
	} else if strings.Contains(pktStats, "errors") {
		_, err := fmt.Sscanf(pktStats, pktErrFormat, &st.SendPkts, &st.RcvdPkts, &st.ErrPkts, &st.Loss)
		if err != nil {
			log.Println("At this point, ignore this error", err, "pktstats", pktStats)
		}
	} else {
		_, err := fmt.Sscanf(pktStats, pktFormat, &st.SendPkts, &st.RcvdPkts, &st.Loss)
		if err != nil {
			log.Println("At this point, ignore this error", err, pktStats, "pktstats")
		}
	}
	// work on rtt line
	_, err := fmt.Sscanf(rttStats, rttFormat, &st.RttMin, &st.RttAvg, &st.RttMax, &st.StdDev)
	if err != nil {
		log.Println("At this point, ignore this error", err, rttStats, "rttstats")
	}

	return st, nil
}
