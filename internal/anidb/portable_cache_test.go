package anidb

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/zekihan/watchledger/internal/store"
)

func TestPortableNativeCatalogWorksWithoutProviderDocumentOrCredentials(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	p, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if err = store.Migrate(ctx, p); err != nil {
		t.Fatal(err)
	}
	aid := fmt.Sprint(time.Now().UnixNano())
	id := "anidb:anime:" + aid
	defer func() {
		_, _ = p.Exec(ctx, "DELETE FROM episodes WHERE catalog_id=$1", id)
		_, _ = p.Exec(ctx, "DELETE FROM catalog WHERE id=$1", id)
	}()
	_, err = p.Exec(ctx, `INSERT INTO catalog(id,source,source_id,kind,title,metadata,fetched_at) VALUES($1,'anidb',$2,'anime','Source fallback','{"en":{"number_of_episodes":1,"titles":[{"text":"Native name","language":"ja","type":"official"},{"text":"English name","language":"en","type":"official"}]}}',now()-interval '2 days')`, id, aid)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Exec(ctx, `INSERT INTO episodes(id,catalog_id,source,source_id,season,number,metadata) VALUES($1,$2,'anidb',$1,1,1,'{"id":501,"name":"Restored episode","episode_number":1,"season_number":1}')`, "portable-episode:"+aid, id); err != nil {
		t.Fatal(err)
	}
	cache := Cache{DB: p, Provider: New(Config{})}
	result, err := cache.Detail(ctx, aid, "tr", "native")
	if err != nil || !result.Stale || result.Warning == "" || result.Metadata["name"] != "Native name" {
		t.Fatal("restored metadata unavailable", result, err)
	}
	raw, err := json.Marshal(result.Metadata["episodes"])
	if err != nil {
		t.Fatal(err)
	}
	var episodes []struct {
		ID   int
		Name string
	}
	if err = json.Unmarshal(raw, &episodes); err != nil || len(episodes) != 1 || episodes[0].ID != 501 {
		t.Fatal("restored source episodes lost", string(raw), err)
	}
	var original time.Time
	if err = p.QueryRow(ctx, "SELECT fetched_at FROM catalog WHERE id=$1", id).Scan(&original); err != nil || !original.Equal(result.FetchedAt) {
		t.Fatal("freshness changed", original, result.FetchedAt, err)
	}
	result, err = cache.Detail(ctx, aid, "en", "english")
	if err != nil || result.Metadata["name"] != "English name" {
		t.Fatal("title style ignored", result, err)
	}
	if _, err = p.Exec(ctx, "UPDATE catalog SET fetched_at=now() WHERE id=$1", id); err != nil {
		t.Fatal(err)
	}
	result, err = cache.Detail(ctx, aid, "en", "english")
	if err != nil || result.Stale {
		t.Fatal("fresh restored cache rejected", result, err)
	}
	var requests int
	if err = p.QueryRow(ctx, "SELECT count(*) FROM provider_request_limits WHERE key=$1", "anidb:anime:"+aid).Scan(&requests); err != nil || requests != 0 {
		t.Fatal("restored read reserved a provider request", requests, err)
	}
	if _, err = cache.Detail(ctx, fmt.Sprint(time.Now().UnixNano()+100), "en", "english"); err == nil {
		t.Fatal("uncached source falsely succeeded")
	}
}
