package config

import "testing"

func TestConfigFailClosed(t *testing.T) {
	m := map[string]string{"DATABASE_URL": "postgres://localhost/test", "PUBLIC_URL": "http://127.0.0.1:8080", "OIDC_ISSUER": "https://identity.example", "OIDC_CLIENT_ID": "app", "OIDC_ADMISSION_CLAIM": "roles", "OIDC_ADMISSION_VALUE": "member", "OIDC_ADMIN_CLAIM": "roles", "OIDC_ADMIN_VALUE": "admin", "SESSION_KEY": "provided"}
	get := func(k string) string { return m[k] }
	if _, e := Parse(get); e != nil {
		t.Fatal(e)
	}
	delete(m, "OIDC_ADMISSION_CLAIM")
	if _, e := Parse(get); e == nil {
		t.Fatal("missing admission accepted")
	}
	m["OIDC_ADMISSION_CLAIM"] = "roles"
	m["PUBLIC_URL"] = "http://public.example"
	if _, e := Parse(get); e == nil {
		t.Fatal("insecure public origin accepted")
	}
}

func TestOIDCClaimConfiguration(t *testing.T) {
	base := map[string]string{"DATABASE_URL": "postgres://localhost/test", "PUBLIC_URL": "http://127.0.0.1:8080", "OIDC_ISSUER": "https://identity.example", "OIDC_CLIENT_ID": "app", "OIDC_ADMISSION_CLAIM": "roles", "OIDC_ADMISSION_VALUE": "member", "OIDC_ADMIN_CLAIM": "roles", "OIDC_ADMIN_VALUE": "admin", "SESSION_KEY": "provided"}
	for _, test := range []struct {
		name, source, scopes string
		fail                 bool
	}{
		{"default", "", "", false}, {"userinfo", "userinfo", "groups custom:access", false}, {"invalid source", "cached", "", true}, {"invalid scope", "auto", "bad\\scope", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			get := func(k string) string {
				if k == "OIDC_CLAIMS_SOURCE" {
					return test.source
				}
				if k == "OIDC_EXTRA_SCOPES" {
					return test.scopes
				}
				return base[k]
			}
			c, err := Parse(get)
			if (err != nil) != test.fail {
				t.Fatal("unexpected configuration result", err)
			}
			if !test.fail && c.OIDCClaimsSource == "" {
				t.Fatal("missing default source")
			}
		})
	}
}
