# REQHttpGet

In JSON.

```json
[
    {
        "directory": "httpget",
        "fileName": "finn.no.html",
        "toNodes": ["node1","node2"],
        "method":"REQHttpGet",
        "methodArgs": ["https://finn.no"],
        "replyMethod":"REQToFile",
        "ACKTimeout":5,
        "retries":3,
        "methodTimeout": 5
    }
]
```

In YAML.

```yaml
---
- toNodes:
    - ["node1","node2"]
  method: REQHttpGet
  methodArgs:
    - "https://finn.no"
  replyMethod: REQToFile
  ACKTimeout: 5
  retries: 3
  methodTimeout: 5
  directory: httpget
  fileName: finn.no.html
```

The result html file of the http get will be written to:

- \<data folder\>\httpget\node1\finn.no.html
- \<data folder\>\httpget\node2\finn.no.html
