// Notes:
package steward

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

// MessageKind describes on the message level if this is
// an event or command kind of message in the Subject name.
// This field is mainly used to be able to spawn up different
// worker processes based on the Subject name so we can have
// one process for handling event kind, and another for
// handling command kind of messages.
type MessageKind string

// TODO: Figure it makes sense to have these types at all.
//  It might make more sense to implement these as two
//  individual subjects.
const (
	// shellCommand, command that will just wait for an
	// ack, and nothing of the output of the command are
	// delivered back in the reply ack message.
	// The message should contain the unique ID of the
	// command.
	Command MessageKind = "command"
	// shellCommand, wait for and return the output
	// of the command in the ACK message. This means
	// that the command should be executed immediately
	// and that we should get the confirmation that it
	// was successful or not.
	Event MessageKind = "event"
	// eventCommand, just wait for the ACK that the
	// message is received. What action happens on the
	// receiving side is up to the received to decide.
)

type Message struct {
	// The Unique ID of the message
	ID int `json:"id" yaml:"id"`
	// The actual data in the message
	// TODO: Change this to a slice instead...or maybe use an
	// interface type here to handle several data types ?
	Data []string `json:"data" yaml:"data"`
	// The type of the message being sent
	MessageType MessageKind `json:"messageType" yaml:"messageType"`
	FromNode    node
}

// server is the structure that will hold the state about spawned
// processes on a local instance.
type server struct {
	natsConn *nats.Conn
	// TODO: sessions should probably hold a slice/map of processes ?
	processes map[subjectName]process
	// The last processID created
	lastProcessID int
	// The name of the node
	nodeName string
	mu       sync.Mutex
	// The channel where we receive new messages from the outside to
	// insert into the system for being processed
	newMessagesCh chan []jsonFromFile
	// errorCh is used to report errors from a process
	// NB: Implementing this as an int to report for testing
	errorCh chan errProcess
	// errorKernel
	errorKernel *errorKernel
	// TODO: replace this with some structure to hold the logCh value
	logCh chan []byte
}

// newServer will prepare and return a server type
func NewServer(brokerAddress string, nodeName string) (*server, error) {
	conn, err := nats.Connect(brokerAddress, nil)
	if err != nil {
		log.Printf("error: nats.Connect failed: %v\n", err)
	}

	s := &server{
		nodeName:      nodeName,
		natsConn:      conn,
		processes:     make(map[subjectName]process),
		newMessagesCh: make(chan []jsonFromFile),
		errorCh:       make(chan errProcess, 2),
		logCh:         make(chan []byte),
	}

	// Start the error kernel that will do all the error handling
	// not done within a process.
	s.errorKernel = newErrorKernel()
	s.errorKernel.startErrorKernel(s.errorCh)

	return s, nil

}

func (s *server) Start() {
	// Start the checking the input file for new messages from operator.
	go s.getMessagesFromFile("./", "inmsg.txt", s.newMessagesCh)

	// Start the textlogging service that will run on the subscribers
	// TODO: Figure out how to structure event services like these

	go s.startTextLogging(s.logCh)

	// Start a subscriber for shellCommand messages
	{
		fmt.Printf("nodeName: %#v\n", s.nodeName)
		sub := newSubject(s.nodeName, "command", "shellcommand")
		proc := s.processPrepareNew(sub, s.errorCh, processKindSubscriber)
		// fmt.Printf("*** %#v\n", proc)
		go s.processSpawnWorker(proc)
	}

	// Start a subscriber for textLogging messages
	{
		fmt.Printf("nodeName: %#v\n", s.nodeName)
		sub := newSubject(s.nodeName, "event", "textlogging")
		proc := s.processPrepareNew(sub, s.errorCh, processKindSubscriber)
		// fmt.Printf("*** %#v\n", proc)
		go s.processSpawnWorker(proc)
	}

	time.Sleep(time.Second * 2)
	fmt.Printf("*** Output of processes map: %#v\n", s.processes)

	// Prepare and start a single process
	//{
	//	sub := newSubject("ship1", "command", "shellcommand")
	//	proc := s.processPrepareNew(sub, s.errorCh, processKindPublisher)
	//	// fmt.Printf("*** %#v\n", proc)
	//	go s.processSpawnWorker(proc)
	//}

	// Prepare and start a single process
	// {
	// 	sub := newSubject("ship2", "command", "shellcommand")
	// 	proc := s.processPrepareNew(sub, s.errorCh, processKindPublisher)
	// 	// fmt.Printf("*** %#v\n", proc)
	// 	go s.processSpawnWorker(proc)
	// }

	s.handleNewOperatorMessages()

	select {}

}

