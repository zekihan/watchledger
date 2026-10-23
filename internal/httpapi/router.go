package httpapi

import "net/http"

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
