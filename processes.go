package ctrl

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
)

// processes holds all the information about running processes
type processes struct {
	// mutex for processes
	mu sync.Mutex
	// The main context for subscriber processes.
	ctx context.Context
	// cancel func to send cancel signal to the subscriber processes context.
	cancel context.CancelFunc
	// The active spawned processes
	// server
	server *server
	active procsMap
	// mutex to lock the map
	// mu sync.RWMutex
	// The last processID created
	lastProcessID int
	// The instance global prometheus registry.
	metrics *metrics
	// Waitgroup to keep track of all the processes started.
	wg sync.WaitGroup
	// errorKernel
	errorKernel *errorKernel
	// configuration
	configuration *Configuration

	// Signatures
	nodeAuth *nodeAuth
}

// newProcesses will prepare and return a *processes which
// is map containing all the currently running processes.
func newProcesses(ctx context.Context, server *server) *processes {
	p := processes{
		server:        server,
		active:        *newProcsMap(),
		errorKernel:   server.errorKernel,
		configuration: server.configuration,
		nodeAuth:      server.nodeAuth,
		metrics:       server.metrics,
	}

	// Prepare the parent context for the subscribers.
	ctx, cancel := context.WithCancel(ctx)

	// // Start the processes map.
	// go func() {
	// 	p.active.run(ctx)
	// }()

	p.ctx = ctx
	p.cancel = cancel

	return &p
}

// ----------------------

// ----------------------

type procsMap struct {
	procNames map[processName]process
	mu        sync.Mutex
}

func newProcsMap() *procsMap {
	cM := procsMap{
		procNames: make(map[processName]process),
	}
	return &cM
}

// ----------------------

