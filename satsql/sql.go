package satsql

import (
	"database/sql"
	"fmt"
	"log"
	"time"
	"unfoldedip/sattypes"
)

// InsertService inserts a new service into the database
func InsertService(H sattypes.BaseHandler, s *sattypes.Service) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("INSERT into services (service_type, service_name, " +
		"service_tocheck, interval, contact_group, owner_id, service_expected, testlocations) values(?,?,?,?,?,?,?,?)")

	if err != nil {
		return err
	}
	defer stmt.Close()

	res, err := stmt.Exec(s.Type, s.Name, s.ToCheck, s.Interval, s.ContactGroup, s.OwnerID, s.Expected, s.Locations)
	if err != nil {
		return err
	}

	s.ServiceID, err = res.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

// UpdateService updates an old service inside the database
func UpdateService(H sattypes.BaseHandler, s *sattypes.Service) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("update services set service_type=?, service_name=?, " +
		"service_tocheck=?, interval=?, contact_group=?, service_expected=?, testlocations=? where service_id=?")

	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(s.Type, s.Name, s.ToCheck, s.Interval, s.ContactGroup, s.Expected, s.Locations, s.ServiceID)
	if err != nil {
		return err
	}

	return nil
}

// InsertAlertGroup inserts a new contact into the database
func InsertAlertGroup(H sattypes.BaseHandler, c *sattypes.AlertGroup) error {

	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("INSERT into alertgroup (groupname, " +
		"emails, owner_id) values(?,?,?)")

	if err != nil {
		return err
	}
	defer stmt.Close()

	// Run the query
	res, err := stmt.Exec(c.GroupName, c.Emails, c.OwnerID)
	if err != nil {
		return err
	}

	c.ContactID, err = res.LastInsertId()
	if err != nil {
		return err
	}

	return nil
}

// UpdateAlertGroup updates a  contact into the database
func UpdateAlertGroup(H sattypes.BaseHandler, c *sattypes.AlertGroup) error {

	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("update alertgroup set groupname=?,emails=? where contact_id=?")

	if err != nil {
		return err
	}
	defer stmt.Close()

	// Run the query
	_, err = stmt.Exec(c.GroupName, c.Emails, c.ContactID)
	if err != nil {
		return err
	}

	return nil
}

// SelectUser searches and returns user record by argument
func SelectUser(H sattypes.BaseHandler, arg string, argValue string) (sattypes.UnfoldedUser, error) {
	var User sattypes.UnfoldedUser

	// run query
	rows := H.DB.QueryRow(fmt.Sprintf("select id, email, password, \"true\", passwordnext, reset from users where %s = ?", arg), argValue)

	// return empty user struct and error code on error
	switch err := rows.Scan(&User.UserID, &User.Email, &User.PasswordHash, &User.Exists, &User.PasswordHashNext, &User.Reset); err {
	case sql.ErrNoRows:
		return sattypes.UnfoldedUser{}, sql.ErrNoRows
	case nil:
		// return filled user struct
		return User, nil
	default:
		return sattypes.UnfoldedUser{}, err
	}
}

// SelectAlertGroup searches and returns user record by argument
func SelectAlertGroup(H sattypes.BaseHandler, arg string, argValue string) (sattypes.AlertGroup, error) {
	var alertgroup sattypes.AlertGroup

	// run query
	rows := H.DB.QueryRow(
		fmt.Sprintf("select contact_id, groupname, emails, owner_id, \"true\" from alertgroup where %s = ?", arg),
		argValue)

	// return empty user struct and error code on error
	switch err := rows.Scan(&alertgroup.ContactID,
		&alertgroup.GroupName, &alertgroup.Emails, &alertgroup.OwnerID, &alertgroup.Exists); err {
	case sql.ErrNoRows:
		return sattypes.AlertGroup{}, sql.ErrNoRows
	case nil:
		// return filled user struct
		return alertgroup, nil
	default:
		return sattypes.AlertGroup{}, err
	}
}

