package local_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/storage/cache/local"
)

type mockLogger struct{}

func (m *mockLogger) Info(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (m *mockLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {}
func (m *mockLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {}
func (m *mockLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload)  {}
func (m *mockLogger) Fatal(ctx context.Context, msg string, payload logger.LogPayload) {}

func newTestCache(t *testing.T, opts ...func(*local.Config)) *local.Cache {
	t.Helper()
	cfg := local.DefaultConfig()
	cfg.CleanupInterval = 0 // disable background cleaner by default for deterministic unit tests
	for _, opt := range opts {
		opt(&cfg)
	}

	c, err := local.NewLocalCache(&mockLogger{}, cfg)
	if err != nil {
		t.Fatalf("failed to create local cache: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close(context.Background())
	})
	return c
}

func TestInterfaceImplementation(t *testing.T) {
	var _ cache.Cache = (*local.Cache)(nil)
}

func TestSetAndGet(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// Get non-existent
	val, err := c.Get(ctx, "non_existent")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil value for cache miss, got: %v", val)
	}

	// Set and Get
	err = c.Set(ctx, "key1", []byte("value1"), time.Minute)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	val, err = c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if string(val) != "value1" {
		t.Fatalf("expected 'value1', got: %s", string(val))
	}

	// Verify slice mutation does not affect cached item
	val[0] = 'X'
	val2, _ := c.Get(ctx, "key1")
	if string(val2) != "value1" {
		t.Fatalf("cache should return independent copy, got mutated value: %s", string(val2))
	}
}

func TestExpiration(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// Set with short TTL
	err := c.Set(ctx, "short_ttl", []byte("temp"), 20*time.Millisecond)
	if err != nil {
		t.Fatalf("set failed: %v", err)
	}

	// Immediate get succeeds
	val, err := c.Get(ctx, "short_ttl")
	if err != nil || string(val) != "temp" {
		t.Fatalf("expected immediate hit, got: %v, err: %v", string(val), err)
	}

	// Wait for expiration
	time.Sleep(30 * time.Millisecond)

	val, err = c.Get(ctx, "short_ttl")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if val != nil {
		t.Fatalf("expected nil for expired item, got: %s", string(val))
	}
}

func TestSetNX(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// First set succeeds
	ok, err := c.SetNX(ctx, "nx_key", "val1", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("expected SetNX to return true for new key")
	}

	// Second set with same key returns false
	ok, err = c.SetNX(ctx, "nx_key", "val2", time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatal("expected SetNX to return false for existing key")
	}

	// Value should still be original
	val, _ := c.Get(ctx, "nx_key")
	if string(val) != "val1" {
		t.Fatalf("expected 'val1', got: %s", string(val))
	}

	// SetNX on expired key succeeds
	_ = c.Set(ctx, "exp_key", []byte("old"), 20*time.Millisecond)
	time.Sleep(30 * time.Millisecond)

	ok, err = c.SetNX(ctx, "exp_key", "new", time.Minute)
	if err != nil || !ok {
		t.Fatalf("expected SetNX to succeed on expired key, ok: %v, err: %v", ok, err)
	}
	val, _ = c.Get(ctx, "exp_key")
	if string(val) != "new" {
		t.Fatalf("expected 'new', got: %s", string(val))
	}
}

func TestIncr(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// Incr non-existent key
	n, err := c.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("incr error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1, got %d", n)
	}

	// Incr again
	n, err = c.Incr(ctx, "counter")
	if err != nil {
		t.Fatalf("incr error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2, got %d", n)
	}

	// Incr non-numeric value fails
	_ = c.Set(ctx, "string_key", []byte("not_a_number"), 0)
	_, err = c.Incr(ctx, "string_key")
	if err == nil {
		t.Fatal("expected error incrementing non-numeric string, got nil")
	}

	// Incr expired key resets to 1
	_ = c.Set(ctx, "temp_counter", []byte("100"), 20*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	n, err = c.Incr(ctx, "temp_counter")
	if err != nil {
		t.Fatalf("incr error on expired key: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected reset to 1 on expired key, got %d", n)
	}
}

func TestDelete(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "del_key", []byte("data"), 0)

	err := c.Delete(ctx, "del_key")
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}

	val, err := c.Get(ctx, "del_key")
	if err != nil || val != nil {
		t.Fatalf("expected nil after delete, got: %v, err: %v", val, err)
	}

	// Deleting non-existent key should not error
	err = c.Delete(ctx, "del_key")
	if err != nil {
		t.Fatalf("expected nil error deleting non-existent key, got: %v", err)
	}
}

