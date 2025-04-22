# Ctrl

NB: This detailed doc should only be considered as a draft that explains the functionality of ctrl in a more detailed way than the more general [README.md](README.md).

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
3. The method type of the message is checked, a subject is created based on the content of the message,  and a publisher process to handle the message type for that specific receiving node is started.
4. The message is then serialized to binary format, and sent to the subscriber on the receiving node.
5. If the message is expected to be ACK'ed by the subcriber then the publisher will wait for an ACK if the message was delivered. If an ACK was not received within the defined timeout the message will be resent. The amount of retries are defined within the message.

### Subscriber

1. The receiving end will need to have a subscriber process started on a specific subject and be allowed handle messages from the sending nodes to execute the method defined in the message.
2. When a message have been received, a handler for the method type specified in the message will be executed.
3. If the output of the method called is supposed to be returned to the publiser it will do so by using the replyMethod specified in the message body.

### Load balancing

ctrl instances with the same **Nodename** will automatically load balance the handling of messages on a given subject, and any given message will only be handled once by one instance.

NB: Messages sent to a **NodeAlias** name will not be load balanced if two or more nodes share the same alias. Instead it will be sent to all nodes with the same alias.

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

Error messages will be sent back to the central error handler and the originating node upon failure. By default the this will be the node named **central**.

```log
Tue Sep 21 09:17:55 2021, info: toNode: ship2, fromNode: central, method: OpProcessList: max retries reached, check if node is up and running and if it got a subscriber for the given request type
```

The error logs can be read on the central server in the directory `<ctrl-home>/data/errorLog`, and in the log of the instance the source node.

### Message handling and threads

- The handling of all messages is done by spawning up a process for handling the message in it's own thread. This allows us to keep the state of each **individual message**  both in regards to ACK's, error handling, send retries, and reruns of methods for a message if the first run was not successful.

- Publisher Processes on a node for handling new messages for new nodes will automatically be spawned when needed if it does not already exist.

- All messages processed by a publisher will be written to a log file after they are processed, with all the information needed to recreate the same message if needed, or it can be used for auditing.

- All handling down on the process and message level are handled concurrently. So if there are problems handling one message sent to a node on a subject it will not affect the messages being sent to other nodes, or other messages sent on other subjects to the same host.

- Message types of both **ACK** and **NO-ACK**, so we can decide if we want or don't want an Acknowledge if a message was delivered succesfully.
Example: We probably want an **ACK** when sending some **CLICommand** to be executed, but we don't care for an acknowledge **NO-ACK** when we send an **Hello** event.
If a message are **ACK** or **NO-ACK** type are defined by the value of the **ACKTimeout** for each individual message:

  1. **ACKTimeout** set to 0 will make the message become a **NO-ACK** message.
  2. **ACKTimeout** set to >=1 will make the message become an **ACK** message.

### Timeouts and retries for requests

- Default timeouts to wait for ACK messages and max attempts to retry sending a message are specified upon startup. This can be overridden on the message level, or with the configuration flags.

Example of a message with timeouts set:

```json
[
    {
        "directory":"/some/result/directory/",
        "fileName":"my-syslog.log",
        "toNode": "ship2",
        "methodArgs": ["bash","-c","tail -f /var/log/syslog"],
        "replyMethod":"fileAppend",
        "method":"cliCommandCont",
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
        "replyMethod":"fileAppend",
        "method":"cliCommandCont",
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
- Retry sending the message 3 times.

### Flags and configuration file

ctrl supports both the use of flags or env variables. An **.env** file can also be used.

### Schema for the messages to send into ctrl via the API's

- toNode : `string`
- toNodes : `string array`
- nodeAlias : `string`
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

All messages are compressed by default using the **zstd** compression algorithm.

### Serialization of messages sent between nodes

CTRL uses **CBOR** as the serialization format for messages sent between nodes.

A benefit of using **CBOR** is the size of the messages when transferred.

### startup folder

#### General functionality

Messages can be automatically scheduled to be read and executed at startup of ctrl.

A folder named **startup** will be present in the working directory of ctrl. To make CTRL execute messages at startup, put them here.

Messages put in the startup folder will not be sent to the broker but handled locally, and only (eventually) the reply message from the Request Method called will be sent to the broker.

If the message is defined with the **schedule** field, it will be re-run at that interval for as long as ctrl is running.

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
        "method": "cliCommand",
        "methodArgs": [
            "bash",
            "-c",
            "curl localhost:2111/metrics"
        ],
        "replyMethod": "console",
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
        "toNode": "local",
        "fromNode": "central",
        "method": "cliCommandCont",
        "methodArgs": [
            "bash",
            "-c",
            "nc -lk localhost 8888"
        ],
        "replyMethod": "console",
        "methodTimeout": -1,
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
        "method": "cliCommand",
        "methodArgs": [
            "bash",
            "-c",
            "hostname && curl -v http://nrk.no"
        ],
        "replyMethod": "console",
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

#### OpProcessList

Get a list of the running processes.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"opProcessList",
        "methodArgs": [],
        "replyMethod":"fileAppend",
    }
]
```

