package ctrl

// ---- Template that can be used for creating request methods

// func (m methodREQCopyFileTo) handler(proc process, message Message, node string) ([]byte, error) {
//
// 	proc.processes.wg.Add(1)
// 	go func() {
// 		defer proc.processes.wg.Done()
//
// 		ctx, cancel := context.WithTimeout(proc.ctx, time.Second*time.Duration(message.MethodTimeout))
// 		defer cancel()
//
// 		// Put data that should be the result of the action done in the inner
// 		// go routine on the outCh.
// 		outCh := make(chan []byte)
// 		// Put errors from the inner go routine on the errCh.
// 		errCh := make(chan error)
//
// 		proc.processes.wg.Add(1)
// 		go func() {
// 			defer proc.processes.wg.Done()
//
// 			// Do some work here....
//
// 		}()
//
// 		// Wait for messages received from the inner go routine.
// 		select {
// 		case <-ctx.Done():
// 			fmt.Printf(" ** DEBUG: got ctx.Done\n")
//
// 			er := fmt.Errorf("error: methodREQ...: got <-ctx.Done(): %v", message.MethodArgs)
// 			sendErrorLogMessage(proc.configuration, proc.processes.metrics, proc.toRingbufferCh, proc.node, er)
// 			return
//
// 		case er := <-errCh:
// 			sendErrorLogMessage(proc.configuration, proc.processes.metrics, proc.toRingbufferCh, proc.node, er)
// 			return
//
// 		case out := <-outCh:
// 			replyData := fmt.Sprintf("info: succesfully created and wrote the file %v\n", out)
// 			newReplyMessage(proc, message, []byte(replyData))
// 			return
// 		}
//
// 	}()
//
// 	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
// 	return ackMsg, nil
// }