func TestDeleteByPrefix(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	_ = c.Set(ctx, "user:1", []byte("alice"), 0)
	_ = c.Set(ctx, "user:2", []byte("bob"), 0)
	_ = c.Set(ctx, "user:session:1", []byte("sess1"), 0)
	_ = c.Set(ctx, "order:1", []byte("order1"), 0)

	err := c.DeleteByPrefix(ctx, "user:")
	if err != nil {
		t.Fatalf("delete by prefix failed: %v", err)
	}

	v1, _ := c.Get(ctx, "user:1")
	v2, _ := c.Get(ctx, "user:2")
	vs, _ := c.Get(ctx, "user:session:1")
	vo, _ := c.Get(ctx, "order:1")

	if v1 != nil || v2 != nil || vs != nil {
		t.Fatal("expected all user:* keys to be deleted")
	}
	if string(vo) != "order1" {
		t.Fatalf("order:1 should not be deleted, got: %s", string(vo))
	}
}

func TestSetJSONAndGetJSON(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	type User struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	u := User{ID: 42, Name: "Alice"}
	err := c.SetJSON(ctx, "user_json", u, time.Minute)
	if err != nil {
		t.Fatalf("set json failed: %v", err)
	}

	var target User
	ok, err := c.GetJSON(ctx, "user_json", &target)
	if err != nil {
		t.Fatalf("get json failed: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true for existing json key")
	}
	if target.ID != 42 || target.Name != "Alice" {
		t.Fatalf("unexpected unmarshaled data: %+v", target)
	}

	// Non-existent key
	var miss User
	ok, err = c.GetJSON(ctx, "missing_json", &miss)
	if err != nil || ok {
		t.Fatalf("expected ok=false, err=nil on missing key, got ok=%v, err=%v", ok, err)
	}
}

func TestJanitorCleanup(t *testing.T) {
	c := newTestCache(t, func(cfg *local.Config) {
		cfg.CleanupInterval = 20 * time.Millisecond
	})
	ctx := context.Background()

	_ = c.Set(ctx, "k1", []byte("v1"), 10*time.Millisecond)
	_ = c.Set(ctx, "k2", []byte("v2"), time.Hour)

	// Wait for janitor to sweep
	time.Sleep(50 * time.Millisecond)

	val1, _ := c.Get(ctx, "k1")
	if val1 != nil {
		t.Fatalf("k1 should have been cleaned up, got: %s", string(val1))
	}

	val2, _ := c.Get(ctx, "k2")
	if string(val2) != "v2" {
		t.Fatalf("k2 should still be present, got: %s", string(val2))
	}
}

func TestCloseIdempotency(t *testing.T) {
	c := newTestCache(t, func(cfg *local.Config) {
		cfg.CleanupInterval = 10 * time.Millisecond
	})
	ctx := context.Background()

	err1 := c.Close(ctx)
	if err1 != nil {
		t.Fatalf("first close failed: %v", err1)
	}

	err2 := c.Close(ctx)
	if err2 != nil {
		t.Fatalf("second close failed: %v", err2)
	}
}

func TestConcurrency(t *testing.T) {
	c := newTestCache(t, func(cfg *local.Config) {
		cfg.CleanupInterval = 20 * time.Millisecond
		cfg.Tracing.Enabled = true
		cfg.Logger.Enabled = true
	})
	ctx := context.Background()

	var wg sync.WaitGroup
	workers := 20
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				key := fmt.Sprintf("concurrent_key_%d", (workerID+j)%10)
				_ = c.Set(ctx, key, []byte("data"), 50*time.Millisecond)
				_, _ = c.Get(ctx, key)
				_, _ = c.SetNX(ctx, key, "val", 50*time.Millisecond)
				_, _ = c.Incr(ctx, fmt.Sprintf("incr_%d", (workerID+j)%5))
				if j%10 == 0 {
					_ = c.Delete(ctx, key)
					_ = c.DeleteByPrefix(ctx, "concurrent_key_")
				}
			}
		}()
	}

	wg.Wait()
}

func TestPureGoLifecycle(t *testing.T) {
	cfg := local.DefaultConfig()

	c, err := local.NewLocalCache(&mockLogger{}, cfg)
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil cache")
	}

	ctx := context.Background()
	_ = c.Set(ctx, "lifecycle_key", []byte("val"), time.Minute)
	val, err := c.Get(ctx, "lifecycle_key")
	if err != nil || string(val) != "val" {
		t.Fatalf("get failed during lifecycle: %v", err)
	}

	if err := c.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}
}

