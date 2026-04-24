package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleSuperAdmin      Role = "super_admin"
	RoleOps             Role = "ops"
	RoleComplianceViewer Role = "compliance_viewer"
	RoleViewer          Role = "viewer"
)

type AdminUser struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         Role
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type APIKey struct {
	ID           uuid.UUID
	AdminUserID  uuid.UUID
	KeyHash      string
	Name         string
	Scopes       []string
	LastUsedAt   *time.Time
	ExpiresAt    *time.Time
	CreatedAt    time.Time
}

type Session struct {
	ID          uuid.UUID
	AdminUserID uuid.UUID
	TokenHash   string
	IPAddress   string
	UserAgent   string
	ExpiresAt   time.Time
	CreatedAt   time.Time
}
