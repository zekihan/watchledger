package httpapi

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/zekihan/watchledger/internal/auth"
	"github.com/zekihan/watchledger/internal/simkl"
	"net/http"
	"strconv"
	"time"
)

var defaultSIMKLReadClient = simkl.NewReadOnlyClient()
var simklImportGate = make(chan struct{}, 1)

func (a *API) fetchCustomList(w http.ResponseWriter, r *http.Request) {
	var in struct {
		ClientID string `json:"client_id"`
		Token    string `json:"access_token"`
		ListID   string `json:"list_id"`
	}
	if !decode(w, r, &in) {
		return
	}
	id, err := strconv.ParseInt(in.ListID, 10, 64)
	if err != nil || id < 1 || strconv.FormatInt(id, 10) != in.ListID {
		http.Error(w, "valid SIMKL list ID required", http.StatusBadRequest)
		return
	}
	// This handler never accepts an upstream URL. Credentials remain request-local.
	ctx, cancel := context.WithTimeout(r.Context(), 90*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	// One migration fetch at a time in the supported single-replica app.
	// Queue without holding a DB connection, keeping cached tracking usable.
	select {
	case simklImportGate <- struct{}{}:
	case <-ctx.Done():
		http.Error(w, "SIMKL import cancelled", http.StatusRequestTimeout)
		return
	}
	defer func() { <-simklImportGate }()

	var raw []byte
	var fetchedAt time.Time
	var listType, cachedID string
	err = a.DB.QueryRow(ctx, `SELECT id::text,archive,api_fetched_at,coalesce(api_list_type,'') FROM imports WHERE user_id=$1 AND kind='simkl' AND preview->>'type'='simkl-custom-list' AND preview->'source'->>'id'=$2 AND api_fetched_at>now()-CASE WHEN api_list_type='auto' THEN interval '24 hours' ELSE interval '5 minutes' END ORDER BY api_fetched_at DESC LIMIT 1`, auth.Current(ctx).ID, in.ListID).Scan(&cachedID, &raw, &fetchedAt, &listType)
	if err == nil {
		in.Token = ""
		a.previewCustomList(w, r, raw, &fetchedAt, listType, cachedID)
		return
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		http.Error(w, "private list cache unavailable", http.StatusServiceUnavailable)
		return
	}
	provider := a.SIMKL
	if provider == nil {
		provider = defaultSIMKLReadClient
	}
	fetched, err := provider.FetchList(ctx, in.ClientID, in.Token, in.ListID)
	in.Token = ""
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	fetchedAt = time.Now().UTC()
	a.previewCustomList(w, r, fetched.Raw, &fetchedAt, fetched.Type)
}
