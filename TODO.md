# TODO

## Key and ACL updates to use jetstream

## tailfile

Replace the hpcloud/tail with <https://github.com/tenebris-tech/tail>

## BUG configuration

bool flags with default value set to "false" becomes "true" if false is set.

## Logging

Remove these error logs:

`level=WARN msg="Thu Jan  9 12:14:24 2025, node: btdev1, error: readFolder: failed to open readFile from readFolder: open readfolder/msg2.yaml: no such file or directory\n"`

Remove httpGetScheduled
