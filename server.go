// Notes:
package ctrl

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/jinzhu/copier"
	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
)

type processName string

// Will return a process name made up of subjectName+processKind
func processNameGet(sn subjectName) processName {
	return processName(sn)
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
	newMessagesCh chan Message
	// messageDeliverLocalCh
	messageDeliverLocalCh chan []Message
	// Channel for messages to publish with Jetstream.
	jetstreamPublishCh chan Message
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
	auditLogCh  chan []Message
	zstdEncoder *zstd.Encoder
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

		err = os.WriteFile(pth, []byte(configuration.NkeySeed), 0600)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to write temp seed file: %v", err)
		}

		opt, err = nats.NkeyOptionFromSeed(pth)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to read temp nkey seed file: %v", err)
		}

		// // TODO: REMOVED for testing
		//defer func() {
		//	err = os.Remove(pth)
		//	if err != nil {
		//		cancel()
		//		log.Fatalf("error: failed to remove temp seed file: %v\n", err)
		//	}
		//}()

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

	// Check if tmp folder for socket exists, if not create it
	if _, err := os.Stat(configuration.DatabaseFolder); os.IsNotExist(err) {
		err := os.MkdirAll(configuration.DatabaseFolder, 0770)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("error: failed to create database folder directory %v: %v", configuration.DatabaseFolder, err)
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

	zstdEncoder, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		log.Fatalf("error: zstd new encoder failed: %v", err)
	}

	defer func() {
		go func() {
			<-ctx.Done()
			zstdEncoder.Close()
		}()
	}()

	s := server{
		ctx:                   ctx,
		cancel:                cancel,
		configuration:         configuration,
		nodeName:              configuration.NodeName,
		natsConn:              conn,
		ctrlSocket:            ctrlSocket,
		newMessagesCh:         make(chan Message),
		messageDeliverLocalCh: make(chan []Message),
		jetstreamPublishCh:    make(chan Message),
		metrics:               metrics,
		version:               version,
		errorKernel:           errorKernel,
		nodeAuth:              nodeAuth,
		helloRegister:         newHelloRegister(),
		centralAuth:           centralAuth,
		auditLogCh:            make(chan []Message),
		zstdEncoder:           zstdEncoder,
	}

	s.processes = newProcesses(ctx, &s)

	// Create the default data folder for where subscribers should
	// write it's data, check if data folder exist, and create it if needed.
	if _, err := os.Stat(configuration.SubscribersDataFolder); os.IsNotExist(err) {
		if configuration.SubscribersDataFolder == "" {
			return nil, fmt.Errorf("error: subscribersDataFolder value is empty, you need to provide the config or the flag value at startup %v: %v", configuration.SubscribersDataFolder, err)
		}
		err := os.MkdirAll(configuration.SubscribersDataFolder, 0770)
		if err != nil {
			return nil, fmt.Errorf("error: failed to create data folder directory %v: %v", configuration.SubscribersDataFolder, err)
		}

		s.errorKernel.logDebug("NewServer: creating subscribers data folder at", "path", configuration.SubscribersDataFolder)
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
	s.processInitial = newProcess(context.TODO(), s, sub)
	// Start all wanted subscriber processes.
	s.processes.Start(s.processInitial)

	time.Sleep(time.Second * 1)
	s.processes.printProcessesMap()

	// Start Jetstream publisher and consumer.
	go s.jetstreamPublish()
	go s.jetstreamConsume()

	// Start exposing the the data folder via HTTP if flag is set.
	if s.configuration.ExposeDataFolder != "" {
		log.Printf("info: Starting expose of data folder via HTTP\n")
		go s.exposeDataFolder()
	}

	// Start the processing of new messages from an input channel.
	s.routeMessagesToPublisherProcess()

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
		case messages := <-s.auditLogCh:

			for _, message := range messages {
				msgForPermStore := Message{}
				copier.Copy(&msgForPermStore, message)
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
			case messages := <-s.messageDeliverLocalCh:
				// fmt.Printf(" * DEBUG: directSAMSChRead: <- sams = %v\n", sams)
				// Range over all the sams, find the process, check if the method exists, and
				// handle the message by starting the correct method handler.
				for i := range messages {
					// TODO: !!!!!! Shoud the node here be the fromNode ???????
					subject := newSubject(messages[i].Method, string(messages[i].ToNode))
					processName := processNameGet(subject.name())

					s.processes.active.mu.Lock()
					p := s.processes.active.procNames[processName]
					s.processes.active.mu.Unlock()

					mh, ok := p.methodsAvailable.CheckIfExists(messages[i].Method)
					if !ok {
						er := fmt.Errorf("error: subscriberHandler: method type not available: %v", p.subject.Method)
						p.errorKernel.errSend(p, messages[i], er, logError)
						continue
					}

					p.handler = mh

					go executeHandler(p, messages[i], s.nodeName)
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

// routeMessagesToPublisherProcess takes a database name it's input argument.
// The database will be used as the persistent k/v store for the work
// queue which is implemented as a ring buffer.
// The ringBufferInCh are where we get new messages to publish.
// Incomming messages will be routed to the correct subject process, where
// the handling of each nats subject is handled within it's own separate
// worker process.
// It will also handle the process of spawning more worker processes
// for publisher subjects if it does not exist.
func (s *server) routeMessagesToPublisherProcess() {
	// Start reading new fresh messages received on the incomming message
	// pipe/file.

	// Process the messages that are in the ring buffer. Check and
	// send if there are a specific subject for it, and if no subject
	// exist throw an error.

	var method Method
	methodsAvailable := method.GetMethodsAvailable()

	go func() {
		for message := range s.newMessagesCh {

			go func(message Message) {

				s.messageID.mu.Lock()
				s.messageID.id++
				message.ID = s.messageID.id
				s.messageID.mu.Unlock()

				s.metrics.promMessagesProcessedIDLast.Set(float64(message.ID))

				// Check if the format of the message is correct.
				if _, ok := methodsAvailable.CheckIfExists(message.Method); !ok {
					er := fmt.Errorf("error: routeMessagesToProcess: the method do not exist, message dropped: %v", message.Method)
					s.errorKernel.errSend(s.processInitial, message, er, logError)
					return
				}

				switch {
				case message.Retries < 0:
					message.Retries = s.configuration.DefaultMessageRetries
				}
				if message.MethodTimeout < 1 && message.MethodTimeout != -1 {
					message.MethodTimeout = s.configuration.DefaultMethodTimeout
				}

				// ---
				// Check for {{CTRL_FILE}} and if we should read and load a local file into
				// the message before sending.

				var filePathToOpen string
				foundFile := false
				var argPos int
				for i, v := range message.MethodArgs {
					if strings.Contains(v, "{{CTRL_FILE:") {
						foundFile = true
						argPos = i

						// Example to split:
						// echo {{CTRL_FILE:/somedir/msg_file.yaml}}>ctrlfile.txt
						//
						// Split at colon. We want the part after.
						ss := strings.Split(v, ":")
						// Split at "}}",so pos [0] in the result contains just the file path.
						sss := strings.Split(ss[1], "}}")
						filePathToOpen = sss[0]

					}
				}

				if foundFile {

					fh, err := os.Open(filePathToOpen)
					if err != nil {
						s.errorKernel.logError("routeMessagesToPublisherProcess: failed to open file given as CTRL_FILE argument", "error", err)
						return
					}
					defer fh.Close()

					b, err := io.ReadAll(fh)
					if err != nil {
						s.errorKernel.logError("routeMessagesToPublisherProcess: failed to read file given as CTRL_FILE argument", "file", filePathToOpen, "error", err)
						return
					}

					// Replace the {{CTRL_FILE}} with the actual content read from file.
					re := regexp.MustCompile(`(.*)({{CTRL_FILE.*}})(.*)`)
					message.MethodArgs[argPos] = re.ReplaceAllString(message.MethodArgs[argPos], `${1}`+string(b)+`${3}`)
					// ---

				}

				message.ArgSignature = s.processInitial.addMethodArgSignature(message)

				go s.processInitial.publishAMessage(message, s.natsConn)

			}(message)

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

// messageSerializeAndCompress will serialize and compress the Message, and
// return the result as a []byte.
func (s *server) messageSerializeAndCompress(msg Message) ([]byte, error) {
	// NB: Implementing json encoding for WebUI messages for now.
	if msg.Method == WebUI {
		bSerialized, err := json.Marshal(msg)
		if err != nil {
			er := fmt.Errorf("error: messageDeliverNats: json encode message failed: %v", err)
			return nil, er
		}
		fmt.Printf("JSON JSON JSON JSON JSON JSON JSON JSON JSON JSON JSON JSON \n")
		return bSerialized, nil
	}

	// encode the message structure into cbor
	bSerialized, err := cbor.Marshal(msg)
	if err != nil {
		er := fmt.Errorf("error: messageDeliverNats: cbor encode message failed: %v", err)
		return nil, er
	}

	// Compress the data payload if selected with configuration flag.
	// The compression chosen is later set in the nats msg header when
	// calling p.messageDeliverNats below.

	bCompressed := s.zstdEncoder.EncodeAll(bSerialized, nil)

	return bCompressed, nil
}

// messageDeserializeAndUncompress will deserialize the ctrl message
func (s *server) messageDeserializeAndUncompress(msgData []byte) (Message, error) {

	// // If debugging is enabled, print the source node name of the nats messages received.
	// headerFromNode := msg.Headers().Get("fromNode")
	// if headerFromNode != "" {
	// 	er := fmt.Errorf("info: subscriberHandlerJetstream: nats message received from %v, with subject %v ", headerFromNode, msg.Subject())
	// 	s.errorKernel.logDebug(er)
	// }
	msgData2 := msgData

	zr, err := zstd.NewReader(nil)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: zstd NewReader failed: %v", err)
		return Message{}, er
	}
	msgData, err = zr.DecodeAll(msgData, nil)
	if err != nil {
		// er := fmt.Errorf("error: subscriberHandlerJetstream: zstd decoding failed: %v", err)
		zr.Close()

		// Not zstd encoded, try to decode as JSON. This is for messages from the WebUI.
		var msg Message
		fmt.Printf("DEBUG: msgData2: %v\n", string(msgData2))
		err = json.Unmarshal(msgData2, &msg)
		if err != nil {
			return Message{}, err
		}

		// JSON decoded, return the message
		return msg, nil
	}

	zr.Close()

	message := Message{}

	err = cbor.Unmarshal(msgData, &message)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: cbor decoding failed, error: %v", err)
		return Message{}, er
	}

	return message, nil
}
