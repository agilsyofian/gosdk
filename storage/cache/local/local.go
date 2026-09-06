package local

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/cache"
	"go.uber.org/fx"
)

// Ensure Cache implements cache.Cache interface
var _ cache.Cache = (*Cache)(nil)

// Ensure Cache implements health.Check interface
var _ health.Check = (*Cache)(nil)

type item struct {
	value     []byte
	expiresAt time.Time
}

func (i item) isExpired(now time.Time) bool {
	if i.expiresAt.IsZero() {
		return false
	}
	return now.After(i.expiresAt)
}

// Cache is an in-memory Cache implementation.
type Cache struct {
	mu        sync.RWMutex
	items     map[string]item
	hook      *Hook
	logger    logger.Logger
	appName   string
	stopCh    chan struct{}
	closeOnce sync.Once
}

// NewLocalCache creates a new instance of LocalCache.
// If health.Registry is provided, it automatically registers a readiness check for LocalCache.
func NewLocalCache(
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*Cache, error) {
	ctx := context.Background()
	appName := coreContext.AppName(ctx)

	c := &Cache{
		items:   make(map[string]item),
		hook:    NewHook(l, cfg, appName),
		logger:  l,
		appName: appName,
		stopCh:  make(chan struct{}),
	}

	for _, hr := range hrs {
		if hr != nil {
			hr.RegisterReadiness("local_cache", c)
			break
		}
	}

	if cfg.CleanupInterval > 0 {
		go c.startJanitor(cfg.CleanupInterval)
	}

	if lc != nil {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				if l != nil {
					l.Info(ctx, "local cache initialized", logger.LogPayload{
						Type:   "STORAGE_LOCAL",
						Method: "START",
					})
				}
				return nil
			},
			OnStop: func(ctx context.Context) error {
				if l != nil {
					l.Info(ctx, "stopping local cache", logger.LogPayload{
						Type:   "STORAGE_LOCAL",
						Method: "SHUTDOWN",
					})
				}
				return c.Close(ctx)
			},
		})
	}

	return c, nil
}

func (c *Cache) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case now := <-ticker.C:
			c.deleteExpired(now)
		}
	}
}

func (c *Cache) deleteExpired(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for k, it := range c.items {
		if it.isExpired(now) {
			delete(c.items, k)
		}
	}
}

// Get retrieves a value from the local cache.
// Returns nil, nil on cache miss or when expired.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	var result []byte
	err := c.hook.Execute(ctx, "get", key, func() error {
		c.mu.RLock()
		it, found := c.items[key]
		c.mu.RUnlock()

		if !found {
			return nil
		}

		now := time.Now()
		if it.isExpired(now) {
			c.mu.Lock()
			if cur, ok := c.items[key]; ok && cur.isExpired(now) {
				delete(c.items, key)
			}
			c.mu.Unlock()
			return nil
		}

		result = make([]byte, len(it.value))
		copy(result, it.value)
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("local get: %w", err)
	}
	return result, nil
}

// Set stores a value in the local cache with an optional expiration.
func (c *Cache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	err := c.hook.Execute(ctx, "set", key, func() error {
		valCopy := make([]byte, len(value))
		copy(valCopy, value)

		var expiresAt time.Time
		if expiration > 0 {
			expiresAt = time.Now().Add(expiration)
		}

		c.mu.Lock()
		c.items[key] = item{
			value:     valCopy,
			expiresAt: expiresAt,
		}
		c.mu.Unlock()
		return nil
	})

	if err != nil {
		return fmt.Errorf("local set: %w", err)
	}
	return nil
}

