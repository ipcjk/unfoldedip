CREATE TABLE IF NOT EXISTS "alertgroup"
(
	contact_id INTEGER not null
		primary key autoincrement,
	groupname TEXT,
	emails TEXT,
	owner_id int
);
CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE IF NOT EXISTS "satagents"
(
	satagent_id integer not null
		constraint satagents_pk
			primary key autoincrement,
	satagent_name varchar default "something",
	access_key varchar default "" not null
, satagent_location varchar default "", lastseen string default "", locationfixed integer default 0);
CREATE TABLE IF NOT EXISTS "sessions"
(
	csrf string,
	sessionid int,
	userid int,
	last_activity text
);
CREATE TABLE IF NOT EXISTS "service_log"
(
	service_id INTEGER,
	status_date TEXT,
	status_from TEXT,
	status_to text,
	status_why text
);
CREATE TABLE IF NOT EXISTS "users"
(
	reset text default "",
	passwordnext text default "",
	id INTEGER not null /*autoincrement needs PK*/
		unique
		primary key autoincrement,
	email TEXT not null,
	password TEXT not null,
	admin int default 0
);
CREATE TABLE IF NOT EXISTS "services"
(
	last_event text default "",
	contact_group INTEGER,
	service_expected text default "",
	interval INTEGER,
	owner_id int,
	service_state string default "" not null,
	service_id INTEGER not null
		primary key autoincrement
		unique,
	service_type TEXT not null,
	service_tocheck TEXT,
	service_name text default "",
	testlocations string default "any"
, last_seen text default "");
