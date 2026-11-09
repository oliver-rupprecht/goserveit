package main

import (
	"net/http"
)

func main() {
	multiplexer := http.NewServeMux()
	server := http.Server{Handler:multiplexer, Addr:":8080"}

	server.ListenAndServe()
}
