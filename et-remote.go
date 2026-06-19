package ctrl

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/fxamacker/cbor/v2"
	"github.com/postmannen/actress"
	"golang.org/x/exp/slog"
)

func etRemoteFn(s *server) actress.ETFunc {
	fn := func(ctx context.Context, p *actress.Process) func() {
		fn := func() {

			var method Method
			methodsAvailable := method.GetMethodsAvailable()

			p.SignalReady()

			for {
				select {
				case remoteEv := <-p.InCh:
					ev := remoteEv.NextEvent

					fmt.Printf("GOT REMOTE EVENT: %v\n", ev.Name)

					msg := Message{}
					err := cbor.Unmarshal(ev.Data, &msg)
					if err != nil {
						fmt.Printf("error: failed to cbor unmarshal event into message: %v\n", err)
						os.Exit(1)
					}

					// ------
					go func(message Message) {

						s.messageID.mu.Lock()
						s.messageID.id++
						message.ID = s.messageID.id
						s.messageID.mu.Unlock()

						s.metrics.promMessagesProcessedIDLast.Set(float64(message.ID))

						// Check if the format of the message is correct.
						if _, ok := methodsAvailable.CheckIfExists(message.Method); !ok {
							er := fmt.Errorf("error: routeMessagesToProcess: the method do not exist, message dropped: %v", message.Method)
							// s.s.processInitial, message, er, logError)
							fmt.Printf("%v\n", er)
							return
						}

						switch {
						case message.Retries < 0:
							message.Retries = s.configuration.DefaultMessageRetries
						}
						if message.MethodTimeout < 1 && message.MethodTimeout != -1 {
							message.MethodTimeout = s.configuration.DefaultMethodTimeout
						}

						// ---
						// Check for {{CTRL_FILE}} and if we should read and load a local file into
						// the message before sending.

						var filePathToOpen string
						foundFile := false
						var argPos int
						for i, v := range message.MethodArgs {
							if strings.Contains(v, "{{CTRL_FILE:") {
								foundFile = true
								argPos = i

								// Example to split:
								// echo {{CTRL_FILE:/somedir/msg_file.yaml}}>ctrlfile.txt
								//
								// Split at colon. We want the part after.
								ss := strings.Split(v, ":")
								// Split at "}}",so pos [0] in the result contains just the file path.
								sss := strings.Split(ss[1], "}}")
								filePathToOpen = sss[0]

							}
						}

						if foundFile {

							fh, err := os.Open(filePathToOpen)
							if err != nil {
								slog.Error("routeMessagesToPublisherProcess: failed to open file given as CTRL_FILE argument", "error", err)
								return
							}
							defer fh.Close()

							b, err := io.ReadAll(fh)
							if err != nil {
								slog.Error("routeMessagesToPublisherProcess: failed to read file given as CTRL_FILE argument", "file", filePathToOpen, "error", err)
								return
							}

							// Replace the {{CTRL_FILE}} with the actual content read from file.
							re := regexp.MustCompile(`(.*)({{CTRL_FILE.*}})(.*)`)
							message.MethodArgs[argPos] = re.ReplaceAllString(message.MethodArgs[argPos], `${1}`+string(b)+`${3}`)
							// ---

						}

						message.ArgSignature = s.processInitial.addMethodArgSignature(message)

						go s.processInitial.publishAMessage(message, s.natsConn)

					}(msg)
					// ------

				case <-ctx.Done():
					return
				}
			}

		}
		return fn
	}
	return fn
}

//const ETNatsPub actress.EventName = "ETNatsPub"
//
//func etNatsPubfn(s *server) actress.ETFunc {
//	fn := func(ctx context.Context, p *actress.Process) func() {
//		fn := func() {
//
//			var method Method
//			methodsAvailable := method.GetMethodsAvailable()
//
//			p.SignalReady()
//
//			for {
//				select {
//				case ev := <-p.InCh:
//					nEV := ev.NextEvent
//
//					msg := Message{}
//					err := cbor.Unmarshal(nEV.Data, msg)
//					if err != nil {
//						log.Printf("error: failed to cbor unmarshal event into message: %v\n", err)
//						os.Exit(1)
//					}
//
//					// ------
//					go func(message Message) {
//
//						s.messageID.mu.Lock()
//						s.messageID.id++
//						message.ID = s.messageID.id
//						s.messageID.mu.Unlock()
//
//						s.metrics.promMessagesProcessedIDLast.Set(float64(message.ID))
//
//						// Check if the format of the message is correct.
//						if _, ok := methodsAvailable.CheckIfExists(message.Method); !ok {
//							er := fmt.Errorf("error: routeMessagesToProcess: the method do not exist, message dropped: %v", message.Method)
//							s.errorKernel.errSend(s.processInitial, message, er, logError)
//							return
//						}
//
//						switch {
//						case message.Retries < 0:
//							message.Retries = s.configuration.DefaultMessageRetries
//						}
//						if message.MethodTimeout < 1 && message.MethodTimeout != -1 {
//							message.MethodTimeout = s.configuration.DefaultMethodTimeout
//						}
//
//						// ---
//						// Check for {{CTRL_FILE}} and if we should read and load a local file into
//						// the message before sending.
//
//						var filePathToOpen string
//						foundFile := false
//						var argPos int
//						for i, v := range message.MethodArgs {
//							if strings.Contains(v, "{{CTRL_FILE:") {
//								foundFile = true
//								argPos = i
//
//								// Example to split:
//								// echo {{CTRL_FILE:/somedir/msg_file.yaml}}>ctrlfile.txt
//								//
//								// Split at colon. We want the part after.
//								ss := strings.Split(v, ":")
//								// Split at "}}",so pos [0] in the result contains just the file path.
//								sss := strings.Split(ss[1], "}}")
//								filePathToOpen = sss[0]
//
//							}
//						}
//
//						if foundFile {
//
//							fh, err := os.Open(filePathToOpen)
//							if err != nil {
//								slog.Error("routeMessagesToPublisherProcess: failed to open file given as CTRL_FILE argument", "error", err)
//								return
//							}
//							defer fh.Close()
//
//							b, err := io.ReadAll(fh)
//							if err != nil {
//								slog.Error("routeMessagesToPublisherProcess: failed to read file given as CTRL_FILE argument", "file", filePathToOpen, "error", err)
//								return
//							}
//
//							// Replace the {{CTRL_FILE}} with the actual content read from file.
//							re := regexp.MustCompile(`(.*)({{CTRL_FILE.*}})(.*)`)
//							message.MethodArgs[argPos] = re.ReplaceAllString(message.MethodArgs[argPos], `${1}`+string(b)+`${3}`)
//							// ---
//
//						}
//
//						message.ArgSignature = s.processInitial.addMethodArgSignature(message)
//
//						go s.processInitial.publishAMessage(message, s.natsConn)
//
//					}(msg)
//					// ------
//
//				case <-ctx.Done():
//					return
//				}
//			}
//
//		}
//		return fn
//	}
//	return fn
//}
