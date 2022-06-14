package steward

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fxamacker/cbor/v2"
	"github.com/google/uuid"
)

type methodREQCopySrc struct {
	event Event
}

func (m methodREQCopySrc) getKind() Event {
	return m.event
}

// Idea:
//
// Initialization, Source:
// - Use the REQCopySrc method to handle the initial request from the user.
// - Spawn a REQCopySrc_uid subscriber to receive sync messages from destination.
// - Send the uid, and full-file hash to the destination in a REQCopyDst message.
//
// Initialization, Destination:
// - Spawn a REQCopyDst-uid from the uid we got from source.
// --------------------------------------------------------------------------------------
//
// All below happens in the From-uid and To-uid methods until the copying is done.
//
// - dst->src, dst sends a REQCopySrc-uid message with status "ready" file receiving to src.
// - src receives the message and start reading the file:
// - src, creates hash of the complete file.
// - src, reads the file in chunks, and create a hash of each chunk.
//   - src->dst, send chunk when read.
//   - dst->src, wait for status "ready" indicating the chuck was transfered.
//   - Loop and read new chunc.
//   - src->dst, when last chunch is sent send status "last"
//   - src->dst, if failure send status "error", abort file copying and clean up on both src and dst.
//
// - dst, read and store each chunch to tmp folder and verify hash.
//   - dst->src, send status "ready" to src when chunch is stored.
//	 - loop and check for status "last", if last:
//     - build original file from chuncs.
//     - verify hash when file is built.
//     - dst->src, send status "done".
//
// - We should also be be able to resend a chunk, or restart the copying from where we left of if it seems to hang.
//
// dataStructure{
//	Data	[]bytes
//	Status	copyStatus
//  id		int
// }
//
// Create a new copy sync process to handle the actual file copying.
// We use the context already created based on the time out specified
// in the requestTimeout field of the message.
//
// -----------------------------------------------------
// Handle writing to a file. Will truncate any existing data if the file did already
// exist.
func (m methodREQCopySrc) handler(proc process, message Message, node string) ([]byte, error) {

	var subProcessName string

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		switch {
		case len(message.MethodArgs) < 3:
			er := fmt.Errorf("error: methodREQCopySrc: got <3 number methodArgs: want srcfilePath,dstNode,dstFilePath")
			proc.errorKernel.errSend(proc, message, er)

			return
		}

		SrcFilePath := message.MethodArgs[0]
		DstNode := message.MethodArgs[1]
		DstFilePath := message.MethodArgs[2]

		// Get a context with the timeout specified in message.MethodTimeout.
		// Since the subProc spawned will outlive this method here we do not
		// want to cancel this method. We care about the methodTimeout, but
		// we ignore the CancelFunc.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)

		// Create a subject for one copy request
		uid := uuid.New()
		subProcessName = fmt.Sprintf("REQSUBCopySrc.%v", uid.String())

		dstDir := filepath.Dir(DstFilePath)
		dstFile := filepath.Base(DstFilePath)
		m := Method(subProcessName)

		// Also choosing to create the naming for the dst method here so
		// we can have all the information in the cia from the beginning
		// at both ends.
		dstSubProcessName := fmt.Sprintf("REQSUBCopyDst.%v", uid.String())
		dstM := Method(dstSubProcessName)

		cia := copyInitialData{
			UUID:        uid.String(),
			SrcNode:     proc.node,
			SrcMethod:   m,
			DstNode:     Node(DstNode),
			DstMethod:   dstM,
			SrcFilePath: SrcFilePath,
			DstDir:      dstDir,
			DstFile:     dstFile,
		}

		sub := newSubjectNoVerifyHandler(m, node)

		// Create a new sub process that will do the actual file copying.
		copySrcSubProc := newProcess(ctx, proc.server, sub, processKindSubscriber, nil)

		// Give the sub process a procFunc so we do the actual copying within a procFunc,
		// and not directly within the handler.
		copySrcSubProc.procFunc = copySrcSubProcFunc(copySrcSubProc, cia)

		// assign a handler to the sub process
		copySrcSubProc.handler = copySrcSubHandler(cia)

		// The process will be killed when the context expires.
		go copySrcSubProc.spawnWorker()

		// Send a message over the the node where the destination file will be written,
		// to also start up a sub process on the destination node.

		// Marshal the data payload to send the the dst.
		cb, err := cbor.Marshal(cia)
		if err != nil {
			er := fmt.Errorf("error: newSubjectAndMessage : %v, message: %v", err, message)
			proc.errorKernel.errSend(proc, message, er)
			cancel()
		}

		msg := message
		msg.ToNode = Node(DstNode)
		//msg.Method = REQToFile
		msg.Method = REQCopyDst
		msg.Data = cb
		// msg.Directory = dstDir
		// msg.FileName = dstFile

		sam, err := newSubjectAndMessage(msg)
		if err != nil {
			er := fmt.Errorf("error: methodREQCopySrc failed to cbor Marshal data: %v, message=%v", err, message)
			proc.errorKernel.errSend(proc, message, er)
			cancel()
		}

		proc.toRingbufferCh <- []subjectAndMessage{sam}

		replyData := fmt.Sprintf("info: succesfully initiated copy source process: procName=%v, srcNode=%v, srcPath=%v, dstNode=%v, dstPath=%v, starting sub process=%v for the actual copying\n", copySrcSubProc.processName, node, SrcFilePath, DstNode, DstFilePath, subProcessName)

		newReplyMessage(proc, message, []byte(replyData))

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

type copyInitialData struct {
	UUID        string
	SrcMethod   Method
	SrcNode     Node
	DstMethod   Method
	DstNode     Node
	SrcFilePath string
	DstDir      string
	DstFile     string
}

// ----

type methodREQCopyDst struct {
	event Event
}

func (m methodREQCopyDst) getKind() Event {
	return m.event
}

// Handle writing to a file. Will truncate any existing data if the file did already
// exist.
// Same as the REQToFile, but this requst type don't use the default data folder path
// for where to store files or add information about node names.
// This method also sends a msgReply back to the publisher if the method was done
// successfully, where REQToFile do not.
// This method will truncate and overwrite any existing files.
func (m methodREQCopyDst) handler(proc process, message Message, node string) ([]byte, error) {
	var subProcessName string

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		var cia copyInitialData
		err := cbor.Unmarshal(message.Data, &cia)
		if err != nil {
			er := fmt.Errorf("error: methodREQCopyDst: failed to cbor Unmarshal data: %v, message=%v", err, message)
			proc.errorKernel.errSend(proc, message, er)
			return
		}

		// Get a context with the timeout specified in message.MethodTimeout.
		// Since the subProc spawned will outlive this method here we do not
		// want to cancel this method. We care about the methodTimeout, but
		// we ignore the CancelFunc.
		ctx, _ := getContextForMethodTimeout(proc.ctx, message)

		// Create a subject for one copy request
		// subProcessName = fmt.Sprintf("REQSUBCopyDst.%v", cia.UUID)
		// m := Method(subProcessName)
		sub := newSubjectNoVerifyHandler(cia.DstMethod, node)

		// Create a new sub process that will do the actual file copying.
		copyDstSubProc := newProcess(ctx, proc.server, sub, processKindSubscriber, nil)

		// Give the sub process a procFunc so we do the actual copying within a procFunc,
		// and not directly within the handler.
		copyDstSubProc.procFunc = copyDstSubProcFunc(copyDstSubProc, cia, message)

		// assign a handler to the sub process
		copyDstSubProc.handler = copyDstSubHandler(cia)

		// The process will be killed when the context expires.
		go copyDstSubProc.spawnWorker()

		fp := filepath.Join(cia.DstDir, cia.DstFile)
		replyData := fmt.Sprintf("info: succesfully initiated copy source process: procName=%v, srcNode=%v, dstPath=%v, starting sub process=%v for the actual copying\n", copyDstSubProc.processName, node, fp, subProcessName)

		newReplyMessage(proc, message, []byte(replyData))

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

func copySrcSubHandler(cia copyInitialData) func(process, Message, string) ([]byte, error) {
	h := func(proc process, message Message, node string) ([]byte, error) {

		// We should receive a ready message generated by the procFunc of Dst.

		select {
		case <-proc.ctx.Done():
			log.Printf(" * copySrcHandler ended: %v\n", proc.processName)
		case proc.procFuncCh <- message:
			log.Printf(" * copySrcHandler passing message over to procFunc: %v\n", proc.processName)
		}

		return nil, nil
	}

	return h
}

func copyDstSubHandler(cia copyInitialData) func(process, Message, string) ([]byte, error) {
	h := func(proc process, message Message, node string) ([]byte, error) {

		select {
		case <-proc.ctx.Done():
			log.Printf(" * copyDstHandler ended: %v\n", proc.processName)
		case proc.procFuncCh <- message:
			log.Printf(" * copySrcHandler passing message over to procFunc: %v\n", proc.processName)
		}

		return nil, nil
	}

	return h
}

func copySrcSubProcFunc(proc process, cia copyInitialData) func(context.Context, chan Message) error {
	pf := func(ctx context.Context, procFuncCh chan Message) error {

		const readChuncSize = 2

		// Initiate the file copier.
		fh, err := os.Open(cia.SrcFilePath)
		if err != nil {
			// errCh <- fmt.Errorf("error: methodREQCopyFile: failed to open file: %v, %v", SrcFilePath, err)
			log.Fatalf("error: copySrcSubProcFunc: failed to open file: %v\n", err)
			return nil
		}
		defer fh.Close()

		// Do action based on copyStatus received.
		for {
			fmt.Printf("\n * DEBUG: copySrcSubProcFunc: cia contains: %+v\n\n", cia)
			select {
			case <-ctx.Done():
				log.Printf(" * copySrcProcFunc ENDED: %v\n", proc.processName)
				return nil

			// Pick up the message recived by the copySrcSubHandler.
			case message := <-procFuncCh:
				var csa copySubData
				err := cbor.Unmarshal(message.Data, &csa)
				if err != nil {
					log.Fatalf("error: copySrcSubHandler: cbor unmarshal of csa failed: %v\n", err)
				}

				switch csa.CopyStatus {
				case copyReady:
					log.Printf(" * RECEIVED in copySrcSubProcFunc *  copyStatus=copyReady: %v\n\n", csa.CopyStatus)
					b := make([]byte, readChuncSize)
					_, err := fh.Read(b)
					if err != nil {
						log.Fatalf("error: copySrcSubHandler: failed to read chuck from file: %v\n", err)
					}

					// Create message and send data to dst
					fmt.Printf("**** DATA READ: %v\n", b)

					// TESTING HERE:
					{
						csa := copySubData{
							CopyStatus: copyData,
							CopyData:   b,
						}

						csaSerialized, err := cbor.Marshal(csa)
						if err != nil {
							log.Fatalf("error: copyDstSubProcFunc: cbor marshal of csa failed: %v\n", err)
						}

						// We want to send a message back to src that we are ready to start.
						fmt.Printf("\n\n\n ************** DEBUG: copyDstHandler sub process sending copyReady to:%v\n ", message.FromNode)
						msg := Message{
							ToNode:      cia.DstNode,
							FromNode:    cia.SrcNode,
							Method:      cia.DstMethod,
							ReplyMethod: REQNone,
							Data:        csaSerialized,
						}

						fmt.Printf("\n ***** DEBUG: copyDstSubProcFunc: cia.SrcMethod: %v\n\n ", cia.SrcMethod)

						sam, err := newSubjectAndMessage(msg)
						if err != nil {
							log.Fatalf("copyDstProcSubFunc: newSubjectAndMessage failed: %v\n", err)
						}

						proc.toRingbufferCh <- []subjectAndMessage{sam}
					}

				default:
					// TODO: Any error logic here ?
					log.Fatalf("error: copySrcSubProcFunc: not valid copyStatus, exiting: %v\n", csa.CopyStatus)
				}
			}
		}

		//return nil
	}

	return pf
}

type copyStatus int

const (
	copyReady copyStatus = 1
	copyData  copyStatus = 2
)

// copySubData is the structure being used between the src and dst while copying data.
type copySubData struct {
	CopyStatus copyStatus
	CopyData   []byte
}

func copyDstSubProcFunc(proc process, cia copyInitialData, message Message) func(context.Context, chan Message) error {
	pf := func(ctx context.Context, procFuncCh chan Message) error {
		fmt.Printf("\n ******* WORKING IN copyDstSubProcFunc: %+v\n\n", cia)

		csa := copySubData{
			CopyStatus: copyReady,
		}

		csaSerialized, err := cbor.Marshal(csa)
		if err != nil {
			log.Fatalf("error: copyDstSubProcFunc: cbor marshal of csa failed: %v\n", err)
		}

		// We want to send a message back to src that we are ready to start.
		fmt.Printf("\n\n\n ************** DEBUG: copyDstHandler sub process sending copyReady to:%v\n ", message.FromNode)
		msg := Message{
			ToNode:      message.FromNode,
			FromNode:    message.ToNode,
			Method:      cia.SrcMethod,
			ReplyMethod: REQNone,
			Data:        csaSerialized,
		}

		fmt.Printf("\n ***** DEBUG: copyDstSubProcFunc: cia.SrcMethod: %v\n\n ", cia.SrcMethod)

		sam, err := newSubjectAndMessage(msg)
		if err != nil {
			log.Fatalf("copyDstProcSubFunc: newSubjectAndMessage failed: %v\n", err)
		}

		proc.toRingbufferCh <- []subjectAndMessage{sam}

		for {
			fmt.Printf("\n * DEBUG: copyDstSubProcFunc: cia contains: %+v\n\n", cia)
			select {
			case <-ctx.Done():
				log.Printf(" * copyDstProcFunc ended: %v\n", proc.processName)
				return nil
			case message := <-procFuncCh:
				var csa copySubData
				err := cbor.Unmarshal(message.Data, &csa)
				if err != nil {
					log.Fatalf("error: copySrcSubHandler: cbor unmarshal of csa failed: %v\n", err)
				}

				switch csa.CopyStatus {
				case copyData:
					// Write the data chunk to disk ?
					fmt.Printf("\n * Received data: %s\n\n", csa.CopyData)
				}
			}
		}

	}

	return pf
}