#### OpProcessStart

Start up a process. Takes the  method to start as it's only argument.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"opProcessStart",
        "methodArgs": ["httpGet"],
        "replyMethod":"fileAppend",
    }
]
```

#### OpProcessStop

Stop a process. Takes the request method, receiving node name, kind publisher/subscriber, and the process ID as it's arguments.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"opProcessStop",
        "methodArgs": ["httpGet","ship2","subscriber","199"],
        "replyMethod":"fileAppend",
    }
]
```

#### cliCommand

Run CLI command on a node. Linux/Windows/Mac/Docker-container or other.

Will run the command given, and return the stdout output of the command when the command is done.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "ship2",
        "method":"cliCommand",
        "methodArgs": ["bash","-c","docker ps -a"],
        "replyMethod":"fileAppend",
    }
]
```

#### cliCommandCont

Run CLI command on a node. Linux/Windows/Mac/Docker-container or other.

Will run the command given, and return the stdout output of the command continously while the command runs. Uses the methodTimeout to define for how long the command will run.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "ship2",
        "method":"cliCommandCont",
        "methodArgs": ["bash","-c","docker ps -a"],
        "replyMethod":"fileAppend",
        "methodTimeout":10,
    }
]
```

**NB**: A github issue is filed on not killing all child processes when using pipes <https://github.com/golang/go/issues/23019>. This is relevant for this request type.

And also a new issue registered <https://github.com/golang/go/issues/50436>

TODO: Check in later if there are any progress on the issue. When testing the problem seems to appear when using sudo, or tcpdump without the -l option. So for now, don't use sudo, and remember to use -l with tcpdump
which makes stdout line buffered. `timeout` in front of the bash command can also be used to get around the problem with any command executed.

#### TailFile

Tail log files on some node, and get the result for each new line read sent back in a reply message. Uses the methodTimeout to define for how long the command will run.

```json
[
    {
        "directory": "/my/tail/files/",
        "fileName": "tailfile.log",
        "toNode": "ship2",
        "method":"tailFile",
        "methodArgs": ["/var/log/system.log"],
        "methodTimeout": 10
    }
]
```

#### HttpGet

Scrape web url, and get the html sent back in a reply message. Uses the methodTimeout for how long it will wait for the http get method to return result.

```json
[
    {
        "directory": "web",
        "fileName": "web.html",
        "toNode": "ship2",
        "method":"httpGet",
        "methodArgs": ["https://web.ics.purdue.edu/~gchopra/class/public/pages/webdesign/05_simple.html"],
        "replyMethod":"file",
        "ACKTimeout":10,
        "retries": 3,
        "methodTimeout": 3
    }
]
```

#### Hello

Send Hello messages.

All nodes have the flag option to start sending Hello message to the central server. The structure of those messages looks like this.

```json
[
    {
        "toNode": "central",
        "method":"hello"
    }
]
```

#### CopySrc

Copy a file from one node to another node.

```json
[
    {
        "directory": "copy",
        "fileName": "copy.log",
        "toNodes": ["central"],
        "method":"copySrc",
        "methodArgs": ["./testbinary","ship1","./testbinary-copied","500000","20","0770"],
        "methodTimeout": 10,
        "replyMethod":"console"
    }
]
```

- toNode/toNodes, specifies what node to send the request to, and which also contains the src file to copy.
- methodArgs, are split into several fields, where each field specifies:
  1. SrcFullPath, specifies the full path including the name of the file to copy.
  2. DstNode, the destination node to copy the file to.
  3. DstFullPath, the full path including the name of the destination file. The filename can be different than the original name.
  4. SplitChunkSize, the size of the chunks to split the file into for transfer.
  5. MaxTotalCopyTime, specifies the maximum allowed time the complete copy should take. Make sure you set this long enough to allow the transfer to complete.
  6. FolderPermission, the permissions to set on the destination folder if it does not exist and needs to be created. Will default to 0755 if no value is set.

To copy from a remote node to the local node, you specify the remote nodeName in the toNode field, and the message will be forwarded to the remote node. The copying request will then be picked up by the remote node's **copySrc** handler, and the copy session will then be handled from the remote node.

#### ErrorLog

Method for receiving error logs for Central error logger.

**NB**: This is **not** to be used by users. Use **fileAppend** instead.

#### None

Don't send a reply message.

An example could be that you send a `cliCommand` message to some node, and you specify `replyMethod: none` if you don't care about the resulting output from the original method.

#### console

