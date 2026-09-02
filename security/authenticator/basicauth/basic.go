// Package basicauth implements core.Authenticator for HTTP Basic
// Authentication, as an independent Go module.
package basicauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/agilsyofian/gosdk/security"
)

// Lookup resolves a username to the data needed to verify a password
// and build a Principal, without leaking any storage/hashing details
// into the authenticator itself.
type Lookup interface {
	// Lookup returns the expected password hash/secret and base
	// Principal for username. ok is false if the username is unknown;
	// callers must still perform a constant-time comparison against a
	// dummy value in that case to avoid username enumeration via
	// timing, which Authenticator handles internally.
	Lookup(ctx context.Context, username string) (secret string, principal *security.Principal, ok bool, err error)

	// Verify checks candidatePassword against the stored secret
	// (e.g. a bcrypt hash) for the given username. Implementations
	// MUST use a constant-time/timing-safe comparison appropriate to
	// their hashing scheme.
	Verify(ctx context.Context, username, secret, candidatePassword string) bool
}

// Authenticator validates HTTP Basic Authentication credentials.
type Authenticator struct {
	lookup Lookup
}

// New constructs a Basic Auth Authenticator backed by the given
// credential lookup.
func New(lookup Lookup) *Authenticator {
	return &Authenticator{lookup: lookup}
}

var _ security.Authenticator = (*Authenticator)(nil)

// Type implements core.Authenticator.
func (a *Authenticator) Type() security.CredentialType {
	return security.CredentialBasic
}

// Authenticate implements core.Authenticator. credential.Value is
// expected to be the base64 payload of an "Authorization: Basic
// <base64>" header (i.e. with the "Basic " prefix already stripped by
// the transport adapter).
func (a *Authenticator) Authenticate(ctx context.Context, credential security.Credential) (*security.AuthenticationResult, error) {
	if credential.Type != security.CredentialBasic {
		return nil, fmt.Errorf("%w: %s", security.ErrUnsupportedCredential, credential.Type)
	}
	if credential.Value == "" {
		return nil, security.ErrNoCredential
	}

	decoded, err := base64.StdEncoding.DecodeString(credential.Value)
	if err != nil {
		return nil, security.ErrInvalidCredential
	}

	username, password, found := strings.Cut(string(decoded), ":")
	if !found {
		return nil, security.ErrInvalidCredential
	}

	secret, principal, ok, err := a.lookup.Lookup(ctx, username)
	if err != nil {
		return nil, fmt.Errorf("basicauth: lookup: %w", err)
	}
	if !ok {
		// Still run Verify against a dummy secret so lookups for
		// unknown vs known-but-wrong-password usernames take a
		// similar code path/time.
		a.lookup.Verify(ctx, username, "", password)
		return nil, security.ErrInvalidCredential
	}

	if !a.lookup.Verify(ctx, username, secret, password) {
		return nil, security.ErrInvalidCredential
	}

	if principal == nil {
		principal = &security.Principal{Subject: username, Username: username}
	}

	return &security.AuthenticationResult{
		Authenticated: true,
		AuthType:      security.CredentialBasic,
		Principal:     principal,
	}, nil
}
