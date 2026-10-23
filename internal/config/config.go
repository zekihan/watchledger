// Package config validates deployment configuration before serving requests.
package config

import (
	"fmt"
	"net/url"
	"os"
)

type Config struct {
	Address, WebDir, DatabaseURL, PublicURL                                                string
	Issuer, ClientID, ClientSecret, AdmissionClaim, AdmissionValue, AdminClaim, AdminValue string
	TMDBToken, SessionKey                                                                  string
}

func Load() (Config, error) { return Parse(os.Getenv) }
func Parse(get func(string) string) (Config, error) {
	c := Config{Address: get("WATCHLEDGER_ADDR"), WebDir: get("WATCHLEDGER_WEB_DIR"), DatabaseURL: get("DATABASE_URL"), PublicURL: get("PUBLIC_URL"), Issuer: get("OIDC_ISSUER"), ClientID: get("OIDC_CLIENT_ID"), ClientSecret: get("OIDC_CLIENT_SECRET"), AdmissionClaim: get("OIDC_ADMISSION_CLAIM"), AdmissionValue: get("OIDC_ADMISSION_VALUE"), AdminClaim: get("OIDC_ADMIN_CLAIM"), AdminValue: get("OIDC_ADMIN_VALUE"), TMDBToken: get("TMDB_TOKEN"), SessionKey: get("SESSION_KEY")}
	if c.Address == "" {
		c.Address = "127.0.0.1:8080"
	}
	for name, value := range map[string]string{"SESSION_KEY": c.SessionKey, "DATABASE_URL": c.DatabaseURL, "PUBLIC_URL": c.PublicURL, "OIDC_ISSUER": c.Issuer, "OIDC_CLIENT_ID": c.ClientID, "OIDC_ADMISSION_CLAIM": c.AdmissionClaim, "OIDC_ADMISSION_VALUE": c.AdmissionValue, "OIDC_ADMIN_CLAIM": c.AdminClaim, "OIDC_ADMIN_VALUE": c.AdminValue} {
		if value == "" {
			return c, fmt.Errorf("%s is required", name)
		}
	}
	if c.AdmissionClaim == c.AdminClaim && c.AdmissionValue == c.AdminValue {
		return c, fmt.Errorf("admission and admin policy must be separate")
	}
	for name, raw := range map[string]string{"PUBLIC_URL": c.PublicURL, "OIDC_ISSUER": c.Issuer} {
		u, e := url.Parse(raw)
		if e != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return c, fmt.Errorf("invalid %s", name)
		}
		loopback := u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1")
		if u.Scheme != "https" && !loopback {
			return c, fmt.Errorf("%s requires HTTPS except loopback development", name)
		}
	}
	return c, nil
}
