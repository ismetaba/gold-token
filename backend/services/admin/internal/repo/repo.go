package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ismetaba/gold-token/backend/services/admin/internal/domain"
)

var ErrNotFound = errors.New("record not found")

// ── AdminUserRepo ─────────────────────────────────────────────────────────────

type AdminUserRepo interface {
	ByEmail(ctx context.Context, email string) (domain.AdminUser, error)
	ByID(ctx context.Context, id uuid.UUID) (domain.AdminUser, error)
	Create(ctx context.Context, u domain.AdminUser) error
}

type pgAdminUserRepo struct{ pool *pgxpool.Pool }

func NewPGAdminUserRepo(pool *pgxpool.Pool) AdminUserRepo { return &pgAdminUserRepo{pool: pool} }

func (r *pgAdminUserRepo) ByEmail(ctx context.Context, email string) (domain.AdminUser, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, active, created_at, updated_at
		 FROM admin.users WHERE email = $1`, email)
	return scanAdminUser(row)
}

func (r *pgAdminUserRepo) ByID(ctx context.Context, id uuid.UUID) (domain.AdminUser, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, active, created_at, updated_at
		 FROM admin.users WHERE id = $1`, id)
	return scanAdminUser(row)
}

func (r *pgAdminUserRepo) Create(ctx context.Context, u domain.AdminUser) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO admin.users (id, email, password_hash, role, active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		u.ID, u.Email, u.PasswordHash, string(u.Role), u.Active, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert admin user: %w", err)
	}
	return nil
}

func scanAdminUser(row pgx.Row) (domain.AdminUser, error) {
	var u domain.AdminUser
	var role string
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &u.Active, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.AdminUser{}, ErrNotFound
		}
		return domain.AdminUser{}, fmt.Errorf("scan admin user: %w", err)
	}
	u.Role = domain.Role(role)
	return u, nil
}

// ── APIKeyRepo ────────────────────────────────────────────────────────────────

type APIKeyRepo interface {
	Create(ctx context.Context, k domain.APIKey) error
	ListByUser(ctx context.Context, adminUserID uuid.UUID) ([]domain.APIKey, error)
}

type pgAPIKeyRepo struct{ pool *pgxpool.Pool }

func NewPGAPIKeyRepo(pool *pgxpool.Pool) APIKeyRepo { return &pgAPIKeyRepo{pool: pool} }

func (r *pgAPIKeyRepo) Create(ctx context.Context, k domain.APIKey) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO admin.api_keys (id, admin_user_id, key_hash, name, scopes, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		k.ID, k.AdminUserID, k.KeyHash, k.Name, k.Scopes, k.ExpiresAt, k.CreatedAt)
	if err != nil {
		return fmt.Errorf("insert api key: %w", err)
	}
	return nil
}

func (r *pgAPIKeyRepo) ListByUser(ctx context.Context, adminUserID uuid.UUID) ([]domain.APIKey, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, admin_user_id, key_hash, name, scopes, last_used_at, expires_at, created_at
		 FROM admin.api_keys WHERE admin_user_id = $1 ORDER BY created_at DESC`, adminUserID)
	if err != nil {
		return nil, fmt.Errorf("query api keys: %w", err)
	}
	defer rows.Close()

	var keys []domain.APIKey
	for rows.Next() {
		var k domain.APIKey
		if err := rows.Scan(
			&k.ID, &k.AdminUserID, &k.KeyHash, &k.Name, &k.Scopes,
			&k.LastUsedAt, &k.ExpiresAt, &k.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}
