# Request Methods

| Method name| Description|
|------------|------------|
|REQOpProcessList | Get a list of the running processes|
|REQOpProcessStart | Start up a process|
|REQOpProcessStop | Stop a process|
|REQCliCommand | Will run the command given, and return the stdout output of the command when the command is done|
|REQCliCommandCont | Will run the command given, and return the stdout output of the command continously while the command runs|
|REQTailFile | Tail log files on some node, and get the result for each new line read sent back in a reply message|
|REQHttpGet | Scrape web url, and get the html sent back in a reply message|
|REQHello | Send Hello messages|
|REQCopySrc| Copy a file from one node to another node|
|REQErrorLog | Method for receiving error logs for Central error logger|
|REQNone | Don't send a reply message|
|REQToConsole | Print to stdout or stderr|
|REQToFileAppend | Append to file, can also write to unix sockets|
|REQToFile | Write to file, can also write to unix sockets|