#!/bin/bash

# Helper script for sending command scripts to Steward.
# Uses a template.yaml file and fils in the shell variables
# specified within the template.
# If no template file exist a default template.yaml file
# will be created.
# The template.yaml file can be modified to suit your own
# needs
#
# Example:
# ./clifoldertemplate.sh myship1,myship2 /bin/bash 'ls -l /etc'
#
# The scripts takes three arguments:
# First: A single node name, or a comma separated list of more node names.
# Seconds: The path to the interpreter as seen on the node the message is sent to.
# Third: The actual command execute specified within single quotes.

# Create template is used when no template file is found,
# and will create a default template.yaml file.
function createTemplate() {
    cat >"$PWD"/template.yaml <<EOF
- toNodes:
    - \${node}
  method: cliCommand
  methodArgs:
    - ${shell}
    - -c
    - echo "--------------------\${node}----------------------" && ${command}
  replyMethod: fileAppend
  retryWait: 5
  ACKTimeout: 10
  retries: 1
  replyACKTimeout: 10
  replyRetries: 1
  methodTimeout: 10
  replyMethodTimeout: 10
  directory: ./data/
  fileName: debug.log

EOF

}

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
export shell=$2
export command=$3

# Check if the template file exist
if [ ! -f template.yaml ]; then
    echo "did not find the template file, will create default template file"
    createTemplate
fi

IFS=',' read -r -a array <<<"$nodes"

for node in "${array[@]}"; do
    export node

    line=$(envsubst <template.yaml)
    echo "$line" >"$PWD"/msg-"$node".yaml

    cp msg-"$node".yaml ./readfolder
done
