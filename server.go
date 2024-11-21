// Notes:
package ctrl

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/jinzhu/copier"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

type processName string

// Will return a process name made up of subjectName+processKind
func processNameGet(sn subjectName, pk processKind) processName {
	pn := fmt.Sprintf("%s_%s", sn, pk)
	return processName(pn)
}

// server is the structure that will hold the state about spawned
// processes on a local instance.
type server struct {
	// The main background context
	ctx context.Context
	// The CancelFunc for the main context
	cancel context.CancelFunc
	// Configuration options used for running the server
	configuration *Configuration
	// The nats connection to the broker
	natsConn *nats.Conn
	// net listener for communicating via the ctrl socket
	ctrlSocket net.Listener
	// processes holds all the information about running processes
	processes *processes
	// The name of the node
	nodeName string
	// toRingBufferCh are the channel where new messages in a bulk
	// format (slice) are put into the system.
	//
	// In general the ringbuffer will read this
	// channel, unfold each slice, and put single messages on the buffer.
	newMessagesCh chan []subjectAndMessage
	// directSAMSCh
	samSendLocalCh chan []subjectAndMessage
	// errorKernel is doing all the error handling like what to do if
	// an error occurs.
	errorKernel *errorKernel
	// metric exporter
	metrics *metrics
	// Version of package
	version string
	// processInitial is the initial process that all other processes are tied to.
	processInitial process

	// nodeAuth holds all the signatures, the public keys and other components
	// related to authentication on an individual node.
	nodeAuth *nodeAuth
	// helloRegister is a register of all the nodes that have sent hello messages
	// to the central server
	helloRegister *helloRegister
	// holds the logic for the central auth services
	centralAuth *centralAuth
	// message ID
	messageID messageID
	// audit logging
	auditLogCh chan []subjectAndMessage
}

type messageID struct {
	id int
	mu sync.Mutex
}

