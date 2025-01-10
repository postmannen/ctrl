package ctrl

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// handler to do a Http Get.
func methodHttpGet(proc process, message Message, node string) ([]byte, error) {
	proc.errorKernel.logDebug("<--- REQHttpGet received", "fromNode", message.FromNode, "data", message.Data)

	msgForErrors := message
	msgForErrors.FileName = msgForErrors.FileName + ".error"

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		switch {
		case len(message.MethodArgs) < 1:
			er := fmt.Errorf("error: methodHttpGet: got <1 number methodArgs")
			proc.errorKernel.errSend(proc, message, er, logWarning)
			newReplyMessage(proc, msgForErrors, []byte(er.Error()))

			return
		}

		url := message.MethodArgs[0]

		client := http.Client{
			Timeout: time.Second * time.Duration(message.MethodTimeout),
		}

		// Get a context with the timeout specified in message.MethodTimeout.
		ctx, cancel := getContextForMethodTimeout(proc.ctx, message)

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			er := fmt.Errorf("error: methodHttpGet: NewRequest failed: %v, bailing out: %v", err, message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			newReplyMessage(proc, msgForErrors, []byte(er.Error()))
			cancel()
			return
		}

		outCh := make(chan []byte)

		proc.processes.wg.Add(1)
		go func() {
			defer proc.processes.wg.Done()

			resp, err := client.Do(req)
			if err != nil {
				er := fmt.Errorf("error: methodHttpGet: client.Do failed: %v, bailing out: %v", err, message.MethodArgs)
				proc.errorKernel.errSend(proc, message, er, logWarning)
				newReplyMessage(proc, msgForErrors, []byte(er.Error()))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				cancel()
				er := fmt.Errorf("error: methodHttpGet: not 200, were %#v, bailing out: %v", resp.StatusCode, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
				newReplyMessage(proc, msgForErrors, []byte(er.Error()))
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				er := fmt.Errorf("error: methodHttpGet: io.ReadAll failed : %v, methodArgs: %v", err, message.MethodArgs)
				proc.errorKernel.errSend(proc, message, er, logWarning)
				newReplyMessage(proc, msgForErrors, []byte(er.Error()))
			}

			out := body

			select {
			case outCh <- out:
			case <-ctx.Done():
				return
			}
		}()

		select {
		case <-ctx.Done():
			cancel()
			er := fmt.Errorf("error: methodHttpGet: method timed out: %v", message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			newReplyMessage(proc, msgForErrors, []byte(er.Error()))
		case out := <-outCh:
			cancel()

			// Prepare and queue for sending a new message with the output
			// of the action executed.
			newReplyMessage(proc, message, out)
		}

	}()

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
