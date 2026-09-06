package idempotency

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestMemoryStorage(t *testing.T) {
	ctx := context.Background()
	m := NewMemoryStorage()

	// Get non-existent
	_, err := m.Get(ctx, "non-existent")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}

	// Set with expiration
	err = m.Set(ctx, "test-key", []byte("test-val"), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected set error: %v", err)
	}

	val, err := m.Get(ctx, "test-key")
	if err != nil || string(val) != "test-val" {
		t.Fatalf("expected test-val, got %s, err: %v", string(val), err)
	}

	// Wait for expiration
	time.Sleep(70 * time.Millisecond)
	_, err = m.Get(ctx, "test-key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected expired key to return ErrNotFound, got %v", err)
	}

	// Delete
	_ = m.Set(ctx, "del-key", []byte("del-val"), time.Minute)
	_ = m.Delete(ctx, "del-key")
	_, err = m.Get(ctx, "del-key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected deleted key to return ErrNotFound, got %v", err)
	}
}

func TestEngine_CacheMissAndHit(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	engine := NewEngine(storage, WithDefaultTTL(time.Minute), WithKeyPrefix("idem"))

	var executionCount int32
	executeFn := func(ctx context.Context) (*Record, error) {
		atomic.AddInt32(&executionCount, 1)
		return &Record{
			StatusCode: 200,
			Headers:    map[string]string{"Content-Type": "application/json"},
			Body:       []byte(`{"message":"success"}`),
		}, nil
	}

	// 1st request -> Miss (Leader)
	rec1, isReplay1, err := engine.Execute(ctx, "req-1", time.Minute, executeFn)
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}
	if isReplay1 {
		t.Fatalf("expected isReplay = false for first execute")
	}
	if string(rec1.Body) != `{"message":"success"}` {
		t.Fatalf("unexpected body: %s", string(rec1.Body))
	}
	if atomic.LoadInt32(&executionCount) != 1 {
		t.Fatalf("expected executionCount = 1, got %d", executionCount)
	}

	// 2nd request -> Hit (Replay)
	rec2, isReplay2, err := engine.Execute(ctx, "req-1", time.Minute, executeFn)
	if err != nil {
		t.Fatalf("second execute failed: %v", err)
	}
	if !isReplay2 {
		t.Fatalf("expected isReplay = true for second execute")
	}
	if string(rec2.Body) != `{"message":"success"}` {
		t.Fatalf("unexpected body: %s", string(rec2.Body))
	}
	if atomic.LoadInt32(&executionCount) != 1 {
		t.Fatalf("expected executionCount to remain 1, got %d", executionCount)
	}
}

func TestEngine_InflightConcurrency(t *testing.T) {
	ctx := context.Background()
	storage := NewMemoryStorage()
	engine := NewEngine(storage, WithDefaultTTL(time.Minute))

	var executionCount int32
	executeFn := func(ctx context.Context) (*Record, error) {
		time.Sleep(50 * time.Millisecond) // Simulate DB processing
		atomic.AddInt32(&executionCount, 1)
		return &Record{
			StatusCode: 201,
			Body:       []byte(`{"created":true}`),
		}, nil
	}

	var wg sync.WaitGroup
	results := make([]*Record, 5)
	replays := make([]bool, 5)

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			rec, isReplay, _ := engine.Execute(ctx, "concurrent-key", time.Minute, executeFn)
			results[idx] = rec
			replays[idx] = isReplay
		}(i)
	}

	wg.Wait()

	if atomic.LoadInt32(&executionCount) != 1 {
		t.Fatalf("expected handler to execute exactly once, executed %d times", executionCount)
	}

	var replayCount int
	for i := 0; i < 5; i++ {
		if results[i] == nil || string(results[i].Body) != `{"created":true}` {
			t.Fatalf("result %d is invalid: %v", i, results[i])
		}
		if replays[i] {
			replayCount++
		}
	}

	// Exactly 1 leader (isReplay=false) and 4 replays (isReplay=true)
	if replayCount != 4 {
		t.Fatalf("expected 4 replays, got %d", replayCount)
	}
}

func TestKeys_HashAndBuild(t *testing.T) {
	hash := HashBytes([]byte("test"))
	if len(hash) != 64 {
		t.Fatalf("expected 64 char hex sha256, got %d", len(hash))
	}

	key := BuildCanonicalKey("idem", "user", "123", "orders")
	if key != "idem:user:123:orders" {
		t.Fatalf("expected idem:user:123:orders, got %s", key)
	}
}