// DeleteService deletes a service record selected by its id
func DeleteService(H sattypes.BaseHandler, argValue int64) error {
	// prepare statement
	stmt, err := H.DB.Prepare("delete from services where service_id =  ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute prepared statement
	_, err = stmt.Exec(argValue)
	return err
}

// DeleteService deletes a service record selected by its id
func DeleteServiceLogs(H sattypes.BaseHandler, argValue int64) error {
	// prepare statement
	stmt, err := H.DB.Prepare("delete from service_log where service_id =  ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	// execute prepared statement
	_, err = stmt.Exec(argValue)
	return err
}

// DeleteAlertGroup deletes an alertgroup record by argument
func DeleteAlertGroup(H sattypes.BaseHandler, argValue int64) error {
	// prepare statement
	stmt, err := H.DB.Prepare("delete from alertgroup where contact_id =  ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	// execute prepared statement
	_, err = stmt.Exec(argValue)
	return err
}

// ResetService resets a service record selected by its id
func ResetService(H sattypes.BaseHandler, argValue int64) error {
	// prepare statement
	stmt, err := H.DB.Prepare("update services set service_state='SERVICE_UNKNOWN' where service_id =  ?")
	if err != nil {
		log.Println(err)
		return err
	}
	defer stmt.Close()

	// execute prepared statement
	_, err = stmt.Exec(argValue)
	return err
}

// ReadServicesLog searches and return service logs for all services from an ownerId
func ReadServicesLog(H sattypes.BaseHandler, ownerID int64) ([]sattypes.ServiceLog, error) {
	var serviceLogs []sattypes.ServiceLog
	var stmt *sql.Stmt
	var rows *sql.Rows
	var err error

	// prepare query
	stmt, err = H.DB.Prepare("select service_log.service_id, service_name, service_tocheck, status_date, " +
		"status_from, status_to, status_why from service_log inner join services " +
		"on service_log.service_id=services.service_id where services.owner_id=? order by status_date desc limit 500")

	// return empty and error code on error
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	// run query
	rows, err = stmt.Query(ownerID)

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		var s sattypes.ServiceLog
		err := rows.Scan(
			&s.ServiceID, &s.Name, &s.ToCheck, &s.Date, &s.Status_From, &s.Status_To, &s.Why)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
		serviceLogs = append(serviceLogs, s)
	}

	// return empty slice and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled  slice
	return serviceLogs, nil

}

// ReadServiceLogs searches and return service logs for a service
func ReadServiceLogs(H sattypes.BaseHandler, argValue string) ([]sattypes.ServiceLog, error) {
	var serviceLogs []sattypes.ServiceLog
	var stmt *sql.Stmt
	var rows *sql.Rows
	var err error

	// prepare query
	stmt, err = H.DB.Prepare("select service_id, status_date, status_from, status_to, status_why from service_log " +
		"where service_id=? order by status_date desc")
	// return empty user struct and error code on error
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query(argValue)

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		var s sattypes.ServiceLog
		err := rows.Scan(
			&s.ServiceID, &s.Date, &s.Status_From, &s.Status_To, &s.Why)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
		serviceLogs = append(serviceLogs, s)
	}

	// return empty slice and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled  slice
	return serviceLogs, nil

}

// SelectService searches and return service record by argument
func SelectService(H sattypes.BaseHandler, arg string, argValue string, ownerid int64) (sattypes.Service, error) {
	var s sattypes.Service
	var row *sql.Row

	if ownerid != 0 {
		// run query
		row = H.DB.QueryRow(
			fmt.Sprintf("select service_id, service_name, service_tocheck, service_type,"+
				"\"true\", owner_id, service_state, service_expected, interval, ifnull(contact_group,0), testlocations  from services where %s = ? and owner_id=?", arg),
			argValue, ownerid)
	} else {
		row = H.DB.QueryRow(
			fmt.Sprintf("select service_id, service_name, service_tocheck, service_type,"+
				"\"true\", owner_id, service_state, service_expected, interval,  ifnull(contact_group,0), testlocations from services where %s = ? and owner_id!=?", arg),
			argValue, ownerid)
	}

	// return empty user struct and error code on error
	switch err := row.Scan(&s.ServiceID, &s.Name, &s.ToCheck, &s.Type, &s.Exists, &s.OwnerID, &s.ServiceState,
		&s.Expected, &s.Interval, &s.ContactGroup, &s.Locations); err {
	case sql.ErrNoRows:
		return sattypes.Service{}, sql.ErrNoRows
	case nil:
		// return filled user struct
		return s, nil
	default:
		return sattypes.Service{}, err
	}
}

// UpdateServiceState updates a service into the database with the last_seen column
func UpdateServiceLastSeenNow(H sattypes.BaseHandler, serviceID int64) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("UPDATE services set last_seen = ? where service_id = ?")

	defer stmt.Close()
	if err != nil {
		return err
	}

	_, err = stmt.Exec(time.Now().String(), serviceID)
	if err != nil {
		return err
	}

	return nil
}

