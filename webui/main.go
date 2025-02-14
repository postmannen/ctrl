package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	// Define command line flag for port
	port := flag.String("port", "localhost:8080", "Port to serve on (default 8080)")
	flag.Parse()

	// Create file server handler
	fs := http.FileServer(http.Dir("."))

	// Register handler for root path
	http.Handle("/", fs)

	// Start server
	log.Printf("Starting server on %v", port)
	if err := http.ListenAndServe(*port, nil); err != nil {
		log.Fatal(err)
	}
}
