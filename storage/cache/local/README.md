# storage/cache/local

In-memory thread-safe cache backend implementing the [`cache.Cache`](../cache.go) interface.

---

## Features

| Feature | Detail |
|---|---|
| Thread-safety | Synchronized with `sync.RWMutex` for high concurrency |
| Expiration | Per-item TTL with lazy evaluation on `Get` |
| Background Janitor | Periodic cleanup of expired items via configurable `CleanupInterval` |
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
    cfg.CleanupInterval = 10 * time.Second

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

Di level bootstrap aplikasi, registrasikan lifecycle hook Fx:

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
    CleanupInterval time.Duration // Interval for background expired item cleanup (default: 1m)
    Logger          struct {
        Enabled bool // Enable detailed debug logs (default: false)
    }
}
```
