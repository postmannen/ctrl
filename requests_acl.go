package ctrl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/fxamacker/cbor/v2"
)

// ----

// Handler to get all acl's from a central server.
func methodAclRequestUpdate(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- subscriber methodAclRequestUpdate received from: %v, hash data = %v", message.FromNode, message.Data)
	proc.errorKernel.logDebug(er)

	// fmt.Printf("\n --- subscriber methodAclRequestUpdate: the message brought to handler : %+v\n", message)

	// Get a context with the timeout specified in message.MethodTimeout.
	ctx, _ := getContextForMethodTimeout(proc.ctx, message)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()
		outCh := make(chan []byte)

		go func() {

			select {
			case <-ctx.Done():
				// Done
			case outCh <- []byte{}:
			}
		}()

		select {
		case <-ctx.Done():
		// case out := <-outCh:
		case <-outCh:
			// Using a func here to set the scope of the lock, and then be able to
			// defer the unlock when leaving that scope.
			func() {
				proc.centralAuth.accessLists.schemaGenerated.mu.Lock()
				defer proc.centralAuth.accessLists.schemaGenerated.mu.Unlock()

				er := fmt.Errorf("info: subscriber methodAclRequestUpdate: got acl hash from NODE=%v, HASH data =%v", message.FromNode, message.Data)
				proc.errorKernel.logDebug(er)

				// Check if the received hash is the same as the one currently active,
				// If it is the same we exit the handler immediately.
				hash32 := proc.centralAuth.accessLists.schemaGenerated.GeneratedACLsMap[message.FromNode].Hash
				hash := hash32[:]
				er = fmt.Errorf("info: subscriber methodAclRequestUpdate:  the central acl hash=%v", hash32)
				proc.errorKernel.logDebug(er)
				if bytes.Equal(hash, message.Data) {
					er := fmt.Errorf("info: subscriber methodAclRequestUpdate:  NODE AND CENTRAL HAVE EQUAL ACL HASH, NOTHING TO DO, EXITING HANDLER")
					proc.errorKernel.logDebug(er)
					return
				}

				er = fmt.Errorf("info: subscriber methodAclRequestUpdate: NODE AND CENTRAL HAD NOT EQUAL ACL, PREPARING TO SEND NEW VERSION OF Acl")
				proc.errorKernel.logDebug(er)

				// Generate JSON for Message.Data

				hdh := HostACLsSerializedWithHash{}
				hdh.Data = proc.centralAuth.accessLists.schemaGenerated.GeneratedACLsMap[message.FromNode].Data
				// fmt.Printf("\n * DEBUGGING: before marshalling, hdh.Data=%v\n", hdh.Data)
				hdh.Hash = proc.centralAuth.accessLists.schemaGenerated.GeneratedACLsMap[message.FromNode].Hash
				// fmt.Printf("\n * DEBUGGING: before marshalling, hdh.Hash=%v\n\n", hdh.Hash)

				js, err := json.Marshal(hdh)
				if err != nil {
					er := fmt.Errorf("error: REQAclRequestUpdate : json marshal failed: %v, message: %v", err, message)
					proc.errorKernel.errSend(proc, message, er, logWarning)
				}

				er = fmt.Errorf("----> subscriber methodAclRequestUpdate: SENDING ACL'S TO NODE=%v, serializedAndHash=%+v", message.FromNode, hdh)
				proc.errorKernel.logDebug(er)

				newReplyMessage(proc, message, js)
			}()
		}
	}()

	// We're not sending an ACK message for this request type.
	return nil, nil
}

func procFuncAclRequestUpdate(ctx context.Context, proc process, procFuncCh chan Message) error {
	ticker := time.NewTicker(time.Second * time.Duration(proc.configuration.AclUpdateInterval))
	defer ticker.Stop()
	for {

		// Send a message with the hash of the currently stored acl's,
		// so we would know for the subscriber at central if it should send
		// and update with new keys back.

		proc.nodeAuth.nodeAcl.mu.Lock()
		er := fmt.Errorf(" ----> publisher AclRequestUpdate: sending our current hash: %v", []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]))
		proc.errorKernel.logDebug(er)

		m := Message{
			FileName:    "aclRequestUpdate.log",
			Directory:   "aclRequestUpdate",
			ToNode:      Node(proc.configuration.CentralNodeName),
			FromNode:    Node(proc.node),
			Data:        []byte(proc.nodeAuth.nodeAcl.aclAndHash.Hash[:]),
			Method:      AclRequestUpdate,
			ReplyMethod: AclDeliverUpdate,
			ACKTimeout:  proc.configuration.DefaultMessageTimeout,
			Retries:     1,
		}
		proc.nodeAuth.nodeAcl.mu.Unlock()

		proc.newMessagesCh <- m

		select {
		case <-ticker.C:
		case <-ctx.Done():
			er := fmt.Errorf("info: stopped handleFunc for: publisher %v", proc.subject.name())
			// sendErrorLogMessage(proc.toRingbufferCh, proc.node, er)
			proc.errorKernel.logDebug(er)
			return nil
		}
	}
}

