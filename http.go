package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/satori/uuid"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"strings"
	"time"
	"unfoldedip/satsql"
	"unfoldedip/sattypes"
)

// general template function
func executeGlobalAgainstTemplate(writer http.ResponseWriter, templateName string, g sattypes.Global) {
	err := templates.ExecuteTemplate(writer, templateName, g)
	if err != nil {
		log.Println(err)
	}
	return
}

// takes the /login - handle, tries to create a session with a username
// and a password, that is given by the user by submitting the html - form
func login(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// local error and g for templating
	var err error
	var g sattypes.Global

	// login is a tiny bit complicated function
	// that's why we defined helper functions
	// ahead, that can be called from every local context
	// to template the login page, but also to clean up sessions

	// deletes pending session and generates a new one
	genAndDestroySession := func() {
		// Default: We will print out the template
		// add a new CSRF token and session cookie
		// for protecting the login area

		// destroy old session from database
		cookieOld, err := request.Cookie("Session")

		// destroy old session if existed
		if err == nil && cookieOld.Value != "" {
			_ = satsql.DestroySession(H, cookieOld.Value)
		}

		// create cookie
		c := &http.Cookie{
			Name:  "Session",
			Value: uuid.NewV4().String(),
		}

		// create csrf token
		g.CSRF = uuid.NewV4().String()

		if H.Debug {
			log.Println(c.Value, "is the new session leader with CSRF", g.CSRF)
		}
		// create a new session, that is not connected with a valid user
		err = satsql.NewSession(H, sattypes.UnfoldedUser{UserID: 0}, c.Value, g.CSRF)
		if err != nil {
			log.Println(err)
			return
		}

		// write cookies
		http.SetCookie(writer, c)
	}

	// clean up from an error, set an error code
	// in global, destroy the current session for safety reason
	// return template
	cleanAndTemplate := func(state int, err error) {
		if H.Debug && err != nil {
			log.Println(err)
		}
		g.State = state
		genAndDestroySession()
		executeGlobalAgainstTemplate(writer, "login.html", g)
	}

	// now, it is time to check if the FORM is posted
	// else template the GET - page
	if request.Method == http.MethodPost {
		// parse form ahead
		err = request.ParseForm()
		if err != nil {
			cleanAndTemplate(0, err)
			return
		}

		// check if session cookie exists and csrf is right
		cookieOld, err := request.Cookie("Session")

		// something very strange, better return early
		if err != nil || cookieOld.Value == "" {
			cleanAndTemplate(0, err)
			return
		}

		// get current CSRF for empty session
		csrf, err := satsql.CheckEmptySession(H, cookieOld.Value)
		if err != nil || csrf == "" {
			cleanAndTemplate(0, err)
			return
		}

		// stored CSRF compare equal to submitted CSRF
		// if not, it could be a break in attempt
		if csrf != request.FormValue("logincsrf") {
			if H.Debug {
				log.Println("CSRF expected", csrf, "but got", request.FormValue("logincsrf"))
			}
			cleanAndTemplate(0, err)
			return
		}

		// at this point CSRF for login submission is validated
		// finally do some basic input sanitize
		email := strings.ToLower(template.HTMLEscapeString(request.Form.Get("email")))
		password := request.Form.Get("password")

		// check if user exists
		user, err := satsql.SelectUser(H, "email", email)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		// check password matches with given user
		if g.State == 0 && user.CheckPassword(password) {
			log.Println(email, user.UserID, "logged into the web console")

			// destroy old session from database
			_ = satsql.DestroySession(H, cookieOld.Value)

			// create a new cookie /
			c := &http.Cookie{
				Name:  "Session",
				Value: uuid.NewV4().String(),
			}

			// create a new csrf token for cookie lifecycle
			csrf := uuid.NewV4().String()

			// create a new session in db
			err = satsql.NewSession(H, user, c.Value, csrf)
			if err != nil {
				cleanAndTemplate(2, err)
				return
			}

			// write cookies and redirect to service dashboard
			http.SetCookie(writer, c)
			http.Redirect(writer, request, "/services", http.StatusFound)
			return
		}
		// login did not work, password mismatch
		cleanAndTemplate(2, err)
		return
	}

	// Are we getting a redirect from register?
	if request.Method == http.MethodGet {
		if request.URL.Query().Get("register") == "success" {
			// set registration was successful state
			g.State = 1
		} else if request.URL.Query().Get("session") == "pwdrecovered" {
			// set registration was successful state
			g.State = 3
		}
	}

	cleanAndTemplate(g.State, err)
	return
}

