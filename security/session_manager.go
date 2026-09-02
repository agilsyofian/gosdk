package security

import (
	"context"
	"time"
)

// SessionManager is specifically responsible for stateful,
// server-side sessions. It is the single source of truth consulted by
// the SecurityManager whenever an AuthenticationResult carries a
// SessionID.
//
// Implementations MUST use their backing store's own expiration
// mechanism (e.g. Redis TTL) rather than relying solely on the
// ExpiresAt field stored inside the serialized Session -- a session
// record must not outlive its intended lifetime just because nothing
// is actively checking ExpiresAt.
type SessionManager interface {
	// Create establishes a new session for the given principal with
	// the given time-to-live and returns it.
	Create(ctx context.Context, sessionId string, principal *Principal, ttl time.Duration) (*Session, error)

	// Get retrieves a session by ID. It returns ErrSessionNotFound if
	// no session exists for the given ID (including if it has expired
	// and been evicted by the backing store), and ErrSessionExpired if
	// the record exists but ExpiresAt has already passed.
	Get(ctx context.Context, sessionID string) (*Session, error)

	// Delete removes a session, immediately invalidating it. This is
	// what powers logout and server-side revocation: once deleted, a
	// structurally valid JWT referencing this session ID will no
	// longer resolve, and callers should treat the request as
	// unauthenticated.
	Delete(ctx context.Context, sessionID string) error

	// Refresh extends a session's lifetime by the given TTL and
	// returns the updated session. It returns ErrSessionNotFound if
	// the session does not exist.
	Refresh(ctx context.Context, sessionID string, ttl time.Duration) (*Session, error)
}
