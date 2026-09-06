package idempotency

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	// ErrTimeoutWaitingInflight is returned when a duplicate concurrent request times out waiting for the in-flight caller.
	ErrTimeoutWaitingInflight = errors.New("idempotency: timeout waiting for in-flight request")
	// ErrInflightFailed is returned when an in-flight request completed without a cacheable record.
	ErrInflightFailed = errors.New("idempotency: in-flight request did not produce a cacheable record")
)

type inflightCall struct {
	done chan struct{}
	rec  *Record
	err  error
}

// Engine coordinates idempotent request processing, in-flight locks, and storage caching.
type Engine struct {
	storage     Storage
	inflight    sync.Map
	defaultTTL  time.Duration
	lockTimeout time.Duration
	prefix      string
}

// NewEngine creates an Engine with the provided storage and options.
func NewEngine(storage Storage, opts ...EngineOption) *Engine {
	if storage == nil {
		storage = NewMemoryStorage()
	}

	e := &Engine{
		storage:     storage,
		defaultTTL:  5 * time.Minute,
		lockTimeout: 10 * time.Second,
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Storage returns the underlying storage implementation.
func (e *Engine) Storage() Storage {
	return e.storage
}

// Execute runs the idempotent operation.
// If the key exists in cache, it returns the cached Record with isReplay = true.
// If another goroutine is concurrently executing for the same key, it waits and returns the result with isReplay = true.
// If neither, it executes executeFn, caches the returned Record, and returns isReplay = false.
func (e *Engine) Execute(
	ctx context.Context,
	key string,
	ttl time.Duration,
	executeFn func(ctx context.Context) (*Record, error),
) (*Record, bool, error) {
	if key == "" {
		rec, err := executeFn(ctx)
		return rec, false, err
	}

	fullKey := e.resolveKey(key)

	// 1. Check storage cache
	if rec, ok := e.getCachedRecord(ctx, fullKey); ok {
		return rec, true, nil
	}

	// 2. Coordinate in-flight execution
	call := &inflightCall{done: make(chan struct{})}
	actual, loaded := e.inflight.LoadOrStore(fullKey, call)
	if loaded {
		return e.waitInflight(ctx, fullKey, actual.(*inflightCall))
	}

	// 3. Current caller is the leader
	return e.executeLeader(ctx, fullKey, ttl, call, executeFn)
}

func (e *Engine) resolveKey(key string) string {
	if e.prefix == "" {
		return key
	}
	return e.prefix + ":" + key
}

func (e *Engine) getCachedRecord(ctx context.Context, key string) (*Record, bool) {
	val, err := e.storage.Get(ctx, key)
	if err != nil || len(val) == 0 {
		return nil, false
	}

	rec, err := UnmarshalRecord(val)
	if err != nil {
		return nil, false
	}

	return rec, true
}

func (e *Engine) waitInflight(ctx context.Context, key string, call *inflightCall) (*Record, bool, error) {
	select {
	case <-call.done:
		if call.err != nil {
			return nil, false, call.err
		}
		if call.rec != nil {
			return call.rec, true, nil
		}
		if rec, ok := e.getCachedRecord(ctx, key); ok {
			return rec, true, nil
		}
		return nil, false, ErrInflightFailed

	case <-time.After(e.lockTimeout):
		return nil, false, ErrTimeoutWaitingInflight

	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

func (e *Engine) executeLeader(
	ctx context.Context,
	key string,
	ttl time.Duration,
	call *inflightCall,
	executeFn func(ctx context.Context) (*Record, error),
) (*Record, bool, error) {
	defer func() {
		e.inflight.Delete(key)
		close(call.done)
	}()

	rec, execErr := executeFn(ctx)
	if execErr != nil {
		call.err = execErr
		return nil, false, execErr
	}

	if rec != nil {
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = time.Now()
		}
		call.rec = rec

		recordBytes, err := rec.Marshal()
		if err == nil {
			actualTTL := ttl
			if actualTTL <= 0 {
				actualTTL = e.defaultTTL
			}
			_ = e.storage.Set(ctx, key, recordBytes, actualTTL)
		}
	}

	return rec, false, nil
}
