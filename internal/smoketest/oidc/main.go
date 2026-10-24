// Command oidc is a synthetic, ephemeral CI fixture, never an application login
// provider. It is not built into the Watchledger production binary/image.
package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

const clientID = "watchledger-smoke"

type code struct {
	nonce, challenge, subject string
	until                     time.Time
}

func main() {
	if err := run(); err != nil {
		slog.Error("synthetic fixture failed", "error", err)
		os.Exit(1)
	}
}
func run() error {
	generate := flag.Bool("generate", false, "generate ephemeral fixture certificate and key")
	dir := flag.String("dir", "", "ephemeral fixture directory")
	issuer := flag.String("issuer", "https://oidc:8443", "synthetic issuer")
	authorize := flag.String("authorize", "https://localhost:8443/authorize", "public test authorize endpoint")
	flag.Parse()
	if os.Getenv("WATCHLEDGER_SYNTHETIC_OIDC") != "1" || *dir == "" {
		return errors.New("explicit synthetic fixture opt-in and directory required")
	}
	if *generate {
		return generateCertificate(*dir)
	}
	raw, err := os.ReadFile(filepath.Join(*dir, "server.key"))
	if err != nil {
		return err
	}
	block, _ := pem.Decode(raw)
	if block == nil {
		return errors.New("missing fixture key")
	}
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return err
	}
	handler, err := newHandler(*issuer, *authorize, key)
	if err != nil {
		return err
	}
	server := &http.Server{Addr: "0.0.0.0:8443", Handler: handler, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			shutdown, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			_ = server.Shutdown(shutdown)
		case <-done:
		}
	}()
	err = server.ListenAndServeTLS(filepath.Join(*dir, "ca.crt"), filepath.Join(*dir, "server.key"))
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func generateCertificate(dir string) error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return err
	}
	template := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "Watchledger synthetic CI only"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(24 * time.Hour), DNSNames: []string{"oidc", "localhost"}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	cert, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert}), 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "server.key"), pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600)
}
func newHandler(issuer, authorize string, key *rsa.PrivateKey) (http.Handler, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key}, (&jose.SignerOptions{}).WithHeader("kid", "synthetic-ci"))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	var mu sync.Mutex
	codes := map[string]code{}
	write := func(w http.ResponseWriter, value interface{}) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(value)
	}
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		write(w, map[string]interface{}{"issuer": issuer, "authorization_endpoint": authorize, "token_endpoint": issuer + "/token", "jwks_uri": issuer + "/keys", "id_token_signing_alg_values_supported": []string{"RS256"}, "response_types_supported": []string{"code"}, "subject_types_supported": []string{"public"}})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		write(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "synthetic-ci", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("GET /authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("client_id") != clientID || q.Get("redirect_uri") != "http://127.0.0.1:8080/auth/callback" || q.Get("nonce") == "" || q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
			http.Error(w, "invalid synthetic authorization", http.StatusBadRequest)
			return
		}
		subject := "synthetic-ci-user"
		if q.Get("fixture_user") == "second" {
			subject = "synthetic-ci-second"
		} else if q.Get("fixture_user") != "" {
			http.Error(w, "unknown synthetic identity", http.StatusBadRequest)
			return
		}
		random := make([]byte, 32)
		if _, err := rand.Read(random); err != nil {
			http.Error(w, "fixture random unavailable", http.StatusInternalServerError)
			return
		}
		value := base64.RawURLEncoding.EncodeToString(random)
		mu.Lock()
		for id, pending := range codes {
			if time.Now().After(pending.until) {
				delete(codes, id)
			}
		}
		codes[value] = code{subject: subject, nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), until: time.Now().Add(time.Minute)}
		mu.Unlock()
		target := url.Values{"code": {value}, "state": {q.Get("state")}}
		http.Redirect(w, r, q.Get("redirect_uri")+"?"+target.Encode(), http.StatusFound)
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		client, _, _ := r.BasicAuth()
		if client == "" {
			client = r.Form.Get("client_id")
		}
		if client != clientID {
			http.Error(w, "invalid fixture client", http.StatusUnauthorized)
			return
		}
		var nonce, subject string
		switch r.Form.Get("grant_type") {
		case "authorization_code":
			mu.Lock()
			pending, ok := codes[r.Form.Get("code")]
			delete(codes, r.Form.Get("code"))
			mu.Unlock()
			challenge := sha256.Sum256([]byte(r.Form.Get("code_verifier")))
			if !ok || time.Now().After(pending.until) || pending.challenge != base64.RawURLEncoding.EncodeToString(challenge[:]) {
				http.Error(w, "invalid code or PKCE", http.StatusUnauthorized)
				return
			}
			nonce, subject = pending.nonce, pending.subject
		case "refresh_token":
			switch r.Form.Get("refresh_token") {
			case "synthetic-ci-refresh:synthetic-ci-user":
				subject = "synthetic-ci-user"
			case "synthetic-ci-refresh:synthetic-ci-second":
				subject = "synthetic-ci-second"
			default:
				http.Error(w, "invalid refresh", http.StatusUnauthorized)
				return
			}
		default:
			http.Error(w, "unsupported grant", http.StatusBadRequest)
			return
		}
		signed, err := jwt.Signed(signer).Claims(map[string]interface{}{"iss": issuer, "sub": subject, "aud": clientID, "exp": time.Now().Add(5 * time.Minute).Unix(), "iat": time.Now().Unix(), "nonce": nonce, "name": "Synthetic CI User", "roles": []string{"member"}}).Serialize()
		if err != nil {
			http.Error(w, "fixture signing failed", http.StatusInternalServerError)
			return
		}
		write(w, map[string]interface{}{"access_token": "synthetic-ci-access", "refresh_token": "synthetic-ci-refresh:" + subject, "token_type": "Bearer", "expires_in": 300, "id_token": signed})
	})
	return mux, nil
}
