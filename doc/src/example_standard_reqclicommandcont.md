# cliCommandCont

The **cliCommand** and the **cliCommandCont** are the same, except for one thing. **cliCommand** will wait until wether the method is finished or the methodTimeout kicks in to send the result as one single message. **cliCommand** will when a line is given to  either stdout or stderr create messages with that single line in the data field, and send it back to the node where the message originated.

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "node2",
        "method":"cliCommandCont",
        "methodArgs": ["bash","-c","tcpdump -nni any port 8080"],
        "replyMethod":"fileAppend",
        "methodTimeout":10,
    }
]
```

Example Will run the command given for 10 seconds (methodTimeout), and return the stdout output of the command continously while the command runs. Uses the methodTimeout to define for how long the command will run.
