# Install on a host

Start up a local nats message broker if you don't already have one.

```bash
docker run -p 4444:4444 nats -p 4444
```

Build the ctrl binary from the source code.

```bash
git clone git@github.com:postmannen/ctrl.git
cd cmd/ctrl
go run build
```

Copy the binary to `/usr/local`.

```bash
mkdir -p /usr/local/ctrl
cp ./ctrl /usr/local/ctrl
```

```bash

For testing we create a folder for the node to store it's data.

```bash
cd /usr/local/ctrl
mkdir node1
cd node1
```

ctrl will create all the folders needed like etc, var and more in the current directory where it was started if they don't already exist. This behaviour can be changed with flags or env variables.

Create a .env file for the startup options. Flags can also be used.

```bash
cat << EOF > .env
NODE_NAME="node1"
BROKER_ADDRESS="127.0.0,1:4444"
ENABLE_DEBUG=1
START_PUB_REQ_HELLO=60
IS_CENTRAL_ERROR_LOGGER=0
EOF
```

Start up ctrl. ctrl will automatically used the local .env file we created.

```bash
../usr/local/ctrl/ctrl
```

If you open another window, and go to the `/usr/local/ctrl/node1` you should see that ctrl have created the directory structure for you with ./etc, ./var, ./directoryfolder and so on.

Prepare and send a message. We send messages by copying them into the ./readfolder where ctrl automatically will pick it up, and process it.

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

With the above message we send to ourselves since we only got 1 node running. To start up more nodes repeat the above steps, replace.

## Run as service

Create a systemctl unit file to run ctrl as a service on the host

```bash
progName="ctrl"
systemctlFile=/etc/systemd/system/$progName.service

cat >$systemctlFile <<EOF
[Unit]
Description=http->${progName} service
Documentation=https://github.com/postmannen/ctrl
After=network-online.target nss-lookup.target
Requires=network-online.target nss-lookup.target

[Service]
ExecStart=env CONFIG_FOLDER=/usr/local/${progName}/etc /usr/local/${progName}/${progName}

[Install]
WantedBy=multi-user.target
EOF

systemctl enable $progName.service &&
systemctl start $progName.service
```
