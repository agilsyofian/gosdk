package security

import (
	"context"
)

// SecurityContext is the unified contract consumed by application
// code. It is transport-independent and is populated by the
// SecurityManager after authentication (and, where applicable, session
// resolution).
//
// Application/business code MUST consume security information through
// this type (retrieved via FromContext) rather than parsing JWTs, HTTP
// headers, cookies, or gRPC metadata directly.
type SecurityContext struct {
	// Authenticated indicates whether the current request has an
	// established identity.
	Authenticated bool

	// AuthType records which credential mechanism authenticated the
	// request.
	AuthType CredentialType

	// Principal is the authenticated identity. When Session is set,
	// this is the Principal attached to that Session (the
	// authoritative source); otherwise it is the Principal returned
	// directly by a stateless Authenticator.
	Principal *Principal

	// Session is the resolved server-side session, if the
	// authentication mechanism used one. Nil for stateless
	// authentication (e.g. a bare API key with no session).
	Session *Session
}

// contextKey is an unexported type to avoid collisions with context
// keys defined in other packages.
type contextKey struct{}

var securityContextKey = contextKey{}

// WithContext returns a new context.Context carrying the given
// SecurityContext.
func WithContext(ctx context.Context, securityContext *SecurityContext) context.Context {
	return context.WithValue(ctx, securityContextKey, securityContext)
}

// FromContext retrieves the SecurityContext previously stored via
// WithContext. The second return value reports whether one was found.
func FromContext(ctx context.Context) (*SecurityContext, bool) {
	sc, ok := ctx.Value(securityContextKey).(*SecurityContext)
	return sc, ok
}

// MustFromContext retrieves the SecurityContext previously stored via
// WithContext, or ErrUnauthenticated if none is present or the
// context is not authenticated.
func MustFromContext(ctx context.Context) (*SecurityContext, error) {
	sc, ok := FromContext(ctx)
	if !ok || sc == nil || !sc.Authenticated {
		return nil, ErrUnauthenticated
	}
	return sc, nil
}