// UpdateServiceState updates a new service into the database
func UpdateServiceState(H sattypes.BaseHandler, serviceID int64, state string) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("UPDATE services set service_state = ?, last_event = datetime('NOW') where service_id = ?")

	if err != nil {
		return err
	}

	_, err = stmt.Exec(state, serviceID)
	if err != nil {
		return err
	}

	return nil
}

// InsertService inserts a new service into the database
func InsertServiceChange(H sattypes.BaseHandler, r sattypes.ServiceResult) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("INSERT into service_log (service_id, status_date, " +
		"status_from, status_to, status_why) values(?,CURRENT_TIMESTAMP,?,?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	var oldState, newState string

	if r.Status == sattypes.ServiceUP {
		newState = "UP"
		oldState = "DOWN"
	} else {
		newState = "DOWN"
		oldState = "UP"
	}

	_, err = stmt.Exec(r.ServiceID, oldState, newState, fmt.Sprintf("%s [%s]: %s", r.TestNode, r.Time, r.Message))
	if err != nil {
		return err
	}

	return nil
}

// InsertUser inserts a User into the database
func InsertUser(H sattypes.BaseHandler, U *sattypes.UnfoldedUser) error {

	// search user by email
	_, err := SelectUser(H, "email", U.Email)
	// abort if there is any error except sqlErrNoRows (no user record found)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	// prepare statement
	stmt, err := H.DB.Prepare("INSERT into users (email, password) values(?,?)")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute prepared statement
	res, err := stmt.Exec(U.Email, U.PasswordHash)
	if err != nil {
		return err
	}

	// retrieve insert id
	U.UserID, err = res.LastInsertId()
	if err != nil {
		return err
	}

	// return nil
	return nil
}

// UpdateUser updates a user
func UpdateUser(H sattypes.BaseHandler, U sattypes.UnfoldedUser, arg, argvalue string) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare(fmt.Sprintf("UPDATE users set %s = ? where id = ?", arg))
	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute statement
	_, err = stmt.Exec(argvalue, U.UserID)
	if err != nil {
		return err
	}

	return nil
}

// ReadServices reads all services from db with some possibility to limit by owner and location (for agents for example)
func ReadServices(H sattypes.BaseHandler, ownerID int64, location string, onlyLocation bool) ([]sattypes.Service, error) {
	var services []sattypes.Service
	var err error
	var stmt *sql.Stmt
	var rows *sql.Rows
	var s sattypes.Service

	// if ownerID == 0, we will read all services
	if ownerID == 0 {
		var sqlStatement = "select service_id, service_type, service_name, service_tocheck, contact_group, interval, " +
			"ifnull(contact_group,''), service_state, ifnull(service_expected,''), last_event  from services "
		// expand sql on arguments
		if location != "" && onlyLocation {
			sqlStatement += " where (' ' || testlocations || ' ') like ?"
		} else if location != "" && !onlyLocation {
			sqlStatement += " where (' ' || testlocations || ' ') like ? or testlocations = 'any'"
		}
		if H.Debug {
			log.Println(sqlStatement)
		}
		stmt, err = H.DB.Prepare(sqlStatement)
	} else {
		stmt, err = H.DB.Prepare(fmt.Sprintf("select service_id, service_type, service_name, service_tocheck, " +
			"contact_group, interval,  ifnull(alertgroup.groupname,''), service_state, ifnull(service_expected,'')," +
			"last_event from services left join alertgroup on services.contact_group=alertgroup.contact_id " +
			"where services.owner_id = ? order by service_state, last_event desc, service_id desc"))
	}

	defer stmt.Close()
	// return empty  and error code on error
	if err != nil {
		return nil, err
	}

	// run query
	if ownerID == 0 {
		if location == "" {
			rows, err = stmt.Query()
		} else {
			rows, err = stmt.Query(fmt.Sprintf("%% %s %%", location))
		}
	} else {
		rows, err = stmt.Query(ownerID)
	}
	defer rows.Close()
	// return empty  and error code on error
	if err != nil {
		return nil, err
	}

	// scan up all rows
	for rows.Next() {
		err := rows.Scan(
			&s.ServiceID, &s.Type, &s.Name, &s.ToCheck, &s.ContactGroup,
			&s.Interval, &s.AlertGroupName, &s.ServiceState, &s.Expected, &s.LastEvent)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
		services = append(services, s)
	}

	// return empty  and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled
	return services, nil
}