// newServer will prepare and return a server type
func NewServer(configuration *Configuration, version string) (*server, error) {
	// Set up the main background context.
	ctx, cancel := context.WithCancel(context.Background())

	metrics := newMetrics(configuration.PromHostAndPort)

	// Start the error kernel that will do all the error handling
	// that is not done within a process.
	errorKernel := newErrorKernel(ctx, metrics, configuration)

	var opt nats.Option

	if configuration.RootCAPath != "" {
		opt = nats.RootCAs(configuration.RootCAPath)
	}

	switch {
	case configuration.NkeySeed != "":
		cwd, err := os.Getwd()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to get current working directory when creating tmp seed file: %v", err)
		}

		pth := filepath.Join(cwd, "seed.txt")

		// f, err := os.CreateTemp(pth, "")
		// if err != nil {
		// 	return nil, fmt.Errorf("error: failed to create tmp seed file: %v", err)
		// }

		err = os.WriteFile(pth, []byte(configuration.NkeySeed), 0700)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to write temp seed file: %v", err)
		}

		opt, err = nats.NkeyOptionFromSeed(pth)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to read temp nkey seed file: %v", err)
		}

		defer func() {
			err = os.Remove(pth)
			if err != nil {
				cancel()
				log.Fatalf("error: failed to remove temp seed file: %v\n", err)
			}
		}()

	case configuration.NkeySeedFile != "" && configuration.NkeyFromED25519SSHKeyFile == "":
		var err error

		opt, err = nats.NkeyOptionFromSeed(configuration.NkeySeedFile)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to read nkey seed file: %v", err)
		}

	case configuration.NkeyFromED25519SSHKeyFile != "":
		var err error

		opt, err = configuration.nkeyOptFromSSHKey()
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to read nkey seed file: %v", err)
		}

	}

	var conn *nats.Conn

	// Connect to the nats server, and retry until succesful.
	for {
		var err error
		// Setting MaxReconnects to -1 which equals unlimited.
		conn, err = nats.Connect(configuration.BrokerAddress,
			opt,
			//nats.FlusherTimeout(time.Second*10),
			nats.MaxReconnects(-1),
			nats.ReconnectJitter(time.Duration(configuration.NatsReconnectJitter)*time.Millisecond, time.Duration(configuration.NatsReconnectJitterTLS)*time.Second),
			nats.Timeout(time.Second*time.Duration(configuration.NatsConnOptTimeout)),
		)
		// If no servers where available, we loop and retry until succesful.
		if err != nil {
			log.Printf("error: could not connect, waiting %v seconds, and retrying: %v\n", configuration.NatsConnectRetryInterval, err)
			time.Sleep(time.Duration(time.Second * time.Duration(configuration.NatsConnectRetryInterval)))
			continue
		}

		break
	}

	log.Printf(" * conn.Opts.ReconnectJitterTLS: %v\n", conn.Opts.ReconnectJitterTLS)
	log.Printf(" * conn.Opts.ReconnectJitter: %v\n", conn.Opts.ReconnectJitter)

	var ctrlSocket net.Listener
	var err error

	// Check if tmp folder for socket exists, if not create it
	if _, err := os.Stat(configuration.SocketFolder); os.IsNotExist(err) {
		err := os.MkdirAll(configuration.SocketFolder, 0770)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to create socket folder directory %v: %v", configuration.SocketFolder, err)
		}
	}

	// Open the ctrl socket file, and start the listener if enabled.
	if configuration.EnableSocket {
		ctrlSocket, err = createSocket(configuration.SocketFolder, "ctrl.sock")
		if err != nil {
			cancel()
			return nil, err
		}
	}

	//var nodeAuth *nodeAuth
	//if configuration.EnableSignatureCheck {
	nodeAuth := newNodeAuth(configuration, errorKernel)
	// fmt.Printf(" * DEBUG: newServer: signatures contains: %+v\n", signatures)
	//}

	//var centralAuth *centralAuth
	//if configuration.IsCentralAuth {
	centralAuth := newCentralAuth(configuration, errorKernel)
	//}

	s := server{
		ctx:            ctx,
		cancel:         cancel,
		configuration:  configuration,
		nodeName:       configuration.NodeName,
		natsConn:       conn,
		ctrlSocket:     ctrlSocket,
		newMessagesCh:  make(chan []subjectAndMessage),
		samSendLocalCh: make(chan []subjectAndMessage),
		metrics:        metrics,
		version:        version,
		errorKernel:    errorKernel,
		nodeAuth:       nodeAuth,
		helloRegister:  newHelloRegister(),
		centralAuth:    centralAuth,
		auditLogCh:     make(chan []subjectAndMessage),
	}

	s.processes = newProcesses(ctx, &s)

	// Create the default data folder for where subscribers should
	// write it's data, check if data folder exist, and create it if needed.
	if _, err := os.Stat(configuration.SubscribersDataFolder); os.IsNotExist(err) {
		if configuration.SubscribersDataFolder == "" {
			return nil, fmt.Errorf("error: subscribersDataFolder value is empty, you need to provide the config or the flag value at startup %v: %v", configuration.SubscribersDataFolder, err)
		}
		err := os.Mkdir(configuration.SubscribersDataFolder, 0770)
		if err != nil {
			return nil, fmt.Errorf("error: failed to create data folder directory %v: %v", configuration.SubscribersDataFolder, err)
		}

		er := fmt.Errorf("info: creating subscribers data folder at %v", configuration.SubscribersDataFolder)
		s.errorKernel.logDebug(er)
	}

	return &s, nil

}

// helloRegister is a register of all the nodes that have sent hello messages.
type helloRegister struct {
}

func newHelloRegister() *helloRegister {
	h := helloRegister{}

	return &h
}

