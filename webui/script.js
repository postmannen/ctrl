/* jshint esversion: 9 */
// import { connect } from 'https://cdn.jsdelivr.net/npm/nats@2.29.1/index.min.js';

// import { wsconnect, nkeyAuthenticator } from "https://esm.run/@nats-io/nats-core";
// import { createUser, fromPublic, fromSeed } from "https://esm.run/@nats-io/nkeys";

import { wsconnect, nkeyAuthenticator } from "https://esm.run/@nats-io/nats-core";
import * as d3 from "https://cdn.jsdelivr.net/npm/d3@7/+esm";
import { flamegraph } from 'https://cdn.jsdelivr.net/npm/d3-flame-graph@4.1.3/+esm';

const commandForm = document.getElementById('commandForm');
const generateBtn = document.getElementById('generateBtn');
const sendBtn = document.getElementById('sendBtn');
const writeFileBtn = document.getElementById('writeFileBtn');
const outputArea = document.getElementById('outputArea');
const fileTemplatesLink = document.getElementById('fileTemplatesLink');
const fileTemplatesPopup = document.getElementById('fileTemplatesPopup');
const graphLink = document.getElementById('graphLink');
const graphPopup = document.getElementById('graphPopup');
const flameGraphLink = document.getElementById('flameGraphLink');
const flameGraphPopup = document.getElementById('flameGraphPopup');

let nc;
let filesList = new Map();
let settings2;
let helloNodesList = new Map();
let nodeList = [];
let nodeInfoMap = new Map(); // Store node information

