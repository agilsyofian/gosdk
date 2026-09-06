package security

import (
	"context"
	"errors"
	"fmt"
)

// ============================================================================
// Registry
// ============================================================================

// Registry dispatches a Credential to the Authenticator registered for its CredentialType.
// It is itself an Authenticator, allowing SecurityManager to hold a single Registry.
type Registry struct {
	authenticators map[CredentialType]Authenticator
}

// NewRegistry constructs an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		authenticators: make(map[CredentialType]Authenticator),
	}
}

// Register adds an Authenticator to the registry, keyed by its declared Type().
func (r *Registry) Register(a Authenticator) {
	r.authenticators[a.Type()] = a
}

// Get returns the authenticator registered for the given credential type, if any.
func (r *Registry) Get(t CredentialType) (Authenticator, bool) {
	a, ok := r.authenticators[t]
	return a, ok
}

// Authenticate implements Authenticator by dispatching to the registered authenticator matching credential.Type.
func (r *Registry) Authenticate(ctx context.Context, credential Credential) (*AuthenticationResult, error) {
	a, ok := r.authenticators[credential.Type]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedCredential, credential.Type)
	}
	return a.Authenticate(ctx, credential)
}

// Type is unset for a Registry since it dispatches across multiple types.
func (r *Registry) Type() CredentialType {
	return ""
}

// ============================================================================
// SecurityManager
// ============================================================================

// SecurityManager is the single orchestrator transport adapters and login/logout handlers talk to.
type SecurityManager interface {
	// Authenticate validates a Credential and resolves the session if present.
	Authenticate(ctx context.Context, credential Credential) (*SecurityContext, error)

	// AuthenticateSession resolves a session directly by ID.
	AuthenticateSession(ctx context.Context, sessionID string) (*SecurityContext, error)

	// Authorize checks whether the SecurityContext in ctx is permitted to perform permission.
	Authorize(ctx context.Context, permission string) error
}

// ManagerConfig configures a Manager.
type ManagerConfig struct {
	Authenticator      Authenticator
	Sessions           SessionManager
	Authorizer         Authorizer
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

// Authorize implements SecurityManager.
func (m *Manager) Authorize(ctx context.Context, permission string) error {
	if m.authorizer == nil && m.authorizerProvider == nil {
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
