package ctrl

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// ---

// Handler to get the public ed25519 key from a node.
func methodPublicKey(proc process, message Message, node string) ([]byte, error) {
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
			case outCh <- proc.nodeAuth.SignPublicKey:
			}
		}()

		select {
		// case proc.toRingbufferCh <- []subjectAndMessage{sam}:
		case <-ctx.Done():
		case out := <-outCh:

			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}
	}()

	// Send back an ACK message.
	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ----

// Handler to get all the public ed25519 keys from a central server.
func methodKeysRequestUpdate(proc process, message Message, node string) ([]byte, error) {
	// Get a context with the timeout specified in message.MethodTimeout.

	// TODO:
	// - Since this is implemented as a NACK message we could implement a
	//   metric thats shows the last time a node did a key request.
	// - We could also implement a metrics on the receiver showing the last
	//   time a node had done an update.

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
		case <-ctx.Done():
		// case out := <-outCh:
		case <-outCh:
			// Using a func here to set the scope of the lock, and then be able to
			// defer the unlock when leaving that scope.
			func() {
				proc.centralAuth.pki.nodesAcked.mu.Lock()
				defer proc.centralAuth.pki.nodesAcked.mu.Unlock()

				er := fmt.Errorf(" <---- methodKeysRequestUpdate: received hash from NODE=%v, HASH=%v", message.FromNode, message.Data)
				proc.errorKernel.logDebug(er)

				// Check if the received hash is the same as the one currently active,
				if bytes.Equal(proc.centralAuth.pki.nodesAcked.keysAndHash.Hash[:], message.Data) {
					er := fmt.Errorf("info: methodKeysRequestUpdate:  node %v and central have equal keys, nothing to do, exiting key update handler", message.FromNode)
					// proc.errorKernel.infoSend(proc, message, er)
					proc.errorKernel.logDebug(er)
					return
				}

				er = fmt.Errorf("info: methodKeysRequestUpdate: node %v and central had not equal keys, preparing to send new version of keys", message.FromNode)
				proc.errorKernel.logDebug(er)

				er = fmt.Errorf("info: methodKeysRequestUpdate: marshalling new keys and hash to send: map=%v, hash=%v", proc.centralAuth.pki.nodesAcked.keysAndHash.Keys, proc.centralAuth.pki.nodesAcked.keysAndHash.Hash)
				proc.errorKernel.logDebug(er)

				b, err := json.Marshal(proc.centralAuth.pki.nodesAcked.keysAndHash)

				if err != nil {
					er := fmt.Errorf("error: methodKeysRequestUpdate, failed to marshal keys map: %v", err)
					proc.errorKernel.errSend(proc, message, er, logWarning)
				}
				er = fmt.Errorf("----> methodKeysRequestUpdate: SENDING KEYS TO NODE=%v", message.FromNode)
				proc.errorKernel.logDebug(er)
				newReplyMessage(proc, message, b)
			}()
		}
	}()

	// We're not sending an ACK message for this request type.
	return nil, nil
}

// ----

