package grpc

import (
	"time"

	"github.com/agilsyofian/gosdk/transport/idempotency"
	"github.com/agilsyofian/gosdk/transport/server/grpc/interceptor"
	"google.golang.org/grpc"
)

// IdempotencyStorage defines the storage interface needed for idempotency caching.
type IdempotencyStorage = idempotency.Storage

// IdempotencyOption provides functional options to configure the gRPC Idempotency interceptor.
type IdempotencyOption = interceptor.IdempotencyOption

// Idempotency returns a gRPC UnaryServerInterceptor that enforces idempotent execution.
func Idempotency(storage IdempotencyStorage, opts ...IdempotencyOption) grpc.UnaryServerInterceptor {
	return interceptor.Idempotency(storage, opts...)
}

// WithIdempotencyTTL sets custom TTL for cached idempotent responses.
func WithIdempotencyTTL(ttl time.Duration) IdempotencyOption {
	return interceptor.WithTTL(ttl)
}

// WithIdempotencyLockTimeout sets maximum wait time for in-flight requests.
func WithIdempotencyLockTimeout(timeout time.Duration) IdempotencyOption {
	return interceptor.WithLockTimeout(timeout)
}

// WithIdempotencyMetadataKey sets the incoming gRPC metadata key (default: "x-idempotency-key").
func WithIdempotencyMetadataKey(key string) IdempotencyOption {
	return interceptor.WithMetadataKey(key)
}

// WithCanonicalKeyFunc sets custom canonical key generation logic for gRPC.
func WithCanonicalKeyFunc(fn interceptor.CanonicalKeyFunc) IdempotencyOption {
	return interceptor.WithCanonicalKeyFunc(fn)
}

// WithIgnoreMethods specifies gRPC full method names that bypass idempotency.
func WithIgnoreMethods(methods ...string) IdempotencyOption {
	return interceptor.WithIgnoreMethods(methods...)
}

// NewMemoryIdempotencyStorage creates an in-memory IdempotencyStorage instance.
func NewMemoryIdempotencyStorage() IdempotencyStorage {
	return idempotency.NewMemoryStorage()
}
