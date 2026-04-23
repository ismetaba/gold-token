// Package tokens handles RS256 JWT signing and verification.
package tokens

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	issuer         = "gold-auth"
	claimSubject   = "sub"
	claimTokenType = "token_type"
	typeAccess     = "access"
	typeRefresh    = "refresh"
)

// Manager signs and verifies JWTs with an RSA key pair.
type Manager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewManager loads an RSA key pair from PEM files, or generates an ephemeral
// 2048-bit key when both paths are empty (local dev only).
func NewManager(privateKeyFile, publicKeyFile string, accessTTL, refreshTTL time.Duration) (*Manager, error) {
	if privateKeyFile == "" && publicKeyFile == "" {
		// Local dev: generate ephemeral key.
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		return &Manager{privateKey: priv, publicKey: &priv.PublicKey, accessTTL: accessTTL, refreshTTL: refreshTTL}, nil
	}

	privPEM, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return nil, fmt.Errorf("private key PEM decode failed")
	}
	priv, err := jwt.ParseRSAPrivateKeyFromPEM(privPEM)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}

	pubPEM, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pubPEM)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	return &Manager{privateKey: priv, publicKey: pub, accessTTL: accessTTL, refreshTTL: refreshTTL}, nil
}

// IssueAccess creates a short-lived access token for userID.
func (m *Manager) IssueAccess(userID uuid.UUID) (string, error) {
	return m.sign(userID, typeAccess, m.accessTTL)
}

// IssueRefresh creates a long-lived refresh token for userID.
func (m *Manager) IssueRefresh(userID uuid.UUID) (string, error) {
	return m.sign(userID, typeRefresh, m.refreshTTL)
}

// VerifyAccess validates an access token and returns the userID.
func (m *Manager) VerifyAccess(tokenStr string) (uuid.UUID, error) {
	return m.verify(tokenStr, typeAccess)
}

// VerifyRefresh validates a refresh token and returns the userID.
func (m *Manager) VerifyRefresh(tokenStr string) (uuid.UUID, error) {
	return m.verify(tokenStr, typeRefresh)
}

// AccessTTLSeconds returns the configured access token TTL in seconds.
func (m *Manager) AccessTTLSeconds() int {
	return int(m.accessTTL.Seconds())
}

func (m *Manager) sign(userID uuid.UUID, tokenType string, ttl time.Duration) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":        issuer,
		"sub":        userID.String(),
		"iat":        now.Unix(),
		"exp":        now.Add(ttl).Unix(),
		"jti":        uuid.Must(uuid.NewV7()).String(),
		"token_type": tokenType,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return t.SignedString(m.privateKey)
}

func (m *Manager) verify(tokenStr, expectedType string) (uuid.UUID, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	}, jwt.WithIssuer(issuer), jwt.WithExpirationRequired())
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, fmt.Errorf("invalid claims")
	}

	tt, _ := claims["token_type"].(string)
	if tt != expectedType {
		return uuid.Nil, fmt.Errorf("wrong token type: got %q, want %q", tt, expectedType)
	}

	subStr, _ := claims["sub"].(string)
	id, err := uuid.Parse(subStr)
	if err != nil {
		return uuid.Nil, fmt.Errorf("invalid sub: %w", err)
	}
	return id, nil
}
