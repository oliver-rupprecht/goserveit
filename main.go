package main

import (
	"database/sql"
	"uuid"
	"time"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	mydb "github.com/oliver-rupprecht/goserveit/internal/database"
)

func main() {
	multiplexer := http.NewServeMux()
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	pf := os.Getenv("PLATFORM")

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		os.Exit(1)
	}

	server := http.Server{
		Handler: multiplexer, Addr:":8080",
	}

	dbq := mydb.New(db)

	apiCfg := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries: dbq,
		platform: pf,
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
			CleanedBody string `json:"cleaned_body"`
		}{censor(reqParam.Body)}

		data, _ := json.Marshal(body)
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")

		rw.Write(data)
	})

	multiplexer.HandleFunc("POST /api/users", func(rw http.ResponseWriter, req *http.Request) {
		reqbody := struct {
			Email string `json:"email"`
		}{}

		if err := json.NewDecoder(req.Body).Decode(&reqbody); err != nil {
			body := struct {
				Error string `json:"error"`
			}{"Couldnt decode request body"}

			data, _ := json.Marshal(body)
			rw.Header().Set("Content-Type", "text/json; charset=utf-8")
			rw.WriteHeader(400)

			rw.Write(data)
			return
		}

		data, err := apiCfg.dbQueries.CreateUser(req.Context(), reqbody.Email)
		if err != nil {
			body := struct {
				Error string `json:"error"`
			}{fmt.Sprintf("Database transaction failed %s", err)}

			data, _ := json.Marshal(body)
			rw.Header().Set("Content-Type", "text/json; charset=utf-8")
			rw.WriteHeader(400)

			rw.Write(data)
			return
		}

		createdUser := User{
			ID: uuid.UUID(data.ID),
			CreatedAt: data.CreatedAt,
			UpdatedAt: data.UpdatedAt,
			Email: data.Email,
		}


		marshaled, err := json.Marshal(createdUser)
		if err != nil {
			body := struct {
				Error string `json:"error"`
			}{"Failed to serialise user object"}

			data, _ := json.Marshal(body)
			rw.Header().Set("Content-Type", "text/json; charset=utf-8")
			rw.WriteHeader(400)

			rw.Write(data)
			return
		}

		rw.Header().Set("Content-Type", "text/json; charset=utf-8")
		rw.WriteHeader(http.StatusCreated)

		rw.Write(marshaled)
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

		if apiCfg.platform != "dev" {
			rw.WriteHeader(http.StatusForbidden)
			rw.Write([]byte("Forbidden"))
			return
		}

		apiCfg.resetRequests()
		apiCfg.dbQueries.DeleteAllFromUsers(req.Context())

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
	dbQueries *mydb.Queries 
	platform string
}


func (cfg *apiConfig) numRequests() int32 {
		return cfg.fileserverHits.Load()
}

func (cfg *apiConfig) resetRequests() {
		cfg.fileserverHits.Store(0)
}

func censor(s string) string {
	o := make([]string, 0)

	for _, w := range strings.Split(s, " ") {
		lower := strings.ToLower(w)
		if string(lower[len(w)-1]) == "!" {
			o = append(o, w)
			continue
		} 

		switch lower {
		case "kerfuffle": o = append(o, "****") 
		case "sharbert": o = append(o, "****") 
		case "fornax": o = append(o, "****") 
		default:  
			o = append(o, w)
		}
	}

	return strings.Join(o, " ")
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}
