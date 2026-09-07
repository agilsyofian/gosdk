# storage/cache/local

In-memory thread-safe cache backend implementing the [`cache.Cache`](../cache.go) interface.

---

## Features

| Feature | Detail |
|---|---|
| Sharded Architecture | Partitioned into $N$ shards (default: 32) to eliminate global mutex lock contention |
| Zero Allocations | Inline FNV-1a 32-bit hashing operating directly on string bytes without heap allocations |
| Bounded Memory | OOM prevention via configurable `MaxItems` and `MaxMemoryMB` bounds |
| Eviction Policy | Sampled LRU eviction algorithm with expired-first sweeping |
| Staggered Janitor | Non-blocking periodic cleanup with inter-shard CPU yields to prevent latency spikes |
| Thread-safety | Synchronized with per-shard `sync.RWMutex` and lock-free atomic read access tracking |
| Expiration | Per-item TTL with lazy evaluation on `Get` |
| JSON Helpers | `SetJSON` / `GetJSON` for easy serialization/deserialization |
| Prefix Delete | `DeleteByPrefix` for efficient namespace invalidation |
| Atomic Operations | `Incr` and `SetNX` for lightweight counters and mutex-like operations |
| Observability | Tracing spans & structured logging hook integration |
| Health Check | Implements `health.Check` for readiness verification |
| Lifecycle | Pure Go constructor with graceful `Close(ctx)` |

---

## Installation

```go
// go.mod
require github.com/agilsyofian/gosdk/storage/cache/local <version>
```

---

## Quick Start (Pure Go)

```go
import (
    "context"
    "time"

    "github.com/agilsyofian/gosdk/core/observability/logger"
    localcache "github.com/agilsyofian/gosdk/storage/cache/local"
)

func main() {
    ctx := context.Background()
    l := logger.NewZerologLogger(logger.Config{Output: "stdout"})

    cfg := localcache.DefaultConfig()
    cfg.NumShards = 32
    cfg.MaxItems = 100_000
    cfg.MaxMemoryMB = 256
    cfg.CleanupInterval = 5 * time.Minute

    cache, err := localcache.NewLocalCache(l, cfg)
    if err != nil {
        panic(err)
    }
    defer cache.Close(ctx)

    // Set and Get
    _ = cache.Set(ctx, "session:123", []byte("active"), 5*time.Minute)
    val, _ := cache.Get(ctx, "session:123")
}
```

---

## Uber Fx Integration (Application Layer)

At application bootstrap, register the Fx lifecycle hook:

```go
fx.Provide(
    func(lc fx.Lifecycle, l logger.Logger, hr health.Registry) (cache.Cache, error) {
        cfg := localcache.DefaultConfig()
        c, err := localcache.NewLocalCache(l, cfg, hr)
        if err != nil {
            return nil, err
        }
        lc.Append(fx.Hook{
            OnStop: func(ctx context.Context) error {
                return c.Close(ctx)
            },
        })
        return c, nil
    },
)
```

---

## Configuration Reference

```go
type Config struct {
    CleanupInterval time.Duration `mapstructure:"cleanup_interval"` // Interval for background cleanup (default: 5m)
    NumShards       int           `mapstructure:"num_shards"`        // Number of partitions (default: 32)
    MaxItems        int           `mapstructure:"max_items"`         // Maximum entries across all shards (default: 100,000)
    MaxMemoryMB     int           `mapstructure:"max_memory_mb"`     // Maximum RAM footprint in MB (default: 256MB)
    Tracing         struct {
        Enabled bool `mapstructure:"enabled"`
    } `mapstructure:"tracing"`
    Logger struct {
        Enabled bool `mapstructure:"enabled"`
    } `mapstructure:"logger"`
}
```