// handle register page
func register(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var U sattypes.UnfoldedUser
	var g sattypes.Global

	// handles the register page, if we will any error, we will bail out directly
	// by setting a global error code and then jumping into the template parse function
	if request.Method == http.MethodPost {
		// Parse form arguments
		err := request.ParseForm()
		if err != nil {
			g.State = 1
			goto ExitAndDefault
		}

		// fill our user object
		if U.SetEmail(template.HTMLEscapeString(request.Form.Get("email"))) != nil {
			g.State = 2
			goto ExitAndDefault
		}

		_, err = satsql.SelectUser(H, "email", U.Email)
		// nil == User already exists and was pulled
		// err != sql.ErrNoRows (other error, then user does not exist)
		if err == nil {
			g.State = 4
			goto ExitAndDefault
		} else if err != sql.ErrNoRows {
			g.State = 3
			goto ExitAndDefault
		}

		// bail out, if password is zero length
		password := request.Form.Get("password")
		if len(password) == 0 {
			g.State = 5
			goto ExitAndDefault
		}

		// hash password
		_, err = U.GeneratePassword(password)
		if err != nil {
			g.State = 6
			goto ExitAndDefault
		}

		// insert user into db, rewrite U struct
		err = satsql.InsertUser(H, &U)
		if err != nil {
			g.State = 7
			goto ExitAndDefault
		}

		// Registration worked? Then redirect
		if U.UserID != 0 && g.State == 0 {
			http.Redirect(writer, request, "/login?register=success", http.StatusFound)
			return
		}
	}

ExitAndDefault:
	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "register.html", g)

}

// forget2 step2 will present a button to the user, that he needs no click for accepting the sent password
// as new user password
func forget2(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var garbageU sattypes.UnfoldedUser
	var g sattypes.Global
	var err error
	var resetHash string

	// handles the forget2 submit, if we will any error, we will bail out directly
	// by setting a global error code and then jumping into the template parse function
	if request.Method == http.MethodPost {
		// Parse form arguments
		err = request.ParseForm()
		if err != nil {
			g.State = 1
			goto ExitAndDefault
		}

		// read hash from query
		resetHash = request.Form.Get("hash")
		if len(resetHash) == 0 {
			g.State = 2
			goto ExitAndDefault
		}

		garbageU, err = satsql.SelectUser(H, "reset", resetHash)
		// nil == User exists and was pulled
		// err != sql.ErrNoRows (other error, then user does not exist)
		if err != nil {
			g.State = 3
			goto ExitAndDefault
		}
		// hash was right, so lets reset the password flipping the column
		err = satsql.UpdateUser(H, garbageU, "password", garbageU.PasswordHashNext)
		if err != nil {
			g.State = 4
			goto ExitAndDefault
		}
		// delete hash and NextPassword afterwards with randomness
		err = satsql.UpdateUser(H, garbageU, "reset", genrandom(8))
		if err != nil {
			g.State = 5
			goto ExitAndDefault
		}
		// delete hash and NextPassword afterwards with randomness
		err = satsql.UpdateUser(H, garbageU, "passwordnext", genrandom(8))
		if err != nil {
			g.State = 6
			goto ExitAndDefault
		}
		// we are here? Great..., signal 1 for "everything fine"
		http.Redirect(writer, request, "/login?session=pwdrecovered", http.StatusSeeOther)
		return
	}
	if request.Method == http.MethodGet {
		// check hash in query url
		resetHash = request.URL.Query().Get("hash")
		if len(resetHash) == 0 {
			g.State = 7
			goto ExitAndDefault
		}

		// is there any valid user for this hash?
		garbageU, err = satsql.SelectUser(H, "reset", resetHash)
		// nil == User exists and was pulled
		// err != sql.ErrNoRows (other error, then user does not exist)
		if err != nil {
			g.State = 8
			goto ExitAndDefault
		}
		// if there is a user, template the user back
		g.U = garbageU
	}

ExitAndDefault:
	// print out error if any
	if err != nil {
		log.Println(err)
	}
	// Safe we, if state is not 0 zero, throw forbidden
	if g.State != 0 {
		writer.WriteHeader(http.StatusForbidden)
		return
	}
	// Default is GET method where we will print out the template
	executeGlobalAgainstTemplate(writer, "forget2.html", g)

}

