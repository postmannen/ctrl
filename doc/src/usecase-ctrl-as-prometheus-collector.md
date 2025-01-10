# ctrl as prometheus collector

ctrl can be used to collect collect metrics from various systems. It can for example scrape/read some defined metrics with one node, and deliver the result to another node where you can expose the metrics directly using ctrl's builtin http server, or you could for example inject the metrics data to some database if that is desired. It is totally up to you.

</style>
</head>
<body>
<p align="center"><img src="https://github.com/postmannen/ctrl/blob/main/doc/usecase-prometheus-collector.svg?raw=true" /></p>
</body>

The example that follows will scrape prometheus metrics with two ctrl nodes, deliver them to a third node called **metrics**, and expose them with the builtin http server. Prometheus can then be used to read all the metrics from the various nodes on the **metrics** node.

Before you start, make sure to read the **User Guides** section for how to start up a NATS broker, and for general information about setting up ctrl.

## Node Setup

### central node

Start up a ctrl node named **central** that will serve as central for audit logs and other system logs happening with the ctrl nodes. Example of .env file to use.

```env
NODE_NAME=central
BROKER_ADDRESS=localhost:4222
LOG_LEVEL=info
START_PUB_HELLO=60
IS_CENTRAL_ERROR_LOGGER=true
```

### collected metrics node

Start up a node named **metrics** that will serve as the central place where we deliver all the metrics. On this node we expose ctrl's data folder over http. Example .env file below.

```env
NODE_NAME=metrics
BROKER_ADDRESS=localhost:4222
LOG_LEVEL=info
START_PUB_HELLO=60
EXPOSE_DATA_FOLDER=localhost:6060
```

### metrics collector nodes

#### prometheus node exporter

If don't yet have any metrics to collect, you can start up [prometheus node_exporter](https://github.com/prometheus/node_exporter) to get some local system metrics.

#### collector node setup

The following configuration file can be used on all nodes, but for each node started replace with a unique **NODE_NAME**, eg. **node1** and **node2**.

```env
NODE_NAME=node1     #give each node a unique node name
BROKER_ADDRESS=localhost:4222
LOG_LEVEL=info
START_PUB_HELLO=60
```

Create 2-3 (node1,node2,node3) nodes which will be the nodes that scrape some metrics on some host.

##### Startup folder

Each ctrl instance started will have a **startup** folder in it's running directory. Messages in the startup folder will be read at startup, and handled by ctrl.

We can use the **schedule** field in the message to make ctrl rerun the method of the message at a scheduled interval.

Put the following message in the **startup** folder on all the nodes that will collect metrics.

```yaml
---
- toNodes:
    # Deliver the message locally
    - local
  # Set the node where we send the reply with the result
  fromNode: metrics
  method: httpGet
  methodArgs:
    - http://localhost:9100/metrics
  # Write the result to a file on the fromNode
  replyMethod: file
  # The directory which we want to write the result in
  directory: nodeexporters
  # The filename we want to write the result to
  fileName: metrics.html
  # Schedule rerun of the method every 30 second, for 999999999 seconds.
  schedule: [30,999999999]
```

## Check the collected metrics

When all nodes are started, they should start to send metrics to the **metrics** node. We can see the result by using curl on the node named **metrics**.

```bash
http://localhost:6060/nodeexporters/node1/metrics.html
http://localhost:6060/nodeexporters/node2/metrics.html
```

If you want to do something further with this example you can install prometheus on the **metrics** node, and direct it to collect read the metrics from the various folders via the url's above.
