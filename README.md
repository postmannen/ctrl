# Ctrl

ctrl is a Command & Control (C2) backend system for Servers, IOT and Edge platforms. Simply put, control anything.

Example use cases:

- Send shell commands or scripts to control one or many end nodes that will instruct to change config, restart services and control those systems.
- Gather data from both secure and not secure devices and systems, and transfer them encrypted in a secure way over the internet to your central system for handling those data.
- Collect metrics or monitor end nodes, then send and store the result to some ctrl instance, or pass those data's on to another ctrl instance for further handling of metrics or monitoring data.
- Distribute certificates.
- Run as a sidecar in Kubernetes for direct access to the pod.

As long as you can do something as an operator in a shell on a system you can do the same with ctrl in a secure and encrypted way to one or all end nodes (servers) in one go with one single message/command.

## Features

| Feature |
|---------|
|Run bash commands or complete scripts in your prefered scripting language (bash, python, powershell..)|
|Read and write to all files on the node|
|Copy files between nodes with builtin download manager in case of connection issues|
|Forward tcp connections between nodes, or use as a tcp proxy|
|ACL's to restric who can do what like who can send to which nodes, and what commands a node are allowed to execute on another node|
|Broadcast the same message to all nodes to for example update config files on all nodes|
|Create groups of nodes, for granular control|
|Graphical view of all nodes and their connections|
|Builtin Http Get method to get the content of web pages|
|Write the result of a command executed to a file, database, or use the result in a next method to craft a new message to send to a next node|
|Schedule tasks to run on a recurring basis, like cron jobs|
|Audit log of all actions taken, and commands executed|
|Centralized error logging and reporting|
|Run as a sidecar in Kubernetes for direct access to the pod|
|Start as a runner in github actions to control your infrastructure with a Github repository as the source of truth. Control nodes with instructions commited to a repository, or as a part your CI/CD pipeline|
|Detailed logs of when a node or host was last seen online|
|Tail log files, and send the output to a central log server to directly inject the output into a database or file|
|Signature verification of messages to ensure the message is authentic, and the sender is who they claim to be|
|All traffic is encrypted with TLS|
|All communication is outbound, so no need to open ports for incoming connections|
|And more...|

## Doc

