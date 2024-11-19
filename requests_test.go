package ctrl

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	natsserver "github.com/nats-io/nats-server/v2/server"
)

var logging = flag.Bool("logging", false, "set to true to enable the normal logger of the package")
var persistTmp = flag.Bool("persistTmp", false, "set to true to persist the tmp folder")

var tstSrv *server
var tstConf *Configuration
var tstNats *natsserver.Server
var tstTempDir string

func TestMain(m *testing.M) {
	flag.Parse()

	if *persistTmp {
		tstTempDir = "tmp"
	} else {
		tstTempDir = os.TempDir()
	}

	// NB: Forcing this for now.
	tstTempDir = "tmp"

	tstNats = newNatsServerForTesting(42222)
	if err := natsserver.Run(tstNats); err != nil {
		natsserver.PrintAndDie(err.Error())
	}

	tstSrv, tstConf = newServerForTesting("127.0.0.1:42222", tstTempDir)
	tstSrv.Start()

	exitCode := m.Run()

	tstSrv.Stop()
	tstNats.Shutdown()

	os.Exit(exitCode)
}

func newServerForTesting(addressAndPort string, testFolder string) (*server, *Configuration) {

	// Start ctrl instance
	// ---------------------------------------
	// tempdir := t.TempDir()

	// Create the config to run a ctrl instance.
	//tempdir := "./tmp"
	conf := newConfigurationDefaults()
	if *logging {
		conf.LogLevel = "warning"
	}
	conf.BrokerAddress = addressAndPort
	conf.NodeName = "central"
	conf.CentralNodeName = "central"
	conf.ConfigFolder = testFolder
	conf.SubscribersDataFolder = testFolder
	conf.SocketFolder = testFolder
	conf.SubscribersDataFolder = testFolder
	conf.DatabaseFolder = testFolder
	conf.StartProcesses.IsCentralErrorLogger = true
	conf.StartProcesses.IsCentralAuth = true
	conf.EnableDebug = false
	conf.LogLevel = "none"

	ctrlServer, err := NewServer(&conf, "test")
	if err != nil {
		log.Fatalf(" * failed: could not start the ctrl instance %v\n", err)
	}

	return ctrlServer, &conf
}

// Start up the nats-server message broker for testing purposes.
func newNatsServerForTesting(port int) *natsserver.Server {
	// Start up the nats-server message broker.
	nsOpt := &natsserver.Options{
		Host: "127.0.0.1",
		Port: port,
	}

	ns, err := natsserver.NewServer(nsOpt)
	if err != nil {
		log.Fatalf(" * failed: could not start the nats-server %v\n", err)
	}

	return ns
}

// Write message to socket for testing purposes.
func writeMsgsToSocketTest(conf *Configuration, messages []Message, t *testing.T) {
	js, err := json.Marshal(messages)
	if err != nil {
		t.Fatalf("writeMsgsToSocketTest: %v\n ", err)
	}

	socket, err := net.Dial("unix", filepath.Join(conf.SocketFolder, "ctrl.sock"))
	if err != nil {
		t.Fatalf(" * failed: could to open socket file for writing: %v\n", err)
	}
	defer socket.Close()

	_, err = socket.Write(js)
	if err != nil {
		t.Fatalf(" * failed: could not write to socket: %v\n", err)
	}

}

