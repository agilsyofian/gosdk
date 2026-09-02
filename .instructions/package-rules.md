# Package Rules

## 1. Put code where the current architecture puts it

Use the existing top-level domains:

- `errors` — application errors/statuses.
- `observability/*` — logging, tracing, health, and future observability integrations.
- `resilience/*` — retry/timeout/rate-limit/circuit-breaker policies.
- `security/*` — authentication, authorization, principals, sessions, and security context.
- `storage/*` — cache/database abstractions and concrete backends.
- `transport/client/*` — outbound HTTP/gRPC communication.
- `transport/server/*` — inbound HTTP/gRPC server behavior.
- `tools/*` — developer/CLI tooling, not runtime framework APIs.
- `utils/*` — narrowly scoped reusable utilities.

Do not introduce an `app`, `config`, `foundation`, or generic `internal` layer merely because an old document mentions one.

## 2. Abstractions live above implementations

A reusable interface should live in the package whose consumers need the abstraction. Concrete adapters belong in their backend package.

Examples:

- `storage.Cache` in `storage/cache`; Redis in `storage/cache/redis`.
- `storage.Database` in `storage/database`; MySQL/PostgreSQL/Oracle in their own modules.
- `security.Authenticator` in `security`; API key/Basic/JWT implementations in `security/authenticator/*`.
- `security.SessionManager` in `security`; memory/Redis implementations in `security/session/*`.

## 3. Avoid sideways imports

Do not introduce dependencies between unrelated concrete backends. A Redis implementation should not import a PostgreSQL implementation, and a database driver should not import HTTP server packages.

## 4. Module boundaries matter

If a new dependency is needed, add it to the specific `go.mod` that owns the code. Update `go.work` only when adding/removing a workspace module.

## 5. Package naming

Use short, lowercase Go package names. Keep package names aligned with directory names except where the current code already establishes a different package name (for example `transport/server/http/security` uses package `security`).

## 6. One concern per leaf package

A leaf package should have a focused responsibility. Avoid adding unrelated helpers to an existing package simply because it is convenient.

## 7. Internal code

There is no top-level `internal/` package in the current source tree. `tools/dbmigration/internal` is intentionally private to the migration tool and must remain so.

## 8. New backends

For a new backend, first define/confirm the stable abstraction in the parent package, then implement the adapter in a separate leaf package/module. Add tests without requiring the real backend for ordinary unit-test execution.

## 9. Generated/example code

Keep `xexamples/` and generated protobuf code isolated from runtime package design. Do not make production packages depend on example packages.
