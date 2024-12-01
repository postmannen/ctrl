package ctrl

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nats-io/nats.go/jetstream"

	"gopkg.in/yaml.v3"
)

// readStartupFolder will check the <workdir>/startup folder when ctrl
// starts for messages to process.
// The purpose of the startup folder is that we can define messages on a
// node that will be run when ctrl starts up.
// Messages defined in the startup folder should have the toNode set to
// self, and the from node set to where we want the answer sent. The reason
// for this is that all replies normally pick up the host from the original
// first message, but here we inject it on an end node so we need to specify
// the fromNode to get the reply back to the node we want.
//
// Messages read from the startup folder will be directly called by the handler
// locally, and the message will not be sent via the nats-server.
func (s *server) readStartupFolder() {

	// Get the names of all the files in the startup folder.
	const startupFolder = "startup"
	filePaths, err := s.getFilePaths(startupFolder)
	if err != nil {
		er := fmt.Errorf("error: readStartupFolder: unable to get filenames: %v", err)
		s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
		return
	}

	for _, fp := range filePaths {
		er := fmt.Errorf("info: ranging filepaths, current filePath contains: %v", fp)
		s.errorKernel.logInfo(er)
	}

	for _, filePath := range filePaths {
		er := fmt.Errorf("info: reading and working on file from startup folder %v", filePath)
		s.errorKernel.logInfo(er)

		// Read the content of each file.
		readBytes, err := func(filePath string) ([]byte, error) {
			fh, err := os.Open(filePath)
			if err != nil {
				er := fmt.Errorf("error: failed to open file in startup folder: %v", err)
				return nil, er
			}
			defer fh.Close()

			b, err := io.ReadAll(fh)
			if err != nil {
				er := fmt.Errorf("error: failed to read file in startup folder: %v", err)
				return nil, er
			}

			return b, nil
		}(filePath)

		if err != nil {
			s.errorKernel.errSend(s.processInitial, Message{}, err, logWarning)
			continue
		}

		readBytes = bytes.Trim(readBytes, "\x00")

		// unmarshal the JSON into a struct
		messages, err := s.convertBytesToMessages(readBytes)
		if err != nil {
			er := fmt.Errorf("error: startup folder: malformed json read: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
			continue
		}

		// Check if fromNode field is specified, and remove the message if blank.
		for i := range messages {
			// We want to allow the use of nodeName local only in startup folder, and
			// if used we substite it for the local node name.
			if messages[i].ToNode == "local" {
				messages[i].ToNode = Node(s.nodeName)
			}

			switch {
			case messages[i].FromNode == "":
				er := fmt.Errorf(" error: missing value in fromNode field in startup message, discarding message")
				s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
				continue

			case messages[i].ToNode == "" && len(messages[i].ToNodes) == 0:
				er := fmt.Errorf(" error: missing value in both toNode and toNodes fields in startup message, discarding message")
				s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
				continue
			}

		}

		j, err := json.MarshalIndent(messages, "", "   ")
		if err != nil {
			log.Printf("test error: %v\n", err)
		}
		er = fmt.Errorf("%v", string(j))
		s.errorKernel.errSend(s.processInitial, Message{}, er, logInfo)

		s.messageDeliverLocalCh <- messages

	}

}

func (s *server) jetstreamPublish() {
	// Create a JetStream management interface
	js, _ := jetstream.New(s.natsConn)

	// Create a stream
	_, _ = js.CreateStream(s.ctx, jetstream.StreamConfig{
		Name:              "NODES",
		Subjects:          []string{"NODES.>"},
		MaxMsgsPerSubject: int64(s.configuration.JetStreamMaxMsgsPerSubject),
	})

	// Publish messages.
	for {
		select {
		case msg := <-s.jetstreamPublishCh:

			b, err := s.messageSerializeAndCompress(msg)
			if err != nil {
				log.Fatalf("error: jetstreamPublish: marshal of message failed: %v\n", err)
			}

			subject := string(fmt.Sprintf("NODES.%v", msg.JetstreamToNode))
			_, err = js.Publish(s.ctx, subject, b)
			if err != nil {
				log.Fatalf("error: jetstreamPublish: publish failed: %v\n", err)
			}

			fmt.Printf("Published jetstream on subject: %q, message: %v\n", subject, msg)
		case <-s.ctx.Done():
		}
	}
}

func (s *server) jetstreamConsume() {
	// Create a JetStream management interface
	js, _ := jetstream.New(s.natsConn)

	// Create a stream
	stream, err := js.CreateOrUpdateStream(s.ctx, jetstream.StreamConfig{
		Name:     "NODES",
		Subjects: []string{"NODES.>"},
	})
	if err != nil {
		log.Printf("error: jetstreamConsume: failed to create stream: %v\n", err)
	}

	// The standard streams we want to consume.
	filterSubjectValues := []string{
		fmt.Sprintf("NODES.%v", s.nodeName),
		"NODES.all",
	}

	// Check if there are more to consume defined in flags/env.
	if s.configuration.JetstreamsConsume != "" {
		splitValues := strings.Split(s.configuration.JetstreamsConsume, ",")
		for _, v := range splitValues {
			filterSubjectValues = append(filterSubjectValues, fmt.Sprintf("NODES.%v", v))
		}
	}

	er := fmt.Errorf("jetstreamConsume: will consume the following subjects: %v", filterSubjectValues)
	s.errorKernel.errSend(s.processInitial, Message{}, er, logInfo)

	cons, err := stream.CreateOrUpdateConsumer(s.ctx, jetstream.ConsumerConfig{
		Name:           s.nodeName,
		Durable:        s.nodeName,
		FilterSubjects: filterSubjectValues,
	})
	if err != nil {
		log.Fatalf("error: jetstreamConsume: CreateOrUpdateConsumer failed: %v\n", err)
	}

	consumeContext, _ := cons.Consume(func(msg jetstream.Msg) {
		er := fmt.Errorf("jetstreamConsume: jetstream msg received: subject %q, data: %q", msg.Subject(), string(msg.Data()))
		s.errorKernel.errSend(s.processInitial, Message{}, er, logInfo)

		msg.Ack()

		m, err := s.messageDeserializeAndUncompress(msg.Data())
		if err != nil {
			er := fmt.Errorf("jetstreamConsume: deserialize and uncompress failed: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er, logError)
			return
		}

		// From here it is the normal message logic that applies, and since messages received
		// via jetstream are to be handled by the node it was consumed we set the current
		// nodeName of the consumer in the ctrl Message, so we are sure it is handled locally.
		m.ToNode = Node(s.nodeName)

		s.messageDeliverLocalCh <- []Message{m}
	})
	defer consumeContext.Stop()

	<-s.ctx.Done()

}

// getFilePaths will get the names of all the messages in
// the folder specified from current working directory.
func (s *server) getFilePaths(dirName string) ([]string, error) {
	dirPath, err := os.Executable()
	dirPath = filepath.Dir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("error: startup folder: unable to get the working directory %v: %v", dirPath, err)
	}

	dirPath = filepath.Join(dirPath, dirName)

	// Check if the startup folder exist.
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.MkdirAll(dirPath, 0770)
		if err != nil {
			er := fmt.Errorf("error: failed to create startup folder: %v", err)
			return nil, er
		}
	}

	fInfo, err := os.ReadDir(dirPath)
	if err != nil {
		er := fmt.Errorf("error: failed to get filenames in startup folder: %v", err)
		return nil, er
	}

	filePaths := []string{}

	for _, v := range fInfo {
		realpath := filepath.Join(dirPath, v.Name())
		filePaths = append(filePaths, realpath)
	}

	return filePaths, nil
}

