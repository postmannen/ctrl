package ctrl

import (
	"context"
	"crypto/ed25519"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	// "google.golang.org/protobuf/internal/errors"
)

// process holds all the logic to handle a message type and it's
// method, subscription/publishin messages for a subject, and more.
type process struct {
	// isSubProcess is used to indentify subprocesses spawned by other processes.
	isSubProcess bool
	// server
	server *server
	// messageID
	messageID int
	// the subject used for the specific process. One process
	// can contain only one sender on a message bus, hence
	// also one subject
	subject Subject
	// Put a node here to be able know the node a process is at.
	node Node
	// The processID for the current process
	processID int
	// methodsAvailable
	methodsAvailable MethodsAvailable
	// procFunc is a function that will be started when a worker process
	// is started. If a procFunc is registered when creating a new process
	// the procFunc will be started as a go routine when the process is started,
	// and stopped when the process is stopped.
	//
	// A procFunc can be started both for publishing and subscriber processes.
	//
	// When used with a subscriber process the usecase is most likely to handle
	// some kind of state needed for a request type. The handlers themselves
	// can not hold state since they are only called once per message received,
	// and exits when the message is handled leaving no state behind. With a procfunc
	// we can have a process function running at all times tied to the process, and
	// this function can be able to hold the state needed in a certain scenario.
	//
	// With a subscriber handler you generally take the message in the handler and
	// pass it on to the procFunc by putting it on the procFuncCh<-, and the
	// message can then be read from the procFuncCh inside the procFunc, and we
	// can do some further work on it, for example update registry for metrics that
	// is needed for that specific request type.
	//
	// With a publisher process you can attach a static function that will do some
	// work to a request type, and publish the result.
	//
	// procFunc's can also be used to wrap in other types which we want to
	// work with. An example can be handling of metrics which the message
	// have no notion of, but a procFunc can have that wrapped in from when it was constructed.
	procFunc func(ctx context.Context, proc process, procFuncCh chan Message) error
	// The channel to send a messages to the procFunc go routine.
	// This is typically used within the methodHandler for so we
	// can pass messages between the procFunc and the handler.
	procFuncCh chan Message
	// copy of the configuration from server
	configuration *Configuration
	// The new messages channel copied from *Server
	newMessagesCh chan<- Message
	// The structure who holds all processes information
	processes *processes
	// nats connection
	natsConn *nats.Conn
	// natsSubscription returned when calling natsConn.Subscribe
	natsSubscription *nats.Subscription
	// context
	ctx context.Context
	// context cancelFunc
	ctxCancel context.CancelFunc
	// Process name
	processName processName

	// handler is used to directly attach a handler to a process upon
	// creation of the process, like when a process is spawning a sub
	// process like copySrc do. If we're not spawning a sub process
	// and it is a regular process the handler to use is found with the
	// getHandler method
	handler func(proc process, message Message, node string) ([]byte, error)

	// startup holds the startup functions for starting up publisher
	// or subscriber processes
	startup *startup
	// Signatures
	nodeAuth *nodeAuth
	// centralAuth
	centralAuth *centralAuth
	// errorKernel
	errorKernel *errorKernel
	// metrics
	metrics *metrics
}

// prepareNewProcess will set the the provided values and the default
// values for a process.
func newProcess(ctx context.Context, server *server, subject Subject) process {
	// create the initial configuration for a sessions communicating with 1 host process.
	server.processes.mu.Lock()
	server.processes.lastProcessID++
	pid := server.processes.lastProcessID
	server.processes.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)

	var method Method

	proc := process{
		server:           server,
		messageID:        0,
		subject:          subject,
		node:             Node(server.configuration.NodeName),
		processID:        pid,
		methodsAvailable: method.GetMethodsAvailable(),
		newMessagesCh:    server.newMessagesCh,
		configuration:    server.configuration,
		processes:        server.processes,
		natsConn:         server.natsConn,
		ctx:              ctx,
		ctxCancel:        cancel,
		startup:          newStartup(server),
		nodeAuth:         server.nodeAuth,
		centralAuth:      server.centralAuth,
		errorKernel:      server.errorKernel,
		metrics:          server.metrics,
	}

	// We use the full name of the subject to identify a unique
	// process. We can do that since a process can only handle
	// one request type.
	proc.processName = processNameGet(proc.subject.name())

	return proc
}

