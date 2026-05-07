package auth

import "github.com/golang-jwt/jwt/v5"

// AccessTokenClaims models the subset of a WorkOS AuthKit access-token
// payload that the platform consumes. The shape matches the JWT contract
// documented in the WorkOS user-management reference: a registered-claim
// envelope plus org/role/permission attributes injected by AuthKit (and
// any additional claims defined in the dashboard's JWT template).
//
// We deliberately keep this struct narrow. Any custom claim a service
// wants to act on must be added here so the type acts as the authoritative
// projection of the token across the codebase.
type AccessTokenClaims struct {
	// SessionID identifies the AuthKit session (sid). Used for log
	// correlation and for revoking sessions via the WorkOS API.
	SessionID string `json:"sid,omitempty"`
	// OrganizationID is the WorkOS organization the user signed in to.
	// May be empty before an organization is selected; downstream code
	// must reject empty values for tenant-scoped operations.
	OrganizationID string `json:"org_id,omitempty"`
	// Role is the user's role inside OrganizationID. Sourced from the
	// JWT template; absent unless the template is configured to emit it.
	Role string `json:"role,omitempty"`
	// Permissions is the optional flat permission list emitted by the
	// JWT template. The Casbin enforcer is the source of truth for
	// authorization; this is exposed for diagnostics only.
	Permissions []string `json:"permissions,omitempty"`

	jwt.RegisteredClaims
}

// Subject returns the user id (sub) of the token bearer.
func (c AccessTokenClaims) Subject() string { return c.RegisteredClaims.Subject }
