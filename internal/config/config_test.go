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
