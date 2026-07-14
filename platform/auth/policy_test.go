package auth

import (
	"fmt"
	"sync"
	"testing"

	casbin "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
)

const (
	orgA = "aaaaaaaa-0000-0000-0000-000000000000"
	orgB = "bbbbbbbb-0000-0000-0000-000000000000"
)

const (
	readProc  = "/inventory.v1.InventoryService/GetInventory"
	writeProc = "/inventory.v1.InventoryService/SetInventory"
)

// newTestBinder builds a SyncedEnforcer over the real embedded model with an
// in-memory policy store, plus a binder over it. These tests therefore exercise
// the same matcher and the same concurrency primitives production uses, without
// touching Postgres.
func newTestBinder(t *testing.T) (*casbin.SyncedEnforcer, *RoleBinder) {
	t.Helper()
	m, err := model.NewModelFromString(casbinModelText)
	if err != nil {
		t.Fatalf("parse model: %v", err)
	}
	e, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		t.Fatalf("new synced enforcer: %v", err)
	}
	return e, NewRoleBinder(e)
}

func TestEnsure_OwnerAndAdminMayWrite(t *testing.T) {
	for _, role := range []string{RoleOwner, RoleAdmin} {
		t.Run(role, func(t *testing.T) {
			e, b := newTestBinder(t)
			if err := b.Ensure("user-1", role, orgA); err != nil {
				t.Fatalf("ensure: %v", err)
			}

			for _, tc := range []struct{ proc, act string }{
				{readProc, "read"},
				{writeProc, "write"},
			} {
				allowed, err := e.Enforce("user-1", orgA, tc.proc, tc.act)
				if err != nil {
					t.Fatalf("enforce: %v", err)
				}
				if !allowed {
					t.Errorf("%s should be allowed %s on %s", role, tc.act, tc.proc)
				}
			}
		})
	}
}

// A member reads but never writes. This is the front-desk analogue.
func TestEnsure_MemberIsReadOnly(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-2", RoleMember, orgA); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if allowed, _ := e.Enforce("user-2", orgA, readProc, "read"); !allowed {
		t.Error("member should be allowed to read")
	}
	if allowed, _ := e.Enforce("user-2", orgA, writeProc, "write"); allowed {
		t.Error("member must not be allowed to write")
	}
}

// Policy is per-domain: an owner of one org has no rights in another. This is
// the property that keeps tenants apart at the RBAC layer, above RLS.
func TestEnsure_RolesDoNotLeakAcrossOrgs(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if allowed, _ := e.Enforce("user-1", orgB, readProc, "read"); allowed {
		t.Error("an owner of org A must have no rights in org B")
	}
}

// The same user may hold different roles in different orgs.
func TestEnsure_PerOrgRoles(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
		t.Fatalf("ensure orgA: %v", err)
	}
	if err := b.Ensure("user-1", RoleMember, orgB); err != nil {
		t.Fatalf("ensure orgB: %v", err)
	}

	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); !allowed {
		t.Error("owner in org A should write")
	}
	if allowed, _ := e.Enforce("user-1", orgB, writeProc, "write"); allowed {
		t.Error("member in org B must not write")
	}
}

// An unknown role degrades to read-only rather than locking the user out or,
// worse, granting more than intended.
func TestEnsure_UnknownRoleDegradesToMember(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-3", "superuser", orgA); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	if allowed, _ := e.Enforce("user-3", orgA, readProc, "read"); !allowed {
		t.Error("unknown role should still read")
	}
	if allowed, _ := e.Enforce("user-3", orgA, writeProc, "write"); allowed {
		t.Error("unknown role must not write")
	}
}

// Without the interceptor's old admin bypass, an empty policy set fails closed.
func TestEnforce_UnboundUserIsDenied(t *testing.T) {
	e, _ := newTestBinder(t)
	if allowed, _ := e.Enforce("nobody", orgA, readProc, "read"); allowed {
		t.Error("a user with no role binding must be denied")
	}
}

// Calling repeatedly must not error, and must leave the decision unchanged.
// The middleware calls this on every authenticated request.
func TestEnsure_Idempotent(t *testing.T) {
	e, b := newTestBinder(t)
	for i := 0; i < 3; i++ {
		if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
			t.Fatalf("ensure #%d: %v", i, err)
		}
		allowed, err := e.Enforce("user-1", orgA, writeProc, "write")
		if err != nil {
			t.Fatalf("enforce #%d: %v", i, err)
		}
		if !allowed {
			t.Fatalf("owner denied on call #%d", i)
		}
	}
}

func TestEnsure_PromotionTakesEffect(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-1", RoleMember, orgA); err != nil {
		t.Fatalf("ensure member: %v", err)
	}
	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); allowed {
		t.Fatal("member should not write")
	}

	if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
		t.Fatalf("ensure owner: %v", err)
	}
	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); !allowed {
		t.Error("promoted owner should write")
	}
}

