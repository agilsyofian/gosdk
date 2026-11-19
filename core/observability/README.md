# Observability

`github.com/agilsyofian/gosdk/core/observability` is the unified observability foundation for the SDK, providing high-performance logging, distributed tracing, Kubernetes health checks, metrics, and audit primitives.

---

## Architecture Overview

All observability subpackages are unified under a single Go module to simplify dependency management, versioning, and workspace synchronization while maintaining clean, idiomatic subpackages:

```
SDK/core/observability/
├── go.mod                     # Module: github.com/agilsyofian/gosdk/core/observability
├── observability.go           # Unified bootstrap (Init, Shutdown, Config)
├── logger/                    # Structured JSON logging powered by zerolog
├── tracing/                   # OpenTelemetry distributed tracing & propagation
├── health/                    # Liveness, readiness, and startup health check registry
├── metrics/                   # Prometheus / OpenTelemetry metrics primitives
└── audit/                     # Audit event logging & audit trail sinks
```

---

## Subpackages

### 1. `logger` (`core/observability/logger`)
- High-performance, zero-allocation structured JSON logging based on `rs/zerolog`.
- Automatic redaction of sensitive parameters (PII, credentials, cards) via `FilterSensitiveData`.
- Correlation with distributed tracing: extracts and logs `trace_id` and `span_id` when available.

```go
import "github.com/agilsyofian/gosdk/core/observability/logger"

log := logger.NewZerologLogger(logger.DefaultConfig())
log.Info(ctx, logger.LogPayload{
    Message: "User authenticated successfully",
    Data:    map[string]interface{}{"user_id": "usr-123"},
})
```

### 2. `tracing` (`core/observability/tracing`)
- OpenTelemetry (OTel) tracer provider initialization with OTLP gRPC export.
- W3C Trace Context and Baggage propagation.
- Context helpers for `TraceID`, `SpanID`, and `SessionID`.

```go
import "github.com/agilsyofian/gosdk/core/observability/tracing"

tracer, err := tracing.NewTracingContext(ctx, tracing.Config{
    Enabled:      true,
    Endpoint:     "otel-collector:4317",
    SamplerRatio: 1.0,
    TLS:          true, // optional; default false keeps a plaintext connection
}, "my-service", "v1.0.0")
if err != nil {
    return err // e.g. a malformed endpoint; tracing is never silently disabled
}
defer tracer.Shutdown(context.Background())
```

`NewTracing(cfg, name, version)` is the same as `NewTracingContext` with
`context.Background()`. If the exporter cannot be created, both return an error
and leave the OpenTelemetry globals untouched.

| Config field   | Key             | Meaning                                                        |
|----------------|-----------------|----------------------------------------------------------------|
| `Enabled`      | `enabled`       | Turn tracing on; when false a no-op `Tracing` is returned       |
| `Endpoint`     | `endpoint`      | OTLP/gRPC collector address                                     |
| `SamplerRatio` | `sampler_ratio` | Fraction of traces sampled (0.0–1.0)                            |
| `TLS`          | `use_tls`       | Use TLS (system CA pool, TLS 1.2+) instead of plaintext         |

### 3. `health` (`core/observability/health`)
- Kubernetes-ready health check registry managing `liveness`, `readiness`, and `startup` probes.
- Built-in panic recovery for probe checks.
- Compatible with `/actuator/health` and `/healthz` endpoints.

```go
import "github.com/agilsyofian/gosdk/core/observability/health"

registry := health.NewRegistry()
registry.RegisterReadiness("database", func(ctx context.Context) error {
    return db.PingContext(ctx)
})
```

---

## Unified Bootstrap

For microservices that initialize logging, tracing, and health checks together:

```go
import "github.com/agilsyofian/gosdk/core/observability"

obs, err := observability.Init(observability.Config{
    Logger:  logger.DefaultConfig(),
    Tracing: tracing.Config{Enabled: true, Endpoint: "localhost:4317"},
}, "auth-service", "v1.0.0")
if err != nil {
    log.Fatalf("failed to initialize observability: %v", err)
}
defer obs.Shutdown(context.Background())

// Use individual primitives:
obs.Logger.Info(ctx, logger.LogPayload{Message: "Service started"})
obs.Health.RegisterLiveness("ping", health.AlwaysHealthy)
```
