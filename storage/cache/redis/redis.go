package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agilsyofian/gosdk/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Ensure Cache implements xcache.Cache interface
var _ cache.Cache = (*Cache)(nil)

// Cache is a Cache implementation using Redis.
type Cache struct {
	client  redis.UniversalClient
	logger  logger.Logger
	appName string
}

// NewRedisCache creates a new instance of RedisCache.
func NewRedisCache(
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
) (*Cache, error) {

	ctx := context.Background()
	appName := coreContext.AppName(ctx)

	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:    cfg.Addrs,
		Username: cfg.Username,
		Password: cfg.Password,
		DB:       cfg.DB,
	})

	// Add hooks for tracing and logging
	client.AddHook(NewHook(l, cfg, appName))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := client.Ping(pingCtx).Err(); err != nil {
				client.Close()
				return fmt.Errorf("ping redis: %w", err)
			}

			l.Info(ctx, "redis connection established", logger.LogPayload{
				Type:      "STORAGE_REDIS",
				Method:    "START",
				IpAddress: fmt.Sprintf("%v", cfg.Addrs),
			})
			return nil
		},
		OnStop: func(ctx context.Context) error {
			l.Info(ctx, "stopping redis", logger.LogPayload{
				Type:   "STORAGE_REDIS",
				Method: "SHUTDOWN",
			})
			return client.Close()
		},
	})

	return &Cache{
		client:  client,
		logger:  l,
		appName: appName,
	}, nil
}

// Get retrieves a value from redis.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, nil // Cache miss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get: %w", err)
	}
	return val, nil
}

// Set stores a value in redis.
func (c *Cache) Set(ctx context.Context, key string, value []byte, expiration time.Duration) error {
	if err := c.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}
	return nil
}

// SetNX stores a value in redis if the key does not exist.
// Returns true if successfully stored, false if key already exists.
func (c *Cache) SetNX(ctx context.Context, key string, value interface{}, expiration time.Duration) (bool, error) {
	val, err := c.client.SetNX(ctx, key, value, expiration).Result()
	if err != nil {
		return false, fmt.Errorf("redis setnx: %w", err)
	}
	return val, nil
}

// Incr increments the integer value of a key in redis.
func (c *Cache) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis incr: %w", err)
	}
	return val, nil
}

// Delete removes a value from redis.
func (c *Cache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete: %w", err)
	}
	return nil
}

func (c *Cache) DeleteByPrefix(ctx context.Context, prefix string) error {
	pattern := prefix + "*"

	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return fmt.Errorf("redis scan: %w", err)
		}

		if len(keys) > 0 {
			if err := c.client.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("redis delete: %w", err)
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// SetJSON saves any object by marshaling it to JSON.
func (c *Cache) SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("redis store marshal: %w", err)
	}
	return c.Set(ctx, key, data, expiration)
}

// GetJSON fetches a value and unmarshal it into the provided target.
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

// Close closes the Redis connection.
func (c *Cache) Close(ctx context.Context) error {
	c.logger.Info(ctx, "closing redis connection", logger.LogPayload{})
	return c.client.Close()
}
