package steward

import (
	"fmt"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func (s *server) ProcessesStart() {

	// --- Subscriber services that can be started via flags

	{
		fmt.Printf("Starting REQOpCommand subscriber: %#v\n", s.nodeName)
		sub := newSubject(REQOpCommand, s.nodeName)
		proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, []node{"*"}, nil)
		go proc.spawnWorker(s)
	}

	// Start a subscriber for textLogging messages
	if s.configuration.StartSubREQTextToLogFile.OK {
		{
			fmt.Printf("Starting text logging subscriber: %#v\n", s.nodeName)
			sub := newSubject(REQTextToLogFile, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubREQTextToLogFile.Values, nil)
			// fmt.Printf("*** %#v\n", proc)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for Hello messages
	if s.configuration.StartSubReqHello.OK {
		{
			fmt.Printf("Starting Hello subscriber: %#v\n", s.nodeName)
			sub := newSubject(ReqHello, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubReqHello.Values, nil)
			proc.procFuncCh = make(chan Message)

			// The reason for running the say hello subscriber as a procFunc is that
			// a handler are not able to hold state, and we need to hold the state
			// of the nodes we've received hello's from in the sayHelloNodes map,
			// which is the information we pass along to generate metrics.
			proc.procFunc = func() error {
				sayHelloNodes := make(map[node]struct{})
				for {
					// Receive a copy of the message sent from the method handler.
					m := <-proc.procFuncCh
					fmt.Printf("--- DEBUG : procFunc call:kind=%v, Subject=%v, toNode=%v\n", proc.processKind, proc.subject, proc.subject.ToNode)

					sayHelloNodes[m.FromNode] = struct{}{}

					// update the prometheus metrics
					proc.processes.metricsCh <- metricType{
						metric: prometheus.NewGauge(prometheus.GaugeOpts{
							Name: "hello_nodes",
							Help: "The current number of total nodes who have said hello",
						}),
						value: float64(len(sayHelloNodes)),
					}
				}
			}
			go proc.spawnWorker(s)
		}
	}

	if s.configuration.StartSubErrorLog.OK {
		// Start a subscriber for ErrorLog messages
		{
			fmt.Printf("Starting ErrorLog subscriber: %#v\n", s.nodeName)
			sub := newSubject(ErrorLog, "errorCentral")
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubErrorLog.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for ECHORequest messages
	if s.configuration.StartSubEchoRequest.OK {
		{
			fmt.Printf("Starting Echo Request subscriber: %#v\n", s.nodeName)
			sub := newSubject(ECHORequest, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubEchoRequest.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for ECHOReply messages
	if s.configuration.StartSubEchoReply.OK {
		{
			fmt.Printf("Starting Echo Reply subscriber: %#v\n", s.nodeName)
			sub := newSubject(ECHOReply, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubEchoReply.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for REQCliCommand messages
	if s.configuration.StartSubREQCliCommand.OK {
		{
			fmt.Printf("Starting CLICommand Request subscriber: %#v\n", s.nodeName)
			sub := newSubject(REQCliCommand, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubREQCliCommand.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for Not In Order Cli Command Request messages
	if s.configuration.StartSubREQnCliCommand.OK {
		{
			fmt.Printf("Starting CLICommand Not Sequential Request subscriber: %#v\n", s.nodeName)
			sub := newSubject(REQnCliCommand, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubREQnCliCommand.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// Start a subscriber for CLICommandReply messages
	if s.configuration.StartSubREQTextToConsole.OK {
		{
			fmt.Printf("Starting Text To Console subscriber: %#v\n", s.nodeName)
			sub := newSubject(REQTextToConsole, s.nodeName)
			proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindSubscriber, s.configuration.StartSubREQTextToConsole.Values, nil)
			go proc.spawnWorker(s)
		}
	}

	// --- Publisher services that can be started via flags

	// --------- Testing with publisher ------------
	// Define a process of kind publisher with subject for SayHello to central,
	// and register a procFunc with the process that will handle the actual
	// sending of say hello.
	if s.configuration.StartPubReqHello != 0 {
		fmt.Printf("Starting Hello Publisher: %#v\n", s.nodeName)

		sub := newSubject(ReqHello, s.configuration.CentralNodeName)
		proc := newProcess(s.processes, s.toRingbufferCh, s.configuration, sub, s.errorKernel.errorCh, processKindPublisher, []node{}, nil)

		// Define the procFunc to be used for the process.
		proc.procFunc = procFunc(
			func() error {
				for {
					fmt.Printf("--- DEBUG : procFunc call:kind=%v, Subject=%v, toNode=%v\n", proc.processKind, proc.subject, proc.subject.ToNode)

					d := fmt.Sprintf("Hello from %v\n", s.nodeName)

					m := Message{
						ToNode:   "central",
						FromNode: node(s.nodeName),
						Data:     []string{d},
						Method:   ReqHello,
					}

					sam, err := newSAM(m)
					if err != nil {
						// In theory the system should drop the message before it reaches here.
						log.Printf("error: ProcessesStart: %v\n", err)
					}
					proc.toRingbufferCh <- []subjectAndMessage{sam}
					time.Sleep(time.Second * time.Duration(s.configuration.StartPubReqHello))
				}
			})
		go proc.spawnWorker(s)
	}
}
