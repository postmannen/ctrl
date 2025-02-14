package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tcell "github.com/gdamore/tcell/v2"
	"github.com/nats-io/nats.go"
	"github.com/rivo/tview"
)

type CommandData struct {
	ToNodes          []string `json:"toNodes"`
	ToNode           string   `json:"toNode"`
	JetstreamToNode  string   `json:"jetstreamToNode,omitempty"`
	UseDetectedShell bool     `json:"useDetectedShell"`
	Method           string   `json:"method"`
	MethodArgs       []string `json:"methodArgs"`
	MethodTimeout    int      `json:"methodTimeout"`
	FromNode         string   `json:"fromNode"`
	ReplyMethod      string   `json:"replyMethod"`
	ACKTimeout       int      `json:"ACKTimeout"`
}

// Create a global variable for the output view so it can be accessed from NATS handlers
var outputView *tview.TextView
var historyList *tview.List
var filesList *tview.List
var app *tview.Application
var nc *nats.Conn

// Store form values globally
var (
	toNodes    string = "btdev1"
	methodArgs        = []string{"/bin/bash", "-c", "ls -l"}
	// Store history commands
	historyCommands []string
)

func createCommandPage() tview.Primitive {
	// Create a flex container for the entire page
	mainFlex := tview.NewFlex().SetDirection(tview.FlexRow)

	// Create form for inputs
	formFields := tview.NewForm()
	formFields.SetBorder(true).SetTitle("Command Form")

	// Create a List for command history
	historyList = tview.NewList()
	historyList.SetBorder(true)
	historyList.SetTitle("History")
	// Handle selection from history
	historyList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		// Update Method Arg 3 field with selected command
		methodArg3Field := formFields.GetFormItemByLabel("Method Arg 3").(*tview.InputField)
		methodArg3Field.SetText(mainText)
		methodArgs[2] = mainText // Update global variable
		app.SetFocus(formFields) // Return focus to form
	})

	// Create a List for files
	filesList = tview.NewList()
	filesList.SetBorder(true)
	filesList.SetTitle("Files")
	filesList.SetSelectedFunc(func(index int, mainText string, secondaryText string, shortcut rune) {
		// When a file is selected, update Method Arg 3 with the CTRL_FILE template
		methodArg3Field := formFields.GetFormItemByLabel("Method Arg 3").(*tview.InputField)
		methodArg3Field.SetText("{{CTRL_FILE:" + secondaryText + "}}")
		methodArgs[2] = methodArg3Field.GetText()
		app.SetFocus(formFields)
	})

	// Create a vertical flex for history and files lists
	rightPanelFlex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(historyList, 0, 1, false).
		AddItem(filesList, 0, 1, false)

	// Create a horizontal flex for the form and right panel
	topFlex := tview.NewFlex().
		AddItem(formFields, 0, 2, true).
		AddItem(rightPanelFlex, 0, 1, false)

	// Create output text view
	outputView = tview.NewTextView().
		SetDynamicColors(true).
		SetChangedFunc(func() {
			app.Draw()
		})
	outputView.SetBorder(true).SetTitle("Output")

	// Add form fields
	formFields.AddInputField("To Nodes", toNodes, 30, nil, func(text string) {
	})

	formFields.AddInputField("Jetstream To Node", "", 30, nil, func(text string) {
		// Value will be read directly from form when needed
	})

	formFields.AddCheckbox("Use Detected Shell", false, func(checked bool) {
		// Value will be read directly from form when needed
	})

	formFields.AddInputField("Method", "cliCommand", 30, nil, func(text string) {
		// Value will be read directly from form when needed
	})

	formFields.AddInputField("Method Arg 1", methodArgs[0], 30, nil, func(text string) {
		methodArgs[0] = text
	})

	formFields.AddInputField("Method Arg 2", methodArgs[1], 30, nil, func(text string) {
		methodArgs[1] = text
	})

	formFields.AddInputField("Method Arg 3", methodArgs[2], 30, nil, func(text string) {
		methodArgs[2] = text
	}).SetFieldStyle(tcell.StyleDefault.Blink(true))

	formFields.AddInputField("Method Timeout", "3", 30, nil, func(text string) {
		// Value will be read directly from form when needed
	})

	formFields.AddInputField("Reply Method", "webUI", 30, nil, func(text string) {
		// Value will be read directly from form when needed
	})

	formFields.AddInputField("ACK Timeout", "0", 30, nil, func(text string) {
		// Value will be read directly from form when needed
	})

	// Subscribe to responses
	fromNode := "btdev1"

	// Add buttons
	formFields.AddButton("Send", func() {
		// Get and save the current command to history
		currentCmd := formFields.GetFormItemByLabel("Method Arg 3").(*tview.InputField).GetText()
		if currentCmd != "" {
			// Add to history if not already present
			found := false
			for _, cmd := range historyCommands {
				if cmd == currentCmd {
					found = true
					break
				}
			}
			if !found {
				historyCommands = append([]string{currentCmd}, historyCommands...)
				historyList.Clear()
				for _, cmd := range historyCommands {
					historyList.AddItem(cmd, "", 0, nil).ShowSecondaryText(false)
				}
			}
		}

		// Get current values from form
		methodTimeout, _ := strconv.Atoi(formFields.GetFormItemByLabel("Method Timeout").(*tview.InputField).GetText())
		ackTimeout, _ := strconv.Atoi(formFields.GetFormItemByLabel("ACK Timeout").(*tview.InputField).GetText())

		// Get and clean toNodes
		toNodesText := formFields.GetFormItemByLabel("To Nodes").(*tview.InputField).GetText()
		toNodes := strings.Split(toNodesText, ",")
		// Trim spaces and filter out empty nodes
		// cleanedToNodes := []string{}
		// for _, node := range toNodes {
		// 	if trimmed := strings.TrimSpace(node); trimmed != "" {
		// 		cleanedToNodes = append(cleanedToNodes, trimmed)
		// 	}
		// }

		cmd := CommandData{
			// ToNodes: The values of toNodes will be used to fill toNode field when sending below.
			JetstreamToNode:  formFields.GetFormItemByLabel("Jetstream To Node").(*tview.InputField).GetText(),
			UseDetectedShell: formFields.GetFormItemByLabel("Use Detected Shell").(*tview.Checkbox).IsChecked(),
			Method:           formFields.GetFormItemByLabel("Method").(*tview.InputField).GetText(),
			MethodArgs:       methodArgs,
			MethodTimeout:    methodTimeout,
			FromNode:         fromNode,
			ReplyMethod:      formFields.GetFormItemByLabel("Reply Method").(*tview.InputField).GetText(),
			ACKTimeout:       ackTimeout,
		}

		// Send command to each node
		for _, node := range toNodes {
			cmd.ToNode = node

			// ---------------------------

			var filePathToOpen string
			foundFile := false

			if strings.Contains(cmd.MethodArgs[2], "{{CTRL_FILE:") {
				foundFile = true

				// Example to split:
				// echo {{CTRL_FILE:/somedir/msg_file.yaml}}>ctrlfile.txt
				//
				// Split at colon. We want the part after.
				ss := strings.Split(cmd.MethodArgs[2], ":")
				// Split at "}}",so pos [0] in the result contains just the file path.
				sss := strings.Split(ss[1], "}}")
				filePathToOpen = sss[0]

			}

			if foundFile {

				fh, err := os.Open(filePathToOpen)
				if err != nil {
					fmt.Fprintf(outputView, "Error opening file: %v\n", err)
					return
				}
				defer fh.Close()

				b, err := io.ReadAll(fh)
				if err != nil {
					fmt.Fprintf(outputView, "Error reading file: %v\n", err)
					return
				}

				// Replace the {{CTRL_FILE}} with the actual content read from file.
				re := regexp.MustCompile(`(.*)({{CTRL_FILE.*}})(.*)`)
				cmd.MethodArgs[2] = re.ReplaceAllString(cmd.MethodArgs[2], `${1}`+string(b)+`${3}`)

				fmt.Fprintf(outputView, "DEBUG: Replaced {{CTRL_FILE}} with file content: %s\n", cmd.MethodArgs[2])
				// ---

			}

			// ---------------------------

			jsonData, err := json.Marshal(cmd)
			if err != nil {
				fmt.Fprintf(outputView, "Error creating command: %v\n", err)
				return
			}

			subject := fmt.Sprintf("%s.%s", strings.TrimSpace(node), cmd.Method)

			if err := nc.Publish(subject, jsonData); err != nil {
				fmt.Fprintf(outputView, "Error sending to %s: %v\n", node, err)
			} else {
				// fmt.Fprintf(outputView, "*** Command %s sent to %s, subject: %s\n", jsonData, node, subject)
			}
		}
	})

	// Layout setup - adjust the proportion between form and output
	mainFlex.AddItem(topFlex, 0, 7, true)     // Form and history section
	mainFlex.AddItem(outputView, 0, 5, false) // Output takes remaining space

	return mainFlex
}

