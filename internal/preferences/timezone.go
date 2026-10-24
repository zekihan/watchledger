// Package preferences validates portable, user-visible application preferences.
package preferences

import (
	"strings"
	"time"
)

// ValidTimezone excludes Go's host-dependent pseudo-zone and zoneinfo tree
// prefixes that PostgreSQL and browser Intl do not interpret as IANA names.
func ValidTimezone(zone string) bool {
	if zone == "" || zone == "Local" || strings.HasPrefix(zone, "posix/") || strings.HasPrefix(zone, "right/") {
		return false
	}
	_, err := time.LoadLocation(zone)
	return err == nil
}
