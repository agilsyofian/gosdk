# Roadmap / Current Status

This file is intentionally aligned with the **current source snapshot**. It is not a promise that every directory name in the repository represents a finished feature.

## 1. Status legend

- **Implemented** — source code and tests are present.
- **Placeholder** — package/directory exists, but there is no functional integration in the current snapshot.
- **Future** — may be implemented later; do not assume it exists.

## 2. Implemented today

### Core

- `errors`: structured application errors (`AppError`) and HTTP status helpers.
- `resilience`: timeout, retry, rate limit, and circuit breaker policies (`sony/gobreaker`, token-bucket rate limiter, exponential backoff).

### Security

- core authentication, authorization, session, and principal abstractions (`security`);
- API key authenticator (`security/authenticator/apikey`);
- Basic Auth authenticator (`security/authenticator/basicauth`);
- JWT authenticator, claims parsing/validation, token issuance (`jwt.Issue`), and PKCS#1/PKCS#8 RSA key loader (`jwt.B64ToPrivateKey`);
- JWKS manager (`security/jwks`): remote JWKS endpoint fetching with caching, TTL expiration, singleflight deduplication, retry, and circuit breaking;
- role/permission authorizer (`security/authorization`);
- in-memory session manager (`security/session/memory`);
- Redis session manager (`security/session/redis`);
- security context, principal extraction, and context propagation utilities.

### Storage

- cache abstraction (`storage/cache`) + Redis implementation (`storage/cache/redis`);
- database abstraction (`storage/database`) + MySQL, PostgreSQL, and Oracle implementations with transaction support.

### Transport

- HTTP client (`transport/client/http`): resilient HTTP client with retry, timeout, circuit breaker, logging, and tracing middleware;
- gRPC client (`transport/client/grpc`): gRPC client factory with interceptor chaining and `AuthorizationBearer` interceptor supporting dynamic `TokenProvider` callbacks for token caching and auto-renewal;
- HTTP server (`transport/server/http`): Gin-based engine with middleware for errors, structured request logging, OpenTelemetry tracing, and metadata extraction (supporting trace, session, device, and field-level encryption headers: `X-Encryption-Version`, `X-Encryption-Kid`, `X-Encryption-Key`, `X-Nonce`, `X-Timestamp`);
- gRPC server (`transport/server/grpc`): gRPC server with interceptor chain and security context extraction.

### Messaging

- `messaging/kafka`: Kafka consumer & producer implementation using `segmentio/kafka-go` with message envelope, retry policies, dead-letter support, concurrent batch/single processing, OpenTelemetry tracing, structured logging, and health checks;
- `messaging/rabbitmq`: RabbitMQ consumer & producer implementation using `amqp091-go` with automated exchange/queue topology binding, acknowledgement semantics, retries, OpenTelemetry tracing, structured logging, and health checks.

### Observability

- zerolog-backed logger (`core/observability/logger`);
- OpenTelemetry tracing and context propagation (`observability/tracing`);
- health-check registry and indicators (`observability/health`).

### Utilities & Tooling

- `utils/crypto`:
  - application-layer envelope encryption/decryption (AES-256-GCM + RSA-OAEP-256);
  - field-level encryption and decryption (`FieldEncryptor`, `FieldDecryptor`, `FieldSession`) per specification with embedded nonce and timestamp verification;
  - key providers: `StaticKeyProvider`, `JWKSKeyProvider` (static config-backed with private key binding), and `RemoteJWKSKeyProvider` (dynamic public key resolution via `PublicKeyFetcher` interface, local private key binding, supporting rotation and revocation);
  - replay protection (`ReplayStore`, `InMemoryReplayStore`, `ReplayProtector`) with freshness window and nonce uniqueness check;
  - strict zero external dependencies (standard library only);
- `utils/concurrency`: concurrency primitives and helpers;
- `utils/mapgen`: code generation tool for struct-to-map mappings;
- `tools/dbmigration`: database migration CLI tool;
- `tools/scaffold`: microservice scaffolding CLI tool;
- `examples`: fully wired, runnable microservice using Uber Fx:
  - config loader with Viper and validation;
  - auth module with JWKS endpoint serving, password login with transparent field-level payload decryption (`username` and `password` decrypted via `FieldDecryptor` using `AuthServiceDeps`), JWT session lifecycle, and logout;
  - user module (Gin HTTP and gRPC service implementations);
  - outbound gRPC client with auto-refreshing bearer token interceptor (`tokenmanagement`);
  - crypto bridge adapter (`cryptoadapter.JWKSManagerAdapter`) connecting `security/jwks.Manager` to `utils/crypto.PublicKeyFetcher` without violating package boundaries.

## 3. Placeholder

The following are present structurally but do not have a functional implementation in the current source snapshot:

- `observability/metrics`;
- `observability/audit`;
- `transport/server/websocket`;
- `storage/objectstorage` and its backend directories (`minio`, `s3`).

Do not add code that assumes these APIs already exist.

## 4. Not part of the current implementation

Older guidance may mention `app/`, `config/`, `foundation/*`, a generic top-level `internal/`, or a unified SDK wrapper. Those are not current source modules. New work should not target them unless the repository is intentionally being re-architected.

## 5. How agents should use this file

Before implementing a requested feature:

1. Check whether the target is already implemented.
2. If it is a placeholder, determine whether the user explicitly wants a new feature rather than assuming it is already available.
3. Preserve existing module boundaries and APIs.
4. Update this file only when the implementation status genuinely changes.

## 6. Open architectural decisions

No future integration should be treated as approved merely because its directory exists. New cross-cutting integrations, new workspace modules, or changes to dependency direction should be documented through an ADR when the decision is substantive.