document.addEventListener('DOMContentLoaded', async function() {
    const outputArea = document.getElementById('outputArea');
    
    // Check for, and load saved settings first
    const savedSettings = localStorage.getItem("settings");

    settings2 = JSON.parse(savedSettings);
    // outputArea.value += "DEBUG: settings2: "+JSON.stringify(settings2)+"\n";
    
    // Initialize NATS client when the page loads.
    try {
        console.log("Connecting to NATS server: "+settings2.natsServer);
        const connectOptions = {
            servers: settings2.natsServer,
            timeout: 3000
        };

        // Only add authenticator if we're using nkeys and have valid seed
        if (settings2.useNkeys && settings2.nkeysSeedRAW) {
            connectOptions.authenticator = nkeyAuthenticator(settings2.nkeysSeedRAW);
            console.log("DEBUG: Using nkeys authentication");
        }
        console.log("******* settings.nkeysSeedRAW", settings2.nkeysSeedRAW);

        nc = await wsconnect(connectOptions);
        console.log("Connected to NATS server: "+settings2.natsServer);
        
        outputArea.value += "Connected to NATS server: "+settings2.natsServer+"\n";
        
        const sub = nc.subscribe(settings2.defaultNode+".webUI");
        (async () => {
            for await (const msg of sub) {
                try {
                    const msgJson = msg.json();
                    if (!msgJson || !msgJson.data) {
                        console.error("Invalid message format:", msg);
                        continue;
                    }

                    // The data is a byte array for UTF8 characters that are base64 encoded.
                    const ctrlMsgData = decodeBase64(msgJson.data);
                    const ctrlMsgMethodInstructions = msgJson.methodInstructions || [];

                    console.log("DEBUG: Received message:", {
                        subject: msg.subject,
                        data: ctrlMsgData,
                        instructions: ctrlMsgMethodInstructions
                    });

                    if (ctrlMsgMethodInstructions[0] === "templateFilesList") {
                        filesList = ctrlMsgData.split("\n");
                        console.log("DEBUG: filesList:", filesList);

                        // For each file in the list, we send a message to the central node
                        for (const file of filesList) {
                            if (!file.trim()) continue; // Skip empty lines

                            console.log("DEBUG: Requesting content for file:", file);
                            // Send a command to get the content of each file
                            const js2 = {
                                "toNodes": ["central"],
                                "useDetectedShell": false,
                                "method": "cliCommand",
                                "methodArgs": ["/bin/bash","-c","cat files/"+file],
                                "methodTimeout": 3,
                                "fromNode": settings2.defaultNode,
                                "replyMethod": "webUI",
                                "replyMethodInstructions": ["templateFileContent",file],
                                "ACKTimeout": 0
                            };
                            nc.publish("central.cliCommand", JSON.stringify(js2));
                        }
                    }

                    // Handle file content responses
                    if (ctrlMsgMethodInstructions[0] === "templateFileContent" && ctrlMsgMethodInstructions[1]) {
                        const file = ctrlMsgMethodInstructions[1];
                        const fileContent = ctrlMsgData;
                        console.log("DEBUG: Storing file content in localStorage:", {
                            file: file,
                            contentLength: fileContent.length
                        });

                        // Get existing filelist or create new one
                        let filelist = {};
                        const existingFilelist = localStorage.getItem('filelist');
                        if (existingFilelist) {
                            filelist = JSON.parse(existingFilelist);
                        }
                        
                        // Add or update file in filelist
                        filelist[file] = {
                            content: fileContent,
                            lastModified: new Date().toISOString()
                        };
                        
                        // Store updated filelist
                        localStorage.setItem('filelist', JSON.stringify(filelist));
                    }

                    if (ctrlMsgMethodInstructions[0] === "helloNodesList") {
                        nodeList = ctrlMsgData.split("\n")
                            .filter(name => name.trim()); // Remove empty entries
                        console.log("DEBUG: nodeList:", nodeList);

                        // -------------------------------
                        // For each file in the list, we send a message to the central node
                        for (const file of nodeList) {
                            if (!file.trim()) continue; // Skip empty lines

                            console.log("DEBUG: Requesting content for file:", file);
                            // Send a command to get the content of each file
                            const js2 = {
                                "toNodes": ["central"],
                                "useDetectedShell": false,
                                "method": "cliCommand",
                                "methodArgs": ["/bin/bash","-c","cat data/hello-messages/"+file+"/hello.log"],
                                "methodTimeout": 3,
                                "fromNode": settings2.defaultNode,
                                "replyMethod": "webUI",
                                "replyMethodInstructions": ["helloNodeContent",file],
                                "ACKTimeout": 0
                            };
                            nc.publish("central.cliCommand", JSON.stringify(js2));
                        }
                        // -------------------------------
                        
                        // Always initialize graph
                        initGraph();
                    }

                    if (ctrlMsgMethodInstructions[0] === "helloNodeContent") {
                        const node = ctrlMsgMethodInstructions[1];
                        const nodeContent = ctrlMsgData;
                        console.log("----HELLO NODE CONTENT------: helloNodeContent", node, nodeContent);
                        
                        try {
                            // Store the raw content instead of parsing as JSON
                            nodeInfoMap.set(node, { content: nodeContent });
                            
                            // Always update graph
                            initGraph();
                        } catch (error) {
                            console.error("Error handling node content:", error);
                        }
                    }

                    outputArea.value += `Received message on ${msg.subject}:\n${ctrlMsgData}\n`;
                    
                    // Scroll to bottom
                    outputArea.scrollTop = outputArea.scrollHeight;
                } catch (error) {
                    console.error("Error processing message:", error);
                    outputArea.value += `\nError processing message: ${error.message}\n`;
                }
            }
        })().catch((err) => {
            console.error("Subscription error:", err);
            outputArea.value += `\nSubscription error: ${err.message}\n`;
        });

        // Handle connection close
        nc.closed().then((err) => {
            if (err) {
                outputArea.value += `\nNATS connection closed with error: ${err.message}\n`;
            } else {
                outputArea.value += "\nNATS connection closed\n";
            }
        });

        // After successful NATS connection, request initial node list
        const js2 = {
            "toNodes": ["central"],
            "useDetectedShell": false,
            "method": "cliCommand",
            "methodArgs": ["/bin/bash","-c","ls -1 data/hello-messages/"],
            "methodTimeout": 3,
            "fromNode": settings2.defaultNode,
            "replyMethod": "webUI",
            "replyMethodInstructions": ["helloNodesList"],
            "ACKTimeout": 0
        };
        nc.publish("central.cliCommand", JSON.stringify(js2));

    } catch (err) {
        outputArea.value += `Failed to connect to NATS server: ${err.message}\n`;
    }

    // Send a command to list the files in the files folder.
    // We will later use the list we get back to ask for the content of each file.
    const js2 = {"toNodes": ["central"],"useDetectedShell": false,"method": "cliCommand","methodArgs": ["/bin/bash","-c","ls -1 files/"],
        "methodTimeout": 3,
        "fromNode": settings2.defaultNode,
        "replyMethod": "webUI",
        "replyMethodInstructions": ["templateFilesList"],
        "ACKTimeout": 0
    };
    nc.publish("central.cliCommand", JSON.stringify(js2));

    // localStorage.clear();

    // Disable default form submission
    commandForm.onsubmit = function(e) {
        e.preventDefault();
        return false;
    };

    // Disable Enter key submission for all inputs except textarea
    commandForm.addEventListener('keypress', function(e) {
        if (e.key === 'Enter' && e.target.tagName !== 'TEXTAREA') {
            e.preventDefault();
            return false;
        }
    }, true);

    initSplitter();

    // Add extras section toggle functionality
    const extrasToggle = document.querySelector('.extras-toggle');
    if (extrasToggle) {
        extrasToggle.addEventListener('click', function() {
            const extrasContent = document.querySelector('.extras-content');
            if (extrasContent) {
                extrasContent.classList.toggle('collapsed');
                // Update arrow direction
                const arrow = this.querySelector('.arrow');
                if (arrow) {
                    arrow.textContent = extrasContent.classList.contains('collapsed') ? '▼' : '▲';
                }
            }
        });
    }

    // Add auto-resize to the third method argument textarea
    const methodArg3 = document.querySelector('.method-arg:last-child');
    if (methodArg3) {
        // Set initial height to one line
        methodArg3.style.height = '2.4em'; // Approximately one line of text
        methodArg3.style.resize = 'none';   // Disable manual resize
        
        // Add input event listener for auto-resize
        methodArg3.addEventListener('input', function() {
            autoResizeTextarea(this);
        });
    }
}); 

