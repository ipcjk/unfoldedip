package sattypes

import (
	"bytes"
	"crypto/tls"
	"database/sql"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"log"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

// resultsChannel is the communication channel between
// main thread and the analyzer thread
var ResultsChannel chan ServiceResult

// A base handler for passing the DB connection
type BaseHandler struct {
	DB     *sql.DB
	Debug  bool
	SatKey string
	SMTPConfiguration
	URL        string
	EndChannel struct{}
}

// SMTP Configuration
type SMTPConfiguration struct {
	SmtpServer, SmtpUser, SmtpPassword, SmtpSender string
}

// SessionManager is a pseudo type for managing, selecting, deleting sessions from SQL
type SessionManager struct{}

// Global  that is used for session tracking and templating
// will 99/100 times be used for delivering our data to the templates
type Global struct {
	U                 UnfoldedUser
	UserLoggedIn      bool
	Errors            []string
	Notices           []string
	State             int
	Service           Service
	Services          []Service
	AllowedIntervals  []int
	ServiceLogs       []ServiceLog
	AlertGroup        AlertGroup
	SatAgent          SatAgentSql
	SatAgents         []SatAgentSql
	SatAgentLocations []string
	AlertGroups       []AlertGroup
	CSRF              string
	NextFunction      string
}

// Service will be filled by sql driver and exported to the satellite agents in JSON
type Service struct {
	ServiceID      int64     `json:"serviceid"`
	Name           string    `json:"name"`
	OwnerID        int64     `json:"ownerid"`
	Type           string    `json:"type"`
	ToCheck        string    `json:"tocheck"`
	Expected       string    `json:"expected"`
	Interval       int       `json:"interval"`
	ContactGroup   int       `json:"contactgroup"`
	NextInterval   int       `json:"nextinterval"`
	AlertGroupName string    `json:"groupname"`
	ServiceState   string    `json:"servicestate"`
	Exists         bool      `json:"exists"`
	LastEvent      string    `json:"lastevent"`
	LastSeen       time.Time `json:"lastseen"`
	Locations      string    `json:"locations"`
}

// AlertGroup will be filled by sql driver
type AlertGroup struct {
	ContactID int64  `json:"contactid"`
	OwnerID   int64  `json:"ownerid"`
	GroupName string `json:"groupname"`
	Emails    string `json:"emails"`
	Exists    bool   `json:"exists"`
}

// ServiceResult is a struct, that will be posted back
// as JSON to the server, then read by the analyzer
type ServiceResult struct {
	ServiceID   int64     `json:"serviceID"`
	Status      string    `json:"status"`
	Message     string    `json:"message"`
	Time        time.Time `json:"time"`
	TestNode    string    `json:"node"`
	RapidChange bool      `json:"rapidchange"`
}

// ServiceLog is a struct, that will  be used to
// present the log enries
type ServiceLog struct {
	ServiceID   int64  `json:"serviceID"`
	ToCheck     string `json:"service_tocheck"`
	Name        string `json:"service_name"`
	Status_From string `json:"status_from"`
	Status_To   string `json:"status_to"`
	Date        string `json:"date"`
	Why         string `json:"status_why"`
}

// UnfoldedUser struct is used for creating and
// managing user objects
type UnfoldedUser struct {
	Email            string
	UserID           int64
	PasswordHash     string
	PasswordHashNext string
	Reset            string
	LoggedIn         bool
	Exists           bool
	UserSession      Session
}

// Session consist of userid and a CSRF token
// both read from the SQL driver
type Session struct {
	UserID    int64
	CSRF      string
	SessionID string
}

// SatAgentSql will be filled by sql driver
type SatAgentSql struct {
	SatAgentID       string
	SatAgentName     string
	SatAgentLocation string
	SatOnlyLocation  bool
	AccessKey        string
	LastSeen         string
}

// Service Results state as expression
const (
	_              = iota
	ServiceUP      = "SERVICE_UP"
	ServiceDown    = "SERVICE_DOWN"
	ServiceUnknown = "SERVICE_UNKNOWN"
)

// check password, encrypt incoming with bcrypt
func (u *UnfoldedUser) SetEmail(email string) error {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return err
	}
	u.Email = email
	return nil
}

// check password, encrypt incoming with bcrypt and compare to the password from the user object
func (u UnfoldedUser) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	if err == nil {
		return true
	}
	return false
}

// bcrypt the password, save into user object and also return directly for further processing
func (u *UnfoldedUser) GeneratePassword(password string) (string, error) {
	passwordBytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	if err == nil {
		u.PasswordHash = string(passwordBytes)
	}
	return "", err
}

