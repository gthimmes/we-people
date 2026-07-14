package auth

import (
	"context"

	"github.com/google/uuid"
)

// Principal is the authenticated caller for a request.
type Principal struct {
	UserID      uuid.UUID
	OrgID       uuid.UUID
	Email       string
	Permissions map[string]bool
}

// Can reports whether the principal holds the given permission.
func (p *Principal) Can(perm string) bool {
	return p != nil && p.Permissions[perm]
}

type principalKey struct{}

// WithPrincipal returns a copy of ctx carrying the principal.
func WithPrincipal(ctx context.Context, p *Principal) context.Context {
	return context.WithValue(ctx, principalKey{}, p)
}

// PrincipalFrom returns the principal stored in ctx, or nil.
func PrincipalFrom(ctx context.Context) *Principal {
	p, _ := ctx.Value(principalKey{}).(*Principal)
	return p
}