// ----

// Handler to receive the acls from a central server.
func methodAclDeliverUpdate(proc process, message Message, node string) ([]byte, error) {
	inf := fmt.Errorf("<--- subscriber methodAclDeliverUpdate received from: %v, containing: %v", message.FromNode, message.Data)
	proc.errorKernel.logDebug(inf)

	// fmt.Printf("\n --- subscriber methodAclRequestUpdate: the message received on handler : %+v\n\n", message)

	// Get a context with the timeout specified in message.MethodTimeout.
	ctx, _ := getContextForMethodTimeout(proc.ctx, message)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()
		outCh := make(chan []byte)

		go func() {
			// Normally we would do some logic here, where the result is passed to outCh when done,
			// so we can split up the working logic, and f.ex. sending a reply logic.
			// In this case this go func and the below select is not needed, but keeping it so the
			// structure is the same as the other handlers.
			select {
			case <-ctx.Done():

			case outCh <- []byte{}:
			}
		}()

		select {
		// case proc.toRingbufferCh <- []subjectAndMessage{sam}:
		case <-ctx.Done():
		case <-outCh:

			proc.nodeAuth.nodeAcl.mu.Lock()

			hdh := HostACLsSerializedWithHash{}

			err := json.Unmarshal(message.Data, &hdh)
			if err != nil {
				er := fmt.Errorf("error: subscriber REQAclDeliverUpdate : json unmarshal failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			mapOfFromNodeCommands := make(map[Node]map[command]struct{})

			if len(hdh.Data) != 0 {
				err = cbor.Unmarshal(hdh.Data, &mapOfFromNodeCommands)
				if err != nil {
					er := fmt.Errorf("error: subscriber REQAclDeliverUpdate : cbor unmarshal failed: %v, message: %v", err, message)
					proc.errorKernel.errSend(proc, message, er, logError)
				}
			}

			proc.nodeAuth.nodeAcl.aclAndHash.Hash = hdh.Hash
			proc.nodeAuth.nodeAcl.aclAndHash.Acl = mapOfFromNodeCommands

			fmt.Printf("\n <---- subscriber REQAclDeliverUpdate: after unmarshal, nodeAuth aclAndhash contains: %+v\n\n", proc.nodeAuth.nodeAcl.aclAndHash)

			proc.nodeAuth.nodeAcl.mu.Unlock()

			err = proc.nodeAuth.nodeAcl.saveToFile()
			if err != nil {
				er := fmt.Errorf("error: subscriber REQAclDeliverUpdate : save to file failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logError)
			}

			// Prepare and queue for sending a new message with the output
			// of the action executed.
			// newReplyMessage(proc, message, out)
		}
	}()

	// Send back an ACK message.
	// ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return nil, nil
}

// ---

func methodAclAddCommand(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclAddCommand received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 3:
				errCh <- fmt.Errorf("error: methodREQAclAddAccessList: got <3 number methodArgs, want 3")
				return
			}

			host := message.MethodArgs[0]
			source := message.MethodArgs[1]
			cmd := message.MethodArgs[2]

			proc.centralAuth.aclAddCommand(Node(host), Node(source), command(cmd))

			outString := fmt.Sprintf("acl added: host=%v, source=%v, command=%v\n", host, source, cmd)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodREQAclAddAccessList: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclDeleteCommand(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclDeleteCommand received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 3:
				errCh <- fmt.Errorf("error: methodAclDeleteCommand: got <3 number methodArgs, want 3")
				return
			}

			host := message.MethodArgs[0]
			source := message.MethodArgs[1]
			cmd := message.MethodArgs[2]

			proc.centralAuth.aclDeleteCommand(Node(host), Node(source), command(cmd))

			outString := fmt.Sprintf("acl deleted: host=%v, source=%v, command=%v\n", host, source, cmd)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclDeleteCommand: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclDeleteSource(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclDeleteSource received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 2:
				errCh <- fmt.Errorf("error: methodAclDeleteSource: got <2 number methodArgs, want 2")
				return
			}

			host := message.MethodArgs[0]
			source := message.MethodArgs[1]

			proc.centralAuth.aclDeleteSource(Node(host), Node(source))

			outString := fmt.Sprintf("acl deleted: host=%v, source=%v\n", host, source)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclDeleteSource: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupNodesAddNode(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupNodesAddNode received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 2:
				errCh <- fmt.Errorf("error: methodAclGroupNodesAddNode: got <2 number methodArgs, want 2")
				return
			}

			ng := message.MethodArgs[0]
			n := message.MethodArgs[1]

			proc.centralAuth.groupNodesAddNode(nodeGroup(ng), Node(n))

			outString := fmt.Sprintf("added node to nodeGroup: nodeGroup=%v, node=%v\n", ng, n)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupNodesAddNode: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupNodesDeleteNode(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupNodesDeleteNode received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 2:
				errCh <- fmt.Errorf("error: methodAclGroupNodesDeleteNode: got <2 number methodArgs, want 2")
				return
			}

			ng := message.MethodArgs[0]
			n := message.MethodArgs[1]

			proc.centralAuth.groupNodesDeleteNode(nodeGroup(ng), Node(n))

			outString := fmt.Sprintf("deleted node from nodeGroup: nodeGroup=%v, node=%v\n", ng, n)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupNodesDeleteNode: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupNodesDeleteGroup(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupNodesDeleteGroup received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 1:
				errCh <- fmt.Errorf("error: methodAclGroupNodesDeleteGroup: got <1 number methodArgs, want 1")
				return
			}

			ng := message.MethodArgs[0]

			proc.centralAuth.groupNodesDeleteGroup(nodeGroup(ng))

			outString := fmt.Sprintf("deleted nodeGroup: nodeGroup=%v\n", ng)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupNodesDeleteGroup: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupCommandsAddCommand(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupCommandsAddCommand received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 2:
				errCh <- fmt.Errorf("error: methodAclGroupCommandsAddCommand: got <2 number methodArgs, want 1")
				return
			}

			cg := message.MethodArgs[0]
			c := message.MethodArgs[1]

			proc.centralAuth.groupCommandsAddCommand(commandGroup(cg), command(c))

			outString := fmt.Sprintf("added command to commandGroup: commandGroup=%v, command=%v\n", cg, c)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupCommandsAddCommand: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupCommandsDeleteCommand(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupCommandsDeleteCommand received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 1:
				errCh <- fmt.Errorf("error: methodAclGroupCommandsDeleteCommand: got <1 number methodArgs, want 1")
				return
			}

			cg := message.MethodArgs[0]
			c := message.MethodArgs[1]

			proc.centralAuth.groupCommandsDeleteCommand(commandGroup(cg), command(c))

			outString := fmt.Sprintf("deleted command from commandGroup: commandGroup=%v, command=%v\n", cg, c)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupCommandsDeleteCommand: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclGroupCommandsDeleteGroup(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclGroupCommandsDeleteGroup received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 1:
				errCh <- fmt.Errorf("error: methodAclGroupCommandsDeleteGroup: got <1 number methodArgs, want 1")
				return
			}

			cg := message.MethodArgs[0]

			proc.centralAuth.groupCommandDeleteGroup(commandGroup(cg))

			outString := fmt.Sprintf("deleted commandGroup: commandGroup=%v\n", cg)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupCommandsDeleteGroup: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclExport(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclExport received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)
		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			out, err := proc.centralAuth.exportACLs()
			if err != nil {
				errCh <- fmt.Errorf("error: methodAclExport failed: %v", err)
				return
			}

			// outString := fmt.Sprintf("Exported acls sent from: %v\n", message.FromNode)
			// out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclExport: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ---

func methodAclImport(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- methodAclImport received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)

		outCh := make(chan []byte)
		errCh := make(chan error)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			switch {
			case len(message.MethodArgs) < 1:
				errCh <- fmt.Errorf("error: methodAclImport: got <1 number methodArgs, want 1")
				return
			}

			js := []byte(message.MethodArgs[0])
			err := proc.centralAuth.importACLs(js)
			if err != nil {
				errCh <- fmt.Errorf("error: methodAclImport failed: %v", err)
				return
			}

			outString := fmt.Sprintf("Imported acl's sent from: %v\n", message.FromNode)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logError)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclImport: method timed out")
			proc.errorKernel.errSend(proc, message, er, logInfo)

		case out := <-outCh:
			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
