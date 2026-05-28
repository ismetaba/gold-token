package httputil

import "crypto/subtle"

// ValidAdminSecret reports whether the request-supplied secret matches the
// configured one, using a constant-time comparison to avoid timing oracles.
//
// It deliberately returns false when EITHER the configured secret or the
// supplied value is empty. This guarantees that a service started without an
// admin secret (e.g. a missing *_ADMIN_SECRET env var) can never be
// authenticated by sending an empty X-Admin-Secret header — a fail-safe
// default rather than a fail-open one.
func ValidAdminSecret(configured, supplied string) bool {
	if configured == "" || supplied == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(supplied), []byte(configured)) == 1
}
