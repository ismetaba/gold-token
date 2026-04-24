// Package jwtverify provides RS256 JWT verification for the notification service.
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

type Verifier struct {
	publicKey any
}

func NewVerifier(publicKeyFile string) (*Verifier, error) {
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
