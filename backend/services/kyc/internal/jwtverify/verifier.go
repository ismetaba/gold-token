// Package jwtverify provides RS256 JWT verification for the KYC service.
//
// The KYC service only needs to verify tokens; it never issues them.
// In local mode (no public key file) the verifier skips signature verification
// and only decodes claims — suitable for development only.
package jwtverify

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	issuer     = "gold-auth"
	typeAccess = "access"
)

// Verifier verifies RS256 access tokens issued by the auth service.
type Verifier struct {
	publicKey any // *rsa.PublicKey; nil means permissive local mode
}

// New loads the RSA public key from publicKeyFile.
// If publicKeyFile is empty, returns a permissive verifier that skips
// signature validation (local dev only — config enforces the file in prod).
func New(publicKeyFile string) (*Verifier, error) {
	if publicKeyFile == "" {
		return &Verifier{}, nil
	}
	pem, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pem)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}
	return &Verifier{publicKey: pub}, nil
}

// VerifyAccess validates an access token and returns the subject user ID.
// In local mode (no public key), signature is not verified.
func (v *Verifier) VerifyAccess(tokenStr string) (uuid.UUID, error) {
	if v.publicKey == nil {
		return v.unsafeExtractSub(tokenStr)
	}

	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return v.publicKey, nil
	}, jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid claims type")
	}

	if tt, _ := claims["token_type"].(string); tt != typeAccess {
		return uuid.Nil, fmt.Errorf("wrong token_type: %q", tt)
	}

	subStr, _ := claims["sub"].(string)
	id, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid sub: %w", err)
	}
	return id, nil
}

// unsafeExtractSub decodes the JWT payload without verifying the signature.
// Only used in local development when no public key is configured.
func (v *Verifier) unsafeExtractSub(tokenStr string) (uuid.UUID, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return uuid.Nil, fmt.Errorf("malformed token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return uuid.Nil, fmt.Errorf("decode payload: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return uuid.Nil, fmt.Errorf("unmarshal claims: %w", err)
	}
	subStr, _ := claims["sub"].(string)
	id, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid sub: %w", err)
	}
	return id, nil
}
