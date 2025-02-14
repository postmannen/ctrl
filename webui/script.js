/* jshint esversion: 9 */
// import { connect } from 'https://cdn.jsdelivr.net/npm/nats@2.29.1/index.min.js';

// import { wsconnect, nkeyAuthenticator } from "https://esm.run/@nats-io/nats-core";
// import { createUser, fromPublic, fromSeed } from "https://esm.run/@nats-io/nkeys";

import { wsconnect, nkeyAuthenticator } from "https://esm.run/@nats-io/nats-core";

const commandForm = document.getElementById('commandForm');
const generateBtn = document.getElementById('generateBtn');
const sendBtn = document.getElementById('sendBtn');
const writeFileBtn = document.getElementById('writeFileBtn');
const outputArea = document.getElementById('outputArea');
const fileTemplatesLink = document.getElementById('fileTemplatesLink');
const fileTemplatesPopup = document.getElementById('fileTemplatesPopup');

let nc;
let filesList = new Map();
let settings2;

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
