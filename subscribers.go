package steward

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
)

func (s *server) subscribersStart() {
	// Start a subscriber for CLICommand messages
	{
		fmt.Printf("Starting CLICommand subscriber: %#v\n", s.nodeName)
		sub := newSubject(CLICommand, CommandACK, s.nodeName)
		proc := newProcess(s.processes, sub, s.errorKernel.errorCh, processKindSubscriber, []node{"central", "ship2"}, nil)
		// fmt.Printf("*** %#v\n", proc)
		go proc.spawnWorker(s)
	}

	// Start a subscriber for textLogging messages
	{
		fmt.Printf("Starting textlogging subscriber: %#v\n", s.nodeName)
		sub := newSubject(TextLogging, EventACK, s.nodeName)
		proc := newProcess(s.processes, sub, s.errorKernel.errorCh, processKindSubscriber, []node{"*"}, nil)
		// fmt.Printf("*** %#v\n", proc)
		go proc.spawnWorker(s)
	}

	// Start a subscriber for SayHello messages
	{
		fmt.Printf("Starting SayHello subscriber: %#v\n", s.nodeName)
		sub := newSubject(SayHello, EventNACK, s.nodeName)
		proc := newProcess(s.processes, sub, s.errorKernel.errorCh, processKindSubscriber, []node{"*"}, nil)
		// fmt.Printf("*** %#v\n", proc)
		go proc.spawnWorker(s)
	}

	if s.centralErrorLogger {
		// Start a subscriber for ErrorLog messages
		{
			fmt.Printf("Starting ErrorLog subscriber: %#v\n", s.nodeName)
			sub := newSubject(ErrorLog, EventNACK, "errorCentral")
			proc := newProcess(s.processes, sub, s.errorKernel.errorCh, processKindSubscriber, []node{"*"}, nil)
			proc.procFunc = func() error {
				sayHelloNodes := make(map[node]struct{})
				for {
					m := <-proc.procFuncCh
					sayHelloNodes[m.FromNode] = struct{}{}

					// update the prometheus metrics
					s.metrics.metricsCh <- metricType{
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
}
