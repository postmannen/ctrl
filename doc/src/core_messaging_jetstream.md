# Jetstream

ctrl takes benefit of some of the streaming features of NATS Jetstream. In short, Jetstream will keep the state of the message queues on the nats-server, whereas with normal ctrl messaging the state of all the messages are kept on and is the responsibility of the node publishing the message.

With Jetstream some cool posibilities with ctrl become possible. A couple of examples are:

- Use ctrl in a github runner, and make messages available to be consumed even after the runner have stopped.
- Broadcast message to all nodes.

## General use

To use Jetstream instead of regular NATS messaging, put the **node name** of the node that is supposed to consume the message in the **jetstreamToNode** field.

```yaml
---
- toNodes:
    - mynode1
  jetstreamToNode: mynode1
  method: cliCommand
  methodArgs:
    - /bin/bash
    - -c
    - |
      tree
  methodTimeout: 3
  replyMethod: console
  ACKTimeout: 0
```

The request will then be published with NATS Jetstream to all nodes registered on the nats-server. The reply with the result will be sent back as a normal NATS message (not Jetstream).

## Broadcast

A message can be broadcasted to all nodes by using the value **all** with jetstreamToNode field of the message like the example below.

```yaml
---
- toNodes:
    - all
  jetstreamToNode: all
  method: cliCommand
  methodArgs:
    - /bin/bash
    - -c
    - |
      tree
  methodTimeout: 3
  replyMethod: console
  ACKTimeout: 0
```

The request will then be published with NATS Jetstream to all nodes registered on the nats-server. The reply with the result will be sent back as a normal NATS message (not Jetstream).

## Specify more subjects to consume with streams

More subject can be specified by using the flag `jetstreamsConsume` or env variable `JETSTREAMS_CONSUME` .

Example:

```env
JETSTREAMS_CONSUME=updates,restart,indreostfold
```
