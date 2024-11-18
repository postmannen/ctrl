# Request Methods

| Method name| Description|
|------------|------------|
|opProcessList | Get a list of the running processes|
|opProcessStart | Start up a process|
|opProcessStop | Stop a process|
|cliCommand | Will run the command given, and return the stdout output of the command when the command is done|
|cliCommandCont | Will run the command given, and return the stdout output of the command continously while the command runs|
|tailFile | Tail log files on some node, and get the result for each new line read sent back in a reply message|
|httpGet | Scrape web url, and get the html sent back in a reply message|
|hello | Send Hello messages|
|copySrc| Copy a file from one node to another node|
|errorLog | Method for receiving error logs for Central error logger|
|none | Don't send a reply message|
|console | Print to stdout or stderr|
|fileAppend | Append to file, can also write to unix sockets|
|file | Write to file, can also write to unix sockets|