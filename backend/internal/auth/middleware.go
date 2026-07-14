package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/gthimmes/we-people/backend/internal/httpx"
)

// PermissionLoader loads the set of permission keys granted to a user.
type PermissionLoader interface {
	LoadPermissions(ctx context.Context, userID uuid.UUID) (map[string]bool, error)
}

// Middleware authenticates requests and loads the caller's permissions.
type Middleware struct {
	tokens *TokenIssuer
	perms  PermissionLoader
}

// NewMiddleware builds an auth Middleware.
func NewMiddleware(tokens *TokenIssuer, perms PermissionLoader) *Middleware {
	return &Middleware{tokens: tokens, perms: perms}
}

// Authenticate verifies the Bearer access token and injects a Principal.
// Requests without a valid token are rejected with 401.
func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "missing bearer token")
			return
		}
		claims, err := m.tokens.ParseAccessToken(token)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid or expired token")
			return
		}
		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.Error(w, http.StatusUnauthorized, "unauthorized", "invalid token subject")
			return
		}
		perms, err := m.perms.LoadPermissions(r.Context(), userID)
		if err != nil {
			httpx.Error(w, http.StatusInternalServerError, "internal_error", "could not load permissions")
			return
		}
		p := &Principal{
			UserID:      userID,
			OrgID:       claims.OrgID,
			Email:       claims.Email,
			Permissions: perms,
		}
		next.ServeHTTP(w, r.WithContext(WithPrincipal(r.Context(), p)))
	})
}

// RequirePermission returns middleware that rejects callers lacking perm with 403.
func RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := PrincipalFrom(r.Context())
			if !p.Can(perm) {
				httpx.Error(w, http.StatusForbidden, "forbidden", "missing required permission: "+perm)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) > len(prefix) && strings.EqualFold(h[:len(prefix)], prefix) {
		return h[len(prefix):]
	}
	return ""
}
