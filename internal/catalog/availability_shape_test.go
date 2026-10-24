package catalog

import (
	"encoding/json"
	"testing"
)

func TestAvailabilityOfferShape(t *testing.T) {
	for _, raw := range []string{`{"flatrate":"wrong"}`, `[]`, `{"free":[{"provider_id":0}]}`, `{"link":{}}`} {
		if validateOffers(json.RawMessage(raw)) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
	for _, raw := range []string{"", `null`, `{}`, `{"free":[{"provider_id":8,"provider_name":"Fixture"}],"ads":[],"link":"https://example.com"}`} {
		if err := validateOffers(json.RawMessage(raw)); err != nil {
			t.Errorf("rejected %s: %v", raw, err)
		}
	}
}
