package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
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

	multiplexer.HandleFunc("/api/validate_chirp", func(rw http.ResponseWriter, req *http.Request) {
		reqParam := struct{ 
			Body string `json:"body"`
		}{}

		d := json.NewDecoder(req.Body)
		if err := d.Decode(&reqParam); err != nil {
			body := struct {
				Error string `json:"error"`
			}{"Couldnt decode request body"}

			data, _ := json.Marshal(body)
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.WriteHeader(400)

			rw.Write(data)
			return
		}

		if len(reqParam.Body) > 140 {
			body := struct {
				Error string `json:"error"`
			}{"Chirp is too long"}

			data, _ := json.Marshal(body)
			rw.Header().Set("Content-Type", "text/html; charset=utf-8")
			rw.WriteHeader(400)

			rw.Write(data)
			return
		}

		body := struct {
			Valid bool `json:"valid"`
		}{true}

		data, _ := json.Marshal(body)
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")

		rw.Write(data)
	})

	multiplexer.HandleFunc("GET /admin/metrics", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.WriteHeader(http.StatusOK)

		html := "<html><body><h1>Welcome, Chirpy Admin</h1><p>Chirpy has been visited %d times!</p></body></html>"

	    fmt.Fprintf(rw, html, apiCfg.numRequests())
	})

	multiplexer.HandleFunc("GET /admin/healthz", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusOK)

		rw.Write([]byte("OK"))
	})

	multiplexer.HandleFunc("POST /admin/reset", func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
		rw.WriteHeader(http.StatusOK)

		apiCfg.resetRequests()

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

func (cfg *apiConfig) numRequests() int32 {
		return cfg.fileserverHits.Load()
}

func (cfg *apiConfig) resetRequests() {
		cfg.fileserverHits.Store(0)
}


