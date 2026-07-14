package auth

import (
	"fmt"
	"sync"

	casbin "github.com/casbin/casbin/v3"
)

// Roles recognised inside an organization.
//
// These are the values stored on tenancy.memberships.role and emitted in the
// WorkOS access token's `role` claim. They are the subjects of Casbin `p`
// rules: a user is bound to a role by a `g` rule scoped to their org.
const (
	// RoleOwner is the property owner. Full control, including the booking
	// engine and integration API keys.
	RoleOwner = "owner"
	// RoleAdmin has the same operational reach as an owner today. Kept
	// distinct so ownership transfer and billing can diverge later.
	RoleAdmin = "admin"
	// RoleMember is read-only. Front-desk and reporting staff.
	RoleMember = "member"
)

// permission is one (object, action) pair a role is granted.
//
// object is a Connect-RPC procedure pattern matched with Casbin's keyMatch, so
// "/*" covers every procedure and "/pms.v1.PmsService/*" covers one service.
// action is "read" or "write", derived from the procedure name by
// actionFromProcedure.
type permission struct {
	object string
	action string
}

// rolePermissions is the authoritative role -> permission mapping.
//
// Deliberately coarse. Objects are procedure patterns rather than a bespoke
// resource vocabulary, because that is what the interceptor passes to Enforce.
// Narrow a role by adding a more specific pattern, not by inventing a resource
// name that nothing enforces.
var rolePermissions = map[string][]permission{
	RoleOwner: {
		{object: "/*", action: "read"},
		{object: "/*", action: "write"},
		{object: bookingEngineObject, action: "read"},
		{object: bookingEngineObject, action: "write"},
	},
	RoleAdmin: {
		{object: "/*", action: "read"},
		{object: "/*", action: "write"},
		{object: bookingEngineObject, action: "read"},
		{object: bookingEngineObject, action: "write"},
	},
	RoleMember: {
		{object: "/*", action: "read"},
		{object: bookingEngineObject, action: "read"},
	},
}

// bookingEngineObject is the Casbin object pattern for the BookingEngineService
// Connect-RPC procedures: the reads (ListDirectReservations, GetSettings) and
// the one write (UpdateSettings — the direct-channel toggle and routing).
//
// These rules are the booking engine's explicit policy. They currently MIRROR
// the coarse "/*" default — owner/admin read+write, member read — and are
// redundant while that default stands: Casbin is allow-based, so the "/*" grant
// alone already admits these procedures. They exist because the direct channel
// is the first surface with its own stated policy, so its intent (staff may
// view direct bookings; only owner/admin may toggle the channel) is legible
// here rather than implied by a catch-all.
//
// To genuinely restrict this surface — e.g. make the toggle owner-only — the
// "/*" write grants must be replaced by explicit per-service writes; narrowing
// here alone would not, because "/*" would still allow it. That is a broader
// change than this service and is deliberately not done now.
const bookingEngineObject = "/bookingengine.v1.BookingEngineService/*"

// KnownRole reports whether role is one this platform grants permissions to.
func KnownRole(role string) bool {
	_, ok := rolePermissions[role]
	return ok
}

// RoleBinder materialises role permissions into a SyncedEnforcer.
//
// Binding happens on the request path, so it must be cheap and safe. Two
// mechanisms provide that:
//
//   - The enforcer is a SyncedEnforcer, so its policy model is guarded.
//   - A per-binder memo records which (user, role, org) triples have already
//     been bound in this process, so the steady state is one sync.Map load and
//     no enforcer write at all. Without it every request would take the
//     enforcer's write lock merely to discover the rules already exist.
//
// The memo is keyed on role as well as user and org, so a role change misses
// the cache and re-binds — which is what makes demotion take effect.
type RoleBinder struct {
	enforcer *casbin.SyncedEnforcer
	bound    sync.Map // key: userID\x00role\x00orgID -> struct{}
}

// NewRoleBinder returns a binder over e. A nil enforcer yields a binder whose
// Ensure is a no-op, which keeps call sites free of nil checks.
func NewRoleBinder(e *casbin.SyncedEnforcer) *RoleBinder {
	return &RoleBinder{enforcer: e}
}

func bindingKey(userID, role, orgID string) string {
	return userID + "\x00" + role + "\x00" + orgID
}

// Ensure makes a user's role effective inside one organization.
//
// It writes, idempotently:
//   - the `p` rules for the role within the org's domain, and
//   - the `g` rule binding the user to that role in that domain.
//
// Policies are per-domain: an org's rules are inserted under its own id and the
// RLS policy on tenancy.casbin_rule confines them there. Two orgs granting
// "owner" full access do not share a row.
//
// Role bindings are exclusive within a domain: any binding to a different role
// in this org is removed first. Without that, demoting an owner to member would
// leave the owner binding in place and the demotion would silently not apply —
// `g` rules are additive and Casbin allows a subject many roles.
//
// An unrecognised role is demoted to RoleMember rather than granted nothing, so
// a typo in a WorkOS role claim degrades to read-only instead of locking a user
// out of an org they legitimately belong to.
func (b *RoleBinder) Ensure(userID, role, orgID string) error {
	if b == nil || b.enforcer == nil || userID == "" || orgID == "" {
		return nil
	}

	if !KnownRole(role) {
		role = RoleMember
	}

	key := bindingKey(userID, role, orgID)
	if _, ok := b.bound.Load(key); ok {
		return nil
	}

	for _, p := range rolePermissions[role] {
		if _, err := b.enforcer.AddPolicy(role, orgID, p.object, p.action); err != nil {
			return fmt.Errorf("auth: add policy %s/%s: %w", role, p.action, err)
		}
	}

	// Drop stale bindings so a demotion actually demotes. Removing a rule that
	// is not present is a no-op returning false, not an error.
	for other := range rolePermissions {
		if other == role {
			continue
		}
		if _, err := b.enforcer.RemoveGroupingPolicy(userID, other, orgID); err != nil {
			return fmt.Errorf("auth: unbind %s from role %s: %w", userID, other, err)
		}
		// A prior role's memo entry must not survive, or switching back to it
		// would skip re-binding after we have just removed its `g` rule.
		b.bound.Delete(bindingKey(userID, other, orgID))
	}

	if _, err := b.enforcer.AddGroupingPolicy(userID, role, orgID); err != nil {
		return fmt.Errorf("auth: bind %s to role %s: %w", userID, role, err)
	}

	b.bound.Store(key, struct{}{})
	return nil
}
