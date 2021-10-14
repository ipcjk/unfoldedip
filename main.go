package main

// unfoldedip (C) 2021 by Jörg Kost, jk@ip-clear.de
// MIT License

import (
	"database/sql"
	"flag"
	"html/template"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"sync"
	"unfoldedip/satagent"
	"unfoldedip/satanalytics"
	"unfoldedip/sattypes"
)

// global parsed and compiled templats
var templates *template.Template

// BaseHandler holds some globals and the DB connection
var BaseHandler sattypes.BaseHandler

func init() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func main() {
	var err error
	var SMTPConfig sattypes.SMTPConfiguration
	wg := sync.WaitGroup{}

	// command line arguments for server
	httpAddr := flag.String("http", "127.0.0.1:8080", "port for the default listener  (server)")
	dbFile := flag.String("db", "unfolded.sqlite", "path to the sqlite database filer (server)")
	server := flag.Bool("server", true, "server / http mode enabled, -server=false for disabling")
	// command line arguments for smtp interface
	flag.StringVar(&SMTPConfig.SmtpServer, "smtp", "", "server for smtp sendmail function")
	flag.StringVar(&SMTPConfig.SmtpUser, "smtpuser", "", "login for smtp authentication")
	flag.StringVar(&SMTPConfig.SmtpPassword, "smtppass", "", "password for smtp authentication")
	flag.StringVar(&SMTPConfig.SmtpSender, "smtpsender", "", "sender email source for all mails")
	// command line arguments for client
	agent := flag.Bool("agent", true, "satellite (satagent) mode only")
	agentLocation := flag.String("agentloc", "Munich", "satagent location")
	agentOnlyLocation := flag.Bool("onlylocation", false, "boolean to control, if the agent can do any check or only for his location")
	agentName := flag.String("agentname", "muc1", "satagent name")
	serverURL := flag.String("serverurl", "http://localhost:8080", "url for satserver")
	// both
	agentKey := flag.String("agentkey", "0000", "shared access key for submitting to the satkey")
	debug := flag.Bool("debug", false, "turns on debug mode")

	// parse command line arguments
	flag.Parse()

	// copy some values in our handlers
	BaseHandler.Debug = *debug
	BaseHandler.SatKey = *agentKey
	BaseHandler.SMTPConfiguration = SMTPConfig

	// todo generate random key
	// if no function is enabled, quit right now
	if !*agent && !*server {
		log.Println("Nothing to do, thanks for calling me anyhow")
		return
	}

	// Start satellite agent
	if *agent {
		s := satagent.CreateSatAgent(*serverURL, *agentName, *agentLocation, *agentOnlyLocation, BaseHandler)
		wg.Add(1)
		go s.Run()
	}

	// Start http server if enabled
	if *server {
		// fill in the CLI
		BaseHandler.URL = *serverURL
		// open db connection
		BaseHandler.DB, err = sql.Open("sqlite", *dbFile)
		if err != nil {
			log.Panic(err)
		}
		// Set maximum limit
		BaseHandler.DB.SetMaxOpenConns(1)
		// close on exit
		defer BaseHandler.DB.Close()

		// init resultsChannel
		// with buffer till 100 messages
		sattypes.ResultsChannel = make(chan sattypes.ServiceResult, 128)

		// HTTP server will contain the  sat analytics thread,
		// so we need to create one
		satAnalytics := satanalytics.CreateSatAnalytics("main", BaseHandler)
		wg.Add(1)
		go satAnalytics.Run()

		// Compile and parse all templates for the web-panel
		templates = template.Must(template.ParseGlob("templates/*"))

		// Function code to handle the web panel, login, registration, service manipulation
		// static file server
		http.Handle("/assets/", http.StripPrefix("/assets/", http.FileServer(http.Dir("./assets"))))
		// function to handle login
		http.HandleFunc("/login", func(writer http.ResponseWriter, request *http.Request) { login(writer, request, BaseHandler) })
		// function to handle login
		http.HandleFunc("/logout", func(writer http.ResponseWriter, request *http.Request) { logout(writer, request, BaseHandler) })
		// function to handle forget and "forgot" password
		http.HandleFunc("/forget", func(writer http.ResponseWriter, request *http.Request) { forget(writer, request, BaseHandler) })
		http.HandleFunc("/forget2", func(writer http.ResponseWriter, request *http.Request) { forget2(writer, request, BaseHandler) })
		// function to handle profile changes
		http.HandleFunc("/profile", func(writer http.ResponseWriter, request *http.Request) { profile(writer, request, BaseHandler) })
		// function to handle register
		http.HandleFunc("/register", func(writer http.ResponseWriter, request *http.Request) { register(writer, request, BaseHandler) })
		// function to handle the service_add call
		http.HandleFunc("/service_add", func(writer http.ResponseWriter, request *http.Request) { serviceAdd(writer, request, BaseHandler) })
		// function to handle the service_delete
		http.HandleFunc("/service_delete", func(writer http.ResponseWriter, request *http.Request) { serviceDelete(writer, request, BaseHandler) })
		// function to reset the service to unknown
		http.HandleFunc("/service_reset", func(writer http.ResponseWriter, request *http.Request) { serviceReset(writer, request, BaseHandler) })
		// function to list user services
		http.HandleFunc("/services", func(writer http.ResponseWriter, request *http.Request) { services(writer, request, BaseHandler) })
		// function to handle the services_log call
		http.HandleFunc("/services_logs", func(writer http.ResponseWriter, request *http.Request) { servicesLogs(writer, request, BaseHandler) })
		// function to handle the service_edit
		http.HandleFunc("/service_edit", func(writer http.ResponseWriter, request *http.Request) { serviceAdd(writer, request, BaseHandler) })
		// function to handle the service_logs call
		http.HandleFunc("/service_logs", func(writer http.ResponseWriter, request *http.Request) { serviceLogs(writer, request, BaseHandler) })
		// function to list user contacts
		http.HandleFunc("/alertgroups", func(writer http.ResponseWriter, request *http.Request) { alertgroups(writer, request, BaseHandler) })
		// function to add contact groups
		http.HandleFunc("/alertgroup_add", func(writer http.ResponseWriter, request *http.Request) { alertgroupAdd(writer, request, BaseHandler) })
		// function to edit contact groups
		http.HandleFunc("/alertgroup_edit", func(writer http.ResponseWriter, request *http.Request) { alertgroupAdd(writer, request, BaseHandler) })
		// function to delete contact groups
		http.HandleFunc("/alertgroup_delete", func(writer http.ResponseWriter, request *http.Request) {
			alertgroupDelete(writer, request, BaseHandler)
		})
		// function to handle requests to "/"
		http.HandleFunc("/", func(writer http.ResponseWriter, request *http.Request) {
			// redirect to /services-dashboard, if path is ending with /
			if request.URL.Path == "/" {
				services(writer, request, BaseHandler)
				return
			}
			http.NotFound(writer, request)
			return
		})

		// functions to handle GET and POST calls by our monitoring satellite agents
		// handler to send satellite configuration
		http.HandleFunc("/agents/config", func(writer http.ResponseWriter, request *http.Request) { agentsConfig(writer, request, BaseHandler) })
		// handler to  retrieve service results
		http.HandleFunc("/agents/results", func(writer http.ResponseWriter, request *http.Request) { agentsResults(writer, request, BaseHandler) })

		// start http listener socket
		log.Println("satserver: Starting listener")
		err = http.ListenAndServe(*httpAddr, nil)
		if err != nil {
			// can't start the HTTP server, then we better quit
			log.Fatal(err)
		}
	}

	// Wait for thread if any is still active
	wg.Wait()
}
