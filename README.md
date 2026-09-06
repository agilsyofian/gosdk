# GO-SDK

> **Source of truth:** the checked-in Go source code is authoritative. The files under `.instructions/` describe how to change that implementation safely.

## 1. Project identity

`gosdk` is a modular Go enterprise SDK/core framework for building production microservices. It is organized as a Go workspace containing independent modules rather than one monolithic Go module.

The current implementation provides reusable building blocks for:

- standardized application errors;
- resilience policies: retry, timeout, rate limiting, circuit breaking;
- security: authentication, authorization, principals, sessions, and context propagation;
- storage abstractions and concrete Redis/MySQL/PostgreSQL/Oracle implementations;
- HTTP and gRPC client/server transport with middleware/interceptors;
- observability primitives for logging, tracing, and health checks;
- cryptographic utility functions;
- developer tooling for database migration and scaffolding.

Some directories exist only as package/documentation placeholders. Do **not** describe an unimplemented package as a working integration.

## 2. Read before changing code

Read these files in this order, selecting all files relevant to the change:

1. `.instructions/project-principles.md` — project-wide invariants.
2. `.instructions/architecture.md` — actual module boundaries and dependency direction.
3. `.instructions/package-rules.md` — package placement and import rules.
4. `.instructions/coding-style.md` — Go coding conventions.
5. `.instructions/api-guidelines.md` — public API compatibility rules.
6. `.instructions/security.md` — security requirements.
7. `.instructions/observability.md` — logging/tracing/health conventions.
8. `.instructions/testing.md` — test requirements.
9. `.instructions/roadmap.md` — current implementation status and intentionally unimplemented areas.

## 3. Repository map — current source

```text
gosdk/
├── core/
│   ├── errors/                     # AppError and HTTP status helpers
│   ├── resilience/                 # retry, timeout, ratelimit, circuitbreaker
│   └── observability/
│       ├── health/                 # health registry/checks
│       ├── logger/                 # Logger interface + zerolog implementation
│       ├── tracing/                # OpenTelemetry + context metadata
│       ├── audit/                  # placeholder
│       └── metrics/                # placeholder
├── security/
│   ├── authenticator/
│   │   ├── apikey/
│   │   ├── basicauth/
│   │   └── jwt/
│   ├── authorization/
│   ├── session/
│   │   ├── memory/
│   │   └── redis/
│   ├── signature/                  # HMAC/digital signature & response signing
│   ├── jwks/
│   └── (core security types, manager, registry, context, errors)
├── storage/
│   ├── cache/
│   │   ├── local/
│   │   └── redis/
│   ├── database/
│   │   ├── mysql/
│   │   ├── oracle/
│   │   └── postgres/
│   └── objectstorage/              # placeholder
│       ├── minio/
│       └── s3/
├── transport/
│   ├── client/
│   │   ├── grpc/
│   │   └── http/
│   ├── idempotency/                # shared standard engine & storage for HTTP/gRPC
│   └── server/
│       ├── grpc/                   # interceptors + security extraction
│       ├── http/                   # Gin server, middleware, DTO/security
│       └── websocket/              # placeholder
├── messaging/
│   ├── kafka/                      # Kafka consumer & producer
│   └── rabbitmq/                   # RabbitMQ consumer & producer
├── utils/
│   ├── concurrency/
│   └── crypto/
├── tools/
│   ├── dbmigration/
│   ├── mapgen/
│   └── scaffold/                   # not registered in go.work (WIP)
├── examples/                       # runnable example module
├── go.work
└── Makefile
```

There is **no current `app/`, `config/`, `foundation/`, or top-level `internal/` implementation**. Do not invent those packages when making changes.

## 4. Workspace and module model

The workspace is defined by `go.work`. Current workspace modules include:

- `core/errors`
- `core/observability/health`
- `core/observability/logger`
- `core/observability/tracing`
- `core/resilience`
- `examples`
- `messaging/kafka`
- `messaging/rabbitmq`
- `security`
- `security/authenticator/apikey`
- `security/authenticator/basicauth`
- `security/authenticator/jwt`
- `security/session/memory`
- `security/session/redis`
- `security/signature`
- `storage/cache`
- `storage/cache/local`
- `storage/cache/redis`
- `storage/database`
- `storage/database/mysql`
- `storage/database/oracle`
- `storage/database/postgres`
- `tools/dbmigration`
- `tools/mapgen`
- `transport/client/grpc`
- `transport/client/http`
- `transport/idempotency`
- `transport/server/grpc`
- `transport/server/http`
- `utils`

A directory is not automatically a Go module. Use the checked-in `go.mod`/`go.work` files as the authority.

## 5. Core engineering rules

1. **Source code wins.** Do not preserve stale documentation claims merely because they exist in an instruction file.
2. **Keep module boundaries explicit.** Do not add imports that create dependency cycles or bypass the layering rules.
3. **Use `context.Context` for I/O and cancellation-aware operations.**
4. **Preserve public APIs.** Additive changes are preferred; removals/renames require an explicit compatibility decision.
5. **Use the SDK's existing abstractions.** Prefer `errors.AppError`, `observability/logger.Logger`, `storage.Cache`, `storage.Database`, `security` interfaces, and existing resilience policies instead of duplicating them.
6. **Fail safely.** Authentication/authorization and security-sensitive failures must not become permissive fallbacks.
7. **Do not add secrets, credentials, tokens, production endpoints, or customer data to source/tests/docs.**
8. **Every behavior change needs appropriate tests.** Security-sensitive changes need negative/fail-closed coverage.
9. **Do not claim functionality that is only represented by an empty package/documentation file.** Metrics, audit, and WebSocket are currently placeholders.
10. **Prefer minimal, idiomatic Go.** Do not introduce frameworks or dependencies without a concrete need.

## 6. Validation before completion

At minimum, run the relevant module tests. For workspace-wide changes, validate the workspace with the available Go tooling and make targets. Also inspect `go.work` and module-specific `go.mod` files when dependency changes are involved.

Before declaring a change complete:

- [ ] Relevant `.instructions/` files were read.
- [ ] The implementation matches the actual package/module structure.
- [ ] Public API compatibility was considered.
- [ ] Tests were added/updated for changed behavior.
- [ ] Formatting and static analysis were run where available.
- [ ] No security-sensitive data was introduced.
- [ ] Documentation was updated when the public behavior or architecture changed.
- [ ] An ADR was added only when a genuine architecture decision was introduced.

## 7. When uncertain

Do not invent missing framework layers or integrations. If the requested change conflicts with the current source architecture, state the conflict and choose the smallest backward-compatible change, or ask for a decision when the trade-off cannot be resolved safely.
