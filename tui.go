package steward

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

type tui struct {
}

func newTui() (*tui, error) {
	s := tui{}
	return &s, nil
}

type slide struct {
	name      string
	key       tcell.Key
	primitive tview.Primitive
}

func (s *tui) Start() error {
	pages := tview.NewPages()

	app := tview.NewApplication()
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyF1 {
			pages.SwitchToPage("info")
			return nil
		} else if event.Key() == tcell.KeyF2 {
			pages.SwitchToPage("message")
			return nil
		}
		return event
	})

	info := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)

	// The slides to draw, and their name.
	// NB: This slice is being looped over further below, to create the menu
	// elements. If adding a new slide, make sure that slides are ordererin
	// chronological order, so we can auto generate the info menu with it's
	// corresponding F key based on the slice index+1.
	slides := []slide{
		{name: "info", key: tcell.KeyF1, primitive: infoSlide(app)},
		{name: "message", key: tcell.KeyF2, primitive: messageSlide(app)},
	}

	// Add on page for each slide.
	for i, v := range slides {
		if i == 0 {
			pages.AddPage(v.name, v.primitive, true, true)
			fmt.Fprintf(info, " F%v:%v  ", i+1, v.name)
			continue
		}

		pages.AddPage(v.name, v.primitive, true, false)
		fmt.Fprintf(info, " F%v:%v  ", i+1, v.name)
	}

	// Create the main layout.
	layout := tview.NewFlex()
	//layout.SetBorder(true)
	layout.SetDirection(tview.FlexRow).
		AddItem(pages, 0, 10, true).
		AddItem(info, 1, 1, false)

	if err := app.SetRoot(layout, true).EnableMouse(true).Run(); err != nil {
		panic(err)
	}

	return nil
}

func infoSlide(app *tview.Application) tview.Primitive {
	flex := tview.NewFlex()
	flex.SetTitle("info")
	flex.SetBorder(true)

	textView := tview.NewTextView()
	flex.AddItem(textView, 0, 1, false)

	fmt.Fprintf(textView, "Information page for Stew.\n")

	return flex
}