// readSocket will read the .sock file specified.
// It will take a channel of []byte as input, and it is in this
// channel the content of a file that has changed is returned.
func (s *server) readSocket() {
	// Loop, and wait for new connections.
	for {
		conn, err := s.ctrlSocket.Accept()
		if err != nil {
			er := fmt.Errorf("error: failed to accept conn on socket: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er, logError)
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var readBytes []byte

			for {
				b := make([]byte, 1500)
				_, err = conn.Read(b)
				if err != nil && err != io.EOF {
					er := fmt.Errorf("error: failed to read data from socket: %v", err)
					s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
					return
				}

				readBytes = append(readBytes, b...)

				if err == io.EOF {
					break
				}
			}

			readBytes = bytes.Trim(readBytes, "\x00")

			// unmarshal the JSON into a struct
			messages, err := s.convertBytesToMessages(readBytes)
			if err != nil {
				er := fmt.Errorf("error: malformed json received on socket: %s\n %v", readBytes, err)
				s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
				return
			}

			for i := range messages {

				// Fill in the value for the FromNode field, so the receiver
				// can check this field to know where it came from.
				messages[i].FromNode = Node(s.nodeName)

				// Send an info message to the central about the message picked
				// for auditing.
				er := fmt.Errorf("info: message read from socket on %v: %v", s.nodeName, messages[i])
				s.errorKernel.errSend(s.processInitial, Message{}, er, logInfo)

				s.newMessagesCh <- messages[i]
			}

			// Send the SAM struct to be picked up by the ring buffer.

			s.auditLogCh <- messages

		}(conn)
	}
}

// readFolder
func (s *server) readFolder() {
	// Check if the startup folder exist.
	if _, err := os.Stat(s.configuration.ReadFolder); os.IsNotExist(err) {
		err := os.MkdirAll(s.configuration.ReadFolder, 0770)
		if err != nil {
			er := fmt.Errorf("error: failed to create readfolder folder: %v", err)
			s.errorKernel.logError(er)
			os.Exit(1)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		er := fmt.Errorf("main: failed to create new logWatcher: %v", err)
		s.errorKernel.logError(er)
		os.Exit(1)
	}

	// Start listening for events.
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}

				if event.Op == fsnotify.Create || event.Op == fsnotify.Write {
					time.Sleep(time.Millisecond * 250)
					er := fmt.Errorf("readFolder: got file event, name: %v, op: %v", event.Name, event.Op)
					s.errorKernel.logDebug(er)

					func() {
						fh, err := os.Open(event.Name)
						if err != nil {
							er := fmt.Errorf("error: readFolder: failed to open readFile from readFolder: %v", err)
							s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
							return
						}

						b, err := io.ReadAll(fh)
						if err != nil {
							er := fmt.Errorf("error: readFolder: failed to readall from readFolder: %v", err)
							s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
							fh.Close()
							return
						}
						fh.Close()

						fmt.Printf("------- DEBUG: %v\n", b)

						b = bytes.Trim(b, "\x00")

						// unmarshal the JSON into a struct
						messages, err := s.convertBytesToMessages(b)
						if err != nil {
							er := fmt.Errorf("error: readFolder: malformed json received: %s\n %v", b, err)
							s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
							return
						}

						for i := range messages {

							// Fill in the value for the FromNode field, so the receiver
							// can check this field to know where it came from.
							messages[i].FromNode = Node(s.nodeName)

							// Send an info message to the central about the message picked
							// for auditing.
							er := fmt.Errorf("info: readFolder: message read from readFolder on %v: %v", s.nodeName, messages[i])
							s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)

							// Check if it is a message to publish with Jetstream.
							if messages[i].JetstreamToNode != "" {

								s.jetstreamPublishCh <- messages[i]
								er = fmt.Errorf("readFolder: read new JETSTREAM message in readfolder and putting it on s.jetstreamPublishCh: %#v", messages)
								s.errorKernel.logDebug(er)

								continue
							}

							s.newMessagesCh <- messages[i]

							er = fmt.Errorf("readFolder: read new message in readfolder and putting it on s.samToSendCh: %#v", messages)
							s.errorKernel.logDebug(er)
						}

						// Send the SAM struct to be picked up by the ring buffer.
						s.auditLogCh <- messages

						// Delete the file.
						err = os.Remove(event.Name)
						if err != nil {
							er := fmt.Errorf("error: readFolder: failed to remove readFile from readFolder: %v", err)
							s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
							return
						}

					}()
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				er := fmt.Errorf("error: readFolder: file watcher error: %v", err)
				s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
			}
		}
	}()

	// Add a path.
	err = watcher.Add(s.configuration.ReadFolder)
	if err != nil {
		er := fmt.Errorf("startLogsWatcher: failed to add watcher: %v", err)
		s.errorKernel.logError(er)
		os.Exit(1)
	}
}

