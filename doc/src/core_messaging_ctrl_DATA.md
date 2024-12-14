# {{CTRL_DATA}} variable

By using the `{{CTRL_DATA}}` you can grab the result output of your initial request method, and then use it as input in your reply method.

**NB:** The echo command in the example below will remove new lines from the data. To also keep any new lines we need to put escaped **quotes** around the template variable. Like this:

- `\"{{CTRL_DATA}}\"`

Example of usage:

```json
[
    {
        "directory":"cli_command_test",
        "fileName":"cli_command.result",
        "toNode": "node2",
        "method":"cliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"cliCommand",
        "replyMethodArgs": ["bash", "-c","echo \"{{CTRL_DATA}}\" > apekatt.txt"],
        "replyMethodTimeOut": 10,
        "ACKTimeout":3,
        "retries":3,
        "methodTimeout": 10
    }
]
```

The above example, with steps explained:

- Send a message from **node1** to **node2** with a Request Method of type cliCommand.
- When received at **node2** we execute the Reqest Method with the arguments specified in the methodArgs.
- When the method on **node2** is done the result data of the method run will be stored in the variable {{CTRL_DATA}}. We can then use this variable when we craft the reply message method by embedding it into a new bash command.
- The reply message is then sent back to **node1**, the method will be executed, and all newlines in the result data will be removed, and all the data with the new lines removed will be stored in a file called `apekatt.txt`

The same using bash's herestring:

```json
[
    {
        "directory":"cli_command_test",
        "fileName":"cli_command.result",
        "toNode": "ship2",
        "method":"cliCommand",
        "methodArgs": ["bash","-c","tree"],
        "replyMethod":"cliCommand",
        "replyMethodArgs": ["bash", "-c","cat <<< {{CTRL_DATA}} > hest.txt"],
        "replyMethodTimeOut": 10,
        "ACKTimeout":3,
        "retries":3,
        "methodTimeout": 10
    }
]
```
