package httpapi

import (
	"encoding/json"
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
	kind, id := r.PathValue("kind"), r.PathValue("id")
	key, e := catalog.Identity(kind, id)
	if e != nil || kind == "person" {
		http.Error(w, "invalid title", http.StatusBadRequest)
		return
	}
	if _, e = a.Catalog.Detail(r.Context(), kind, id, "", false); e != nil {
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	country := r.URL.Query().Get("country")
	if len(country) != 2 {
		http.Error(w, "country is required", http.StatusBadRequest)
		return
	}
	var cached json.RawMessage
	var fetched interface{}
	_ = a.DB.QueryRow(r.Context(), "SELECT offers,fetched_at FROM availability WHERE catalog_id=$1 AND country=$2", key, country).Scan(&cached, &fetched)
	raw, e := a.Catalog.Provider.Get(r.Context(), "/"+kind+"/"+id+"/watch/providers", nil)
	if e != nil {
		if len(cached) > 0 {
			writeJSON(w, map[string]interface{}{"offers": cached, "fetched_at": fetched, "stale": true, "warning": e.Error()})
			return
		}
		http.Error(w, e.Error(), http.StatusBadGateway)
		return
	}
	var result struct {
		Results map[string]json.RawMessage `json:"results"`
	}
	if e = json.Unmarshal(raw, &result); e != nil {
		http.Error(w, "invalid provider response", http.StatusBadGateway)
		return
	}
	offers := result.Results[country]
	if len(offers) == 0 {
		offers = json.RawMessage(`null`)
	}
	if e = a.DB.QueryRow(r.Context(), `INSERT INTO availability(catalog_id,country,offers) VALUES($1,$2,$3) ON CONFLICT(catalog_id,country) DO UPDATE SET offers=EXCLUDED.offers,fetched_at=now(),used_at=now() RETURNING fetched_at`, key, country, offers).Scan(&fetched); e != nil {
		http.Error(w, "availability cache unavailable", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, map[string]interface{}{"offers": offers, "fetched_at": fetched, "stale": false, "attribution": "JustWatch via TMDB"})
}