// handleNewOperatorMessages will handle all the new operator messages
// given to the system, and route them to the correct subject queue.
func (s *server) handleNewOperatorMessages() {
	// Process the messages that have been received on the incomming
	// message pipe. Check and send if there are a specific subject
	// for it, and no subject exist throw an error.
	//
	// TODO: Later on the only thing that should be checked here is
	// that there is a node for the specific message, and the super-
	// visor should create the process with the wanted subject on both
	// the publishing and the receiving node. If there is no such node
	// an error should be generated and processed by the error-kernel.
	go func() {
		for v := range s.newMessagesCh {
			for i, vv := range v {

				// Adding a label here so we are able to redo the sending
				// of the last message if a process with specified subject
				// is not present.
			redo:
				m := vv.Message
				subjName := vv.Subject.name()
				fmt.Printf("** handleNewOperatorMessages: message: %v, ** subject: %#v\n", m, vv.Subject)
				_, ok := s.processes[subjName]
				if ok {
					log.Printf("info: found the specific subject: %v\n", subjName)
					// Put the message on the correct process's messageCh
					s.processes[subjName].subject.messageCh <- m
				} else {
					// If a publisher do not exist for the given subject, create it.
					log.Printf("info: did not find that specific subject, starting new process for subject: %v\n", subjName)

					sub := newSubject(v[i].Subject.Node, v[i].Subject.MessageKind, v[i].Subject.Method)
					proc := s.processPrepareNew(sub, s.errorCh, processKindPublisher)
					// fmt.Printf("*** %#v\n", proc)
					go s.processSpawnWorker(proc)

					time.Sleep(time.Millisecond * 500)
					goto redo
				}
			}
		}
	}()
}

type node string

// subject contains the representation of a subject to be used with one
// specific process
type Subject struct {
	// node, the name of the node
	Node string `json:"node" yaml:"node"`
	// messageType, command/event
	MessageKind MessageKind `json:"messageKind" yaml:"messageKind"`
	// method, what is this message doing, etc. shellcommand, syslog, etc.
	Method string `json:"method" yaml:"method"`
	// messageCh is the channel for receiving new content to be sent
	messageCh chan Message
}

// newSubject will return a new variable of the type subject, and insert
// all the values given as arguments. It will also create the channel
// to receive new messages on the specific subject.
func newSubject(node string, messageKind MessageKind, method string) Subject {
	return Subject{
		Node:        node,
		MessageKind: messageKind,
		Method:      method,
		messageCh:   make(chan Message),
	}
}

// subjectName is the complete representation of a subject
type subjectName string

func (s Subject) name() subjectName {
	return subjectName(fmt.Sprintf("%s.%s.%s", s.Node, s.MessageKind, s.Method))
}

// processKind are either kindSubscriber or kindPublisher, and are
// used to distinguish the kind of process to spawn and to know
// the process kind put in the process map.
type processKind string

const (
	processKindSubscriber processKind = "subscriber"
	processKindPublisher  processKind = "publisher"
)

// process are represent the communication to one individual host
type process struct {
	messageID int
	// the subject used for the specific process. One process
	// can contain only one sender on a message bus, hence
	// also one subject
	subject Subject
	// Put a node here to be able know the node a process is at.
	// NB: Might not be needed later on.
	node node
	// The processID for the current process
	processID int
	// errorCh is used to report errors from a process
	// NB: Implementing this as an int to report for testing
	errorCh     chan errProcess
	processKind processKind
}

// prepareNewProcess will set the the provided values and the default
// values for a process.
func (s *server) processPrepareNew(subject Subject, errCh chan errProcess, processKind processKind) process {
	// create the initial configuration for a sessions communicating with 1 host process.
	s.lastProcessID++
	proc := process{
		messageID:   0,
		subject:     subject,
		node:        node(subject.Node),
		processID:   s.lastProcessID,
		errorCh:     errCh,
		processKind: processKind,
		//messageCh: make(chan Message),
	}

	return proc
}