function generateCommandData() {
    const methodArgs = Array.from(document.querySelectorAll('.method-arg'))
        .map(arg => arg.value)
        .filter(value => value !== ''); // Remove empty values

    return {
        toNodes: document.getElementById('toNodes').value.split(',').map(node => node.trim()).filter(node => node !== ''),
        jetstreamToNode: document.getElementById('jetstreamToNode').value || undefined,
        useDetectedShell: document.getElementById('useDetectedShell').value === 'true',
        method: document.getElementById('method').value,
        methodArgs: methodArgs,
        methodTimeout: parseInt(document.getElementById('methodTimeout').value),
        fromNode: settings2.defaultNode,
        replyMethod: document.getElementById('replyMethod').value,
        ACKTimeout: parseInt(document.getElementById('ackTimeout').value)
    };
}

// Handle Send button click
sendBtn.addEventListener('click', function(e) {
    e.preventDefault();

    if (!nc) {
        outputArea.value += "Error: Not connected to NATS server\n";
        return false;
    }

    const formData = generateCommandData();
    
    // Check if Method Arg 3 contains a CTRL_FILE template
    if (formData.methodArgs[2] && formData.methodArgs[2].includes('{{CTRL_FILE:')) {
        // Extract the file path from the template
        const match = formData.methodArgs[2].match(/{{CTRL_FILE:([^}]+)}}/);
        if (match) {
            const filePath = match[1];
            const fileName = filePath.split('/').pop(); // Get just the filename
            const filelist = JSON.parse(localStorage.getItem('filelist') || '{}');
            const fileContent = filelist[fileName] && filelist[fileName].content;
            
            if (fileContent) {
                // Replace the entire CTRL_FILE clause with the file content
                formData.methodArgs[2] = formData.methodArgs[2].replace(/{{CTRL_FILE:[^}]+}}/, fileContent);
            } else {
                outputArea.value += `\nError: File content not found for ${fileName}\n`;
                return false;
            }
        }
    }

    // Send the command to the NATS server
    for (const toNode of formData.toNodes) {
        console.log("Sending command to "+toNode);
        nc.publish(toNode+"."+formData.method, JSON.stringify(formData));
    }

    return false;
});

