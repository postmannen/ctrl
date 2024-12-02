// The structure of how to add new method types to the system.
// -----------------------------------------------------------
// All methods need 3 things:
//  - A type definition
//  - The type needs a getKind method
//  - The type needs a handler method
// Overall structure example shown below.
//
// ---
// type methodCommandCLICommandRequest struct {
// 	commandOrEvent CommandOrEvent
// }
//
// func (m methodCommandCLICommandRequest) getKind() CommandOrEvent {
// 	return m.commandOrEvent
// }
//
// func (m methodCommandCLICommandRequest) handler(s *server, message Message, node string) ([]byte, error) {
//  ...
//  ...
// 	ackMsg := []byte(fmt.Sprintf("confirmed from node: %v: messageID: %v\n---\n%s---", node, message.ID, out))
// 	return ackMsg, nil
// }
//
// ---
// You also need to make a constant for the Method, and add
// that constant as the key in the MethodsAvailable map, where
// the value is the actual type you want to map it to with a
// handler method. You also specify if it is a Command or Event,
// and if it is ACK or NACK.
//
// Requests used in sub processes should always start with the
// naming REQSUB. Since the method of a sub process are defined
// within the method handler of the owning reqest type we should
// use the methodSUB for these types. The methodSUB handler
// does nothing.
//
// Check out the existing code below for more examples.

package ctrl

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

// Method is used to specify the actual function/method that
// is represented in a typed manner.
type Method string

// ------------------------------------------------------------
// The constants that will be used throughout the system for
// when specifying what kind of Method to send or work with.
const (
	// Initial parent method used to start other processes.
	Initial Method = "initial"
	// Get a list of all the running processes.
	OpProcessList Method = "opProcessList"
	// Start up a process.
	OpProcessStart Method = "opProcessStart"
	// Stop up a process.
	OpProcessStop Method = "opProcessStop"
	// Execute a CLI command in for example bash or cmd.
	// This is an event type, where a message will be sent to a
	// node with the command to execute and an ACK will be replied
	// if it was delivered succesfully. The output of the command
	// ran will be delivered back to the node where it was initiated
	// as a new message.
	// The data field is a slice of strings where the first string
	// value should be the command, and the following the arguments.
	CliCommand Method = "cliCommand"
	// REQCliCommandCont same as normal Cli command, but can be used
	// when running a command that will take longer time and you want
	// to send the output of the command continually back as it is
	// generated, and not wait until the command is finished.
	CliCommandCont Method = "cliCommandCont"
	// Send text to be logged to the console.
	// The data field is a slice of strings where the first string
	// value should be the command, and the following the arguments.
	Console Method = "console"
	// Send text logging to some host by appending the output to a
	// file, if the file do not exist we create it.
	// A file with the full subject+hostName will be created on
	// the receiving end.
	// The data field is a slice of strings where the values of the
	// slice will be written to the log file.
	FileAppend Method = "fileAppend"
	// Send text to some host by overwriting the existing content of
	// the fileoutput to a file. If the file do not exist we create it.
	// A file with the full subject+hostName will be created on
	// the receiving end.
	// The data field is a slice of strings where the values of the
	// slice will be written to the file.
	File Method = "file"
	// Initiated by the user.
	CopySrc Method = "copySrc"
	// Initial request for file copying.
	// Generated by the source to send initial information to the destination.
	CopyDst Method = "copyDst"
	// Read the source file to be copied to some node.
	SUBCopySrc Method = "subCopySrc"
	// Write the destination copied to some node.
	SUBCopyDst Method = "subCopyDst"
	// Hello I'm here message.
	Hello          Method = "hello"
	HelloPublisher Method = "helloPublisher"
	// Error log methods to centralError node.
	ErrorLog Method = "errorLog"
	// Http Get
	HttpGet Method = "httpGet"
	// Http Get Scheduled
	// The second element of the MethodArgs slice holds the timer defined in seconds.
	HttpGetScheduled Method = "httpGetScheduled"
	// Tail file
	TailFile Method = "tailFile"
	// REQNone is used when there should be no reply.
	None Method = "none"
	// REQTest is used only for testing to be able to grab the output
	// of messages.
	Test Method = "test"

	// REQPublicKey will get the public ed25519 key from a node.
	PublicKey Method = "publicKey"
	// REQKeysRequestUpdate will get all the public keys from central if an update is available.
	KeysRequestUpdate Method = "keysRequestUpdate"
	// REQKeysDeliverUpdate will deliver the public from central to a node.
	KeysDeliverUpdate Method = "keysDeliverUpdate"
	// REQKeysAllow
	KeysAllow Method = "keysAllow"
	// REQKeysDelete
	KeysDelete Method = "keysDelete"

	// REQAclRequestUpdate will get all node acl's from central if an update is available.
	AclRequestUpdate Method = "aclRequestUpdate"
	// REQAclDeliverUpdate will deliver the acl from central to a node.
	AclDeliverUpdate Method = "aclDeliverUpdate"

	// REQAclAddCommand
	AclAddCommand = "aclAddCommand"
	// REQAclDeleteCommand
	AclDeleteCommand = "aclDeleteCommand"
	// REQAclDeleteSource
	AclDeleteSource = "aclDeleteSource"
	// REQGroupNodesAddNode
	AclGroupNodesAddNode = "aclGroupNodesAddNode"
	// REQAclGroupNodesDeleteNode
	AclGroupNodesDeleteNode = "aclGroupNodesDeleteNode"
	// REQAclGroupNodesDeleteGroup
	AclGroupNodesDeleteGroup = "aclGroupNodesDeleteGroup"
	// REQAclGroupCommandsAddCommand
	AclGroupCommandsAddCommand = "aclGroupCommandsAddCommand"
	// REQAclGroupCommandsDeleteCommand
	AclGroupCommandsDeleteCommand = "aclGroupCommandsDeleteCommand"
	// REQAclGroupCommandsDeleteGroup
	AclGroupCommandsDeleteGroup = "aclGroupCommandsDeleteGroup"
	// REQAclExport
	AclExport = "aclExport"
	// REQAclImport
	AclImport = "aclImport"
)