// SetNX stores a value in the local cache if the key does not exist or has expired.
// Returns true if successfully stored, false if key already exists.
func (c *Cache) SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error) {
	var stored bool
	err := c.hook.Execute(ctx, "setnx", key, func() error {
		valBytes, err := toBytes(value)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		now := time.Now()
		var expiresAt time.Time
		if expiration > 0 {
			expiresAt = now.Add(expiration)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		if it, exists := c.items[key]; exists && !it.isExpired(now) {
			stored = false
			return nil
		}

		c.items[key] = item{
			value:     valBytes,
			expiresAt: expiresAt,
		}
		stored = true
		return nil
	})

	if err != nil {
		return false, fmt.Errorf("local setnx: %w", err)
	}
	return stored, nil
}

// Incr increments the integer value of a key in the local cache.
// If the key does not exist or has expired, it is initialized to 1.
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	var newVal int64
	err := c.hook.Execute(ctx, "incr", key, func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		now := time.Now()
		it, exists := c.items[key]
		if !exists || it.isExpired(now) {
			newVal = 1
			c.items[key] = item{
				value: []byte("1"),
			}
			return nil
		}

		current, err := strconv.ParseInt(string(it.value), 10, 64)
		if err != nil {
			return fmt.Errorf("value is not an integer: %w", err)
		}

		newVal = current + 1
		c.items[key] = item{
			value:     []byte(strconv.FormatInt(newVal, 10)),
			expiresAt: it.expiresAt,
		}
		return nil
	})

	if err != nil {
		return 0, fmt.Errorf("local incr: %w", err)
	}
	return newVal, nil
}

// Delete removes a value from the local cache.
func (c *Cache) Delete(ctx context.Context, key string) error {
	err := c.hook.Execute(ctx, "delete", key, func() error {
		c.mu.Lock()
		delete(c.items, key)
		c.mu.Unlock()
		return nil
	})

	if err != nil {
		return fmt.Errorf("local delete: %w", err)
	}
	return nil
}

// DeleteByPrefix removes all keys matching the given prefix.
func (c *Cache) DeleteByPrefix(ctx context.Context, prefix string) error {
	err := c.hook.Execute(ctx, "delete_by_prefix", prefix+"*", func() error {
		c.mu.Lock()
		for k := range c.items {
			if strings.HasPrefix(k, prefix) {
				delete(c.items, k)
			}
		}
		c.mu.Unlock()
		return nil
	})

	if err != nil {
		return fmt.Errorf("local delete by prefix: %w", err)
	}
	return nil
}

// SetJSON saves any object by marshaling it to JSON.
func (c *Cache) SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("local store marshal: %w", err)
	}
	return c.Set(ctx, key, data, expiration)
}

// GetJSON fetches a value and unmarshals it into the provided target.
func (c *Cache) GetJSON(ctx context.Context, key string, target any) (bool, error) {
	data, err := c.Get(ctx, key)
	if err != nil {
		return false, err
	}

	if data == nil {
		return false, nil
	}

	if err := json.Unmarshal(data, target); err != nil {
		return false, err
	}

	return true, nil
}

// Close gracefully closes the local cache and stops the background cleaner.
func (c *Cache) Close(ctx context.Context) error {
	c.closeOnce.Do(func() {
		close(c.stopCh)
		c.mu.Lock()
		c.items = make(map[string]item)
		c.mu.Unlock()
	})

	if c.logger != nil {
		c.logger.Info(ctx, "closing local cache", logger.LogPayload{})
	}
	return nil
}

func toBytes(v any) ([]byte, error) {
	switch val := v.(type) {
	case []byte:
		b := make([]byte, len(val))
		copy(b, val)
		return b, nil
	case string:
		return []byte(val), nil
	case int:
		return []byte(strconv.Itoa(val)), nil
	case int64:
		return []byte(strconv.FormatInt(val, 10)), nil
	case uint:
		return []byte(strconv.FormatUint(uint64(val), 10)), nil
	case uint64:
		return []byte(strconv.FormatUint(val, 10)), nil
	case float64:
		return []byte(strconv.FormatFloat(val, 'f', -1, 64)), nil
	case bool:
		return []byte(strconv.FormatBool(val)), nil
	default:
		return json.Marshal(val)
	}
}

// Check implements health.Check for LocalCache readiness.
func (c *Cache) Check(ctx context.Context) health.Health {
	return health.Up()
}
