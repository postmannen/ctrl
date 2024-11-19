# Startup folder

Messages can be automatically scheduled to be read and executed at startup of ctrl.

A folder named **startup** will be present in the working directory of ctrl. To inject messages at startup, put them here.

Messages put in the startup folder will not be sent to the broker but handled locally, and only (eventually) the reply message from the Request Method called will be sent to the broker.

This can be really handy when you have some 1-off job you want to done at startup, like some cleaning up.

Another example could be that you have some local prometheus metrics you want to scrape every 5 minutes, and you want to send them to some central metrics system. 

#### How to send the reply to another node

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

This example message will be read at startup, and executed on the local node where it was read, the method will be executed, and the result of the method will be sent to **central**.