// Handler to receive the public keys from a central server.
func methodKeysDeliverUpdate(proc process, message Message, node string) ([]byte, error) {
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

			proc.nodeAuth.publicKeys.mu.Lock()

			var keysAndHash keysAndHash

			err := json.Unmarshal(message.Data, &keysAndHash)
			if err != nil {
				er := fmt.Errorf("error: REQKeysDeliverUpdate : json unmarshal failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			er := fmt.Errorf("<---- REQKeysDeliverUpdate: after unmarshal, nodeAuth keysAndhash contains: %+v", keysAndHash)
			proc.errorKernel.logDebug(er)

			// If the received map was empty we also want to delete all the locally stored keys,
			// else we copy the marshaled keysAndHash we received from central into our map.
			if len(keysAndHash.Keys) < 1 {
				proc.nodeAuth.publicKeys.keysAndHash = newKeysAndHash()
			} else {
				proc.nodeAuth.publicKeys.keysAndHash = &keysAndHash
			}

			proc.nodeAuth.publicKeys.mu.Unlock()

			if err != nil {
				er := fmt.Errorf("error: REQKeysDeliverUpdate : json unmarshal failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			// We need to also persist the hash on the receiving nodes. We can then load
			// that key upon startup.

			err = proc.nodeAuth.publicKeys.saveToFile()
			if err != nil {
				er := fmt.Errorf("error: REQKeysDeliverUpdate : save to file failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
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

// ----

// Handler to allow new public keys into the database on central auth.
// Nodes will send the public key in the REQHello messages. When they
// are recived on the central server they will be put into a temp key
// map, and we need to acknowledge them before they are moved into the
// main key map, and then allowed to be sent out to other nodes.
func methodKeysAllow(proc process, message Message, node string) ([]byte, error) {
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
		case <-ctx.Done():
		case <-outCh:
			proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Lock()
			defer proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Unlock()

			// Range over all the MethodArgs, where each element represents a node to allow,
			// and move the node from the notAcked map to the allowed map.
			for _, n := range message.MethodArgs {
				key, ok := proc.centralAuth.pki.nodeNotAckedPublicKeys.KeyMap[Node(n)]
				if ok {

					func() {
						proc.centralAuth.pki.nodesAcked.mu.Lock()
						defer proc.centralAuth.pki.nodesAcked.mu.Unlock()

						// Store/update the node and public key on the allowed pubKey map.
						proc.centralAuth.pki.nodesAcked.keysAndHash.Keys[Node(n)] = key
					}()

					// Add key to persistent storage.
					proc.centralAuth.pki.dbUpdatePublicKey(string(n), key)

					// Delete the key from the NotAcked map
					delete(proc.centralAuth.pki.nodeNotAckedPublicKeys.KeyMap, Node(n))

					er := fmt.Errorf("info: REQKeysAllow : allowed new/updated public key for %v to allowed public key map", n)
					proc.errorKernel.infoSend(proc, message, er)
				}
			}

			// All new elements are now added, and we can create a new hash
			// representing the current keys in the allowed map.
			proc.centralAuth.updateHash(proc, message)

			// If new keys were allowed into the main map, we should send out one
			// single update to all the registered nodes to inform of an update.
			// If a node is not reachable at the time the update is sent it is
			// not a problem since the nodes will periodically check for updates.
			//
			// If there are errors we will return from the function, and send no
			// updates.
			err := pushKeys(proc, message, []Node{})

			if err != nil {
				proc.errorKernel.errSend(proc, message, err, logWarning)
				return
			}

		}
	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// pushKeys will try to push the the current keys and hashes to each node once.
// As input arguments it takes the current process, the current message, and a
// []Node in nodes.
// The nodes are used when a node or nodes have been deleted from the current
// nodesAcked map since it will contain the nodes that were deleted so we are
// also able to send an update to them as well.
func pushKeys(proc process, message Message, nodes []Node) error {
	er := fmt.Errorf("info: beginning of pushKeys, nodes=%v", nodes)
	var knh []byte
	proc.errorKernel.logDebug(er)

	err := func() error {
		proc.centralAuth.pki.nodesAcked.mu.Lock()
		defer proc.centralAuth.pki.nodesAcked.mu.Unlock()

		b, err := json.Marshal(proc.centralAuth.pki.nodesAcked.keysAndHash)
		if err != nil {
			er := fmt.Errorf("error: methodKeysAllow, failed to marshal keys map: %v", err)
			return er
		}

		copy(knh, b)

		return nil
	}()

	if err != nil {
		return err
	}

	// proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Lock()
	// defer proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Unlock()

	// For all nodes that is not ack'ed we try to send an update once.
	for n := range proc.centralAuth.pki.nodeNotAckedPublicKeys.KeyMap {
		er := fmt.Errorf("info: node to send REQKeysDeliverUpdate to:%v ", n)
		proc.errorKernel.logDebug(er)
		msg := Message{
			ToNode:      n,
			Method:      KeysDeliverUpdate,
			ReplyMethod: None,
			ACKTimeout:  0,
		}

		sam, err := newSubjectAndMessage(msg)
		if err != nil {
			// In theory the system should drop the message before it reaches here.
			er := fmt.Errorf("error: newSubjectAndMessage : %v, message: %v", err, message)
			proc.errorKernel.errSend(proc, message, er, logWarning)
		}

		proc.newMessagesCh <- sam

		er = fmt.Errorf("----> methodKeysAllow: SENDING KEYS TO NODE=%v", message.FromNode)
		proc.errorKernel.logDebug(er)
	}

	// Create the data payload of the current allowed keys.
	b, err := json.Marshal(proc.centralAuth.pki.nodesAcked.keysAndHash)

	if err != nil {
		er := fmt.Errorf("error: methodKeysAllow, failed to marshal keys map: %v", err)
		proc.errorKernel.errSend(proc, message, er, logWarning)
	}

	proc.centralAuth.pki.nodesAcked.mu.Lock()
	defer proc.centralAuth.pki.nodesAcked.mu.Unlock()

	// Concatenate the current nodes in the keysAndHash map and the nodes
	// we got from the function argument when this function was called.
	nodeMap := make(map[Node]struct{})

	for n := range proc.centralAuth.pki.nodesAcked.keysAndHash.Keys {
		nodeMap[n] = struct{}{}
	}
	for _, n := range nodes {
		nodeMap[n] = struct{}{}
	}

	// For all nodes that is ack'ed we try to send an update once.
	for n := range nodeMap {
		er := fmt.Errorf("info: node to send REQKeysDeliverUpdate to:%v ", n)
		proc.errorKernel.logDebug(er)
		msg := Message{
			ToNode:      n,
			Method:      KeysDeliverUpdate,
			Data:        b,
			ReplyMethod: None,
			ACKTimeout:  0,
		}

		sam, err := newSubjectAndMessage(msg)
		if err != nil {
			// In theory the system should drop the message before it reaches here.
			er := fmt.Errorf("error: newSubjectAndMessage : %v, message: %v", err, message)
			proc.errorKernel.errSend(proc, message, er, logWarning)
		}

		proc.newMessagesCh <- sam

		er = fmt.Errorf("----> methodKeysAllow: sending keys update to node=%v", message.FromNode)
		proc.errorKernel.logDebug(er)
	}

	return nil

}

func methodKeysDelete(proc process, message Message, node string) ([]byte, error) {
	inf := fmt.Errorf("<--- methodKeysDelete received from: %v, containing: %v", message.FromNode, message.MethodArgs)
	proc.errorKernel.logDebug(inf)

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
				errCh <- fmt.Errorf("error: methodAclGroupNodesDeleteNode: got <1 number methodArgs, want >0")
				return
			}

			// HERE:
			//  We want to be able to define more nodes keys to delete.
			//  Loop over the methodArgs, and call the keyDelete function for each node,
			//  or rewrite the deleteKey to deleteKeys and it takes a []node as input
			//  so all keys can be deleted in 1 go, and we create 1 new hash, instead
			//  of doing it for each node delete.

			proc.centralAuth.deletePublicKeys(proc, message, message.MethodArgs)
			er := fmt.Errorf("info: Deleted public keys: %v", message.MethodArgs)
			proc.errorKernel.logDebug(er)

			// All new elements are now added, and we can create a new hash
			// representing the current keys in the allowed map.
			proc.centralAuth.updateHash(proc, message)
			er = fmt.Errorf(" * DEBUG updated hash for public keys")
			proc.errorKernel.logDebug(er)

			var nodes []Node

			for _, n := range message.MethodArgs {
				nodes = append(nodes, Node(n))
			}

			err := pushKeys(proc, message, nodes)

			if err != nil {
				proc.errorKernel.errSend(proc, message, err, logWarning)
				return
			}

			outString := fmt.Sprintf("deleted public keys for the nodes=%v\n", message.MethodArgs)
			out := []byte(outString)

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case err := <-errCh:
			proc.errorKernel.errSend(proc, message, err, logWarning)

		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodAclGroupNodesDeleteNode: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logWarning)

		case out := <-outCh:

			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
