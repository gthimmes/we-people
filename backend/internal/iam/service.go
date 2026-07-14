package iam

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/auth"
	"github.com/gthimmes/we-people/backend/internal/org"
)

// Common service errors.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserDisabled       = errors.New("user is disabled")
	ErrTokenInvalid       = errors.New("refresh token invalid or expired")
	ErrSlugTaken          = errors.New("organization slug already taken")
)

// Service implements registration and authentication.
type Service struct {
	store  *Store
	orgs   *org.Store
	tokens *auth.TokenIssuer
}

// NewService builds the IAM service.
func NewService(store *Store, orgs *org.Store, tokens *auth.TokenIssuer) *Service {
	return &Service{store: store, orgs: orgs, tokens: tokens}
}

// TokenPair is the result of a successful authentication.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// RegisterResult is returned when a new organization is created.
type RegisterResult struct {
	Org   org.Organization `json:"organization"`
	User  User             `json:"user"`
	Token TokenPair        `json:"token"`
}

// Register creates a new organization, its first admin user, and the default
// system roles, all in one transaction, then issues tokens for the admin.
func (s *Service) Register(ctx context.Context, orgName, email, password string) (RegisterResult, error) {
	slug := Slugify(orgName)
	if slug == "" {
		return RegisterResult{}, fmt.Errorf("organization name produces empty slug")
	}
	passwordHash, err := auth.HashPassword(password)
	if err != nil {
		return RegisterResult{}, err
	}
	allPerms, err := s.store.AllPermissionKeys(ctx)
	if err != nil {
		return RegisterResult{}, err
	}

	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return RegisterResult{}, err
	}
	defer tx.Rollback(ctx)

	organization, err := s.orgs.CreateTx(ctx, tx, orgName, slug)
	if err != nil {
		if isUniqueViolation(err) {
			return RegisterResult{}, ErrSlugTaken
		}
		return RegisterResult{}, fmt.Errorf("create org: %w", err)
	}

	user, err := s.store.CreateUserTx(ctx, tx, organization.ID, email, passwordHash)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("create user: %w", err)
	}

	adminRole, err := s.store.CreateRoleTx(ctx, tx, organization.ID, "Org Admin", "Full administrative access", true)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("create admin role: %w", err)
	}
	if err := s.store.GrantPermissionsTx(ctx, tx, adminRole.ID, allPerms); err != nil {
		return RegisterResult{}, fmt.Errorf("grant admin perms: %w", err)
	}
	employeeRole, err := s.store.CreateRoleTx(ctx, tx, organization.ID, "Employee", "Standard employee self-service", true)
	if err != nil {
		return RegisterResult{}, fmt.Errorf("create employee role: %w", err)
	}
	if err := s.store.GrantPermissionsTx(ctx, tx, employeeRole.ID, []string{"worker:read", "orgstructure:read"}); err != nil {
		return RegisterResult{}, fmt.Errorf("grant employee perms: %w", err)
	}
	if err := s.store.AssignRoleTx(ctx, tx, user.ID, adminRole.ID); err != nil {
		return RegisterResult{}, fmt.Errorf("assign admin role: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return RegisterResult{}, err
	}

	pair, err := s.issueTokens(ctx, user)
	if err != nil {
		return RegisterResult{}, err
	}
	return RegisterResult{Org: organization, User: user, Token: pair}, nil
}

// CreateWorkerUser creates a login linked to a worker and assigns a role by
// name. Used by seeding and (later) onboarding.
func (s *Service) CreateWorkerUser(ctx context.Context, orgID, workerID uuid.UUID, email, password, roleName string) (User, error) {
	role, err := s.store.GetRoleByName(ctx, orgID, roleName)
	if err != nil {
		return User{}, fmt.Errorf("role %q: %w", roleName, err)
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return User{}, err
	}
	tx, err := s.store.Pool().Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback(ctx)

	user, err := s.store.CreateWorkerUserTx(ctx, tx, orgID, workerID, email, hash)
	if err != nil {
		return User{}, err
	}
	if err := s.store.AssignRoleTx(ctx, tx, user.ID, role.ID); err != nil {
		return User{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return user, nil
}

// Login authenticates a user by org slug + email + password.
func (s *Service) Login(ctx context.Context, slug, email, password string) (TokenPair, User, error) {
	lr, err := s.store.GetUserForLogin(ctx, slug, email)
	if errors.Is(err, ErrNotFound) {
		return TokenPair{}, User{}, ErrInvalidCredentials
	}
	if err != nil {
		return TokenPair{}, User{}, err
	}
	if !auth.CheckPassword(lr.PasswordHash, password) {
		return TokenPair{}, User{}, ErrInvalidCredentials
	}
	if lr.User.Status != "active" {
		return TokenPair{}, User{}, ErrUserDisabled
	}
	pair, err := s.issueTokens(ctx, lr.User)
	if err != nil {
		return TokenPair{}, User{}, err
	}
	return pair, lr.User, nil
}

// Refresh rotates a refresh token: the old token is revoked and a new token
// pair issued. Reusing a revoked or expired token fails.
func (s *Service) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	hash := auth.HashRefreshToken(refreshToken)
	row, err := s.store.GetRefreshToken(ctx, hash)
	if errors.Is(err, ErrNotFound) {
		return TokenPair{}, ErrTokenInvalid
	}
	if err != nil {
		return TokenPair{}, err
	}
	if row.RevokedAt != nil || time.Now().After(row.ExpiresAt) {
		return TokenPair{}, ErrTokenInvalid
	}
	if err := s.store.RevokeRefreshToken(ctx, row.ID); err != nil {
		return TokenPair{}, err
	}
	user, err := s.store.GetUserByID(ctx, row.UserID)
	if err != nil {
		return TokenPair{}, err
	}
	return s.issueTokens(ctx, user)
}

// Logout revokes a refresh token.
func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	row, err := s.store.GetRefreshToken(ctx, auth.HashRefreshToken(refreshToken))
	if errors.Is(err, ErrNotFound) {
		return nil // already gone; treat as success
	}
	if err != nil {
		return err
	}
	return s.store.RevokeRefreshToken(ctx, row.ID)
}

func (s *Service) issueTokens(ctx context.Context, u User) (TokenPair, error) {
	access, err := s.tokens.IssueAccessToken(u.ID, u.OrgID, u.Email)
	if err != nil {
		return TokenPair{}, err
	}
	refresh, hash, err := auth.GenerateRefreshToken()
	if err != nil {
		return TokenPair{}, err
	}
	expiresAt := time.Now().Add(s.tokens.RefreshTTL())
	if err := s.store.StoreRefreshToken(ctx, u.ID, hash, expiresAt); err != nil {
		return TokenPair{}, err
	}
	return TokenPair{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresAt:    time.Now().Add(s.tokens.AccessTTL()),
	}, nil
}

var slugInvalid = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts an organization name to a URL-safe slug.
func Slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugInvalid.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(err.Error(), "SQLSTATE 23505")
}
