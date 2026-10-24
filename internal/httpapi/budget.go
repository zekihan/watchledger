package httpapi

import (
	"net/http"
	"strings"
	"time"
)

// Large imports have a separate bounded budget. Cancellation reaches database
// transactions and provider calls, so timeout does not authorize partial writes.
func RequestBudget(next http.Handler) http.Handler {
	return requestBudget(next, 2*time.Minute, 5*time.Minute)
}
func requestBudget(next http.Handler, ordinary, large time.Duration) http.Handler {
	normal := http.TimeoutHandler(next, ordinary, "request timed out; retry later")
	bulk := http.TimeoutHandler(next, large, "import timed out; check its status before retrying")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/restore/") || strings.HasPrefix(r.URL.Path, "/api/simkl/") || strings.HasPrefix(r.URL.Path, "/api/admin/") || r.URL.Path == "/api/export" {
			bulk.ServeHTTP(w, r)
			return
		}
		normal.ServeHTTP(w, r)
	})
}
