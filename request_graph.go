package ctrl

import (
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
func methodGraphGetNode(proc process, message Message, node string) ([]byte, error) {
	fmt.Println("------------------------**********------------------------")

	var nodeName string
	if len(message.MethodArgs) < 1 {
		return nil, fmt.Errorf("invalid number of arguments when getting node from graph: %v", message.MethodArgs)
	}

	nodeName = message.MethodArgs[0]

	n, err := proc.graph.db.Node(nodeName)
	if err != nil {
		return nil, err
	}

	for _, v := range n.Values {
		fmt.Printf("DEBUG: printing value from node %v, value: %v\n", message.FromNode, string(v))
	}

	ackMsg := []byte("confirmed from: " + node + ": " + fmt.Sprint(message.ID))
	return ackMsg, nil
}
