package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	casbin "github.com/casbin/casbin/v3"
	"github.com/jackc/pgx/v5"
)

// NewUnaryInterceptor returns a Connect-RPC unary interceptor that:
//  1. Verifies the Bearer JWT and extracts claims.
//  2. Resolves the WorkOS org_id to the local UUID.
//  3. Enforces RBAC via Casbin (sub=userID, dom=orgID, obj=procedure, act=read|write).
//  4. Attaches a TenantContext to the request context for downstream handlers.
//
// Unauthenticated procedures should be mounted without this interceptor.
func NewUnaryInterceptor(v *Verifier, s *Store, e *casbin.Enforcer) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			claims, err := v.Verify(ctx, req.Header().Get("Authorization"))
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or missing token"))
			}

			if claims.OrganizationID == "" {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("organization context required"))
			}

			localOrgID, err := s.ResolveOrgID(ctx, claims.OrganizationID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, connect.NewError(connect.CodePermissionDenied, errors.New("organization not registered"))
				}
				return nil, connect.NewError(connect.CodeInternal, errors.New("identity resolution failed"))
			}

			// RBAC enforcement: (sub=userID, dom=orgID, obj=procedure, act=read|write).
			procedure := req.Spec().Procedure
			action := actionFromProcedure(procedure)
			allowed, err := e.Enforce(claims.Subject(), localOrgID, procedure, action)
			if err != nil {
				return nil, connect.NewError(connect.CodeInternal, errors.New("authorization check failed"))
			}
			if !allowed {
				return nil, connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
			}

			ctx = WithTenantContext(ctx, TenantContext{
				UserID: claims.Subject(),
				OrgID:  localOrgID,
				Role:   claims.Role,
			})
			return next(ctx, req)
		})
	})
}

// actionFromProcedure maps a Connect-RPC procedure path to a Casbin action.
// Procedures starting with Get or List are "read"; everything else is "write".
// Example: "/inventory.v1.InventoryService/GetInventory" → "read"
func actionFromProcedure(procedure string) string {
	// procedure is "/package.Service/Method" — extract the method name.
	if idx := strings.LastIndex(procedure, "/"); idx >= 0 {
		method := procedure[idx+1:]
		if strings.HasPrefix(method, "Get") || strings.HasPrefix(method, "List") {
			return "read"
		}
	}
	return "write"
}
