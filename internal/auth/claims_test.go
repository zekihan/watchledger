package auth

import "testing"

func TestPolicy(t *testing.T) {
	p := Policy{"realm.roles", "watchledger", "realm.roles", "watchledger-admin"}
	cases := []struct {
		name         string
		claims       map[string]interface{}
		allow, admin bool
	}{
		{"missing", map[string]interface{}{}, false, false},
		{"admin alone", map[string]interface{}{"realm": map[string]interface{}{"roles": []interface{}{"watchledger-admin"}}}, false, false},
		{"member", map[string]interface{}{"realm": map[string]interface{}{"roles": []interface{}{"watchledger"}}}, true, false},
		{"admin", map[string]interface{}{"realm": map[string]interface{}{"roles": []interface{}{"watchledger", "watchledger-admin"}}}, true, true},
		{"malformed", map[string]interface{}{"realm": map[string]interface{}{"roles": true}}, false, false},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			admin, e := p.Evaluate(tt.claims)
			if (e == nil) != tt.allow || admin != tt.admin {
				t.Fatalf("admin=%v err=%v", admin, e)
			}
		})
	}
}
