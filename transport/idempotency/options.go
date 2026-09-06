package idempotency

import (
	"time"
)

// EngineOption configures an Engine instance.
type EngineOption func(*Engine)

// WithDefaultTTL sets the default cache TTL for records.
func WithDefaultTTL(ttl time.Duration) EngineOption {
	return func(e *Engine) {
		if ttl > 0 {
			e.defaultTTL = ttl
		}
	}
}

// WithLockTimeout sets the maximum time a concurrent caller will wait for an in-flight request.
func WithLockTimeout(timeout time.Duration) EngineOption {
	return func(e *Engine) {
		if timeout > 0 {
			e.lockTimeout = timeout
		}
	}
}

// WithKeyPrefix sets a global key namespace prefix.
func WithKeyPrefix(prefix string) EngineOption {
	return func(e *Engine) {
		e.prefix = prefix
	}
}
