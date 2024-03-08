package ctrl

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

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

	proc.startup.subscriber(proc, REQOpProcessList, nil)
	proc.startup.subscriber(proc, REQOpProcessStart, nil)
	proc.startup.subscriber(proc, REQOpProcessStop, nil)
	proc.startup.subscriber(proc, REQTest, nil)

	if proc.configuration.StartSubREQToFileAppend {
		proc.startup.subscriber(proc, REQToFileAppend, nil)
	}

	if proc.configuration.StartSubREQToFile {
		proc.startup.subscriber(proc, REQToFile, nil)
	}

	if proc.configuration.StartSubREQToFileNACK {
		proc.startup.subscriber(proc, REQToFileNACK, nil)
	}

	if proc.configuration.StartSubREQCopySrc {
		proc.startup.subscriber(proc, REQCopySrc, nil)
	}

	if proc.configuration.StartSubREQCopyDst {
		proc.startup.subscriber(proc, REQCopyDst, nil)
	}

	if proc.configuration.StartSubREQHello {
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
					p.errorKernel.logDebug(er, p.configuration)
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
		proc.startup.subscriber(proc, REQHello, pf)
	}

	if proc.configuration.IsCentralErrorLogger {
		proc.startup.subscriber(proc, REQErrorLog, nil)
	}

	if proc.configuration.StartSubREQCliCommand {
		proc.startup.subscriber(proc, REQCliCommand, nil)
	}

	if proc.configuration.StartSubREQToConsole {
		proc.startup.subscriber(proc, REQToConsole, nil)
	}

	if proc.configuration.StartPubREQHello != 0 {
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.StartPubREQHello))
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
					Method:     REQHello,
					ACKTimeout: proc.configuration.DefaultMessageTimeout,
					Retries:    1,
				}

				sam, err := newSubjectAndMessage(m)
				if err != nil {
					// In theory the system should drop the message before it reaches here.
					er := fmt.Errorf("error: ProcessesStart: %v", err)
					p.errorKernel.errSend(proc, m, er, logError)
				}
				proc.toRingbufferCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er, p.configuration)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, REQHello, pf)
	}

	if proc.configuration.EnableKeyUpdates {
		// pubREQKeysRequestUpdate defines the startup of a publisher that will send REQREQKeysRequestUpdate
		// to central server and ask for publics keys, and to get them deliver back with a request
		// of type pubREQKeysDeliverUpdate.
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.REQKeysRequestUpdateInterval))
			defer ticker.Stop()
			for {

				// Send a message with the hash of the currently stored keys,
				// so we would know on the subscriber at central if it should send
				// and update with new keys back.

				proc.nodeAuth.publicKeys.mu.Lock()
				er := fmt.Errorf(" ----> publisher REQKeysRequestUpdate: sending our current hash: %v", []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]))
				p.errorKernel.logDebug(er, p.configuration)

				m := Message{
					FileName:    "publickeysget.log",
					Directory:   "publickeysget",
					ToNode:      Node(p.configuration.CentralNodeName),
					FromNode:    Node(proc.node),
					Data:        []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]),
					Method:      REQKeysRequestUpdate,
					ReplyMethod: REQKeysDeliverUpdate,
					ACKTimeout:  proc.configuration.DefaultMessageTimeout,
					Retries:     1,
				}
				proc.nodeAuth.publicKeys.mu.Unlock()

				sam, err := newSubjectAndMessage(m)
				if err != nil {
					// In theory the system should drop the message before it reaches here.
					p.errorKernel.errSend(proc, m, err, logError)
				}
				proc.toRingbufferCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er, p.configuration)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, REQKeysRequestUpdate, pf)
		proc.startup.subscriber(proc, REQKeysDeliverUpdate, nil)
	}

	if proc.configuration.EnableAclUpdates {
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.REQAclRequestUpdateInterval))
			defer ticker.Stop()
			for {

				// Send a message with the hash of the currently stored acl's,
				// so we would know for the subscriber at central if it should send
				// and update with new keys back.

				proc.nodeAuth.nodeAcl.mu.Lock()
				er := fmt.Errorf(" ----> publisher REQAclRequestUpdate: sending our current hash: %v", []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]))
				p.errorKernel.logDebug(er, p.configuration)

				m := Message{
					FileName:    "aclRequestUpdate.log",
					Directory:   "aclRequestUpdate",
					ToNode:      Node(p.configuration.CentralNodeName),
					FromNode:    Node(proc.node),
					Data:        []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]),
					Method:      REQAclRequestUpdate,
					ReplyMethod: REQAclDeliverUpdate,
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
				proc.toRingbufferCh <- []subjectAndMessage{sam}

				select {
				case <-ticker.C:
				case <-ctx.Done():
					er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
					// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
					p.errorKernel.logDebug(er, p.configuration)
					return nil
				}
			}
		}
		proc.startup.publisher(proc, REQAclRequestUpdate, pf)
		proc.startup.subscriber(proc, REQAclDeliverUpdate, nil)
	}

	if proc.configuration.IsCentralAuth {
		proc.startup.subscriber(proc, REQKeysRequestUpdate, nil)
		proc.startup.subscriber(proc, REQKeysAllow, nil)
		proc.startup.subscriber(proc, REQKeysDelete, nil)
		proc.startup.subscriber(proc, REQAclRequestUpdate, nil)
		proc.startup.subscriber(proc, REQAclAddCommand, nil)
		proc.startup.subscriber(proc, REQAclDeleteCommand, nil)
		proc.startup.subscriber(proc, REQAclDeleteSource, nil)
		proc.startup.subscriber(proc, REQAclGroupNodesAddNode, nil)
		proc.startup.subscriber(proc, REQAclGroupNodesDeleteNode, nil)
		proc.startup.subscriber(proc, REQAclGroupNodesDeleteGroup, nil)
		proc.startup.subscriber(proc, REQAclGroupCommandsAddCommand, nil)
		proc.startup.subscriber(proc, REQAclGroupCommandsDeleteCommand, nil)
		proc.startup.subscriber(proc, REQAclGroupCommandsDeleteGroup, nil)
		proc.startup.subscriber(proc, REQAclExport, nil)
		proc.startup.subscriber(proc, REQAclImport, nil)
	}

	if proc.configuration.StartSubREQHttpGet {
		proc.startup.subscriber(proc, REQHttpGet, nil)
	}

	if proc.configuration.StartSubREQHttpGetScheduled {
		proc.startup.subscriber(proc, REQHttpGetScheduled, nil)
	}

	if proc.configuration.StartSubREQTailFile {
		proc.startup.subscriber(proc, REQTailFile, nil)
	}

	if proc.configuration.StartSubREQCliCommandCont {
		proc.startup.subscriber(proc, REQCliCommandCont, nil)
	}

	proc.startup.subscriber(proc, REQPublicKey, nil)
}