This is a method that can be used to get the data of the message printed to console where ctrl is running.

Default is to print to **stdout**, but printing to **stderr** can be done by setting the value of **methodArgs** to `"methodArgs": ["stderr"]`.

If used as a **replyMethod** set the **replyMethodArgs** `"replyMethodArgs": ["stderr"],`.

```json
[
    {
        "directory": "web",
        "fileName": "web.html",
        "toNode": "ship2",
        "method":"httpGet",
        "methodArgs": ["https://web.ics.purdue.edu/~gchopra/class/public/pages/webdesign/05_simple.html"],
        "replyMethod":"console",
        "ACKTimeout":10,
        "retries": 3,
        "methodTimeout": 3
    }
]
```

#### fileAppend

Append the output of the reply message to a log file specified with the `directory` and `fileName` fields.

If the value of the **directory** field is not prefixed with `./` or `/` the directory structure file will be created within the **ctrl data folder** specified in the config file.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"opProcessList",
        "methodArgs": [],
        "replyMethod":"fileAppend",
    }
]
```

##### Write to a socket file

If there is already a file at the specified path with the specified name, and if that file is a socket, then the request method will automatically switch to socket communication and write to the socket instead of normal file writing.

#### file

Write the output of the reply message to a file specified with the `directory` and `fileName` fields, where the writing will write over any existing content of that file.

If the value of the **directory** field is not prefixed with `./` or `/` the directory structure file will be created within the **ctrl data folder** specified in the config file.

```json
[
    {
        "directory":"test/dir",
        "fileName":"test.result",
        "toNode": "ship2",
        "method":"opProcessList",
        "methodArgs": [],
        "replyMethod":"ile",
    }
]
```

##### Write to socket file

If there is already a file at the specified path with the specified name, and if that file is a socket, then the request method will automatically switch to socket communication and write to the socket instead of normal file writing.

#### cliCommand as reply method

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
        "method":"cliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"cliCommand",
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
        "method":"cliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"cliCommand",
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

Some request types, like **cliCommand** also allow authorization of the message payload. The payload of the message can be checked against a list of allowed or denied commands configured in a main Access List on the central server.

With each message created a signature will also be created with the private key of the node, and the signature is then attached to the message.
NB: The keypair used for the signing of messages are a separate keypair used only for signing messages, and are not the same pair that is used for authentication with the NATS server.

The nodes will have a copy of the allowed public signing keys from the central server, and when a message is received, the signature is checked against the allowed public keys. If the signature is valid, the message is allowed to be processed further, otherwise it is denied if signature checking is enabled.

ctrl can be used either with no authorization at all, with signature checks only, or with ACL and signature checks. The features can be enabled or disabled in the **config.yaml** file.

##### Key registration on Central Server

All nodes will generate a private and a public key pair only used for signing messages. For building a complete database of all the public keys in the system and to be able to distribute them to other nodes, each node will send it's public key to the central server as the payload in the **hello** messages. The received keys will be stored in the central server's database.

For storing the keys on the central server two databases are involved.

- A Database for all the keys that have not been acknowledged.
- A Database for all the keys that have been acknowledged into the system with a hash of all the keys. This is also the database that gets distributed out to the nodes when they request and update

1. When a new not already registered key is received on the central server it will be added to the **NO_ACK_DB** database, and a message will be sent to the operator to permit the key to be added to the system.
2. When the operator permits the key, it will be added to the **Acknowledged** database, and the node will be removed from the Not-Acknowledged database.
3. If the key is already in the acked database no changes will be made.

If new keys are allowed into or deleted from the system, one attempt will be done to push the updated key database to all current nodes heard from in the network. If the push fails, the nodes will get the update the next time they ask for it based on the key update interval set on each node.

##### Key distribution to nodes

1. ctrl nodes will request key updates by sending a message to the central server with the **keysRequestUpdate** method on a timed interval. The hash of the current keys on a node will be put as the payload of the message.
2. On the Central server the received hash will be compared with the current hash on the central server. If the hashes are equal nothing will be done, and no reply message will be sent back to the end node.
3. If the hashes are not equal a reply message of type **keysDeliverUpdate** will be sent back to the end node with a copy of the acknowledged public keys database and a hash of those new keys.
4. The end node will then update it's local key database.

The interval of the updates can be controlled with it's own config or flag **keysRequestUpdateInterval**

##### Management of the keys on the central server

###### keysAllow

Will allow a key to be added to the system by moving the key from the **NO_ACK_DB** to the **ACK_DB**.

###### keysDelete

Will remove the specified keys from the **ACK_DB**.

##### Acl updates

1. ctrl nodes will request acl updates by sending a message to the central server with the **aclRequestUpdate** method on a timed interval. The hash of the current Acl on a node will be put as the payload of the message.
2. On the Central server the received hash will be compared with the current hash on the central server. If the hashes are equal nothing will be done, and no reply message will be sent back to the end node.
3. If the hashes are not equal a reply message of type **aclDeliverUpdate** will be sent back to the end node with a copy of the Acl's database for the node the request came from. The update will also contain the new hash of the new Acl's.
4. The end node will then replace it's local Acl database with the update.

The interval of the updates can be controlled with it's own config or flag **aclRequestUpdateInterval**

NB: The update process is initiated by the end nodes on a timed interval. No ACL updates are initiaded from the central server.

##### Management of the Acl on the central server

Several Request methods exists for handling the management of the active Acl's on the central server.

If the element specified is prefixed with **grp_** it will be treated as a group, otherwise it will be treated as a single node or command.

Groups or nodes do not have to exist to be used with an acl. The acl will be created with the elements specifed, and if a non existing group was specified you will have an Acl that is not yet functional, but it will become functional as soon as you add elements to the group's.

###### aclAddCommand

Takes the methodArgs: ["host or group of hosts", "src or group of src","cmd or group of cmd"]

###### aclDeleteCommand

Takes the methodArgs: ["host or group of hosts", "src or group of src","cmd or group of cmd"]

###### aclDeleteSource

Takes the methodArgs: ["host or group of hosts", "src or group of src"]

###### aclGroupNodesAddNode

Takes the methodArgs: ["nodegroup name", "node name"]

###### aclGroupNodesDeleteNode

Takes the methodArgs: ["nodegroup name", "node name"]

###### aclGroupNodesDeleteGroup

Takes the methodArgs: ["nodegroup name"]

###### aclGroupCommandsAddCommand

Takes the methodArgs: ["commandgroup name", "command"]

###### aclGroupCommandsDeleteCommand

Takes the methodArgs: ["commandgroup name", "command"]

###### aclGroupCommandsDeleteGroup

Takes the methodArgs: ["commandgroup name"]

###### aclExport

Creates an export of the current Acl's database, and delivers it to the requesting node with the replyMethod specified.

###### aclImport

Imports the Acl given in JSON format in the first argument of the methodArgs.

## Howto

ctrl will by default create the data and config directories needed in the current folder. This can be changed by using the different flags or editing the config file.

You can also run multiple instances of ctrl on the same machine. For testing you can create sub folders for each ctrl instance, go into each folder and start ctrl. When starting each ctrl instance make sure you give each node a unique `--nodeName`.

### Send messages with ctrl

You can now go to one of the folders for nodes started, and inject messages into the socket file `./tmp/ctrl.sock` with the **nc** tool.

Example on Mac:

`nc -U ./tmp/ctrl.sock < reqnone.msg`

Example on Linux:

`nc -NU ./tmp/ctrl.sock < reqnone.msg`

**NB**: By default ctrl creates it's folders like `./etc`, `./var`, and `./data` in the folder you're in when you start it. If you want to run multiple instances on the same machine you should create separate folders for each instance, and start ctrl when you're in that folder. The location of the folders can also be specified within the config file.

#### Nkey Authentication

Nkey's can be used for authentication, and you use the `nkeySeedFile` flag to specify the seed file to use.

Read more in the sections below on how to generate nkey's.

#### nats-server (the message broker)

The broker for messaging is Nats-server from <https://nats.io>. Download, run it, and use the `-brokerAddress` flag on **ctrl** to point to the ip and port:

`-brokerAddress="nats://10.0.0.124:4222"`

There is a lot of different variants of how you can setup and configure Nats. Full mesh, leaf node, TLS, Authentication, and more. You can read more about how to configure the Nats broker called nats-server at <https://nats.io/>.

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

#### To specify more messages at once do

```json
[
    {
        "directory":"cli-command-executed-result",
        "fileName": "some.log",
        "toNode": "ship1",
        "method":"cliCommand",
        "methodArgs": ["bash","-c","sleep 3 & tree ./"],
        "ACKTimeout":10,
        "retries":3,
        "methodTimeout": 4
    },
    {
        "directory":"cli-command-executed-result",
        "fileName": "some.log",
        "toNode": "ship2",
        "method":"cliCommand",
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
        "method":"httpGet",
        "methodArgs": ["https://finn.no"],
        "replyMethod":"file",
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
        "method":"tailFile",
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
        "method":"cliCommand",
        "methodArgs": ["bash","-c","rm -rf /usr/local/ctrl/lib/incomingBuffer.db & systemctl restart ctrl"],
        "replyMethod":"fileAppend",
        "ACKTimeout":30,
        "retries":1,
        "methodTimeout": 30
    }
]
```

You can save the content to myfile.JSON and append it to the `socket` file:

- `cp <message-name> <pathto>/readfolder`

#### Subject

`<nodename>.<method>`

Example:

`ship3.cliCommand`