The complete documentation can be found here [https://postmannen.github.io/ctrl/introduction.html](https://postmannen.github.io/ctrl/introduction.html)

## Example

An example of a **request** message to copy into ctrl's **readfolder**.

## Quick start

Start up a local nats message broker

```bash
docker run -p 4444:4444 nats -p 4444
```

Create a ctrl docker image.

```bash
git clone git@github.com:postmannen/ctrl.git
cd ctrl
docker build -t ctrl:test1 .
mkdir -p testrun/readfolder
cd testrun
```

create a .env file

```bash
cat << EOF > .env
NODE_NAME="node1"
BROKER_ADDRESS="127.0.0,1:4444"
ENABLE_DEBUG=1
START_PUB_HELLO=60
IS_CENTRAL_ERROR_LOGGER=0
EOF
```

Start the ctrl container.

```bash
docker run --env-file=".env" --rm -ti -v $(PWD)/readfolder:/app/readfolder ctrl:test1
```

Prepare and send a message.

```yaml
cat << EOF > msg.yaml
---
- toNodes:
    - node1
  method: cliCommand
  methodArgs:
    - "bash"
    - "-c"
    - |
      echo "some config line" > /etc/my-service-config.1
      echo "some config line" > /etc/my-service-config.2
      echo "some config line" > /etc/my-service-config.3
      systemctl restart my-service

  replyMethod: none
  ACKTimeout: 0
EOF

cp msg.yaml readfolder
```

### Input methods

New Request Messages in Json/Yaml format can be injected by the user to ctrl in the following ways:

- **Unix Socket**. Use for example netcat or another tool to deliver new messages to a socket like `nc -U tmp/ctrl.sock < msg.yaml`.
- **Read Folder**. Write/Copy messages to be delivered to the `readfolder` of ctrl.
- **TCP Listener**, Use for example netcat or another tool to deliver new messages a TCP Listener like `nc localhost:8888 < msg.yaml`.

### Error messages from nodes

- Error messages will be sent back to the central error handler and the originating node upon failure.

The error logs can be read on the central server in the directory `<ctrl-home>/data/errorLog`, and in the log of the instance the source node. You can also create a message to read the errorlog if you don't have direct access to the central server.

### Flags and configuration file

ctrl supports both the use of flags with env variables. An .env file can also be used.

### Schema for the messages to send into ctrl via the API's

|Field Name | Value Type | Description|
|-----|-------|------------|
|toNode | `string` | A single node to send a message to|
|toNodes | `string array` | A comma separated list of nodes to send a message to|
|jetstreamToNode| `string array` | JetstreamToNode, the topic used to prefix the stream name with the format NODES.\<JetstreamToNode> |
|method | `string` | The request method to use |
|methodArgs | `string array` | The arguments to use for the method |
|replyMethod | `string` | The method to use for the reply message |
|replyMethodArgs | `string array` | The arguments to use for the reply method |
|ACKTimeout | `int` | The time to wait for a received acknowledge (ACK). 0 for no acknowledge|
|retries | `int` | The number of times to retry if no ACK was received |
|replyACKTimeout | `int` | The timeout to wait for an ACK message before we retry |
|replyRetries | `int` | The number of times to retry if no ACK was received for repply messages |
|methodTimeout | `int` | The timeout in seconds for how long we wait for a method to complete |
|replyMethodTimeout | `int` | The timeout in seconds for how long we wait for a method to complete for repply messages |
|directory | `string` | The directory for where to store the data of the repply message |
|fileName | `string` | The name of the file for where we store the data of the reply message |
|schedule | [int type value for interval in seconds, int type value for total run time in seconds] | Schedule a message to re run at interval |

### Request Methods

| Method name| Description|
|------------|------------|
|opProcessList | Get a list of the running processes|
|opProcessStart | Start up a process|
|opProcessStop | Stop a process|
|cliCommand | Will run the command given, and return the stdout output of the command when the command is done|
|cliCommandCont | Will run the command given, and return the stdout output of the command continously while the command runs|
|tailFile | Tail log files on some node, and get the result for each new line read sent back in a reply message|
|httpGet | Scrape web url, and get the html sent back in a reply message|
|hello | Send Hello messages|
|copySrc| Copy a file from one node to another node|
|errorLog | Method for receiving error logs for Central error logger|
|none | Don't send a reply message|
|console | Print to stdout or stderr|
|fileAppend | Append to file, can also write to unix sockets|
|file | Write to file, can also write to unix sockets|
|portSrc | Forward tcp conntections between ctrl nodes|

## History

ctrl is the continuation of the code I earlier wrote for RaaLabs called Steward. The original repo was public with a MIT license, but in October 2023 the original repo was made private, and are no longer avaialable to the public. The goal of this repo is to provide an actively maintained, reliable and stable version. This is also a playground for myself to test out ideas an features for such a service as described earlier.

This started out as an idea I had for how to control infrastructure.  This is the continuation of the same idea, and a project I'm working on free of charge in my own spare time, so please be gentle :)

NB: Filing of issues and bug fixes are highly appreciated. Feature requests will genereally not be followed up simply because I don't have the time to review it at this time :

Steward was written with an MIT License. With the new fork the service was renamed to ctrl and the license were changed to AGPL V3.0. More information in the [LICENSE-CHANGE.md](LICENSE-CHANGE.md) and [LICENSE](LICENSE) files.

## Disclaimer

All code in this repository are to be concidered not-production-ready, and the use is at your own responsibility and risk. The code are the attempt to concretize the idea of a purely async management system where the controlling unit is decoupled from the receiving unit, and that that we know the state of all the receiving units at all times.

Also read the license file for further details.

Expect the main branch to have breaking changes. If stability is needed, use the released packages, and read the release notes where changes will be explained.
