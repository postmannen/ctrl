# Message fields

```yaml
toNode : "some-node"
```

The node to send the message to.

```yaml
toNodes : node1,node2
```

ToNodes to specify several hosts to send message to in the form of an slice/array. When used, the message will be split up into one message for each node specified, so the sending to each node will be handled individually.

```yaml
data : data here in byte format
```

The actual data in the message. This is the field where we put the returned data in a reply message. The data field are of type []byte.

```yaml
method : cliCommand
```

What request method type to use, like cliCommand, httpGet..

```yaml
  methodArgs :
  - "bash"
  - "-c"
  - |
      echo "this is a test"
      echo "and some other test"
```

Additional arguments that might be needed when executing the method. Can be f.ex. an ip address if it is a tcp sender, or the actual shell command to execute in a cli.

```yaml
replyMethod : file
```

ReplyMethod, is the method to use for the reply message. By default the reply method will be set to log to file, but you can override it setting your own here.

```yaml
  methodArgs :
  - "bash"
  - "-c"
  - |
      echo "this is a test"
```

Additional arguments that might be needed when executing the reply method. Can be f.ex. an ip address if it is a tcp sender, or the shell command to execute in a cli session.
replyMethodArgs :

```yaml
fromNode Node : "node2"
```

From what node the message originated. This field is automatically filled by ctrl when left blanc, so that when a message are sent from a node the user don't have to worry about getting eventual repply messages back.
Setting a

```yaml
ACKTimeout: 10 
```

ACKTimeout for waiting for an Ack message in seconds.

If the ACKTimeout value is set to 0 the message will become an No Ack message. With No Ack messages we will not wait for an Ack, nor will the receiver send an Ack, and we will never try to resend a message.

```yaml
retryWait : 60
```

RetryWait specifies the time in seconds to wait between retries. This value is added to the ACKTimeout and to make the total time before retrying to sending a message. A usecase can be when you want a low ACKTimeout, but you want to add more time between the retries to avoid spamming the receiving node. 

```yaml
retries : 3
```

How many times we should try to resend the message if no ACK was received.

```yaml
replyACKTimeout int `json:"replyACKTimeout" yaml:"replyACKTimeout"`
```

The ACK timeout used with reply messages. If not specified the value of `ACKTimeout` will be used.

```yaml
replyRetries int `json:"replyRetries" yaml:"replyRetries"`
```

The number of retries for trying to resend a message for reply messages.

```yaml
methodTimeout : 10
```

Timeout for how long a method should be allowed to run before it is timed out.

```yaml
replyMethodTimeout : 10
```

Timeout for how long a method should be allowed to run before it is timed out, but for the reply message.

```yaml
directory string `json:"directory" yaml:"directory"`
```

Specify the directory structure to use when saving the result data for a repply message.

- When the values are comma separated like `"syslog","metrics"` a syslog folder with a subfolder metrics will be created in the directory specified with startup env variable `SUBSCRIBER_DATA_FOLDER`.
- Absolute paths can also be used if you want to save the result somewhere else on the system, like "/etc/myservice".

```yaml
fileName : myfile.conf
```

The fileName field are used together with the directory field mentioned earlier to create a full path for where to write the resulting data of a repply message.

```yaml
schedule : [2,5]
```

Schedule are used for scheduling the method of messages to be executed several times. The schedule is defined as an array of two values, where the first value defines how often the schedule should trigger a run in seconds, and the second value is for how long the schedule are allowed to run in seconds total. `[2,5]` will trigger the method to be executed again every 2 seconds, but when a total time of 5 seconds have passed it will stop scheduling.
