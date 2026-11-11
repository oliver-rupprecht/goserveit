package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"
	"github.com/google/uuid"

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

	multiplexer.HandleFunc("POST /api/chirps", func(rw http.ResponseWriter, r *http.Request) {
		chirp := Chirp{}
		if err := json.NewDecoder(r.Body).Decode(&chirp); err != nil {
        	marshalAndSendError(rw, http.StatusBadRequest, "couldn't decode request body")
			return
		}

		if len(chirp.Body) > 140 {
            marshalAndSendError(rw, http.StatusBadRequest, "chirp length exceeded")
            return
		}

        censored := censorText(chirp.Body)
		transObj := mydb.SaveChirpParams {
			Body: censored,
			UserID: uuid.NullUUID {
                UUID: chirp.UserId, 
                Valid: true, 
			},
		}

		savedChrip, err := apiCfg.dbQueries.SaveChirp(r.Context(), transObj)
		if err != nil {
			marshalAndSendError(rw, http.StatusInternalServerError, fmt.Sprintf("%s", err))
			return
		}

		data, err := json.Marshal(
			struct {
				Id uuid.UUID `json:"id"`
				CreatedAt time.Time `json:"created_at"`
				UpdatedAt time.Time `json:"updated_at"`
				Body string `json:"body"`
				UserId uuid.UUID `json:"user_id"`
			}{savedChrip.ID, savedChrip.CreatedAt, savedChrip.UpdatedAt, savedChrip.Body, savedChrip.UserID.UUID},
		)
		if err != nil {
			marshalAndSendError(rw, http.StatusInternalServerError, fmt.Sprintf("%s", err))
			return
		}
        
		rw.Header().Set("Content-Type", "text/json; charset=utf-8")
		rw.WriteHeader(http.StatusCreated)

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

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type ErrorJson struct {
	Body string `json:"error"`
}

type Chirp struct {
	Body string `json:"body"`
	UserId uuid.UUID `json:"user_id"`
}

func marshalAndSendError(rw http.ResponseWriter, status int, errorS string) {
    bytes, _ := json.Marshal(
        ErrorJson{Body:errorS},
    )

    rw.Header().Set(
        "Content-Type", "text/html; charset=utf-8",
    )

    rw.WriteHeader(status)
	rw.Write(bytes)
}

func censorText(s string) string {
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


