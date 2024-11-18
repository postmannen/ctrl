# cliCommand

In JSON.

```json
[
    {
        "directory":"system",
        "fileName":"system.log",
        "toNodes": ["node2"],
        "method":"cliCommand",
        "methodArgs": ["bash","-c","rm -rf ./data & systemctl restart ctrl"],
        "replyMethod":"fileAppend",
        "ACKTimeout":30,
        "retries":1,
        "methodTimeout": 30
    }
]
```

In YAML.

```yaml
---
- toNodes:
    - node2
  method: cliCommand
  methodArgs:
    - "bash"
    - "-c"
    - |
      rm -rf ./data & systemctl restart ctrl

  replyMethod: fileAppend
  ACKTimeout: 30
  retries: 1
  ACKTimeout: 30
  directory: system
  fileName: system.log
```

Will send a message to node2 to delete the ctrl data folder, and then restart ctrl. The end result will be appended to the specified file on the node where the request originated.

## More examples

### Get the prometheus metrics of the central server

```json
[
    {
        "toNode": "central",
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

### Start up a tcp listener for number of seconds

```json
[
    {
        "toNode": "node1",
        "method": "cliCommandCont",
        "methodArgs": [
            "bash",
            "-c",
            "nc -lk localhost 8888"
        ],
        "replyMethod": "toConsole",
        "methodTimeout": 10,
    }
]
```

The netcat tcp listener will run for 10 seconds before the method timeout kicks in and ends the process.

### Get the running docker containers from a node

```json
[
    {
        "directory":"some/cli/command",
        "fileName":"cli.result",
        "toNode": "node2",
        "method":"cliCommand",
        "methodArgs": ["bash","-c","docker ps -a"],
        "replyMethod":"fileAppend",
    }
]
```
