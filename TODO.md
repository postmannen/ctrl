# TODO

## Key and ACL updates to use jetstream

## tailfile

Replace the hpcloud/tail with <https://github.com/tenebris-tech/tail>

## BUG configuration

bool flags with default value set to "false" becomes "true" if false is set.

## Logging

Remove these error logs:

`level=WARN msg="Thu Jan  9 12:14:24 2025, node: btdev1, error: readFolder: failed to open readFile from readFolder: open readfolder/msg2.yaml: no such file or directory\n"`

## Monitoring

Idea:

- Be able to send a request that runs a command, and wait for some result. Like wait for certain output by using f.ex. a regexp. If match, send a monitor request (separate type?, or use existing request types?) to inform about the happening of an event.

Thoughts:

- Since ctrl have no notition of the actual command to run, we could could add a regexp field to the message body.
- If the regexp matches, send a monitor request (separate type?) to inform about the happening of an event.
- We could have an extensible set of regexp patterns to match against, that define the type of event to monitor.
  - This could be the thing that makes it feasible to implements a Query Language?
- Should we implement our own store for the result, so we can directly use the QL to query the results?
