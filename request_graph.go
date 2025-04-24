package ctrl

import (
	"log"
	"path"

	"github.com/postmannen/graphed"
)

type graph struct {
	db *graphed.PersistentNodeStore
}

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
	return nil, nil
}

// methodGraphGetNode is used to get a node from the graph.
func methodGraphGetNode(proc process, message Message, node string) ([]byte, error) {
	return nil, nil
}
