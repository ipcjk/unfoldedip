package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"unfoldedip/satsql"
	"unfoldedip/sattypes"
)

// handle contacts and contact groups
func alertgroups(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var g sattypes.Global
	var err error

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired2", http.StatusSeeOther)
		return
	}

	// retrieve alert groups from SQL driver
	Contacts, err := satsql.ReadAlertGroups(H, g.U.UserID)
	if err != nil {
		log.Println(err)
	} else {
		g.AlertGroups = Contacts
	}

	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "alertgroups.html", g)

}

// handle delete service (will be called by ajax query)
// redirect then (or not?)
func alertgroupDelete(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local variables
	var g sattypes.Global
	var delAlertGroup sattypes.AlertGroup
	var alertID string
	var err error

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired5", http.StatusSeeOther)
		return
	}

	// Delete method?  Else,  we will return
	if request.Method != http.MethodPost {
		goto DefaultAndExit
	}

	/* read params form */
	err = request.ParseForm()
	if err != nil {
		log.Println(err)
		goto DefaultAndExit
	}

	// check if csrf token is valid
	if !CheckCSRFToken(writer, request, H, g.U.UserSession) {
		return
	}

	// read alert id
	alertID = request.FormValue("id")
	if alertID == "" {
		log.Println("No id for alergroup deletion")
		goto DefaultAndExit
	}

	// retrieve group by groupid
	delAlertGroup, err = satsql.SelectAlertGroup(H, "contact_id", alertID)

	// service exists and owner is current user, then delete
	if err == nil && delAlertGroup.OwnerID == g.U.UserID {
		if H.Debug {
			log.Println("Deleting alertgroup", delAlertGroup.ContactID)
		}
		err := satsql.DeleteAlertGroup(H, delAlertGroup.ContactID)
		if err != nil {
			log.Println(err)
			goto DefaultAndExit
		}
		writer.WriteHeader(http.StatusOK)
		return
	}

	/* return no content by default */
DefaultAndExit:
	writer.WriteHeader(http.StatusNoContent)
	return
}

// adding a contact
func alertgroupAdd(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local variables
	var g sattypes.Global
	var err error
	var newContact, editContact, garbageContact sattypes.AlertGroup
	// for create, edit and checking access

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired7", http.StatusSeeOther)
		return
	}

	// things we expect to read from our form
	expectedVars := []string{
		"groupname",
		"emails",
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
		// is allowed to access the contact id
		if request.Form.Get("nextfunction") == "edit" {
			// read serviceID
			contactID := request.FormValue("id")
			if contactID == "" {
				if H.Debug {
					log.Println("No id for service logs")
				}
				goto DefaultAndExit
			}

			// retrieve service by service_id
			garbageContact, err = satsql.SelectAlertGroup(H, "contact_id", contactID)
			if garbageContact.ContactID == 0 {
				if H.Debug {
					log.Println("Not allowing access for contact edit or contact not existing")
				}
				goto DefaultAndExit
			}

			// check if the user is owner of this contact group
			if garbageContact.OwnerID != g.U.UserID {
				if H.Debug {
					log.Println("Not allowing access for contact edit or contact not existing")
				}
				goto DefaultAndExit
			}
			// access ok, copy over the id, release memory
			newContact.ContactID = garbageContact.ContactID
		}

		// check for all mandatory fields and collect errors if missing or invalid
		for _, x := range expectedVars {
			formValue := template.HTMLEscapeString(request.Form.Get(x))
			switch x {
			case "groupname":
				newContact.GroupName = func(arg string) string {
					if len(arg) == 0 {
						g.Errors = append(g.Errors, "Group name cant be zero")
						return ""
					}
					return arg
				}(formValue)
			case "emails":
				newContact.Emails = func(arg string) string {
					if len(arg) == 0 {
						g.Errors = append(g.Errors, "Email addresses cant be count of zero")
						return ""
					}
					return arg
				}(formValue)
			}
		}

		// add userid to service for db insert
		newContact.OwnerID = g.U.UserID

		// use the size from errorCollection as error indicator
		if len(g.Errors) == 0 {
			// check if we are in edit or post
			if request.Form.Get("nextfunction") == "edit" {
				if H.Debug {
					log.Println("Updating contact", newContact.ContactID)
				}
				err = satsql.UpdateAlertGroup(H, &newContact)
			} else {
				err = satsql.InsertAlertGroup(H, &newContact)
			}
			if err != nil {
				g.State = 2
			} else {
				http.Redirect(writer, request, fmt.Sprintf("/alertgroups"), http.StatusSeeOther)
				return
			}
		}
	}

	// check if we are in edit mode
	if request.Method == http.MethodGet {
		if strings.Contains(request.URL.Path, "alertgroup_edit") {
			// this becomes now tricky, we will try to parse the id
			contactID := request.FormValue("id")
			if contactID == "" {
				goto DefaultAndExit
			}
			editContact, err = satsql.SelectAlertGroup(H, "contact_id", contactID)

			// service exists and owner is current user, then proceed
			if err == nil && editContact.OwnerID == g.U.UserID {
				// template the selected service into the global var
				g.AlertGroup = editContact
				// tell the template function, that we want to edit
				// so it renders the right information
				g.NextFunction = "edit"
			}
			// else do nothing...
		}
	}

DefaultAndExit:
	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "alertgroup_add.html", g)

}
