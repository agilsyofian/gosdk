package local

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/cache"
)

// Ensure Cache implements cache.Cache interface
var _ cache.Cache = (*Cache)(nil)

// Ensure Cache implements health.Check interface
var _ health.Check = (*Cache)(nil)

type item struct {
	value          []byte
	expiresAt      time.Time
	lastAccessedAt int64 // unix nano timestamp updated atomically
}

func (i *item) isExpired(now time.Time) bool {
	if i.expiresAt.IsZero() {
		return false
	}
	return now.After(i.expiresAt)
}

// itemSize calculates the approximate memory footprint in bytes.
func itemSize(key string, valLen int) int64 {
	// key header (16B) + key bytes + slice header (24B) + slice bytes + item struct (48B) + map entry overhead (~48B)
	return int64(len(key) + valLen + 136)
}

// shard represents an isolated partition of the cache with its own read-write lock.
type shard struct {
	mu           sync.RWMutex
	items        map[string]*item
	currentBytes int64
}

// deleteKey removes a key and decrements the shard's byte footprint.
func (s *shard) deleteKey(key string) {
	if it, ok := s.items[key]; ok {
		s.currentBytes -= itemSize(key, len(it.value))
		delete(s.items, key)
	}
}

// isOverQuota returns true if the shard exceeds either item count or byte limits.
func (s *shard) isOverQuota(maxItems int, maxBytes int64) bool {
	if maxItems > 0 && len(s.items) >= maxItems {
		return true
	}
	if maxBytes > 0 && s.currentBytes > maxBytes {
		return true
	}
	return false
}

// sweepExpiredSample iterates through up to sampleLimit items and removes any expired entries.
func (s *shard) sweepExpiredSample(now time.Time, sampleLimit int) {
	count := 0
	for k, it := range s.items {
		if it.isExpired(now) {
			s.deleteKey(k)
		}
		count++
		if count >= sampleLimit {
			break
		}
	}
}

// sampleOldest finds the least recently accessed key among up to sampleLimit items.
func (s *shard) sampleOldest(sampleLimit int) string {
	var oldestKey string
	var oldestTime int64 = 1<<63 - 1
	count := 0

	for k, it := range s.items {
		access := atomic.LoadInt64(&it.lastAccessedAt)
		if access < oldestTime {
			oldestTime = access
			oldestKey = k
		}
		count++
		if count >= sampleLimit {
			break
		}
	}
	return oldestKey
}

// evict implements a hybrid Sampled LRU eviction strategy.
// It first attempts to sweep expired entries; if still over capacity, it evicts
// the least recently accessed item among random samples.
func (s *shard) evict(now time.Time, maxItems int, maxBytes int64) {
	if !s.isOverQuota(maxItems, maxBytes) {
		return
	}

	const sampleLimit = 10
	s.sweepExpiredSample(now, sampleLimit)

	if !s.isOverQuota(maxItems, maxBytes) {
		return
	}

	if oldest := s.sampleOldest(sampleLimit); oldest != "" {
		s.deleteKey(oldest)
	}
}

// fnv32 implements a zero-allocation inline FNV-1a 32-bit hash on string bytes.
func fnv32(key string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(key); i++ {
		h ^= uint32(key[i])
		h *= 16777619
	}
	return h
}

