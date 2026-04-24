// Package tokens handles admin JWT signing and verification with role claims.
package tokens

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	adminIssuer = "gold-admin"
	claimRole   = "role"
	claimEmail  = "email"
)

// AdminClaims are extracted from a verified admin JWT.
type AdminClaims struct {
	UserID uuid.UUID
	Email  string
	Role   string
}

// Manager signs and verifies admin JWTs with an RSA key pair.
type Manager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	tokenTTL   time.Duration
}

// NewManager loads an RSA key pair from PEM files, or generates an ephemeral
// 2048-bit key when both paths are empty (local dev only).
func NewManager(privateKeyFile, publicKeyFile string) (*Manager, error) {
	ttl := 8 * time.Hour

	if privateKeyFile == "" && publicKeyFile == "" {
		priv, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			return nil, fmt.Errorf("generate ephemeral key: %w", err)
		}
		return &Manager{privateKey: priv, publicKey: &priv.PublicKey, tokenTTL: ttl}, nil
	}

	privPEM, err := os.ReadFile(privateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
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

	return &Manager{privateKey: priv, publicKey: pub, tokenTTL: ttl}, nil
}

// Issue creates a signed admin JWT carrying the user's ID, email, and role.
func (m *Manager) Issue(userID uuid.UUID, email, role string) (string, error) {
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   adminIssuer,
		"sub":   userID.String(),
		"iat":   now.Unix(),
		"exp":   now.Add(m.tokenTTL).Unix(),
		"jti":   uuid.Must(uuid.NewV7()).String(),
		"email": email,
		"role":  role,
	}
	t := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return t.SignedString(m.privateKey)
}

// Verify validates a token string and returns the embedded claims.
func (m *Manager) Verify(tokenStr string) (AdminClaims, error) {
	t, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	}, jwt.WithIssuer(adminIssuer), jwt.WithExpirationRequired())
	if err != nil {
		return AdminClaims{}, fmt.Errorf("invalid token: %w", err)
	}

	mapClaims, ok := t.Claims.(jwt.MapClaims)
	if !ok {
		return AdminClaims{}, fmt.Errorf("invalid claims type")
	}

	subStr, _ := mapClaims["sub"].(string)
	userID, err := uuid.Parse(subStr)
	if err != nil {
		return AdminClaims{}, fmt.Errorf("invalid sub: %w", err)
	}

	email, _ := mapClaims["email"].(string)
	role, _ := mapClaims["role"].(string)

	return AdminClaims{UserID: userID, Email: email, Role: role}, nil
}