func TestMaxItems_Eviction(t *testing.T) {
	c := newTestCache(t, func(cfg *local.Config) {
		cfg.NumShards = 2
		cfg.MaxItems = 4 // 2 items per shard max
	})
	ctx := context.Background()

	// Insert 10 items
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("evict_key_%d", i)
		err := c.Set(ctx, key, []byte("val"), time.Hour)
		if err != nil {
			t.Fatalf("set failed for %s: %v", key, err)
		}
	}

	// Count how many keys remain
	count := 0
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("evict_key_%d", i)
		val, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("get failed: %v", err)
		}
		if val != nil {
			count++
		}
	}

	// Should not exceed maxItems = 4
	if count > 4 {
		t.Fatalf("expected at most 4 items remaining, got %d", count)
	}
	if count == 0 {
		t.Fatal("expected some items to remain, got 0")
	}
}

func TestShardingDistribution(t *testing.T) {
	c := newTestCache(t, func(cfg *local.Config) {
		cfg.NumShards = 16
		cfg.MaxItems = 10000
	})
	ctx := context.Background()

	// Insert 200 distinct keys
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("dist_key_%d", i)
		_ = c.Set(ctx, key, []byte("val"), time.Hour)
	}

	// Verify all 200 keys can be retrieved
	for i := 0; i < 200; i++ {
		key := fmt.Sprintf("dist_key_%d", i)
		val, err := c.Get(ctx, key)
		if err != nil || string(val) != "val" {
			t.Fatalf("failed to retrieve key %s: val=%v, err=%v", key, val, err)
		}
	}
}

func TestMGet(t *testing.T) {
	c := newTestCache(t)
	ctx := context.Background()

	// 1. Empty keys
	res, err := c.MGet(ctx)
	if err != nil {
		t.Fatalf("expected nil error on empty keys, got: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty result map, got: %v", res)
	}

	// 2. Non-existent keys
	res, err = c.MGet(ctx, "non_existent_1", "non_existent_2")
	if err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected empty map for missing keys, got: %v", res)
	}

	// 3. Populate test items
	_ = c.Set(ctx, "k1", []byte("val1"), time.Minute)
	_ = c.Set(ctx, "k2", []byte("val2"), time.Minute)
	_ = c.Set(ctx, "k_expired", []byte("val_exp"), 20*time.Millisecond)

	// Wait for k_expired to expire
	time.Sleep(30 * time.Millisecond)

	// 4. MGet with existing, missing, and expired keys
	res, err = c.MGet(ctx, "k1", "k2", "k_missing", "k_expired", "k1")
	if err != nil {
		t.Fatalf("mget failed: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("expected exactly 2 items in result, got %d: %v", len(res), res)
	}
	if string(res["k1"]) != "val1" {
		t.Errorf("expected k1 = 'val1', got '%s'", string(res["k1"]))
	}
	if string(res["k2"]) != "val2" {
		t.Errorf("expected k2 = 'val2', got '%s'", string(res["k2"]))
	}
	if _, ok := res["k_missing"]; ok {
		t.Errorf("k_missing should not be in result")
	}
	if _, ok := res["k_expired"]; ok {
		t.Errorf("k_expired should not be in result")
	}

	// Verify expired key was lazily purged
	getVal, err := c.Get(ctx, "k_expired")
	if err != nil || getVal != nil {
		t.Errorf("expected k_expired to be purged, got: %v, %v", getVal, err)
	}

	// 5. Verify byte slice independence (mutation of result does not mutate cache)
	res["k1"][0] = 'Z'
	checkVal, _ := c.Get(ctx, "k1")
	if string(checkVal) != "val1" {
		t.Errorf("mutation affected cache, expected 'val1', got '%s'", string(checkVal))
	}
}

func BenchmarkShardedCache_GetSet(b *testing.B) {
	cfg := local.DefaultConfig()
	cfg.NumShards = 32
	c, _ := local.NewLocalCache(&mockLogger{}, cfg)
	defer func() { _ = c.Close(context.Background()) }()

	ctx := context.Background()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := fmt.Sprintf("bench_key_%d", i%1000)
			_ = c.Set(ctx, key, []byte("bench_value"), time.Minute)
			_, _ = c.Get(ctx, key)
		}
	})
}