// Handle Generate Command button click
generateBtn.addEventListener('click', function(e) {
    e.preventDefault();

    const formData = generateCommandData();
    const output = formatCommandYaml(formData);

    outputArea.value += output;
    return false;
});

// Handle Write File button click
writeFileBtn.addEventListener('click', function(e) {
    e.preventDefault();
    const formData = generateCommandData();
    const output = formatCommandYaml(formData);
    outputArea.value += output;

    // Use hardcoded path instead of reading from input
    writeToFile(output, '/Users/bt/ctrl/tmp/bt1/readfolder/command.yaml');
    
    return false;
});

// Add this after the other event listeners in the DOMContentLoaded function

fileTemplatesLink.addEventListener("click", function (e) {
    e.preventDefault();
    
    // Get the popup content div and clear it
    const popupContent = document.querySelector('.popup-content');
    
    // Get filelist from localStorage
    const filelist = JSON.parse(localStorage.getItem('filelist') || '{}');
    const fileListHTML = Object.keys(filelist)
        .map(fileName => 
            fileName.trim() ? `<div class="file-item">${fileName}</div>` : ''
        ).join('');
    
    // Add title and container for files
    popupContent.innerHTML = `
        <span class="close-popup">&times;</span>
        <h2>File Templates</h2>
        <div class="files-list">
            ${fileListHTML}
        </div>
    `;

    // Re-attach close button handler since we replaced the content
    document.querySelector('.close-popup').addEventListener('click', function() {
        fileTemplatesPopup.style.display = "none";
    });

    // Add click handlers for file items
    document.querySelectorAll('.file-item').forEach(item => {
        item.addEventListener('click', () => {
            const methodArg3 = document.querySelector('.method-arg:last-child');
            methodArg3.value = `{{CTRL_FILE:files/${item.textContent}}}`;
            fileTemplatesPopup.style.display = "none";
        });
    });

    fileTemplatesPopup.style.display = "block";
    closeMenu();
});

// Add keyboard shortcut for template popup
document.addEventListener('keydown', function(e) {
    // Check if Ctrl+T was pressed (and not Cmd+T on Mac which is for new tab)
    if (e.ctrlKey && e.key === 't') {
        e.preventDefault(); // Prevent default browser behavior
        
        // Show the template popup
        fileTemplatesPopup.style.display = "block";
        
        // Get the popup content div and clear it
        const popupContent = document.querySelector('.popup-content');
        
        // Get filelist from localStorage
        const filelist = JSON.parse(localStorage.getItem('filelist') || '{}');
        const fileListHTML = Object.keys(filelist)
            .map(fileName => 
                fileName.trim() ? `<div class="file-item">${fileName}</div>` : ''
            ).join('');
        
        // Add title and container for files
        popupContent.innerHTML = `
            <span class="close-popup">&times;</span>
            <h2>File Templates</h2>
            <div class="files-list">
                ${fileListHTML}
            </div>
        `;

        // Re-attach close button handler
        document.querySelector('.close-popup').addEventListener('click', function() {
            fileTemplatesPopup.style.display = "none";
        });

        // Add click handlers for file items
        document.querySelectorAll('.file-item').forEach(item => {
            item.addEventListener('click', () => {
                const methodArg3 = document.querySelector('.method-arg:last-child');
                methodArg3.value = `{{CTRL_FILE:files/${item.textContent}}}`;
                fileTemplatesPopup.style.display = "none";
            });
        });
    }
});

function writeToFile(content, filename) {
    // Create a Blob containing the text content
    const blob = new Blob([content], { type: 'text/plain' });
    
    // Create a URL for the Blob
    const url = globalThis.URL.createObjectURL(blob);
    
    // Create a temporary anchor element
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;  // Use the provided filename
    
    // Append link to body, click it, and remove it
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    
    // Clean up by revoking the Blob URL
    globalThis.URL.revokeObjectURL(url);
}


function decodeBase64(base64) {
    const text = atob(base64);
    const length = text.length;
    const bytes = new Uint8Array(length);
    for (let i = 0; i < length; i++) {
        bytes[i] = text.charCodeAt(i);
    }
    const decoder = new TextDecoder(); // default is utf-8
    return decoder.decode(bytes);
}

