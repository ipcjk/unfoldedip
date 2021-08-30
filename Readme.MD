## Unfolded
Unfolded is a small, distributed monitoring tool for checking
the availability of hosts or single services (TCP, HTTP) in one
or several networks.

It works by starting a central HTTP server that is receiving
monitoring results by unfolded agents, Therefore the HTTP and
the agents can be region isolated. The central HTTP server can
be run behind a reverse proxy for SSL offloading like Nginx or
Apache webserver.

### How to start
The agent thread runs inside the HTTP server for smaller
or local setups, therefore the easiest way to start is
to run the unfolded program without any arguments.

### Download compiled version
Check the latest release on the release tab, then process
to download it and extract the contents.

### Start the simple way

After extraction, you  run directly from the path:

For Linux
`./unfolded.linux`

For Mac
`./unfolded.darwin`

For Raspberry Pi
`./unfolded.pi`

For Windows
`./unfolded.exe`

This will start unfolded  HTTP server and a local agent thread on *127.0.0.1* or *localhost* on port *8080*. You can
then connect with your favorite browser. Your monitoring solution is ready. You can go ahead, point your browser to
http://localhost:8080 and register a user account.

### How to build your own version or join the development circle

#### Download Go
First, download and install
a recent version of Go from https://golang.org/dl/ for your OS.

#### Clone repo
Then, you need to clone the
repository and install the dependencies.

$ git clone https://github.com/ipcjk/unfoldedip/

#### Install / update all necessary modules

$ go mod tidy

#### Run unit tests

$ go test -v ./...

#### Build your own binary  
$ go build

#### Run, run, run  
$ ./unfoldedip

#### Unit tests



### Start the complex way

In a real-world scenario, you would like to run the HTTP server
on a central server inside a data center and let unfolded agents from worldwide
connect to it.

It may be necessary to start the unfolded HTTP server with some additional arguments. Let us take a look at a complete example.

In this case, we will start the unfolded HTTP server with a web server running on the IP address, *46.232.189.24* and port *3000*, and we will also generate client URLs beginning with *http://46.232.189.24:3000*.

This is done by setting the parameter *http* and *serverurl*.
`./unfolded.linux -http 46.232.189.24:3000 -serverurl http://46.232.189.24:3000`

By default, the central server starts a checking agent with the default hostname and location. To adjust, you can overwrite them on the command line and set it to your server's name and location.

This
is done by setting the parameters *agent name* and *agentloc*. In the same call, we also adjust a global access control
key for future agent connects by setting the parameter *agentkey* to *253121*.

`./unfolded.linux -http 46.232.189.24:3000 -serverurl http://46.232.189.24:3000 -agentkey 253121 -agentname muc1 -agentloc Munich`

Now remote agents can connect by pinning them to the *serverurl* path and the access-key
*253121*.

### Agent setup

First, you need to copy one of the matching binaries for your platform to the small side for the agent.  For example, to use the Raspberry Pi, you would like  to copy the binary with the suffix *.pi*:

`SCP unfolded.pi pi@raspberry:`

On this machine, you need to point it back to the central server by supplying the *serverurl*-
parameter and the agent-key values from the server command. You should also set a location and an agent name (if not, the hostname is used). A complete, working example that matches the server call from
above looks like this:

`./unfolded.pi -server=false -agentkey 253121 -serverurl http://46.232.189.24:3000 -agentloc Munich-Trudering -agentname vodafone-cable`

If the connection were successful, you would see a similar output like this on the terminal:

`2021/08/15 09:47:37 satagent.go:262: --- satagent Vodafone-cable (Munich-Trudering):  retrieved configuration`

The server will automatically register this agent with it's key and its location to the database, and a new test location will be visible in the *service-add* menu.

#### location-only services

Suppose you want agents to check services on its local segment and not on the worldwide Internet. In that case, you can add parameter *onlylocation* with the value true as value. *-onlylocation=true*

For example, the agent starting up with these command line parameters will only check services that are configured with the location *Munich-Trudering* from the web panel and will ignore any other areas.

