// Package apikey implements core.Authenticator for API key
// credentials, as an independent Go module.
package apikey

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/security"
)

// Lookup resolves an API key to a Principal.
type Lookup interface {
	// Lookup returns the Principal associated with apiKey. ok is
	// false if the key is unknown or revoked. Implementations MUST
	// compare the key using a constant-time comparison (see
	// core.ConstantTimeEqual) rather than a plain map lookup keyed
	// by the raw secret if the comparison itself is security-sensitive;
	// a map lookup by hashed key is an acceptable and common pattern.
	Lookup(ctx context.Context, apiKey string) (principal *security.Principal, ok bool, err error)
}

// Authenticator validates API key credentials, typically extracted
// from an X-API-Key header.
//
// This is a stateless authentication mechanism: no session is
// created, and the resolved Principal is used directly.
type Authenticator struct {
	lookup Lookup
}

// New constructs an API Key Authenticator backed by the given lookup.
func New(lookup Lookup) *Authenticator {
	return &Authenticator{lookup: lookup}
}

var _ security.Authenticator = (*Authenticator)(nil)

// Type implements core.Authenticator.
func (a *Authenticator) Type() security.CredentialType {
	return security.CredentialAPIKey
}

// Authenticate implements core.Authenticator.
func (a *Authenticator) Authenticate(ctx context.Context, credential security.Credential) (*security.AuthenticationResult, error) {
	if credential.Type != security.CredentialAPIKey {
		return nil, fmt.Errorf("%w: %s", security.ErrUnsupportedCredential, credential.Type)
	}
	if credential.Value == "" {
		return nil, fmt.Errorf("%w", security.ErrNoCredential)
	}

	principal, ok, err := a.lookup.Lookup(ctx, credential.Value)
	if err != nil {
		return nil, fmt.Errorf("apikey: lookup: %w", err)
	}
	if !ok || principal == nil {
		return nil, security.ErrInvalidCredential
	}

	return &security.AuthenticationResult{
		Authenticated: true,
		AuthType:      security.CredentialAPIKey,
		Principal:     principal,
	}, nil
}
