package ctrl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"
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
			newReplyMessage(proc, message, out)
		}
	}()

	// Send back an ACK message.
	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ----

// methodKeysRequestUpdate.
// The nodes publish messages with the hash of all the public keys it currently
// have stored. If the hash is different than the one we currently have on central
// we send out an update with all the current keys to the node.
func methodKeysUpdateRequest(proc process, message Message, node string) ([]byte, error) {
	// Get a context with the timeout specified in message.MethodTimeout.

	// TODO:
	// - Since this is implemented as a NACK message we could implement a
	//   metric thats shows the last time a node did a key request.
	// - We could also implement a metrics on the receiver showing the last
	//   time a node had done an update.

	ctx, _ := getContextForMethodTimeout(proc.ctx, message)
	_ = node

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

				proc.errorKernel.logDebug(" <---- methodKeysRequestUpdate: received hash from node", "fromNode", message.FromNode, "data", message.Data)

				// Check if the received hash is the same as the one currently active,
				if bytes.Equal(proc.centralAuth.pki.nodesAcked.keysAndHash.Hash[:], message.Data) {
					proc.errorKernel.logDebug("methodKeysUpdateRequest: node and central have equal keys, nothing to do, exiting key update handler", "fromNode", message.FromNode)
					// proc.errorKernel.infoSend(proc, message, er)
					return
				}

				proc.errorKernel.logDebug("methodKeysUpdateRequest: node and central had not equal keys, preparing to send new version of keys", "fromNode", message.FromNode)

				proc.errorKernel.logDebug("methodKeysUpdateRequest: marshalling new keys and hash to send", "keys", proc.centralAuth.pki.nodesAcked.keysAndHash.Keys, "hash", proc.centralAuth.pki.nodesAcked.keysAndHash.Hash)

				b, err := json.Marshal(proc.centralAuth.pki.nodesAcked.keysAndHash)

				if err != nil {
					er := fmt.Errorf("error: methodKeysRequestUpdate, failed to marshal keys map: %v", err)
					proc.errorKernel.errSend(proc, message, er, logWarning)
				}
				proc.errorKernel.logDebug("----> methodKeysUpdateRequest: SENDING KEYS TO NODE=", "node", message.FromNode)
				newReplyMessage(proc, message, b)
			}()
		}
	}()

	// We're not sending an ACK message for this request type.
	return nil, nil
}

// Define the startup of a publisher that will send KeysRequestUpdate
// to central server and ask for publics keys, and to get them deliver back with a request
// of type KeysDeliverUpdate.
func procFuncKeysRequestUpdate(ctx context.Context, proc process, procFuncCh chan Message) error {
	ticker := time.NewTicker(time.Second * time.Duration(proc.configuration.KeysUpdateInterval))
	defer ticker.Stop()
	for {

		// Send a message with the hash of the currently stored keys,
		// so we would know on the subscriber at central if it should send
		// and update with new keys back.

		proc.nodeAuth.publicKeys.mu.Lock()
		proc.errorKernel.logDebug(" ----> publisher KeysRequestUpdate: sending our current hash", "hash", []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]))

		m := Message{
			FileName:    "publickeysget.log",
			Directory:   "publickeysget",
			ToNode:      Node(proc.configuration.CentralNodeName),
			FromNode:    Node(proc.node),
			Data:        []byte(proc.nodeAuth.publicKeys.keysAndHash.Hash[:]),
			Method:      KeysUpdateRequest,
			ReplyMethod: KeysUpdateReceive,
			ACKTimeout:  proc.configuration.DefaultMessageTimeout,
			Retries:     1,
		}
		proc.nodeAuth.publicKeys.mu.Unlock()

		proc.newMessagesCh <- m

		select {
		case <-ticker.C:
		case <-ctx.Done():
			proc.errorKernel.logDebug("procFuncKeysRequestUpdate: stopped handleFunc for: publisher", "subject", proc.subject.name())

			return nil
		}
	}
}

// ----

// Handler to receive the public keys from a central server.
func methodKeysUpdateReceive(proc process, message Message, node string) ([]byte, error) {
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
				er := fmt.Errorf("error: methodKeysReceiveUpdate : json unmarshal failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			proc.errorKernel.logDebug("<---- methodKeysUpdateReceive: after unmarshal, nodeAuth keysAndhash contains", "keysAndHash", keysAndHash)

			// If the received map was empty we also want to delete all the locally stored keys,
			// else we copy the marshaled keysAndHash we received from central into our map.
			if len(keysAndHash.Keys) < 1 {
				proc.nodeAuth.publicKeys.keysAndHash = newKeysAndHash()
			} else {
				proc.nodeAuth.publicKeys.keysAndHash = &keysAndHash
			}

			proc.nodeAuth.publicKeys.mu.Unlock()

			if err != nil {
				er := fmt.Errorf("error: methodKeysReceiveUpdate : json unmarshal failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}

			// We need to also persist the hash on the receiving nodes. We can then load
			// that key upon startup.

			err = proc.nodeAuth.publicKeys.saveToFile()
			if err != nil {
				er := fmt.Errorf("error: methodKeysReceiveUpdate : save to file failed: %v, message: %v", err, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
			}
		}
	}()

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
	proc.errorKernel.logDebug("methodKeysAllow: beginning of pushKeys", "nodes", nodes)

	proc.centralAuth.pki.nodesAcked.mu.Lock()
	defer proc.centralAuth.pki.nodesAcked.mu.Unlock()

	// Create the data payload of the current allowed keys.
	b, err := json.Marshal(proc.centralAuth.pki.nodesAcked.keysAndHash)
	if err != nil {
		er := fmt.Errorf("error: methodKeysAllow, failed to marshal keys map: %v", err)
		proc.errorKernel.errSend(proc, message, er, logWarning)
	}

	// proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Lock()
	// defer proc.centralAuth.pki.nodeNotAckedPublicKeys.mu.Unlock()

	// For all nodes that is not ack'ed we try to send an update once.
	for n := range proc.centralAuth.pki.nodeNotAckedPublicKeys.KeyMap {
		proc.errorKernel.logDebug("pushKeys: node to send REQKeysDeliverUpdate to", "node", n)
		msg := Message{
			ToNode:      n,
			Method:      KeysUpdateReceive,
			Data:        b,
			ReplyMethod: None,
			ACKTimeout:  0,
		}

		proc.newMessagesCh <- msg

		proc.errorKernel.logDebug("----> pushKeys: SENDING KEYS TO NODE", "node", message.FromNode)
	}

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
		proc.errorKernel.logDebug("pushKeys: node to send REQKeysDeliverUpdate to", "node", n)
		msg := Message{
			ToNode:      n,
			Method:      KeysUpdateReceive,
			Data:        b,
			ReplyMethod: None,
			ACKTimeout:  0,
		}

		proc.newMessagesCh <- msg

		proc.errorKernel.logDebug("----> methodKeysAllow: sending keys update to", "node", message.FromNode)
	}

	return nil

}

func methodKeysDelete(proc process, message Message, node string) ([]byte, error) {
	proc.errorKernel.logDebug("<--- methodKeysDelete received from", "node", message.FromNode, "methodArgs", message.MethodArgs)

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
			proc.errorKernel.logDebug("methodKeysDelete: Deleted public keys", "methodArgs", message.MethodArgs)

			// All new elements are now added, and we can create a new hash
			// representing the current keys in the allowed map.
			proc.centralAuth.updateHash(proc, message)
			proc.errorKernel.logDebug("methodKeysDelete: updated hash for public keys")

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
