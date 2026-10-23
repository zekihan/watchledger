// Package auth implements OIDC-only authentication and current-claim authorization.
package auth

import (
	"errors"
	"strings"
)

var ErrAdmission = errors.New("admission denied")

type Policy struct{ AdmissionClaim, AdmissionValue, AdminClaim, AdminValue string }

// Claim paths use dot-separated object keys. String arrays and scalar strings are supported.
func matches(claims map[string]interface{}, path, want string) bool {
	var v interface{} = claims
	for _, key := range strings.Split(path, ".") {
		m, ok := v.(map[string]interface{})
		if !ok {
			return false
		}
		v = m[key]
	}
	switch x := v.(type) {
	case string:
		return x == want
	case []interface{}:
		for _, item := range x {
			if s, ok := item.(string); ok && s == want {
				return true
			}
		}
	}
	return false
}
func (p Policy) Evaluate(claims map[string]interface{}) (bool, error) {
	if p.AdmissionClaim == "" || p.AdmissionValue == "" || !matches(claims, p.AdmissionClaim, p.AdmissionValue) {
		return false, ErrAdmission
	}
	return p.AdminClaim != "" && p.AdminValue != "" && matches(claims, p.AdminClaim, p.AdminValue), nil
}