// create socket will create a socket file, and return the net.Listener to
// communicate with that socket.
func createSocket(socketFolder string, socketFileName string) (net.Listener, error) {

	// Just as an extra check we eventually delete any existing ctrl socket files if found.
	socketFilepath := filepath.Join(socketFolder, socketFileName)
	if _, err := os.Stat(socketFilepath); !os.IsNotExist(err) {
		err = os.Remove(socketFilepath)
		if err != nil {
			er := fmt.Errorf("error: could not delete sock file: %v", err)
			return nil, er
		}
	}

	// Open the socket.
	nl, err := net.Listen("unix", socketFilepath)
	if err != nil {
		er := fmt.Errorf("error: failed to open socket: %v", err)
		return nil, er
	}

	return nl, nil
}

// Start will spawn up all the predefined subscriber processes.
// Spawning of publisher processes is done on the fly by checking
// if there is publisher process for a given message subject, and
// if it does not exist it will spawn one.
func (s *server) Start() {
	log.Printf("Starting ctrl, version=%+v\n", s.version)
	s.metrics.promVersion.With(prometheus.Labels{"version": string(s.version)})

	go func() {
		err := s.errorKernel.start(s.newMessagesCh)
		if err != nil {
			log.Printf("%v\n", err)
		}
	}()

	// Start collecting the metrics
	go func() {
		err := s.metrics.start()
		if err != nil {
			log.Printf("%v\n", err)
			os.Exit(1)
		}
	}()

	// Start the checking the input socket for new messages from operator.
	if s.configuration.EnableSocket {
		go s.readSocket()
	}

	// Start the checking the readfolder for new messages from operator.
	if s.configuration.EnableReadFolder {
		go s.readFolder()
	}

	// Check if we should start the tcp listener for new messages from operator.
	if s.configuration.TCPListener != "" {
		go s.readTCPListener()
	}

	// Check if we should start the http listener for new messages from operator.
	if s.configuration.HTTPListener != "" {
		go s.readHttpListener()
	}

	// Start audit logger.
	go s.startAuditLog(s.ctx)

	// Start up the predefined subscribers.
	//
	// Since all the logic to handle processes are tied to the process
	// struct, we need to create an initial process to start the rest.
	//
	// The context of the initial process are set in processes.Start.
	sub := newSubject(Initial, s.nodeName)
	s.processInitial = newProcess(context.TODO(), s, sub, streamInfo{}, "")
	// Start all wanted subscriber processes.
	s.processes.Start(s.processInitial)

	time.Sleep(time.Second * 1)
	s.processes.printProcessesMap()

	// Start exposing the the data folder via HTTP if flag is set.
	if s.configuration.ExposeDataFolder != "" {
		log.Printf("info: Starting expose of data folder via HTTP\n")
		go s.exposeDataFolder()
	}

	// Start the processing of new messages from an input channel.
	s.routeMessagesToProcess()

	// Start reading the channel for injecting direct messages that should
	// not be sent via the message broker.
	s.directSAMSChRead()

	// Check and enable read the messages specified in the startup folder.
	s.readStartupFolder()

}

// startAuditLog will start up the logging of all messages to audit file
func (s *server) startAuditLog(ctx context.Context) {

	storeFile := filepath.Join(s.configuration.DatabaseFolder, "store.log")
	f, err := os.OpenFile(storeFile, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		log.Printf("error: startPermanentStore: failed to open file: %v\n", err)
	}
	defer f.Close()

	for {
		select {
		case sams := <-s.auditLogCh:

			for _, sam := range sams {
				msgForPermStore := Message{}
				copier.Copy(&msgForPermStore, sam.Message)
				// Remove the content of the data field.
				msgForPermStore.Data = nil

				js, err := json.Marshal(msgForPermStore)
				if err != nil {
					er := fmt.Errorf("error:fillBuffer: json marshaling: %v", err)
					s.errorKernel.errSend(s.processInitial, Message{}, er, logError)
				}
				d := time.Now().Format("Mon Jan _2 15:04:05 2006") + ", " + string(js) + "\n"

				_, err = f.WriteString(d)
				if err != nil {
					log.Printf("error:failed to write entry: %v\n", err)
				}
			}
		case <-ctx.Done():
			return
		}
	}

}

