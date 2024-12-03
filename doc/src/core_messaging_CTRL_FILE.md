# Message methodArgs variables

## {{CTRL_FILE}}

Read a local text file, and embed the content of the file into the methodArgs.

```yaml
---
- toNodes:
    - btdev1
  #jetstreamToNode: btdev1
  method: cliCommand
  methodArgs:
    - /bin/bash
    - -c
    - |
      echo {{CTRL_FILE:/some_directory/source_file.yaml}}>/other_directory/destination_file.yaml
  methodTimeout: 3
  replyMethod: console
  ACKTimeout: 0
```

The above example will before sending the message read the content of the file `/some_directory/source_file.yaml`. When the message is received at it's destination node and the cliCommand is executed the content will be written to `/other_directory/destination_file.yaml`.
