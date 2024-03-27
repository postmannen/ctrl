#!/bin/bash

# Script to be copied over to the host, and executed.
# The script should exit with a line echoing out the
# error or success message, and should then be followed
# by an error code 0 if ok, and >0 if not ok.
##
# The HOSTNAME environmental variable is passed on to
# the host when the ssh session are initiated within
# the wrapper script. It is read from the hosts.txt
# file which should be in the format:
# 10.10.10.1,myhostnamehere

workdir=/usr/local/steward

# ------- Generate steward config

mkdir -p $workdir/etc
touch $workdir/etc/config.toml

# The ${NODENAME} is exported directly in the ssh command in the loop-wrapper script
cat >$workdir/etc/config.toml <<EOF
BrokerAddress = "<INSERT_BROKER-ADDRESS-HERE>:40223"
CentralNodeName = "central"
ConfigFolder = "${workdir}/etc/"
DatabaseFolder = "${workdir}/lib"
DefaultMessageRetries = 1
DefaultMessageTimeout = 10
ExposeDataFolder = ""
NatsConnectRetryInterval = 10
NkeySeedFile = "${workdir}/seed.txt"
NodeName = "${NODENAME}"
ProfilingPort = "localhost:8090"
PromHostAndPort = "localhost:2111"
RootCAPath = ""
SocketFolder = "/usr/local/steward/tmp"
StartPubREQHello = 60
SubscribersDataFolder = "/usr/local/steward/data"
TCPListener = ""
EOF

# ------- create systemctl
progName="steward"
systemctlFile=/etc/systemd/system/$progName.service

if test -f "$systemctlFile"; then
  if ! systemctl disable $progName.service >/dev/null 2>&1; then
    echo "*" error: systemctl disable progName.service
    exit 1
  fi

  if ! systemctl stop $progName.service >/dev/null 2>&1; then
    echo "*" error: systemctl stop $progName.service
    exit 1
  fi

  if ! rm $systemctlFile >/dev/null 2>&1; then
    echo "*" error: removing $systemctlFile service file
    exit 1
  fi
fi

#echo "*" creating systemd $progName.service
cat >$systemctlFile <<EOF
[Unit]
Description=http->${progName} service
Documentation=https://github.com/postmannen/ctrl
After=network-online.target nss-lookup.target
Requires=network-online.target nss-lookup.target

[Service]
ExecStart=env CONFIG_FOLDER=/usr/local/steward/etc /usr/local/steward/steward 

[Install]
WantedBy=multi-user.target
EOF

# ------- generate nats keys

errorMessage=$(
  # Redirect stderr to stdout for all command with the closure.
  {
    # Generate private nk key
    /usr/local/bin/nk -gen user >$workdir/seed.txt &&
      # Generate public nk key
      /usr/local/bin/nk -inkey $workdir/seed.txt -pubout >$workdir/user.txt &&
      chmod 600 $workdir/seed.txt &&
      chmod 600 $workdir/user.txt &&
      systemctl enable $progName.service &&
      systemctl start $progName.service
  } 2>&1
)

# ------- We're done, send output back to ssh client

# Check if all went ok with the previous command sequence, or if any errors happened.
# We return any result back to the ssh sessions which called this script by echoing
# the result, and do an exit <error code>.
# We can return one line back, so if more values are needed to be returned, they have
# to be formated into the same line.
#
# All went ok.
if [ $? -eq 0 ]; then
  user=$(cat $workdir/user.txt)
  echo "nkey-user=$user"
  exit 0
# An error happened.
else
  echo "failed executing script: $errorMessage"
  exit 1
fi