// forget will send out a possible new password with a hash to submit
func forget(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var U sattypes.UnfoldedUser
	var g sattypes.Global
	var err error

	// code is a bit copied from func login
	// because we need to take, that third parties do not generate
	// forget panels  // - maybe kind of middleware would be cool
	// deletes pending session and generates a new one
	genAndDestroySession := func() {
		// Default: We will print out the template
		// add a new CSRF token and session cookie
		// for protecting the login area

		// destroy old session from database
		cookieOld, err := request.Cookie("Session")

		// destroy old session if existed
		if err == nil && cookieOld.Value != "" {
			_ = satsql.DestroySession(H, cookieOld.Value)
		}

		// create cookie
		c := &http.Cookie{
			Name:  "Session",
			Value: uuid.NewV4().String(),
		}

		// create csrf token
		g.CSRF = uuid.NewV4().String()

		if H.Debug {
			log.Println(c.Value, "is the new session leader with CSRF", g.CSRF)
		}
		// create a new session, that is not connected with a valid user
		err = satsql.NewSession(H, sattypes.UnfoldedUser{UserID: 0}, c.Value, g.CSRF)
		if err != nil {
			log.Println(err)
			return
		}

		// write cookies
		http.SetCookie(writer, c)
	}

	// clean up from an error, set an error code
	// in global, destroy the current session for safety reason
	// return template
	cleanAndTemplate := func(state int, err error) {
		if H.Debug && err != nil {
			log.Println(err)
		}
		g.State = state
		genAndDestroySession()
		executeGlobalAgainstTemplate(writer, "forget1.html", g)
	}

	// handles the forget1 page, if we will any error, we will bail out directly
	// by setting a global error code and then jumping into the template parse function
	if request.Method == http.MethodPost {
		// Parse form arguments
		err = request.ParseForm()
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		// check if session cookie exists and csrf is right
		cookieOld, err := request.Cookie("Session")

		// get current CSRF for empty session
		csrf, err := satsql.CheckEmptySession(H, cookieOld.Value)
		if err != nil || csrf == "" {
			cleanAndTemplate(0, err)
			return
		}

		// get current CSRF for empty session
		csrf, err = satsql.CheckEmptySession(H, cookieOld.Value)
		if err != nil || csrf == "" {
			cleanAndTemplate(0, err)
			return
		}

		// stored CSRF compare equal to submitted CSRF
		// if not, it could be a break in attempt
		if csrf != request.FormValue("logincsrf") {
			if H.Debug {
				log.Println("CSRF expected", csrf, "but got", request.FormValue("logincsrf"))
			}
			cleanAndTemplate(0, err)
			return
		}

		// at this point CSRF for login submission is validated
		// finally do some basic input sanitize

		// fill our user object
		if U.SetEmail(strings.ToLower(template.HTMLEscapeString(request.Form.Get("email")))) != nil {
			cleanAndTemplate(2, err)
			return
		}

		U, err = satsql.SelectUser(H, "email", U.Email)
		// nil == User exists and was pulled
		// err != sql.ErrNoRows (other error, then user does not exist)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		// bail out, if password is zero length
		password := genrandom(8)
		// hash password
		_, err = U.GeneratePassword(password)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		resetHash := uuid.NewV4().String()
		// update user with hash and a possibile new password
		err = satsql.UpdateUser(H, U, "reset", resetHash)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}
		// insert user into db, rewrite U struct
		err = satsql.UpdateUser(H, U, "passwordnext", U.PasswordHash)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		err = H.SendPasswordForget(U.Email, password, resetHash, H.URL)
		if err != nil {
			cleanAndTemplate(2, err)
			return
		}

		cleanAndTemplate(1, err)
		return
	}

	// collect error if there is some
	if err != nil {
		log.Println(err)
	}
	cleanAndTemplate(0, err)
	return
}