function formatCommandYaml(commandData) {
    const output = `---
- toNodes:
    - ${commandData.toNodes[0]}
  ${commandData.jetstreamToNode ? `jetstreamToNode: ${commandData.jetstreamToNode}` : '#jetstreamToNode: btdev1'}
  useDetectedShell: ${commandData.useDetectedShell}
  method: ${commandData.method}
  methodArgs:
    - ${commandData.methodArgs.join('\n    - ')}
  methodTimeout: ${commandData.methodTimeout}
  replyMethod: ${commandData.replyMethod}
  ACKTimeout: ${commandData.ACKTimeout}`;
  
  return output;
}

// Add splitter functionality
function initSplitter() {
    const splitter = document.createElement('div');
    splitter.className = 'splitter';
    
    const leftPanel = document.querySelector('.left-panel');
    const container = document.querySelector('.split-container');
    
    let isDragging = false;

    splitter.addEventListener('mousedown', function(e) {
        isDragging = true;
        document.body.style.cursor = 'col-resize';
        
        document.addEventListener('mousemove', handleMouseMove);
        document.addEventListener('mouseup', handleMouseUp);
        
        // Prevent text selection while dragging
        e.preventDefault();
    });

    function handleMouseMove(e) {
        if (!isDragging) return;

        const containerRect = container.getBoundingClientRect();
        let newWidth = e.clientX - containerRect.left;
        
        // Calculate percentage width
        const containerWidth = containerRect.width;
        const minWidth = 200;
        const maxWidth = containerWidth - 200;

        // Constrain the width
        newWidth = Math.max(minWidth, Math.min(maxWidth, newWidth));
        
        leftPanel.style.width = `${newWidth}px`;
    }

    function handleMouseUp() {
        isDragging = false;
        document.body.style.cursor = '';
        document.removeEventListener('mousemove', handleMouseMove);
        document.removeEventListener('mouseup', handleMouseUp);
    }

    // Insert splitter between panels
    leftPanel.after(splitter);
}

// Add these styles to the existing style section in the head
const style = document.createElement('style');
style.textContent = `
    .files-list {
        margin-top: 20px;
        max-height: 400px;
        overflow-y: auto;
    }

    .file-item {
        padding: 10px;
        border: 1px solid #ddd;
        margin-bottom: 5px;
        border-radius: 4px;
        cursor: pointer;
        transition: background-color 0.2s;
    }

    .file-item:hover {
        background-color: #f0f0f0;
    }

    .popup-content h2 {
        margin-top: 0;
        margin-bottom: 20px;
    }
`;
document.head.appendChild(style);

// Add this after your other event listeners

