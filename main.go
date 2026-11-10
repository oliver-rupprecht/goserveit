package main

import (
	"net/http"
	"sync/atomic"
	"fmt"
)

func main() {
	multiplexer := http.NewServeMux()

	server := http.Server{
		Handler: multiplexer, Addr:":8080",
	}

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	handler := &metricsHandler{
		cfg: &apiCfg, 
		handler: http.FileServer(http.Dir(".")),
	}

	multiplexer.Handle("/app/", http.StripPrefix("/app", handler))

	multiplexer.HandleFunc("GET /api/metrics", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusOK)

	    fmt.Fprintf(rw, "Hits: %d", apiCfg.numRequests())
	})

	multiplexer.HandleFunc("POST /api/reset", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusOK)

		apiCfg.resetRequests()
		fmt.Println("reset")

		rw.Write([]byte("OK"))
	})

	multiplexer.HandleFunc("GET /api/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")

		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("OK"))
	})

	server.ListenAndServe()
}

type metricsHandler struct {
	cfg *apiConfig
	handler http.Handler
}

func (h *metricsHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	h.cfg.fileserverHits.Add(1)
	h.handler.ServeHTTP(rw, req)
}

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	handlerFunc := http.HandlerFunc(func(rw http.ResponseWriter, res *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(rw, res)
	})

	return handlerFunc
}

func (cfg *apiConfig) numRequests() int32 {
		return cfg.fileserverHits.Load()
}

func (cfg *apiConfig) resetRequests() {
		cfg.fileserverHits.Store(0)
}


