# Errors

Errors happening on **all** nodes will be reported back to the node(s) started with the flag `-isCentralErrorLogger` set to true, or by using the `IS_CENTRAL_ERROR_LOGGER` env variable.

## Log level

The log level can also be specified with the `LOG_LEVEL` env variable. Values are `error/info/warning/debug/none`.

## Debug

To get more debug information in the logs printed to STDERR the env variable `ENABLE_DEBUG` can be set to true. This will not affect the information printed to the log files, only to STDERR.
