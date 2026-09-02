package cache

import (
	"context"
	"time"
)

// Cache defines an interface for redis, in-memory operations.
type Cache interface {
	Set(ctx context.Context, key string, value []byte, expiration time.Duration) error
	Get(ctx context.Context, key string) ([]byte, error)
	SetNX(ctx context.Context, key string, value any, expiration time.Duration) (bool, error)
	Incr(ctx context.Context, key string) (int64, error)
	Delete(ctx context.Context, key string) error
	DeleteByPrefix(ctx context.Context, prefix string) error
	SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error
	GetJSON(ctx context.Context, key string, target any) (bool, error)
	Close(ctx context.Context) error
}
