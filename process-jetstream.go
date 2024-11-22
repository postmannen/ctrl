package ctrl

import (
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go/jetstream"
)

// messageDeserializeAndUncompress will deserialize the ctrl message
func (p *process) messageDeserializeAndUncompress(msg jetstream.Msg) (Message, error) {

	// Variable to hold a copy of the message data.
	msgData := msg.Data()

	// If debugging is enabled, print the source node name of the nats messages received.
	headerFromNode := msg.Headers().Get("fromNode")
	if headerFromNode != "" {
		er := fmt.Errorf("info: subscriberHandlerJetstream: nats message received from %v, with subject %v ", headerFromNode, msg.Subject())
		p.errorKernel.logDebug(er)
	}

	zr, err := zstd.NewReader(nil)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: zstd NewReader failed: %v", err)
		return Message{}, er
	}
	msgData, err = zr.DecodeAll(msgData, nil)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: zstd decoding failed: %v", err)
		zr.Close()
		return Message{}, er
	}

	zr.Close()

	message := Message{}

	err = cbor.Unmarshal(msgData, &message)
	if err != nil {
		er := fmt.Errorf("error: subscriberHandlerJetstream: cbor decoding failed, subject: %v, error: %v", msg.Subject(), err)
		return Message{}, er
	}

	return message, nil
}

// messageSerializeAndCompress will serialize and compress the Message, and
// return the result as a []byte.
func (p *process) messageSerializeAndCompress(msg Message) ([]byte, error) {

	// encode the message structure into cbor
	bSerialized, err := cbor.Marshal(msg)
	if err != nil {
		er := fmt.Errorf("error: messageDeliverNats: cbor encode message failed: %v", err)
		p.errorKernel.logDebug(er)
		return nil, er
	}

	// Compress the data payload if selected with configuration flag.
	// The compression chosen is later set in the nats msg header when
	// calling p.messageDeliverNats below.

	bCompressed := p.server.zstdEncoder.EncodeAll(bSerialized, nil)

	return bCompressed, nil
}
