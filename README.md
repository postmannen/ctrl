# Ctrl

Ctrl is a fork of Steward which I wrote for Raalabs. The intention of this fork is to have a repo for bug fixes and do updates. The fork was renamed from Steward to ctrl to avoid possible naming conflicts.

Steward was written with an MIT License. With the new fork the service was renamed to ctrl and the license were changed to AGPL V3.0. More information in the [LICENSE-CHANGE.md](LICENSE-CHANGE.md) and [LICENSE](LICENSE) files.

## Intro

ctrl is a Command & Control backend system for Servers, IOT and Edge platforms. Simply put, control anything that runs an operating system.

Example use cases:

- Send command to one or many end nodes that will instruct to run scripts or a series of shell commands to change config, restart services and control those systems.
- Gather IOT/OT data from both secure and not secure devices and systems, and transfer them encrypted in a secure way over the internet to your central system for handling those data.
- Collect metrics or monitor end nodes and store the result on a central ctrl instance, or pass those data on to another central system for handling metrics or monitoring data.
- Distribute certificates.

As long as you can do something as an operator on in a shell on a system you can do the same with ctrl in a secure way to one or all end nodes (servers) in one go with one single message/command.

- [Ctrl](#ctrl)
  - [Intro](#intro)
  - [Example](#example)
  - [Overview](#overview)
    - [Example of message flow](#example-of-message-flow)
  - [Inspiration](#inspiration)
  - [Why ctrl was created](#why-ctrl-was-created)
  - [Publishing and Subscribing processes](#publishing-and-subscribing-processes)
    - [Publisher](#publisher)
    - [Subscriber](#subscriber)
    - [Load balancing](#load-balancing)
  - [Terminology](#terminology)
  - [Features](#features)
    - [Input methods](#input-methods)
    - [Error messages from nodes](#error-messages-from-nodes)
    - [Message handling and threads](#message-handling-and-threads)
    - [Timeouts and retries for requests](#timeouts-and-retries-for-requests)
      - [RetryWait](#retrywait)
    - [Flags and configuration file](#flags-and-configuration-file)
    - [Schema for the messages to send into ctrl via the API's](#schema-for-the-messages-to-send-into-ctrl-via-the-apis)
    - [Nats messaging timeouts](#nats-messaging-timeouts)
    - [Compression of the Nats message payload](#compression-of-the-nats-message-payload)
    - [Serialization of messages sent between nodes](#serialization-of-messages-sent-between-nodes)
    - [startup folder](#startup-folder)
      - [General functionality](#general-functionality)
      - [How to send the reply to another node](#how-to-send-the-reply-to-another-node)
      - [Use local as the toNode nodename](#use-local-as-the-tonode-nodename)
      - [method timeout](#method-timeout)
        - [Example for method timeout](#example-for-method-timeout)
      - [Schedule a Method in a message to be run several times](#schedule-a-method-in-a-message-to-be-run-several-times)
    - [Request Methods](#request-methods)
      - [REQOpProcessList](#reqopprocesslist)
      - [REQOpProcessStart](#reqopprocessstart)
      - [REQOpProcessStop](#reqopprocessstop)
      - [REQCliCommand](#reqclicommand)
      - [REQCliCommandCont](#reqclicommandcont)
      - [REQTailFile](#reqtailfile)
      - [REQHttpGet](#reqhttpget)
      - [REQHello](#reqhello)
      - [REQCopySrc](#reqcopysrc)
      - [REQErrorLog](#reqerrorlog)
      - [REQNone](#reqnone)
      - [REQToConsole](#reqtoconsole)
      - [REQToFileAppend](#reqtofileappend)
        - [Write to a socket file](#write-to-a-socket-file)
      - [REQToFile](#reqtofile)
        - [Write to socket file](#write-to-socket-file)
      - [ReqCliCommand as reply method](#reqclicommand-as-reply-method)
    - [Errors reporting](#errors-reporting)
    - [Prometheus metrics](#prometheus-metrics)
    - [Security / Authorization](#security--authorization)
      - [Authorization based on the NATS subject](#authorization-based-on-the-nats-subject)
      - [Authorization based on the message payload](#authorization-based-on-the-message-payload)
        - [Key registration on Central Server](#key-registration-on-central-server)
        - [Key distribution to nodes](#key-distribution-to-nodes)
        - [Management of the keys on the central server](#management-of-the-keys-on-the-central-server)
          - [REQKeysAllow](#reqkeysallow)
          - [REQKeysDelete](#reqkeysdelete)
        - [Acl updates](#acl-updates)
        - [Management of the Acl on the central server](#management-of-the-acl-on-the-central-server)
          - [REQAclAddCommand](#reqacladdcommand)
          - [REQAclDeleteCommand](#reqacldeletecommand)
          - [REQAclDeleteSource](#reqacldeletesource)
          - [REQAclGroupNodesAddNode](#reqaclgroupnodesaddnode)
          - [REQAclGroupNodesDeleteNode](#reqaclgroupnodesdeletenode)
          - [REQAclGroupNodesDeleteGroup](#reqaclgroupnodesdeletegroup)
          - [REQAclGroupCommandsAddCommand](#reqaclgroupcommandsaddcommand)
          - [REQAclGroupCommandsDeleteCommand](#reqaclgroupcommandsdeletecommand)
          - [REQAclGroupCommandsDeleteGroup](#reqaclgroupcommandsdeletegroup)
          - [REQAclExport](#reqaclexport)
          - [REQAclImport](#reqaclimport)
    - [Other](#other)
  - [Howto](#howto)
    - [Options for running](#options-for-running)
    - [How to Run](#how-to-run)
      - [Nats-server](#nats-server)
      - [Build ctrl from source](#build-ctrl-from-source)
        - [Get it up and running](#get-it-up-and-running)
        - [Send messages with ctrl](#send-messages-with-ctrl)
        - [Example for starting ctrl with some more options set](#example-for-starting-ctrl-with-some-more-options-set)
      - [Nkey Authentication](#nkey-authentication)
      - [nats-server (the message broker)](#nats-server-the-message-broker)
        - [Nats-server config with nkey authentication example](#nats-server-config-with-nkey-authentication-example)
      - [Nkey from ED25519 SSH key](#nkey-from-ed25519-ssh-key)
    - [How to send a Message](#how-to-send-a-message)
      - [Send to socket with netcat](#send-to-socket-with-netcat)
      - [Sending a command from one Node to Another Node](#sending-a-command-from-one-node-to-another-node)
        - [Example JSON for appending a message of type command into the `socket` file](#example-json-for-appending-a-message-of-type-command-into-the-socket-file)
        - [Specify more messages at once do](#specify-more-messages-at-once-do)
        - [Send the same message to several hosts by using the toHosts field](#send-the-same-message-to-several-hosts-by-using-the-tohosts-field)
        - [Tail a log file on a node, and save the result of the tail centrally at the directory specified](#tail-a-log-file-on-a-node-and-save-the-result-of-the-tail-centrally-at-the-directory-specified)
        - [Example for deleting the ringbuffer database and restarting ctrl](#example-for-deleting-the-ringbuffer-database-and-restarting-ctrl)
  - [Concepts/Ideas](#conceptsideas)
    - [Naming](#naming)
      - [Subject](#subject)
        - [Complete subject example](#complete-subject-example)
  - [History](#history)
  - [Disclaimer](#disclaimer)

## Example

An example of a **request** message to copy into ctrl's **readfolder**.

```json
[
    {
        "directory":"/var/cli/command_result/",
        "fileName": "some-file-name.result",
        "toNode": "ship1",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","sleep 5 & tree ./"],
        "replyMethod":"REQToFileAppend",
        "ACKTimeout":5,
        "retries":3,
        "replyACKTimeout":5,
        "replyRetries":3,
        "methodTimeout": 10
    }
]
```

If the receiver `toNode` is down when the message was sent, it will be **retried** until delivered within the criterias set for `timeouts` and `retries`. The state of each message processed is handled by the owning ctrl instance where the message originated, and no state about the messages are stored in the NATS message broker.

Since the initial connection from a ctrl node is outbound towards the central NATS message broker no inbound firewall openings are needed.

## Overview

Send Commands with Request Methods to control your servers by passing a messages. If a receiving node is down, the message will be retried with the criterias set within the message body. The result of the method executed will be delivered back to you from the node you sent it from.

ctrl uses **NATS** as message passing architecture for the commands back and forth from nodes. Delivery is guaranteed within the criterias set. All of the processes in the system are running concurrently. If some process is slow or fails it will not affect the handling and delivery of the other messages in the system.

**ctrl** can be run on almost any host operating system, containers living in the cloud somewhere, a Rapsberry Pi, or something else that needs to be controlled that have an operating system installed.

ctrl can be compiled to run on most major architectures like **x86**, **amd64**,**arm64**, **ppc64** and more, with for example operating systems like **Linux**, **OSX**, **Windows**.

### Example of message flow

![message flow](doc/message-flow.svg)

## Inspiration

The idea for how to handle processes, messages and errors are based on Joe Armstrongs idea behind Erlang described in his Thesis <https://erlang.org/download/armstrong_thesis_2003.pdf>.

Joe's document describes how to build a system where everything is based on sending messages back and forth between processes in Erlang, and where everything is done concurrently.

I used those ideas as inspiration for building a fully concurrent system to control servers or container based systems by passing  messages between processes asynchronously to execute methods, handle errors, or handle the retrying if something fails.

ctrl is written in the programming language Go with NATS as the message broker.

## Why ctrl was created

With existing solutions there is often either a push or a pull kind of setup to control the nodes.

In a push setup the commands to be executed is pushed to the receiver, but if a command fails because for example a broken network link it is up to you as an administrator to detect those failures and retry them at a later time until it is executed successfully.

In a pull setup an agent is installed at the Edge unit, and the configuration or commands to execute locally are pulled from a central repository. With this kind of setup you can be pretty certain that sometime in the future the node will reach it's desired state, but you don't know when. And if you want to know the current state you will need to have some second service which gives you that information.

In it's simplest form the idea about using an event driven system as the core for management of Edge units is that the sender/publisher are fully decoupled from the receiver/subscriber. We can get an acknowledge message if a message is received or not, and with this functionality we will at all times know the current state of the receiving end.

## Publishing and Subscribing processes

All parts of the system like processes, method handlers, messages, error handling are running concurrently.

If one process hangs on a long running message method it will not affect the rest of the system.

### Publisher

1. A message in valid format is appended to one of the input methods. Available inputs are Unix Socket listener, TCP listener, and File Reader (**readfolder**).
2. The message is picked up by the system.
3. The method type of the message is checked, a subject is created based on the content of the message,  and a publisher process to handle the message type for that specific receiving node is started if it does not exist.
4. The message is then serialized to binary format, and sent to the subscriber on the receiving node.
5. If the message is expected to be ACK'ed by the subcriber then the publisher will wait for an ACK if the message was delivered. If an ACK was not received within the defined timeout the message will be resent. The amount of retries are defined within the message.

### Subscriber

1. The receiving end will need to have a subscriber process started on a specific subject and be allowed handle messages from the sending nodes to execute the method defined in the message.
2. When a message have been received, a handler for the method type specified in the message will be executed.
3. If the output of the method called is supposed to be returned to the publiser it will do so by using the replyMethod specified.

### Load balancing

ctrl instances with the same **Nodename** will automatically load balance the handling of messages on a given subject, and any given message will only be handled once by one instance.

## Terminology

- **Node**: An instance of **ctrl** running on an operating system that have network available. This can be a server, a cloud instance, a container, or other.
- **Process**: A message handler that knows how to handle messages of a given subject concurrently.
- **Message**: A message sent from one **ctrl** node to another.

## Features

### Input methods

New Request Messages in Json/Yaml format can be injected by the user to ctrl in the following ways:

- **Unix Socket**. Use for example netcat or another tool to deliver new messages to a socket like `nc -U tmp/ctrl.sock < msg.yaml`.
- **Read Folder**. Write/Copy messages to be delivered to the `readfolder` of ctrl.
- **TCP Listener**, Use for example netcat or another tool to deliver new messages a TCP Listener like `nc localhost:8888 < msg.yaml`.

### Error messages from nodes

- Error messages will be sent back to the central error handler and the originating node upon failure.

```log
Tue Sep 21 09:17:55 2021, info: toNode: ship2, fromNode: central, method: REQOpProcessList: max retries reached, check if node is up and running and if it got a subscriber for the given REQ type
```

The error logs can be read on the central server in the directory `<ctrl-home>/data/errorLog`, and in the log of the instance the source node.

### Message handling and threads

- The handling of all messages is done by spawning up a process for handling the message in it's own thread. This allows us to keep the state of each **individual message level**  both in regards to ACK's, error handling, send retries, and reruns of methods for a message if the first run was not successful.

- Processes for handling messages on a host can be **restarted** upon **failure**, or asked to just terminate and send a message back to the operator that something have gone seriously wrong. This is right now just partially implemented to test that the concept works, where the error action is  **action=no-action**.

- Publisher Processes on a node for handling new messages for new nodes will automatically be spawned when needed if it does not already exist.

- If enabled, messages not fully processed or not started yet will be automatically rehandled if the service is restarted since the current state of all the messages being processed are stored on the local node in a **key value store** until they are finished.

- All messages processed by a publisher will be written to a log file after they are processed, with all the information needed to recreate the same message if needed, or it can be used for auditing.

- All handling down to the process and message level are handled concurrently. So if there are problems handling one message sent to a node on a subject it will not affect the messages being sent to other nodes, or other messages sent on other subjects to the same host.

- Message types of both **ACK** and **NACK**, so we can decide if we want or don't want an Acknowledge if a message was delivered succesfully.
Example: We probably want an **ACK** when sending some **REQCLICommand** to be executed, but we don't care for an acknowledge **NACK** when we send an **REQHello** event.
If a message are **ACK** or **NACK** type are defined by the value of the **ACKTimeout** for each individual message:

  1. **ACKTimeout** set to 0 will make the message become a **NACK** message.
  2. **ACKTimeout** set to >=1 will make the message become an **ACK** message.

### Timeouts and retries for requests

- Default timeouts to wait for ACK messages and max attempts to retry sending a message are specified upon startup. This can be overridden on the message level.

- Timeouts can be specified on both the **message** delivery, and the **method**.
  - A message can have a timeout used for used for when to resend and how many retries.
  - If the method triggers a shell command, the command can have its own timeout, allowing process timeout for long/stuck commands, or for telling how long the command is supposed to run.

Example of a message with timeouts set:

```json
[
    {
        "directory":"/some/result/directory/",
        "fileName":"my-syslog.log",
        "toNode": "ship2",
        "methodArgs": ["bash","-c","tail -f /var/log/syslog"],
        "replyMethod":"REQToFileAppend",
        "method":"REQCliCommandCont",
        "ACKTimeout":3,
        "retries":3,
        "methodTimeout": 60
    }
]
```

In the above example, the values set meaning:

- **ACKTimeout** : Wait 3 seconds for an **ACK** message.
- **retries** : If an **ACK** is not received, retry sending the message 3 times.
- **methodTimeout** : Let the bash command `tail -f ./tmp.log` run for 60 seconds before it is terminated.

If no timeout are specified in a message the defaults specified in the **etc/config.yaml** are used.

#### RetryWait

Instead of solely depending in the ack timeout the **RetryWait** can be used. RetryWait specifies the time in seconds to wait between retries.

```json
[
    {
        "directory":"/some/result/directory/",
        "fileName":"my-syslog.log",
        "toNode": "ship2",
        "methodArgs": ["bash","-c","tail -f /var/log/syslog"],
        "replyMethod":"REQToFileAppend",
        "method":"REQCliCommandCont",
        "ACKTimeout":3,
        "RetryWait":10,
        "retries":3,
        "methodTimeout": 60
    }
]
```

This is the same as the previous example, but it will also wait another 10 seconds after it noticed that an ACK was not received before the message is retried.

The flow will be like this:

- Send message.
- Wait 3 seconds for an Acknowledge from the destination node.
- If an Acknowledge was not received, wait another 10 seconds before the message is retried.
- Retry sending message.

### Flags and configuration file

ctrl supports both the use of flags with env variables. An .env file can also be used.

### Schema for the messages to send into ctrl via the API's

- toNode : `string`
- toNodes : `string array`
- method : `string`
- methodArgs : `string array`
- replyMethod : `string`
- replyMethodArgs : `string array`
- ACKTimeout : `int`
- retries : `int`
- replyACKTimeout : `int`
- replyRetries : `int`
- methodTimeout : `int`
- replyMethodTimeout : `int`
- directory : `string`
- fileName : `string`
- schedule : [int type value for interval in seconds, int type value for total run time in seconds]

### Nats messaging timeouts

The various timeouts for the messages can be controlled via the configuration file or flags.

If the network media is a high latency like satellite links, it will make sense to adjust the client timeout to reflect the latency

```text
  -natsConnOptTimeout int
        default nats client conn timeout in seconds (default 20)
```

The interval in seconds the nats client should try to reconnect to the nats-server if the connection is lost.

```text
  -natsConnectRetryInterval int
        default nats retry connect interval in seconds. (default 10)
```

Jitter values.

```text
  -natsReconnectJitter int
        default nats ReconnectJitter interval in milliseconds. (default 100)
  -natsReconnectJitterTLS int
        default nats ReconnectJitterTLS interval in seconds. (default 5)
```

### Compression of the Nats message payload

You can choose to enable compression of the payload in the Nats messages.

```text
  -compression string
        compression method to use. defaults to no compression, z = zstd, g = gzip. Undefined value will default to no compression
```

When starting a ctrl instance with compression enabled it is the publishing of the message payload that is compressed.

The subscribing instance of ctrl will automatically detect if the message is compressed or not, and decompress it if needed.

With other words, ctrl will by default receive and handle both compressed and uncompressed messages, and you decide on the publishing side if you want to enable compression or not.

### Serialization of messages sent between nodes

ctrl support two serialization formats when sending messages. By default it uses the Go spesific **GOB** format, but serialization with **CBOR** are also supported.

A benefit of using **CBOR** is the size of the messages when transferred.

To enable **CBOR** serialization either start **ctrl** by setting the serialization flag:

```bash
./ctrl -serialization="cbor" <other flags here...>
```

### startup folder

#### General functionality

Messages can be automatically scheduled to be read and executed at startup of ctrl.

A folder named **startup** will be present in the working directory of ctrl. To inject messages at startup, put them here.

Messages put in the startup folder will not be sent to the broker but handled locally, and only (eventually) the reply message from the Request Method called will be sent to the broker.

#### How to send the reply to another node

Normally the **fromNode** field is automatically filled in with the node name of the node where a message originated. Since messages within the startup folder is not received from another node via the normal message path we need to specify the **fromNode** field within the message for where we want the reply delivered.

As an example. If You want to place a message on the startup folder of **node1** and send the result to **node2**. Specify **node2** as the **fromNode**, and **node1** as the **toNode**

#### Use local as the toNode nodename

Since messages used in startup folder are ment to be delivered locally we can simplify things a bit by setting the **toNode** field value of the message to **local**.

```json
[
    {
        "toNode": "local",
        "fromNode": "central",
        "method": "REQCliCommand",
        "methodArgs": [
            "bash",
            "-c",
            "curl localhost:2111/metrics"
        ],
        "replyMethod": "REQToConsole",
        "methodTimeout": 10
    }
]

```

#### method timeout

We can also make the request method run for as long as the ctrl instance itself is running. We can do that by setting the **methodTimeout** field to a value of `-1`.

This can make sense if you for example wan't to continously ping a host, or continously run a script on a node.

##### Example for method timeout

```json
[
    {
        "toNode": "ship1",
        "fromNode": "central",
        "method": "REQCliCommandCont",
        "methodArgs": [
            "bash",
            "-c",
            "nc -lk localhost 8888"
        ],
        "replyMethod": "REQToConsole",
        "methodTimeout": 10,
    }
]
```

This message is put in the `./startup` folder on **node1**.</br>
We send the message to ourself, hence specifying ourself in the `toNode` field.</br>
We specify the reply messages with the result to be sent to the console on **central** in the `fromNode` field.</br>
In the example we start a TCP listener on port 8888, and we want the method to run for as long as ctrl is running. So we set the **methodTimeout** to `-1`.</br>

#### Schedule a Method in a message to be run several times

Methods with their MethodArgs can be scheduled to be run any number of times. Meaning you can send the message once, and the method will be re-called at the interval specified with the **schedule** field. A max run time for the schedule must also be specified.

`schedule : [int type value for interval in seconds, int type value for total run time in seconds]`

**schedule** can also be used with messages specified in the **startup folder**.

Example below will be run each 2nd seconds, with a total run of 5 seconds:

```json
[
    {
        "toNodes": ["central"],
        "method": "REQCliCommand",
        "methodArgs": [
            "bash",
            "-c",
            "hostname && curl -v http://edgeos.raalabs.tech"
        ],
        "replyMethod": "REQToConsole",
        "ACKTimeout": 5,
        "retries": 3,
        "replyACKTimeout": 5,
        "replyRetries": 5,
        "methodTimeout": 5,
        "replyMethodTimeout": 120,
        "directory": "debug",
        "fileName": "test.txt",
        "schedule": [2,5]
    }
]
```

### Request Methods

#### REQOpProcessList

Get a list of the running processes.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"REQOpProcessList",
        "methodArgs": [],
        "replyMethod":"REQToFileAppend",
    }
]
```

#### REQOpProcessStart

Start up a process. Takes the REQ method to start as it's only argument.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"REQOpProcessStart",
        "methodArgs": ["REQHttpGet"],
        "replyMethod":"REQToFileAppend",
    }
]
```

#### REQOpProcessStop

Stop a process. Takes the REQ method, receiving node name, kind publisher/subscriber, and the process ID as it's arguments.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"REQOpProcessStop",
        "methodArgs": ["REQHttpGet","ship2","subscriber","199"],
        "replyMethod":"REQToFileAppend",
    }
]
```

#### REQCliCommand

Run CLI command on a node. Linux/Windows/Mac/Docker-container or other.

Will run the command given, and return the stdout output of the command when the command is done.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "ship2",
        "method":"REQnCliCommand",
        "methodArgs": ["bash","-c","docker ps -a"],
        "replyMethod":"REQToFileAppend",
    }
]
```

#### REQCliCommandCont

Run CLI command on a node. Linux/Windows/Mac/Docker-container or other.

Will run the command given, and return the stdout output of the command continously while the command runs. Uses the methodTimeout to define for how long the command will run.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "ship2",
        "method":"REQCliCommandCont",
        "methodArgs": ["bash","-c","docker ps -a"],
        "replyMethod":"REQToFileAppend",
        "methodTimeout":10,
    }
]
```

**NB**: A github issue is filed on not killing all child processes when using pipes <https://github.com/golang/go/issues/23019>. This is relevant for this request type.

And also a new issue registered <https://github.com/golang/go/issues/50436>

TODO: Check in later if there are any progress on the issue. When testing the problem seems to appear when using sudo, or tcpdump without the -l option. So for now, don't use sudo, and remember to use -l with tcpdump
which makes stdout line buffered. `timeout` in front of the bash command can also be used to get around the problem with any command executed.

#### REQTailFile

Tail log files on some node, and get the result for each new line read sent back in a reply message. Uses the methodTimeout to define for how long the command will run.

```json
[
    {
        "directory": "/my/tail/files/",
        "fileName": "tailfile.log",
        "toNode": "ship2",
        "method":"REQTailFile",
        "methodArgs": ["/var/log/system.log"],
        "methodTimeout": 10
    }
]
```

#### REQHttpGet

Scrape web url, and get the html sent back in a reply message. Uses the methodTimeout for how long it will wait for the http get method to return result.

```json
[
    {
        "directory": "web",
        "fileName": "web.html",
        "toNode": "ship2",
        "method":"REQHttpGet",
        "methodArgs": ["https://web.ics.purdue.edu/~gchopra/class/public/pages/webdesign/05_simple.html"],
        "replyMethod":"REQToFile",
        "ACKTimeout":10,
        "retries": 3,
        "methodTimeout": 3
    }
]
```

#### REQHello

Send Hello messages.

All nodes have the flag option to start sending Hello message to the central server. The structure of those messages looks like this.

```json
[
    {
        "toNode": "central",
        "method":"REQHello"
    }
]
```

#### REQCopySrc

Copy a file from one node to another node.

```json
[
    {
        "directory": "copy",
        "fileName": "copy.log",
        "toNodes": ["central"],
        "method":"REQCopySrc",
        "methodArgs": ["./testbinary","ship1","./testbinary-copied","500000","20","0770"],
        "methodTimeout": 10,
        "replyMethod":"REQToConsole"
    }
]
```

- toNode/toNodes, specifies what node to send the request to, and which also contains the src file to copy.
- methodArgs, are split into several fields, where each field specifies:
  - 1. SrcFullPath, specifies the full path including the name of the file to copy.
  - 2. DstNode, the destination node to copy the file to.
  - 3. DstFullPath, the full path including the name of the destination file. The filename can be different than the original name.
  - 4. SplitChunkSize, the size of the chunks to split the file into for transfer.
  - 5. MaxTotalCopyTime, specifies the maximum allowed time the complete copy should take. Make sure you set this long enough to allow the transfer to complete.
  - 6. FolderPermission, the permissions to set on the destination folder if it does not exist and needs to be created. Will default to 0755 if no value is set.

To copy from a remote node to the local node, you specify the remote nodeName in the toNode field, and the message will be forwarded to the remote node. The copying request will then be picked up by the remote node's **REQCopySrc** handler, and the copy session will then be handled from the remote node.

#### REQErrorLog

Method for receiving error logs for Central error logger.

**NB**: This is **not** to be used by users. Use **REQToFileAppend** instead.

#### REQNone

Don't send a reply message.

An example could be that you send a `REQCliCommand` message to some node, and you specify `replyMethod: REQNone` if you don't care about the resulting output from the original method.

#### REQToConsole

This is a method that can be used to get the data of the message printed to console where ctrl is running.

Default is to print to **stdout**, but printing to **stderr** can be done by setting the value of **methodArgs** to `"methodArgs": ["stderr"]`.

If used as a **replyMethod** set the **replyMethodArgs** `"replyMethodArgs": ["stderr"],`.

```json
[
    {
        "directory": "web",
        "fileName": "web.html",
        "toNode": "ship2",
        "method":"REQHttpGet",
        "methodArgs": ["https://web.ics.purdue.edu/~gchopra/class/public/pages/webdesign/05_simple.html"],
        "replyMethod":"REQToConsole",
        "ACKTimeout":10,
        "retries": 3,
        "methodTimeout": 3
    }
]
```

#### REQToFileAppend

Append the output of the reply message to a log file specified with the `directory` and `fileName` fields.

If the value of the **directory** field is not prefixed with `./` or `/` the directory structure file will be created within the **ctrl data folder** specified in the config file.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"REQOpProcessList",
        "methodArgs": [],
        "replyMethod":"REQToFileAppend",
    }
]
```

##### Write to a socket file

If there is already a file at the specified path with the specified name, and if that file is a socket, then the request method will automatically switch to socket communication and write to the socket instead of normal file writing.

#### REQToFile

Write the output of the reply message to a file specified with the `directory` and `fileName` fields, where the writing will write over any existing content of that file.

If the value of the **directory** field is not prefixed with `./` or `/` the directory structure file will be created within the **ctrl data folder** specified in the config file.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"REQOpProcessList",
        "methodArgs": [],
        "replyMethod":"REQToFile",
    }
]
```

##### Write to socket file

If there is already a file at the specified path with the specified name, and if that file is a socket, then the request method will automatically switch to socket communication and write to the socket instead of normal file writing.

#### ReqCliCommand as reply method

By using the `{{ctrl_DATA}}` you can grab the output of your initial request method, and then use it as input in your reply method.

**NB:** The echo command in the example below will remove new lines from the data. To also keep any new lines we need to put escaped **quotes** around the template variable. Like this:

- `\"{{ctrl_DATA}}\"`

Example of usage:

```json
[
    {
        "directory":"cli_command_test",
        "fileName":"cli_command.result",
        "toNode": "ship2",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"REQCliCommand",
        "replyMethodArgs": ["bash", "-c","echo \"{{ctrl_DATA}}\" > apekatt.txt"],
        "replyMethodTimeOut": 10,
        "ACKTimeout":3,
        "retries":3,
        "methodTimeout": 10
    }
]
```

Or the same using bash's herestring:

```json
[
    {
        "directory":"cli_command_test",
        "fileName":"cli_command.result",
        "toNode": "ship2",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"REQCliCommand",
        "replyMethodArgs": ["bash", "-c","cat <<< {{ctrl_DATA}} > hest.txt"],
        "replyMethodTimeOut": 10,
        "ACKTimeout":3,
        "retries":3,
        "methodTimeout": 10
    }
]
```

### Errors reporting

- Errors happening on **all** nodes will be reported back to the node(s) started with the flag `-isCentralErrorLogger` set to true.

### Prometheus metrics

- Prometheus exporters for Metrics.

### Security / Authorization

#### Authorization based on the NATS subject

Main authentication and authorization are done on the **subject level** with NATS. Each node have a unique public and private key pair, where the individual publics keys are either allowed or denied to subscribe/publish on a subject in an authorization file on the Nats-server.

#### Authorization based on the message payload

Some request types, like **REQCliCommand** also allow authorization of the message payload. The payload of the message can be checked against a list of allowed or denied commands configured in a main Access List on the central server.

With each message created a signature will also be created with the private key of the node, and the signature is then attached to the message.
NB: The keypair used for the signing of messages are a separate keypair used only for signing messages, and are not the same pair that is used for authentication with the NATS server.

The nodes will have a copy of the allowed public signing keys from the central server, and when a message is received, the signature is checked against the allowed public keys. If the signature is valid, the message is allowed to be processed further, otherwise it is denied if signature checking is enabled.

ctrl can be used either with no authorization at all, with signature checks only, or with ACL and signature checks. The features can be enabled or disabled in the **config.yaml** file.

##### Key registration on Central Server

All nodes will generate a private and a public key pair only used for signing messages. For building a complete database of all the public keys in the system and to be able to distribute them to other nodes, each node will send it's public key to the central server as the payload in the **REQHello** messages. The received keys will be stored in the central server's database.

For storing the keys on the central server two databases are involved.

- A Database for all the keys that have not been acknowledged.
- A Database for all the keys that have been acknowledged into the system with a hash of all the keys. This is also the database that gets distributed out to the nodes when they request and update

1. When a new not already registered key is received on the central server it will be added to the **NO_ACK_DB** database, and a message will be sent to the operator to permit the key to be added to the system.
2. When the operator permits the key, it will be added to the **Acknowledged** database, and the node will be removed from the Not-Acknowledged database.
3. If the key is already in the acked database no changes will be made.

If new keys are allowed into or deleted from the system, one attempt will be done to push the updated key database to all current nodes heard from in the network. If the push fails, the nodes will get the update the next time they ask for it based on the key update interval set on each node.

##### Key distribution to nodes

1. ctrl nodes will request key updates by sending a message to the central server with the **REQKeysRequestUpdate** method on a timed interval. The hash of the current keys on a node will be put as the payload of the message.
2. On the Central server the received hash will be compared with the current hash on the central server. If the hashes are equal nothing will be done, and no reply message will be sent back to the end node.
3. If the hashes are not equal a reply message of type **REQKeysDeliverUpdate** will be sent back to the end node with a copy of the acknowledged public keys database and a hash of those new keys.
4. The end node will then update it's local key database.

The interval of the updates can be controlled with it's own config or flag **REQKeysRequestUpdateInterval**

##### Management of the keys on the central server

###### REQKeysAllow

Will allow a key to be added to the system by moving the key from the **NO_ACK_DB** to the **ACK_DB**.

###### REQKeysDelete

Will remove the specified keys from the **ACK_DB**.

##### Acl updates

1. ctrl nodes will request acl updates by sending a message to the central server with the **REQAclRequestUpdate** method on a timed interval. The hash of the current Acl on a node will be put as the payload of the message.
2. On the Central server the received hash will be compared with the current hash on the central server. If the hashes are equal nothing will be done, and no reply message will be sent back to the end node.
3. If the hashes are not equal a reply message of type **REQAclDeliverUpdate** will be sent back to the end node with a copy of the Acl's database for the node the request came from. The update will also contain the new hash of the new Acl's.
4. The end node will then replace it's local Acl database with the update.

The interval of the updates can be controlled with it's own config or flag **REQAclRequestUpdateInterval**

NB: The update process is initiated by the end nodes on a timed interval. No ACL updates are initiaded from the central server.

##### Management of the Acl on the central server

Several Request methods exists for handling the management of the active Acl's on the central server.

If the element specified is prefixed with **grp_** it will be treated as a group, otherwise it will be treated as a single node or command.

Groups or nodes do not have to exist to be used with an acl. The acl will be created with the elements specifed, and if a non existing group was specified you will have an Acl that is not yet functional, but it will become functional as soon as you add elements to the group's.

###### REQAclAddCommand

Takes the methodArgs: ["host or group of hosts", "src or group of src","cmd or group of cmd"]

###### REQAclDeleteCommand

Takes the methodArgs: ["host or group of hosts", "src or group of src","cmd or group of cmd"]

###### REQAclDeleteSource

Takes the methodArgs: ["host or group of hosts", "src or group of src"]

###### REQAclGroupNodesAddNode

Takes the methodArgs: ["nodegroup name", "node name"]

###### REQAclGroupNodesDeleteNode

Takes the methodArgs: ["nodegroup name", "node name"]

###### REQAclGroupNodesDeleteGroup

Takes the methodArgs: ["nodegroup name"]

###### REQAclGroupCommandsAddCommand

Takes the methodArgs: ["commandgroup name", "command"]

###### REQAclGroupCommandsDeleteCommand

Takes the methodArgs: ["commandgroup name", "command"]

###### REQAclGroupCommandsDeleteGroup

Takes the methodArgs: ["commandgroup name"]

###### REQAclExport

Creates an export of the current Acl's database, and delivers it to the requesting node with the replyMethod specified.

###### REQAclImport

Imports the Acl given in JSON format in the first argument of the methodArgs.

### Other

- In active development.

## Howto

### Options for running

The location of the config file are given via an env variable at startup (default "./etc/).

`env CONFIG_FOLDER </myconfig/folder/here> <ctrl binary>`

The different fields and their type in the config file. The fields of the config file can also be set by providing flag values at startup. Use the `-help` flag to get all the options.

### How to Run

#### Nats-server

Download the **nats-server** from <https://github.com/nats-io/nats-server/releases/>

Or use the curl (replace the version information with wanted version):

`curl -L https://github.com/nats-io/nats-server/releases/download/vX.Y.Z/nats-server-vX.Y.Z-linux-amd64.zip -o nats-server.zip`

Unpack:

`unzip nats-server.zip -d nats-server`

Start the nats server listening on local interfaces and port 4222.

`./nats-server -D`

#### Build ctrl from source

ctrl is written in Go, so you need Go installed to compile it. You can get Go at <https://golang.org/dl/>.

Clone the repository:

`git clone https://github.com/postmannen/ctrl.git`.

Change directory and build:

```bash
cd ./ctrl/cmd/ctrl
go build -o ctrl
```

##### Get it up and running

**NB:** Remember to run the nats setup above before running the ctrl binary.

ctrl will create some directories for things like configuration file and other state files. By default it will create those files in the directory where you start ctrl. So create individual directories for each ctrl instance you want to run below.

Start up a  **central** server which will act as your master server for things like logs and authorization.

```bash
 mkdir central & cd central
 env CONFIG_FOLDER=./etc/ <path-to-ctrl-binary-here> --nodeName="central" --centralNodeName="central"
```

Start up a node that will attach to the **central** node

```bash
 mkdir ship1 & cd ship1
 env CONFIG_FOLDER=./etc/ <path-to-ctrl-binary-here> --nodeName="ship1" --centralNodeName="central"
```

You can get all the options with `./ctrl --help`

ctrl will by default create the data and config directories needed in the current folder. This can be changed by using the different flags or editing the config file.

You can also run multiple instances of ctrl on the same machine. For testing you can create sub folders for each ctrl instance, go into each folder and start ctrl. When starting each ctrl instance make sure you give each node a unique `--nodeName`.

##### Send messages with ctrl

You can now go to one of the folders for nodes started, and inject messages into the socket file `./tmp/ctrl.sock` with the **nc** tool.

Example on Mac:

`nc -U ./tmp/ctrl.sock < reqnone.msg`

Example on Linux:

`nc -NU ./tmp/ctrl.sock < reqnone.msg`

##### Example for starting ctrl with some more options set

A complete example to start a central node called `central`.

```bash
env CONFIG_FOLDER=./etc/ ./ctrl \
 -nodeName="central" \
 -defaultMessageRetries=3 \
 -defaultMessageTimeout=5 \
 -subscribersDataFolder="./data" \
 -centralNodeName="central" \
 -IsCentralErrorLogger=true \
 -subscribersDataFolder="./var" \
 -brokerAddress="127.0.0.1:4222"
```

And start another node that will be managed via central.

```bash
env CONFIG_FOLDER=./etc/ ./ctrl \
 -nodeName="ship1" \ 
 -startPubREQHello=200 \
 -centralNodeName="central" \
 -promHostAndPort=":12112" \
 -brokerAddress="127.0.0.1:4222"
```

**NB**: By default ctrl creates it's folders like `./etc`, `./var`, and `./data` in the folder you're in when you start it. If you want to run multiple instances on the same machine you should create separate folders for each instance, and start ctrl when you're in that folder. The location of the folders can also be specified within the config file.

#### Nkey Authentication

Nkey's can be used for authentication, and you use the `nkeySeedFile` flag to specify the seed file to use.

Read more in the sections below on how to generate nkey's.

#### nats-server (the message broker)

The broker for messaging is Nats-server from <https://nats.io>. Download, run it, and use the `-brokerAddress` flag on **ctrl** to point to the ip and port:

`-brokerAddress="nats://10.0.0.124:4222"`

There is a lot of different variants of how you can setup and confiure Nats. Full mesh, leaf node, TLS, Authentication, and more. You can read more about how to configure the Nats broker called nats-server at <https://nats.io/>.

##### Nats-server config with nkey authentication example

```config
port: 4222
tls {
  cert_file: "some.crt"
  key_file: "some.key"
}


authorization: {
    users = [
        {
            # central
            nkey: <USER_NKEY_HERE>
            permissions: {
                publish: {
      allow: ["some.>","errorCentral.>"]
    }
            subscribe: {
      allow: ["some.>","errorCentral.>"]
    }
            }
        }
        {
            # node1
            nkey: <USER_NKEY_HERE>
            permissions: {
                publish: {
                        allow: ["central.>"]
                }
                subscribe: {
                        allow: ["central.>","some.node1.>","some.node10'.>"]
                }
            }
        }
        {
            # node10
            nkey: <USER_NKEY_HERE>
            permissions: {
                publish: {
                        allow: ["some.node1.>","errorCentral.>"]
                }
                subscribe: {
                        allow: ["central.>"]
                }
            }
        }
    ]
}
```

The official docs for nkeys can be found here <https://docs.nats.io/nats-server/configuration/securing_nats/auth_intro/nkey_auth>.

- Generate private (seed) and public (user) key pair:
  - `nk -gen user -pubout`

- Generate a public (user) key from a private (seed) key file called `seed.txt`.
  - `nk -inkey seed.txt -pubout > user.txt`

More example configurations for the nats-server are located in the [doc](https://github.com/RaaLabs/ctrl/tree/main/doc) folder in this repository.

#### Nkey from ED25519 SSH key

ctrl can also use an existing SSH ED25519 private key for authentication with the flag **-nkeyFromED25519SSHKeyFile="full-path-to-ssh-private-key"**. The SSH key will be converted to an Nkey if the option is used. The Seed and the User file will be stored in the **socketFolder** which by default is at **./tmp**

### How to send a Message

The API for sending a message from one node to another node is by sending a structured JSON or YAML object into a listener port in of of the following ways.

- unix socket called `ctrl.sock`. By default lives in the `./tmp` directory
- tcpListener, specify host:port with startup flag, or config file.
- httpListener, specify host:port with startup flag, or config file.
- readfolder, copy messages to send directly into the folder.

#### Send to socket with netcat

`nc -U ./tmp/ctrl.sock < myMessage.json`

#### Sending a command from one Node to Another Node

##### Example JSON for appending a message of type command into the `socket` file

In JSON:

```json
[
    {
        "directory":"/var/ctrl/cli-command/executed-result",
        "fileName": "some.log",
        "toNode": "ship1",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","sleep 3 & tree ./"],
        "ACKTimeout":10,
        "retries":3,
        "methodTimeout": 4
    }
]
```

Or in YAML:

```yaml
---
- toNodes:
  - ship1
  method: REQCliCommand
  methodArgs:
  - bash
  - "-c"
  - "
    cat <<< $'[{
    \"directory\": \"metrics\",
    \"fileName\": \"edgeAgent.prom\",
    \"fromNode\":\"metrics\",
    \"toNode\": \"ship1\",
    \"method\":\"REQHttpGetScheduled\",
    \"methodArgs\": [\"http://127.0.0.1:8080/metrics\",
    \"60\",\"5000000\"],\"replyMethod\":\"REQToFile\",
    \"ACKTimeout\":10,
    \"retries\": 3,\"methodTimeout\": 3
    }]'>scrape-metrics.msg
    "
  replyMethod: REQToFile
  ACKTimeout: 5
  retries: 3
  replyACKTimeout: 5
  replyRetries: 3
  methodTimeout: 5
  directory: system
  fileName: system.log
```

##### Specify more messages at once do

```json
[
    {
        "directory":"cli-command-executed-result",
        "fileName": "some.log",
        "toNode": "ship1",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","sleep 3 & tree ./"],
        "ACKTimeout":10,
        "retries":3,
        "methodTimeout": 4
    },
    {
        "directory":"cli-command-executed-result",
        "fileName": "some.log",
        "toNode": "ship2",
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","sleep 3 & tree ./"],
        "ACKTimeout":10,
        "retries":3,
        "methodTimeout": 4
    }
]
```

##### Send the same message to several hosts by using the toHosts field

```json
[
    {
        "directory": "httpget",
        "fileName": "finn.no.html",
        "toNodes": ["central","ship2"],
        "method":"REQHttpGet",
        "methodArgs": ["https://finn.no"],
        "replyMethod":"REQToFile",
        "ACKTimeout":5,
        "retries":3,
        "methodTimeout": 5
    }
]
```

##### Tail a log file on a node, and save the result of the tail centrally at the directory specified

```json
[
    {
        "directory": "./my/log/files/",
        "fileName": "some.log",
        "toNode": "ship2",
        "method":"REQTailFile",
        "methodArgs": ["./test.log"],
        "ACKTimeout":5,
        "retries":3,
        "methodTimeout": 200
    }
]
```

##### Example for deleting the ringbuffer database and restarting ctrl

```json
[
    {
        "directory":"system",
        "fileName":"system.log",
        "toNodes": ["ship2"],
        "method":"REQCliCommand",
        "methodArgs": ["bash","-c","rm -rf /usr/local/ctrl/lib/incomingBuffer.db & systemctl restart ctrl"],
        "replyMethod":"REQToFileAppend",
        "ACKTimeout":30,
        "retries":1,
        "methodTimeout": 30
    }
]
```

You can save the content to myfile.JSON and append it to the `socket` file:

- `cp <message-name> <pathto>/readfolder`

## Concepts/Ideas

### Naming

#### Subject

`<nodename>.<method>.<event>`

Example:

`ship3.REQCliCommand.EventACK`

For ACK messages (using the reply functionality in NATS) we append the `.reply` to the subject.

`ship3.REQCliCommand.EventACK.reply`

**Nodename**: Are the hostname of the device. This do not have to be resolvable via DNS, it is just a unique name for the host to receive the message.

**Event**: Desribes if we want and Acknowledge or No Acknowledge when the message was delivered :

- `EventACK`
- `EventNACK`

Info: The field **Event** are present in both the **Subject** structure and the **Message** structure.
This is due to **Event** being used in both the naming of a subject, and for specifying message type to allow for specific processing of a message.

**Method**: Are the functionality the message provide. Example could be for example `REQCliCommand` or `REQHttpGet`

##### Complete subject example

For Hello Message to a node named "central" of type Event and there is No Ack.

`central.REQHello.EventNACK`

For CliCommand message to a node named "ship1" of type Event and it wants an Ack.

`ship1.REQCliCommand.EventACK`

## History

ctrl is the continuation of the code I earlier wrote for RaaLabs called Steward. The original repo was public with a MIT license, but in October 2023 the original repo was made private, and are no longer avaialable to the public. The goal of this repo is to provide an actively maintained, reliable and stable version. This is also a playground for myself to test out ideas an features for such a service as described earlier.

This started out as an idea I had for how to control infrastructure.  This is the continuation of the same idea, and a project I'm working on free of charge in my own spare time, so please be gentle :)

NB: Filing of issues and bug fixes are highly appreciated. Feature requests will genereally not be followed up simply because I don't have the time to review it at this time :

## Disclaimer

All code in this repository are to be concidered not-production-ready, and the use is at your own responsibility and risk. The code are the attempt to concretize the idea of a purely async management system where the controlling unit is decoupled from the receiving unit, and that that we know the state of all the receiving units at all times.

Also read the license file for further details.

Expect the main branch to have breaking changes. If stability is needed, use the released packages, and read the release notes where changes will be explained.