func TestRequest(t *testing.T) {
	if !*logging {
		log.SetOutput(io.Discard)
	}

	type containsOrEquals int
	const (
		REQTestContains containsOrEquals = iota
		REQTestEquals   containsOrEquals = iota
		fileContains    containsOrEquals = iota
	)

	type viaSocketOrCh int
	const (
		viaSocket viaSocketOrCh = iota
		viaCh     viaSocketOrCh = iota
	)

	type test struct {
		info    string
		message Message
		want    []byte
		containsOrEquals
		viaSocketOrCh
	}

	// Web server for testing.
	{
		h := func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("web page content"))
		}
		http.HandleFunc("/", h)

		go func() {
			http.ListenAndServe(":10080", nil)
		}()
	}

	tests := []test{
		{
			info: "hello test",
			message: Message{
				ToNode:        "errorCentral",
				FromNode:      "errorCentral",
				Method:        ErrorLog,
				MethodArgs:    []string{},
				MethodTimeout: 5,
				Data:          []byte("error data"),
				// ReplyMethod:   REQTest,
				Directory: "error_log",
				FileName:  "error.results",
			}, want: []byte("error data"),
			containsOrEquals: fileContains,
			viaSocketOrCh:    viaCh,
		},
		{
			info: "hello test",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        Hello,
				MethodArgs:    []string{},
				MethodTimeout: 5,
				// ReplyMethod:   REQTest,
				Directory: "test",
				FileName:  "hello.results",
			}, want: []byte("Received hello from \"central\""),
			containsOrEquals: fileContains,
			viaSocketOrCh:    viaCh,
		},
		{
			info: "cliCommand test, echo gris",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        CliCommand,
				MethodArgs:    []string{"bash", "-c", "echo gris"},
				MethodTimeout: 5,
				ReplyMethod:   Test,
			}, want: []byte("gris"),
			containsOrEquals: REQTestEquals,
			viaSocketOrCh:    viaCh,
		},
		{
			info: "cliCommand test via socket, echo sau",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        CliCommand,
				MethodArgs:    []string{"bash", "-c", "echo sau"},
				MethodTimeout: 5,
				ReplyMethod:   Test,
			}, want: []byte("sau"),
			containsOrEquals: REQTestEquals,
			viaSocketOrCh:    viaSocket,
		},
		{
			info: "cliCommand test, echo sau, result in file",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        CliCommand,
				MethodArgs:    []string{"bash", "-c", "echo sau"},
				MethodTimeout: 5,
				ReplyMethod:   File,
				Directory:     "test",
				FileName:      "file1.result",
			}, want: []byte("sau"),
			containsOrEquals: fileContains,
			viaSocketOrCh:    viaCh,
		},
		{
			info: "cliCommand test, echo several, result in file continous",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        CliCommand,
				MethodArgs:    []string{"bash", "-c", "echo giraff && echo sau && echo apekatt"},
				MethodTimeout: 5,
				ReplyMethod:   File,
				Directory:     "test",
				FileName:      "file2.result",
			}, want: []byte("sau"),
			containsOrEquals: fileContains,
			viaSocketOrCh:    viaCh,
		},
		{
			info: "httpGet test, localhost:10080",
			message: Message{
				ToNode:        "central",
				FromNode:      "central",
				Method:        HttpGet,
				MethodArgs:    []string{"http://localhost:10080"},
				MethodTimeout: 5,
				ReplyMethod:   Test,
			}, want: []byte("web page content"),
			containsOrEquals: REQTestContains,
			viaSocketOrCh:    viaCh,
		},
		// TODO: Check out this one why it fails, and also why I'm checking for REQHttpGet here ??
		//{
		//	info: "opProcessList test",
		//	message: Message{
		//		ToNode:        "central",
		//		FromNode:      "central",
		//		Method:        REQOpProcessList,
		//		MethodArgs:    []string{},
		//		MethodTimeout: 5,
		//		ReplyMethod:   REQTest,
		//	}, want: []byte("central.REQHttpGet"),
		//	containsOrEquals: REQTestContains,
		//	viaSocketOrCh:    viaCh,
		//},
	}

	// Range over the tests defined, and execute them, one at a time.
	for _, tt := range tests {
		switch tt.viaSocketOrCh {
		case viaCh:
			sam, err := newSubjectAndMessage(tt.message)
			if err != nil {
				t.Fatalf("newSubjectAndMessage failed: %v\n", err)
			}

			tstSrv.newMessagesCh <- []subjectAndMessage{sam}

		case viaSocket:
			msgs := []Message{tt.message}
			writeMsgsToSocketTest(tstConf, msgs, t)

		}

		switch tt.containsOrEquals {
		case REQTestEquals:
			result := <-tstSrv.errorKernel.testCh
			resStr := string(result)
			resStr = strings.TrimSuffix(resStr, "\n")
			result = []byte(resStr)

			if !bytes.Equal(result, tt.want) {
				t.Fatalf(" \U0001F631  [FAILED]	:%v : want: %v, got: %v\n", tt.info, string(tt.want), string(result))
			}
			t.Logf(" \U0001f600 [SUCCESS]	: %v\n", tt.info)

		case REQTestContains:
			result := <-tstSrv.errorKernel.testCh
			resStr := string(result)
			resStr = strings.TrimSuffix(resStr, "\n")
			result = []byte(resStr)

			if !strings.Contains(string(result), string(tt.want)) {
				t.Fatalf(" \U0001F631  [FAILED]	:%v : want: %v, got: %v\n", tt.info, string(tt.want), string(result))
			}
			t.Logf(" \U0001f600 [SUCCESS]	: %v\n", tt.info)

		case fileContains:
			resultFile := filepath.Join(tstConf.SubscribersDataFolder, tt.message.Directory, string(tt.message.FromNode), tt.message.FileName)

			found, err := findStringInFileTest(string(tt.want), resultFile)
			if err != nil || found == false {
				t.Fatalf(" \U0001F631  [FAILED]	: %v: %v\n", tt.info, err)

			}

			t.Logf(" \U0001f600 [SUCCESS]	: %v\n", tt.info)
		}
	}

	// --- Other REQ tests that does not fit well into the general table above.

	checkREQTailFileTest(tstConf, t, tstTempDir)
	checkMetricValuesTest(tstSrv, t)
	checkErrorKernelMalformedJSONtest(tstConf, t)
	checkREQCopySrc(tstConf, t, tstTempDir)
}

