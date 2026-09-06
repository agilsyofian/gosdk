# Architecture

## 1. Current architecture

`gosdk` is a Go workspace composed of independent modules. The current implemented layers are:

```text
transport/
├── idempotency (standard engine) ────────┐
├── server/*  ────────────────────────────┤
├── client/*  ────────────────────────────┤
                                          ├── security
                                          ├── storage/*
                                          └── core/*

security/
├── session/
    ├── memory ────────────────────────────── security
├── session/
    ├──redis ──────────────────────────────── security + storage/cache
├── authenticator/* ───────────────────────── security
├── signature ─────────────────────────────── (zero external deps)
├── * ─────────────────────────────────────── core/observability/*

storage/
├── cache                (interface)
│   ├── local            (in-process)
│   └── redis ───────────────────────────── core/observability/*
├── database             (interface)
│   ├── mysql  ──────────────────────────── core/observability/*
│   ├── postgres ────────────────────────── core/observability/*
│   └── oracle ──────────────────────────── core/observability/*
└── objectstorage        (placeholder)
    ├── minio
    └── s3

messaging/
├── kafka       (confluent-kafka / segmentio)
└── rabbitmq    (amqp)

utils/          (golang.org/x/sync only)
├── concurrency
└── crypto

tools/          (dev-time only, not imported by runtime packages)
├── dbmigration
├── mapgen
└── scaffold

core/
  ├── errors                (zero external deps)
  ├── resilience/*          (zero external deps)
  └── observability/
        ├── health          (zero external deps)
        ├── tracing         (zero external deps)
        └── logger          (zerolog only)
```

The `core/` layer is the foundation: packages here have zero or near-zero external dependencies and must never import from `transport`, `security`, `storage`, `messaging`, or `tools`.

`tools/` contains dev-time CLI utilities only. They must never be imported by any runtime SDK package.

This is a practical dependency graph, not a requirement that every package import every lower layer. A package should depend only on the smallest abstraction it needs.

### Current important edges

- `security/authenticator/apikey` and `security/authenticator/basicauth` depend on `security`.
- `security/authenticator/jwt` depends on `security`, `core/resilience`, `core/observability/logger`, and HTTP client functionality for JWKS-related work.
- `security/session/memory` depends on `security`; `security/session/redis` depends on `security`, `core/errors`, and `storage/cache`.
- `storage/cache/redis` implements `storage/cache` and uses `core/observability/logger` and `core/observability/tracing`.
- database drivers implement `storage/database` and use `core/observability/logger` and `core/observability/tracing`.
- HTTP/gRPC clients use `core/resilience` and `core/observability/*`; the gRPC client also provides JWT bearer authorization.
- HTTP/gRPC servers use `core/errors`, `core/observability/*`, and security-related middleware/interceptors.
- HTTP and gRPC servers both use `transport/idempotency` for standardized, transport-agnostic idempotency storage, concurrency singleflight locking, and response caching.
- `transport/idempotency` has zero external dependencies and defines the standard `Storage` interface (which matches `storage/cache.Cache`).
- **`core/observability/logger` does not depend on `core/observability/tracing`**. Callers are responsible for populating `LogPayload.TraceId`, `SessionId`, and `SpanId` before logging.

Do not create reverse dependencies that make lower-level abstractions depend on transport or application code.

## 2. Current module responsibilities

### `core/errors`

Centralized application error representation (`AppError`) and HTTP status classification. Zero external dependencies.

### `core/resilience/*`

Small policies exposing `Execute(ctx, fn)` for timeout, retry, rate limiting, and circuit breaking. Zero external dependencies.

### `core/observability/logger`

Logger interface (`Logger`), `LogPayload` struct, zerolog implementation, and sensitive-data filtering. Depends only on `zerolog`. Does **not** read trace/session data from context — callers must populate `LogPayload` fields before calling the logger.

### `core/observability/tracing`

OpenTelemetry tracing setup plus context helpers for `app_name`, `trace_id`, and `session_id`. Zero external dependencies beyond the OTel SDK.

