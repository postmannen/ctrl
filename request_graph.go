package ctrl

import (
	"encoding/json"
	"fmt"
	"log"
	"path"

	"github.com/postmannen/graphed"
)

type graph struct {
	db *graphed.PersistentNodeStore
}

// newGraph creates a new graph.
func newGraph(c *Configuration) *graph {
	p := path.Join(c.DatabaseFolder, "graph")
	db, err := graphed.NewPersistentNodeStore(p, graphed.WithChunkSize(5))
	if err != nil {
		log.Fatalf("Failed to create database on path %s: %v", p, err)
	}

	g := graph{
		db: db,
	}

	return &g
}

// methodGraphAddNode is used to add a node to the graph.
func methodGraphAddNode(proc process, message Message, node string) ([]byte, error) {
	var nodeName string
	var parentName string

	switch len(message.MethodArgs) {
	case 1:
		nodeName = message.MethodArgs[0]
	case 2:
		nodeName = message.MethodArgs[0]
		parentName = message.MethodArgs[1]
	default:
		return nil, fmt.Errorf("invalid number of arguments when adding node to graph: %v", message.MethodArgs)
	}

	err := proc.graph.db.AddNode(nodeName, parentName)
	if err != nil {
		return nil, err
	}

	proc.graph.db.AddToValues(message.MethodArgs[0], message.Data)

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// methodGraphGetNode is used to get a node from the graph.
// It returns a JSON object of the node struct graphed.Node.
func methodGraphGetNode(proc process, message Message, node string) ([]byte, error) {
	fmt.Println("------------------------**********------------------------")

	var nodeName string
	if len(message.MethodArgs) < 1 {
		return nil, fmt.Errorf("invalid number of arguments when getting node from graph: %v", message.MethodArgs)
	}

	nodeName = message.MethodArgs[0]

	n, err := proc.graph.db.GetNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	out, err := json.Marshal(n)
	if err != nil {
		return nil, err
	}

	newReplyMessage(proc, message, out)

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

// methodGraphGetNodeChildren is used to get the children of a node.
// It returns a JSON array of the children's names.
func methodGraphGetNodeChildren(proc process, message Message, node string) ([]byte, error) {
	var nodeName string
	if len(message.MethodArgs) < 1 {
		return nil, fmt.Errorf("invalid number of arguments when getting node children from graph: %v", message.MethodArgs)
	}

	nodeName = message.MethodArgs[0]

	n, err := proc.graph.db.GetNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	var childrenNames = struct {
		Names []string
	}{}

	for childUUID := range n.Children {
		n, err := proc.graph.db.GetNodeByID(childUUID)
		if err != nil {
			return nil, err
		}

		childrenNames.Names = append(childrenNames.Names, n.Name)

		fmt.Printf("DEBUG: printing child from node %v, child: %v\n", nodeName, n.Name)
	}

	out, err := json.Marshal(childrenNames)
	if err != nil {
		return nil, err
	}

	newReplyMessage(proc, message, out)

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}

func methodGraphGetNodeParents(proc process, message Message, node string) ([]byte, error) {

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
