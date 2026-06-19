package ctrl

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/fxamacker/cbor/v2"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/postmannen/actress"
	"golang.org/x/exp/slog"

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
		// s.s.processInitial, Message{}, er, logWarning)
		fmt.Printf("TOSEND: %v\n", er)
		return
	}

	for _, fp := range filePaths {
		slog.Info("readStartupFolder: ranging filepaths, current filePath contains", "filepath", fp)
	}

	for _, filePath := range filePaths {
		slog.Info("readStartupFolder: reading and working on file from startup folder ", "file", filePath)

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
			// s.s.processInitial, Message{}, err, logWarning)
			fmt.Printf("TOSEND: %v\n", err)
			continue
		}

		readBytes = bytes.Trim(readBytes, "\x00")

		// unmarshal the JSON into a struct
		messages, err := s.convertBytesToMessages(readBytes)
		if err != nil {
			er := fmt.Errorf("error: startup folder: malformed json read: %v", err)
			// s.s.processInitial, Message{}, er, logWarning)
			fmt.Printf("TOSEND: %v\n", er)
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
				// s.s.processInitial, Message{}, er, logWarning)
				fmt.Printf("TOSEND: %v\n", er)
				continue

			case messages[i].ToNode == "" && len(messages[i].ToNodes) == 0:
				er := fmt.Errorf(" error: missing value in both toNode and toNodes fields in startup message, discarding message")
				// s.s.processInitial, Message{}, er, logWarning)
				fmt.Printf("TOSEND: %v\n", er)
				continue
			}

		}

		er := fmt.Errorf("%v", messages)
		// s.s.processInitial, Message{}, er, logInfo)
		fmt.Printf("TOSEND: %v\n", er)

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
	// s.s.processInitial, Message{}, er, logInfo)
	fmt.Printf("TOSEND: %v\n", er)

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
		// s.s.processInitial, Message{}, er, logInfo)
		fmt.Printf("TOSEND: %v\n", er)

		msg.Ack()

		m, err := s.messageDeserializeAndUncompress(msg.Data())
		if err != nil {
			er := fmt.Errorf("jetstreamConsume: deserialize and uncompress failed: %v", err)
			// s.s.processInitial, Message{}, er, logError)
			fmt.Printf("TOSEND: %v\n", er)
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

const ETReadSocket actress.EventName = "ETReadSocket"

func etReadSocketFn(s *server) actress.ETFunc {
	fn := func(ctx context.Context, p *actress.Process) func() {
		fn := func() {
			// Open the ctrl socket file, and start the listener if enabled.
			var err error
			var ctrlSocket net.Listener

			if s.configuration.EnableSocket {
				ctrlSocket, err = createSocket(s.configuration.SocketFolder, "ctrl.sock")
				if err != nil {
					fmt.Printf("error: failed to create socket: %v\n", err)
					os.Exit(1)
				}
			}

			p.SignalReady()

			// TODO: REFACTOR: The listener (ctrlSocekt) should only be defined here, and not in
			//	 the server struct.

			go func() {
				for {
					fmt.Printf("DEBUG 1\n")
					conn, err := ctrlSocket.Accept()
					fmt.Printf("DEBUG 2\n")
					if err != nil {
						er := fmt.Errorf("error: failed to accept conn on socket: %v", err)
						// s.s.processInitial, Message{}, er, logError)
						fmt.Printf("TOSEND: %v\n", er)
						os.Exit(0)
					}

					go func(conn net.Conn) {
						defer conn.Close()

						var readBytes []byte

						for {
							b := make([]byte, 1500)
							_, err = conn.Read(b)
							if err != nil && err != io.EOF {
								er := fmt.Errorf("error: failed to read data from socket: %v", err)
								// s.s.processInitial, Message{}, er, logWarning)
								fmt.Printf("TOSEND: %v\n", er)
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
							// s.s.processInitial, Message{}, er, logWarning)
							fmt.Printf("TOSEND: %v\n", er)
							return
						}

						for i := range messages {

							// Fill in the value for the FromNode field, so the receiver
							// can check this field to know where it came from.
							messages[i].FromNode = Node(s.nodeName)

							// Send an info message to the central about the message picked
							// for auditing.
							er := fmt.Errorf("info: message read from socket on %v: %v", s.nodeName, messages[i])
							// s.s.processInitial, Message{}, er, logInfo)
							fmt.Printf("TOSEND: %v\n", er)

							// -------------------
							b, err := cbor.Marshal(messages[i])
							if err != nil {
								fmt.Printf("error: TestRequest: faield to cbor marshal: %v\n", err)
							}

							ev := actress.Event{
								Name:    ETNone,
								Data:    b,
								DstNode: "REMOTE",
							}

							s.root.AddEvent(ev)
							// -------------------

							// s.newMessagesCh <- messages[i]
						}

						// Send the SAM struct to be picked up by the ring buffer.

						s.auditLogCh <- messages

					}(conn)
				}
			}()

			<-ctx.Done()
			ctrlSocket.Close()

		}
		return fn
	}
	return fn
}

const ETReadFolder actress.EventName = "ETReadFolder"

func etReadFolderFn(s *server) actress.ETFunc {
	fn := func(ctx context.Context, p *actress.Process) func() {
		fn := func() {
			p.SignalReady()

			// Check if the startup folder exist.
			if _, err := os.Stat(s.configuration.ReadFolder); os.IsNotExist(err) {
				err := os.MkdirAll(s.configuration.ReadFolder, 0770)
				if err != nil {
					slog.Error("readfolder: failed to create readfolder", "error", err)
					os.Exit(1)
				}
			}

			watcher, err := fsnotify.NewWatcher()
			if err != nil {
				slog.Error("readfolder: failed to create new logWatcher", "error", err)
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
							slog.Debug("readFolder: got file event", "name", event.Name, "op", event.Op)

							func() {
								fh, err := os.Open(event.Name)
								if err != nil {
									er := fmt.Errorf("error: readFolder: failed to open readFile from readFolder: %v", err)
									// s.s.processInitial, Message{}, er, logDebug)
									fmt.Printf("TOSEND: %v\n", er)
									return
								}

								b, err := io.ReadAll(fh)
								if err != nil {
									er := fmt.Errorf("error: readFolder: failed to readall from readFolder: %v", err)
									// s.s.processInitial, Message{}, er, logWarning)
									fmt.Printf("TOSEND: %v\n", er)
									fh.Close()
									return
								}
								fh.Close()

								b = bytes.Trim(b, "\x00")

								// unmarshal the JSON into a struct
								messages, err := s.convertBytesToMessages(b)
								if err != nil {
									er := fmt.Errorf("error: readFolder: malformed json received: %s\n %v", b, err)
									// s.s.processInitial, Message{}, er, logWarning)
									fmt.Printf("TOSEND: %v\n", er)
									return
								}

								for i := range messages {

									// Fill in the value for the FromNode field, so the receiver
									// can check this field to know where it came from.
									messages[i].FromNode = Node(s.nodeName)

									// Send an info message to the central about the message picked
									// for auditing.
									er := fmt.Errorf("info: readFolder: message read from readFolder on %v: %v", s.nodeName, messages[i])
									// s.s.processInitial, Message{}, er, logWarning)
									fmt.Printf("TOSEND: %v\n", er)

									// Check if it is a message to publish with Jetstream.
									if messages[i].JetstreamToNode != "" {

										s.jetstreamPublishCh <- messages[i]
										slog.Debug("readFolder: read new JETSTREAM message in readfolder and putting it on s.jetstreamPublishCh", "messages", messages)

										continue
									}

									s.newMessagesCh <- messages[i]

									slog.Debug("readFolder: read new message in readfolder and putting it on s.samToSendCh", "messages", messages)
								}

								// Send the SAM struct to be picked up by the ring buffer.
								s.auditLogCh <- messages

								// Delete the file.
								err = os.Remove(event.Name)
								if err != nil {
									er := fmt.Errorf("error: readFolder: failed to remove readFile from readFolder: %v", err)
									// s.s.processInitial, Message{}, er, logWarning)
									fmt.Printf("TOSEND: %v\n", er)
									return
								}

							}()
						}

					case err, ok := <-watcher.Errors:
						if !ok {
							return
						}
						er := fmt.Errorf("error: readFolder: file watcher error: %v", err)
						// s.s.processInitial, Message{}, er, logWarning)
						fmt.Printf("TOSEND: %v\n", er)
					}
				}
			}()

			// Add a path.
			err = watcher.Add(s.configuration.ReadFolder)
			if err != nil {
				slog.Error("readFolder: start logs watcher: failed to add watcher", "error", err)
				os.Exit(1)
			}

			<-ctx.Done()

		}
		return fn
	}
	return fn
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
			// s.s.processInitial, v, er, logWarning)
			fmt.Printf("TOSEND: %v\n", er)
			continue
		}
	}

	return msgs
}
