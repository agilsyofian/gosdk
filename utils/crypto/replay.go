package crypto

import (
	"context"
	"sync"
	"time"
)

// ReplayStore tracks which request IDs have already been processed
// within the replay window, per STEERFILE.md §10. Production
// implementations SHOULD back this with a shared store (Redis, etc.) so
// replay detection works across multiple backend instances; a
// process-local store only protects a single instance.
type ReplayStore interface {
	// CheckAndStore atomically records requestID as seen and reports
	// whether it was already present. Implementations MUST make this
	// atomic (e.g. Redis SETNX) to avoid a race where two concurrent
	// requests with the same request_id both observe "not seen".
	// ttl is how long the ID must be remembered.
	CheckAndStore(ctx context.Context, requestID string, ttl time.Duration) (alreadySeen bool, err error)
}

// InMemoryReplayStore is a reference ReplayStore for tests and local
// development. It is NOT suitable for a multi-instance production
// deployment — use a shared store (e.g. Redis) there, per STEERFILE.md §10.
type InMemoryReplayStore struct {
	mu      sync.Mutex
	seen    map[string]time.Time
	nowFunc func() time.Time
}

// NewInMemoryReplayStore builds an empty InMemoryReplayStore.
func NewInMemoryReplayStore() *InMemoryReplayStore {
	return &InMemoryReplayStore{
		seen:    make(map[string]time.Time),
		nowFunc: time.Now,
	}
}

func (s *InMemoryReplayStore) CheckAndStore(_ context.Context, requestID string, ttl time.Duration) (bool, error) {
	now := s.nowFunc()
	s.mu.Lock()
	defer s.mu.Unlock()

	s.evictLocked(now)

	if expiry, ok := s.seen[requestID]; ok && expiry.After(now) {
		return true, nil
	}
	s.seen[requestID] = now.Add(ttl)
	return false, nil
}

// evictLocked drops expired entries. Caller must hold s.mu.
func (s *InMemoryReplayStore) evictLocked(now time.Time) {
	for id, expiry := range s.seen {
		if !expiry.After(now) {
			delete(s.seen, id)
		}
	}
}

// ReplayPolicy configures freshness and replay validation, per
// STEERFILE.md §10. Zero-value ReplayPolicy uses DefaultReplayPolicy.
type ReplayPolicy struct {
	// MaxClockSkew is how far a timestamp may lag or lead the backend
	// clock and still be accepted.
	MaxClockSkew time.Duration
	// Window is how long a request_id must be remembered to reject
	// duplicates; SHOULD be >= 2*MaxClockSkew so a request can't be
	// replayed just after it ages out of memory but is still "fresh"
	// under skew rules.
	Window time.Duration
}

// DefaultReplayPolicy is a conservative default suitable for
// state-changing operations (payments, transfers, OTP, password change,
// account recovery, permission changes) called out in STEERFILE.md §10.
var DefaultReplayPolicy = ReplayPolicy{
	MaxClockSkew: 2 * time.Minute,
	Window:       10 * time.Minute,
}

func (p ReplayPolicy) withDefaults() ReplayPolicy {
	if p.MaxClockSkew <= 0 {
		p.MaxClockSkew = DefaultReplayPolicy.MaxClockSkew
	}
	if p.Window <= 0 {
		p.Window = DefaultReplayPolicy.Window
	}
	return p
}

func (p ReplayPolicy) validateTimestamp(ts int64, now time.Time) error {
	t := time.Unix(ts, 0)
	skew := p.withDefaults().MaxClockSkew
	if t.Before(now.Add(-skew)) || t.After(now.Add(skew)) {
		return newError(ErrCodeStaleTimestamp, "timestamp %d outside +/-%s window of now=%d", ts, skew, now.Unix())
	}
	return nil
}