// ReadAlertGroups reads all alert groups from DB
func ReadAlertGroups(H sattypes.BaseHandler, ownerID int64) ([]sattypes.AlertGroup, error) {
	var contacts []sattypes.AlertGroup

	stmt, err := H.DB.Prepare(fmt.Sprintf("select contact_id, groupname, emails, owner_id from " +
		"alertgroup where owner_id = ? order by  groupname asc"))
	defer stmt.Close()
	// return empty user struct and error code on error
	if err != nil {
		return nil, err
	}

	// run query
	rows, err := stmt.Query(ownerID)
	// return empty user struct and error code on error
	if err != nil {
		return nil, err
	}

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		var c sattypes.AlertGroup
		err := rows.Scan(&c.ContactID, &c.GroupName, &c.Emails, &c.OwnerID)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
		contacts = append(contacts, c)
	}

	// return empty user struct and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled user struct
	return contacts, nil
}

// SearchAgentAccessKey searches for valid satagent key
func SearchAgentAccessKey(H sattypes.BaseHandler, accessNode, accessKey string) error {
	var satAgentID int64
	// run query
	rows := H.DB.QueryRow("select satagent_id from satagents where access_key = ? and satagent_name=?",
		accessKey, accessNode)

	// return empty user struct and error code on error
	switch err := rows.Scan(&satAgentID); err {
	case sql.ErrNoRows:
		return sql.ErrNoRows
	case nil:
		// return filled user struct
		return nil
	default:
		return err
	}
}

// SelectAgent searches and returns agent record by argument
func SelectAgent(H sattypes.BaseHandler, arg string, argValue string) (sattypes.SatAgentSql, error) {
	var Agent sattypes.SatAgentSql

	var query = fmt.Sprintf(
		"select satagent_id, satagent_name, satagent_location, access_key, lastseen from satagents where %s = ?",
		arg)
	// run query
	rows := H.DB.QueryRow(query, argValue)

	// return empty user struct and error code on error
	switch err := rows.Scan(&Agent.SatAgentID, &Agent.SatAgentName, &Agent.SatAgentLocation, &Agent.AccessKey,
		&Agent.LastSeen); err {
	case sql.ErrNoRows:
		return sattypes.SatAgentSql{}, sql.ErrNoRows
	case nil:
		// return filled user struct
		return Agent, nil
	default:
		return sattypes.SatAgentSql{}, err
	}
}

// InsertAgent insert an agent record
func InsertAgent(H sattypes.BaseHandler, agentName, accessKey, agentLocation string) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare("INSERT into satagents" +
		" (satagent_name, satagent_location, access_key, lastseen) values(?,?,?, datetime('NOW'))")

	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(agentName, agentLocation, accessKey)
	if err != nil {
		return err
	}

	return nil

}

// UpdateAgent updates a loation and lastseen of an agent
func UpdateAgentLocation(H sattypes.BaseHandler, agentLocation, agentId string) error {
	// prepare insert query for sqlite*/
	stmt, err := H.DB.Prepare(
		"update satagents set satagent_location=?, lastseen=datetime('now') where satagent_id=?")
	defer stmt.Close()
	if err != nil {
		return err
	}

	_, err = stmt.Exec(agentLocation, agentId)
	if err != nil {
		return err
	}

	return nil
}

// ReadAgents  returns a slice of possible sat agents
func ReadAgents(H sattypes.BaseHandler) ([]sattypes.SatAgentSql, error) {
	var agents []sattypes.SatAgentSql
	var stmt *sql.Stmt
	var rows *sql.Rows
	var err error

	// prepare query to show the agents, that have been active for last -5 days
	stmt, err = H.DB.Prepare(
		"select satagent_id, satagent_name satagent_location from satagents where lastseen >= date('now', '-5 day')")

	// return empty and error code on error
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query()

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		var s sattypes.SatAgentSql
		err := rows.Scan(
			&s.SatAgentID, &s.SatAgentName, &s.SatAgentLocation)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
	}

	// return empty slice and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled  slice
	return agents, nil

}

