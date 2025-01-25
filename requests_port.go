package ctrl

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

// portInitialData is the data that is sent to the source and destination nodes when a port forwarding is created.
type portInitialData struct {
	UUID                 string
	SourceNode           Node
	SourceSubMethod      Method
	SourceIPAndPort      string
	DestinationNode      Node
	DestinationSubMethod Method
	DestinationIPAndPort string
	MaxSessionTimeout    int
}

// NewPortInitialData creates a new portInitialData struct.
func NewPortInitialData(uuid string, sourceNode Node, sourceSubMethod Method, sourcePort string, destinationNode Node, destinationSubMethod Method, destinationPort string, maxSessionTimeout int) portInitialData {
	pid := portInitialData{
		UUID:                 uuid,
		SourceNode:           sourceNode,
		SourceSubMethod:      sourceSubMethod,
		SourceIPAndPort:      sourcePort,
		DestinationNode:      destinationNode,
		DestinationSubMethod: destinationSubMethod,
		DestinationIPAndPort: destinationPort,
		MaxSessionTimeout:    maxSessionTimeout,
	}

	return pid
}

// The main method that will handle the initial message, and setup whats needed for an outbound connection.
// It will setup a sub process that will handle the individual session of the port forwarding on the
// source node, and also setup the local tcp listener on the source node that a client can connect to.
//
// NB: All logic for the forwarding are done in the subprocesses started.
func methodPortSrc(proc process, message Message, node string) ([]byte, error) {

	go func() {
		defer proc.processes.wg.Done()

		// Message example to start an outbound connection
		// ---
		// - toNode:
		//     - node1
		//   method: portSrc
		//   methodArgs:
		//     - node1			# destination node
		//     - localhost:8080	# destination node and port
		//	   - localhost:8090	# source node and port
		//     - 100 			# max session timeout
		//   replymethod: console

		const (
			arg0_DestinationNode      = 0
			arg1_DestinationIPAndPort = 1
			arg2_SourceIPandPort      = 2
			arg3_MaxSessionTimeout    = 3
		)

		wantMethodArgs := "want: (0)destination-node, (1)destination-ip-and-port, (2) source-ip-and-port, (3)max-session-timeout"

		uuid := uuid.New().String()
		sourceNode := proc.node

		proc.processes.wg.Add(1)
		if len(message.MethodArgs) < arg3_MaxSessionTimeout {
			slog.Error("methodPortSrc: to few methodArgs defined in message", "want", wantMethodArgs)
			return
		}

		// Destination node
		if message.MethodArgs[arg0_DestinationNode] == "" {
			slog.Error("methodPortSrc: no destination node specified in method args", "want", wantMethodArgs)
			return
		}
		destinationNode := Node(message.MethodArgs[arg0_DestinationNode])

		// Destination port
		if message.MethodArgs[arg1_DestinationIPAndPort] == "" {
			slog.Error("methodPortSrc: no destination port specified in method args", "want", wantMethodArgs)
			return
		}
		destinationIPAndPort := message.MethodArgs[arg1_DestinationIPAndPort]

		// Source port
		if message.MethodArgs[arg2_SourceIPandPort] == "" {
			slog.Error("methodPortSrc: no source port specified in method args", "want", wantMethodArgs)
			return
		}
		sourceIPAndPort := message.MethodArgs[arg2_SourceIPandPort]

		// Max session timeout
		if message.MethodArgs[arg3_MaxSessionTimeout] == "" {
			slog.Error("methodPortSrc: no max session time specified in method args", "want", wantMethodArgs)
			return
		}
		n, err := strconv.Atoi(message.MethodArgs[arg3_MaxSessionTimeout])
		if err != nil {
			slog.Error("methodPortSrc: unable to convert max session timeout from string to int", "error", err)
			return
		}
		maxSessionTimeout := n

		// Prepare the naming for the dst method here so we can have all the
		// information in the pid from the beginning at both ends and not have
		// to generate naming on the destination node.
		dstSubProcessName := fmt.Sprintf("%v.%v", SUBPortDst, uuid)
		destinationSubMethod := Method(dstSubProcessName)

		// ------- Create the source sub process -------

		// Create a child context to use with the procFunc with timeout set to the max allowed total copy time
		// specified in the message.
		var ctx context.Context
		var cancel context.CancelFunc
		func() {
			ctx, cancel = context.WithTimeout(proc.ctx, time.Second*time.Duration(maxSessionTimeout))
		}()

		srcSubProcessName := fmt.Sprintf("%v.%v", SUBPortSrc, uuid)
		sourceSubMethod := Method(srcSubProcessName)

		// Create a new subprocess that will handle the network source side.
		subject := newSubjectNoVerifyHandler(sourceSubMethod, node)
		portSrcSubProc := newSubProcess(ctx, proc.server, subject)

		// Create a new portInitialData with all the information we need for starting the sub processes.
		// NB: Using the destination port as the source port on the source process for now.
		pid := NewPortInitialData(uuid, sourceNode, sourceSubMethod, sourceIPAndPort, destinationNode, destinationSubMethod, destinationIPAndPort, maxSessionTimeout)

		// Attach a procfunc to the sub process that will do the actual logic with the network source port.
		portSrcSubProc.procFunc = portSrcSubProcFunc(pid, message, cancel)

		// Assign a handler to the sub process for receiving messages for the subprocess.
		portSrcSubProc.handler = portSrcSubHandler()

		// Start sub process. The process will be killed when the context expires.
		go portSrcSubProc.start()

		fmt.Printf("DEBUG: methodPortSrc, pid: %+v\n", pid)

		// ------- Prepare the data payload to send to the dst to start the dst sub process -------
		cb, err := cbor.Marshal(pid)
		if err != nil {
			er := fmt.Errorf("error: methodPortSrc: cbor marshalling failed: %v, message: %v", err, message)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			cancel()
		}

		// Send a message to the the destination node, to also start up a sub
		// process there.

		msg := Message{
			ToNode:          pid.DestinationNode,
			Method:          PortDst,
			Data:            cb,
			ACKTimeout:      message.ACKTimeout,
			Retries:         message.Retries,
			ReplyMethod:     Console, // TODO: Adding for debug output
			ReplyACKTimeout: message.ReplyACKTimeout,
			ReplyRetries:    message.ReplyRetries,
		}

		proc.newMessagesCh <- msg

		replyData := fmt.Sprintf("info: succesfully started port source process: procName=%v, srcNode=%v, sourcePort=%v, dstNode=%v, starting sub process=%v", portSrcSubProc.processName, node, pid.SourceIPAndPort, pid.DestinationNode, srcSubProcessName)

		newReplyMessage(proc, message, []byte(replyData))

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// The main method that will handle the initial message sendt from the source process, and setup
// sub process that will handle the individual session of the port forwarding, and also
// setup the connection the final tcp endpoint on the destination node.
//
// NB: All logic for the forwarding are done in the subprocesses started.
func methodPortDst(proc process, message Message, node string) ([]byte, error) {
	var subProcessName string

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get the status message sent from source.
		var pid portInitialData
		err := cbor.Unmarshal(message.Data, &pid)
		if err != nil {
			er := fmt.Errorf("error: methodPortDst: failed to cbor Unmarshal data: %v, message=%v", err, message)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		fmt.Printf(" *** DEBUG: ON DST: got pid: %+v\n", pid)

		// Create a child context to use with the procFunc
		var ctx context.Context
		var cancel context.CancelFunc
		func() {
			ctx, cancel = context.WithTimeout(proc.ctx, time.Second*time.Duration(pid.MaxSessionTimeout))
		}()

		// Start preparing to start a sub process.
		sub := newSubjectNoVerifyHandler(pid.DestinationSubMethod, node)

		// But first...
		// Check if we already got a sub process registered and started with
		// the processName. If true, return here and don't start up another
		// process.
		//
		// This check is put in here if a message for some reason are
		// received more than once. The reason that this might happen can be
		// that a message for the same copy request was received earlier, but
		// was unable to start up within the timeout specified. The Sender of
		// that request will then resend the message, but at the time that
		// second message is received the subscriber process started for the
		// previous message is then fully up and running, so we just discard
		// that second message in those cases.

		pn := processNameGet(sub.name())
		// fmt.Printf("\n\n *** DEBUG: processNameGet: %v\n\n", pn)

		proc.processes.active.mu.Lock()
		_, ok := proc.processes.active.procNames[pn]
		proc.processes.active.mu.Unlock()

		if ok {
			proc.errorKernel.logDebug("methodCopyDst: subprocesses already existed, will not start another subscriber for", "processName", pn)

			// If the process name already existed we return here before any
			// new information is registered in the process map and we avoid
			// having to clean that up later.
			return
		}

		// Create a new sub process.
		portDstSubProc := newSubProcess(ctx, proc.server, sub)

		// Give the sub process a procFunc that will do the actual networking within a procFunc,
		portDstSubProc.procFunc = portDstSubProcFunc(pid, message, cancel)

		// assign a handler to the sub process
		portDstSubProc.handler = portDstSubHandler()

		// The process will be killed when the context expires.
		go portDstSubProc.start()

		replyData := fmt.Sprintf("info: succesfully initiated port destination process: procName=%v, srcNode=%v, starting sub process=%v for the actual copying", portDstSubProc.processName, node, subProcessName)

		newReplyMessage(proc, message, []byte(replyData))

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// portSrcSubHandler is the handler for messages received and destined for the port source sub process.
// It will pass the message to the procFunc of the port source sub process, which will do the actual
// handling of the messages.
func portSrcSubHandler() func(process, Message, string) ([]byte, error) {
	h := func(proc process, message Message, node string) ([]byte, error) {

		select {
		case <-proc.ctx.Done():
			proc.errorKernel.logDebug("copySrcHandler: process ended", "processName", proc.processName)
		case proc.procFuncCh <- message:
			proc.errorKernel.logDebug("copySrcHandler: passing message over to procFunc", "processName", proc.processName)
		}

		return nil, nil
	}

	return h
}

// portDstSubHandler is the handler for messages received and destined for the port destination sub process.
// It will pass the message to the procFunc of the port destination sub process, which will do the actual
// handling of the messages.
func portDstSubHandler() func(process, Message, string) ([]byte, error) {
	h := func(proc process, message Message, node string) ([]byte, error) {

		select {
		case <-proc.ctx.Done():
			proc.errorKernel.logDebug("copyDstHandler: process ended", "processName", proc.processName)
		case proc.procFuncCh <- message:
			proc.errorKernel.logDebug("copyDstHandler: passing message over to procFunc", "processName", proc.processName)

		}

		return nil, nil
	}

	return h
}

type portData struct {
	OK       bool
	ErrorMsg string
	Data     []byte
	ID       int
}

// portSrcSubProcFunc is the function that will be run by the portSrcSubProc process.
// It will listen on the source IP and port, and send messages to the destination node
// to write the data to the destination IP and port.
// It will also handle the incomming messages from the destination node and write the
// data in the message to the source IP and port.
func portSrcSubProcFunc(pid portInitialData, initialMessage Message, cancel context.CancelFunc) func(context.Context, process, chan Message) error {
	pf := func(ctx context.Context, proc process, procFuncCh chan Message) error {
		fmt.Printf("STARTED PROCFUNC: %v\n", proc.subject.name())
		defer cancel()
		defer fmt.Println("portSrcProcFunc: canceled procFunc", "processName", proc.processName)

		listener, err := net.Listen("tcp", pid.SourceIPAndPort)
		if err != nil {
			// TODO: Send a message to destination sub process that there was an error,
			// and that it should stop.
			fmt.Printf("error: portSrcSubProcFunc: net.Listen failed. err=%v\n", err)
			return err
		}

		// Start a goroutine to handle the tcp listener.
		go func() {
			for {
				conn, err := listener.Accept()
				if err != nil {
					// TODO: Send a message to destination sub process that there was an error,
					// and that it should stop.
					fmt.Printf("error: portSrcSubProcFunc: listener.Accept failed. err=%v\n", err)
					return
				}

				defer func() {
					conn.Close()
					listener.Close()
					fmt.Println(" DEBUG: Canceled portSrcSubProcFunc connection and listener")
				}()

				// Read the data from the tcp connection, create messages from it, and
				// send it to the dst node.
				go func() {
					id := 0
					for {
						b := make([]byte, 65535)
						n, err := conn.Read(b)
						if err != nil {
							log.Printf("error: portSrcSubProcFunc: conn.Read failed. err=%v", err)
							return
						}

						pd := portData{
							ID:   id,
							Data: b[:n],
						}
						id++

						cb, err := cbor.Marshal(pd)
						if err != nil {
							log.Fatalf("error: portDstSubProcFunc: cbor marshalling failed. err=%v", err)
						}

						msg := Message{
							ToNode:      pid.DestinationNode,
							Method:      Method(pid.DestinationSubMethod),
							Data:        cb,
							ACKTimeout:  initialMessage.ACKTimeout,
							Retries:     initialMessage.Retries,
							ReplyMethod: None,
						}

						fmt.Printf(" ******* SRC: Created message to send with id : %v\n", pd.ID)

						select {
						case <-ctx.Done():
							fmt.Println("portSrcProcFunc: canceling procFunc", "processName", proc.processName)
							return
						case proc.newMessagesCh <- msg:
							fmt.Printf(" ---->: Sending message, id: %v, size: %v\n", pd.ID, len(pd.Data))
						}
					}
				}()

				// -----------------------------------------------------------------------------------
				// Read from messages from dst node and write to the source network connection.
				// -----------------------------------------------------------------------------------

				expectedID := 0
				buffer := NewPortSortedBuffer()
				for {
					select {
					case <-ctx.Done():
						fmt.Println("portSrcProcFunc: canceling procFunc", "processName", proc.processName)
						return

					// Handle the messages reveived from the sub process on the src node.
					// The messages will contain the data to be sent to the dst node.
					case message := <-procFuncCh:

						var pd portData
						err := cbor.Unmarshal(message.Data, &pd)
						if err != nil {
							log.Fatalf("error: portSrcSubProcFunc: cbor unmarshalling failed. err=%v", err)
						}

						fmt.Printf("<---- GOT MESSAGE ON SRC, pdd.OK:%v, id: %v, length of pddData ::: %v\n", pd.OK, pd.ID, len(pd.Data))

						buffer.Push(pd)

						err = func() error {

							// Write the data to the channel that the listener is reading from.
							for {
								nextID, _ := buffer.PeekNextID()
								if expectedID != nextID {
									log.Printf("WRONG ID, WILL WAIT FOR NEXT MESSAGE, expectedID %v < nextID: %v\n", expectedID, pd.ID)
									return nil
								}

								log.Printf("------------CORRECT ID, EXPECTED: %v, GOT: %v\n", expectedID, pd.ID)

								pdPopped, ok := buffer.Pop()
								if !ok {
									fmt.Println("src: Buffer is empty, break out, and wait for next message.")
									return nil
								}
								fmt.Printf("popped, id: %v, size: %v\n", pdPopped.ID, len(pdPopped.Data))

								n, err := conn.Write(pdPopped.Data)
								if err != nil {
									log.Printf("error: portSrcSubProcFunc: conn.Write failed. err=%v", err)
									return err
								}
								fmt.Printf("--------> conn: portSrcSubProcFunc: wrote %v bytes with ID=%v to connection, exptedID was %v\n", n, pdPopped.ID, expectedID)

								expectedID++

								if !pdPopped.OK {
									log.Printf("error: portSrcSubProcFunc: pdd.OK is false. err=%v\n", pdPopped.ErrorMsg)
									return fmt.Errorf("%v", pdPopped.ErrorMsg)
								}
							}

						}()

						if err != nil {
							return
						}
					}
				}

			}
		}()

		<-ctx.Done()
		fmt.Println("------------------------------------------------EXITING portSrcSubProcFunc---------------------------------------------------------------")
		return nil
	}

	return pf
}

// portDstSubProcFunc is the function that will be run by the portDstSubProc process.
// It will connect to the destination IP and port, and send messages to the source node
// to write the data to the source IP and port.
// It will also handle the incomming messages from the source node and write the
// data in the message to the destination IP and port.
func portDstSubProcFunc(pid portInitialData, message Message, cancel context.CancelFunc) func(context.Context, process, chan Message) error {

	pf := func(ctx context.Context, proc process, procFuncCh chan Message) error {
		defer cancel()

		fmt.Printf("STARTED PROCFUNC: %v\n", proc.subject.name())
		defer fmt.Println("portDstProcFunc: canceled procFunc", "processName", proc.processName)

		// TODO: Start the tcp connection for the dst node here.
		// ------------
		conn, err := net.Dial("tcp", pid.DestinationIPAndPort)
		if err != nil {
			log.Fatalf("error: portDstSubProcFunc: dial failed. err=%v", err)
		}
		defer conn.Close()

		// Read from destination network connection and send messages to src node of whats read.
		go func() {
			id := 0

			for {
				var errorMsg string
				ok := true

				b := make([]byte, 65535)

				n, err := conn.Read(b)
				if err != nil {
					// If there was an error while reading, set the value of errorMsg and ok accordingly.
					// This information will then be used on the src node to stop sub processes etc.
					switch {
					case err == io.EOF:
						ok = false
						errorMsg = fmt.Sprintf("portDstSubProcFunc: conn.Read() returned EOF, n: %v", n)
						log.Printf("AT DST: %v\n", errorMsg)
					case strings.Contains(err.Error(), "use of closed network connection"):
						ok = false
						fmt.Printf("AT DST: portDstSubProcFunc: conn.Read(): closed network connection, err: %v, n: %v\n", err, n)
					default:
						ok = false
						fmt.Printf("AT DST: portDstSubProcFunc: conn.Read(): other error, err: %v, n: %v\n", err, n)
					}

				}

				fmt.Printf("portDstSubProcFunc: read from network conn: length: %v\n", n)

				pdd := portData{
					OK:       ok,
					ErrorMsg: errorMsg,
					Data:     b[:n],
					ID:       id,
				}
				id++

				cb, err := cbor.Marshal(pdd)
				if err != nil {
					log.Fatalf("error: portDstSubProcFunc: cbor marshalling failed. err=%v", err)
				}

				msg := Message{
					ToNode:      pid.SourceNode,
					Method:      Method(pid.SourceSubMethod),
					Data:        cb,
					ACKTimeout:  message.ACKTimeout,
					Retries:     message.Retries,
					ReplyMethod: None,
				}

				proc.newMessagesCh <- msg
				fmt.Printf(" ******* DST: Created message to send with id : %v\n", id)

				// If there was en error while reading, we exit the loop, so the connection is closed.
				if !ok {
					// TODO: Check out the cancelation!!!
					//cancel()
					return
				}

			}

		}()

		// -----------------------------------------------------------------------------------
		// Read from messages from src node and write to the destination network connection.
		// -----------------------------------------------------------------------------------

		expectedID := -1
		buffer := NewPortSortedBuffer()

		for {
			select {
			case <-ctx.Done():
				fmt.Println("portDstProcFunc: got <-ctx.Done() cancelling procFunc", "processName", proc.processName)
				return nil

			// Pick up the message recived by the copySrcSubHandler.
			case message := <-procFuncCh:
				expectedID++

				var pd portData
				err := cbor.Unmarshal(message.Data, &pd)
				if err != nil {
					log.Fatalf("error: portDstSubProcFunc: cbor unmarshalling failed. err=%v", err)
				}

				fmt.Printf("<---- GOT DATA ON DST, id: %v, length: %v\n", pd.ID, len(pd.Data))

				buffer.Push(pd)

				err = func() error {
					nextID, _ := buffer.PeekNextID()
					// Messages might come out of order.
					// If the expected id is larger  than the next id, we've lost packages, and end the process.
					// If the expected id is less than the next id, we return and wait for the next message.
					//    if expectedID > nextID {
					//    	err := fmt.Errorf("portDstSubProcFunc: error: expectedID %v > nextID %v, we've probably lost packages", expectedID, nextID)
					//    	log.Println(err)
					//
					//    	log.Printf(" ---------- DEBUG portDstSubProcFunc: buffer now contains: %+v\n", buffer.buffer)
					//
					//    	return err
					//    }
					if expectedID < nextID {
						log.Printf("------------WRONG ID, WILL WAIT FOR NEXT MESSAGE, expectedID %v < nextID: %v\n", expectedID, pd.ID)
						return nil
					}

					log.Printf("------------CORRECT ID, EXPECTED: %v, GOT: %v\n", expectedID, pd.ID)

					// Loop over eventual messages in the buffer and write them to the connection.
					for {
						pdPopped, ok := buffer.Pop()

						if !ok {
							// Buffer is empty, break out and wait for next message.
							break
						}

						n, err := conn.Write(pdPopped.Data)
						if err != nil {
							err := fmt.Errorf("error: portDstSubProcFunc: conn.Write failed. err=%v", err)
							log.Println(err)
							return err
						}
						fmt.Printf("--------> conn: portDstSubProcFunc: wrote %v bytes to connection\n", n)
					}

					return nil
				}()

				// Check if there was an error in the above function.
				if err != nil {
					return err
				}
			}
		}

	}

	return pf
}

// -----------------------------------------------------------

// portSortedBuffer is a thread-safe buffer that sorts the data by ID.
type portSortedBuffer struct {
	buffer []portData
	mu     sync.Mutex
}

// NewPortSortedBuffer creates a new portSortedBuffer.
func NewPortSortedBuffer() *portSortedBuffer {
	b := portSortedBuffer{}
	return &b
}

// Push adds a new portData to the buffer and sorts it by ID.
func (b *portSortedBuffer) Push(value portData) {
	b.buffer = append(b.buffer, value)

	b.mu.Lock()
	defer b.mu.Unlock()
	sort.SliceStable(b.buffer, func(i, j int) bool {
		return b.buffer[i].ID < b.buffer[j].ID
	})
}

// Pop removes and returns the first portData from the buffer.
func (b *portSortedBuffer) Pop() (portData, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffer) == 0 {
		return portData{}, false
	}
	value := b.buffer[0]
	b.buffer = b.buffer[1:]
	return value, true
}

// PeekNextID returns the ID of the next portData in the buffer without removing it.
func (b *portSortedBuffer) PeekNextID() (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.buffer) == 0 {
		return -1, false
	}
	id := b.buffer[0].ID

	return id, true
}
