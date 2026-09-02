package crypto

import (
	"context"
	"sync"
	"time"
)

// ReplayProtector validates request-level freshness and uniqueness for
// the field-encryption protocol, exactly as specified in
// encryption-steerfile.md §16/§17. It is deliberately NOT a dependency
// of FieldDecryptor: "Replay protection MUST be implemented outside the
// low-level Decrypt() function" (§16). Wire it into HTTP middleware
// instead, calling Validate with the X-Nonce/X-Timestamp header values
// before (or after — order is a middleware decision) calling
// FieldDecryptor.DecryptFields.
//
// nonce is the raw nonce string as carried in the X-Nonce header
// (Base64URL-encoded, per §11) — callers pass it through unchanged
// rather than decoding it first, since the store only needs it as an
// opaque uniqueness key. timestamp is Unix seconds, as carried in
// X-Timestamp.
type ReplayProtector interface {
	Validate(ctx context.Context, nonce string, timestamp int64) error
}

// NonceWindowPolicy configures an InMemoryNonceStore. Zero value uses
// DefaultNonceWindowPolicy.
type NonceWindowPolicy struct {
	// ClockSkew is how far timestamp may lag or lead the server clock
	// and still be accepted. §16's example is symmetric (server_time
	// ± 5 minutes), so this one field drives both directions.
	ClockSkew time.Duration
	// NonceTTL is how long a nonce must be remembered to reject
	// duplicates. §16's Redis example uses `EX 300` (5 minutes),
	// matching the recommended clock-skew window.
	NonceTTL time.Duration
}

// DefaultNonceWindowPolicy matches encryption-steerfile.md §16's
// recommended default: a 5-minute timestamp window, and a nonce
// remembered for the same 5 minutes (mirroring the `SET <nonce>
// <marker> NX EX 300` example).
var DefaultNonceWindowPolicy = NonceWindowPolicy{
	ClockSkew: 5 * time.Minute,
	NonceTTL:  5 * time.Minute,
}

func (p NonceWindowPolicy) withDefaults() NonceWindowPolicy {
	if p.ClockSkew <= 0 {
		p.ClockSkew = DefaultNonceWindowPolicy.ClockSkew
	}
	if p.NonceTTL <= 0 {
		p.NonceTTL = DefaultNonceWindowPolicy.NonceTTL
	}
	return p
}

// InMemoryNonceStore is a reference ReplayProtector for tests and local
// development ONLY. A process-local map only protects a single backend
// instance; production deployments MUST back this with a shared store
// (Redis `SET <nonce> <marker> NX EX <ttl>`, per §16) so replay
// detection holds across every instance behind the load balancer.
type InMemoryNonceStore struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	policy  NonceWindowPolicy
	nowFunc func() time.Time
}

// NewInMemoryNonceStore builds an InMemoryNonceStore. Passing a zero
// NonceWindowPolicy uses DefaultNonceWindowPolicy.
func NewInMemoryNonceStore(policy NonceWindowPolicy) *InMemoryNonceStore {
	return &InMemoryNonceStore{
		seen:    make(map[string]time.Time),
		policy:  policy.withDefaults(),
		nowFunc: time.Now,
	}
}

// Validate implements ReplayProtector: it checks timestamp freshness,
// then atomically checks-and-stores nonce, in that order. Both checks
// use the same generic-message *Error convention as the rest of this
// package (§22: no cryptographic internals leaked to callers).
func (s *InMemoryNonceStore) Validate(_ context.Context, nonce string, timestamp int64) error {
	if nonce == "" {
		return newError(ErrCodeInvalidNonce, "empty nonce")
	}
	now := s.nowFunc()
	ts := time.Unix(timestamp, 0)
	skew := s.policy.ClockSkew
	if ts.Before(now.Add(-skew)) || ts.After(now.Add(skew)) {
		return newError(ErrCodeExpiredRequest, "timestamp %d outside +/-%s window of now=%d", timestamp, skew, now.Unix())
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.evictLocked(now)
	if expiry, ok := s.seen[nonce]; ok && expiry.After(now) {
		return newError(ErrCodeReplayedRequest, "nonce already processed")
	}
	s.seen[nonce] = now.Add(s.policy.NonceTTL)
	return nil
}

// evictLocked drops expired entries. Caller must hold s.mu.
func (s *InMemoryNonceStore) evictLocked(now time.Time) {
	for id, expiry := range s.seen {
		if !expiry.After(now) {
			delete(s.seen, id)
		}
	}
}
