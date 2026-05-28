// Package secrets provides helpers for generating ephemeral credentials.
package secrets

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// RandomHex returns a hex-encoded, cryptographically-random string of nBytes
// of entropy (so the returned string is 2*nBytes characters long).
//
// It is used to mint an ephemeral admin secret for local development when none
// is configured, so services never fall back to a well-known static default
// that could be reused verbatim in a real environment.
func RandomHex(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random secret: %w", err)
	}
	return hex.EncodeToString(b), nil
}