// Add the graph initialization function
function initGraph() {
    // Update single container
    const containerId = "#graphContainer2";
    // Clear any existing graph
    d3.select(containerId).selectAll("*").remove();
    
    // Only proceed if we have nodes
    if (!nodeList || nodeList.length === 0) {
        console.log("No nodes to display");
        return;
    }

    // Set up the SVG container
    const container = document.querySelector(containerId);
    const width = container.clientWidth;
    const height = container.clientHeight;

    const svg = d3.select(containerId)
        .append("svg")
        .attr("width", width)
        .attr("height", height);

    // Create nodes data from nodeList
    const nodes = nodeList.map(nodeId => ({
        id: nodeId,
        group: nodeId === "central" ? 1 : 2,
        status: "active",
        connections: nodeId === "central" ? nodeList.length - 1 : 1,
        uptime: "calculating...",
        role: nodeId === "central" ? "central node" : "edge node"
    }));

    // Create links - connect all non-central nodes to central
    const links = nodes
        .filter(node => node.id !== "central")
        .map(node => ({
            source: "central",
            target: node.id,
            value: 1
        }));

    // Create the force simulation
    const simulation = d3.forceSimulation(nodes)
        .force("link", d3.forceLink(links).id(d => d.id).distance(100))
        .force("charge", d3.forceManyBody().strength(-300))
        .force("center", d3.forceCenter(width / 2, height / 2));

    // Add the links
    const link = svg.append("g")
        .selectAll("line")
        .data(links)
        .join("line")
        .attr("class", "link");

    // Update the tooltip initialization
    const tooltip = d3.select(containerId)
        .append("div")
        .attr("class", "tooltip")
        .style("opacity", 0)
        .style("position", "absolute")
        .style("background-color", "white")
        .style("padding", "5px")
        .style("border", "1px solid #ddd")
        .style("border-radius", "4px")
        .style("pointer-events", "auto")
        .style("z-index", "1000");

    // Update node selection to include hover behavior
    const node = svg.append("g")
        .selectAll("g")
        .data(nodes)
        .join("g")
        .attr("class", d => `node ${d.id}`)
        .call(d3.drag()
            .on("start", dragstarted)
            .on("drag", dragged)
            .on("end", dragended))
        .on("mouseover", function(event, d) {
            // Clear any existing tooltips first
            d3.select(containerId + " .tooltip")
                .style("opacity", 0)
                .html("");  // Clear content
            
            const tooltip = d3.select(containerId + " .tooltip");
            
            // Always ensure pointer events are enabled
            tooltip.style("pointer-events", "auto")
                   .style("display", "block"); // Ensure tooltip is displayed
            
            // Show the tooltip with full opacity
            tooltip.transition()
                .duration(200)
                .style("opacity", 1);
            
            // Get node info from the map
            const nodeInfo = nodeInfoMap.get(d.id) || {};
            
            // Extract timestamp from the raw content
            let timestamp = 'N/A';
            if (nodeInfo.content) {
                timestamp = nodeInfo.content.trim().split(',')[0];
            }
            
            // Get filelist from localStorage for creating command buttons
            const filelist = JSON.parse(localStorage.getItem('filelist') || '{}');
            const fileButtons = Object.keys(filelist)
                .map(fileName => `
                    <button class="tooltip-cmd-btn" 
                            data-filename="${fileName}" 
                            data-nodeid="${d.id}"
                            style="margin: 2px; 
                                   padding: 6px 12px; 
                                   background-color: #4CAF50; 
                                   color: white;
                                   border: none; 
                                   border-radius: 4px; 
                                   cursor: pointer;
                                   font-size: 12px;
                                   font-weight: 500;
                                   text-align: center;
                                   transition: background-color 0.2s;">
                        ${fileName}
                    </button>
                `).join('');
            
            // Create tooltip content with node info and command buttons
            const tooltipContent = `
                <div style="min-width: 200px;">
                    <table style="border-collapse: collapse; width: 100%; margin-bottom: 10px;">
                        <tr>
                            <td style="padding: 5px;"><strong>Node: ${d.id || 'Unknown'}</strong></td>
                        </tr>
                        <tr>
                            <td style="padding: 5px;"><strong>Last Hello: ${timestamp}</strong></td>
                        </tr>
                    </table>
                    <div style="border-top: 1px solid #ccc; padding-top: 8px;">
                        <div style="margin-bottom: 5px;"><strong>Available Commands:</strong></div>
                        <div style="display: flex; flex-wrap: wrap; gap: 4px;">
                            ${fileButtons}
                        </div>
                    </div>
                </div>
            `;
            
            tooltip
                .html(tooltipContent)
                .style("background-color", "white")
                .style("border", "1px solid black")
                .style("padding", "10px")
                .style("border-radius", "5px")
                .style("font-size", "12px")
                .style("box-shadow", "2px 2px 6px rgba(0, 0, 0, 0.3)");
            
            // Position tooltip above the node
            const tooltipNode = tooltip.node();
            const tooltipHeight = tooltipNode.getBoundingClientRect().height;
            const nodeRadius = d.id === "central" ? 15 : 10;
            const yOffset = tooltipHeight + nodeRadius + 5; // 5px extra padding
            
            // Get SVG container's position
            const svgRect = svg.node().getBoundingClientRect();
            
            tooltip
                .style("position", "absolute")
                .style("left", `${svgRect.left + d.x}px`)
                .style("top", `${svgRect.top + d.y - yOffset}px`)
                .style("transform", "translateX(-50%)") // Center horizontally
                .style("z-index", "1000");
            
            // Add click handlers for the command buttons
            tooltip.selectAll('.tooltip-cmd-btn').on('click', function(e) {
                e.preventDefault();  // Prevent default button behavior
                e.stopPropagation(); // Stop event bubbling
                
                const fileName = this.getAttribute('data-filename');
                const nodeId = this.getAttribute('data-nodeid');
                const filelist = JSON.parse(localStorage.getItem('filelist') || '{}');
                const fileContent = filelist[fileName]?.content;
                
                if (fileContent) {
                    const commandData = {
                        toNodes: [nodeId],
                        useDetectedShell: false,
                        method: "cliCommand",
                        methodArgs: ["/bin/bash", "-c", fileContent],
                        methodTimeout: 3,
                        fromNode: settings2.defaultNode,
                        replyMethod: "webUI",
                        ACKTimeout: 0
                    };
                    
                    if (nc) {
                        nc.publish(`${nodeId}.cliCommand`, JSON.stringify(commandData));
                        outputArea.value += `\nSent command from template ${fileName} to node ${nodeId}\n`;
                        outputArea.scrollTop = outputArea.scrollHeight;
                    } else {
                        console.error("NATS connection not available");
                        outputArea.value += `\nError: NATS connection not available\n`;
                    }
                } else {
                    console.error(`File content not found for ${fileName}`);
                    outputArea.value += `\nError: File content not found for ${fileName}\n`;
                }
            });
        })
        .on("mouseout", function(event, d) {
            const tooltip = d3.select(containerId + " .tooltip");
            const toElement = event.relatedTarget;
            
            // Don't hide if moving to the tooltip
            if (tooltip.node() && tooltip.node().contains(toElement)) {
                return;
            }
            
            // Start a timer to hide
            setTimeout(() => {
                // Only hide if not hovering over tooltip or node
                if (!document.querySelector(containerId + " .tooltip:hover") && 
                    !document.querySelector(containerId + " .node:hover")) {
                    tooltip.transition()
                        .duration(300)
                        .style("opacity", 0)
                        .on("end", function() {
                            // After fade out, set display to none
                            tooltip.style("display", "none");
                        });
                }
            }, 100);
        });

    // Update the tooltip mouseleave handler
    tooltip.on("mouseleave", function(event) {
        const tooltip = d3.select(this);
        
        setTimeout(() => {
            // Only hide if not hovering over node
            if (!document.querySelector(containerId + " .node:hover")) {
                tooltip.transition()
                    .duration(300)
                    .style("opacity", 0)
                    .on("end", function() {
                        // After fade out, set display to none
                        tooltip.style("display", "none");
                    });
            }
        }, 100);
    });

    // Add circles to nodes
    node.append("circle")
        .attr("r", d => d.id === "central" ? 15 : 10);

    // Add labels to nodes
    node.append("text")
        .text(d => d.id)
        .attr("x", 15)
        .attr("y", 5);

    // Update positions on each tick
    simulation.on("tick", () => {
        link
            .attr("x1", d => d.source.x)
            .attr("y1", d => d.source.y)
            .attr("x2", d => d.target.x)
            .attr("y2", d => d.target.y);

        node
            .attr("transform", d => `translate(${d.x},${d.y})`);
    });

    // Drag functions
    function dragstarted(event, d) {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
    }

    function dragged(event, d) {
        d.fx = event.x;
        d.fy = event.y;
    }

    function dragended(event, d) {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
    }
}

