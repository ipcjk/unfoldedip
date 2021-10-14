package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unfoldedip/satsql"
	"unfoldedip/sattypes"
)

// services prints all configured services from a customer
func services(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var g sattypes.Global
	var err error

	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired1", http.StatusSeeOther)
		return
	}

	//  read only user service
	Services, err := satsql.ReadServices(H, g.U.UserID, "", false)
	if err != nil {
		log.Println(err)
	} else {
		g.Services = Services
	}

	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "services.html", g)
}

// servicesLog prints out all log messages for all services
func servicesLogs(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var g sattypes.Global
	var serviceLogs []sattypes.ServiceLog
	var err error

	// check if user is loggedin
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired1", http.StatusSeeOther)
		return
	}

	// read service logs service by service_id
	serviceLogs, err = satsql.ReadServicesLog(H, g.U.UserID)
	if err != nil {
		log.Println(err)
	} else {
		g.ServiceLogs = serviceLogs
	}

	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "services_logs.html", g)
}

// serviceLogs prints out all log messages for a service
func serviceLogs(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var g sattypes.Global
	var serviceLogs []sattypes.ServiceLog
	var service sattypes.Service
	var serviceID string
	var err error

	// check if user is loggedin
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired1", http.StatusSeeOther)
		return
	}

	/* read params from form  */
	err = request.ParseForm()
	if err != nil {
		log.Println(err)
		goto DefaultAndExit
	}

	// read serviceID
	serviceID = request.FormValue("id")
	if serviceID == "" {
		if H.Debug {
			log.Println("No id for service logs")
		}
		goto DefaultAndExit
	}

	// retrieve service by service_id
	service, err = satsql.SelectService(H, "service_id", serviceID, g.U.UserID)
	if service.ServiceID == 0 {
		if H.Debug {
			log.Println("Not allowing access for service logs or service not existing")
		}
		goto DefaultAndExit
	}

	// read service logs service by service_id
	serviceLogs, err = satsql.ReadServiceLogs(H, serviceID)
	if err != nil {
		log.Println(err)
	} else {
		g.ServiceLogs = serviceLogs
		g.Service = service
	}

DefaultAndExit:
	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "service_logs.html", g)
}