func roundUpPowerOfTwo(n int) int {
	if n <= 1 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

// Cache is a high-performance, sharded in-memory Cache implementation with bounded memory.
type Cache struct {
	shards           []*shard
	shardMask        uint32
	maxItemsPerShard int
	maxBytesPerShard int64
	hook             *Hook
	logger           logger.Logger
	appName          string
	stopCh           chan struct{}
	closeOnce        sync.Once
}

// NewLocalCache creates a new instance of LocalCache in Pure Go with sharded partitions.
// If health.Registry is provided, it automatically registers a readiness check for LocalCache.
func NewLocalCache(
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*Cache, error) {
	ctx := context.Background()
	appName := coreContext.AppName(ctx)

	numShards := cfg.NumShards
	if numShards <= 0 {
		numShards = 32
	}
	numShards = roundUpPowerOfTwo(numShards)

	shards := make([]*shard, numShards)
	for i := 0; i < numShards; i++ {
		shards[i] = &shard{
			items: make(map[string]*item),
		}
	}

	var maxItemsPerShard int
	if cfg.MaxItems > 0 {
		maxItemsPerShard = cfg.MaxItems / numShards
		if maxItemsPerShard < 1 {
			maxItemsPerShard = 1
		}
	}

	var maxBytesPerShard int64
	if cfg.MaxMemoryMB > 0 {
		totalBytes := int64(cfg.MaxMemoryMB) * 1024 * 1024
		maxBytesPerShard = totalBytes / int64(numShards)
		if maxBytesPerShard < 1024 {
			maxBytesPerShard = 1024
		}
	}

	c := &Cache{
		shards:           shards,
		shardMask:        uint32(numShards - 1),
		maxItemsPerShard: maxItemsPerShard,
		maxBytesPerShard: maxBytesPerShard,
		hook:             NewHook(l, cfg, appName),
		logger:           l,
		appName:          appName,
		stopCh:           make(chan struct{}),
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

	if l != nil {
		l.Info(ctx, "local cache initialized", logger.LogPayload{
			Type:   "STORAGE_LOCAL",
			Method: "START",
		})
	}

	return c, nil
}

func (c *Cache) getShard(key string) *shard {
	return c.shards[fnv32(key)&c.shardMask]
}

// startJanitor runs periodic cleanup across shards in a staggered, non-blocking fashion.
func (c *Cache) startJanitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case now := <-ticker.C:
			c.sweepShards(now)
		}
	}
}

// sweepShards cleans expired keys sequentially across shards with CPU yields to avoid latency spikes.
func (c *Cache) sweepShards(now time.Time) {
	for _, s := range c.shards {
		select {
		case <-c.stopCh:
			return
		default:
		}

		s.mu.Lock()
		for k, it := range s.items {
			if it.isExpired(now) {
				s.deleteKey(k)
			}
		}
		s.mu.Unlock()

		// Yield scheduler briefly between shards to prioritize real-time request traffic
		runtime.Gosched()
	}
}

// Get retrieves a value from the local cache.
// Returns nil, nil on cache miss or when expired.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	var result []byte
	err := c.hook.Execute(ctx, "get", key, func() error {
		s := c.getShard(key)

		s.mu.RLock()
		it, found := s.items[key]
		if !found {
			s.mu.RUnlock()
			return nil
		}

		now := time.Now()
		if it.isExpired(now) {
			s.mu.RUnlock()
			s.mu.Lock()
			if cur, ok := s.items[key]; ok && cur.isExpired(now) {
				s.deleteKey(key)
			}
			s.mu.Unlock()
			return nil
		}

		// Non-blocking lock-free access tracking for Sampled LRU
		atomic.StoreInt64(&it.lastAccessedAt, now.UnixNano())

		result = make([]byte, len(it.value))
		copy(result, it.value)
		s.mu.RUnlock()
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("local get: %w", err)
	}
	return result, nil
}

