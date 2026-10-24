package preferences

import "testing"

func TestTimezoneNamesPortableAcrossConsumers(t *testing.T) {
	for _, zone := range []string{"UTC", "Europe/Istanbul", "America/New_York", "Asia/Tokyo"} {
		if !ValidTimezone(zone) {
			t.Fatalf("valid zone rejected %s", zone)
		}
	}
	for _, zone := range []string{"", "Local", "right/UTC", "posix/Europe/Istanbul", "Not/AZone"} {
		if ValidTimezone(zone) {
			t.Fatalf("unsupported zone accepted %s", zone)
		}
	}
}
