package ctrl

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// handler to do a Http Get.
func methodREQHttpGet(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- REQHttpGet received from: %v, containing: %v", message.FromNode, message.Data)
	proc.errorKernel.logDebug(er)

	msgForErrors := message
	msgForErrors.FileName = msgForErrors.FileName + ".error"

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		switch {
		case len(message.MethodArgs) < 1:
			er := fmt.Errorf("error: methodREQHttpGet: got <1 number methodArgs")
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
			er := fmt.Errorf("error: methodREQHttpGet: NewRequest failed: %v, bailing out: %v", err, message.MethodArgs)
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
				er := fmt.Errorf("error: methodREQHttpGet: client.Do failed: %v, bailing out: %v", err, message.MethodArgs)
				proc.errorKernel.errSend(proc, message, er, logWarning)
				newReplyMessage(proc, msgForErrors, []byte(er.Error()))
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				cancel()
				er := fmt.Errorf("error: methodREQHttpGet: not 200, were %#v, bailing out: %v", resp.StatusCode, message)
				proc.errorKernel.errSend(proc, message, er, logWarning)
				newReplyMessage(proc, msgForErrors, []byte(er.Error()))
				return
			}

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				er := fmt.Errorf("error: methodREQHttpGet: io.ReadAll failed : %v, methodArgs: %v", err, message.MethodArgs)
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
			er := fmt.Errorf("error: methodREQHttpGet: method timed out: %v", message.MethodArgs)
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

// ---

// handler to do a Http Get Scheduled.
// The second element of the MethodArgs slice holds the timer defined in seconds.
func methodREQHttpGetScheduled(proc process, message Message, node string) ([]byte, error) {
	er := fmt.Errorf("<--- REQHttpGetScheduled received from: %v, containing: %v", message.FromNode, message.Data)
	proc.errorKernel.logDebug(er)

	proc.processes.wg.Add(1)
	go func() {
		defer proc.processes.wg.Done()

		// --- Check and prepare the methodArgs

		switch {
		case len(message.MethodArgs) < 3:
			er := fmt.Errorf("error: methodREQHttpGet: got <3 number methodArgs. Want URL, Schedule Interval in seconds, and the total time in minutes the scheduler should run for")
			proc.errorKernel.errSend(proc, message, er, logWarning)

			return
		}

		url := message.MethodArgs[0]

		scheduleInterval, err := strconv.Atoi(message.MethodArgs[1])
		if err != nil {
			er := fmt.Errorf("error: methodREQHttpGetScheduled: schedule interval value is not a valid int number defined as a string value seconds: %v, bailing out: %v", err, message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		schedulerTotalTime, err := strconv.Atoi(message.MethodArgs[2])
		if err != nil {
			er := fmt.Errorf("error: methodREQHttpGetScheduled: scheduler total time value is not a valid int number defined as a string value minutes: %v, bailing out: %v", err, message.MethodArgs)
			proc.errorKernel.errSend(proc, message, er, logWarning)
			return
		}

		// --- Prepare and start the scheduler.

		outCh := make(chan []byte)

		ticker := time.NewTicker(time.Second * time.Duration(scheduleInterval))

		// Prepare a context that will be for the schedule as a whole.
		// NB: Individual http get's will create their own context's
		// derived from this one.
		ctxScheduler, cancel := context.WithTimeout(proc.ctx, time.Minute*time.Duration(schedulerTotalTime))

		go func() {
			// Prepare the http request.
			client := http.Client{
				Timeout: time.Second * time.Duration(message.MethodTimeout),
			}

			for {

				select {
				case <-ticker.C:
					proc.processes.wg.Add(1)

					// Get a context with the timeout specified in message.MethodTimeout
					// for the individual http requests.
					ctx, cancel := getContextForMethodTimeout(proc.ctx, message)

					req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
					if err != nil {
						er := fmt.Errorf("error: methodREQHttpGet: NewRequest failed: %v, error: %v", err, message.MethodArgs)
						proc.errorKernel.errSend(proc, message, er, logWarning)
						cancel()
						return
					}

					// Run each individual http get in it's own go routine, and
					// deliver the result on the outCh.
					go func() {
						defer proc.processes.wg.Done()

						resp, err := client.Do(req)
						if err != nil {
							er := fmt.Errorf("error: methodREQHttpGet: client.Do failed: %v, error: %v", err, message.MethodArgs)
							proc.errorKernel.errSend(proc, message, er, logWarning)
							return
						}
						defer resp.Body.Close()

						if resp.StatusCode != 200 {
							cancel()
							er := fmt.Errorf("error: methodREQHttpGet: not 200, were %#v, error: %v", resp.StatusCode, message)
							proc.errorKernel.errSend(proc, message, er, logWarning)
							return
						}

						body, err := io.ReadAll(resp.Body)
						if err != nil {
							er := fmt.Errorf("error: methodREQHttpGet: io.ReadAll failed : %v, methodArgs: %v", err, message.MethodArgs)
							proc.errorKernel.errSend(proc, message, er, logWarning)
						}

						out := body

						select {
						case outCh <- out:
						case <-ctx.Done():
							return
						case <-ctxScheduler.Done():
							// If the scheduler context is done then we also want to kill
							// all running http request.
							cancel()
							return
						}
					}()

				case <-ctxScheduler.Done():
					cancel()
					return

				}
			}
		}()

		for {
			select {
			case <-ctxScheduler.Done():
				// fmt.Printf(" * DEBUG: <-ctxScheduler.Done()\n")
				cancel()
				er := fmt.Errorf("error: methodREQHttpGet: schedule context timed out: %v", message.MethodArgs)
				proc.errorKernel.errSend(proc, message, er, logWarning)
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
