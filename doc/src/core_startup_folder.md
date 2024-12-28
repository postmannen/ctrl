# Startup folder

Messages can be automatically scheduled to be read and executed at startup of ctrl.

A folder named **startup** will be present in the working directory of ctrl. To inject messages at startup, put them here.

Messages put in the startup folder will not be sent to the broker but handled locally, and only (eventually) the reply message from the Request Method called will be sent to the broker.

This can be really handy when you have some 1-off job you want to done at startup, like some cleaning up.

## Example reqest metrics from nodes

Another example could be that you have some local prometheus metrics you want to scrape every 5 minutes, and you want to send them to some central metrics system.

```yaml
---
- toNodes:
    - ["node1","node2"]
  method: httpGet
  methodArgs:
    - "http://localhost:8080/metrics"
  replyMethod: file
  methodTimeout: 5
  directory: metrics
  fileName: metrics.html
  schedule : [120,999999999]
```

The example above will send out a request to node1 and node2 every 120 second to scrape the metrics and write the results that came back to a folder named **data/metrics** in the current running directory.

## Example read metrics locally first, and then send to remote node

But we can also make the nodes publish their metrics instead of requesting it by putting a message in each nodes startup folder, set the **toNode** field to local, and instead use the **fromNode** field to decide where to deliver the result.

```yaml
---
- toNodes:
    - ["local"]
  fromNode: my-metrics-node
  method: httpGet
  methodArgs:
    - "http://localhost:8080/metrics"
  replyMethod: file
  methodTimeout: 5
  directory: metrics
  fileName: metrics.html
  schedule : [120,999999999]
```

In the above example, the httpGet will be run on the local node, and the result will be sent to my-metrics-node.

### How to send the reply to another node further explained

Normally the **fromNode** field is automatically filled in with the node name of the node where a message originated. Since messages within the startup folder is not received from another node via the normal message path we need set the **fromNode** field in the message to where we want the reply (result) delivered.

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

This example message will be read at startup, and executed on the local node where it was read, the method will be executed, and the result of the method will be sent to **central**. This is basically the same as the previous example, but we're using cliCommand method with curl instead of the httpGet method.
