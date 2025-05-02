# Introduction

ctrl is a Command & Control (C2) backend system for Servers, IOT and Edge platforms. Simply put, control anything with your own Fleet Manager.

ctrl lets you control your infrastructure by sending commands to be executed on an endpoint, a group of endpoints. A command can be a shell command, a script, or a series of commands based on an template where you pull the input arguments from an API or a github repository (IAC).  

It can be used to gather data like logs and metrics from all endpoints in secure way.

For forensics and monitoring ctrl can be used to execute commands that run tools and binaries to gather the information needed. ctrl will then deliver the data back to your SIEM or monitoring platform.

If the tools needed are not already present on an endpoint ctrl can be used to copy the tools to that node with it's builtin distribution system to copy files.

ctrl can be fully managed and used to extend the possibilites with your existing CI/CD pipelines to extend the possibilities of your IAC.

ctrl's code base are open sourced, and is hosted on github at [https://github.com/postmannen/ctrl](https://github.com/postmannen/ctrl).

</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/core-messaging.svg?raw=true" /></p>
</body>

## Usecases

Some example usecases are:

- Send shell commands or scripts to control one or many end nodes that will instruct to change config, restart services and control those systems. Scripts can be written in any language like python, bash, powershell, as long as it us supported by the endpoint.
- Gather data like metrics or logs from both secure and not secure devices and systems, and transfer the data encrypted in a secure way over the internet to your central monitoring platforms.
- Distribute certificates.
- Run as a sidecar in Kubernetes for direct access to the pod.

Ctrl is a versatile tool that enables users to control their whole infrastructure. It's designed as a fleet manager, leveraging NATS as its messaging architecture, allowing for secure and encrypted communication between an operator and one or more servers.

With Ctrl, you can accomplish tasks that you typically perform on a shell in a system - all with simple messages, using Ctrl's request methods. This allows for the execution of commands on remote servers, even if they are down at the time of sending. Messages will be retried based on the specified criteria set for that request.

Ctrl is built to handle multiple messages independently and concurrently. It can handle tasks such as executing a slow process without affecting other processes or systems. If a process fails, Ctrl ensures that it doesn't affect other operations, providing a reliable and robust system control solution.

Since Ctrl is written in Go, it is compatible with most host operating systems. It supports  Linux, Windows, Mac OS, containers, or more or less any device with an installed operating system. It can run on most architectures such as x86, amd64, arm64, ppc64, making it a versatile tool for system administration.