// isLoggedIn checks if a user cookie exists and the user session is logged in
func isLoggedIn(request *http.Request, H sattypes.BaseHandler) (sattypes.UnfoldedUser, bool) {
	// pull session cookie
	c, err := request.Cookie("Session")
	if err != nil {
		log.Printf("Cookie not found for ip %s, uri %s", request.RemoteAddr, request.URL)
		return sattypes.UnfoldedUser{}, false
	}

	// check session cookie
	userSession, err := satsql.CheckUserSession(H, c.Value)
	if err != nil {
		log.Printf("Cookie or session not found for ip %s, uri %s", request.RemoteAddr, request.URL)
		return sattypes.UnfoldedUser{}, false
	}

	// select user from db
	U, err := satsql.SelectUser(H, "id", fmt.Sprintf("%d", userSession.UserID))
	if err != nil {
		return sattypes.UnfoldedUser{}, false
	}

	// update session in database
	err = satsql.UpdateSession(H, userSession)
	if H.Debug && err != nil {
		log.Println(err)
	}

	// embedding session into user object
	U.UserSession = userSession

	// return User object
	return U, true
}

// handle profile page and updates to email and password
func profile(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	var g sattypes.Global

	// retrieve session
	if g.U, g.U.LoggedIn = isLoggedIn(request, H); !g.U.LoggedIn {
		http.Redirect(writer, request, "/login?session=expired3", http.StatusSeeOther)
		return
	}

	// check if submission was done
	if request.Method == http.MethodPost {

		err := request.ParseForm()
		if err != nil {
			g.State = 1
			goto ExitAndDefault
		}

		// check if csrf token is valid
		if !CheckCSRFToken(writer, request, H, g.U.UserSession) {
			return
		}

		// read, what action needs to be done
		if template.HTMLEscapeString(request.Form.Get("changepassword")) == "1" {
			// change password?
			// check incoming old password
			if !g.U.CheckPassword(template.HTMLEscapeString(request.Form.Get("password"))) {
				g.State = 5
				goto ExitAndDefault
			}

			// bail out, if new password is zero length
			newpassword := request.Form.Get("newpassword")
			if len(newpassword) == 0 {
				g.State = 6
				goto ExitAndDefault
			}

			// hash/generate new password inside user object
			_, err = g.U.GeneratePassword(newpassword)
			if err != nil {
				g.State = 6
				goto ExitAndDefault
			}

			if H.Debug {
				log.Println("Chaning password for", g.U.Email, g.U.UserID)
			}
			// if ok, then
			// change to new password
			err = satsql.UpdateUser(H, g.U, "password", g.U.PasswordHash)
			if err != nil {
				g.State = 7
				goto ExitAndDefault
			}
			// set to success for template engine
			g.State = 8
		} else if template.HTMLEscapeString(request.Form.Get("changeemail")) == "1" {
			// create new user object
			V := g.U
			// or change email address
			if V.SetEmail(template.HTMLEscapeString(request.Form.Get("emailnew"))) != nil {
				g.State = 2
				goto ExitAndDefault
			}
			_, err = satsql.SelectUser(H, "email", V.Email)
			// nil == User exists and was pulled
			// err != sql.ErrNoRows (other error),  else user does not exist
			if err == nil {
				g.State = 4
				goto ExitAndDefault
			} else if err != sql.ErrNoRows {
				g.State = 3
				goto ExitAndDefault
			}

			// update User in db
			err := satsql.UpdateUser(H, g.U, "email", V.Email)
			if err != nil {
				log.Println(err)
				goto ExitAndDefault
			}
			// set to success for template engine
			g.State = 8
			g.U = V
		}
	}

	// Default is GET method where we will print out the template
ExitAndDefault:
	executeGlobalAgainstTemplate(writer, "profile.html", g)

}

// logout takes care of the logout process
func logout(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// read current cookie
	c, err := request.Cookie("Session")

	// cookie does exist
	if err == nil {
		if c.Value != "" {
			err := satsql.DestroySession(H, c.Value)
			if err != nil {
				log.Println(err)
			}
		}
	}

	// create new useless cookie
	c = &http.Cookie{
		Name:   "Session",
		Value:  uuid.NewV4().String(),
		MaxAge: -1,
	}

	// overwrite session cookie and redirect to log in
	http.SetCookie(writer, c)
	http.Redirect(writer, request, "/login", http.StatusSeeOther)
	return
}

