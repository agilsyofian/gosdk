package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	coreErr "github.com/agilsyofian/gosdk/errors"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/storage/cache"
)

// ErrNotFound is the error Client implementations should return from
// Get when the key does not exist.
var ErrNotFound = errors.New("redis: key not found")

// KeyPrefix is prepended to every session ID to form the Redis key,
// i.e. "session:{sessionID}".
const KeyPrefix = "session:"

// record is the JSON-serializable form of core.Session stored in Redis
type record struct {
	ID         string              `json:"id"`
	Principal  *security.Principal `json:"principal"`
	CreatedAt  time.Time           `json:"created_at"`
	ExpiresAt  time.Time           `json:"expires_at"`
	Attributes map[string]any      `json:"attributes"`
}

// Manager is the recommended production implementation of
// core.SessionManager, backed by Redis (or any store satisfying
// Client).
type Manager struct {
	client cache.Cache

	// IDGenerator produces new session IDs. Defaults to
	// core.NewSessionID if not set.
	IDGenerator func() (string, error)
}

// New constructs a Manager backed by the given client.
func New(client cache.Cache) security.SessionManager {
	return &Manager{
		client:      client,
		IDGenerator: security.NewSessionID,
	}
}

var _ security.SessionManager = (*Manager)(nil)

func (m *Manager) key(sessionID string) string {
	return KeyPrefix + sessionID
}

// Create establishes a new session for the given principal and stores
// it in Redis with the given TTL.
func (m *Manager) Create(ctx context.Context, sessId string, principal *security.Principal, ttl time.Duration) (*security.Session, error) {
	now := time.Now()
	sess := &security.Session{
		ID:         sessId,
		Principal:  principal,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		Attributes: map[string]any{},
	}

	if err := m.write(ctx, sess, ttl); err != nil {
		return nil, err
	}
	return sess, nil
}

// Get retrieves a session from Redis by ID.
func (m *Manager) Get(ctx context.Context, sessionID string) (*security.Session, error) {
	if sessionID == "" {
		return nil, coreErr.Wrap(security.ErrSessionNotFound)
	}

	raw, err := m.client.Get(ctx, m.key(sessionID))
	if errors.Is(err, ErrNotFound) || raw == nil {
		return nil, coreErr.Wrap(security.ErrSessionNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("redis: get: %w", err)
	}

	var rec record
	if err := json.Unmarshal([]byte(raw), &rec); err != nil {
		return nil, fmt.Errorf("redis: decode: %w", err)
	}

	sess := &security.Session{
		ID:         rec.ID,
		Principal:  rec.Principal,
		CreatedAt:  rec.CreatedAt,
		ExpiresAt:  rec.ExpiresAt,
		Attributes: rec.Attributes,
	}

	if sess.Expired() {
		return nil, security.ErrSessionExpired
	}

	return sess, nil
}

// Delete removes a session from Redis, immediately invalidating it.
func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}
	if err := m.client.DeleteByPrefix(ctx, m.key(sessionID)); err != nil {
		return fmt.Errorf("redis: delete: %w", err)
	}
	return nil
}

// Refresh extends a session's TTL in Redis.
func (m *Manager) Refresh(ctx context.Context, sessionID string, ttl time.Duration) (*security.Session, error) {
	sess, err := m.Get(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	sess.ExpiresAt = time.Now().Add(ttl)

	if err := m.write(ctx, sess, ttl); err != nil {
		return nil, err
	}
	return sess, nil
}

func (m *Manager) write(ctx context.Context, sess *security.Session, ttl time.Duration) error {
	rec := record{
		ID:         sess.ID,
		Principal:  sess.Principal,
		CreatedAt:  sess.CreatedAt,
		ExpiresAt:  sess.ExpiresAt,
		Attributes: sess.Attributes,
	}

	data, err := json.Marshal(rec)
	if err != nil {
		return fmt.Errorf("redis: encode: %w", err)
	}

	if err := m.client.Set(ctx, m.key(sess.ID), data, ttl); err != nil {
		return fmt.Errorf("redis: set: %w", err)
	}
	return nil
}