// SendMail sends out mail using the smtp configuration from command line
func (smtpConfig SMTPConfiguration) SendMail(subject, recipient, body string) error {
	// split server and port away
	authHostName := strings.Split(smtpConfig.SmtpServer, ":")[0]
	if len(authHostName) == 0 {
		log.Println("No server name for smtp auth")
		return fmt.Errorf("no server name for smtp auth, given %s", smtpConfig.SmtpServer)
	}

	// create smtp auth object
	auth := smtp.PlainAuth("", smtpConfig.SmtpUser, smtpConfig.SmtpPassword, authHostName)

	// Enable TLS for Golang Dial
	tlsconfig := &tls.Config{
		InsecureSkipVerify: true,
		ServerName:         smtpConfig.SmtpServer,
	}

	// Start the connection
	c, err := smtp.Dial(smtpConfig.SmtpServer)
	if err != nil {
		return err
	}
	defer c.Close()

	hostname, err := os.Hostname()
	if err != nil {
		return err
	}
	c.Hello(hostname)

	// Upgrade to TLS
	err = c.StartTLS(tlsconfig)
	if err != nil {
		return err
	}

	// Send Auth
	err = c.Auth(auth)
	if err != nil {
		return err
	}

	err = c.Mail(smtpConfig.SmtpSender)
	if err != nil {
		return err
	}

	err = c.Rcpt(recipient)
	if err != nil {
		return err
	}

	/* get current time / data */
	t := time.Now()

	/* Add header */
	header := new(bytes.Buffer)
	/* prepare header with from, to, subject and the current date / time */
	fmt.Fprintf(header, "From: %s\r\n", smtpConfig.SmtpSender)
	fmt.Fprintf(header, "To: %s\r\n", recipient)
	fmt.Fprintf(header, "Subject:  %s\r\n", subject)
	fmt.Fprintf(header, "Date: %s", t.Format(time.RFC1123Z))
	fmt.Fprintf(header, "\r\n\r\n")

	/* data begins here */
	wc, err := c.Data()
	if err != nil {
		log.Println(err)
		return err
	}
	defer wc.Close()

	/* output header lines */
	if _, err = header.WriteTo(wc); err != nil {
		log.Println(err)
		return err
	}

	/* and output body header lines */
	if _, err = strings.NewReader(body).WriteTo(wc); err != nil {
		log.Println(err)
		return err
	}

	// and close the curtain
	wc.Close()

	return nil

}

// SendServiceMail templates and prepares the mail for service related information
func (smtpConfig SMTPConfiguration) SendServiceMail(s Service, r ServiceResult, recipient string) error {
	var state = "UNKNOWN"
	var body bytes.Buffer
	var message string
	if s.ServiceState == ServiceUP {
		state = "UP"
		message = `
IP-Unfolded monitoring service notification

{{.S.Name}} is UP and has recovered from an error or an unknown state.

Type of Check: {{.S.Type}}
Timepoint: {{.R.Time}}
Message: {{.R.Message}}

BR
IP Unfolded`
	} else if s.ServiceState == ServiceDown {
		state = "DOWN"
		message = `
IP-Unfolded monitoring service notification

{{.S.Name}} is DOWN and has encountered an error.

Type of Check: {{.S.Type}}
Timepoint: {{.R.Time}}
Message: {{.R.Message}}

BR
IP Unfolded
`
	} else if s.ServiceState == ServiceUnknown {
		state = "UNKNOWN"
		message = `
IP-Unfolded monitoring service notification

{{.S.Name}} is in an UNKNOWN state and unfolded not received any check results in the last 600 seconds.

Type of Check: {{.S.Type}}
Timepoint: {{.R.Time}}
Message: {{.R.Message}}

BR
IP Unfolded
`
	}

	/* Custom template content */
	type Content struct {
		S Service
		R ServiceResult
	}

	// construct template
	tmpl1, err := template.New("Mail").Parse(message)

	// Execute
	err = tmpl1.Execute(&body, Content{S: s, R: r})
	if err != nil {
		return err
	}

	err = smtpConfig.SendMail(fmt.Sprintf("Your Service: %s is %s", s.Name, state), recipient, body.String())
	if err != nil {
		return err
	}

	return nil
}

// SendPasswordForget templates and prepares the mail for the password forget function
func (smtpConfig SMTPConfiguration) SendPasswordForget(recp, password, hash, serverurl string) error {
	var body bytes.Buffer
	var message string = `IP-Unfolded registration service

Hello,

Would you please follow and submit the webpage on the following link? Then we will assign a new password for ` +
		`our monitoring service. The new password will be {{.P}}

{{.U}}/forget2?hash={{.H}}

BR
IP Unfolded
`

	/* Custom template content */
	type Content struct {
		P string
		H string
		U string
	}

	// construct template
	tmpl1, err := template.New("Mail").Parse(message)
	if err != nil {
		return err
	}

	// Execute
	err = tmpl1.Execute(&body, Content{P: password, H: hash, U: serverurl})
	if err != nil {
		return err
	}

	err = smtpConfig.SendMail(fmt.Sprintf("Your login information for our website"), recp, body.String())
	if err != nil {
		return err
	}

	return nil
}