// handle adding a service
func serviceAdd(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local variables
	var g sattypes.Global
	var err error
	// for create, edit and checking access
	var newService, editService, garbageService sattypes.Service
	// for hardcoded
	var AllowedIntervals = []int{5, 15, 30, 60, 90, 120}

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired4", http.StatusSeeOther)
		return
	}

	// load valid contact groups for the user
	Contacts, err := satsql.ReadAlertGroups(H, g.U.UserID)
	if err != nil {
		log.Println(err)
	} else {
		g.AlertGroups = Contacts
	}

	// things we expect to read from our form
	expectedVars := []string{
		"interval",
		"hostname",
		"contactgroup",
		"checktype",
		"url",
		"expected",
		"hosttcp",
		"servicename",
		"locations",
	}

	// handle POST
	if request.Method == http.MethodPost {

		// Parse form arguments
		err = request.ParseForm()
		if err != nil {
			log.Println(err)
			return
		}

		// check if csrf token is valid
		if !CheckCSRFToken(writer, request, H, g.U.UserSession) {
			return
		}

		// if we are in edit mode, we need carefully check, if the user
		// is allowed to access the service id
		if request.Form.Get("nextfunction") == "edit" {
			// read serviceID
			serviceID := request.FormValue("id")
			if serviceID == "" {
				if H.Debug {
					log.Println("No id for service logs")
				}
				goto DefaultAndExit
			}

			// retrieve service by service_id
			garbageService, err = satsql.SelectService(H, "service_id", serviceID, g.U.UserID)
			if garbageService.ServiceID == 0 {
				if H.Debug {
					log.Println("Not allowing access for service edit or service not existing")
				}
				goto DefaultAndExit
			}
			// access ok, copy over the id, release memory
			newService.ServiceID = garbageService.ServiceID
		}

		// check for all mandatory fields and collect errors if missing or invalid
		for _, x := range expectedVars {
			formValue := template.HTMLEscapeString(request.Form.Get(x))
			if formValue == "" {
				continue
			}
			switch x {
			case "locations":
				newService.Locations = func(selectedLocs []string) string {
					var Locations string
					for i := range selectedLocs {
						Locations += strings.ReplaceAll(selectedLocs[i], " ", "") + " "
					}
					return Locations
				}(request.Form["locations"])
			case "checktype":
				newService.Type = func(arg string) string {
					if arg == "ping" || arg == "http" || arg == "tcp" || arg == "tls" {
						return arg
					}
					g.Errors = append(g.Errors, "Unknown type of check, please correct")
					return ""
				}(formValue)
			case "hostname":
				newService.ToCheck = func(arg string) string {
					addr := net.ParseIP(arg)
					if addr != nil {
						// ip or ipv6 incoming
					}
					return arg
				}(formValue)
			case "servicename":
				newService.Name = func(arg string) string {
					if arg == "" {
						g.Errors = append(g.Errors, "Please enter a service name for identification")
					}
					return arg
				}(formValue)
			case "hosttcp":
				newService.ToCheck = func(arg string) string {
					return arg
				}(formValue)
			case "interval":
				newService.Interval = func(arg string) int {
					val, err := strconv.Atoi(arg)
					if err == nil {
						for i := range AllowedIntervals {
							if AllowedIntervals[i] == val {
								return val
							}
						}
					}
					return 90
				}(formValue)
			case "contactgroup":
				newService.ContactGroup = func(arg string) int {
					val, err := strconv.Atoi(arg)
					if err != nil {
						g.Errors = append(g.Errors, "Wrong or not alert group")
						log.Println(err)
					} else {
						ag, err := satsql.SelectAlertGroup(H, "contact_id", arg)
						if err == nil {
							if ag.OwnerID != g.U.UserID {
								g.Errors = append(g.Errors, "Wrong contactgroup")
								log.Println(err)
								return 0
							}
							return val
						} else if err != sql.ErrNoRows {
							g.Errors = append(g.Errors, "Wrong contactgroup")
							log.Println(err)
							return 0
						}
						return val
					}
					return val
				}(formValue)
			case "url":
				newService.ToCheck = func(arg string) string {
					if strings.HasSuffix(arg, "https://") || strings.HasSuffix(arg, "http://") {
						// empty url
						return ""
					}
					_, err := url.ParseRequestURI(arg)
					if err != nil {
						return ""
					}
					return arg
				}(formValue)
			case "expected":
				newService.Expected = func(arg string) string {
					return arg
				}(formValue)
			}
		}

		// make some combination check, for example if checktype is ping, we
		// need a hostname or an ip address, if checktype is http we need
		// httpurl value set
		if newService.Type == "ping" && newService.ToCheck == "" {
			g.Errors = append(g.Errors, "No hostname or IP/IPv6 address given")
		} else if newService.Type == "http" && newService.ToCheck == "" {
			g.Errors = append(g.Errors, "No/Invalid URL given"+newService.Type+newService.ToCheck)
		} else if (newService.Type == "tcp" || newService.Type == "tls") && newService.ToCheck == "" {
			g.Errors = append(g.Errors, "No host and / or tcp port given")
		} else if newService.Type == "" || newService.ToCheck == "" {
			g.Errors = append(g.Errors, "No type or check given")
		}

		// add userid to service for db insert
		newService.OwnerID = g.U.UserID
		// add location if necessary
		if newService.Locations == "" {
			newService.Locations = "any"
		}

		// use the size from errorCollection as error indicator
		if len(g.Errors) == 0 {
			// check if we are in edit or post
			if request.Form.Get("nextfunction") == "edit" {
				if H.Debug {
					log.Println("Updating service", newService.ServiceID)
				}
				err = satsql.UpdateService(H, &newService)
			} else {
				err = satsql.InsertService(H, &newService)
			}
			if err != nil {
				log.Println("SQL error", err)
			} else {
				http.Redirect(writer, request, fmt.Sprintf("/services"), http.StatusSeeOther)
				return
			}
		}
	}

	// check if we are in edit mode
	if request.Method == http.MethodGet {
		if strings.Contains(request.URL.Path, "service_edit") {
			// this becomes now tricky, we will try to parse the id
			serviceID := request.FormValue("id")
			if serviceID == "" {
				log.Println("No id for service edit")
				goto DefaultAndExit
			}
			editService, err = satsql.SelectService(H, "service_id", serviceID, g.U.UserID)

			// service exists and owner is current user, then proceed
			if err == nil && editService.OwnerID == g.U.UserID {
				// template the selected service into the global var
				g.Service = editService
				// tell the template function, that we want to edit
				// so it renders the right information
				g.NextFunction = "edit"
			}
			// else do nothing...
		}
	}

