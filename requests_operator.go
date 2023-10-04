package ctrl

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// --- OpProcessList

// Handle Op Process List
func methodREQOpProcessList(proc process, message Message, node string) ([]byte, error) {

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		out := []byte{}

		// Loop the the processes map, and find all that is active to
		// be returned in the reply message.

		proc.processes.active.mu.Lock()
		for _, pTmp := range proc.processes.active.procNames {
			s := fmt.Sprintf("%v, process: %v, id: %v, name: %v\n", time.Now().Format("Mon Jan _2 15:04:05 2006"), pTmp.processKind, pTmp.processID, pTmp.subject.name())
			sb := []byte(s)
			out = append(out, sb...)

		}
		proc.processes.active.mu.Unlock()

		newReplyMessage(proc, message, out)
	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// --- OpProcessStart

// Handle Op Process Start
func methodREQOpProcessStart(proc process, message Message, node string) ([]byte, error) {
	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()
		var out []byte

		// We need to create a tempory method type to look up the kind for the
		// real method for the message.
		var mt Method

		switch {
		case len(message.MethodArgs) < 1:
			er := fmt.Errorf("error: methodREQOpProcessStart: got <1 number methodArgs")
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		m := message.MethodArgs[0]
		method := Method(m)
		tmpH := mt.getHandler(Method(method))
		if tmpH == nil {
			er := fmt.Errorf("error: OpProcessStart: no such request type defined: %v" + m)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		// Create the process and start it.
		sub := newSubject(method, proc.configuration.NodeName)
		procNew := newProcess(proc.ctx, proc.server, sub, processKindSubscriber, nil)
		go procNew.spawnWorker()

		txt := fmt.Sprintf("info: OpProcessStart: started id: %v, subject: %v: node: %v", procNew.processID, sub, message.ToNode)
		er := fmt.Errorf(txt)
		proc.errorKernel.errSend(proc, message, er, logWarning)

		out = []byte(txt + "\n")
		newReplyMessage(proc, message, out)
	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil

}

// --- OpProcessStop

// RecevingNode Node        `json:"receivingNode"`
// Method       Method      `json:"method"`
// Kind         processKind `json:"kind"`
// ID           int         `json:"id"`

// Handle Op Process Start
func methodREQOpProcessStop(proc process, message Message, node string) ([]byte, error) {
	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()
		var out []byte

		// We need to create a tempory method type to use to look up the kind for the
		// real method for the message.
		var mt Method

		// --- Parse and check the method arguments given.
		// The Reason for also having the node as one of the arguments is
		// that publisher processes are named by the node they are sending the
		// message to. Subscriber processes names are named by the node name
		// they are running on.

		if v := len(message.MethodArgs); v != 3 {
			er := fmt.Errorf("error: methodREQOpProcessStop: got <4 number methodArgs, want: method,node,kind")
			proc.errorKernel.errSend(proc, message, er, logWarning)
		}

		methodString := message.MethodArgs[0]
		node := message.MethodArgs[1]
		kind := message.MethodArgs[2]

		method := Method(methodString)
		tmpH := mt.getHandler(Method(method))
		if tmpH == nil {
			er := fmt.Errorf("error: OpProcessStop: no such request type defined: %v, check that the methodArgs are correct: " + methodString)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		// --- Find, and stop process if found

		// Based on the arg values received in the message we create a
		// processName structure as used in naming the real processes.
		// We can then use this processName to get the real values for the
		// actual process we want to stop.
		sub := newSubject(method, string(node))
		processName := processNameGet(sub.name(), processKind(kind))

		// Remove the process from the processes active map if found.
		proc.processes.active.mu.Lock()
		toStopProc, ok := proc.processes.active.procNames[processName]

		if ok {
			// Delete the process from the processes map
			delete(proc.processes.active.procNames, processName)
			// Stop started go routines that belong to the process.
			toStopProc.ctxCancel()
			// Stop subscribing for messages on the process's subject.
			err := toStopProc.natsSubscription.Unsubscribe()
			if err != nil {
				er := fmt.Errorf("error: methodREQOpStopProcess failed to stop nats.Subscription: %v, methodArgs: %v", err, message.MethodArgs)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			// Remove the prometheus label
			proc.metrics.promProcessesAllRunning.Delete(prometheus.Labels{"processName": string(processName)})

			txt := fmt.Sprintf("info: OpProcessStop: process stopped id: %v, method: %v on: %v", toStopProc.processID, sub, message.ToNode)
			er := fmt.Errorf(txt)
			proc.errorKernel.errSend(proc, message, er, logWarning)

			out = []byte(txt + "\n")
			newReplyMessage(proc, message, out)

		} else {
			txt := fmt.Sprintf("error: OpProcessStop: did not find process to stop: %v on %v", sub, message.ToNode)
			er := fmt.Errorf(txt)
			proc.errorKernel.errSend(proc, message, er, logWarning)

			out = []byte(txt + "\n")
			newReplyMessage(proc, message, out)
		}

		proc.processes.active.mu.Unlock()

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil

}

// ----