func messageSlide(app *tview.Application) tview.Primitive {
	type pageMessage struct {
		flex          *tview.Flex
		msgInputForm  *tview.Form
		msgOutputForm *tview.TextView
		logForm       *tview.TextView
	}

	p := pageMessage{}
	app = tview.NewApplication()

	p.msgInputForm = tview.NewForm()
	p.msgInputForm.SetBorder(true).SetTitle("Request values").SetTitleAlign(tview.AlignLeft)

	p.msgOutputForm = tview.NewTextView()
	p.msgOutputForm.SetBorder(true).SetTitle("Message output").SetTitleAlign(tview.AlignLeft)
	p.msgOutputForm.SetChangedFunc(func() {
		// Will cause the log window to be redrawn as soon as
		// new output are detected.
		app.Draw()
	})

	p.logForm = tview.NewTextView()
	p.logForm.SetBorder(true).SetTitle("Log/Status").SetTitleAlign(tview.AlignLeft)
	p.logForm.SetChangedFunc(func() {
		// Will cause the log window to be redrawn as soon as
		// new output are detected.
		app.Draw()
	})

	// Create a flex layout.
	//
	// First create the outer flex layout.
	p.flex = tview.NewFlex().SetDirection(tview.FlexRow).
		// Add the top windows with columns.
		AddItem(tview.NewFlex().SetDirection(tview.FlexColumn).
			AddItem(p.msgInputForm, 0, 10, false).
			AddItem(p.msgOutputForm, 0, 10, false),
			0, 10, false).
		// Add the bottom log window.
		AddItem(tview.NewFlex().
			AddItem(p.logForm, 0, 2, false),
			0, 2, false)

	// Check that the message struct used within stew are up to date, and
	// consistent with the fields used in the main Steward message file.
	// If it throws an error here we need to update the msg struct type,
	// or add a case for the field to except.
	err := compareMsgAndMessage()
	if err != nil {
		log.Printf("%v\n", err)
		os.Exit(1)
	}

	m := msg{}

	// Loop trough all the fields of the Message struct, and create
	// a an input field or dropdown selector for each field.
	// If a field of the struct is not defined below, it will be
	// created a "no defenition" element, so it we can easily spot
	// Message fields who miss an item in the form.
	//
	// INFO: The reason that reflect are being used here is to have
	// a simple way of detecting that we are creating form fields
	// for all the fields in the struct. If we have forgot'en one
	// it will create a "no case" field in the console, to easily
	// detect that a struct field are missing a defenition below.

	mRefVal := reflect.ValueOf(m)

	for i := 0; i < mRefVal.NumField(); i++ {
		var err error
		values := []string{"1", "2"}

		fieldName := mRefVal.Type().Field(i).Name

		switch fieldName {
		case "ToNode":
			// Get nodes from file.
			values, err = getNodeNames("nodeslist.cfg")
			if err != nil {
				// TODO: Handle error here, exit ?
				return nil
			}

			item := tview.NewDropDown()
			item.SetLabelColor(tcell.ColorIndianRed)
			item.SetLabel(fieldName).SetOptions(values, nil)
			p.msgInputForm.AddFormItem(item)
			//c.msgForm.AddDropDown(mRefVal.Type().Field(i).Name, values, 0, nil).SetItemPadding(1)
		case "ID":
			// This value is automatically assigned by steward.
		case "Data":
			value := `"bash","-c","..."`
			p.msgInputForm.AddInputField(fieldName, value, 30, nil, nil)
		case "Method":
			var m Method
			ma := m.GetMethodsAvailable()
			values := []string{}
			for k := range ma.Methodhandlers {
				values = append(values, string(k))
			}
			p.msgInputForm.AddDropDown(fieldName, values, 0, nil).SetItemPadding(1)
		case "ReplyMethod":
			var m Method
			rm := m.GetReplyMethods()
			values := []string{}
			for _, k := range rm {
				values = append(values, string(k))
			}
			p.msgInputForm.AddDropDown(fieldName, values, 0, nil).SetItemPadding(1)
		case "ACKTimeout":
			value := 30
			p.msgInputForm.AddInputField(fieldName, fmt.Sprintf("%d", value), 30, validateInteger, nil)
		case "Retries":
			value := 1
			p.msgInputForm.AddInputField(fieldName, fmt.Sprintf("%d", value), 30, validateInteger, nil)
		case "ReplyACKTimeout":
			value := 30
			p.msgInputForm.AddInputField(fieldName, fmt.Sprintf("%d", value), 30, validateInteger, nil)
		case "ReplyRetries":
			value := 1
			p.msgInputForm.AddInputField(fieldName, fmt.Sprintf("%d", value), 30, validateInteger, nil)
		case "MethodTimeout":
			value := 120
			p.msgInputForm.AddInputField(fieldName, fmt.Sprintf("%d", value), 30, validateInteger, nil)
		case "Directory":
			value := "/some-dir/"
			p.msgInputForm.AddInputField(fieldName, value, 30, nil, nil)
		case "FileName":
			value := ".log"
			p.msgInputForm.AddInputField(fieldName, value, 30, nil, nil)

		default:
			// Add a no definition fields to the form if a a field within the
			// struct were missing an action above, so we can easily detect
			// if there is missing a case action for one of the struct fields.
			p.msgInputForm.AddDropDown("error: no case for: "+fieldName, values, 0, nil).SetItemPadding(1)
		}

	}

	// Add Buttons below the message fields. Like Generate and Exit.
	p.msgInputForm.
		// Add a generate button, which when pressed will loop through all the
		// message form items, and if found fill the value into a msg struct,
		// and at last write it to a file.
		//
		// TODO: Should also add a write directly to socket here.
		AddButton("generate to console", func() {
			// fh, err := os.Create("message.json")
			// if err != nil {
			// 	log.Fatalf("error: failed to create test.log file: %v\n", err)
			// }
			// defer fh.Close()

			p.msgOutputForm.Clear()
			fh := p.msgOutputForm

			m := msg{}
			// Loop trough all the form fields
			for i := 0; i < p.msgInputForm.GetFormItemCount(); i++ {
				fi := p.msgInputForm.GetFormItem(i)
				label, value := getLabelAndValue(fi)

				switch label {
				case "ToNode":
					if value == "" {
						fmt.Fprintf(p.logForm, "%v : error: missing ToNode \n", time.Now().Format("Mon Jan _2 15:04:05 2006"))
						return
					}

					m.ToNode = Node(value)
				case "Data":
					// Split the comma separated string into a
					// and remove the start and end ampersand.
					sp := strings.Split(value, ",")

					var data []string

					for _, v := range sp {
						// Check if format is correct, return if not.
						pre := strings.HasPrefix(v, "\"")
						suf := strings.HasSuffix(v, "\"")
						if !pre || !suf {
							fmt.Fprintf(p.logForm, "%v : error: missing or malformed format for command, should be \"cmd\",\"arg1\",\"arg2\" ...\n", time.Now().Format("Mon Jan _2 15:04:05 2006"))
							return
						}
						// Remove leading and ending ampersand.
						v = v[1:]
						v = strings.TrimSuffix(v, "\"")

						data = append(data, v)
					}

					m.Data = data
				case "Method":
					m.Method = Method(value)
				case "ReplyMethod":
					m.ReplyMethod = Method(value)
				case "ACKTimeout":
					v, _ := strconv.Atoi(value)
					m.ACKTimeout = v
				case "Retries":
					v, _ := strconv.Atoi(value)
					m.Retries = v
				case "ReplyACKTimeout":
					v, _ := strconv.Atoi(value)
					m.ReplyACKTimeout = v
				case "ReplyRetries":
					v, _ := strconv.Atoi(value)
					m.ReplyRetries = v
				case "MethodTimeout":
					v, _ := strconv.Atoi(value)
					m.MethodTimeout = v
				case "Directory":
					m.Directory = value
				case "FileName":
					m.FileName = value

				default:
					fmt.Fprintf(p.logForm, "%v : error: did not find case defenition for how to handle the \"%v\" within the switch statement\n", time.Now().Format("Mon Jan _2 15:04:05 2006"), label)
				}
			}
			msgs := []msg{}
			msgs = append(msgs, m)

			msgsIndented, err := json.MarshalIndent(msgs, "", "    ")
			if err != nil {
				fmt.Fprintf(p.logForm, "%v : error: jsonIndent failed: %v\n", time.Now().Format("Mon Jan _2 15:04:05 2006"), err)
			}

			_, err = fh.Write(msgsIndented)
			if err != nil {
				fmt.Fprintf(p.logForm, "%v : error: write to fh failed: %v\n", time.Now().Format("Mon Jan _2 15:04:05 2006"), err)
			}
		}).

		// Add exit button.
		AddButton("exit", func() {
			app.Stop()
		})

	app.SetFocus(p.msgInputForm)

	return p.flex
}