// Start all the subscriber processes.
// Takes an initial process as it's input. All processes
// will be tied to this single process's context.
func (p *processes) Start(proc process) {
	// Set the context for the initial process.
	proc.ctx = p.ctx

	// --- Subscriber services that can be started via flags

	proc.startup.subscriber(proc, OpProcessList, nil)
	proc.startup.subscriber(proc, OpProcessStart, nil)
	proc.startup.subscriber(proc, OpProcessStop, nil)
	proc.startup.subscriber(proc, Test, nil)

	if proc.configuration.StartSubFileAppend {
		proc.startup.subscriber(proc, FileAppend, nil)
	}

	if proc.configuration.StartSubFile {
		proc.startup.subscriber(proc, File, nil)
	}

	if proc.configuration.StartSubCopySrc {
		proc.startup.subscriber(proc, CopySrc, nil)
	}

	if proc.configuration.StartSubCopyDst {
		proc.startup.subscriber(proc, CopyDst, nil)
	}

	if proc.configuration.StartSubHello {
		// subREQHello is the handler that is triggered when we are receiving a hello
		// message. To keep the state of all the hello's received from nodes we need
		// to also start a procFunc that will live as a go routine tied to this process,
		// where the procFunc will receive messages from the handler when a message is
		// received, the handler will deliver the message to the procFunc on the
		// proc.procFuncCh, and we can then read that message from the procFuncCh in
		// the procFunc running.
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			// sayHelloNodes := make(map[Node]struct{})

			for {
				// Receive a copy of the message sent from the method handler.
				var m Message

				select {
				case m = <-procFuncCh:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: subscriber %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er)
					return nil
				}

				proc.centralAuth.addPublicKey(proc, m)

				// update the prometheus metrics

				proc.server.centralAuth.pki.nodesAcked.mu.Lock()
				mapLen := len(proc.server.centralAuth.pki.nodesAcked.keysAndHash.Keys)
				proc.server.centralAuth.pki.nodesAcked.mu.Unlock()
				proc.metrics.promHelloNodesTotal.Set(float64(mapLen))
				proc.metrics.promHelloNodesContactLast.With(prometheus.Labels{"nodeName": string(m.FromNode)}).SetToCurrentTime()

			}
		}
		proc.startup.subscriber(proc, Hello, pf)
	}

	fmt.Printf("--------------------------------IsCentralErrorLogger = %v------------------------------\n", proc.configuration.IsCentralErrorLogger)
	if proc.configuration.IsCentralErrorLogger {
		proc.startup.subscriber(proc, ErrorLog, nil)
	}

	if proc.configuration.StartSubCliCommand {
		proc.startup.subscriber(proc, CliCommand, nil)
	}

	if proc.configuration.StartSubConsole {
		proc.startup.subscriber(proc, Console, nil)
	}

	if proc.configuration.StartPubHello != 0 {
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.StartPubHello))
			defer ticker.Stop()
			for {

				// d := fmt.Sprintf("Hello from %v\n", p.node)
				// Send the ed25519 public key used for signing as the payload of the message.
				d := proc.server.nodeAuth.SignPublicKey

				m := Message{
					FileName:   "hello.log",
					Directory:  "hello-messages",
					ToNode:     Node(p.configuration.CentralNodeName),
					FromNode:   Node(proc.node),
					Data:       []byte(d),
					Method:     Hello,
					ACKTimeout: proc.configuration.DefaultMessageTimeout,
					Retries:    1,
				}

				sam, err := newSubjectAndMessage(m)
				if err != nil {
					// In theory the system should drop the message before it reaches here.
					er := fmt.Errorf("error: ProcessesStart: %v", err)
					p.errorKernel.errSend(proc, m, er, logError)
				}
				proc.newMessagesCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, Hello, pf)
	}

	if proc.configuration.EnableKeyUpdates {
		// Define the startup of a publisher that will send KeysRequestUpdate
		// to central server and ask for publics keys, and to get them deliver back with a request
		// of type KeysDeliverUpdate.
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.KeysUpdateInterval))
			defer ticker.Stop()
			for {

				// Send a message with the hash of the currently stored keys,
				// so we would know on the subscriber at central if it should send
				// and update with new keys back.

				proc.nodeAuth.publicKeys.mu.Lock()
				er := fmt.Errorf(" ----> publisher KeysRequestUpdate: sending our current hash: %v", []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]))
				p.errorKernel.logDebug(er)

				m := Message{
					FileName:    "publickeysget.log",
					Directory:   "publickeysget",
					ToNode:      Node(p.configuration.CentralNodeName),
					FromNode:    Node(proc.node),
					Data:        []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]),
					Method:      KeysRequestUpdate,
					ReplyMethod: KeysDeliverUpdate,
					ACKTimeout:  proc.configuration.DefaultMessageTimeout,
					Retries:     1,
				}
				proc.nodeAuth.publicKeys.mu.Unlock()

				sam, err := newSubjectAndMessage(m)
				if err != nil {
					// In theory the system should drop the message before it reaches here.
					p.errorKernel.errSend(proc, m, err, logError)
				}
				proc.newMessagesCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, KeysRequestUpdate, pf)
		proc.startup.subscriber(proc, KeysDeliverUpdate, nil)
	}

	if proc.configuration.EnableAclUpdates {
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.AclUpdateInterval))
			defer ticker.Stop()
			for {

				// Send a message with the hash of the currently stored acl's,
				// so we would know for the subscriber at central if it should send
				// and update with new keys back.

				proc.nodeAuth.nodeAcl.mu.Lock()
				er := fmt.Errorf(" ----> publisher AclRequestUpdate: sending our current hash: %v", []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]))
				p.errorKernel.logDebug(er)

				m := Message{
					FileName:    "aclRequestUpdate.log",
					Directory:   "aclRequestUpdate",
					ToNode:      Node(p.configuration.CentralNodeName),
					FromNode:    Node(proc.node),
					Data:        []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]),
					Method:      AclRequestUpdate,
					ReplyMethod: AclDeliverUpdate,
					ACKTimeout:  proc.configuration.DefaultMessageTimeout,
					Retries:     1,
				}
				proc.nodeAuth.nodeAcl.mu.Unlock()

				sam, err := newSubjectAndMessage(m)
				if err != nil {
					// In theory the system should drop the message before it reaches here.
					p.errorKernel.errSend(proc, m, err, logError)
					log.Printf("error: ProcessesStart: %v\n", err)
				}
				proc.newMessagesCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, AclRequestUpdate, pf)
		proc.startup.subscriber(proc, AclDeliverUpdate, nil)
	}

	if proc.configuration.IsCentralAuth {
		proc.startup.subscriber(proc, KeysRequestUpdate, nil)
		proc.startup.subscriber(proc, KeysAllow, nil)
		proc.startup.subscriber(proc, KeysDelete, nil)
		proc.startup.subscriber(proc, AclRequestUpdate, nil)
		proc.startup.subscriber(proc, AclAddCommand, nil)
		proc.startup.subscriber(proc, AclDeleteCommand, nil)
		proc.startup.subscriber(proc, AclDeleteSource, nil)
		proc.startup.subscriber(proc, AclGroupNodesAddNode, nil)
		proc.startup.subscriber(proc, AclGroupNodesDeleteNode, nil)
		proc.startup.subscriber(proc, AclGroupNodesDeleteGroup, nil)
		proc.startup.subscriber(proc, AclGroupCommandsAddCommand, nil)
		proc.startup.subscriber(proc, AclGroupCommandsDeleteCommand, nil)
		proc.startup.subscriber(proc, AclGroupCommandsDeleteGroup, nil)
		proc.startup.subscriber(proc, AclExport, nil)
		proc.startup.subscriber(proc, AclImport, nil)
	}

	if proc.configuration.StartSubHttpGet {
		proc.startup.subscriber(proc, HttpGet, nil)
	}

	if proc.configuration.StartSubTailFile {
		proc.startup.subscriber(proc, TailFile, nil)
	}

	if proc.configuration.StartSubCliCommandCont {
		proc.startup.subscriber(proc, CliCommandCont, nil)
	}

	proc.startup.subscriber(proc, PublicKey, nil)

	// --------------------------------------------------
	// ProcFunc for Jetstream publishers.
	// --------------------------------------------------
	if proc.configuration.StartJetstreamPublisher {
		pfJetstreamPublishers := func(ctx context.Context, procFuncCh chan Message) error {
			js, err := jetstream.New(proc.natsConn)
			if err != nil {
				log.Fatalf("error: jetstream new failed: %v\n", err)
			}

			_, err = js.CreateOrUpdateStream(proc.ctx, jetstream.StreamConfig{
				Name:        "nodes",
				Description: "nodes stream",
				Subjects:    []string{"nodes.>"},
				// Discard older messages and keep only the last one.
				MaxMsgsPerSubject: 1,
			})

			if err != nil {
				log.Fatalf("error: jetstream create or update failed: %v\n", err)
			}

			for {
				// TODO:
				select {
				case msg := <-proc.jetstreamOut:
					b, err := proc.messageSerializeAndCompress(msg)
					if err != nil {
						log.Fatalf("error: pfJetstreamPublishers: js failed to marshal message: %v\n", err)
					}

					subject := fmt.Sprintf("nodes.%v", msg.JetstreamToNode)
					_, err = js.Publish(proc.ctx, subject, b)
					if err != nil {
						log.Fatalf("error: pfJetstreamPublishers:js failed to publish message: %v\n", err)
					}
				case <-ctx.Done():
					return fmt.Errorf("%v", "info: pfJetstreamPublishers: got <-ctx.done")
				}
			}
		}
		proc.startup.publisher(proc, JetStreamPublishers, pfJetstreamPublishers)
	}

	// --------------------------------------------------
	// Procfunc for Jetstream consumers.
	// --------------------------------------------------

	// pfJetstreamConsumers connect to a given nats jetstream, and consume messages
	// for the node on specified subjects within that stream.
	// After a jetstream message is picked up from the stream, the steward message
	// will be extracted from the data field, and the ctrl message will be put
	// on the local delivery channel, and handled as a normal ctrl message.
	if proc.configuration.StartJetstreamConsumer {
		pfJetstreamConsumers := func(ctx context.Context, procFuncCh chan Message) error {
			js, err := jetstream.New(proc.natsConn)
			if err != nil {
				log.Fatalf("error: jetstream new failed: %v\n", err)
			}

			stream, err := js.Stream(proc.ctx, "nodes")
			if err != nil {
				log.Fatalf("error: js.Stream failed: %v\n", err)
			}

			// Check for more subjects via flags to listen to, and if defined prefix all
			// the values with "nodes."
			filterSubjectValues := []string{
				fmt.Sprintf("nodes.%v", proc.server.nodeName),
				"nodes.all",
			}

			// Check if there are more to consume defined in flags/env.
			if proc.configuration.JetstreamsConsume != "" {
				splitValues := strings.Split(proc.configuration.JetstreamsConsume, ",")
				for i, v := range splitValues {
					filterSubjectValues[i] = fmt.Sprintf("nodes.%v", v)
				}
			}

			consumer, err := stream.CreateOrUpdateConsumer(proc.ctx, jetstream.ConsumerConfig{
				Name:           "nodes_processor",
				Durable:        "nodes_processor",
				FilterSubjects: filterSubjectValues,
			})
			if err != nil {
				log.Fatalf("error: create or update consumer failed: %v\n", err)
			}

			cctx, err := consumer.Consume(func(msg jetstream.Msg) {
				stewardMessage := Message{}
				stewardMessage, err := proc.messageDeserializeAndUncompress(msg)
				if err != nil {
					log.Fatalf("error: pfJetstreamConsumers: json.Unmarshal failed: %v\n", err)
				}

				log.Printf("Received jetstream message to convert and handle as normal nats message: %v, with ctrl method: %v\n", string(msg.Subject()), string(stewardMessage.Method))

				msg.Ack()

				// Messages received here via jetstream are for this node. Put the message into
				// a SubjectAndMessage structure, and we use the deliver local from here.
				sam, err := newSubjectAndMessage(stewardMessage)
				if err != nil {
					log.Fatalf("error: pfJetstreamConsumers: newSubjectAndMessage failed: %v\n", err)
				}
				proc.server.samSendLocalCh <- []subjectAndMessage{sam}
			})
			if err != nil {
				log.Fatalf("error: create or update consumer failed: %v\n", err)
			}

			defer cctx.Stop()

			<-proc.ctx.Done()

			return nil
		}

		proc.startup.subscriber(proc, JetstreamConsumers, pfJetstreamConsumers)
	}

}