// directSAMSChRead for injecting messages directly in to the local system
// without sending them via the message broker.
func (s *server) directSAMSChRead() {
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				log.Printf("info: stopped the directSAMSCh reader\n\n")
				return
			case sams := <-s.samSendLocalCh:
				// fmt.Printf(" * DEBUG: directSAMSChRead: <- sams = %v\n", sams)
				// Range over all the sams, find the process, check if the method exists, and
				// handle the message by starting the correct method handler.
				for i := range sams {
					processName := processNameGet(sams[i].Subject.name(), processKindSubscriberNats)

					s.processes.active.mu.Lock()
					p := s.processes.active.procNames[processName]
					s.processes.active.mu.Unlock()

					mh, ok := p.methodsAvailable.CheckIfExists(sams[i].Message.Method)
					if !ok {
						er := fmt.Errorf("error: subscriberHandler: method type not available: %v", p.subject.Method)
						p.errorKernel.errSend(p, sams[i].Message, er, logError)
						continue
					}

					p.handler = mh

					go executeHandler(p, sams[i].Message, s.nodeName)
				}
			}
		}
	}()
}

// Will stop all processes started during startup.
func (s *server) Stop() {
	// Stop the started pub/sub message processes.
	s.processes.Stop()
	log.Printf("info: stopped all subscribers\n")

	// Stop the errorKernel.
	s.errorKernel.stop()
	log.Printf("info: stopped the errorKernel\n")

	// Stop the main context.
	s.cancel()
	log.Printf("info: stopped the main context\n")

	// Delete the ctrl socket file when the program exits.
	socketFilepath := filepath.Join(s.configuration.SocketFolder, "ctrl.sock")

	if _, err := os.Stat(socketFilepath); !os.IsNotExist(err) {
		err = os.Remove(socketFilepath)
		if err != nil {
			er := fmt.Errorf("error: could not delete sock file: %v", err)
			log.Printf("%v\n", er)
		}
	}

}

