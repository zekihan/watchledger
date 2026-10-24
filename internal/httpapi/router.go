package httpapi

import (
	"encoding/json"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/anidb"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/catalog"
	"net/http"
)

// NewHandler exposes process liveness and optionally serves the frontend build.
// Liveness does not imply database readiness or authenticated application access.
func NewHandler(webDirectory string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}` + "\n"))
	})
	if webDirectory != "" {
		mux.Handle("/", http.FileServer(http.Dir(webDirectory)))
	}
	return mux
}

// NewApplication serves only authenticated product APIs. Liveness stays independent.
func NewApplication(webDir string, pool *pgxpool.Pool, identity *auth.Service, tmdbToken string, animeConfig ...anidb.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			http.Error(w, "database unavailable", http.StatusServiceUnavailable)
			return
		}
		writeJSON(w, map[string]string{"status": "ready"})
	})
	mux.HandleFunc("GET /auth/login", identity.Login)
	mux.HandleFunc("GET /auth/callback", identity.Callback)
	api := http.NewServeMux()
	application := &API{Identity: identity, DB: pool, Catalog: &catalog.Cache{DB: pool, Provider: catalog.New(tmdbToken)}}
	var native anidb.Config
	if len(animeConfig) > 0 {
		native = animeConfig[0]
	}
	application.Anime = &anidb.Cache{DB: pool, Provider: anidb.New(native)}
	application.routes(api)
	api.HandleFunc("POST /api/logout", identity.Logout)
	api.HandleFunc("GET /api/me", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, auth.Current(r.Context())) })
	mux.Handle("/api/", identity.Middleware(api))
	if webDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(webDir)))
	}
	return mux
}
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
