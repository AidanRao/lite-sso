// Package mask provides utilities for safely displaying sensitive values.
package mask

import (
	"strings"
)

// Email keeps a small, length-dependent portion of an email local part visible.
func Email(value string) string {
	value = strings.TrimSpace(value)
	at := strings.LastIndex(value, "@")
	if at <= 0 || at == len(value)-1 {
		return "***"
	}

	local := []rune(value[:at])
	switch length := len(local); {
	case length > 8:
		return string(local[:2]) + "***" + string(local[length-2:]) + value[at:]
	case length >= 6:
		return string(local[:2]) + "***" + string(local[length-1:]) + value[at:]
	default:
		return string(local[0]) + "***" + value[at:]
	}
}
