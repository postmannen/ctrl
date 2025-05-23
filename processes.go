package ctrl

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
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

	proc.startup.startProcess(proc, OpProcessList, nil)
	proc.startup.startProcess(proc, OpProcessStart, nil)
	proc.startup.startProcess(proc, OpProcessStop, nil)
	proc.startup.startProcess(proc, Test, nil)

	if proc.configuration.StartProcesses.StartSubFileAppend {
		proc.startup.startProcess(proc, FileAppend, nil)
	}

	if proc.configuration.StartProcesses.StartSubFile {
		proc.startup.startProcess(proc, File, nil)
	}

	if proc.configuration.StartProcesses.StartSubCopySrc {
		proc.startup.startProcess(proc, CopySrc, nil)
	}

	if proc.configuration.StartProcesses.StartSubCopyDst {
		proc.startup.startProcess(proc, CopyDst, nil)
	}

	if proc.configuration.StartProcesses.StartSubHello {

		proc.startup.startProcess(proc, Hello, procFuncHelloSubscriber)
	}

	if proc.configuration.StartProcesses.IsCentralErrorLogger {
		proc.startup.startProcess(proc, ErrorLog, nil)
	}

	if proc.configuration.StartProcesses.StartSubCliCommand {
		proc.startup.startProcess(proc, CliCommand, nil)
	}

	if proc.configuration.StartProcesses.StartSubConsole {
		proc.startup.startProcess(proc, Console, nil)
	}

	if proc.configuration.StartProcesses.StartPubHello != 0 {
		proc.startup.startProcess(proc, HelloPublisher, procFuncHelloPublisher)
	}

	if proc.configuration.StartProcesses.EnableKeyUpdates {
		// The key update on the client is only a proc func that publish requests.
		proc.startup.startProcess(proc, None, procFuncKeysRequestUpdate)
		proc.startup.startProcess(proc, KeysUpdateReceive, nil)
	}

	if proc.configuration.StartProcesses.EnableAclUpdates {

		proc.startup.startProcess(proc, AclRequestUpdate, procFuncAclRequestUpdate)
		proc.startup.startProcess(proc, AclDeliverUpdate, nil)
	}

	if proc.configuration.StartProcesses.IsCentralKey {
		proc.startup.startProcess(proc, KeysUpdateRequest, nil)
		proc.startup.startProcess(proc, KeysAllow, nil)
		proc.startup.startProcess(proc, KeysDelete, nil)
	}

	if proc.configuration.StartProcesses.IsCentralAcl {
		proc.startup.startProcess(proc, KeysUpdateRequest, nil)
		proc.startup.startProcess(proc, KeysAllow, nil)
		proc.startup.startProcess(proc, KeysDelete, nil)
		proc.startup.startProcess(proc, AclRequestUpdate, nil)
		proc.startup.startProcess(proc, AclAddCommand, nil)
		proc.startup.startProcess(proc, AclDeleteCommand, nil)
		proc.startup.startProcess(proc, AclDeleteSource, nil)
		proc.startup.startProcess(proc, AclGroupNodesAddNode, nil)
		proc.startup.startProcess(proc, AclGroupNodesDeleteNode, nil)
		proc.startup.startProcess(proc, AclGroupNodesDeleteGroup, nil)
		proc.startup.startProcess(proc, AclGroupCommandsAddCommand, nil)
		proc.startup.startProcess(proc, AclGroupCommandsDeleteCommand, nil)
		proc.startup.startProcess(proc, AclGroupCommandsDeleteGroup, nil)
		proc.startup.startProcess(proc, AclExport, nil)
		proc.startup.startProcess(proc, AclImport, nil)
	}

	if proc.configuration.StartProcesses.StartSubHttpGet {
		proc.startup.startProcess(proc, HttpGet, nil)
	}

	if proc.configuration.StartProcesses.StartSubTailFile {
		proc.startup.startProcess(proc, TailFile, nil)
	}

	if proc.configuration.StartProcesses.StartSubCliCommandCont {
		proc.startup.startProcess(proc, CliCommandCont, nil)
	}

	proc.startup.startProcess(proc, PublicKey, nil)

	// TODO: Create configuration flags for enabling processes.
	proc.startup.startProcess(proc, PortSrc, nil)
	proc.startup.startProcess(proc, PortDst, nil)
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

// startProcess will start a process. It takes the initial process, request method,
// and a procFunc as it's input arguments. If a procFunc is not needed, use the value nil.
func (s *startup) startProcess(p process, m Method, pf func(ctx context.Context, proc process, procFuncCh chan Message) error) {
	p.errorKernel.logDebug("starting subscriber", "node", m, p.node)

	var sub Subject
	switch {
	case m == ErrorLog:
		sub = newSubject(m, "errorCentral")
	default:
		sub = newSubject(m, string(p.node))
	}

	fmt.Printf("DEBUG:::startup subscriber, subject: %v\n", sub)
	proc := newProcess(p.ctx, p.processes.server, sub)
	proc.procFunc = pf

	go proc.start(true)

	// If NodeAlias is set, also start a process for each alias.
	if p.configuration.NodeAlias != "" {
		for _, alias := range strings.Split(p.configuration.NodeAlias, ",") {
			sub = newSubject(m, alias)
			proc := newProcess(p.ctx, p.processes.server, sub)
			proc.procFunc = pf

			go proc.start(false)
		}
	}
}

// ---------------------------------------------------------------

// Print the content of the processes map.
func (p *processes) printProcessesMap() {
	p.errorKernel.logDebug("output of processes map : ")

	{
		p.active.mu.Lock()

		for pName, proc := range p.active.procNames {
			p.errorKernel.logDebug("process map", "name", pName, "ID", proc.processID, "subject", proc.subject.name())
		}

		p.metrics.promProcessesTotal.Set(float64(len(p.active.procNames)))

		p.active.mu.Unlock()
	}

}
