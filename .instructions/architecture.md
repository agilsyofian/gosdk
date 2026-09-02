# Architecture

## 1. Current architecture

`gosdk` is a Go workspace composed of independent modules. The current implemented layers are:

```text
transport/server/*  ───────┐
transport/client/*  ───────┤
                           ├── security
                           ├── resilience
                           ├── observability/*
                           └── storage/*
security/session/redis ─────── storage/cache
security/authenticator/* ──── security
storage/* ─────────────────── observability
```

This is a practical dependency graph, not a requirement that every package import every lower layer. A package should depend only on the smallest abstraction it needs.

### Current important edges

- `security/authenticator/apikey` and `security/authenticator/basicauth` depend on `security`.
- `security/authenticator/jwt` depends on `security`, resilience, logging, and HTTP client functionality for JWKS-related work.
- `security/session/memory` depends on `security`; `security/session/redis` depends on `security`, `errors`, and `storage/cache`.
- `storage/cache/redis` implements `storage/cache` and uses logging/tracing.
- database drivers implement `storage/database` and use logging/tracing.
- HTTP/gRPC clients use resilience and observability helpers; the gRPC client also provides JWT bearer authorization.
- HTTP/gRPC servers use errors, observability, and security-related middleware/interceptors.

Do not create reverse dependencies that make lower-level abstractions depend on transport or application code.

## 2. Current module responsibilities

### `errors`

Centralized application error representation (`AppError`) and HTTP status classification.

### `resilience/*`

Small policies exposing `Execute(ctx, fn)` for timeout, retry, rate limiting, and circuit breaking.

### `security`

Core credential, authentication, principal, session, security-context, registry, manager, and authorization interfaces/types.

### `security/authenticator/*`

Concrete authentication mechanisms: API key, Basic Auth, and JWT/JWKS.

### `security/session/*`

Concrete session managers: in-memory and Redis-backed.

### `security/authorization`

Role/permission authorization implementation.

### `storage/cache`

Cache abstraction. `storage/cache/redis` supplies the Redis implementation.

### `storage/database`

Database abstraction. `mysql`, `postgres`, and `oracle` supply concrete implementations.

### `observability/logger`

Logger interface and zerolog implementation, including sensitive-data filtering.

### `observability/tracing`

OpenTelemetry tracing setup plus application/trace/session context metadata.

### `observability/health`

Named health checks and registry aggregation.

### `transport/client/http`

HTTP client with request options, middleware, logging, tracing, retry, and circuit-breaker integration.

### `transport/client/grpc`

gRPC client with configurable interceptors and retry/circuit-breaker/tracing/logging/JWT authorization helpers.

### `transport/server/http`

Gin-based HTTP server with middleware, DTO helpers, health/actuator support, and security extraction/middleware.

### `transport/server/grpc`

gRPC server with interceptors, security extraction, registration, start/shutdown, and server options.

### `tools/dbmigration`

Database migration command/tool and internal migration implementation.

### `tools/scaffold`

Scaffolding command/tool. Keep generator behavior isolated from runtime SDK packages.

### `utils/crypto`

AES-GCM encryption/decryption and key generation helpers.

## 3. Placeholder packages

The following directories currently contain package documentation/placeholders rather than complete integrations:

- `messaging/kafka`
- `messaging/rabbitmq`
- `observability/audit`
- `observability/metrics`
- `transport/server/websocket`
- `storage/objectstorage` and its backend directories currently have no Go implementation in the source snapshot.

Do not write instructions that assume these are production-ready integrations.

## 4. Concurrency model

Stateful implementations such as in-memory sessions and resilience policies must be safe according to their existing implementation. New shared mutable state must use appropriate synchronization or immutable ownership. Avoid introducing goroutine leaks or background loops without explicit lifecycle management.

## 5. Examples and documentation

`examples/` is the current example module. `xdocs/` contains architecture/ADR/example documentation. Use these as supporting documentation, while keeping the implementation as the final authority.

## 6. Architecture decisions

Use an ADR when changing module boundaries, introducing a new cross-cutting pattern, changing a public architectural contract, or introducing a substantial dependency. Do not create ADRs for ordinary bug fixes or local refactors.