// The purpose of this function is to check if we should start a
// publisher or subscriber process, where a process is a go routine
// that will handle either sending or receiving messages on one
// subject.
//
// It will give the process the next available ID, and also add the
// process to the processes map in the server structure.
func (p process) start() {

	// Add prometheus metrics for the process.
	if !p.isSubProcess {
		p.metrics.promProcessesAllRunning.With(prometheus.Labels{"processName": string(p.processName)})
	}

	// Start a subscriber worker, which will start a go routine (process)
	// to handle executing the request method defined in the message.
	p.startSubscriber()

	// Add information about the new process to the started processes map.
	p.processes.active.mu.Lock()
	p.processes.active.procNames[p.processName] = p
	p.processes.active.mu.Unlock()

	p.errorKernel.logDebug("successfully started process", "processName", p.processName)
}

func (p process) startSubscriber() {
	// If there is a procFunc for the process, start it.
	if p.procFunc != nil {
		// Initialize the channel for communication between the proc and
		// the procFunc.
		p.procFuncCh = make(chan Message)

		// Start the procFunc in it's own anonymous func so we are able
		// to get the return error.
		go func() {
			err := p.procFunc(p.ctx, p, p.procFuncCh)
			if err != nil {
				er := fmt.Errorf("error: spawnWorker: start procFunc failed: %v", err)
				p.errorKernel.errSend(p, Message{}, er, logError)
			}
		}()
	}

	p.natsSubscription = p.startNatsSubscriber()

	// We also need to be able to remove all the information about this process
	// when the process context is canceled.
	go func() {
		<-p.ctx.Done()
		err := p.natsSubscription.Unsubscribe()
		if err != nil {
			er := fmt.Errorf("error: spawnWorker: got <-ctx.Done, but unable to unsubscribe natsSubscription failed: %v", err)
			p.errorKernel.errSend(p, Message{}, er, logError)
		}

		p.processes.active.mu.Lock()
		delete(p.processes.active.procNames, p.processName)
		p.processes.active.mu.Unlock()

		p.errorKernel.logDebug("successfully stopped process", "processName", p.processName)

	}()
}

var (
	ErrACKSubscribeRetry = errors.New("ctrl: retrying to subscribe for ack message")
	// TODO: Other errors are not used for anything other than printing.
	ErrOther = errors.New("other error")
)

