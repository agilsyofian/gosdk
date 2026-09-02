package security

import "errors"

// Standard security errors.
//
// Transport adapters are responsible for mapping these to the
// appropriate transport-level status (HTTP status codes, gRPC codes,
// etc). Business/application code should compare against these
// sentinel errors using errors.Is.
//
// Do not leak internal authentication details (e.g. "JWT signature
// validation failed at ...") to callers. Always return one of these
// sentinel errors (optionally wrapped) instead.
var (
	// ErrUnauthenticated indicates that no valid identity could be
	// established for the request.
	ErrUnauthenticated = errors.New("unauthenticated")

	// ErrForbidden indicates that the caller is authenticated but not
	// authorized to perform the requested action.
	ErrForbidden = errors.New("forbidden")

	// ErrInvalidCredential indicates that a credential was supplied but
	// failed validation (bad signature, expired, malformed, wrong password/key, etc).
	ErrInvalidCredential = errors.New("invalid credential")

	// ErrUnsupportedCredential indicates that no registered
	// Authenticator can handle the supplied credential type.
	ErrUnsupportedCredential = errors.New("unsupported credential")

	// ErrSessionNotFound indicates that a session ID was resolved (e.g.
	// from a JWT "sid" claim or a session cookie) but no corresponding
	// session exists in the SessionManager. This is what enables
	// server-side revocation: a structurally valid JWT is rejected the
	// moment its session is deleted.
	ErrSessionNotFound = errors.New("session not found")

	// ErrSessionExpired indicates that the resolved session exists but
	// has passed its ExpiresAt time.
	ErrSessionExpired = errors.New("session expired")

	// ErrConflictingCredential indicates that a request presented more
	// than one credential and the adapter could not unambiguously
	// determine which one should be used (e.g. both a session cookie
	// and a Bearer token were present). Adapters should fail closed
	// rather than silently choosing a winner.
	ErrConflictingCredential = errors.New("conflicting credential")

	// ErrNoCredential indicates that no credential could be extracted
	// from the request at all.
	ErrNoCredential = errors.New("no credential")
)