// Check the tailing of files type.
func checkREQTailFileTest(conf *Configuration, t *testing.T, tmpDir string) error {
	// Create a file with some content.
	fp := filepath.Join(tmpDir, "test.file")
	fh, err := os.OpenFile(fp, os.O_APPEND|os.O_RDWR|os.O_CREATE|os.O_SYNC, 0660)
	if err != nil {
		return fmt.Errorf(" * failed: unable to open temporary file: %v", err)
	}
	defer fh.Close()

	ctx, cancel := context.WithCancel(context.Background())

	// Write content to the file with specified intervals.
	go func() {
		for i := 1; i <= 10; i++ {
			_, err = fh.Write([]byte("some file content\n"))
			if err != nil {
				fmt.Printf(" * failed: writing to temporary file: %v\n", err)
			}
			fh.Sync()
			time.Sleep(time.Millisecond * 500)

			// Check if we've received a done, else default to continuing.
			select {
			case <-ctx.Done():
				return
			default:
				// no done received, we're continuing.
			}

		}
	}()

	s := `[
		{
			"directory": "tail-files",
			"fileName": "fileName.result",
			"toNode": "central",
			"methodArgs": ["` + fp + `"],
			"method":"tailFile",
			"ACKTimeout":5,
			"retries":3,
			"methodTimeout": 10
		}
	]`

	writeToSocketTest(conf, s, t)

	resultFile := filepath.Join(conf.SubscribersDataFolder, "tail-files", "central", "fileName.result")

	// Wait n times for result file to be created.
	n := 50
	for i := 0; i <= n; i++ {
		_, err := os.Stat(resultFile)
		if os.IsNotExist(err) {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if os.IsNotExist(err) && i >= n {
			cancel()
			return fmt.Errorf(" \U0001F631  [FAILED]	: checkREQTailFileTest:  no result file created for request within the given time")
		}
	}

	cancel()

	_, err = findStringInFileTest("some file content", resultFile)
	if err != nil {
		return fmt.Errorf(" \U0001F631  [FAILED]	: checkREQTailFileTest: %v", err)
	}

	t.Logf(" \U0001f600 [SUCCESS]	: checkREQTailFileTest\n")
	return nil
}

// Check the file copier.
func checkREQCopySrc(conf *Configuration, t *testing.T, tmpDir string) error {
	testFiles := 5

	for i := 1; i <= testFiles; i++ {

		// Create a file with some content.
		srcFileName := fmt.Sprintf("copysrc%v.file", i)
		srcfp := filepath.Join(tmpDir, srcFileName)
		fh, err := os.OpenFile(srcfp, os.O_APPEND|os.O_RDWR|os.O_CREATE|os.O_SYNC, 0660)
		if err != nil {
			t.Fatalf(" \U0001F631  [FAILED]	: checkREQCopySrc: unable to open temporary file: %v", err)
		}
		defer fh.Close()

		// Write content to the file.

		_, err = fh.Write([]byte("some file content\n"))
		if err != nil {
			t.Fatalf(" \U0001F631  [FAILED]	: checkREQCopySrc: writing to temporary file: %v\n", err)
		}

		dstFileName := fmt.Sprintf("copydst%v.file", i)
		dstfp := filepath.Join(tmpDir, dstFileName)

		s := `[
					{
						"toNode": "central",
						"method":"copySrc",
						"methodArgs": ["` + srcfp + `","central","` + dstfp + `","20","10"],
						"ACKTimeout":5,
						"retries":3,
						"methodTimeout": 10
					}
				]`

		writeToSocketTest(conf, s, t)

		// Wait n times for result file to be created.
		n := 50
		for i := 0; i <= n; i++ {
			_, err := os.Stat(dstfp)
			if os.IsNotExist(err) {
				time.Sleep(time.Millisecond * 100)
				continue
			}

			if os.IsNotExist(err) && i >= n {
				t.Fatalf(" \U0001F631  [FAILED]	: checkREQCopySrc:  no result file created for request within the given time")
			}
		}

		t.Logf(" \U0001f600 [SUCCESS]	: src=%v, dst=%v", srcfp, dstfp)
	}

	return nil
}

func checkMetricValuesTest(ctrlServer *server, t *testing.T) error {
	mfs, err := ctrlServer.metrics.promRegistry.Gather()
	if err != nil {
		return fmt.Errorf("error: promRegistry.gathering: %v", mfs)
	}

	if len(mfs) <= 0 {
		return fmt.Errorf("error: promRegistry.gathering: did not find any metric families: %v", mfs)
	}

	found := false
	for _, mf := range mfs {
		if mf.GetName() == "ctrl_processes_total" {
			found = true

			m := mf.GetMetric()

			if m[0].Gauge.GetValue() <= 0 {
				return fmt.Errorf("error: promRegistry.gathering: did not find any running processes in metric for processes_total : %v", m[0].Gauge.GetValue())
			}
		}
	}

	if !found {
		return fmt.Errorf("error: promRegistry.gathering: did not find specified metric processes_total")
	}

	t.Logf(" \U0001f600 [SUCCESS]	: checkMetricValuesTest")

	return nil
}

// Check errorKernel
func checkErrorKernelMalformedJSONtest(conf *Configuration, t *testing.T) error {

	// JSON message with error, missing brace.
	m := `[
		{
			"directory": "some dir",
			"fileName":"someext",
			"toNode": "somenode",
			"data": ["some data"],
			"method": "errorLog"
		missing brace here.....
	]`

	writeToSocketTest(conf, m, t)

	resultFile := filepath.Join(conf.SubscribersDataFolder, "errorLog", "errorCentral", "error.log")

	// Wait n times for error file to be created.
	n := 50
	for i := 0; i <= n; i++ {
		_, err := os.Stat(resultFile)
		if os.IsNotExist(err) {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if os.IsNotExist(err) && i >= n {
			return fmt.Errorf(" \U0001F631  [FAILED]	: checkErrorKernelMalformedJSONtest: no result file created for request within the given time")
		}
	}

	// Start checking if the result file is being updated.
	chUpdated := make(chan bool)
	go checkFileUpdated(resultFile, chUpdated)

	// We wait 5 seconds for an update, or else we fail.
	ticker := time.NewTicker(time.Second * 5)

	for {
		select {
		case <-chUpdated:
			// We got an update, so we continue to check if we find the string we're
			// looking for.
			found, err := findStringInFileTest("error: malformed json", resultFile)
			if !found && err != nil {
				return fmt.Errorf(" \U0001F631  [FAILED]	: checkErrorKernelMalformedJSONtest: %v", err)
			}

			if !found && err == nil {
				continue
			}

			if found {
				t.Logf(" \U0001f600 [SUCCESS]	: checkErrorKernelMalformedJSONtest")
				return nil
			}
		case <-ticker.C:
			return fmt.Errorf(" * failed: did not get an update in the errorKernel log file")
		}
	}
}

// Check if file are getting updated with new content.
func checkFileUpdated(fileRealPath string, fileUpdated chan bool) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println("Failed fsnotify.NewWatcher")
		return
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		//Give a true value to updated so it reads the file the first time.
		fileUpdated <- true
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
					//testing with an update chan to get updates
					fileUpdated <- true
				}
			case err := <-watcher.Errors:
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(fileRealPath)
	if err != nil {
		log.Fatal(err)
	}
	<-done
}

