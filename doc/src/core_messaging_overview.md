# Messaging

The handling of all messages is done by spawning up a process for handling the message in it's own thread. This allows us to keep the state of each **individual message level**  both in regards to ACK's, error handling, send retries, and reruns of methods for a message if the first run was not successful.

All messages processed by a publisher will be written to a log file after they are processed, with all the information needed to recreate the same message if needed, or it can be used for auditing.

All handling down to the process and message level are handled concurrently. So if there are problems handling one message sent to a node on a subject it will not affect the messages being sent to other nodes, or other messages sent on other subjects to the same host.

Message types of both **ACK** and **NACK**, so we can decide if we want or don't want an Acknowledge if a message was delivered succesfully.
Example: We probably want an **ACK** when sending some **REQCLICommand** to be executed, but we don't care for an acknowledge **NACK** when we send an **REQHello** event.
If a message are **ACK** or **NACK** type are defined by the value of the **ACKTimeout** for each individual message:

  - **ACKTimeout** set to 0 will make the message become a **NACK** message.
  - **ACKTimeout** set to >=1 will make the message become an **ACK** message.

If ACK is expected, and not received, then the message will be retried the amount of time defined in the **retries** field of a message. If numer of retries are reached, and still no ACK received, the message will be discared, and a log message will be sent to the log server.

To make things easier, all timeouts used for messages can be set with env variables or flags at startup of ctrl. Locally specified timeout directly in a message will override the startup values, and can be used for more granularity when needed.

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