// --------------------------------------------------

// Stop all subscriber processes.
func (p *processes) Stop() {
	log.Printf("info: canceling all subscriber processes...\n")
	p.cancel()
	p.wg.Wait()
	log.Printf("info: done canceling all subscriber processes.\n")

}

// ---------------------------------------------------------------------------------------
// Helper functions, and other
// ---------------------------------------------------------------------------------------

// Startup holds all the startup methods for subscribers.
type startup struct {
	server      *server
	centralAuth *centralAuth
	metrics     *metrics
}

func newStartup(server *server) *startup {
	s := startup{
		server:      server,
		centralAuth: server.centralAuth,
		metrics:     server.metrics,
	}

	return &s
}

// subscriber will start a subscriber process. It takes the initial process, request method,
// and a procFunc as it's input arguments. If a procFunc is not needed, use the value nil.
func (s *startup) subscriber(p process, m Method, pf func(ctx context.Context, procFuncCh chan Message) error) {
	er := fmt.Errorf("starting %v subscriber: %#v", m, p.node)
	p.errorKernel.logDebug(er)

	var sub Subject
	switch {
	case m == ErrorLog:
		sub = newSubject(m, "errorCentral")
	default:
		sub = newSubject(m, string(p.node))
	}

	fmt.Printf("DEBUG:::startup subscriber, subject: %v\n", sub)
	proc := newProcess(p.ctx, p.processes.server, sub, streamInfo{}, processKindSubscriberNats)
	proc.procFunc = pf

	go proc.Start()
}

// publisher will start a publisher process. It takes the initial process, request method,
// and a procFunc as it's input arguments. If a procFunc is not needed, use the value nil.
func (s *startup) publisher(p process, m Method, pf func(ctx context.Context, procFuncCh chan Message) error) {
	er := fmt.Errorf("starting %v publisher: %#v", m, p.node)
	p.errorKernel.logDebug(er)
	sub := newSubject(m, string(p.node))
	proc := newProcess(p.ctx, p.processes.server, sub, streamInfo{}, processKindPublisherNats)
	proc.procFunc = pf
	proc.isLongRunningPublisher = true

	go proc.Start()
}

// ---------------------------------------------------------------

// Print the content of the processes map.
func (p *processes) printProcessesMap() {
	er := fmt.Errorf("output of processes map : ")
	p.errorKernel.logDebug(er)

	{
		p.active.mu.Lock()

		for pName, proc := range p.active.procNames {
			er := fmt.Errorf("info: proc - pub/sub: %v, procName in map: %v , id: %v, subject: %v", proc.processKind, pName, proc.processID, proc.subject.name())
			proc.errorKernel.logDebug(er)
		}

		p.metrics.promProcessesTotal.Set(float64(len(p.active.procNames)))

		p.active.mu.Unlock()
	}

}