// Stop all subscriber processes.
func (p *processes) Stop() {
	log.Printf("info: canceling all subscriber processes...\n")
	p.cancel()
	p.wg.Wait()
	log.Printf("info: done canceling all subscriber processes.\n")

}

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
// and a procFunc as it's input arguments. If a procFunc os not needed, use the value nil.
func (s *startup) subscriber(p process, m Method, pf func(ctx context.Context, procFuncCh chan Message) error) {
	er := fmt.Errorf("starting %v subscriber: %#v", m, p.node)
	p.errorKernel.logDebug(er, p.configuration)

	var sub Subject
	switch {
	case m == REQErrorLog:
		sub = newSubject(m, "errorCentral")
	default:
		sub = newSubject(m, string(p.node))
	}

	fmt.Printf("DEBUG:::startup subscriber, subject: %v\n", sub)
	proc := newProcess(p.ctx, p.processes.server, sub, processKindSubscriber)
	proc.procFunc = pf

	go proc.spawnWorker()
}

func (s *startup) publisher(p process, m Method, pf func(ctx context.Context, procFuncCh chan Message) error) {
	er := fmt.Errorf("starting %v publisher: %#v", m, p.node)
	p.errorKernel.logDebug(er, p.configuration)
	sub := newSubject(m, string(p.node))
	proc := newProcess(p.ctx, p.processes.server, sub, processKindPublisher)
	proc.procFunc = pf
	proc.isLongRunningPublisher = true

	go proc.spawnWorker()
}

// ---------------------------------------------------------------

// Print the content of the processes map.
func (p *processes) printProcessesMap() {
	er := fmt.Errorf("output of processes map : ")
	p.errorKernel.logDebug(er, p.configuration)

	{
		p.active.mu.Lock()

		for pName, proc := range p.active.procNames {
			er := fmt.Errorf("info: proc - pub/sub: %v, procName in map: %v , id: %v, subject: %v", proc.processKind, pName, proc.processID, proc.subject.name())
			proc.errorKernel.logDebug(er, proc.configuration)
		}

		p.metrics.promProcessesTotal.Set(float64(len(p.active.procNames)))

		p.active.mu.Unlock()
	}

}