### `core/observability/health`

Named health checks and registry aggregation. Zero external dependencies.

### `security`

Core credential, authentication, principal, session, security-context, registry, manager, and authorization interfaces/types.

### `security/authenticator/*`

Concrete authentication mechanisms: API key, Basic Auth, and JWT/JWKS.

### `security/session/*`

Concrete session managers: in-memory and Redis-backed.

### `security/authorization`

Role/permission authorization implementation.

### `security/signature`

Cryptographic request signing and verification engine (HMAC-SHA256, HMAC-SHA512, RSA-SHA256, Hex/Base64). Provides pluggable `Canonicalizer`, `ResponseCanonicalizer`, and `SecretProvider` interfaces with zero external dependencies. Used by transport middleware for Secure Transport and response signing.

### `storage/cache`

Cache abstraction. `storage/cache/local` supplies the in-process implementation; `storage/cache/redis` supplies the Redis implementation.

### `storage/database`

Database abstraction. `mysql`, `postgres`, and `oracle` supply concrete implementations.

### `transport/client/http`

HTTP client with request options, middleware, logging, tracing, retry, and circuit-breaker integration.

### `transport/client/grpc`

gRPC client with configurable interceptors and retry/circuit-breaker/tracing/logging/JWT authorization helpers.

### `transport/server/http`

Gin-based HTTP server with middleware, DTO helpers, health/actuator support, and security extraction/middleware.

### `transport/server/grpc`

gRPC server with interceptors, security extraction, registration, start/shutdown, and server options.

### `transport/idempotency`

Standard, transport-agnostic idempotency engine. Provides the `Storage` interface, in-memory storage fallback, agnostic `Record` caching model, in-flight concurrency singleflight locking, and canonical key utilities. Has zero external dependencies and serves as the common standard for both HTTP middleware and gRPC interceptors.

### `messaging/kafka`

Kafka producer and consumer implementation. Placeholder — not yet production-ready (see §3).

### `messaging/rabbitmq`

RabbitMQ producer and consumer implementation. Placeholder — not yet production-ready (see §3).

### `utils`

Shared runtime helpers with minimal dependencies (`golang.org/x/sync` only).

- `utils/concurrency` — singleflight and worker-pool helpers.
- `utils/crypto` — AES-GCM encryption/decryption and key generation helpers.

### `tools/dbmigration`

Database migration CLI tool. Dev-time only; must not be imported by runtime SDK packages.

### `tools/mapgen`

Code-generation tool for producing mapping boilerplate. Dev-time only.

### `tools/scaffold`

Scaffolding CLI tool. Dev-time only; keep generator behavior isolated from runtime SDK packages.

### `examples`

Runnable example module demonstrating SDK usage patterns. Not imported by any SDK package; exists solely for documentation and integration reference.

## 3. Placeholder packages

The following directories currently contain package documentation/placeholders rather than complete integrations:

- `messaging/kafka` and `messaging/rabbitmq` — implementation exists but is not production-hardened.
- `core/observability/audit` and `core/observability/metrics` — no implementation yet.
- `transport/server/websocket` — no implementation yet.
- `storage/objectstorage` and its backend directories — no Go implementation in the source snapshot.
- `tools/scaffold` — not registered in `go.work`; treat as work-in-progress.

Do not write instructions that assume these are production-ready integrations.

## 4. Concurrency model

Stateful implementations such as in-memory sessions and resilience policies must be safe according to their existing implementation. New shared mutable state must use appropriate synchronization or immutable ownership. Avoid introducing goroutine leaks or background loops without explicit lifecycle management.

## 5. Examples and documentation

`examples/` is the current example module. `xdocs/` contains architecture/ADR/example documentation. Use these as supporting documentation, while keeping the implementation as the final authority.

## 6. Architecture decisions

Use an ADR when changing module boundaries, introducing a new cross-cutting pattern, changing a public architectural contract, or introducing a substantial dependency. Do not create ADRs for ordinary bug fixes or local refactors.
