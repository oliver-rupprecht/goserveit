package main

import (
	"net/http"
)

func main() {
	multiplexer := http.NewServeMux()
	server := http.Server{Handler:multiplexer, Addr:":8080"}

	multiplexer.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))

	multiplexer.HandleFunc("/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")

		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("OK"))
	})

	server.ListenAndServe()
}