// Check and compare all the fields of the main message struct
// used in Steward, and the message struct used in Stew that they are
// equal.
// If they are not equal an error will be returned to the user with
// the name of the field that was missing in the Stew message struct.
//
// Some of the fields in the Steward Message struct are used by the
// system for control, and not needed when creating an initial message
// template, and we can add case statements for those fields below
// that we do not wan't to check.
func compareMsgAndMessage() error {
	stewardMessage := Message{}
	stewMsg := msg{}

	stewardRefVal := reflect.ValueOf(stewardMessage)
	stewRefVal := reflect.ValueOf(stewMsg)

	// Loop trough all the fields of the Message struct.
	for i := 0; i < stewardRefVal.NumField(); i++ {
		found := false

		for ii := 0; ii < stewRefVal.NumField(); ii++ {
			if stewardRefVal.Type().Field(i).Name == stewRefVal.Type().Field(ii).Name {
				found = true
				break
			}
		}

		// Case statements for the fields we don't care about for
		// the message template.
		if !found {
			switch stewardRefVal.Type().Field(i).Name {
			case "ID":
				// Not used in message template.
			case "FromNode":
				// Not used in message template.
			case "PreviousMessage":
				// Not used in message template.
			case "done":
				// Not used in message template.
			default:
				return fmt.Errorf("error: %v within the steward Message struct were not found in the stew msg struct", stewardRefVal.Type().Field(i).Name)

			}
		}
	}

	return nil
}

// Will return the Label And the text Value of an input or dropdown form field.
func getLabelAndValue(fi tview.FormItem) (string, string) {
	var label string
	var value string
	switch v := fi.(type) {
	case *tview.InputField:
		value = v.GetText()
		label = v.GetLabel()
	case *tview.DropDown:
		label = v.GetLabel()
		_, value = v.GetCurrentOption()
	}

	return label, value
}

// Check if number is int.
func validateInteger(text string, ch rune) bool {
	if text == "-" {
		return true
	}
	_, err := strconv.Atoi(text)
	return err == nil
}

// getNodes will load all the node names from a file, and return a slice of
// string values, each representing a unique node.
func getNodeNames(filePath string) ([]string, error) {

	fh, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error: unable to open node file: %v", err)
	}
	defer fh.Close()

	nodes := []string{}

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		node := scanner.Text()
		nodes = append(nodes, node)
	}

	return nodes, nil
}

// -----------------------

// Creating a copy of the real Message struct here to use within the
// field specification, but without the control kind of fields from
// the original to avoid changing them to pointer values in the main
// struct which would be needed when json marshaling to omit those
// empty fields.
type msg struct {
	// The node to send the message to
	ToNode Node `json:"toNode" yaml:"toNode"`
	// The actual data in the message
	Data []string `json:"data" yaml:"data"`
	// Method, what is this message doing, etc. CLI, syslog, etc.
	Method Method `json:"method" yaml:"method"`
	// ReplyMethod, is the method to use for the reply message.
	// By default the reply method will be set to log to file, but
	// you can override it setting your own here.
	ReplyMethod Method `json:"replyMethod" yaml:"replyMethod"`
	// From what node the message originated
	ACKTimeout int `json:"ACKTimeout" yaml:"ACKTimeout"`
	// Resend retries
	Retries int `json:"retries" yaml:"retries"`
	// The ACK timeout of the new message created via a request event.
	ReplyACKTimeout int `json:"replyACKTimeout" yaml:"replyACKTimeout"`
	// The retries of the new message created via a request event.
	ReplyRetries int `json:"replyRetries" yaml:"replyRetries"`
	// Timeout for long a process should be allowed to operate
	MethodTimeout int `json:"methodTimeout" yaml:"methodTimeout"`
	// Directory is a string that can be used to create the
	//directory structure when saving the result of some method.
	// For example "syslog","metrics", or "metrics/mysensor"
	// The type is typically used in the handler of a method.
	Directory string `json:"directory" yaml:"directory"`
	// FileName is used to be able to set a wanted extension
	// on a file being saved as the result of data being handled
	// by a method handler.
	FileName string `json:"fileName" yaml:"fileName"`
}