// publishNats will create the Nats message with headers and payload.
// The payload of the nats message, which is the ctrl message will be
// serialized and compress before put in the data field of the nats
// message.
// It will also take care of resending if not delievered, and timeouts.
func (p process) publishNats(natsMsgPayload []byte, natsMsgHeader nats.Header, natsConn *nats.Conn, message Message) {
	retryAttempts := 0

	if message.RetryWait <= 0 {
		message.RetryWait = 0
	}

	subject := newSubject(message.Method, string(message.ToNode))

	// The for loop will run until the message is delivered successfully,
	// or that retries are reached.
	for {
		msg := &nats.Msg{
			Subject: string(subject.name()),
			// Subject: fmt.Sprintf("%s.%s.%s", proc.node, "command", "CLICommandRequest"),
			// Structure of the reply message are:
			// <nodename>.<message type>.<method>.reply
			Reply:  fmt.Sprintf("%s.reply", subject.name()),
			Data:   natsMsgPayload,
			Header: natsMsgHeader,
		}

		p.errorKernel.logDebug("publishNats: preparing to send nats message", "subject", msg.Subject, "id", message.ID)

		var err error

		switch {
		// If it is a NACK message we just deliver the message and return
		// here so we don't create a ACK message and then stop waiting for it.
		case message.ACKTimeout < 1:
			err = func() error {
				err := natsConn.PublishMsg(msg)
				if err != nil {
					p.errorKernel.logDebug("publishNats: nats publish for message with subject failed", "error", err)
					return ErrACKSubscribeRetry
				}
				p.metrics.promNatsDeliveredTotal.Inc()

				// The remaining logic is for handling ACK messages, so we return here
				// since it was a NACK message, and all or now done.

				return nil
			}()

		case message.ACKTimeout >= 1:
			// The function below will return nil if the message should not be retried.
			//
			// All other errors happening will return ErrACKSubscribeRetry which will lead
			// to a 'continue' for the for loop when checking the error directly after this
			// function is called
			err = func() error {
				defer func() { retryAttempts++ }()

				if retryAttempts > message.Retries {
					// max retries reached
					er := fmt.Errorf("info: toNode: %v, fromNode: %v, subject: %v, methodArgs: %v: max retries reached, check if node is up and running and if it got a subscriber started for the given REQ type", message.ToNode, message.FromNode, msg.Subject, message.MethodArgs)

					// We do not want to send errorLogs for REQErrorLog type since
					// it will just cause an endless loop.
					if message.Method != ErrorLog {
						p.errorKernel.infoSend(p, message, er)
					}

					p.metrics.promNatsMessagesFailedACKsTotal.Inc()
					return nil
				}

				p.errorKernel.logDebug("publishNats: ", "retryAttempts", retryAttempts, "retries", message.Retries, "ACKTimeout", message.ACKTimeout, "ID", message.ID, "method", message.Method, "toNode", message.ToNode)

				// The SubscribeSync used in the subscriber, will get messages that
				// are sent after it started subscribing.
				//
				// Create a subscriber for the ACK reply message.
				subReply, err := natsConn.SubscribeSync(msg.Reply)
				defer func() {
					err := subReply.Unsubscribe()
					if err != nil {
						p.errorKernel.logDebug("nats SubscribeSync: failed when unsubscribing for ACK", "error", err)
					}
				}()
				if err != nil {
					er := fmt.Errorf("error: nats SubscribeSync failed: failed to create reply message for subject: %v, error: %v", msg.Reply, err)
					// sendErrorLogMessage(p.toRingbufferCh, node(p.node), er)
					p.errorKernel.logDebug("publishNats: waiting equal to RetryWait before retrying", "error", er, "RetryWait", message.RetryWait)

					time.Sleep(time.Second * time.Duration(message.RetryWait))

					return ErrACKSubscribeRetry
				}

				// Publish message
				err = natsConn.PublishMsg(msg)
				if err != nil {
					p.errorKernel.logDebug("publishNats: nats publish failed, waiting equal to RetryWait before retrying", "error", err, "RetryWait", message.RetryWait)

					time.Sleep(time.Second * time.Duration(message.RetryWait))

					return ErrACKSubscribeRetry
				}

				// Wait up until ACKTimeout specified for a reply,
				// continue and resend if no reply received,
				// or exit if max retries for the message reached.
				//
				// The nats.Msg returned is discarded with '_' since
				// we don't use it.
				_, err = subReply.NextMsg(time.Second * time.Duration(message.ACKTimeout))
				if err != nil {

					switch {
					case err == nats.ErrNoResponders || err == nats.ErrTimeout:
						p.errorKernel.logDebug("publishNats: ack receive failed: waiting before retrying", "seconds", message.RetryWait, "subject", subject.name(), "error", err)

						time.Sleep(time.Second * time.Duration(message.RetryWait))
						p.metrics.promNatsMessagesMissedACKsTotal.Inc()

						return ErrACKSubscribeRetry

					case err == nats.ErrBadSubscription || err == nats.ErrConnectionClosed:
						p.errorKernel.logDebug("publishNats: ack receive failed: conneciton closed or bad subscription, will not retry message", "subject", subject.name(), "error", err)

						return ErrOther

					default:
						p.errorKernel.logDebug("error: ack receive failed: the error was not defined, check if nats client have been updated with new error values, and update ctrl to handle the new error type", "subject", subject.name(), "error", err)

						return ErrOther
					}

				}

				return nil
			}()
		}

		if err == ErrACKSubscribeRetry {
			continue
		}
		if err != nil {
			// All error printing are handled within the function that returns
			// the error, so we do nothing and return.
			// No more trying to deliver the message
			return
		}

		// Message were delivered successfully.
		p.metrics.promNatsDeliveredTotal.Inc()

		p.errorKernel.logDebug("publishNats: sent message", "subject", msg.Subject, "ID", message.ID)

		return
	}
}

