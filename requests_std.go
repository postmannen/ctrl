package ctrl

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// -----

// Handler for receiving hello messages.
func methodHello(proc process, message Message, node string) ([]byte, error) {
	data := fmt.Sprintf("%v, Received hello from %#v\n", time.Now().Format("Mon Jan _2 15:04:05 2006"), message.FromNode)

	fileName := message.FileName
	folderTree := filepath.Join(proc.configuration.SubscribersDataFolder, message.Directory, string(message.FromNode))

	// Check if folder structure exist, if not create it.
	if _, err := os.Stat(folderTree); os.IsNotExist(err) {
		err := os.MkdirAll(folderTree, 0770)
		if err != nil {
			return nil, fmt.Errorf("error: failed to create errorLog directory tree %v: %v", folderTree, err)
		}

		proc.errorKernel.logDebug("methodHello: Creating subscribers data folder at ", "foldertree", folderTree)
	}

	// Open file and write data.
	file := filepath.Join(folderTree, fileName)
	//f, err := os.OpenFile(file, os.O_APPEND|os.O_RDWR|os.O_CREATE|os.O_SYNC, 0660)
	f, err := os.OpenFile(file, os.O_TRUNC|os.O_RDWR|os.O_CREATE|os.O_SYNC, 0660)

	if err != nil {
		er := fmt.Errorf("error: methodHello.handler: failed to open file: %v", err)
		return nil, er
	}
	defer f.Close()

	_, err = f.Write([]byte(data))
	f.Sync()
	if err != nil {
		er := fmt.Errorf("error: methodEventTextLogging.handler: failed to write to file: %v", err)
		proc.errorKernel.errSend(proc, message, er, logWarning)
	}

	// The handling of the public key that is in the message.Data field is handled in the procfunc.
	proc.procFuncCh <- message

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// procFuncHello is the procFunc used with the hello subscriber process.
// To keep the state of all the hello's received from nodes we need
// to also start a procFunc that will live as a go routine tied to this process,
// where the procFunc will receive messages from the handler when a message is
// received, the handler will deliver the message to the procFunc on the
// proc.procFuncCh, and we can then read that message from the procFuncCh in
// the procFunc running.
// The public key of a node are sent in the data field of the hello message,
// so we also handle the logic with registering keys from here.
func procFuncHelloSubscriber(ctx context.Context, proc process, procFuncCh chan Message) error {
	// sayHelloNodes := make(map[Node]struct{})

	for {
		// Receive a copy of the message sent from the method handler.
		var m Message

		select {
		case m = <-procFuncCh:
		case <-ctx.Done():
			proc.errorKernel.logDebug("procFuncHelloSubscriber: stopped handleFunc for: subscriber", "subject", proc.subject.name())

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

func procFuncHelloPublisher(ctx context.Context, proc process, procFuncCh chan Message) error {

	ticker := time.NewTicker(time.Second * time.Duration(proc.configuration.StartProcesses.StartPubHello))
	defer ticker.Stop()
	for {

		// d := fmt.Sprintf("Hello from %v\n", p.node)
		// Send the ed25519 public key used for signing as the payload of the message.
		d := proc.server.nodeAuth.SignPublicKey

		m := Message{
			FileName:   "hello.log",
			Directory:  "hello-messages",
			ToNode:     Node(proc.configuration.CentralNodeName),
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
			proc.errorKernel.logDebug("procFuncHelloPublisher: stopped handleFunc for: publisher", "subject", proc.subject.name())

			return nil
		}
	}
}

// ---

// Handle the writing of error logs.
func methodErrorLog(proc process, message Message, node string) ([]byte, error) {
	proc.metrics.promErrorMessagesReceivedTotal.Inc()

	// If it was a request type message we want to check what the initial messages
	// method, so we can use that in creating the file name to store the data.
	fileName, folderTree := selectFileNaming(message, proc)

	// Check if folder structure exist, if not create it.
	if _, err := os.Stat(folderTree); os.IsNotExist(err) {
		err := os.MkdirAll(folderTree, 0770)
		if err != nil {
			return nil, fmt.Errorf("error: failed to create errorLog directory tree %v: %v", folderTree, err)
		}

		proc.errorKernel.logDebug("methodErrorLog: Creating subscribers data folder", "foldertree", folderTree)
	}

	// Open file and write data.
	file := filepath.Join(folderTree, fileName)
	f, err := os.OpenFile(file, os.O_APPEND|os.O_RDWR|os.O_CREATE|os.O_SYNC, 0660)
	if err != nil {
		er := fmt.Errorf("error: methodErrorLog.handler: failed to open file: %v", err)
		return nil, er
	}
	defer f.Close()

	_, err = f.Write(message.Data)
	f.Sync()
	if err != nil {
		er := fmt.Errorf("error: methodEventTextLogging.handler: failed to write to file: %v", err)
		proc.errorKernel.errSend(proc, message, er, logWarning)
	}

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

// Handler to write directly to console.
// This handler handles the writing to console.
func methodConsole(proc process, message Message, node string) ([]byte, error) {

	switch {
	case len(message.MethodArgs) > 0 && message.MethodArgs[0] == "stderr":
		log.Printf("* DEBUG: MethodArgs: got stderr \n")
		fmt.Fprintf(os.Stderr, "%v", string(message.Data))
		fmt.Println()
	default:
		fmt.Fprintf(os.Stdout, "%v", string(message.Data))
		fmt.Println()
	}

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

// handler to be used as a reply method when testing requests.
// We can then within the test listen on the testCh for received
// data and validate it.
// If no test is listening the data will be dropped.
func methodTest(proc process, message Message, node string) ([]byte, error) {

	go func() {
		// Try to send the received message data on the test channel. If we
		// have a test started the data will be read from the testCh.
		// If no test is reading from the testCh the data will be dropped.
		select {
		case proc.errorKernel.testCh <- message.Data:
		default:
			// drop.
		}
	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
