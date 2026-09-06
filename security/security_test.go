package security_test

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/security"
)

func TestWithContextFromContext(t *testing.T) {
	sc := &security.SecurityContext{
		Authenticated: true,
		AuthType:      security.CredentialAPIKey,
		Principal:     &security.Principal{Subject: "USR001"},
	}

	ctx := security.WithContext(context.Background(), sc)

	got, ok := security.FromContext(ctx)
	if !ok {
		t.Fatal("expected SecurityContext to be found")
	}
	if got.Principal.Subject != "USR001" {
		t.Fatalf("expected subject USR001, got %s", got.Principal.Subject)
	}
}

func TestFromContext_Missing(t *testing.T) {
	_, ok := security.FromContext(context.Background())
	if ok {
		t.Fatal("expected no SecurityContext to be found in bare context")
	}
}

func TestMustFromContext_Unauthenticated(t *testing.T) {
	_, err := security.MustFromContext(context.Background())
	if err != security.ErrUnauthenticated {
		t.Fatalf("expected ErrUnauthenticated, got %v", err)
	}
}

func TestPrincipal_ClaimsAndPermissions(t *testing.T) {
	principal := &security.Principal{
		Subject:     "usr-42",
		Username:    "alice",
		Roles:       []string{"admin"},
		Permissions: []string{"read", "write"},
		Claims:      map[string]any{"tenant_id": "tenant-1"},
	}

	if !principal.HasRole("admin") {
		t.Error("expected principal to have role admin")
	}
	if principal.HasRole("superadmin") {
		t.Error("did not expect principal to have role superadmin")
	}
	if !principal.HasPermission("read") {
		t.Error("expected principal to have permission read")
	}
	if principal.HasPermission("delete") {
		t.Error("did not expect principal to have permission delete")
	}

	val, ok := principal.Claim("tenant_id")
	if !ok || val != "tenant-1" {
		t.Fatalf("expected tenant-1, got %v", val)
	}
}

func TestConstantTimeEqual_And_NewSessionID(t *testing.T) {
	if !security.ConstantTimeEqual("secret123", "secret123") {
		t.Error("expected equal strings to match")
	}
	if security.ConstantTimeEqual("secret123", "secret456") {
		t.Error("expected different strings not to match")
	}

	sessID, err := security.NewSessionID()
	if err != nil {
		t.Fatalf("unexpected error creating session ID: %v", err)
	}
	if len(sessID) == 0 {
		t.Error("expected non-empty session ID")
	}
}
