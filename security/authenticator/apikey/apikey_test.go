package apikey

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/security"
)

type staticLookup struct {
	key       string
	principal *security.Principal
}

func (l *staticLookup) Lookup(ctx context.Context, apiKey string) (*security.Principal, bool, error) {
	if !security.ConstantTimeEqual(apiKey, l.key) {
		return nil, false, nil
	}
	return l.principal, true, nil
}

func TestAuthenticator_Success(t *testing.T) {
	auth := New(&staticLookup{
		key:       "valid-key",
		principal: &security.Principal{Subject: "service-a"},
	})

	result, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialAPIKey,
		Value: "valid-key",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Principal.Subject != "service-a" {
		t.Fatalf("expected subject service-a, got %s", result.Principal.Subject)
	}
}

func TestAuthenticator_Invalid(t *testing.T) {
	auth := New(&staticLookup{
		key:       "valid-key",
		principal: &security.Principal{Subject: "service-a"},
	})

	_, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialAPIKey,
		Value: "wrong-key",
	})
	if err == nil {
		t.Fatal("expected error for invalid api key")
	}
}

// TestWithCoreRegistry exercises this module's Authenticator through
// the core core.Registry, confirming it satisfies
// core.Authenticator and dispatches correctly alongside other
// (unregistered) credential types.
func TestWithCoreRegistry(t *testing.T) {
	reg := security.NewRegistry()
	reg.Register(New(&staticLookup{
		key:       "valid-key",
		principal: &security.Principal{Subject: "service-a"},
	}))

	_, err := reg.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBasic,
		Value: "irrelevant",
	})
	if err == nil {
		t.Fatal("expected ErrUnsupportedCredential for unregistered type")
	}

	result, err := reg.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialAPIKey,
		Value: "valid-key",
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if !result.Authenticated {
		t.Fatal("expected Authenticated = true")
	}
}
