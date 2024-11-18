# Send more messages

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