// MGet retrieves multiple values from the local cache simultaneously.
// Returns a map of key -> value for existing, non-expired keys. Missing or expired keys are omitted.
func (c *Cache) MGet(ctx context.Context, keys ...string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return make(map[string][]byte), nil
	}

	result := make(map[string][]byte)
	err := c.hook.Execute(ctx, "mget", strings.Join(keys, " "), func() error {
		shardGroups := c.groupKeysByShard(keys)
		now := time.Now()
		for s, shardKeys := range shardGroups {
			s.mget(shardKeys, now, result)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("local mget: %w", err)
	}
	return result, nil
}

func (c *Cache) groupKeysByShard(keys []string) map[*shard][]string {
	shardGroups := make(map[*shard][]string)
	for _, key := range keys {
		s := c.getShard(key)
		shardGroups[s] = append(shardGroups[s], key)
	}
	return shardGroups
}

func (s *shard) mget(keys []string, now time.Time, result map[string][]byte) {
	expiredKeys := s.mgetReadKeys(keys, now, result)
	s.deleteExpiredKeys(expiredKeys, now)
}

func (s *shard) mgetReadKeys(keys []string, now time.Time, result map[string][]byte) []string {
	var expiredKeys []string
	s.mu.RLock()
	for _, key := range keys {
		it, found := s.items[key]
		if !found {
			continue
		}
		if it.isExpired(now) {
			expiredKeys = append(expiredKeys, key)
			continue
		}

		atomic.StoreInt64(&it.lastAccessedAt, now.UnixNano())
		valCopy := make([]byte, len(it.value))
		copy(valCopy, it.value)
		result[key] = valCopy
	}
	s.mu.RUnlock()
	return expiredKeys
}

func (s *shard) deleteExpiredKeys(keys []string, now time.Time) {
	if len(keys) == 0 {
		return
	}

	s.mu.Lock()
	for _, key := range keys {
		if cur, ok := s.items[key]; ok && cur.isExpired(now) {
			s.deleteKey(key)
		}
	}
	s.mu.Unlock()
}

// Set stores a value in the local cache with an optional expiration.
func (c *Cache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	err := c.hook.Execute(ctx, "set", key, func() error {
		valCopy := make([]byte, len(value))
		copy(valCopy, value)

		now := time.Now()
		var expiresAt time.Time
		if expiration > 0 {
			expiresAt = now.Add(expiration)
		}

		s := c.getShard(key)
		s.mu.Lock()
		defer s.mu.Unlock()

		// If key already exists, subtract its prior memory footprint
		if old, exists := s.items[key]; exists {
			s.currentBytes -= itemSize(key, len(old.value))
		}

		// Enforce capacity bounds
		s.evict(now, c.maxItemsPerShard, c.maxBytesPerShard)

		newEntry := &item{
			value:          valCopy,
			expiresAt:      expiresAt,
			lastAccessedAt: now.UnixNano(),
		}
		s.items[key] = newEntry
		s.currentBytes += itemSize(key, len(valCopy))

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

		s := c.getShard(key)
		s.mu.Lock()
		defer s.mu.Unlock()

		if it, exists := s.items[key]; exists && !it.isExpired(now) {
			stored = false
			return nil
		}

		if old, exists := s.items[key]; exists {
			s.currentBytes -= itemSize(key, len(old.value))
		}

		s.evict(now, c.maxItemsPerShard, c.maxBytesPerShard)

		s.items[key] = &item{
			value:          valBytes,
			expiresAt:      expiresAt,
			lastAccessedAt: now.UnixNano(),
		}
		s.currentBytes += itemSize(key, len(valBytes))
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
		s := c.getShard(key)
		s.mu.Lock()
		defer s.mu.Unlock()

		now := time.Now()
		it, exists := s.items[key]
		if !exists || it.isExpired(now) {
			if exists {
				s.currentBytes -= itemSize(key, len(it.value))
			}
			s.evict(now, c.maxItemsPerShard, c.maxBytesPerShard)

			newVal = 1
			valBytes := []byte("1")
			s.items[key] = &item{
				value:          valBytes,
				lastAccessedAt: now.UnixNano(),
			}
			s.currentBytes += itemSize(key, len(valBytes))
			return nil
		}

		current, err := strconv.ParseInt(string(it.value), 10, 64)
		if err != nil {
			return fmt.Errorf("value is not an integer: %w", err)
		}

		newVal = current + 1
		valBytes := []byte(strconv.FormatInt(newVal, 10))
		s.currentBytes -= itemSize(key, len(it.value))
		s.currentBytes += itemSize(key, len(valBytes))
		it.value = valBytes
		atomic.StoreInt64(&it.lastAccessedAt, now.UnixNano())
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
		s := c.getShard(key)
		s.mu.Lock()
		s.deleteKey(key)
		s.mu.Unlock()
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
		for _, s := range c.shards {
			s.mu.Lock()
			for k := range s.items {
				if strings.HasPrefix(k, prefix) {
					s.deleteKey(k)
				}
			}
			s.mu.Unlock()
		}
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
		for _, s := range c.shards {
			s.mu.Lock()
			s.items = make(map[string]*item)
			s.currentBytes = 0
			s.mu.Unlock()
		}
	})

	if c.logger != nil {
		c.logger.Info(ctx, "stopping local cache", logger.LogPayload{
			Type:   "STORAGE_LOCAL",
			Method: "SHUTDOWN",
		})
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
