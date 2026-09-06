package http

import (
	"time"

	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
)

// IdempotencyStorage defines the storage interface needed for idempotency caching.
// Any storage/cache.Cache implementation automatically satisfies this interface.
type IdempotencyStorage = middleware.IdempotencyStorage

// CanonicalKeyFunc defines the signature for custom idempotency key generators.
type CanonicalKeyFunc = middleware.CanonicalKeyFunc

// IdempotencyOption provides functional options to configure Idempotency middleware.
type IdempotencyOption = middleware.IdempotencyOption

// Idempotency returns a Gin middleware ensuring idempotent request execution.
func Idempotency(storage IdempotencyStorage, opts ...IdempotencyOption) gin.HandlerFunc {
	return middleware.Idempotency(storage, opts...)
}

// WithCanonicalKeyFunc sets custom canonical key generation logic.
func WithCanonicalKeyFunc(fn CanonicalKeyFunc) IdempotencyOption {
	return middleware.WithCanonicalKeyFunc(fn)
}

// WithSimpleKeyFunc sets custom canonical key generation returning a string.
func WithSimpleKeyFunc(fn func(c *gin.Context, reqBody []byte) string) IdempotencyOption {
	return middleware.WithSimpleKeyFunc(fn)
}

// WithHeader configures idempotency to read from a specific header (e.g. "X-Idempotency-Key").
func WithHeader(headerName string, fallback ...CanonicalKeyFunc) IdempotencyOption {
	return middleware.WithHeader(headerName, fallback...)
}

// WithIdempotencyTTL sets custom TTL for cached idempotent responses (default: 30s).
func WithIdempotencyTTL(ttl time.Duration) IdempotencyOption {
	return middleware.WithTTL(ttl)
}

// WithIdempotencyLockTimeout sets maximum wait time for in-flight requests (default: 10s).
func WithIdempotencyLockTimeout(timeout time.Duration) IdempotencyOption {
	return middleware.WithLockTimeout(timeout)
}

// DefaultCanonicalKey provides the default canonical key implementation.
var DefaultCanonicalKey = middleware.DefaultCanonicalKey

// HeaderCanonicalKey provides header-based canonical key extraction.
var HeaderCanonicalKey = middleware.HeaderCanonicalKey

// NewMemoryIdempotencyStorage creates an in-memory IdempotencyStorage instance.
func NewMemoryIdempotencyStorage() IdempotencyStorage {
	return middleware.NewMemoryStorage()
}
