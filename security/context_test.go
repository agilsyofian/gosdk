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
