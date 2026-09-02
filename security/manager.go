package security

import (
	"context"
	"errors"
	"fmt"
)

// SecurityManager is the single orchestrator transport adapters and
// login/logout handlers talk to. It ties together Authenticator
// dispatch, session resolution, and authorization behind one
// transport-independent API.
//
// The same SecurityManager instance is meant to be shared by every
// transport adapter (Gin, gRPC, ...) so authentication/authorization
// logic is never duplicated per transport.
type SecurityManager interface {
	// Authenticate validates a Credential and, if the result carries a
	// SessionID, resolves the corresponding Session from the
	// SessionManager. Returns a populated SecurityContext on success.
	Authenticate(ctx context.Context, credential Credential) (*SecurityContext, error)

	// AuthenticateSession resolves a session directly by ID, e.g. from
	// a session cookie, without going through an Authenticator.
	AuthenticateSession(ctx context.Context, sessionID string) (*SecurityContext, error)

	// Authorize checks whether the SecurityContext already present in
	// ctx (see FromContext) is permitted to perform permission.
	Authorize(ctx context.Context, permission string) error
}

// ManagerConfig configures a Manager.
type ManagerConfig struct {
	// Authenticator dispatches credentials to the right underlying
	// Authenticator implementation. In practice this is typically an
	// *authenticator.Registry, but any core.Authenticator works.
	Authenticator Authenticator

	// Sessions resolves session IDs to Sessions. Required whenever any
	// registered Authenticator can return a SessionID (i.e. for the
	// recommended stateful flow).
	Sessions SessionManager

	// Authorizer performs permission checks. If nil, check AuthorizerProvider.
	Authorizer Authorizer

	// AuthorizerProvider performs permission checks with a custom provider. If nil, Authorize always
	//	// returns ErrForbidden for any non-empty permission (fail closed).
	AuthorizerProvider func(ctx context.Context, securityContext *SecurityContext, permission string) error
}

// Manager is the default SecurityManager implementation.
type Manager struct {
	authenticator      Authenticator
	sessions           SessionManager
	authorizer         Authorizer
	authorizerProvider func(ctx context.Context, securityContext *SecurityContext, permission string) error
}

// NewManager constructs a Manager from the given configuration.
func NewManager(cfg ManagerConfig) (SecurityManager, error) {
	if cfg.Authenticator == nil {
		return nil, errors.New("security: ManagerConfig.Authenticator is required")
	}
	return &Manager{
		authenticator:      cfg.Authenticator,
		sessions:           cfg.Sessions,
		authorizer:         cfg.Authorizer,
		authorizerProvider: cfg.AuthorizerProvider,
	}, nil
}

var _ SecurityManager = (*Manager)(nil)

// Authenticate implements SecurityManager.
//
// Flow:
//
//	Credential -> Authenticator.Authenticate() -> AuthenticationResult
//	  - SessionID present -> SessionManager.Get() -> Session -> SecurityContext
//	  - no SessionID       -> Principal -> SecurityContext
func (m *Manager) Authenticate(ctx context.Context, credential Credential) (*SecurityContext, error) {
	result, err := m.authenticator.Authenticate(ctx, credential)
	if err != nil {
		return nil, err
	}
	if result == nil || !result.Authenticated {
		return nil, ErrUnauthenticated
	}

	if result.SessionID != "" {
		return m.resolveSession(ctx, result.AuthType, result.SessionID)
	}

	if result.Principal == nil {
		return nil, fmt.Errorf("security: authenticator for %s returned no session and no principal: %w", result.AuthType, ErrUnauthenticated)
	}

	return &SecurityContext{
		Authenticated: true,
		AuthType:      result.AuthType,
		Principal:     result.Principal,
	}, nil
}

// AuthenticateSession implements SecurityManager.
func (m *Manager) AuthenticateSession(ctx context.Context, sessionID string) (*SecurityContext, error) {
	return m.resolveSession(ctx, CredentialSession, sessionID)
}

func (m *Manager) resolveSession(ctx context.Context, authType CredentialType, sessionID string) (*SecurityContext, error) {
	if m.sessions == nil {
		return nil, fmt.Errorf("security: session credential resolved but no SessionManager configured: %w", ErrUnauthenticated)
	}

	sess, err := m.sessions.Get(ctx, sessionID)
	if err != nil {
		// Covers both "JWT valid but session missing" and "session
		// expired": either way the caller is unauthenticated. This is
		// what provides server-side revocation on logout.
		if errors.Is(err, ErrSessionNotFound) || errors.Is(err, ErrSessionExpired) {
			return nil, ErrUnauthenticated
		}
		return nil, fmt.Errorf("security: resolve session: %w", err)
	}

	return &SecurityContext{
		Authenticated: true,
		AuthType:      authType,
		Principal:     sess.Principal,
		Session:       sess,
	}, nil
}

// Authorize implements SecurityManager. It reads the SecurityContext
// from ctx (it must have been placed there by a transport adapter via
// WithContext) and delegates to the configured Authorizer.
func (m *Manager) Authorize(ctx context.Context, permission string) error {

	if m.authorizer == nil && m.authorizerProvider == nil {
		// Fail closed: no authorizer configured means nothing is
		// authorized.
		return ErrForbidden
	}

	securityContext, ok := FromContext(ctx)
	if !ok || securityContext == nil || !securityContext.Authenticated {
		return ErrUnauthenticated
	}

	if m.authorizer != nil {
		return m.authorizer.Authorize(ctx, securityContext, permission)
	}

	return m.authorizerProvider(ctx, securityContext, permission)

}
