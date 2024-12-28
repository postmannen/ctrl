# Hello messages

All nodes can send hello messages to inform that they are up. The interval between sending a hello message can be set with the `START_PUB_HELLO` environment variable.

Hello messages are sent to the node with the name **central**. When a hello message are received on central, information with the time and node name will be stored in the **ctrl data folder**

## Public keys

ctrl nodes can use ed25519 keys for signing messages, so each ctrl instance will generate a public and private key pair on startup. The public keys are sent to the central server with the hello messages.

To read more about signing keys here: [signing keys](./core_signing_keys.md)
