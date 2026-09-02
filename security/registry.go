package security

import (
	"context"
	"fmt"
)

// Registry dispatches a Credential to the Authenticator registered
// for its CredentialType. It is itself an Authenticator, so a
// SecurityManager can simply hold a single Registry rather than a
// list of authenticators.
//
// Registry lives in the core module (not a provider submodule) since
// it has no provider-specific dependencies -- it only depends on the
// Authenticator interface. Adding a new authenticator implementation
// (e.g. a digital-signature or mTLS provider, each as its own Go
// module under authenticator/<name>) only requires registering an
// instance of it here -- it never requires modifying Registry itself,
// the SecurityManager, or any transport adapter.
type Registry struct {
	authenticators map[CredentialType]Authenticator
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		authenticators: make(map[CredentialType]Authenticator),
	}
}

// Register adds an Authenticator to the registry, keyed by its
// declared Type(). Registering a second authenticator for the same
// type replaces the first.
func (r *Registry) Register(a Authenticator) {
	r.authenticators[a.Type()] = a
}

// Get returns the authenticator registered for the given credential
// type, if any.
func (r *Registry) Get(t CredentialType) (Authenticator, bool) {
	a, ok := r.authenticators[t]
	return a, ok
}

// Authenticate implements Authenticator by dispatching to the
// registered authenticator matching credential.Type.
func (r *Registry) Authenticate(ctx context.Context, credential Credential) (*AuthenticationResult, error) {
	a, ok := r.authenticators[credential.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCredential, credential.Type)
	}
	return a.Authenticate(ctx, credential)
}

// Type is unset for a Registry since it dispatches across multiple
// types; it satisfies the Authenticator interface for convenience but
// should not be registered into another Registry.
func (r *Registry) Type() CredentialType {
	return ""
}