DefaultAndExit:
	// Pass some defaults down the template
	g.AllowedIntervals = AllowedIntervals
	if g.Service.ServiceID == 0 {
		g.Service.Interval = 90
	}
	// load list of locations
	g.SatAgentLocations, err = satsql.ReadAgentLocations(H)
	if err != nil {
		log.Println(err)
	}
	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "service_add.html", g)

}

// handle delete service (will be called by ajax query)
// redirect then (or not?)
func serviceDelete(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local variables
	var g sattypes.Global
	var delService sattypes.Service
	var serviceID string
	var err error

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired6", http.StatusSeeOther)
		return
	}

	// Not POST method? Then jump to clean up
	if request.Method != http.MethodPost {
		goto DefaultAndExit
	}

	/* read params from form  */
	err = request.ParseForm()
	if err != nil {
		log.Println(err)
		goto DefaultAndExit
	}

	// check if csrf token is valid
	if !CheckCSRFToken(writer, request, H, g.U.UserSession) {
		return
	}

	// read serviceID
	serviceID = request.FormValue("id")
	if serviceID == "" {
		log.Println("No id for service deletion")
		goto DefaultAndExit
	}

	// retrieve service by service_id
	delService, err = satsql.SelectService(H, "service_id", serviceID, g.U.UserID)

	// service exists and owner is current user, then delete
	if err == nil && delService.OwnerID == g.U.UserID {
		if H.Debug {
			log.Println("Deleting service", delService.ServiceID)
		}
		err = satsql.DeleteService(H, delService.ServiceID)
		if err != nil {
			log.Println(err)
			goto DefaultAndExit
		}
		if H.Debug {
			log.Println("Deleting service logs", delService.ServiceID)
		}
		err = satsql.DeleteServiceLogs(H, delService.ServiceID)
		if err != nil {
			log.Println(err)
			goto DefaultAndExit
		}
		writer.WriteHeader(http.StatusOK)
		return
	}

	/* return no content */
DefaultAndExit:
	writer.WriteHeader(http.StatusNoContent)
	return
}

// handle reset service (will be called by ajax query)
// resets a service to "UNKNOWN"
func serviceReset(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local variables
	var g sattypes.Global
	var resetService sattypes.Service
	var serviceID string
	var err error

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired6", http.StatusSeeOther)
		return
	}

	// Not POST method? Then jump to clean up
	if request.Method != http.MethodPost {
		goto DefaultAndExit
	}

	/* read params from form  */
	err = request.ParseForm()
	if err != nil {
		log.Println(err)
		goto DefaultAndExit
	}

	// check if csrf token is valid
	if !CheckCSRFToken(writer, request, H, g.U.UserSession) {
		return
	}

	// read serviceID
	serviceID = request.FormValue("id")
	if serviceID == "" {
		log.Println("No id for service reset")
		goto DefaultAndExit
	}

	// retrieve service by service_id
	resetService, err = satsql.SelectService(H, "service_id", serviceID, g.U.UserID)

	// service exists and owner is current user, then reset
	if err == nil && resetService.OwnerID == g.U.UserID {
		if H.Debug {
			log.Println("Resetting service", resetService.ServiceID)
		}
		err = satsql.ResetService(H, resetService.ServiceID)
		if err != nil {
			log.Println(err)
			goto DefaultAndExit
		}

		// convert serviceID to int64
		sID, err := strconv.ParseInt(serviceID, 10, 64)
		if err == nil {
			log.Println(err)
		}

		// pushing a message down the channel?
		// attention, fixme, this could be a deadlock if the channel is FULl?
		sattypes.ResultsChannel <- sattypes.ServiceResult{
			ServiceID:   sID,
			Status:      sattypes.ServiceUnknown,
			Message:     "The service state has been reset",
			Time:        time.Now(),
			TestNode:    "User",
			RapidChange: true,
		}

		writer.WriteHeader(http.StatusOK)
		return
	}

	/* return no content */
DefaultAndExit:
	writer.WriteHeader(http.StatusNoContent)
	return
}
