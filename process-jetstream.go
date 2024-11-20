package ctrl

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go/jetstream"
)

// messageSubscriberHandlerJetstream will deserialize the message when a new message is
// received, check the MessageType field in the message to decide what
// kind of message it is and then it will check how to handle that message type,
// and then call the correct method handler for it.
//
// This handler function should be started in it's own go routine,so
// one individual handler is started per message received so we can keep
// the state of the message being processed, and then reply back to the
// correct sending process's reply, meaning so we ACK back to the correct
// publisher.
func (p process) messageSubscriberHandlerJetstream(thisNode string, msg jetstream.Msg, subject string) {

	// Variable to hold a copy of the message data.
	msgData := msg.Data()

	// If debugging is enabled, print the source node name of the nats messages received.
	headerFromNode := msg.Headers().Get("fromNode")
	if headerFromNode != "" {
		er := fmt.Errorf("info: subscriberHandlerJetstream: nats message received from %v, with subject %v ", headerFromNode, subject)
		p.errorKernel.logDebug(er)
	}

	zr, err := zstd.NewReader(nil)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: zstd NewReader failed: %v", err)
		p.errorKernel.errSend(p, Message{}, er, logWarning)
		return
	}
	msgData, err = zr.DecodeAll(msgData, nil)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: zstd decoding failed: %v", err)
		p.errorKernel.errSend(p, Message{}, er, logWarning)
		zr.Close()
		return
	}

	zr.Close()

	message := Message{}

	err = cbor.Unmarshal(msgData, &message)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: cbor decoding failed, subject: %v, error: %v", subject, err)
		p.errorKernel.errSend(p, message, er, logError)
		return
	}

	er := fmt.Errorf("subscriberHandlerJetstream: received message: %v, from: %v, id:%v", message.Method, message.FromNode, message.ID)
	p.errorKernel.logDebug(er)
	// When spawning sub processes we can directly assign handlers to the process upon
	// creation. We here check if a handler is already assigned, and if it is nil, we
	// lookup and find the correct handler to use if available.
	if p.handler == nil {
		// Look up the method handler for the specified method.
		mh, ok := p.methodsAvailable.CheckIfExists(message.Method)
		p.handler = mh
		if !ok {
			er := fmt.Errorf("error: subscriberHandlerJetstream: no such method type: %v", p.subject.Method)
			p.errorKernel.errSend(p, message, er, logWarning)
			return
		}
	}

	_ = p.callHandler(message, thisNode)

	msg.Ack()
}
