# Install with docker

Start up a local nats message broker

```bash
docker run -p 4444:4444 nats -p 4444
```

Create a ctrl docker image.

```bash
git clone git@github.com:postmannen/ctrl.git
cd ctrl
docker build -t ctrl:test1 .
```

Create a folder which will be the working directory for the node. This is where we keep the .env file, and can mount local host folders to folders within the container.

```bash
mkdir -p testrun/readfolder
cd testrun
```

create a .env file

```bash
cat << EOF > .env
NODE_NAME="node1"
BROKER_ADDRESS="127.0.0,1:4444"
ENABLE_DEBUG=1
START_PUB_REQ_HELLO=60
IS_CENTRAL_ERROR_LOGGER=0
EOF
```

Start the ctrl container. To be able to send messages into ctrl we mount the readfolder to a local directory. When we later got a messages to send we just copy it into the read folder and ctrl will pick it up and handle it. Messages can be in either YAML or JSON format.

```bash
docker run --env-file=".env" --rm -ti -v $(PWD)/readfolder:/app/readfolder ctrl:test1
```

Prepare and send a message.

```yaml
cat << EOF > msg.yaml
---
- toNodes:
    - node1
  method: cliCommand
  methodArgs:
    - "bash"
    - "-c"
    - |
      echo "some config line" > /etc/my-service-config.1
      echo "some config line" > /etc/my-service-config.2
      echo "some config line" > /etc/my-service-config.3
      systemctl restart my-service

  replyMethod: none
  ACKTimeout: 0
EOF

cp msg.yaml readfolder
```

With the above message we send to ourselves since we only got 1 node running. To start up more nodes repeat the above steps, replace the node name under `toNodes` with new names for new nodes.
NB: If more nodes share the same name the requests will be loadbalanced between them round robin.
