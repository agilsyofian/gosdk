package idempotency

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrNotFound is returned when an idempotency key is not found in storage.
	ErrNotFound = errors.New("idempotency: key not found")
)

// Storage defines the storage contract for idempotency caching.
// It matches 1-to-1 with github.com/agilsyofian/gosdk/storage/cache.Cache.
type Storage interface {
	Get(ctx context.Context, key string) ([]byte, error)
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Delete(ctx context.Context, key string) error
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time
}

// MemoryStorage is an in-memory thread-safe implementation of Storage.
type MemoryStorage struct {
	mu    sync.RWMutex
	items map[string]memoryItem
}

// NewMemoryStorage creates a new in-memory Storage instance.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{
		items: make(map[string]memoryItem),
	}
}

// Get retrieves a value from memory storage.
func (m *MemoryStorage) Get(ctx context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	item, exists := m.items[key]
	if !exists {
		return nil, ErrNotFound
	}

	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		return nil, ErrNotFound
	}

	return item.value, nil
}

// Set stores a value in memory storage with an expiration duration.
func (m *MemoryStorage) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var expiresAt time.Time
	if expiration > 0 {
		expiresAt = time.Now().Add(expiration)
	}

	// Make a defensive copy of the slice
	valCopy := make([]byte, len(value))
	copy(valCopy, value)

	m.items[key] = memoryItem{
		value:     valCopy,
		expiresAt: expiresAt,
	}

	return nil
}

// Delete removes a key from memory storage.
func (m *MemoryStorage) Delete(ctx context.Context, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.items, key)
	return nil
}