// messageSubscriberHandler will deserialize the message when a new message is
// received, check the MessageType field in the message to decide what
// kind of message it is and then it will check how to handle that message type,
// and then call the correct method handler for it.
//
// This function should be started in it's own go routine,so
// one individual handler is started per message received so we can keep
// the state of the message being processed, and then reply back to the
// correct sending process's reply, meaning so we ACK back to the correct
// publisher.
func (p process) messageSubscriberHandler(natsConn *nats.Conn, thisNode string, msg *nats.Msg, subject string) {

	// Variable to hold a copy of the message data, so we don't mess with
	// the original data since the original is a pointer value.
	msgData := make([]byte, len(msg.Data))
	copy(msgData, msg.Data)

	// fmt.Printf(" * DEBUG: header value on subscriberHandler: %v\n", msg.Header)

	// If debugging is enabled, print the source node name of the nats messages received.
	if val, ok := msg.Header["fromNode"]; ok {
		p.errorKernel.logDebug("nats message received from", "node", val, "subject", subject)
	}

	message, err := p.server.messageDeserializeAndUncompress(msgData)
	if err != nil {
		er := fmt.Errorf("error: messageSubscriberHandler: deserialize and uncompress failed: %v", err)
		// p.errorKernel.logDebug(er)
		log.Fatalf("%v\n", er)
	}

	// Check if it is an ACK or NACK message, and do the appropriate action accordingly.
	//
	// With ACK messages ctrl will keep the state of the message delivery, and try to
	// resend the message if an ACK is not received within the timeout/retries specified
	// in the message.
	// When a process sends an ACK message, it will stop and wait for the nats-reply message
	// for the time specified in the replyTimeout value. If no reply message is received
	// within the given timeout the publishing process will try to resend the message for
	// number of times specified in the retries field of the ctrl message.
	// When receiving a ctrl-message with ACK enabled we send a message back the the
	// node where the message originated using the msg.Reply subject field of the nats-message.
	//
	// With NACK messages we do not send a nats reply message, so the message will only be
	// sent from the publisher once, and if it is not delivered it will not be retried.
	switch {

	// Check for ACK type Event.
	case message.ACKTimeout >= 1:
		p.errorKernel.logDebug("messageSubscriberHandler: received ACK message from", "method", message.Method, "fromNode", message.FromNode, "ID", message.ID)
		// When spawning sub processes we can directly assign handlers to the process upon
		// creation. We here check if a handler is already assigned, and if it is nil, we
		// lookup and find the correct handler to use if available.
		if p.handler == nil {
			// Look up the method handler for the specified method.
			mh, ok := p.methodsAvailable.CheckIfExists(message.Method)
			p.handler = mh
			if !ok {
				er := fmt.Errorf("error: subscriberHandler: no such method type: %v", p.subject.Method)
				p.errorKernel.errSend(p, message, er, logWarning)
			}
		}

		//var err error

		p.callHandler(message, thisNode)

		// Send a confirmation message back to the publisher to ACK that the
		// message was received by the subscriber. The reply should be sent
		// no matter if the handler was executed successfully or not
		natsConn.Publish(msg.Reply, []byte{})

	case message.ACKTimeout < 1:
		p.errorKernel.logDebug("messageSubscriberHandler: received NACK message from", ",method", message.Method, "fromNode", message.FromNode, "ID", message.ID)
		// When spawning sub processes we can directly assign handlers to the process upon
		// creation. We here check if a handler is already assigned, and if it is nil, we
		// lookup and find the correct handler to use if available.
		if p.handler == nil {
			// Look up the method handler for the specified method.
			mh, ok := p.methodsAvailable.CheckIfExists(message.Method)
			p.handler = mh
			if !ok {
				er := fmt.Errorf("error: subscriberHandler: no such method type: %v", p.subject.Method)
				p.errorKernel.errSend(p, message, er, logWarning)
			}
		}

		// We do not send reply messages for EventNACL, so we can discard the output.
		p.callHandler(message, thisNode)

	default:
		er := fmt.Errorf("info: did not find that specific type of event: %#v", p.subject.Method)
		p.errorKernel.infoSend(p, message, er)

	}
}