// Demotion must actually revoke. `g` rules are additive, so without removing
// the stale binding an ex-owner would keep writing forever.
func TestEnsure_DemotionRevokesWrite(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
		t.Fatalf("ensure owner: %v", err)
	}
	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); !allowed {
		t.Fatal("owner should write")
	}

	if err := b.Ensure("user-1", RoleMember, orgA); err != nil {
		t.Fatalf("demote to member: %v", err)
	}

	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); allowed {
		t.Error("demoted user must no longer write")
	}
	if allowed, _ := e.Enforce("user-1", orgA, readProc, "read"); !allowed {
		t.Error("demoted user should still read")
	}
}

// Demoting in one org must not disturb the user's role in another.
func TestEnsure_DemotionIsScopedToOneOrg(t *testing.T) {
	e, b := newTestBinder(t)
	if err := b.Ensure("user-1", RoleOwner, orgA); err != nil {
		t.Fatalf("ensure orgA owner: %v", err)
	}
	if err := b.Ensure("user-1", RoleOwner, orgB); err != nil {
		t.Fatalf("ensure orgB owner: %v", err)
	}

	if err := b.Ensure("user-1", RoleMember, orgA); err != nil {
		t.Fatalf("demote in orgA: %v", err)
	}

	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); allowed {
		t.Error("demoted in org A, must not write there")
	}
	if allowed, _ := e.Enforce("user-1", orgB, writeProc, "write"); !allowed {
		t.Error("org B ownership must be untouched")
	}
}

// Re-promoting after a demotion must work. The memo entry for the old role is
// deleted when its `g` rule is removed, so this does not hit a stale cache.
func TestEnsure_DemoteThenRepromote(t *testing.T) {
	e, b := newTestBinder(t)
	for _, role := range []string{RoleOwner, RoleMember, RoleOwner} {
		if err := b.Ensure("user-1", role, orgA); err != nil {
			t.Fatalf("ensure %s: %v", role, err)
		}
	}
	if allowed, _ := e.Enforce("user-1", orgA, writeProc, "write"); !allowed {
		t.Error("re-promoted owner should write again")
	}
}

// Guards against a nil enforcer or empty identifiers, which the middleware may
// hand over before an org is selected.
func TestEnsure_NoopOnMissingInputs(t *testing.T) {
	if err := NewRoleBinder(nil).Ensure("user", RoleOwner, orgA); err != nil {
		t.Errorf("nil enforcer should no-op, got %v", err)
	}

	var nilBinder *RoleBinder
	if err := nilBinder.Ensure("user", RoleOwner, orgA); err != nil {
		t.Errorf("nil binder should no-op, got %v", err)
	}

	_, b := newTestBinder(t)
	if err := b.Ensure("", RoleOwner, orgA); err != nil {
		t.Errorf("empty user should no-op, got %v", err)
	}
	if err := b.Ensure("user", RoleOwner, ""); err != nil {
		t.Errorf("empty org should no-op, got %v", err)
	}
}

func TestKnownRole(t *testing.T) {
	for _, role := range []string{RoleOwner, RoleAdmin, RoleMember} {
		if !KnownRole(role) {
			t.Errorf("%q should be a known role", role)
		}
	}
	if KnownRole("front_desk") {
		t.Error("front_desk is PMS vocabulary, not a CM role")
	}
}

// NormalizeRole guards the tenancy.memberships CHECK constraint: an arbitrary
// WorkOS slug must never reach the insert.
func TestNormalizeRole(t *testing.T) {
	cases := []struct{ slug, want string }{
		{RoleOwner, RoleOwner},
		{RoleAdmin, RoleAdmin},
		{RoleMember, RoleMember},
		{"", RoleMember},
		{"superuser", RoleMember},
		// 'viewer' satisfies the DB CHECK but has no entry in rolePermissions,
		// so it degrades to member rather than becoming an unenforceable role.
		{"viewer", RoleMember},
		{"OWNER", RoleMember}, // slugs are case-sensitive
	}
	for _, tc := range cases {
		if got := NormalizeRole(tc.slug); got != tc.want {
			t.Errorf("NormalizeRole(%q) = %q, want %q", tc.slug, got, tc.want)
		}
	}
}

// Reproduces the production access pattern: many request goroutines binding
// roles while others enforce. Run with -race; this fails on a plain
// casbin.Enforcer and is the reason NewEnforcer returns a SyncedEnforcer.
func TestRoleBinder_ConcurrentEnsureAndEnforce(t *testing.T) {
	e, b := newTestBinder(t)

	const goroutines = 32
	const iterations = 20

	var wg sync.WaitGroup
	errCh := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			// Spread across orgs and users so the memo misses often and the
			// binder actually writes, rather than short-circuiting.
			user := fmt.Sprintf("user-%d", g%4)
			org := orgA
			if g%2 == 0 {
				org = orgB
			}
			for i := 0; i < iterations; i++ {
				if err := b.Ensure(user, RoleOwner, org); err != nil {
					errCh <- err
					return
				}
				if _, err := e.Enforce(user, org, readProc, "read"); err != nil {
					errCh <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent access failed: %v", err)
	}

	if allowed, _ := e.Enforce("user-0", orgB, writeProc, "write"); !allowed {
		t.Error("owner binding should have survived concurrent access")
	}
}