type HandlerFunc func(proc process, message Message, node string) ([]byte, error)

// The mapping of all the method constants specified, what type
// it references.
// The primary use of this table is that messages are not able to
// pass the actual type of the request since it is sent as a string,
// so we use the below table to find the actual type based on that
// string type.
func (m Method) GetMethodsAvailable() MethodsAvailable {

	ma := MethodsAvailable{
		Methodhandlers: map[Method]HandlerFunc{
			Initial:           HandlerFunc(methodInitial),
			OpProcessList:     HandlerFunc(methodOpProcessList),
			OpProcessStart:    HandlerFunc(methodOpProcessStart),
			OpProcessStop:     HandlerFunc(methodOpProcessStop),
			CliCommand:        HandlerFunc(methodCliCommand),
			CliCommandCont:    HandlerFunc(methodCliCommandCont),
			Console:           HandlerFunc(methodConsole),
			FileAppend:        HandlerFunc(methodFileAppend),
			File:              HandlerFunc(methodToFile),
			CopySrc:           HandlerFunc(methodCopySrc),
			CopyDst:           HandlerFunc(methodCopyDst),
			SUBCopySrc:        HandlerFunc(methodSUB),
			SUBCopyDst:        HandlerFunc(methodSUB),
			Hello:             HandlerFunc(methodHello),
			HelloPublisher:    HandlerFunc(nil),
			ErrorLog:          HandlerFunc(methodErrorLog),
			HttpGet:           HandlerFunc(methodHttpGet),
			HttpGetScheduled:  HandlerFunc(methodHttpGetScheduled),
			TailFile:          HandlerFunc(methodTailFile),
			PublicKey:         HandlerFunc(methodPublicKey),
			KeysRequestUpdate: HandlerFunc(methodKeysRequestUpdate),
			KeysDeliverUpdate: HandlerFunc(methodKeysDeliverUpdate),
			KeysAllow:         HandlerFunc(methodKeysAllow),
			KeysDelete:        HandlerFunc(methodKeysDelete),

			AclRequestUpdate: HandlerFunc(methodAclRequestUpdate),
			AclDeliverUpdate: HandlerFunc(methodAclDeliverUpdate),

			AclAddCommand:                 HandlerFunc(methodAclAddCommand),
			AclDeleteCommand:              HandlerFunc(methodAclDeleteCommand),
			AclDeleteSource:               HandlerFunc(methodAclDeleteSource),
			AclGroupNodesAddNode:          HandlerFunc(methodAclGroupNodesAddNode),
			AclGroupNodesDeleteNode:       HandlerFunc(methodAclGroupNodesDeleteNode),
			AclGroupNodesDeleteGroup:      HandlerFunc(methodAclGroupNodesDeleteGroup),
			AclGroupCommandsAddCommand:    HandlerFunc(methodAclGroupCommandsAddCommand),
			AclGroupCommandsDeleteCommand: HandlerFunc(methodAclGroupCommandsDeleteCommand),
			AclGroupCommandsDeleteGroup:   HandlerFunc(methodAclGroupCommandsDeleteGroup),
			AclExport:                     HandlerFunc(methodAclExport),
			AclImport:                     HandlerFunc(methodAclImport),
			Test:                          HandlerFunc(methodTest),
		},
	}

	return ma
}

// getHandler will check the methodsAvailable map, and return the
// method handler for the method given
// as input argument.
func (m Method) getHandler(method Method) HandlerFunc {
	ma := m.GetMethodsAvailable()
	mh, _ := ma.CheckIfExists(method)
	// mh := ma.Methodhandlers[method]

	return mh
}

// getContextForMethodTimeout, will return a context with cancel function
// with the timeout set to the method timeout in the message.
// If the value of timeout is set to -1, we don't want it to stop, so we
// return a context with a timeout set to 200 years.
func getContextForMethodTimeout(ctx context.Context, message Message) (context.Context, context.CancelFunc) {
	// If methodTimeout == -1, which means we don't want a timeout, set the
	// time out to 200 years.
	if message.MethodTimeout == -1 {
		return context.WithTimeout(ctx, time.Hour*time.Duration(8760*200))
	}

	return context.WithTimeout(ctx, time.Second*time.Duration(message.MethodTimeout))
}

