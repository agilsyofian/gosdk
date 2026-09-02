// Package authorization provides core.Authorizer implementations.
package authorization

import (
	"context"

	"github.com/agilsyofian/gosdk/security"
)

// PermissionAuthorizer is a straightforward Authorizer that checks
// the resolved Principal's Permissions (and, optionally, Roles via
// RoleMapping) against the requested permission.
type PermissionAuthorizer struct {
	// RolePermissions optionally maps a role to the set of
	// permissions it implies, e.g. {"admin": {"transaction:transfer"}}.
	// This lets callers grant permissions via role membership instead
	// of (or in addition to) listing permissions explicitly on the
	// Principal.
	RolePermissions map[string][]string
}

// NewPermissionAuthorizer constructs a PermissionAuthorizer.
func NewPermissionAuthorizer(rolePermissions map[string][]string) *PermissionAuthorizer {
	return &PermissionAuthorizer{RolePermissions: rolePermissions}
}

var _ security.Authorizer = (*PermissionAuthorizer)(nil)

// Authorize implements core.Authorizer.
//
// It fails closed: a missing/unauthenticated SecurityContext, or a
// SecurityContext with no resolvable Principal, is always denied.
func (a *PermissionAuthorizer) Authorize(ctx context.Context, securityContext *security.SecurityContext, permission string) error {
	if securityContext == nil || !securityContext.Authenticated {
		return security.ErrUnauthenticated
	}

	principal := securityContext.Principal
	if securityContext.Session != nil && securityContext.Session.Principal != nil {
		// The session is authoritative when present.
		principal = securityContext.Session.Principal
	}

	if principal == nil {
		return security.ErrUnauthenticated
	}

	if principal.HasPermission(permission) {
		return nil
	}

	for _, role := range principal.Roles {
		for _, perm := range a.RolePermissions[role] {
			if perm == permission {
				return nil
			}
		}
	}

	return security.ErrForbidden
}
