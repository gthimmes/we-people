// Package iam owns identity and access: users, roles, permissions, and tokens.
package iam

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a user or role does not exist.
var ErrNotFound = errors.New("not found")

// User is a login identity within an organization.
type User struct {
	ID        uuid.UUID  `json:"id"`
	OrgID     uuid.UUID  `json:"org_id"`
	Email     string     `json:"email"`
	Status    string     `json:"status"`
	WorkerID  *uuid.UUID `json:"worker_id,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Role is a named bundle of permissions within an organization.
type Role struct {
	ID          uuid.UUID `json:"id"`
	OrgID       uuid.UUID `json:"org_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
}

// Store provides data access for IAM entities.
type Store struct{ pool *pgxpool.Pool }

// NewStore builds an IAM Store.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Pool exposes the underlying pool so services can run cross-store transactions.
func (s *Store) Pool() *pgxpool.Pool { return s.pool }

// --- Users ---

// CreateUserTx inserts a user within a transaction.
func (s *Store) CreateUserTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, email, passwordHash string) (User, error) {
	var u User
	err := tx.QueryRow(ctx, `
		INSERT INTO users (org_id, email, password_hash)
		VALUES ($1, $2, $3)
		RETURNING id, org_id, email, status, worker_id, created_at`,
		orgID, email, passwordHash).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.Status, &u.WorkerID, &u.CreatedAt)
	return u, err
}

// loginRow carries the fields needed to authenticate.
type loginRow struct {
	User         User
	PasswordHash string
}

// GetUserForLogin finds an active user by org slug + email and returns the
// password hash for verification.
func (s *Store) GetUserForLogin(ctx context.Context, slug, email string) (loginRow, error) {
	var lr loginRow
	err := s.pool.QueryRow(ctx, `
		SELECT u.id, u.org_id, u.email, u.status, u.worker_id, u.created_at, u.password_hash
		FROM users u
		JOIN organizations o ON o.id = u.org_id
		WHERE o.slug = $1 AND u.email = $2`,
		slug, email).
		Scan(&lr.User.ID, &lr.User.OrgID, &lr.User.Email, &lr.User.Status,
			&lr.User.WorkerID, &lr.User.CreatedAt, &lr.PasswordHash)
	if errors.Is(err, pgx.ErrNoRows) {
		return loginRow{}, ErrNotFound
	}
	return lr, err
}

// GetUserByID returns a user by id.
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	err := s.pool.QueryRow(ctx, `
		SELECT id, org_id, email, status, worker_id, created_at
		FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.Status, &u.WorkerID, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateWorkerUserTx inserts a user linked to a worker within a transaction.
func (s *Store) CreateWorkerUserTx(ctx context.Context, tx pgx.Tx, orgID, workerID uuid.UUID, email, passwordHash string) (User, error) {
	var u User
	err := tx.QueryRow(ctx, `
		INSERT INTO users (org_id, email, password_hash, worker_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, email, status, worker_id, created_at`,
		orgID, email, passwordHash, workerID).
		Scan(&u.ID, &u.OrgID, &u.Email, &u.Status, &u.WorkerID, &u.CreatedAt)
	return u, err
}

// --- Roles & permissions ---

// GetRoleByName returns a role by name within an org.
func (s *Store) GetRoleByName(ctx context.Context, orgID uuid.UUID, name string) (Role, error) {
	var r Role
	err := s.pool.QueryRow(ctx, `
		SELECT id, org_id, name, description, is_system
		FROM roles WHERE org_id=$1 AND name=$2`, orgID, name).
		Scan(&r.ID, &r.OrgID, &r.Name, &r.Description, &r.IsSystem)
	if errors.Is(err, pgx.ErrNoRows) {
		return Role{}, ErrNotFound
	}
	return r, err
}

// CreateRoleTx inserts a role within a transaction.
func (s *Store) CreateRoleTx(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, name, desc string, isSystem bool) (Role, error) {
	var r Role
	err := tx.QueryRow(ctx, `
		INSERT INTO roles (org_id, name, description, is_system)
		VALUES ($1, $2, $3, $4)
		RETURNING id, org_id, name, description, is_system`,
		orgID, name, desc, isSystem).
		Scan(&r.ID, &r.OrgID, &r.Name, &r.Description, &r.IsSystem)
	return r, err
}

// GrantPermissionsTx attaches permission keys to a role within a transaction.
func (s *Store) GrantPermissionsTx(ctx context.Context, tx pgx.Tx, roleID uuid.UUID, keys []string) error {
	for _, k := range keys {
		if _, err := tx.Exec(ctx, `
			INSERT INTO role_permissions (role_id, permission_key) VALUES ($1, $2)
			ON CONFLICT DO NOTHING`, roleID, k); err != nil {
			return err
		}
	}
	return nil
}

// AssignRoleTx grants a role to a user within a transaction.
func (s *Store) AssignRoleTx(ctx context.Context, tx pgx.Tx, userID, roleID uuid.UUID) error {
	_, err := tx.Exec(ctx, `
		INSERT INTO user_roles (user_id, role_id) VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, userID, roleID)
	return err
}

// AllPermissionKeys returns every permission key in the global catalog.
func (s *Store) AllPermissionKeys(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT key FROM permissions ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// LoadPermissions returns the set of permission keys granted to a user via
// their roles. Implements auth.PermissionLoader.
func (s *Store) LoadPermissions(ctx context.Context, userID uuid.UUID) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT rp.permission_key
		FROM user_roles ur
		JOIN role_permissions rp ON rp.role_id = ur.role_id
		WHERE ur.user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	perms := map[string]bool{}
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		perms[k] = true
	}
	return perms, rows.Err()
}

// --- Refresh tokens ---

// StoreRefreshToken persists a hashed refresh token.
func (s *Store) StoreRefreshToken(ctx context.Context, userID uuid.UUID, hash string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO refresh_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)`, userID, hash, expiresAt)
	return err
}

// RefreshTokenRow is a stored refresh token.
type RefreshTokenRow struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	ExpiresAt time.Time
	RevokedAt *time.Time
}

// GetRefreshToken looks up a refresh token by its hash.
func (s *Store) GetRefreshToken(ctx context.Context, hash string) (RefreshTokenRow, error) {
	var r RefreshTokenRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, expires_at, revoked_at
		FROM refresh_tokens WHERE token_hash = $1`, hash).
		Scan(&r.ID, &r.UserID, &r.ExpiresAt, &r.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshTokenRow{}, ErrNotFound
	}
	return r, err
}

// RevokeRefreshToken marks a refresh token revoked by id.
func (s *Store) RevokeRefreshToken(ctx context.Context, id uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return err
}
