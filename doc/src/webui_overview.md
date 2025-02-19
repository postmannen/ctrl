# WebUI Overview

The WebUI is a web application that allows you to interact with CTRL via NATS.

NB: The WebUI is in early development.

![WebUI General](https://github.com/postmannen/ctrl/blob/main/doc/webui-general.png?raw=true)

## Features

### Send Commands

Send commands to one or many nodes. The command will be sendt, executed, and the result will be displayed in the WebUI.

### Network Graph

Visualize the network of nodes.

The graph shows the nodes that are connected to the central node, and the time since the last **hello** message was received from the node.

When hovering over a node, the node details will be shown in a tooltip. The defined file templates for the node will also be shown in the tooltip, and the user can select one of the templates to use for the command to execute on the node.

![WebUI Network Graph](https://github.com/postmannen/ctrl/blob/main/doc/webui-network-graph-infobox.png?raw=true)

### File Templates

File templates containing a script can be used to execute commands. The file templates should be stored in the **files** directory on the **central** node. The WebUI will then ask the central node for available templates, and show the file templates in a dropdown menu for the user to select from.

![WebUI File Templates](https://github.com/postmannen/ctrl/blob/main/doc/webui-file-templates.png?raw=true)

### Settings

Set setting like the Node name of the UI, NATS server URL, and NKEY to use for authentication to the NATS server.

![WebUI Settings](https://github.com/postmannen/ctrl/blob/main/doc/webui-settings.png?raw=true)

### Flame Graph

In development.
