package auth

import (
	"context"
	"errors"
)

// TenantContext holds the authenticated user's tenant information.
type TenantContext struct {
	UserID string
	OrgID  string
	Role   string
}

type contextKey struct{}

// ErrNoTenantContext is returned when no tenant context is found in the context.
var ErrNoTenantContext = errors.New("no tenant context in context")

// WithTenantContext returns a new context with the given TenantContext attached.
func WithTenantContext(ctx context.Context, tc TenantContext) context.Context {
	return context.WithValue(ctx, contextKey{}, tc)
}

// FromContext extracts the TenantContext from the given context.
func FromContext(ctx context.Context) (TenantContext, error) {
	tc, ok := ctx.Value(contextKey{}).(TenantContext)
	if !ok {
		return TenantContext{}, ErrNoTenantContext
	}
	return tc, nil
}