// Add these styles to the existing style section
const extraStyles = document.createElement('style');
extraStyles.textContent = `
    .extras-toggle {
        cursor: pointer;
        padding: 10px;
        background-color: #f5f5f5;
        border: 1px solid #ddd;
        border-radius: 4px;
        margin: 10px 0;
        display: flex;
        justify-content: space-between;
        align-items: center;
    }

    .extras-toggle:hover {
        background-color: #e9e9e9;
    }

    .extras-content {
        max-height: 1000px;
        overflow: hidden;
        transition: max-height 0.3s ease-out;
    }

    .extras-content.collapsed {
        max-height: 0;
    }

    .arrow {
        margin-left: 10px;
    }
`;
document.head.appendChild(extraStyles);

// Add this function after your existing functions
function autoResizeTextarea(textarea) {
    // Reset height to 1 line (using line-height as reference)
    textarea.style.height = 'auto';
    
    // Set new height based on scrollHeight
    textarea.style.height = textarea.scrollHeight + 'px';
}

// Add this event listener with the other DOMContentLoaded event listeners
flameGraphLink.addEventListener("click", function (e) {
    e.preventDefault();
    
    const popupContent = document.querySelector('#flameGraphPopup .popup-content');
    
    popupContent.innerHTML = `
        <span class="close-popup">&times;</span>
        <h2>Flame Graph Demo</h2>
        <div id="flameGraphContainer"></div>
    `;

    // Create demo data
    const data = {
        name: "root",
        value: 380,  // Sum of all child process values
        children: [
            {
                name: "Process A",
                value: 180,  // Sum of all Task A values
                children: [
                    { name: "Task A1", value: 20 },
                    { name: "Task A2", value: 20 },
                    { name: "Task A3", value: 20 },
                    { name: "Task A4", value: 20 },
                    { name: "Task A5", value: 20 },
                    { name: "Task A6", value: 20 },
                    { name: "Task A7", value: 20 },
                    { name: "Task A8", value: 20 },
                    { name: "Task A9", value: 20 }
                ]
            },
            {
                name: "Process B",
                value: 200,  // Sum of B1 (180) + B2 (20)
                children: [
                    { 
                        name: "Task B1", 
                        value: 180,  // Sum of subtasks
                        children: [
                            { name: "Subtask B1.1", value: 90 },
                            { name: "Subtask B1.2", value: 90 }
                        ]
                    },
                    { name: "Task B2", value: 20 }
                ]
            }
        ]
    };

    // Update chart dimensions
    const chart = flamegraph()
        .width(900)
        .height(350)    // Increased height but keeping it less than container height
        .cellHeight(25)
        .transitionDuration(750)
        .minFrameSize(5)
        .tooltip(true);

    // Render flame graph
    d3.select("#flameGraphContainer")
        .datum(data)
        .call(chart);

    // Re-attach close button handler
    document.querySelector('#flameGraphPopup .close-popup').addEventListener('click', function() {
        flameGraphPopup.style.display = "none";
    });

    flameGraphPopup.style.display = "block";
    closeMenu();
});

