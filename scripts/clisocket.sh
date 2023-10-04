#!/bin/bash

if [ -z "$1" ]; then
    echo "No toNode supplied"
    exit 1
fi
if [ -z "$2" ]; then
    echo "No shell path supplied"
    exit 1
fi
if [ -z "$3" ]; then
    echo "No cmd supplied"
    exit 1
fi

nodes=$1
shell=$2
command=$3

IFS=',' read -r -a array <<<"$nodes"

function sendMessage() {
    cat >msg-"$element".yaml <<EOF
[
    {
        "toNodes": ["${element}"],
        "method": "REQCliCommand",
        "methodArgs":
            [
                "${shell}",
                "-c",
                'echo "--------------------${element}----------------------" && ${command}',
            ],
        "replyMethod": "REQToFileAppend",
        "retryWait": 5,
        "ACKTimeout": 10,
        "retries": 1,
        "replyACKTimeout": 10,
        "replyRetries": 1,
        "methodTimeout": 10,
        "replyMethodTimeout": 10,
        "directory": "./data/",
        "fileName": "debug.log",
    },
]
EOF

}

for element in "${array[@]}"; do
    sendMessage element "$command"
    nc -U ./tmp/ctrl.sock <msg-"$element".yaml
done
