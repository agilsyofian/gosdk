package basicauth

import (
	"context"
	"encoding/base64"
	"testing"

	"github.com/agilsyofian/gosdk/security"
)

type staticLookup struct {
	username string
	password string
}

func (l *staticLookup) Lookup(ctx context.Context, username string) (string, *security.Principal, bool, error) {
	if username != l.username {
		return "", nil, false, nil
	}
	return l.password, &security.Principal{Subject: username, Username: username}, true, nil
}

func (l *staticLookup) Verify(ctx context.Context, username, secret, candidatePassword string) bool {
	return security.ConstantTimeEqual(secret, candidatePassword)
}

func basicHeader(user, pass string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + pass))
}

func TestAuthenticator_Success(t *testing.T) {
	auth := New(&staticLookup{username: "alice", password: "s3cret"})

	result, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBasic,
		Value: basicHeader("alice", "s3cret"),
	})
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}
	if result.Principal.Subject != "alice" {
		t.Fatalf("expected subject alice, got %s", result.Principal.Subject)
	}
}

func TestAuthenticator_WrongPassword(t *testing.T) {
	auth := New(&staticLookup{username: "alice", password: "s3cret"})

	_, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBasic,
		Value: basicHeader("alice", "wrong"),
	})
	if err == nil {
		t.Fatal("expected error for wrong password")
	}
}

func TestAuthenticator_UnknownUser(t *testing.T) {
	auth := New(&staticLookup{username: "alice", password: "s3cret"})

	_, err := auth.Authenticate(context.Background(), security.Credential{
		Type:  security.CredentialBasic,
		Value: basicHeader("bob", "whatever"),
	})
	if err == nil {
		t.Fatal("expected error for unknown user")
	}
}
