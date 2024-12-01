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

	proc.startup.subscriber(proc, OpProcessList, nil)
	proc.startup.subscriber(proc, OpProcessStart, nil)
	proc.startup.subscriber(proc, OpProcessStop, nil)
	proc.startup.subscriber(proc, Test, nil)

	if proc.configuration.StartProcesses.StartSubFileAppend {
		proc.startup.subscriber(proc, FileAppend, nil)
	}

	if proc.configuration.StartProcesses.StartSubFile {
		proc.startup.subscriber(proc, File, nil)
	}

	if proc.configuration.StartProcesses.StartSubCopySrc {
		proc.startup.subscriber(proc, CopySrc, nil)
	}

	if proc.configuration.StartProcesses.StartSubCopyDst {
		proc.startup.subscriber(proc, CopyDst, nil)
	}

	if proc.configuration.StartProcesses.StartSubHello {
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

	if proc.configuration.StartProcesses.IsCentralErrorLogger {
		proc.startup.subscriber(proc, ErrorLog, nil)
	}

	if proc.configuration.StartProcesses.StartSubCliCommand {
		proc.startup.subscriber(proc, CliCommand, nil)
	}

	if proc.configuration.StartProcesses.StartSubConsole {
		proc.startup.subscriber(proc, Console, nil)
	}

	if proc.configuration.StartProcesses.StartPubHello != 0 {
		pf := func(ctx context.Context, procFuncCh chan Message) error {
			ticker := time.NewTicker(time.Second * time.Duration(p.configuration.StartProcesses.StartPubHello))
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

				proc.newMessagesCh <- m

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

	if proc.configuration.StartProcesses.EnableKeyUpdates {
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

				proc.newMessagesCh <- m

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

	if proc.configuration.StartProcesses.EnableAclUpdates {
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

				proc.newMessagesCh <- m

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

	if proc.configuration.StartProcesses.IsCentralAuth {
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

	if proc.configuration.StartProcesses.StartSubHttpGet {
		proc.startup.subscriber(proc, HttpGet, nil)
	}

	if proc.configuration.StartProcesses.StartSubTailFile {
		proc.startup.subscriber(proc, TailFile, nil)
	}

	if proc.configuration.StartProcesses.StartSubCliCommandCont {
		proc.startup.subscriber(proc, CliCommandCont, nil)
	}

	proc.startup.subscriber(proc, PublicKey, nil)
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
	proc := newProcess(p.ctx, p.processes.server, sub, processKindSubscriber)
	proc.procFunc = pf

	go proc.start()
}

// publisher will start a publisher process. It takes the initial process, request method,
// and a procFunc as it's input arguments. If a procFunc is not needed, use the value nil.
func (s *startup) publisher(p process, m Method, pf func(ctx context.Context, procFuncCh chan Message) error) {
	er := fmt.Errorf("starting %v publisher: %#v", m, p.node)
	p.errorKernel.logDebug(er)
	sub := newSubject(m, string(p.node))
	proc := newProcess(p.ctx, p.processes.server, sub, processKindPublisher)
	proc.procFunc = pf
	proc.isLongRunningPublisher = true

	go proc.start()
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
