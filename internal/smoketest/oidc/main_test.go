package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSyntheticProviderRequiresPKCEAndSingleUseCode(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	h, err := newHandler("https://oidc:8443", "https://localhost:8443/authorize", key)
	if err != nil {
		t.Fatal(err)
	}
	verifier := "synthetic-verifier-for-pkce"
	sum := sha256.Sum256([]byte(verifier))
	q := url.Values{"client_id": {clientID}, "redirect_uri": {"http://127.0.0.1:8080/auth/callback"}, "nonce": {"synthetic-nonce"}, "state": {"synthetic-state"}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}}
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/authorize?"+q.Encode(), nil))
	if response.Code != http.StatusFound {
		t.Fatal(response.Code, response.Body.String())
	}
	target, err := url.Parse(response.Header().Get("Location"))
	if err != nil || target.Query().Get("state") != "synthetic-state" {
		t.Fatal(target, err)
	}
	form := url.Values{"grant_type": {"authorization_code"}, "code": {target.Query().Get("code")}, "code_verifier": {verifier}}
	exchange := func() *httptest.ResponseRecorder {
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/token", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.SetBasicAuth(clientID, "synthetic")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w
	}
	token := exchange()
	if token.Code != http.StatusOK {
		t.Fatal(token.Code, token.Body.String())
	}
	var body map[string]interface{}
	if err = json.Unmarshal(token.Body.Bytes(), &body); err != nil || body["id_token"] == nil {
		t.Fatal(err, body)
	}
	if repeated := exchange(); repeated.Code != http.StatusUnauthorized {
		t.Fatal("code reused", repeated.Code)
	}

	q.Set("fixture_user", "second")
	response = httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/authorize?"+q.Encode(), nil))
	target, err = url.Parse(response.Header().Get("Location"))
	if err != nil || response.Code != http.StatusFound {
		t.Fatal("second identity authorization failed", err)
	}
	form.Set("code", target.Query().Get("code"))
	token = exchange()
	if err = json.Unmarshal(token.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	verifySecond := func() {
		t.Helper()
		raw, ok := body["id_token"].(string)
		if !ok {
			t.Fatal("missing ID token")
		}
		signed, e := jwt.ParseSigned(raw, []jose.SignatureAlgorithm{jose.RS256})
		if e != nil {
			t.Fatal(e)
		}
		var claims jwt.Claims
		if e = signed.Claims(&key.PublicKey, &claims); e != nil || claims.Subject != "synthetic-ci-second" {
			t.Fatal("second identity changed", claims.Subject, e)
		}
	}
	verifySecond()
	form.Set("grant_type", "refresh_token")
	refresh, ok := body["refresh_token"].(string)
	if !ok {
		t.Fatal("missing refresh")
	}
	form.Set("refresh_token", refresh)
	token = exchange()
	if token.Code != http.StatusOK {
		t.Fatal("refresh failed", token.Code)
	}
	if err = json.Unmarshal(token.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	verifySecond()
	q.Set("code_challenge_method", "plain")
	response = httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/authorize?"+q.Encode(), nil))
	if response.Code != http.StatusBadRequest {
		t.Fatal("unprotected fixture authorization", response.Code)
	}
}
func TestSyntheticCertificateUsesVerifiedHostnames(t *testing.T) {
	dir := t.TempDir()
	if err := generateCertificate(dir); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "ca.crt"))
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		t.Fatal("missing certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	for _, host := range []string{"oidc", "localhost", "127.0.0.1"} {
		if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: host}); err != nil {
			t.Fatal(host, err)
		}
	}
	if _, err = cert.Verify(x509.VerifyOptions{Roots: roots, DNSName: "external.example"}); err == nil {
		t.Fatal("fixture certificate trusted unrelated host")
	}
}

func TestSyntheticAdminClaimsAreExplicitAndSubjectBound(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	h, err := newHandler("https://oidc:8443", "https://localhost:8443/authorize", key)
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"synthetic-ci-user", "synthetic-ci-second", "synthetic-ci-admin", "unknown-admin"} {
		form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {"synthetic-ci-refresh:" + subject}}
		r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/token", strings.NewReader(form.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.SetBasicAuth(clientID, "synthetic")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if subject == "unknown-admin" {
			if w.Code != http.StatusUnauthorized {
				t.Fatal("unknown subject granted access")
			}
			continue
		}
		if w.Code != http.StatusOK {
			t.Fatal(w.Code)
		}
		var body struct {
			IDToken string `json:"id_token"`
		}
		if err = json.Unmarshal(w.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		signed, e := jwt.ParseSigned(body.IDToken, []jose.SignatureAlgorithm{jose.RS256})
		if e != nil {
			t.Fatal(e)
		}
		var claims struct {
			Subject string   `json:"sub"`
			Roles   []string `json:"roles"`
		}
		if e = signed.Claims(&key.PublicKey, &claims); e != nil || claims.Subject != subject {
			t.Fatal("identity changed", e)
		}
		expected := "member"
		if subject == "synthetic-ci-admin" {
			expected = "member,admin"
		}
		if strings.Join(claims.Roles, ",") != expected {
			t.Fatal("unexpected privilege", claims.Roles)
		}
	}
}