func createNodesPage() tview.Primitive {
	// Create an empty page for now
	return tview.NewBox().SetBorder(true).SetTitle("Nodes")
}

func updateFilesList() {
	// Clear current list
	// filesList.Clear()

	// Create tui/files directory if it doesn't exist
	filesDir := "files"

	// Read all files from the directory
	files, err := os.ReadDir(filesDir)
	if err != nil {
		fmt.Fprintf(outputView, "Error reading files directory: %v\n", err)
		return
	}

	if filesList == nil {
		fmt.Printf("FILES FILE FILE: %v\n", files)
		os.Exit(1)
	}

	// Add each file to the list
	for _, file := range files {
		//fmt.Fprintf(outputView, "DEBUG: File: %s\n", file.Name())
		//if !file.IsDir() {
		filesList.AddItem(file.Name(), filepath.Join(filesDir, file.Name()), 0, nil)
		//}
	}
}

func main() {
	app = tview.NewApplication()

	// Connect to NATS
	var err error
	nc, err = nats.Connect("nats://localhost:4222")
	if err != nil {
		panic(err)
	}
	defer nc.Close()

	fromNode := "btdev1"
	// --------------------------
	// fmt.Fprintf(outputView, "DEBUG: BEFORE SUBSCRIBE\n")
	sub, err := nc.Subscribe(fromNode+".webUI", func(msg *nats.Msg) {
		var jsonMsg struct {
			Data string `json:"data"`
		}
		if err := json.Unmarshal(msg.Data, &jsonMsg); err != nil {
			app.QueueUpdateDraw(func() {
				fmt.Fprintf(outputView, "Error decoding message: %v\n", err)
			})
			return
		}

		// Decode base64 data
		decoded, err := base64.StdEncoding.DecodeString(jsonMsg.Data)
		if err != nil {
			app.QueueUpdateDraw(func() {
				fmt.Fprintf(outputView, "Error decoding base64: %v\n", err)
			})
			return
		}

		app.QueueUpdateDraw(func() {
			fmt.Fprintf(outputView, "\nReceived message on %s:\n%s\n", msg.Subject, string(decoded))
		})
	})
	// fmt.Fprintf(outputView, "DEBUG: AFTER SUBSCRIBE\n")
	if err != nil {
		panic(err)
	}
	defer func() {
		sub.Unsubscribe()
	}()
	// --------------------------

	// Create pages to manage multiple screens
	pages := tview.NewPages()

	// Create menu
	menu := tview.NewList().
		AddItem("Commands", "Send commands to nodes", 'c', func() {
			pages.SwitchToPage("commands")
		}).ShowSecondaryText(false).
		AddItem("Nodes", "View and manage nodes", 'n', func() {
			pages.SwitchToPage("nodes")
		}).ShowSecondaryText(false).
		AddItem("Quit", "Press to exit", 'q', func() {
			app.Stop()
		})
	menu.SetBorder(true).SetTitle("Menu")

	// Create layout with menu on left and pages on right
	flex := tview.NewFlex().
		AddItem(menu, 20, 1, true).
		AddItem(pages, 0, 1, false)

	// Add pages
	pages.AddPage("commands", createCommandPage(), true, true)
	pages.AddPage("nodes", createNodesPage(), true, false)

	// Global keyboard shortcuts
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			// If on a page, go back to menu
			if menu.HasFocus() {
				app.Stop()
			} else {
				app.SetFocus(menu)
			}
			return nil
		case tcell.KeyTab:
			// Toggle between menu and current page
			if menu.HasFocus() {
				app.SetFocus(pages)
			} else {
				app.SetFocus(menu)
			}
			return nil
		}
		return event
	})

	// Update files list at startup
	updateFilesList()

	if err := app.SetRoot(flex, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}
}