`./unfolded.pi -server=false -onlylocation=true -agentkey 253121 -serverurl http://46.232.189.24:3000 -agentloc Munich-Trudering -agentname vodafone-cable`

#### Agent access control

By default, the agent reads the access-key for accessing the server from the command line.  This is like a shared secret
between server and agents. After the first connection, there will be an row inside the sqlite-database for every connected
agents. Here you can modify the access keys for the agent with the sqlite - client or additional tools like SQLite-browser
inside the table called *satagents*:

    SQLite version 3.28.0 2019-04-15 14:49:49lite
    sqlite> .headers on
    sqlite> select * from satagents;
    satagent_id|satagent_name|access_key|satagent_location|lastseen|locationfixed
    90|muc1|1234|Munich|2021-08-15|0
    91|fra1|37199314481|Frankfurt|2021-08-13|0
    sqlite> update satagents set access_key="371zfh37372" where satagent_id=90;

The HTTP server will always accept an agent connected with the default key by default and in the current code. So you are encouraged to change the value of the parameter *-agentkey* on startup from the webserver.

#### ping checks
Ping is called by the agent an external tool and needs to be installed ahead. For example, in Debian-based systems, the package *iputils-ping* needs to be installed. Ping is currently not supported on Windows agents.

### Security notice for multiuser systems

In the current design, all parameters for servers and agents are valued on the command line. Therefore all parameters could be visible to users on the same system. It is about to change in future versions.

### Configuring SMTP connector

To use email notifications for service events and also to use *forget-password*-function, you need to configure an SMTP connector. This is currently done on the command line by setting the following arguments on the central
server:

- SMTP containing the hostname and the TCP port of a valid SMTP server in the form *host:port*
- smtpuser login for SMTP authentication
- smtppass password for SMTP authentication
- smtpsender sender email source for all mails

### Real life setups
In real life, you would likely run the service behind a reverse proxy with
Apache or Nginx. Here you can also add SSL encryption and use additional features like limiting access to the */agents*- URI path.

Please see the file *Nginx-sample.conf* in the distribution as an example.

A complete example behind a reverse proxy and a working SMTP connector looks like this:

`./unfolded.linux  -http 127.0.0.1:3000 -serverurl https://icmp.info -agentkey 253121 -agentname muc1 -agentloc Munich   -smtp icmp.info:25 -smtpuser unfolded -smtppass gj4782371 -smtpsender unfolded@icmp.info`

### Additional arguments

Please call the binary with the *-h* - parameter to see additional, helpful arguments:


    Usage of ./unfolded.pi:
      -agent
            satellite (satagent) mode only (default true)
      -agentkey string
            shared access key for submitting to the satkey (default "0000")
      -agentloc string
            satagent location (default "Munich")
      -agentname string
        satagent name (default "muc1")
      -db string
        path to the sqlite database filer (server) (default "unfolded.sqlite")
      -debug
        turns on debug mode
      -http string
        port for the default listener  (server) (default "127.0.0.1:8080")
      -onlylocation
        boolean to control, if the agent can do any check or only for his location
      -server
        server / http mode enabled, -server=false for disabling (default true)
      -serverurl string
        url for satserver (default "http://localhost:8080")
      -smtp string
        server for smtp sendmail function
      -smtpPass string
        password for smtp authentication
      -smtpUser string
        login for smtp authentication
      -smtpsender string
        sender email source for all mails

## When will service be down or up?

Deciding when a service is transiting to a new state is hard. In this code, we use a very simplistic way. When a service result arrives, we save the
service condition in a 64-bit long integer and shift the bits to the left. A  shift with 1 means the service is down. A 0 means the service is up. When the code sees 0xF (4x 1-bit set) on the LSB, the service is stated as down. When the code sees 0x0 (4x0 bit set), the service is stated to be up. Because we will receive events from different sources, this hopefully leads to an efficient and convincing result.

Other improvements are thinkable, for example, calling a third different source for a second guess on the service.

## License
MIT © 2020