// agentsConfig pushes configuration from services
func agentsConfig(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {

	// check agents access key
	satAgent, allowed := CheckAgentAccessKey(request, H)
	if !allowed {
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	// Only accept GET
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	// return matching services from DB
	var location = satAgent.SatAgentLocation
	var onlyLocation = satAgent.SatOnlyLocation
	dbServices, err := satsql.ReadServices(H, 0, location, onlyLocation)
	if err != nil {
		log.Println(err)
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	// new json encoder
	err = json.NewEncoder(writer).Encode(dbServices)
	if err != nil {
		http.Error(writer, err.Error(), http.StatusInternalServerError)
		return
	}

}

// agentsResults takes services results from agents
func agentsResults(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler) {
	// only accept POST
	if request.Method != http.MethodPost {
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	// check agents access key
	_, allowed := CheckAgentAccessKey(request, H)
	if !allowed {
		writer.WriteHeader(http.StatusForbidden)
		return
	}

	// Read response and try to parse
	var agentResults []sattypes.ServiceResult
	err := json.NewDecoder(request.Body).Decode(&agentResults)
	if err != nil {
		log.Println(err)
	}

	// for every parsed result, ...
	for i := range agentResults {
		// ... send  result to analytics via golang channel
		sattypes.ResultsChannel <- agentResults[i]
	}

	// thank back the sender with a statusOK
	writer.WriteHeader(http.StatusOK)
	return

}

// CheckAgentAccessKey retrieves the agent node and key of satagent and tries to match it
// against a sql entry
func CheckAgentAccessKey(request *http.Request, H sattypes.BaseHandler) (sattypes.SatAgentSql, bool) {
	var newAgent bool

	// retrieve access-key from header
	accessKey := request.Header.Get("agent-key")
	if len(accessKey) == 0 {
		return sattypes.SatAgentSql{}, false
	}

	// retrieve agent-name and location from header
	accessNode := request.Header.Get("agent-name")
	locationNode := request.Header.Get("agent-location")

	// try to match node in database , if nil == token found
	switch err := satsql.SearchAgentAccessKey(H, accessNode, accessKey); err {
	case nil:
		// accessNode was found, continue
		log.Println("Matched global server key for", accessKey, accessNode, request.RemoteAddr)
		break
	case sql.ErrNoRows:
		if accessKey != H.SatKey {
			log.Println("Invalid key for client, stopping operation for", accessNode, request.RemoteAddr)
			return sattypes.SatAgentSql{}, false
		}
		// nothing was found, continue to global check
		break
	default:
		return sattypes.SatAgentSql{}, false
	}

	satAgent, err := satsql.SelectAgent(H, "satagent_name", accessNode)
	if err != nil {
		log.Println("SatAgentSql not known", err)
		err = nil
		newAgent = true
	}

	if newAgent {
		if H.Debug {
			log.Println("Creating SQL entry for sat agent", accessNode, accessKey, locationNode)
		}
		err = satsql.InsertAgent(H, accessNode, accessKey, locationNode)
		if err != nil {
			log.Println(err)
			return sattypes.SatAgentSql{}, false
		}
		satAgent, err = satsql.SelectAgent(H, "satagent_name", accessNode)
		if err != nil {
			log.Println("SatAgentSql still not known", err)
			return sattypes.SatAgentSql{}, false
		}
	}

	err = satsql.UpdateAgentLocation(H, locationNode, satAgent.SatAgentID)
	if err != nil {
		log.Println(err)
		return sattypes.SatAgentSql{}, false
	}

	if H.Debug {
		log.Println("Allowing access to satagent-node", accessNode)
	}

	// check if only location is set for this host
	onlyLocation := request.Header.Get("agent-onlylocation")
	if onlyLocation != "" && onlyLocation == "YES" {
		satAgent.SatOnlyLocation = true
	}

	return satAgent, true
}

// CheckCSRFToken check CSRF token from a form against expected CSRF from the usersession
func CheckCSRFToken(writer http.ResponseWriter, request *http.Request, H sattypes.BaseHandler, US sattypes.Session) bool {
	// check csrf against current session
	if request.FormValue("csrf") != US.CSRF {
		log.Println("CSRF is wrong, shall be", US.CSRF)
		writer.WriteHeader(http.StatusForbidden)
		return false
	}

	if H.Debug {
		log.Println("service_add / CSRF is right", US.CSRF, request.FormValue("csrf"))
	}

	return true
}

// generate random string from rune slice
func genrandom(length int) string {
	rand.Seed(time.Now().UnixNano())
	charAndSpecial := []rune("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789")
	var tempString strings.Builder
	for i := 0; i < length; i++ {
		tempString.WriteRune(charAndSpecial[rand.Intn(len(charAndSpecial))])
	}
	return tempString.String()
}
