# copySrc

Copy a file from one node to another node.

```json
[
    {
        "directory": "copy",
        "fileName": "copy.log",
        "toNodes": ["central"],
        "method":"copySrc",
        "methodArgs": ["./testbinary","ship1","./testbinary-copied","500000","20","0770"],
        "methodTimeout": 10,
        "replyMethod":"console"
    }
]
```

- toNode/toNodes, specifies what node to send the request to, and which also contains the src file to copy.
- methodArgs, are split into several fields, where each field specifies:
  1. SrcFullPath, specifies the full path including the name of the file to copy.
  2. DstNode, the destination node to copy the file to.
  3. DstFullPath, the full path including the name of the destination file. The filename can be different than the original name.
  4. SplitChunkSize, the size of the chunks to split the file into for transfer.
  5. MaxTotalCopyTime, specifies the maximum allowed time the complete copy should take. Make sure you set this long enough to allow the transfer to complete.
  6. FolderPermission, the permissions to set on the destination folder if it does not exist and needs to be created. Will default to 0755 if no value is set.

To copy from a remote node to the local node, you specify the remote nodeName in the toNode field, and the message will be forwarded to the remote node. The copying request will then be picked up by the remote node's **copySrc** handler, and the copy session will then be handled from the remote node.
