package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zekihan/watchledger/internal/config"
	"github.com/zekihan/watchledger/internal/store/dbgen"
	"golang.org/x/oauth2"
)

type Principal struct {
	ID, Name string
	Admin    bool
	Settings json.RawMessage
}
type contextKey struct{}

func Current(ctx context.Context) Principal { p, _ := ctx.Value(contextKey{}).(Principal); return p }

type flow struct {
	nonce, verifier string
	until           time.Time
}
type Service struct {
	pool     *pgxpool.Pool
	q        *dbgen.Queries
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
	policy   Policy
	origin   string
	secure   bool
	box      cipher.AEAD
	mu       sync.Mutex
	flows    map[string]flow
}

func New(ctx context.Context, pool *pgxpool.Pool, c config.Config) (*Service, error) {
	key, e := base64.StdEncoding.DecodeString(c.SessionKey)
	if e != nil || len(key) != 32 {
		return nil, errors.New("SESSION_KEY must be base64 of 32 random bytes")
	}
	block, e := aes.NewCipher(key)
	if e != nil {
		return nil, e
	}
	box, e := cipher.NewGCM(block)
	if e != nil {
		return nil, e
	}
	provider, e := oidc.NewProvider(ctx, c.Issuer)
	if e != nil {
		return nil, fmt.Errorf("OIDC discovery: %w", e)
	}
	return &Service{pool: pool, q: dbgen.New(pool), provider: provider, verifier: provider.Verifier(&oidc.Config{ClientID: c.ClientID}), oauth: oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: provider.Endpoint(), RedirectURL: strings.TrimRight(c.PublicURL, "/") + "/auth/callback", Scopes: []string{oidc.ScopeOpenID, "profile", "offline_access"}}, policy: Policy{c.AdmissionClaim, c.AdmissionValue, c.AdminClaim, c.AdminValue}, origin: strings.TrimRight(c.PublicURL, "/"), secure: strings.HasPrefix(c.PublicURL, "https:"), box: box, flows: map[string]flow{}}, nil
}
func random() string       { return oauth2.GenerateVerifier() }
func hash(s string) string { x := sha256.Sum256([]byte(s)); return hex.EncodeToString(x[:]) }
func (s *Service) cookie(w http.ResponseWriter, name, value string, seconds int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: "/", HttpOnly: true, Secure: s.secure, SameSite: http.SameSiteLaxMode, MaxAge: seconds})
}
func (s *Service) Login(w http.ResponseWriter, r *http.Request) {
	state, nonce, verifier := random(), random(), oauth2.GenerateVerifier()
	s.mu.Lock()
	for k, f := range s.flows {
		if time.Now().After(f.until) {
			delete(s.flows, k)
		}
	}
	if len(s.flows) >= 1000 {
		s.mu.Unlock()
		http.Error(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}
	s.flows[state] = flow{nonce, verifier, time.Now().Add(10 * time.Minute)}
	s.mu.Unlock()
	s.cookie(w, "wl_login", state, 600)
	http.Redirect(w, r, s.oauth.AuthCodeURL(state, oidc.Nonce(nonce), oauth2.S256ChallengeOption(verifier)), http.StatusFound)
}
func (s *Service) Callback(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("state")
	cookie, e := r.Cookie("wl_login")
	if e != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		http.Error(w, "invalid login state", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	f, ok := s.flows[state]
	delete(s.flows, state)
	s.mu.Unlock()
	s.cookie(w, "wl_login", "", -1)
	if !ok || time.Now().After(f.until) {
		http.Error(w, "login expired", http.StatusBadRequest)
		return
	}
	tok, e := s.oauth.Exchange(r.Context(), r.URL.Query().Get("code"), oauth2.VerifierOption(f.verifier))
	if e != nil {
		http.Error(w, "login exchange failed", http.StatusUnauthorized)
		return
	}
	raw, _ := tok.Extra("id_token").(string)
	id, e := s.verifier.Verify(r.Context(), raw)
	if e != nil || id.Nonce != f.nonce {
		http.Error(w, "invalid identity token", http.StatusUnauthorized)
		return
	}
	claims := map[string]interface{}{}
	if e = id.Claims(&claims); e != nil {
		http.Error(w, "invalid identity claims", http.StatusUnauthorized)
		return
	}
	admin, e := s.policy.Evaluate(claims)
	if e != nil {
		http.Error(w, "access not admitted", http.StatusForbidden)
		return
	}
	name, _ := claims["name"].(string)
	user, e := s.q.UpsertUser(r.Context(), dbgen.UpsertUserParams{Issuer: id.Issuer, Subject: id.Subject, Name: name, Admin: admin})
	if e != nil {
		http.Error(w, "account unavailable", http.StatusServiceUnavailable)
		return
	}
	// Never accept session fixation: replace any previous browser session.
	if old, err := r.Cookie("wl_session"); err == nil {
		_ = s.q.DeleteSession(r.Context(), hash(old.Value))
	}
	encrypted, e := s.encrypt(tok)
	if e != nil {
		http.Error(w, "session unavailable", http.StatusServiceUnavailable)
		return
	}
	sid := random()
	expires := tok.Expiry
	if expires.IsZero() || expires.After(time.Now().Add(time.Hour)) {
		expires = time.Now().Add(time.Hour)
	}
	e = s.q.SaveSession(r.Context(), dbgen.SaveSessionParams{TokenHash: hash(sid), UserID: user.ID, Token: encrypted, ExpiresAt: pgtype.Timestamptz{Time: expires, Valid: true}})
	if e != nil {
		http.Error(w, "session unavailable", http.StatusServiceUnavailable)
		return
	}
	s.cookie(w, "wl_session", sid, 43200)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
func (s *Service) encrypt(tok *oauth2.Token) ([]byte, error) {
	plain, e := json.Marshal(tok)
	if e != nil {
		return nil, e
	}
	nonce := make([]byte, s.box.NonceSize())
	if _, e = rand.Read(nonce); e != nil {
		return nil, e
	}
	return json.Marshal(s.box.Seal(nonce, nonce, plain, []byte("watchledger-session-v1")))
}
func (s *Service) decrypt(data []byte) (*oauth2.Token, error) {
	var sealed []byte
	if e := json.Unmarshal(data, &sealed); e != nil {
		return nil, e
	}
	n := s.box.NonceSize()
	if len(sealed) < n {
		return nil, errors.New("invalid session")
	}
	plain, e := s.box.Open(nil, sealed[:n], sealed[n:], []byte("watchledger-session-v1"))
	if e != nil {
		return nil, e
	}
	var tok oauth2.Token
	e = json.Unmarshal(plain, &tok)
	return &tok, e
}

// Current refresh claims must be obtained anew. A refresh without an ID token
// requires a subject-validated UserInfo response, never cached claims.
func (s *Service) refresh(ctx context.Context, tok *oauth2.Token, subject string) (*oauth2.Token, bool, error) {
	if tok.RefreshToken == "" {
		return nil, false, errors.New("login required")
	}
	tok.Expiry = time.Now().Add(-time.Minute)
	fresh, e := s.oauth.TokenSource(ctx, tok).Token()
	if e != nil {
		return nil, false, e
	}
	claims := map[string]interface{}{}
	if raw, ok := fresh.Extra("id_token").(string); ok && raw != "" {
		id, err := s.verifier.Verify(ctx, raw)
		if err != nil {
			return nil, false, err
		}
		if id.Subject != subject {
			return nil, false, errors.New("subject changed")
		}
		if err = id.Claims(&claims); err != nil {
			return nil, false, err
		}
	} else {
		info, err := s.provider.UserInfo(ctx, oauth2.StaticTokenSource(fresh))
		if err != nil {
			return nil, false, err
		}
		if info.Subject != subject {
			return nil, false, errors.New("userinfo subject changed")
		}
		if err = info.Claims(&claims); err != nil {
			return nil, false, err
		}
	}
	admin, e := s.policy.Evaluate(claims)
	return fresh, admin, e
}
func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Origin") != s.origin {
			http.Error(w, "invalid origin", http.StatusForbidden)
			return
		}
		cookie, e := r.Cookie("wl_session")
		if e != nil {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		key := hash(cookie.Value)
		// Serialize refreshes per session across application replicas using a database lock.
		conn, e := s.pool.Acquire(r.Context())
		if e != nil {
			http.Error(w, "session unavailable", http.StatusServiceUnavailable)
			return
		}
		released := false
		defer func() {
			if !released {
				conn.Release()
			}
		}()
		if _, e = conn.Exec(r.Context(), "SELECT pg_advisory_lock(hashtextextended($1,0))", key); e != nil {
			http.Error(w, "session unavailable", http.StatusServiceUnavailable)
			return
		}
		defer func() {
			if released {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_, _ = conn.Exec(ctx, "SELECT pg_advisory_unlock(hashtextextended($1,0))", key)
		}()
		q := dbgen.New(conn)
		u, e := q.UserBySession(r.Context(), key)
		if e != nil {
			http.Error(w, "login required", http.StatusUnauthorized)
			return
		}
		if time.Now().After(u.ExpiresAt.Time) {
			tok, err := s.decrypt(u.Token)
			if err == nil {
				var fresh *oauth2.Token
				fresh, u.Admin, err = s.refresh(r.Context(), tok, u.Subject)
				if err == nil {
					var encrypted []byte
					encrypted, err = s.encrypt(fresh)
					if err == nil {
						expiry := fresh.Expiry
						if expiry.IsZero() || expiry.After(time.Now().Add(time.Hour)) {
							expiry = time.Now().Add(time.Hour)
						}
						err = q.RefreshSession(r.Context(), dbgen.RefreshSessionParams{TokenHash: key, Token: encrypted, ExpiresAt: pgtype.Timestamptz{Time: expiry, Valid: true}})
					}
					if err == nil {
						err = q.SetAdmin(r.Context(), dbgen.SetAdminParams{ID: u.ID, Admin: u.Admin})
					}
				}
			}
			if err != nil {
				_ = q.DeleteSession(r.Context(), key)
				s.cookie(w, "wl_session", "", -1)
				http.Error(w, "login required", http.StatusUnauthorized)
				return
			}
		}
		p := Principal{ID: u.ID.String(), Name: u.Name, Admin: u.Admin, Settings: u.Settings}
		if _, e = conn.Exec(r.Context(), "SELECT pg_advisory_unlock(hashtextextended($1,0))", key); e != nil {
			http.Error(w, "session unavailable", http.StatusServiceUnavailable)
			return
		}
		conn.Release()
		released = true
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), contextKey{}, p)))
	})
}
func (s *Service) Logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("wl_session"); e == nil {
		_ = s.q.DeleteSession(r.Context(), hash(c.Value))
	}
	s.cookie(w, "wl_session", "", -1)
	w.WriteHeader(http.StatusNoContent)
}
