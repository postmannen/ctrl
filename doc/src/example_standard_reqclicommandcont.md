# REQCliCommandCont

The **REQCliCommand** and the **REQCliCommandCont** are the same, except for one thing. **REQCliCommand** will wait until wether the method is finished or the methodTimeout kicks in to send the result as one single message. **REQCliCommand** will when a line is given to  either stdout or stderr create messages with that single line in the data field, and send it back to the node where the message originated.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "node2",
        "method":"REQCliCommandCont",
        "methodArgs": ["bash","-c","tcpdump -nni any port 8080"],
        "replyMethod":"REQToFileAppend",
        "methodTimeout":10,
    }
]
```

Example Will run the command given for 10 seconds (methodTimeout), and return the stdout output of the command continously while the command runs. Uses the methodTimeout to define for how long the command will run.
