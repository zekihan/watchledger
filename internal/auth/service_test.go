package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"github.com/zekihan/watchledger/internal/config"
	"golang.org/x/oauth2"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRefreshRequiresCurrentClaims(t *testing.T) {
	for _, tt := range []struct {
		name, subject      string
		claims             map[string]interface{}
		wantErr, wantAdmin bool
	}{
		{"admin removed", "stable", map[string]interface{}{"roles": []string{"member"}}, false, false},
		{"admin present", "stable", map[string]interface{}{"roles": []string{"member", "admin"}}, false, true},
		{"admission removed", "stable", map[string]interface{}{}, true, false},
		{"wrong userinfo subject", "other", map[string]interface{}{"roles": []string{"member", "admin"}}, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var issuer string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "userinfo_endpoint": issuer + "/userinfo", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
				case "/token":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "synthetic-current", "refresh_token": "synthetic-refresh", "token_type": "Bearer", "expires_in": 300})
				case "/userinfo":
					claims := map[string]interface{}{"sub": tt.subject}
					for k, v := range tt.claims {
						claims[k] = v
					}
					_ = json.NewEncoder(w).Encode(claims)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			issuer = server.URL
			c := config.Config{Issuer: issuer, PublicURL: "http://127.0.0.1:8080", ClientID: "test", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), AdmissionClaim: "roles", AdmissionValue: "member", AdminClaim: "roles", AdminValue: "admin"}
			s, e := New(context.Background(), nil, c)
			if e != nil {
				t.Fatal(e)
			}
			_, admin, e := s.refresh(context.Background(), &oauth2.Token{AccessToken: "old", RefreshToken: "synthetic-refresh", Expiry: time.Now().Add(-time.Hour)}, "stable")
			if (e != nil) != tt.wantErr || admin != tt.wantAdmin {
				t.Fatalf("admin=%v err=%v", admin, e)
			}
		})
	}
}
func TestTokenEncryptionRejectsTampering(t *testing.T) {
	b, e := aes.NewCipher(make([]byte, 32))
	if e != nil {
		t.Fatal(e)
	}
	box, e := cipher.NewGCM(b)
	if e != nil {
		t.Fatal(e)
	}
	s := Service{box: box}
	data, e := s.encrypt(&oauth2.Token{AccessToken: "synthetic-private"})
	if e != nil {
		t.Fatal(e)
	}
	tok, e := s.decrypt(data)
	if e != nil || tok.AccessToken != "synthetic-private" {
		t.Fatal(e)
	}
	var sealed []byte
	if e = json.Unmarshal(data, &sealed); e != nil {
		t.Fatal(e)
	}
	sealed[len(sealed)-1] ^= 1
	data, e = json.Marshal(sealed)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = s.decrypt(data); e == nil {
		t.Fatal("tampered session accepted")
	}
}
