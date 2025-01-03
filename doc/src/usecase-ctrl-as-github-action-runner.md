# ctrl as github action runner

Run ctrl as a docker container in a github workflow. This can for example be as part of a CI/CD pipeline, or for content versioning.

This howto will show the steps involved to get ctrl up and running as Github Action Runner. First we start with setting up a NATS Server, we then setup the local ctrl node **node1**, and then the node that will be started as a Github Action Runner.

This howto assumes that you have a nats-server setup, and at least one ctrl node instance up running. How to setup a **nats-server** and a **ctrl node** on a computer/server can be found in the **User Guides** section of the documentation.

In the examples below I've used the name **node1** as an example for the node that will receive the message when the github repository are updated.
 
## Github Action Runner setup

Create a Github repository.

Clone the repository down to your local machine or other.

```bash
git clone <my-repo-name> && cd my-repo-name
```

Create a github workflows folder

```bash
mkdir -p .github/workflows
```

Create a `workflow.yaml` in the new directory with the following content.

```yaml
name: GitHub Actions Demo
run-name: ${{ github.actor }} is testing out GitHub Actions 🚀
on: [push]

jobs:
  send-message:
    name: send-message
    runs-on: ubuntu-latest

    services:
      ctrl:
        image: postmannen/ctrl:amd64-0.03
        env:
          NKEY_SEED: ${{ secrets.SEED }}
          NODE_NAME: "github"
          BROKER_ADDRESS: "<REPLACE WITH ip address of the NATS broker here>:4222"
          ENABLE_KEY_UPDATES: 0
          ENABLE_ACL_UPDATES: 0
        volumes:
          # mount the readfolder where we put the message to send.
          - ${{ github.workspace }}/readfolder:/app/readfolder
          # mount the files folder from the repo to get the file
          # to inject into the message.
          - ${{ github.workspace }}/files:/app/files
        options: --name ctrl

    steps:
      # When the container was started the repo was not yeat available since it was not
      # the workspace at that time. We want to make the /files mount to point to the
      # folder in the repo. Checkout the git repo, and restart the docker container so
      # it will mount the directories from the repo which now is the current github.workspace.
      - name: checkout repo
        uses: actions/checkout@v4
      - name: restart docker
        uses: docker://docker
        with:
          args: docker restart ctrl
      - run: sudo chmod 777 ${{ github.workspace }}/readfolder
      - run: sleep 3
      # Send the message by moving it to the readfolder.
      - run: mv ${{ github.workspace }}/message.yaml ${{ github.workspace }}/readfolder
      - run: >
          sleep 5 && tree
```

## Define NKEY as github secret

In your repository, go to **Settings->Secrets And Variables->Actions** and press **New Repository Secret**. Give the new secret the name **SEED**, and put the content of the seed into **Secret**. This is the seed that we referenced earlier in the github action workflow.

## Define message with command to send

We want to send a message from the Github Action, so we need to specify the content of the message to use.

In the root folder of the github folder create a **message.yaml** file, and fill in the following content:

```yaml
---
- toNodes:
    - node1
  method: cliCommand
  methodArgs:
    - "bash" # Change from bash to ash for alpine containers
    - "-c"
    - |
      echo '{{CTRL_FILE:./files/file.txt}}'>file.txt

  replyMethod: none
  ACKTimeout: 0
```

The message references a file with the `{{CTRL_FILE:<file path>}}` to use with the cli command found in the `files` folder. The file referenced will be embedded into the methodArgs defined in the message.

From the repository folder run the following commands:

```bash
mkdir -p files
echo "some cool text to put as file content.........." > files/file.txt
```

The example tries to show how we can get the message to run a shell/cli command on node1 at delivery. The shell command will use the content of the file located at `<repo>/files/file.txt`, and create a file called **file.txt** in the ctrl working directory on **node1**.

## Update the repository and send message with command

Commit the changes of the repository. If you check the **Actions** section of the new repo you should see that a an action have started.

When the action is done, you should have received a file called **file.txt** in the ctrl working directory on **node1**, with the content you provided in **text.txt**.

## Other cool things you can do ... like deploy kubernetes manifests

Replace the the bash command specified in the method arguments with a kubetctl command like this:

```yaml
---
- toNodes:
    - node1
  method: cliCommand
  methodArgs:
    - "bash" # Change from bash to ash for alpine containers
    - "-c"
    - |
      kubectl apply -f '{{CTRL_FILE:./files/mydeployment.yaml}}'

  replyMethod: none
  ACKTimeout: 0
```

Create a new file in the `<repo>/files` folder named **mydeployment.yaml**.

Commit the changes to the repo, and the deployment should be executed if you have a kubernetes instance running on **node1**.