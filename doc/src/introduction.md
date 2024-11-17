# introduction

ctrl is a Command & Control (C2) backend system for Servers, IOT and Edge platforms. Simply put, control anything.

Example use cases:

- Send shell commands or scripts to control one or many end nodes that will instruct to change config, restart services and control those systems.
- Gather data from both secure and not secure devices and systems, and transfer them encrypted in a secure way over the internet to your central system for handling those data.
- Collect metrics or monitor end nodes, then send and store the result to some ctrl instance, or pass those data's on to another ctrl instance for further handling of metrics or monitoring data.
- Distribute certificates.
- Run as a sidecar in Kubernetes for direct access to the pod.

As long as you can do something as an operator in a shell on a system you can do the same with ctrl in a secure and encrypted way to one or all end nodes (servers) in one go with one single message/command.

Ctrl is a system control tool that uses NATS as its messaging architecture. It allows you to send commands using request methods which are then executed on servers. If a receiving node is down, messages are retried based on the criteria set in their body. The results of these methods are delivered back to the
sender.

Ctrl is designed for concurrent processing and can handle multiple messages independently, even if some processes are slow or fail. It's compatible with various host OSs and systems including cloud containers, Raspberry Pi, and others with an installed operating system. Ctrl supports most major architectures such as x86, amd64, arm64, ppc64, and can run on operating systems like Linux, OSX, Windows.
