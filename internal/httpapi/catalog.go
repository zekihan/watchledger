package httpapi

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/catalog"
	"net/http"
	"net/url"
	"strconv"
)

type API struct {
	DB      *pgxpool.Pool
	Catalog *catalog.Cache
}

func (a *API) routes(mux *http.ServeMux) {
	a.libraryRoutes(mux)
	a.listRoutes(mux)
	a.portableRoutes(mux)
	a.mappingRoutes(mux)
	mux.HandleFunc("GET /api/dashboard", a.dashboard)
	mux.HandleFunc("PUT /api/up-next/order", a.upNextOrder)
	mux.HandleFunc("GET /api/statistics", a.statistics)
	mux.HandleFunc("GET /api/calendar", a.calendar)
	mux.HandleFunc("GET /api/search", a.search)
	mux.HandleFunc("GET /api/discover/{kind}", a.discover)
	mux.HandleFunc("GET /api/catalog/{kind}/{id}", a.detail)
	mux.HandleFunc("POST /api/catalog/{kind}/{id}/refresh", a.refresh)
	mux.HandleFunc("GET /api/catalog/tv/{id}/seasons/{season}", a.season)
	mux.HandleFunc("GET /api/catalog/{kind}/{id}/providers", a.providers)
	mux.HandleFunc("GET /api/providers/{kind}", a.providerList)
}
func (a *API) search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if kind := q.Get("kind"); kind != "" && kind != "multi" && !catalog.ValidKind(kind) {
		http.Error(w, "catalog source is not available for this kind", http.StatusBadRequest)
		return
	}
	if q.Get("query") == "" {
		http.Error(w, "search query is required", http.StatusBadRequest)
		return
	}
	params := url.Values{"query": {q.Get("query")}, "language": {q.Get("language")}, "page": {q.Get("page")}}
	if params.Get("page") == "" {
		params.Set("page", "1")
	}
	raw, e := a.Catalog.Provider.Get(r.Context(), catalog.SearchPath(q.Get("kind")), params)
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, raw)
}
func (a *API) discover(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind == "tv" && r.URL.Query().Get("person_id") != "" {
		raw, e := a.Catalog.PeopleTV(r.Context(), r.URL.Query().Get("person_id"), r.URL.Query())
		if e != nil {
			http.Error(w, e.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, raw)
		return
	}
	params, e := catalog.Discovery(kind, r.URL.Query())
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	raw, e := a.Catalog.Provider.Get(r.Context(), "/discover/"+kind, params)
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, raw)
}
func (a *API) detail(w http.ResponseWriter, r *http.Request)  { a.getDetail(w, r, false) }
func (a *API) refresh(w http.ResponseWriter, r *http.Request) { a.getDetail(w, r, true) }
func (a *API) getDetail(w http.ResponseWriter, r *http.Request, force bool) {
	if _, e := catalog.Identity(r.PathValue("kind"), r.PathValue("id")); e != nil {
		http.Error(w, e.Error(), http.StatusBadRequest)
		return
	}
	d, e := a.Catalog.Detail(r.Context(), r.PathValue("kind"), r.PathValue("id"), r.URL.Query().Get("language"), force)
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, d)
}
func (a *API) season(w http.ResponseWriter, r *http.Request) {
	n, e := strconv.Atoi(r.PathValue("season"))
	if e != nil || n < 0 {
		http.Error(w, "invalid season", http.StatusBadRequest)
		return
	}
	raw, e := a.Catalog.Season(r.Context(), r.PathValue("id"), n, r.URL.Query().Get("language"))
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, raw)
}
func (a *API) providerList(w http.ResponseWriter, r *http.Request) {
	kind := r.PathValue("kind")
	if kind != "movie" && kind != "tv" {
		http.Error(w, "invalid kind", http.StatusBadRequest)
		return
	}
	raw, e := a.Catalog.Provider.Get(r.Context(), "/watch/providers/"+kind, url.Values{"watch_region": {r.URL.Query().Get("country")}, "language": {r.URL.Query().Get("language")}})
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, raw)
}
func (a *API) providers(w http.ResponseWriter, r *http.Request) {
	out, e := a.Catalog.Availability(r.Context(), r.PathValue("kind"), r.PathValue("id"), r.URL.Query().Get("country"), false)
	if e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, out)
}
