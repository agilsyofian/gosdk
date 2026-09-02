package authorization

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/security"
)

func TestPermissionAuthorizer_DirectPermission(t *testing.T) {
	authz := NewPermissionAuthorizer(nil)

	sc := &security.SecurityContext{
		Authenticated: true,
		Principal:     &security.Principal{Subject: "USR001", Permissions: []string{"transaction:transfer"}},
	}

	if err := authz.Authorize(context.Background(), sc, "transaction:transfer"); err != nil {
		t.Fatalf("expected allowed, got %v", err)
	}
	if err := authz.Authorize(context.Background(), sc, "transaction:delete"); err != security.ErrForbidden {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestPermissionAuthorizer_SessionIsAuthoritative(t *testing.T) {
	authz := NewPermissionAuthorizer(nil)

	// Principal directly on the SecurityContext has no permissions,
	// but the Session's Principal does -- session must win.
	sc := &security.SecurityContext{
		Authenticated: true,
		Principal:     &security.Principal{Subject: "USR001"},
		Session: &security.Session{
			Principal: &security.Principal{Subject: "USR001", Permissions: []string{"transaction:transfer"}},
		},
	}

	if err := authz.Authorize(context.Background(), sc, "transaction:transfer"); err != nil {
		t.Fatalf("expected allowed via session principal, got %v", err)
	}
}

func TestPermissionAuthorizer_RoleImpliedPermission(t *testing.T) {
	authz := NewPermissionAuthorizer(map[string][]string{
		"admin": {"transaction:transfer"},
	})

	sc := &security.SecurityContext{
		Authenticated: true,
		Principal:     &security.Principal{Subject: "USR001", Roles: []string{"admin"}},
	}

	if err := authz.Authorize(context.Background(), sc, "transaction:transfer"); err != nil {
		t.Fatalf("expected allowed via role mapping, got %v", err)
	}
}

func TestPermissionAuthorizer_Unauthenticated(t *testing.T) {
	authz := NewPermissionAuthorizer(nil)

	if err := authz.Authorize(context.Background(), nil, "any"); err != security.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated for nil context, got %v", err)
	}

	sc := &security.SecurityContext{Authenticated: false}
	if err := authz.Authorize(context.Background(), sc, "any"); err != security.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated for unauthenticated context, got %v", err)
	}
}
