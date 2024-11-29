# TODO

## file variable in methodArguments

Create {{ file:<somefilehere> }} to be used within methodArguments. The content of the file could be used directly via stdin to a command.
Examples with kubectl:

Use stdin directly.
`{{file:/my.yaml}}|kubectl -f -`, where the last dash makes kubectl use the content from stdin.

Write to a local file on the destination node first, then use that file.
`echo {{file:/my.yaml}}>/home/bt/mydeployments/dep1.yaml && kubectl -f /home/bt/mydeployments/dep1.yaml`,

## Key and ACL updates to use jetstream

## Problem: loosing access to file, connection to the broker

level=DEBUG msg="error: ack receive failed: waiting for 0 seconds before retrying:   subject=errorCentral.errorLog: nats: timeout"
level=DEBUG msg="info: preparing to send nats message with subject errorCentral.errorLog, id: 233"
level=DEBUG msg="send attempt:8, max retries: 10, ack timeout: 60, message.ID: 233, method: errorLog, toNode: errorCentral"
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19866]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19868]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19870]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19872]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19874]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19876]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19878]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19880]
error signing nonce: unable to extract key pair from file "/Users/bt/ctrl/tmp/bt1/seed.txt": nats: open /Users/bt/ctrl/tmp/bt1/seed.txt: no such file or directory on connection [19882]
level=DEBUG msg="error: ack receive failed: waiting for 0 seconds before retrying:   subject=errorCentral.errorLog: nats: timeout"