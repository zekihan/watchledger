package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/store"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"
)

func TestOIDCLoginAndStableIdentity(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	pool, e := store.Open(ctx, dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer pool.Close()
	if e = store.Migrate(ctx, pool); e != nil {
		t.Fatal(e)
	}
	key, e := rsa.GenerateKey(rand.Reader, 2048)
	if e != nil {
		t.Fatal(e)
	}
	signer, e := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "fixture"))
	if e != nil {
		t.Fatal(e)
	}
	var issuer string
	var mu sync.Mutex
	nonce, name := "", "Original name"
	admitted := true
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
		case "/keys":
			_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "fixture", Algorithm: "RS256", Use: "sig"}}})
		case "/token":
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid request", http.StatusBadRequest)
				return
			}
			if r.Form.Get("code_verifier") == "" {
				t.Error("missing PKCE verifier")
			}
			roles := []string{}
			if admitted {
				roles = append(roles, "member")
			}
			signed, err := jwt.Signed(signer).Claims(map[string]interface{}{"iss": issuer, "sub": "stable-subject", "aud": "client", "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "name": name, "roles": roles}).Serialize()
			if err != nil {
				t.Error(err)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "synthetic-access", "refresh_token": "synthetic-refresh", "token_type": "Bearer", "expires_in": 3600, "id_token": signed})
		default:
			http.NotFound(w, r)
		}
	}))
	defer provider.Close()
	issuer = provider.URL
	defer func() { _, _ = pool.Exec(ctx, "DELETE FROM users WHERE issuer=$1", issuer) }()
	s, e := New(ctx, pool, config.Config{Issuer: issuer, ClientID: "client", PublicURL: "http://127.0.0.1:8080", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), AdmissionClaim: "roles", AdmissionValue: "member", AdminClaim: "roles", AdminValue: "admin"})
	if e != nil {
		t.Fatal(e)
	}
	login := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		s.Login(w, httptest.NewRequestWithContext(ctx, "GET", "/auth/login", nil))
		location, err := url.Parse(w.Header().Get("Location"))
		if err != nil {
			t.Fatal(err)
		}
		if location.Query().Get("code_challenge_method") != "S256" {
			t.Fatal("missing PKCE challenge")
		}
		mu.Lock()
		nonce = location.Query().Get("nonce")
		mu.Unlock()
		callback := httptest.NewRequestWithContext(ctx, "GET", "/auth/callback?code=fixture&state="+url.QueryEscape(location.Query().Get("state")), nil)
		for _, c := range w.Result().Cookies() {
			callback.AddCookie(c)
		}
		result := httptest.NewRecorder()
		s.Callback(result, callback)
		return result
	}
	mu.Lock()
	admitted = false
	mu.Unlock()
	denied := login()
	if denied.Code != http.StatusForbidden {
		t.Fatal("unadmitted login accepted", denied.Code)
	}
	var deniedCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE issuer=$1", issuer).Scan(&deniedCount); err != nil || deniedCount != 0 {
		t.Fatal("unadmitted account created", deniedCount, err)
	}
	mu.Lock()
	admitted = true
	mu.Unlock()
	result := login()
	if result.Code != http.StatusSeeOther {
		t.Fatal(result.Code, result.Body.String())
	}
	var count int
	var admin bool
	if e = pool.QueryRow(ctx, "SELECT count(*),bool_or(admin) FROM users WHERE issuer=$1", issuer).Scan(&count, &admin); e != nil || count != 1 || admin {
		t.Fatal("first user promotion", count, admin, e)
	}
	mu.Lock()
	name = "Renamed person"
	mu.Unlock()
	if r := login(); r.Code != http.StatusSeeOther {
		t.Fatal(r.Code, r.Body.String())
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM users WHERE issuer=$1", issuer).Scan(&count); e != nil || count != 1 {
		t.Fatal("mutable name split identity", count, e)
	}
	mu.Lock()
	admitted = false
	mu.Unlock()
	if r := login(); r.Code != http.StatusForbidden {
		t.Fatal("admission ignored", r.Code)
	}
	forged := httptest.NewRecorder()
	s.Callback(forged, httptest.NewRequestWithContext(ctx, "GET", "/auth/callback?state=forged&code=fixture", nil))
	if forged.Code != http.StatusBadRequest {
		t.Fatal("forged state accepted")
	}
}
