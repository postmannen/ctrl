# ctrl as tcp forwarder for ssh

ctrl can be used to forward a TCP stream between two nodes. The Individual TCP packet will be read from a TCP listener on a node, put into a standard ctrl message, the message are then sent to the destination node using NATS PUB/SUB where the TCP packet is extracted, and written to the TCP connection.

In the follwing example we have two nodes, where we can think of **node1** as the local node running on your computer, and node2 are the remote node running on a server somewhere.

We want to connect with ssh to node2, but it is not directly available from the network where the local node1 resides, so we use ctrl as a proxy server and forward the ssh tcp stream.

</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/usecase-portforward-ssh.svg?raw=true" /></p>
</body>

## Steps

1. Create the message with forwarding details, and copy iy into **node1's readfolder**.

```yaml
---
- toNodes:
    - node1                 # The source node where we start the tcp listener
  method: portSrc           # The ctrl tcp forward method
  methodArgs:
    - node2                 # The destination node who connects to the actual endpoint
    - 192.168.99.1:22       # The ip address and port we want to connect to from endpoint.
    - :10022                # The local port to start the listener on node1
    - 30                    # How many seconds the tcp forwarder should be active
  methodTimeout: 30         # Same as above, but at the method level. Set them to the same.
  replyMethod: console      # Do logging to console on node1
  ACKTimeout: 0             # No ACK'in of messages. Fire and forget.
```

1. On **node1** a process with a TCP listener will be started at **0.0.0.0:10022**.

1. The process on **node1** will then send a message to **node2** to start up a process and connect to the endpoint defined in the **methodArgs**.

1. The forwarding are now up running, and we can use a ssh client to connect to the endpoint which are now forwarded the **node1** on port **10022**.

```bash
ssh -p 10022 user@localhost
```

The forwarding will automatically end after the timeperiod specified, which in this example is 30 seconds.
