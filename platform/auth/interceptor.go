package auth

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	casbin "github.com/casbin/casbin/v3"
)

// NewUnaryInterceptor returns a Connect-RPC unary interceptor that:
//  1. Extracts the TenantContext populated by the HTTP middleware.
//  2. Enforces RBAC via Casbin (sub=userID, dom=orgID, obj=procedure, act=read|write).
//
// Unauthenticated procedures should be mounted without this interceptor.
func NewUnaryInterceptor(e *casbin.Enforcer) connect.UnaryInterceptorFunc {
	return connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
		return connect.UnaryFunc(func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			tc, err := FromContext(ctx)
			if err != nil {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated request"))
			}

			// RBAC enforcement: (sub=userID, dom=orgID, obj=procedure, act=read|write).
			if tc.Role != "admin" {
				procedure := req.Spec().Procedure
				action := actionFromProcedure(procedure)
				allowed, err := e.Enforce(tc.UserID, tc.OrgID, procedure, action)
				if err != nil {
					return nil, connect.NewError(connect.CodeInternal, errors.New("authorization check failed"))
				}
				if !allowed {
					return nil, connect.NewError(connect.CodePermissionDenied, errors.New("access denied"))
				}
			}

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