// routeMessagesToProcess takes a database name it's input argument.
// The database will be used as the persistent k/v store for the work
// queue which is implemented as a ring buffer.
// The ringBufferInCh are where we get new messages to publish.
// Incomming messages will be routed to the correct subject process, where
// the handling of each nats subject is handled within it's own separate
// worker process.
// It will also handle the process of spawning more worker processes
// for publisher subjects if it does not exist.
func (s *server) routeMessagesToProcess() {
	// Start reading new fresh messages received on the incomming message
	// pipe/file.

	// Process the messages that are in the ring buffer. Check and
	// send if there are a specific subject for it, and if no subject
	// exist throw an error.

	var method Method
	methodsAvailable := method.GetMethodsAvailable()

	go func() {
		for samSlice := range s.newMessagesCh {
			for _, sam := range samSlice {

				go func(sam subjectAndMessage) {
					s.messageID.mu.Lock()
					s.messageID.id++
					sam.Message.ID = s.messageID.id
					s.messageID.mu.Unlock()

					s.metrics.promMessagesProcessedIDLast.Set(float64(sam.Message.ID))

					// Check if the format of the message is correct.
					if _, ok := methodsAvailable.CheckIfExists(sam.Message.Method); !ok {
						er := fmt.Errorf("error: routeMessagesToProcess: the method do not exist, message dropped: %v", sam.Message.Method)
						s.errorKernel.errSend(s.processInitial, sam.Message, er, logError)
						return
					}

					switch {
					case sam.Message.Retries < 0:
						sam.Message.Retries = s.configuration.DefaultMessageRetries
					}
					if sam.Message.MethodTimeout < 1 && sam.Message.MethodTimeout != -1 {
						sam.Message.MethodTimeout = s.configuration.DefaultMethodTimeout
					}

					// ---

					m := sam.Message

					subjName := sam.Subject.name()
					pn := processNameGet(subjName, processKindPublisherNats)

					sendOK := func() bool {
						var ctxCanceled bool

						s.processes.active.mu.Lock()
						defer s.processes.active.mu.Unlock()

						// Check if the process exist, if it do not exist return false so a
						// new publisher process will be created.
						proc, ok := s.processes.active.procNames[pn]
						if !ok {
							return false
						}

						if proc.ctx.Err() != nil {
							ctxCanceled = true
						}
						if ok && ctxCanceled {
							er := fmt.Errorf(" ** routeMessagesToProcess: context is already ended for process %v, will not try to reuse existing publisher, deleting it, and creating a new publisher !!! ", proc.processName)
							s.errorKernel.logDebug(er)
							delete(proc.processes.active.procNames, proc.processName)
							return false
						}

						// If found in map above, and go routine for publishing is running,
						// put the message on that processes incomming message channel.
						if ok && !ctxCanceled {
							select {
							case proc.subject.messageCh <- m:
								er := fmt.Errorf(" ** routeMessagesToProcess: passed message: %v to existing process: %v", m.ID, proc.processName)
								s.errorKernel.logDebug(er)
							case <-proc.ctx.Done():
								er := fmt.Errorf(" ** routeMessagesToProcess: got ctx.done for process %v", proc.processName)
								s.errorKernel.logDebug(er)
							}

							return true
						}

						// The process was not found, so we return false here so a new publisher
						// process will be created later.
						return false
					}()

					if sendOK {
						return
					}

					er := fmt.Errorf("info: processNewMessages: did not find publisher process for subject %v, starting new", subjName)
					s.errorKernel.logDebug(er)

					sub := newSubject(sam.Subject.Method, sam.Subject.ToNode)
					var proc process
					switch {
					case m.IsSubPublishedMsg:
						proc = newSubProcess(s.ctx, s, sub, processKindPublisherNats)
					default:
						proc = newProcess(s.ctx, s, sub, streamInfo{}, processKindPublisherNats)
					}

					proc.Start()
					er = fmt.Errorf("info: processNewMessages: new process started, subject: %v, processID: %v", subjName, proc.processID)
					s.errorKernel.logDebug(er)

					// Now when the process is spawned we continue,
					// and send the message to that new process.
					select {
					case proc.subject.messageCh <- m:
						er := fmt.Errorf(" ** routeMessagesToProcess: passed message: %v to the new process: %v", m.ID, proc.processName)
						s.errorKernel.logDebug(er)
					case <-proc.ctx.Done():
						er := fmt.Errorf(" ** routeMessagesToProcess: got ctx.done for process %v", proc.processName)
						s.errorKernel.logDebug(er)
					}

				}(sam)
			}
		}
	}()
}

func (s *server) exposeDataFolder() {
	fileHandler := func(w http.ResponseWriter, r *http.Request) {
		// w.Header().Set("Content-Type", "text/html")
		http.FileServer(http.Dir(s.configuration.SubscribersDataFolder)).ServeHTTP(w, r)
	}

	//create a file server, and serve the files found in ./
	//fd := http.FileServer(http.Dir(s.configuration.SubscribersDataFolder))
	http.HandleFunc("/", fileHandler)

	// we create a net.Listen type to use later with the http.Serve function.
	nl, err := net.Listen("tcp", s.configuration.ExposeDataFolder)
	if err != nil {
		log.Println("error: starting net.Listen: ", err)
	}

	// start the web server with http.Serve instead of the usual http.ListenAndServe
	err = http.Serve(nl, nil)
	if err != nil {
		log.Printf("Error: failed to start web server: %v\n", err)
	}
	os.Exit(1)

}
