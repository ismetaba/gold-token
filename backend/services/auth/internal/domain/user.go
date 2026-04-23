// Package domain defines the User entity and auth domain types.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is the core auth entity. PII fields are minimal — KYC service owns richer profile.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TokenPair is the response for successful login or refresh.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds until access token expiry
	TokenType    string `json:"token_type"` // always "Bearer"
}
