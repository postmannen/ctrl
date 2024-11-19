package ctrl

import (
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"path/filepath"

	"github.com/hpcloud/tail"
)

func reqWriteFileOrSocket(isAppend bool, proc process, message Message) error {
	// If it was a request type message we want to check what the initial messages
	// method, so we can use that in creating the file name to store the data.
	fileName, folderTree := selectFileNaming(message, proc)
	file := filepath.Join(folderTree, fileName)

	fmt.Printf("******************* DEBUG: CHECK IF SOCKET OR FILE: %v\n", file)

	// log.Fatalf("EXITING\n")

	// Check the file is a unix socket, and if it is we write the
	// data to the socket instead of writing it to a normal file.
	fi, err := os.Stat(file)
	if err == nil && !os.IsNotExist(err) {
		er := fmt.Errorf("info: reqWriteFileOrSocket: failed to stat file, but will continue: %v", folderTree)
		proc.errorKernel.logDebug(er)
	}

	if fi != nil && fi.Mode().Type() == fs.ModeSocket {
		socket, err := net.Dial("unix", file)
		if err != nil {
			er := fmt.Errorf("error: methodToFile/Append could to open socket file for writing: subject:%v, folderTree: %v, %v", proc.subject, folderTree, err)
			return er
		}
		defer socket.Close()

		_, err = socket.Write([]byte(message.Data))
		if err != nil {
			er := fmt.Errorf("error: methodToFile/Append could not write to socket: subject:%v, folderTree: %v, %v", proc.subject, folderTree, err)
			return er
		}

	}

	// The file is a normal file and not a socket.
	// Check if folder structure exist, if not create it.
	if _, err := os.Stat(folderTree); os.IsNotExist(err) {
		err := os.MkdirAll(folderTree, 0770)
		if err != nil {
			er := fmt.Errorf("error: methodToFile/Append failed to create toFile directory tree: subject:%v, folderTree: %v, %v", proc.subject, folderTree, err)

			return er
		}

		er := fmt.Errorf("info: Creating subscribers data folder at %v", folderTree)
		proc.errorKernel.logDebug(er)
	}

	var fileFlag int
	switch isAppend {
	case true:
		fileFlag = os.O_APPEND | os.O_RDWR | os.O_CREATE | os.O_SYNC
	case false:
		fileFlag = os.O_CREATE | os.O_RDWR | os.O_TRUNC
	}

	// Open file and write data.
	f, err := os.OpenFile(file, fileFlag, 0755)
	if err != nil {
		er := fmt.Errorf("error: methodToFile/Append: failed to open file, check that you've specified a value for fileName in the message: directory: %v, fileName: %v, %v", message.Directory, message.FileName, err)

		return er
	}
	defer f.Close()

	_, err = f.Write(message.Data)
	f.Sync()
	if err != nil {
		er := fmt.Errorf("error: methodToFile/Append: failed to write to file: file: %v, %v", file, err)

		return er
	}

	return nil
}

// Handle appending data to file.
func methodFileAppend(proc process, message Message, node string) ([]byte, error) {
	err := reqWriteFileOrSocket(true, proc, message)
	proc.errorKernel.errSend(proc, message, err, logWarning)

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// -----

// Handle writing to a file. Will truncate any existing data if the file did already
// exist.
func methodToFile(proc process, message Message, node string) ([]byte, error) {
	err := reqWriteFileOrSocket(false, proc, message)
	proc.errorKernel.errSend(proc, message, err, logWarning)

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// --- methodTailFile

// handler to run a tailing of files with timeout context. The handler will
// return the output of the command run back to the calling publisher
// as a new message.
func methodTailFile(proc process, message Message, node string) ([]byte, error) {
	inf := fmt.Errorf("<--- TailFile REQUEST received from: %v, containing: %v", message.FromNode, message.Data)
	proc.errorKernel.logDebug(inf)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		switch {
		case len(message.MethodArgs) < 1:
			er := fmt.Errorf("error: methodTailFile: got <1 number methodArgs")
			proc.errorKernel.errSend(proc, message, er, logWarning)

			return
		}

		fp := message.MethodArgs[0]

		// var ctx context.Context
		// var cancel context.CancelFunc

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)

		// Note: Replacing the 0 timeout with specific timeout.
		// if message.MethodTimeout != 0 {
		// 	ctx, cancel = context.WithTimeout(proc.ctx, time.Second*time.Duration(message.MethodTimeout))
		// } else {
		// 	ctx, cancel = context.WithCancel(proc.ctx)
		// }

		outCh := make(chan []byte)
		t, err := tail.TailFile(fp, tail.Config{Follow: true, Location: &tail.SeekInfo{
			Offset: 0,
			Whence: io.SeekEnd,
		}})
		if err != nil {
			er := fmt.Errorf("error: methodREQToTailFile: tailFile: %v", err)
			proc.errorKernel.errSend(proc, message, er, logWarning)
		}

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			for {
				select {
				case line := <-t.Lines:
					outCh <- []byte(line.Text + "\n")
				case <-ctx.Done():
					return
				}

			}
		}()

		for {
			select {
			case <-ctx.Done():
				cancel()
				// Close the lines channel so we exit the reading lines
				// go routine.
				// close(t.Lines)
				er := fmt.Errorf("info: method timeout reached REQTailFile, canceling: %v", message.MethodArgs)
				proc.errorKernel.infoSend(proc, message, er)

				return
			case out := <-outCh:

				// Prepare and queue for sending a new message with the output
				// of the action executed.
				newReplyMessage(proc, message, out)
			}
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