// callHandler will call the handler for the Request type defined in the message.
// If checking signatures and/or acl's are enabled the signatures will be verified,
// and if OK the handler is called.
func (p process) callHandler(message Message, thisNode string) {
	//out := []byte{}

	// Call the handler if ACL/signature checking returns true.
	go func() {
		conf := p.nodeAuth.configuration

		p.errorKernel.logDebug("callhandler: checking how to call message", "fromNode", message.FromNode, "method", message.Method, "EnableSignatureCheck", conf.EnableSignatureCheck, "EnableAclCheck", conf.EnableAclCheck)

		switch {
		// If no checking enabled we should just allow the message.
		case !conf.EnableSignatureCheck && !conf.EnableAclCheck:
			p.errorKernel.logDebug("callhandler: NO CHECK OF SIG OR ACL FLAG ENABLED, EXECUTING HANDLER", "method", message.Method)

			executeHandler(p, message, thisNode)
			return

		// If only sig check enabled, and sig OK, we should allow the message.
		case conf.EnableSignatureCheck && !conf.EnableAclCheck:
			sigOK := p.nodeAuth.verifySignature(message)
			p.errorKernel.logDebug("callhandler: CHECK SIG TRUE", "method", message.Method)

			if sigOK {
				p.errorKernel.logDebug("callhandler: CHECK SIG TRUE EVALUATED TO TRUE, EXECUTING HANDLER", "method", message.Method)

				executeHandler(p, message, thisNode)
				return
			}
			p.errorKernel.logDebug("callhandler: CHECK SIG TRUE EVALUATED TO FALSE", "method", message.Method)

		// If both sig and acl check enabled, and sig and acl OK, we should allow the message.
		case conf.EnableSignatureCheck && conf.EnableAclCheck:
			sigOK := p.nodeAuth.verifySignature(message)
			aclOK := p.nodeAuth.verifyAcl(message)
			p.errorKernel.logDebug("callhandler: CHECK SIG AND ACK TRUE", "method", message.Method)

			if sigOK && aclOK {
				p.errorKernel.logDebug("callhandler: CHECK SIG AND ACK TRUE EVALUATED TO FALSE, EXECUTING HANDLER", "method", message.Method)

				executeHandler(p, message, thisNode)
				return
			}
			p.errorKernel.logDebug("callhandler: CHECK SIG AND ACK TRUE EVALUATED TO FALSE", "method", message.Method)

		default:
			p.errorKernel.logDebug("callHandler: None of the verify flags matched, not doing handler for message", "method", message.Method)
		}

		er := fmt.Errorf("error: subscriberHandler: ACL or Signature were verified not-OK, doing nothing")
		p.errorKernel.errSend(p, message, er, logWarning)

	}()

}

// executeHandler will call the handler for the Request type defined in the message.
// Will also take care of executing a method as scheduled.
func executeHandler(p process, message Message, thisNode string) {
	var err error

	// Check if it is a message to run scheduled.
	var interval int
	var totalTime int
	var runAsScheduled bool
	switch {
	case len(message.Schedule) < 2:
		// Not at scheduled message,
	case len(message.Schedule) == 2:
		interval = message.Schedule[0]
		totalTime = message.Schedule[1]
		fallthrough

	case interval > 0 && totalTime > 0:
		runAsScheduled = true
	}

	switch {
	case !runAsScheduled && p.handler != nil:

		go func() {
			_, err = p.handler(p, message, thisNode)
			if err != nil {
				er := fmt.Errorf("error: subscriberHandler: handler method failed: %v", err)
				p.errorKernel.errSend(p, message, er, logError)
			}
		}()

	case runAsScheduled:
		// Create two tickers to use for the scheduling.
		intervalTicker := time.NewTicker(time.Second * time.Duration(interval))
		totalTimeTicker := time.NewTicker(time.Second * time.Duration(totalTime))
		defer intervalTicker.Stop()
		defer totalTimeTicker.Stop()

		// Run the handler once, so we don't have to wait for the first ticker.
		go func() {
			_, err := p.handler(p, message, thisNode)
			if err != nil {
				er := fmt.Errorf("error: subscriberHandler: handler method failed: %v", err)
				p.errorKernel.errSend(p, message, er, logError)
			}
		}()

		for {
			select {
			case <-p.ctx.Done():
				p.errorKernel.logDebug("executeHandler: proc ctx done", "toNode", message.ToNode, "fromNode", message.FromNode, "method", message.Method, "methodArgs", message.MethodArgs)

				//cancel()
				return
			case <-totalTimeTicker.C:
				// Total time reached. End the process.
				//cancel()
				p.errorKernel.logDebug("executeHandler: schedule totalTime done", "toNode", message.ToNode, "fromNode", message.FromNode, "method", message.Method, "methodArgs", message.MethodArgs)

				return

			case <-intervalTicker.C:
				go func() {
					_, err := p.handler(p, message, thisNode)
					if err != nil {
						er := fmt.Errorf("error: subscriberHandler: handler method failed: %v", err)
						p.errorKernel.errSend(p, message, er, logError)
					}
				}()
			}
		}
	}
}

