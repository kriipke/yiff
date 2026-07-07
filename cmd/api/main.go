package main

import (
	"log"
	"net/http"

	httpx "github.com/kriipke/driftmap/internal/adapters/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/diff", httpx.DiffHandler)

	addr := ":8080"
	log.Printf("Starting web server on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