// ReadAgentLocations  returns a slice of possible sat agent regions
func ReadAgentLocations(H sattypes.BaseHandler) ([]string, error) {
	var locations []string
	var location string
	var stmt *sql.Stmt
	var rows *sql.Rows
	var err error

	// prepare query to show the agents, that have been active for last -5 days
	stmt, err = H.DB.Prepare("select distinct satagent_location from satagents where lastseen >= date('now', '-5 day')")

	// return empty and error code on error
	if err != nil {
		return nil, err
	}
	defer stmt.Close()

	rows, err = stmt.Query()

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		err := rows.Scan(
			&location)
		// return empty user struct and error code on error
		if err != nil {
			return nil, err
		}
		locations = append(locations, location)
	}

	// return empty slice and error code on error
	if err = rows.Err(); err != nil {
		return nil, err
	}

	// return filled  slice
	return locations, nil

}

// CheckEmptySession returns a csrf for a session, when Session exist, else returns an error
// used to check for login and register forms only
func CheckEmptySession(H sattypes.BaseHandler, uuid string) (string, error) {

	var csrf string
	stmt, err := H.DB.Prepare(fmt.Sprintf("select csrf from sessions where %s = ?", "sessionid"))
	if err != nil {
		return "", err
	}
	defer stmt.Close()

	// run query
	rows, err := stmt.Query(uuid)
	// return empty user struct and error code on error
	if err != nil {
		return "", err
	}

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		err := rows.Scan(&csrf)
		// return empty user struct and error code on error
		if err != nil {
			return "", err
		}
	}

	// return empty user struct and error code on error
	if err = rows.Err(); err != nil {
		return "", err
	}

	// return filled user session
	return csrf, nil
}

// CheckUserSession returns user id from session uuid else return zero and error code on error
func CheckUserSession(H sattypes.BaseHandler, uuid string) (sattypes.Session, error) {
	var UserSession sattypes.Session

	// check for real sessions where userid is not equal 0
	stmt, err := H.DB.Prepare(fmt.Sprintf("select userid, csrf, sessionid from sessions where %s = ? and userid != 0", "sessionid"))

	if err != nil {
		return sattypes.Session{}, err
	}
	defer stmt.Close()

	// run query
	rows, err := stmt.Query(uuid)
	// return empty user struct and error code on error
	if err != nil {
		return sattypes.Session{}, err
	}

	// prepare rows close on exit
	defer rows.Close()
	// scan up all rows
	for rows.Next() {
		err := rows.Scan(&UserSession.UserID, &UserSession.CSRF, &UserSession.SessionID)
		// return empty user struct and error code on error
		if err != nil {
			return sattypes.Session{}, err
		}
	}

	// return empty user struct and error code on error
	if err = rows.Err(); err != nil {
		return sattypes.Session{}, err
	}

	// return filled user session
	return UserSession, nil
}

// UpdateSession updates activity timepoint for a session
func UpdateSession(H sattypes.BaseHandler, Usersession sattypes.Session) error {
	// prepare insert query for the new session
	stmt, err := H.DB.Prepare("update sessions set last_activity = CURRENT_TIMESTAMP where sessionid = ?")

	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute statement
	_, err = stmt.Exec(Usersession.SessionID)
	if err != nil {
		return err
	}

	return nil
}

// DestroySession by removing the sessionid from table
func DestroySession(H sattypes.BaseHandler, sessionid string) error {
	// prepare insert query for the new session
	stmt, err := H.DB.Prepare("delete from sessions where sessionid = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute statement
	_, err = stmt.Exec(sessionid)
	if err != nil {
		return err
	}

	return nil
}

// DestroySessions cleans up old sessions, that show no activity for over 24 hours
func DestroySessions(H sattypes.BaseHandler) error {
	// prepare query
	stmt, err := H.DB.Prepare("delete from sessions where last_activity < date('now', '-1 day')")

	if err != nil {
		return err
	}
	defer stmt.Close()

	// execute statement
	_, err = stmt.Exec()
	if err != nil {
		return err
	}

	return nil
}

// NewSession creates a new session for a given user id
func NewSession(H sattypes.BaseHandler, user sattypes.UnfoldedUser, uuid string, csrf string) error {
	// prepare insert query for the new session
	stmt, err := H.DB.Prepare("INSERT into sessions (sessionid, userid, csrf, last_activity) values(?,?,?,CURRENT_TIMESTAMP)")

	if err != nil {
		return err
	}
	defer stmt.Close()

	// insert / execs against database
	_, err = stmt.Exec(uuid, user.UserID, csrf)
	if err != nil {
		return err
	}

	return nil
}
