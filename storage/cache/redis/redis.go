package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// Ensure Cache implements cache.Cache interface
var _ cache.Cache = (*Cache)(nil)

// Ensure Cache implements health.Check interface
var _ health.Check = (*Cache)(nil)

// Cache is a Cache implementation using Redis.
// It supports standalone, cluster, and sentinel topologies via redis.UniversalClient.
type Cache struct {
	client  redis.UniversalClient
	logger  logger.Logger
	appName string
}

// NewRedisCache creates a new instance of RedisCache.
// If health.Registry is provided, it automatically registers a readiness check for Redis.
// The topology (standalone / cluster / sentinel) is selected automatically from Config.Type:
//   - standalone: single node, uses Config.DB
//   - cluster:    multi-node sharding, uses Config.MaxRedirects / routing options
//   - sentinel:   HA failover, requires Config.MasterName
func NewRedisCache(
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
	hrs ...health.Registry,
) (*Cache, error) {

	cfg = mergeConfig(DefaultConfig(), cfg)

	ctx := context.Background()
	appName := coreContext.AppName(ctx)

	client := redis.NewUniversalClient(buildUniversalOptions(cfg))

	// Add hooks for tracing and logging
	client.AddHook(NewHook(l, cfg, appName))

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()

			if err := client.Ping(pingCtx).Err(); err != nil {
				client.Close()
				return fmt.Errorf("ping redis (%s): %w", cfg.Type, err)
			}

			l.Info(ctx, "redis connection established", logger.LogPayload{
				Type:      "STORAGE_REDIS",
				Method:    "START",
				IpAddress: fmt.Sprintf("%v", cfg.Addrs),
				AppName:   appName,
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

	c := &Cache{
		client:  client,
		logger:  l,
		appName: appName,
	}

	for _, hr := range hrs {
		if hr != nil {
			hr.RegisterReadiness("redis", c)
			break
		}
	}

	return c, nil
}

// Check implements health.Check for Redis readiness.
func (c *Cache) Check(ctx context.Context) health.Health {
	if c.client == nil {
		return health.DownWithDetails(map[string]any{"error": "redis client is not initialized"})
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	if err := c.client.Ping(ctx).Err(); err != nil {
		return health.DownWithDetails(map[string]any{"error": err.Error()})
	}
	return health.Up()
}

// mergeConfig applies non-zero fields from override on top of base.
// This lets callers pass a partial Config and still get sensible defaults
// for any fields they did not explicitly set.
func mergeConfig(base, override Config) Config {
	mergeConnectionConfig(&base, override)
	mergePoolAndTimeoutConfig(&base, override)
	mergeRetryAndClusterConfig(&base, override)
	mergeFlagsConfig(&base, override)
	return base
}

func mergeConnectionConfig(base *Config, override Config) {
	if override.Type != "" {
		base.Type = override.Type
	}
	if len(override.Addrs) > 0 {
		base.Addrs = override.Addrs
	}
	if override.MasterName != "" {
		base.MasterName = override.MasterName
	}
	if override.Username != "" {
		base.Username = override.Username
	}
	if override.Password != "" {
		base.Password = override.Password
	}
	if override.DB != 0 {
		base.DB = override.DB
	}
}

func mergePoolAndTimeoutConfig(base *Config, override Config) {
	if override.PoolSize != 0 {
		base.PoolSize = override.PoolSize
	}
	if override.MinIdleConns != 0 {
		base.MinIdleConns = override.MinIdleConns
	}
	if override.DialTimeout != 0 {
		base.DialTimeout = override.DialTimeout
	}
	if override.ReadTimeout != 0 {
		base.ReadTimeout = override.ReadTimeout
	}
	if override.WriteTimeout != 0 {
		base.WriteTimeout = override.WriteTimeout
	}
}

func mergeRetryAndClusterConfig(base *Config, override Config) {
	if override.MaxRetries != 0 {
		base.MaxRetries = override.MaxRetries
	}
	if override.MinRetryBackoff != 0 {
		base.MinRetryBackoff = override.MinRetryBackoff
	}
	if override.MaxRetryBackoff != 0 {
		base.MaxRetryBackoff = override.MaxRetryBackoff
	}
	if override.MaxRedirects != 0 {
		base.MaxRedirects = override.MaxRedirects
	}
}

func mergeFlagsConfig(base *Config, override Config) {
	if override.RouteByLatency {
		base.RouteByLatency = true
	}
	if override.RouteRandomly {
		base.RouteRandomly = true
	}
	if override.ReadOnly {
		base.ReadOnly = true
	}
	if override.Tracing.Enabled {
		base.Tracing.Enabled = true
	}
	if override.Logger.Enabled {
		base.Logger.Enabled = true
	}
}

// buildUniversalOptions maps Config fields to redis.UniversalOptions.
// redis.NewUniversalClient will select the appropriate client type:
//
//	len(Addrs) > 1  && MasterName == ""  → ClusterClient
//	MasterName != ""                      → SentinelClient (Failover)
//	otherwise                             → single-node Client
//
// We enforce the same rule via Config.Type for explicitness.
func buildUniversalOptions(cfg Config) *redis.UniversalOptions {
	opts := &redis.UniversalOptions{
		Addrs:    cfg.Addrs,
		Username: cfg.Username,
		Password: cfg.Password,

		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,

		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,

		MaxRetries:      cfg.MaxRetries,
		MinRetryBackoff: cfg.MinRetryBackoff,
		MaxRetryBackoff: cfg.MaxRetryBackoff,
	}

	switch cfg.Type {
	case ModeStandalone:
		opts.DB = cfg.DB

	case ModeCluster:
		opts.MaxRedirects = cfg.MaxRedirects
		opts.RouteByLatency = cfg.RouteByLatency
		opts.RouteRandomly = cfg.RouteRandomly
		opts.ReadOnly = cfg.ReadOnly

	case ModeSentinel:
		opts.MasterName = cfg.MasterName
		opts.DB = cfg.DB
	}

	return opts
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

// DeleteByPrefix removes all keys matching the given prefix using SCAN + DEL.
// In cluster mode, SCAN is scoped per-shard automatically by UniversalClient.
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