// spawnProcess will spawn a new process. It will give the process
// the next available ID, and also add the process to the processes
// map.
func (s *server) processSpawnWorker(proc process) {
	s.mu.Lock()
	// We use the full name of the subject to identify a unique
	// process. We can do that since a process can only handle
	// one message queue.
	s.processes[proc.subject.name()] = proc
	s.mu.Unlock()

	// TODO: I think it makes most sense that the messages would come to
	// here from some other message-pickup-process, and that process will
	// give the message to the correct publisher process. A channel that
	// is listened on in the for loop below could be used to receive the
	// messages from the message-pickup-process.
	if proc.processKind == processKindPublisher {
		for {
			// Wait and read the next message on the message channel
			m := <-proc.subject.messageCh
			m.ID = s.processes[proc.subject.name()].messageID
			messageDeliver(proc, m, s.natsConn)

			// Increment the counter for the next message to be sent.
			proc.messageID++
			s.processes[proc.subject.name()] = proc
			time.Sleep(time.Second * 1)

			// NB: simulate that we get an error, and that we can send that
			// out of the process and receive it in another thread.
			ep := errProcess{
				infoText:      "process failed",
				process:       proc,
				message:       m,
				errorActionCh: make(chan errorAction),
			}
			s.errorCh <- ep

			// Wait for the response action back from the error kernel, and
			// decide what to do. Should we continue, quit, or .... ?
			switch <-ep.errorActionCh {
			case errActionContinue:
				log.Printf("The errAction was continue...so we're continuing\n")
			}
		}
	}

	if proc.processKind == processKindSubscriber {
		//subject := fmt.Sprintf("%s.%s.%s", s.nodeName, "command", "shellcommand")
		subject := string(proc.subject.name())

		// Subscribe will start up a Go routine under the hood calling the
		// callback function specified when a new message is received.
		_, err := s.natsConn.Subscribe(subject, func(msg *nats.Msg) {
			// We start one handler per message received by using go routines here.
			// This is for being able to reply back the current publisher who sent
			// the message.
			go s.handler(s.natsConn, s.nodeName, msg)
		})
		if err != nil {
			log.Printf("error: Subscribe failed: %v\n", err)
		}
	}
}

func messageDeliver(proc process, message Message, natsConn *nats.Conn) {
	for {
		dataPayload, err := gobEncodePayload(message)
		if err != nil {
			log.Printf("error: createDataPayload: %v\n", err)
		}

		msg := &nats.Msg{
			Subject: string(proc.subject.name()),
			// Subject: fmt.Sprintf("%s.%s.%s", proc.node, "command", "shellcommand"),
			// Structure of the reply message are:
			// reply.<nodename>.<message type>.<method>
			Reply: fmt.Sprintf("reply.%s", proc.subject.name()),
			Data:  dataPayload,
		}

		// The SubscribeSync used in the subscriber, will get messages that
		// are sent after it started subscribing, so we start a publisher
		// that sends out a message every second.
		//
		// Create a subscriber for the reply message.
		subReply, err := natsConn.SubscribeSync(msg.Reply)
		if err != nil {
			log.Printf("error: nc.SubscribeSync failed: %v\n", err)
			continue
		}

		// Publish message
		err = natsConn.PublishMsg(msg)
		if err != nil {
			log.Printf("error: publish failed: %v\n", err)
			continue
		}

		// Wait up until 10 seconds for a reply,
		// continue and resend if to reply received.
		msgReply, err := subReply.NextMsg(time.Second * 10)
		if err != nil {
			log.Printf("error: subRepl.NextMsg failed for node=%v, subject=%v: %v\n", proc.node, proc.subject.name(), err)
			// did not receive a reply, continuing from top again
			continue
		}
		log.Printf("publisher: received ACK: %s\n", msgReply.Data)
		return
	}
}

// gobEncodePayload will encode the message structure along with its
// valued in gob binary format.
// TODO: Check if it adds value to compress with gzip.
func gobEncodePayload(m Message) ([]byte, error) {
	var buf bytes.Buffer
	gobEnc := gob.NewEncoder(&buf)
	err := gobEnc.Encode(m)
	if err != nil {
		return nil, fmt.Errorf("error: gob.Enode failed: %v", err)
	}

	return buf.Bytes(), nil
}
