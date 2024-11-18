# tailFile

In JSON.

```json
[
    {
        "directory": "tails",
        "fileName": "some.log",
        "toNodes": "node1","node2","node3",
        "method":"tailFile",
        "methodArgs": ["/var/log/syslog"],
        "ACKTimeout":5,
        "retries":3,
        "methodTimeout": 200
    }
]
```

NB: If no replyMethod are specified, it will default to **file**

In YAML.

```yaml
---
- toNodes:
    - ["node1","node2","node3"]
  method: tailFile
  methodArgs:
    - "/var/log/syslog"
  replyMethod: file
  ACKTimeout: 5
  retries: 3
  methodTimeout: 5
  directory: tails
  fileName: var_log_syslog.log
```

The above example will tail the syslog file on 3 nodes for 5 seconds, and save the result on the node where the request came from in the local `data` folder.