// Add keyboard shortcut for flame graph popup
document.addEventListener('keydown', function(e) {
    if (e.ctrlKey && e.key === 'f') {
        e.preventDefault();
        flameGraphLink.click();
    }
});

// Add these styles after the other styles in the file
const flameGraphStyles = document.createElement('style');
flameGraphStyles.textContent = `
    #flameGraphContainer {
        width: 100%;
        height: 400px;  // Increased from 200px to match chart height plus margins
        margin: 20px auto;
    }

    #flameGraphPopup .popup-content {
        width: 90%;
        max-width: 1000px;
        margin: 5% auto;
        padding: 30px;
        box-sizing: border-box;
        overflow: hidden;  // Added to contain the graph
    }

    .d3-flame-graph rect {
        stroke: #EEE;
        fill-opacity: .8;
    }

    .d3-flame-graph rect:hover {
        stroke: #474747;
        stroke-width: 0.5;
        cursor: pointer;
    }

    .d3-flame-graph-label {
        pointer-events: none;
        white-space: nowrap;
        text-overflow: ellipsis;
        overflow: hidden;
        font-size: 12px;
        font-family: Verdana;
        margin-left: 4px;
        margin-right: 4px;
        line-height: 1.5;
        padding: 0 0 0;
        font-weight: 400;
        color: black;
    }

    .d3-flame-graph .fade {
        opacity: 0.6 !important;
    }

    .d3-flame-graph .title {
        font-size: 20px;
        font-family: Verdana;
    }

    #flameGraphPopup {
        display: none;
        position: fixed;
        z-index: 1000;
        left: 0;
        top: 0;
        width: 100%;
        height: 100%;
        background-color: rgba(0,0,0,0.4);
    }

    #flameGraphLink {
        display: inline-block;
        margin: 10px;
        padding: 8px 15px;
        background-color: #4CAF50;
        color: white;
        text-decoration: none;
        border-radius: 4px;
        transition: background-color 0.3s;
    }

    #flameGraphLink:hover {
        background-color: #45a049;
    }
`;
document.head.appendChild(flameGraphStyles);