// readTCPListener wait and read messages delivered on the TCP
// port if started.
// It will take a channel of []byte as input, and it is in this
// channel the content of a file that has changed is returned.
func (s *server) readTCPListener() {
	ln, err := net.Listen("tcp", s.configuration.TCPListener)
	if err != nil {
		er := fmt.Errorf("error: readTCPListener: failed to start tcp listener: %v", err)
		s.errorKernel.logError(er)
		os.Exit(1)
	}
	// Loop, and wait for new connections.
	for {

		conn, err := ln.Accept()
		if err != nil {
			er := fmt.Errorf("error: failed to accept conn on socket: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er, logError)
			continue
		}

		go func(conn net.Conn) {
			defer conn.Close()

			var readBytes []byte

			for {
				b := make([]byte, 1500)
				_, err = conn.Read(b)
				if err != nil && err != io.EOF {
					er := fmt.Errorf("error: failed to read data from tcp listener: %v", err)
					s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
					return
				}

				readBytes = append(readBytes, b...)

				if err == io.EOF {
					break
				}
			}

			readBytes = bytes.Trim(readBytes, "\x00")

			// unmarshal the JSON into a struct
			messages, err := s.convertBytesToMessages(readBytes)
			if err != nil {
				er := fmt.Errorf("error: malformed json received on tcp listener: %v", err)
				s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
				return
			}

			for i := range messages {

				// Fill in the value for the FromNode field, so the receiver
				// can check this field to know where it came from.
				messages[i].FromNode = Node(s.nodeName)
				s.newMessagesCh <- messages[i]
			}

			// Send the SAM struct to be picked up by the ring buffer.
			s.auditLogCh <- messages

		}(conn)
	}
}

func (s *server) readHTTPlistenerHandler(w http.ResponseWriter, r *http.Request) {

	var readBytes []byte

	for {
		b := make([]byte, 1500)
		_, err := r.Body.Read(b)
		if err != nil && err != io.EOF {
			er := fmt.Errorf("error: failed to read data from tcp listener: %v", err)
			s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
			return
		}

		readBytes = append(readBytes, b...)

		if err == io.EOF {
			break
		}
	}

	readBytes = bytes.Trim(readBytes, "\x00")

	// unmarshal the JSON into a struct
	messages, err := s.convertBytesToMessages(readBytes)
	if err != nil {
		er := fmt.Errorf("error: malformed json received on HTTPListener: %v", err)
		s.errorKernel.errSend(s.processInitial, Message{}, er, logWarning)
		return
	}

	for i := range messages {

		// Fill in the value for the FromNode field, so the receiver
		// can check this field to know where it came from.
		messages[i].FromNode = Node(s.nodeName)
		s.newMessagesCh <- messages[i]
	}

	// Send the SAM struct to be picked up by the ring buffer.
	s.auditLogCh <- messages

}

func (s *server) readHttpListener() {
	go func() {
		n, err := net.Listen("tcp", s.configuration.HTTPListener)
		if err != nil {
			er := fmt.Errorf("error: startMetrics: failed to open prometheus listen port: %v", err)
			s.errorKernel.logError(er)
			os.Exit(1)
		}
		mux := http.NewServeMux()
		mux.HandleFunc("/", s.readHTTPlistenerHandler)

		err = http.Serve(n, mux)
		if err != nil {
			er := fmt.Errorf("error: startMetrics: failed to start http.Serve: %v", err)
			s.errorKernel.logError(er)
			os.Exit(1)
		}
	}()
}

// convertBytesToSAMs will range over the  byte representing a message given in
// json format. For each element found the Message type will be converted into
// a SubjectAndMessage type value and appended to a slice, and the slice is
// returned to the caller.
func (s *server) convertBytesToMessages(b []byte) ([]Message, error) {
	MsgSlice := []Message{}

	err := yaml.Unmarshal(b, &MsgSlice)
	if err != nil {
		return nil, fmt.Errorf("error: unmarshal of file failed: %#v", err)
	}

	// Check for toNode and toNodes field.
	MsgSlice = s.checkMessageToNodes(MsgSlice)
	s.metrics.promUserMessagesTotal.Add(float64(len(MsgSlice)))

	return MsgSlice, nil
}

// checkMessageToNodes will check that either toHost or toHosts are
// specified in the message. If not specified it will drop the message
// and send an error.
// if toNodes is specified, the original message will be used, and
// and an individual message will be created with a toNode field for
// each if the toNodes specified.
func (s *server) checkMessageToNodes(MsgSlice []Message) []Message {
	msgs := []Message{}

	for _, v := range MsgSlice {
		switch {
		// if toNode specified, we don't care about the toHosts.
		case v.ToNode != "":
			msgs = append(msgs, v)
			continue

		// if toNodes specified, we use the original message, and
		// create new node messages for each of the nodes specified.
		case len(v.ToNodes) != 0:
			for _, n := range v.ToNodes {
				m := v
				// Set the toNodes field to nil since we're creating
				// an individual toNode message for each of the toNodes
				// found, and hence we no longer need that field.
				m.ToNodes = nil
				m.ToNode = n
				msgs = append(msgs, m)
			}
			continue

		// No toNode or toNodes specified. Drop the message by not appending it to
		// the slice since it is not valid.
		default:
			er := fmt.Errorf("error: no toNode or toNodes where specified in the message, dropping message: %v", v)
			s.errorKernel.errSend(s.processInitial, v, er, logWarning)
			continue
		}
	}

	return msgs
}