// SubscribeMessage will register the Nats callback function for the specified
// nats subject. This allows us to receive Nats messages for a given subject
// on a node.
func (p process) startNatsSubscriber() *nats.Subscription {
	subject := string(p.subject.name())
	// natsSubscription, err := p.natsConn.Subscribe(subject, func(msg *nats.Msg) {
	natsSubscription, err := p.natsConn.QueueSubscribe(subject, subject, func(msg *nats.Msg) {
		//_, err := p.natsConn.Subscribe(subject, func(msg *nats.Msg) {

		// Start up the subscriber handler.
		go p.messageSubscriberHandler(p.natsConn, p.configuration.NodeName, msg, subject)
	})
	if err != nil {
		p.errorKernel.logDebug("Subscribe failed", "error", err)
		return nil
	}

	return natsSubscription
}

func (p process) addMethodArgSignature(m Message) []byte {
	argsString := argsToString(m.MethodArgs)
	sign := ed25519.Sign(p.nodeAuth.SignPrivateKey, []byte(argsString))

	return sign
}

func (p process) publishAMessage(m Message, natsConn *nats.Conn) {
	// Create the initial header, and set values below depending on the
	// various configuration options chosen.
	natsMsgHeader := make(nats.Header)
	natsMsgHeader["fromNode"] = []string{string(p.node)}

	b, err := p.server.messageSerializeAndCompress(m)
	if err != nil {
		p.errorKernel.logDebug("publishAMessage: serialize and compress failed", "error", err)
		return
	}

	// Create the Nats message with headers and payload, and do the
	// sending of the message.
	p.publishNats(b, natsMsgHeader, natsConn, m)

	// Get the process name so we can look up the process in the
	// processes map, and increment the message counter.
	pn := processNameGet(p.subject.name())
	// Increment the counter for the next message to be sent.
	p.messageID++

	{
		p.processes.active.mu.Lock()
		p.processes.active.procNames[pn] = p
		p.processes.active.mu.Unlock()
	}

	// // Handle the error.
	// //
	// // NOTE: None of the processes above generate an error, so the the
	// // if clause will never be triggered. But keeping it here as an example
	// // for now for how to handle errors.
	// if err != nil {
	// 	// Create an error type which also creates a channel which the
	// 	// errorKernel will send back the action about what to do.
	// 	ep := errorEvent{
	// 		//errorType:     logOnly,
	// 		process:       p,
	// 		message:       m,
	// 		errorActionCh: make(chan errorAction),
	// 	}
	// 	p.errorCh <- ep
	//
	// 	// Wait for the response action back from the error kernel, and
	// 	// decide what to do. Should we continue, quit, or .... ?
	// 	switch <-ep.errorActionCh {
	// 	case errActionContinue:
	// 		// Just log and continue
	// 		log.Printf("The errAction was continue...so we're continuing\n")
	// 	case errActionKill:
	// 		log.Printf("The errAction was kill...so we're killing\n")
	// 		// ....
	// 	default:
	// 		log.Printf("Info: publishMessages: The errAction was not defined, so we're doing nothing\n")
	// 	}
	// }
}