// ----

// Initial parent method used to start other processes.
func methodInitial(proc process, message Message, node string) ([]byte, error) {
	// proc.procFuncCh <- message
	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ----

// place holder method used for sub processes.
// Methods used in sub processes are defined within the the requests
// they are spawned in, so this type is primarily for us to use the
// same logic with sub process requests as we do with normal requests.
func methodSUB(proc process, message Message, node string) ([]byte, error) {
	// proc.procFuncCh <- message
	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// ----

// MethodsAvailable holds a map of all the different method types and the
// associated handler to that method type.
type MethodsAvailable struct {
	Methodhandlers map[Method]HandlerFunc
}

// Check if exists will check if the Method is defined. If true the bool
// value will be set to true, and the methodHandler function for that type
// will be returned.
func (ma MethodsAvailable) CheckIfExists(m Method) (HandlerFunc, bool) {
	// First check if it is a sub process.
	if strings.HasPrefix(string(m), "sub") {
		// Strip of the uuid after the method name.
		sp := strings.Split(string(m), ".")
		m = Method(sp[0])
	}

	mFunc, ok := ma.Methodhandlers[m]
	if ok {
		return mFunc, true
	} else {
		return nil, false
	}
}

// newReplyMessage will create and send a reply message back to where
// the original provided message came from. The primary use of this
// function is to report back to a node who sent a message with the
// result of the request method of the original message.
//
// The method to use for the reply message when reporting back should
// be specified within a message in the  replyMethod field. We will
// pick up that value here, and use it as the method for the new
// request message. If no replyMethod is set we default to the
// REQToFileAppend method type.
//
// There will also be a copy of the original message put in the
// previousMessage field. For the copy of the original message the data
// field will be set to nil before the whole message is put in the
// previousMessage field so we don't copy around the original data in
// the reply response when it is not needed anymore.
func newReplyMessage(proc process, message Message, outData []byte) {
	// If REQNone is specified, we don't want to send a reply message
	// so we silently just return without sending anything.
	if message.ReplyMethod == None || message.IsReply {
		return
	}

	// If no replyMethod is set we default to writing to writing to
	// a log file.
	if message.ReplyMethod == "" {
		message.ReplyMethod = FileAppend
	}

	// Make a copy of the message as it is right now to use
	// in the previous message field, but set the data field
	// to nil so we don't copy around the original data when
	// we don't need to for the reply message.
	thisMsg := message
	thisMsg.Data = nil

	// Create a new message for the reply, and put it on the
	// ringbuffer to be published.
	newMsg := Message{
		ToNode: message.FromNode,
		// The ToNodes field is not needed since it is only a concept that exists when messages
		// are injected f.ex. on a socket, and there they are directly converted into separate
		// node messages. With other words a message in the system are only for single nodes,
		// so we don't have to worry about the ToNodes field when creating reply messages.
		FromNode:      message.ToNode,
		Data:          outData,
		Method:        message.ReplyMethod,
		MethodArgs:    message.ReplyMethodArgs,
		MethodTimeout: message.ReplyMethodTimeout,
		IsReply:       true,
		RetryWait:     message.RetryWait,
		ACKTimeout:    message.ReplyACKTimeout,
		Retries:       message.ReplyRetries,
		Directory:     message.Directory,
		FileName:      message.FileName,

		// Put in a copy of the initial request message, so we can use it's properties if
		// needed to for example create the file structure naming on the subscriber.
		PreviousMessage: &thisMsg,
	}

	proc.newMessagesCh <- newMsg
}

// selectFileNaming will figure out the correct naming of the file
// structure to use for the reply data.
// It will return the filename, and the tree structure for the folders
// to create.
func selectFileNaming(message Message, proc process) (string, string) {
	var fileName string
	// As default we set the folder tree to what is specified in the
	// message.Directory field. If we don't want that in the checks
	// done later we then replace the value with what we want.
	folderTree := message.Directory

	checkPrefix := func(s string) bool {
		if strings.HasPrefix(s, "./") || strings.HasPrefix(s, "/") {
			return true
		}

		return false
	}

	switch {
	case message.PreviousMessage == nil:
		// If this was a direct request there are no previous message to take
		// information from, so we use the one that are in the current mesage.
		fileName = message.FileName
		if !checkPrefix(message.Directory) {
			folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.Directory, string(message.ToNode))
		}
	case message.PreviousMessage.ToNode != "":
		fileName = message.PreviousMessage.FileName
		if !checkPrefix(message.PreviousMessage.Directory) {
			folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.PreviousMessage.Directory, string(message.PreviousMessage.ToNode))
		}
	case message.PreviousMessage.ToNode == "":
		fileName = message.PreviousMessage.FileName
		if !checkPrefix(message.PreviousMessage.Directory) {
			folderTree = filepath.Join(proc.configuration.SubscribersDataFolder, message.PreviousMessage.Directory, string(message.FromNode))
		}
	}

	return fileName, folderTree
}
