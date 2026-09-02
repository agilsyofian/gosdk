package security

import (
	"context"
)

// AuthenticationResult is what an Authenticator returns after
// validating a Credential. It intentionally stops short of being a
// SecurityContext: authenticators prove identity (and, where
// applicable, resolve a session ID from the credential), but they do
// not construct the final SecurityContext themselves. Session
// resolution against the SessionManager is the SecurityManager's job.
type AuthenticationResult struct {
	// Authenticated indicates whether the credential was successfully
	// validated. An Authenticator should return a non-nil error rather
	// than Authenticated = false for invalid credentials; this field
	// exists mainly for completeness and for authenticators that may
	// legitimately return a "soft" unauthenticated result.
	Authenticated bool

	// AuthType records which credential type produced this result.
	AuthType CredentialType

	// SessionID is set when the credential resolves to a server-side
	// session (e.g. a JWT's "sid" claim, or a session cookie value).
	// When set, the SecurityManager will resolve the full Session via
	// the SessionManager and treat it as authoritative.
	//
	// Typical stateful JWT result:
	//
	//	Authenticated = true
	//	AuthType      = bearer
	//	SessionID     = "abc123"
	//	Principal     = nil
	SessionID string

	// Principal is set for authentication mechanisms that are
	// genuinely stateless and do not use a session, e.g.:
	//
	//	Authenticated = true
	//	AuthType      = api_key
	//	SessionID     = ""
	//	Principal     = resolved principal
	//
	// For the primary FE -> BE flow in this SDK, prefer using a
	// session (set SessionID) over returning a Principal directly.
	Principal *Principal
}

// Authenticator validates a single Credential type and produces an
// AuthenticationResult.
//
// Implementations MUST NOT depend on Gin, net/http, gRPC, or any other
// transport-specific package -- they receive an already-extracted,
// transport-independent Credential.
type Authenticator interface {
	// Type returns the CredentialType this authenticator handles.
	Type() CredentialType

	// Authenticate validates the credential and returns the result.
	// It should return one of the sentinel errors in errors.go (e.g.
	// ErrInvalidCredential) rather than leaking internal validation
	// details.
	Authenticate(ctx context.Context, credential Credential) (*AuthenticationResult, error)
}
