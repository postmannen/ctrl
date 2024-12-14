# Messaging

The handling of all messages is done by spawning up a process for handling the message in it's own thread. This allows us for indivudual control of each message both in regards to ACK's, error handling, send retries, and reruns of methods for a message if the first run was not successful.

All messages processed by a publisher will be written to a log file after they are processed, with all the information needed to recreate the same message if needed, or it can be used for auditing.

## Message types of both **Acknowledge** and **No-Acknoweldge**:

- **ACKTimeout** set to 0 will make the message become a **No ACK** message.
- **ACKTimeout** set to >=1 will make the message become an **ACK** message.

If ACK is expected, and not received, then the message will be retried the amount of time defined in the **retries** field of a message. If numer of retries are reached, and still no ACK received, the message will be discared, and a log message will be sent to the log server.

To make things easier, all timeouts used for messages can be set with env variables or flags at startup of ctrl. Locally specified timeouts directly in a message will override the startup values, and can be used for more granular control when needed.

## Example of message flow:

<style>
img {
  background-color: #FFFFFF;
}
</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/message-flow.svg?raw=true" /></p>
</body>

## Handling the result of a successful message delivery

### Default reply

We can decide what to do with the result of a message, and it's method have run successfully. By default if nothing is specified ctrls **file** method will be used. The result will be written to ctrl's **data folder**, and structured with the name of the node the result came from. This behavior can be overridden by defining the **directory** and **fileName** fields in the message body. 

If no reply is wanted, set the **replyMethod** to **none**

### Setting your own replyMethod

All methods that can be used as a method in message, can also be used as a **replyMethod**.

As an example we can use the **cliCommand** also as **replyMethod**.

```json
[
    {
        "toNode": "node2",
        "method": "cliCommand",
        "methodArgs": [
            "bash",
            "-c",
            "curl localhost:2111/metrics"
        ],
        "replyMethod": "cliCommand",
        "replyMethodArgs": [
            "bash",
            "-c",
            "echo \"{{CTRL_DATA}}\"> /somedirectory/somefile.html"
        ],
        "methodTimeout": 10
    }
]
```

The use of {{CTRL_DATA}} allows us to take the result of the initial method, and embed that in the reply message. The use are further explained in [{{CTRL_DATA}} variable](core_messaging_CTRL_DATA.md)