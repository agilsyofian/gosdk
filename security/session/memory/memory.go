// Package memory implements an in-process core.SessionManager,
// as an independent Go module. It exists ONLY for unit tests and
// local development -- see the Manager doc comment.
package memory

import (
	"context"
	"sync"
	"time"

	"github.com/agilsyofian/gosdk/security"
)

// Manager is an in-process, unbounded SessionManager implementation.
//
// It exists ONLY for unit tests and local development. Per the
// project's security rules, an unbounded in-memory map MUST NOT be
// used as the production session store -- use session/redis (or an
// equivalent externally-backed, TTL-enforcing provider module)
// instead. This type performs no eviction beyond lazy expiry checks
// on read, so it will grow without bound under real traffic.
type Manager struct {
	mu          sync.RWMutex
	sessions    map[string]*security.Session
	IDGenerator func() (string, error)
}

// New constructs a Manager. Test/dev use only -- see the type doc
// comment.
func New() *Manager {
	return &Manager{
		sessions:    make(map[string]*security.Session),
		IDGenerator: security.NewSessionID,
	}
}

var _ security.SessionManager = (*Manager)(nil)

func (m *Manager) Create(ctx context.Context, sessId string, principal *security.Principal, ttl time.Duration) (*security.Session, error) {
	now := time.Now()
	sess := &security.Session{
		ID:         sessId,
		Principal:  principal,
		CreatedAt:  now,
		ExpiresAt:  now.Add(ttl),
		Attributes: map[string]any{},
	}

	m.mu.Lock()
	m.sessions[sessId] = sess
	m.mu.Unlock()

	return sess, nil
}

func (m *Manager) Get(ctx context.Context, sessionID string) (*security.Session, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, security.ErrSessionNotFound
	}
	if sess.Expired() {
		return nil, security.ErrSessionExpired
	}
	return sess, nil
}

func (m *Manager) Delete(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, sessionID)
	return nil
}

func (m *Manager) Refresh(ctx context.Context, sessionID string, ttl time.Duration) (*security.Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return nil, security.ErrSessionNotFound
	}
	sess.ExpiresAt = time.Now().Add(ttl)
	return sess, nil
}
