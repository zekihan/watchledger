package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func testProofService(t *testing.T, key byte) *Service {
	t.Helper()
	raw := make([]byte, 32)
	raw[0] = key
	block, err := aes.NewCipher(raw)
	if err != nil {
		t.Fatal(err)
	}
	box, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return &Service{box: box}
}
func TestAbandonedLoginsCannotExhaustGlobalPool(t *testing.T) {
	s := testProofService(t, 1)
	var first string
	for i := 0; i < 1100; i++ {
		r := httptest.NewRequestWithContext(context.Background(), "GET", "/auth/login", nil)
		w := httptest.NewRecorder()
		s.Login(w, r)
		if w.Code != 302 {
			t.Fatal(i, w.Code)
		}
		if i == 0 {
			first = w.Result().Cookies()[0].Value
		}
	}
	if _, err := s.openFlow(first); err != nil {
		t.Fatal("older flow evicted", err)
	}
}
func TestLoginCookieIsBoundedEncryptedAndExpires(t *testing.T) {
	s := testProofService(t, 1)
	f := flow{State: "state", Nonce: "nonce", Verifier: "verifier", Until: time.Now().Add(time.Minute)}
	encoded, err := s.sealFlow(f)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := s.openFlow(encoded)
	if err != nil || decoded.State != f.State || decoded.Verifier != f.Verifier {
		t.Fatal(err)
	}
	if _, err = testProofService(t, 2).openFlow(encoded); err == nil {
		t.Fatal("foreign installation cookie accepted")
	}
	if _, err = s.openFlow(strings.Repeat("a", 2049)); err == nil {
		t.Fatal("oversized cookie accepted")
	}
	f.Until = time.Now().Add(-time.Minute)
	encoded, err = s.sealFlow(f)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequestWithContext(context.Background(), "GET", "/auth/callback?state=state", nil)
	r.AddCookie(&http.Cookie{Name: "wl_login", Value: encoded})
	w := httptest.NewRecorder()
	s.Callback(w, r)
	if w.Code != 400 || !strings.Contains(w.Body.String(), "expired") {
		t.Fatal(w.Code)
	}
}
func TestCatalogProofIsInstallationBoundAndTamperEvident(t *testing.T) {
	makeService := func(key byte) *Service {
		raw := make([]byte, 32)
		raw[0] = key
		block, err := aes.NewCipher(raw)
		if err != nil {
			t.Fatal(err)
		}
		box, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		return &Service{box: box}
	}
	first, other := makeService(1), makeService(2)
	digest := []byte("synthetic digest")
	proof, err := first.SignCatalogProof(digest)
	if err != nil {
		t.Fatal(err)
	}
	if !first.VerifyCatalogProof(proof, digest) {
		t.Fatal("valid proof rejected")
	}
	if first.VerifyCatalogProof(proof, []byte("changed")) || other.VerifyCatalogProof(proof, digest) || first.VerifyCatalogProof("invalid", digest) {
		t.Fatal("invalid proof accepted")
	}
}