// Check if a file contains the given string.
func findStringInFileTest(want string, fileName string) (bool, error) {
	// Wait n seconds for the results file to be created
	n := 50

	for i := 0; i <= n; i++ {
		_, err := os.Stat(fileName)
		if os.IsNotExist(err) {
			time.Sleep(time.Millisecond * 100)
			continue
		}

		if os.IsNotExist(err) && i >= n {
			return false, fmt.Errorf(" * failed: no result file created for request within the given time\n")
		}
	}

	fh, err := os.Open(fileName)
	if err != nil {
		return false, fmt.Errorf(" * failed: could not open result file: %v", err)
	}

	result, err := io.ReadAll(fh)
	if err != nil {
		return false, fmt.Errorf(" * failed: could not read result file: %v", err)
	}

	found := strings.Contains(string(result), want)
	if !found {
		return false, nil
	}

	return true, nil
}

// Write message to socket for testing purposes.
func writeToSocketTest(conf *Configuration, messageText string, t *testing.T) {
	socket, err := net.Dial("unix", filepath.Join(conf.SocketFolder, "ctrl.sock"))
	if err != nil {
		t.Fatalf(" * failed: could to open socket file for writing: %v\n", err)
	}
	defer socket.Close()

	_, err = socket.Write([]byte(messageText))
	if err != nil {
		t.Fatalf(" * failed: could not write to socket: %v\n", err)
	}

}
