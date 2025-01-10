# Introduction

ctrl is a Command & Control (C2) backend system for Servers, IOT and Edge platforms. Simply put, control anything like a Fleet Manager.

In operations ctrl can be used to send a set of commands to execute to a node, a group of nodes, or all nodes. It can also be used to gather operational data like logs and metrics from all the ctrl nodes.

In forensics ctrl can be used to execute commands with tools to gather information needed. If the tools are not already present on an end ctrl node, ctrl can be used to copy the tools to that node, and then ctrl can run them to gather information.

</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/core-messaging.svg?raw=true" /></p>
</body>

## Usecases

Some example usecases are:

- Send shell commands or scripts to control one or many end nodes that will instruct to change config, restart services and control those systems.
- Gather data from both secure and not secure devices and systems, and transfer them encrypted in a secure way over the internet to your central system for handling those data.
- Collect metrics or monitor end nodes, then send and store the result to some ctrl instance, or pass those data's on to another ctrl instance for further handling of metrics or monitoring data.
- Distribute certificates.
- Run as a sidecar in Kubernetes for direct access to the pod.

Ctrl is a versatile tool that enables users to control their systems with precision and efficiency. It's designed as a fleet manager, leveraging NATS as its messaging architecture, allowing for secure and encrypted communication between an operator and one or more servers.

With Ctrl, you can accomplish tasks that you typically perform on a shell in a system - all with simple messages, using Ctrl's request methods. This feature allows for the execution of commands on remote servers, even if they are down at the time of sending. Messages will be retried based on the specified criteria in their body.

Ctrl is built to handle multiple messages independently, by utilizing Go, the programming languages builtin concurrency. It can handle tasks such as executing a slow process without affecting other processes or systems. If a process fails, Ctrl ensures that it doesn't affect other operations, providing a reliable and robust system control solution.

Furthermore, Ctrl is highly compatible with various host operating systems. It supports cloud containers, Raspberry Pi, and any other device with an installed operating system. It can run on a variety of architectures such as x86, amd64, arm64, ppc64, making it a versatile tool for system
administration.

Ctrl is compatible with most major operating systems including Linux, OSX, and Windows, giving users the flexibility to use Ctrl across different platforms.
