# storage/cache/redis

Redis cache backend implementing the [`cache.Cache`](../cache.go) interface.

Supports **standalone**, **cluster**, and **sentinel** topologies through a single
`redis.UniversalClient` from [go-redis/v9](https://github.com/redis/go-redis).

---

## Features

| Feature | Detail |
|---|---|
| Topology | Standalone, Cluster, Sentinel — switched via `Config.Type` |
| Default config | Applied automatically; caller only needs to override what differs |
| Observability | Per-command OpenTelemetry spans + structured logger hooks |
| Retry | Configurable `MaxRetries` with exponential backoff |
| JSON helpers | `SetJSON` / `GetJSON` for struct round-trips without boilerplate |
| Prefix delete | `DeleteByPrefix` using cursor-based `SCAN + DEL` (cluster-safe) |
| Lifecycle | Integrates with `go.uber.org/fx` — ping on start, graceful close on stop |

---

## Installation

```go
// go.mod
require github.com/agilsyofian/gosdk/storage/redis <version>
```

---

## Quick Start

### Standalone (single node)

```go
import rediscache "github.com/agilsyofian/gosdk/storage/redis"

cache, err := rediscache.NewRedisCache(lc, logger, rediscache.Config{
    Type:  rediscache.ModeStandalone,
    Addrs: []string{"localhost:6379"},
})
```

### Cluster

```go
cache, err := rediscache.NewRedisCache(lc, logger, rediscache.Config{
    Type:  rediscache.ModeCluster,
    Addrs: []string{
        "node1:6379",
        "node2:6379",
        "node3:6379",
    },
})
```

### Sentinel (High Availability)

```go
cache, err := rediscache.NewRedisCache(lc, logger, rediscache.Config{
    Type:       rediscache.ModeSentinel,
    MasterName: "mymaster",
    Addrs: []string{
        "sentinel1:26379",
        "sentinel2:26379",
    },
})
```

---

## Configuration Reference

`NewRedisCache` starts from `DefaultConfig()` and merges the caller-provided `Config`
on top — any field left at its zero value keeps the default.

```go
type Config struct {
    // Topology selection
    Type       Mode     // "standalone" | "cluster" | "sentinel"  (default: standalone)
    Addrs      []string // Redis addresses                         (default: ["localhost:6379"])
    MasterName string   // Sentinel master name (sentinel only)

    // Authentication
    Username string
    Password string
    DB       int // Logical database index (standalone/sentinel only, default: 0)

    // Connection pool
    PoolSize     int           // Max connections per node  (default: 10)
    MinIdleConns int           // Min idle connections      (default: 2)

    // Timeouts
    DialTimeout  time.Duration // (default: 5s)
    ReadTimeout  time.Duration // (default: 3s)
    WriteTimeout time.Duration // (default: 3s)

    // Retry
    MaxRetries      int           // (default: 3)
    MinRetryBackoff time.Duration // (default: 8ms)
    MaxRetryBackoff time.Duration // (default: 512ms)

    // Cluster-only options
    MaxRedirects   int  // MOVED/ASK redirections (default: 8)
    RouteByLatency bool // Route to lowest-latency node
    RouteRandomly  bool // Route to a random node
    ReadOnly       bool // Allow reads from replicas

    // Observability
    Tracing struct{ Enabled bool }
    Logger  struct{ Enabled bool }
}
```

### Defaults

```go
rediscache.DefaultConfig()
// → Type: standalone, Addrs: ["localhost:6379"]
//   PoolSize: 10, MinIdleConns: 2
//   DialTimeout: 5s, ReadTimeout: 3s, WriteTimeout: 3s
//   MaxRetries: 3, MinRetryBackoff: 8ms, MaxRetryBackoff: 512ms
//   MaxRedirects: 8
```

---

## API Reference

All methods satisfy the [`cache.Cache`](../cache.go) interface.

| Method | Description |
|---|---|
| `Get(ctx, key)` | Fetch raw bytes; returns `nil, nil` on cache miss |
| `Set(ctx, key, value, ttl)` | Store raw bytes with TTL |
| `SetNX(ctx, key, value, ttl)` | Store only if key absent; returns `true` on write |
| `Incr(ctx, key)` | Atomic integer increment |
| `Delete(ctx, key)` | Remove a single key |
| `DeleteByPrefix(ctx, prefix)` | Remove all keys with the given prefix (SCAN+DEL) |
| `SetJSON(ctx, key, value, ttl)` | Marshal struct to JSON, then store |
| `GetJSON(ctx, key, &target)` | Fetch and unmarshal JSON; returns `false, nil` on miss |
| `Close(ctx)` | Gracefully close the underlying connection |

---

## Observability

### Tracing

Enable per-command OpenTelemetry spans:

```go
Config{
    Tracing: struct{ Enabled bool }{Enabled: true},
}
```

Each command produces a span named `redis: <command>` with attributes:
- `db.system = redis`
- `db.operation = <command>`
- `db.statement = <full command string>`

### Logging

Enable structured per-command logs:

```go
Config{
    Logger: struct{ Enabled bool }{Enabled: true},
}
```

Logs include: operation, key, duration (ms), trace/span/session IDs, error (if any).
Pipeline commands are grouped into a single `REDIS PIPELINE` log entry.

---

## Cluster Notes

- `DeleteByPrefix` uses `SCAN` which is **per-shard** in cluster mode —
  `UniversalClient` handles cross-shard iteration automatically.
- `DB` selection is **ignored** in cluster mode (Redis Cluster only supports DB 0).
- `INCR` and `SetNX` are atomic on each shard; for cross-key atomicity use Lua scripts or transactions.

---

## Dependency

```
go-redis/v9                 — Redis client (UniversalClient supports all topologies)
go.opentelemetry.io/otel    — Tracing
go.uber.org/fx              — Lifecycle management
```
