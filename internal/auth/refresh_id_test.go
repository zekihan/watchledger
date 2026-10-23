package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/zekihan/watchledger/internal/config"
	"golang.org/x/oauth2"
)

func TestRefreshValidatesNewIDTokenWithoutAuthorizationFallback(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "synthetic-refresh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, subject, audience                           string
		roles                                             []string
		userInfoRoles                                     []string
		userInfoSubject                                   string
		explicitUserInfo                                  bool
		expired, wrongIssuer, invalid, wantErr, wantAdmin bool
	}{
		{name: "explicit UserInfo current admin", subject: "stable", audience: "test", roles: []string{}, explicitUserInfo: true, userInfoRoles: []string{"member", "admin"}, userInfoSubject: "stable", wantAdmin: true},
		{name: "explicit UserInfo removes admin", subject: "stable", audience: "test", roles: []string{"member", "admin"}, explicitUserInfo: true, userInfoRoles: []string{"member"}, userInfoSubject: "stable"},
		{name: "explicit UserInfo removes admission", subject: "stable", audience: "test", roles: []string{"member", "admin"}, explicitUserInfo: true, userInfoRoles: []string{}, userInfoSubject: "stable", wantErr: true},
		{name: "explicit UserInfo rejects changed subject", subject: "stable", audience: "test", roles: []string{"member"}, explicitUserInfo: true, userInfoRoles: []string{"member", "admin"}, userInfoSubject: "other", wantErr: true},
		{name: "current admin", subject: "stable", audience: "test", roles: []string{"member", "admin"}, wantAdmin: true},
		{name: "admin removed", subject: "stable", audience: "test", roles: []string{"member"}},
		{name: "admission removed", subject: "stable", audience: "test", roles: []string{"admin"}, wantErr: true},
		{name: "subject changed", subject: "other", audience: "test", roles: []string{"member", "admin"}, wantErr: true},
		{name: "wrong audience", subject: "stable", audience: "other", roles: []string{"member", "admin"}, wantErr: true},
		{name: "expired", subject: "stable", audience: "test", roles: []string{"member", "admin"}, expired: true, wantErr: true},
		{name: "wrong issuer", subject: "stable", audience: "test", roles: []string{"member", "admin"}, wrongIssuer: true, wantErr: true},
		{name: "invalid token", subject: "stable", audience: "test", roles: []string{"member", "admin"}, invalid: true, wantErr: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var issuer, encoded string
			var userInfoCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/.well-known/openid-configuration":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"issuer": issuer, "authorization_endpoint": issuer + "/authorize", "token_endpoint": issuer + "/token", "userinfo_endpoint": issuer + "/userinfo", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}})
				case "/keys":
					_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "synthetic-refresh", Algorithm: "RS256", Use: "sig"}}})
				case "/token":
					_ = json.NewEncoder(w).Encode(map[string]interface{}{"access_token": "synthetic-current", "refresh_token": "synthetic-refresh", "token_type": "Bearer", "expires_in": 300, "id_token": encoded})
				case "/userinfo":
					userInfoCalls.Add(1)
					claims := map[string]interface{}{"sub": "stable", "roles": []string{"member", "admin"}}
					if test.explicitUserInfo {
						claims["sub"], claims["roles"] = test.userInfoSubject, test.userInfoRoles
					}
					_ = json.NewEncoder(w).Encode(claims)
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			issuer = server.URL
			tokenIssuer := issuer
			if test.wrongIssuer {
				tokenIssuer = "https://other.example"
			}
			expiry := time.Now().Add(time.Hour)
			if test.expired {
				expiry = time.Now().Add(-time.Hour)
			}
			encoded, err = jwt.Signed(signer).Claims(jwt.Claims{Issuer: tokenIssuer, Subject: test.subject, Audience: jwt.Audience{test.audience}, Expiry: jwt.NewNumericDate(expiry), IssuedAt: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour))}).Claims(map[string]interface{}{"roles": test.roles}).Serialize()
			if err != nil {
				t.Fatal(err)
			}
			if test.invalid {
				encoded = "synthetic.invalid.token"
			}
			source := "auto"
			if test.explicitUserInfo {
				source = "userinfo"
			}
			service, err := New(context.Background(), nil, config.Config{OIDCClaimsSource: source, Issuer: issuer, ClientID: "test", PublicURL: "http://127.0.0.1:8080", SessionKey: base64.StdEncoding.EncodeToString(make([]byte, 32)), AdmissionClaim: "roles", AdmissionValue: "member", AdminClaim: "roles", AdminValue: "admin"})
			if err != nil {
				t.Fatal(err)
			}
			_, admin, err := service.refresh(context.Background(), &oauth2.Token{AccessToken: "old", RefreshToken: "synthetic-refresh", Expiry: time.Now().Add(-time.Hour)}, "stable")
			if (err != nil) != test.wantErr || admin != test.wantAdmin {
				t.Fatalf("admin=%v err=%v", admin, err)
			}
			wantCalls := int32(0)
			if test.explicitUserInfo {
				wantCalls = 1
			}
			if userInfoCalls.Load() != wantCalls {
				t.Fatal("invalid or incomplete new ID token silently fell back to different claims")
			}
		})
	}
}
