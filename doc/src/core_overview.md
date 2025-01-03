# Core overview

This is just a quick introduction to the core concepts of ctrl. Details can be found by reading further in the **Core ctrl** section.

## Concept

In ctrl all nodes are treated equally, and there is no concept of client/server. There are just nodes that can send messages to other nodes, and the messages contains commands of what to do on the node that receives the message.

<style>
img {
  background-color: #FFFFFF;
}
</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/core-messaging.svg?raw=true" /></p>
</body>

A message can be sent from a node to one, many, groups or all nodes with a command to execute, and the result of when the command is done will be sent back to where the message originated.

## Central

There should be at least one node acting as the **central** in each environment. The central node will have functionality like:

- Audit logging of all commands sent and executed on nodes.
- Handling and distributing keys for signing messages.
- Handling and distributing Access Lists (ACL) for authorizing messages.
- General error logging.
