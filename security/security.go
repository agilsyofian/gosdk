package security

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"time"
)

// ============================================================================
// Sentinel Errors
// ============================================================================

// Standard security errors.
//
// Transport adapters are responsible for mapping these to the
// appropriate transport-level status (HTTP status codes, gRPC codes, etc).
// Business/application code should compare against these sentinel errors using errors.Is.
var (
	// ErrUnauthenticated indicates that no valid identity could be established for the request.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrForbidden indicates that the caller is authenticated but not authorized to perform the requested action.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidCredential indicates that a credential was supplied but failed validation
	// (bad signature, expired, malformed, wrong password/key, etc).
	ErrInvalidCredential = errors.New("invalid credential")

	// ErrUnsupportedCredential indicates that no registered Authenticator can handle the supplied credential type.
	ErrUnsupportedCredential = errors.New("unsupported credential")

	// ErrSessionNotFound indicates that a session ID was resolved (e.g. from a JWT "sid" claim or a session cookie)
	// but no corresponding session exists in the SessionManager.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired indicates that the resolved session exists but has passed its ExpiresAt time.
	ErrSessionExpired = errors.New("session expired")

	// ErrConflictingCredential indicates that a request presented more than one conflicting credential.
	ErrConflictingCredential = errors.New("conflicting credential")

	// ErrNoCredential indicates that no credential could be extracted from the request at all.
	ErrNoCredential = errors.New("no credential")
)

// ============================================================================
// Credentials
// ============================================================================

// CredentialType identifies the kind of authentication credential presented by the caller.
type CredentialType string

const (
	CredentialBearer  CredentialType = "bearer"
	CredentialBasic   CredentialType = "basic"
	CredentialAPIKey  CredentialType = "api_key"
	CredentialHMAC    CredentialType = "hmac"
	CredentialMTLS    CredentialType = "mtls"
	CredentialOAuth   CredentialType = "oauth"
	CredentialSession CredentialType = "session"
)

// Credential is a transport-independent representation of whatever the caller presented to authenticate.
type Credential struct {
	// Type identifies which Authenticator should handle this credential.
	Type CredentialType

	// Value holds the raw credential material (JWT string, base64 payload, API key, session ID, or signature).
	Value string

	// Identifier optionally carries a secondary value (e.g. API key ID or HMAC key ID).
	Identifier string
}

// ============================================================================
// Principal & Identity
// ============================================================================

// Principal represents an authenticated identity as established at the moment of authentication.
type Principal struct {
	// Subject is the stable identifier of the authenticated entity, e.g. a user ID.
	Subject string

	// Username is a human-readable identifier, if applicable.
	Username string

	// Roles lists the roles assigned to the principal.
	Roles []string

	// Permissions lists the fine-grained permissions assigned to the principal, e.g. "transaction:transfer".
	Permissions []string

	// Claims holds any additional authentication-related data.
	Claims map[string]any
}

// HasRole reports whether the principal has the given role.
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// HasPermission reports whether the principal has the given permission.
func (p *Principal) HasPermission(permission string) bool {
	if p == nil {
		return false
	}
	for _, perm := range p.Permissions {
		if perm == permission {
			return true
		}
	}
	return false
}

// Claim retrieves an additional authentication claim by name.
func (p *Principal) Claim(name string) (any, bool) {
	if p == nil || p.Claims == nil {
		return nil, false
	}
	value, ok := p.Claims[name]
	return value, ok
}

// ============================================================================
// Session
// ============================================================================

// Session represents server-side, stateful user context.
type Session struct {
	// ID is the unique session identifier (the "sid").
	ID string

	// Principal is the identity this session belongs to.
	Principal *Principal

	CreatedAt time.Time
	ExpiresAt time.Time

	// Attributes holds arbitrary, business-defined session data.
	Attributes map[string]any
}

// Expired reports whether the session has passed its ExpiresAt time as of now.
func (s *Session) Expired() bool {
	if s == nil {
		return true
	}
	return time.Now().After(s.ExpiresAt)
}

// Attribute retrieves a session attribute by key.
func (s *Session) Attribute(key string) (any, bool) {
	if s == nil || s.Attributes == nil {
		return nil, false
	}
	v, ok := s.Attributes[key]
	return v, ok
}

// SessionManager is specifically responsible for stateful, server-side sessions.
type SessionManager interface {
	// Create establishes a new session for the given principal with the given time-to-live.
	Create(ctx context.Context, sessionId string, principal *Principal, ttl time.Duration) (*Session, error)

	// Get retrieves a session by ID.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Delete removes a session, immediately invalidating it.
	Delete(ctx context.Context, sessionID string) error

	// Refresh extends a session's lifetime by the given TTL and returns the updated session.
	Refresh(ctx context.Context, sessionID string, ttl time.Duration) (*Session, error)
}

// ============================================================================
// Authentication & Authorization Interfaces
// ============================================================================

// AuthenticationResult is what an Authenticator returns after validating a Credential.
type AuthenticationResult struct {
	Authenticated bool
	AuthType      CredentialType
	SessionID     string
	Principal     *Principal
}

// Authenticator validates a single Credential type and produces an AuthenticationResult.
type Authenticator interface {
	// Type returns the CredentialType this authenticator handles.
	Type() CredentialType

	// Authenticate validates the credential and returns the result.
	Authenticate(ctx context.Context, credential Credential) (*AuthenticationResult, error)
}

// Authorizer decides whether the identity carried by a SecurityContext is allowed to perform an action.
type Authorizer interface {
	Authorize(ctx context.Context, securityContext *SecurityContext, permission string) error
}

// ============================================================================
// SecurityContext
// ============================================================================

// SecurityContext is the unified contract consumed by application code.
type SecurityContext struct {
	Authenticated bool
	AuthType      CredentialType
	Principal     *Principal
	Session       *Session
}

type contextKey struct{}

var securityContextKey = contextKey{}

// WithContext returns a new context.Context carrying the given SecurityContext.
func WithContext(ctx context.Context, securityContext *SecurityContext) context.Context {
	return context.WithValue(ctx, securityContextKey, securityContext)
}

// FromContext retrieves the SecurityContext previously stored via WithContext.
func FromContext(ctx context.Context) (*SecurityContext, bool) {
	sc, ok := ctx.Value(securityContextKey).(*SecurityContext)
	return sc, ok
}

// MustFromContext retrieves the SecurityContext previously stored via WithContext, or ErrUnauthenticated.
func MustFromContext(ctx context.Context) (*SecurityContext, error) {
	sc, ok := FromContext(ctx)
	if !ok || sc == nil || !sc.Authenticated {
		return nil, ErrUnauthenticated
	}
	return sc, nil
}

// ============================================================================
// Cryptographic Helpers
// ============================================================================

// ConstantTimeEqual performs a constant-time comparison of two secrets (e.g. static API keys, HMAC digests).
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		subtle.ConstantTimeCompare([]byte(a), []byte(a))
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

// NewSessionID generates a cryptographically random, URL-safe session ID.
func NewSessionID() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
