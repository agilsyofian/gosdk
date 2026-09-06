# HTTP Server Transport (`gosdk/transport/server/http`)

The `github.com/agilsyofian/gosdk/transport/server/http` module is an enterprise-grade, Gin-based HTTP server transport designed for fintech/banking standards, high-throughput microservices, and cloud-native Kubernetes workloads. It delivers out-of-the-box pentest security hardening, Kubernetes-compliant graceful shutdown, integrated distributed tracing & observability, distributed idempotency, and secure transport (HMAC signature verification & response signing).

---

## Key Features

- 🛡️ **Security & Pentest Hardening (Out-of-the-Box)**:
  - **CORS Middleware**: Preflight handling, dynamic origin reflection, and automatic `Vary: Origin` injection.
  - **Block Method Override**: Prevents HTTP verb tampering via `X-HTTP-Method-Override`, `X-Method-Override`, `X-HTTP-Method` headers, and `_method` query parameter.
  - **Anti-XSS & SQLi in JSON**: Recursive string validation on JSON payloads with smart exemptions for valid AES ciphertexts (Hex / Base64).
  - **Industry-Standard Security Headers**: Automatically injects `HSTS`, `Content-Security-Policy (CSP)`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff`, `Referrer-Policy`, and `Cache-Control: no-store`.
  - **Secure Transport (HMAC Signature & Response Signing)**: Verifies request payload integrity and signs outgoing responses using HMAC-SHA256, HMAC-SHA512, or RSA-SHA256.
- 🔐 **Declarative Security & Authentication Ergonomics**:
  - Declarative configuration in `env.yaml` under `server.http.security` (`required`, `enable_bearer`, `enable_basic`, `enable_api_key`, `api_key_header`, `session_cookie`).
  - Seamless server binding via `server.SetSecurityManager(mgr)`.
  - Flexible route-group and handler protection: `server.SecurityMiddleware()` and `server.RequirePermission("...")`.
  - Package-level helpers: `http.Security(mgr, ...)` and `http.RequirePermission(mgr, ...)`.
  - Standardized error mapping: rejects unauthenticated requests with `401 Unauthorized` and forbidden access with `403 Forbidden` mapped into SDK standard error envelope.
- 🔁 **Distributed Idempotency**:
  - In-flight concurrency locking via singleflight (prevents double-clicks and retry storms from executing handlers more than once).
  - Automatic response caching & replay (`X-Cache-Lookup: HIT`, `X-Idempotent-Replayed: true`).
  - Pluggable canonical key generator (`WithCanonicalKeyFunc`, `WithHeader`, etc.).
- ☸️ **Kubernetes Lifecycle & Graceful Shutdown**:
  - Instant HTTP 503 on health check probes during shutdown to promptly signal Ingress/Kubernetes to divert incoming traffic.
  - Bounded graceful drain timeout with context cancellation when the deadline expires.
  - Separated Kubernetes probes: `/health` (composite), `/health/liveness`, and `/health/readiness`.
- 📊 **Observability & Performance**:
  - Distributed tracing correlation (`X-Trace-Id`, `X-Session-Id`, `X-Request-Id`).
  - Dynamic logger route exclusions (regex, prefix, subpath) to eliminate memory allocations on frequent health checks.
  - Standardized JSON Response Envelope (`dto.BaseResponse`) and unified error mapping.

---

## Configuration Guide (`env.yaml`)

HTTP server options can be configured declaratively via YAML configuration (e.g. `env.yaml`):

```yaml
server:
  http:
    base_path: "/example"
    addr: ":8001"
    read_timeout: "60s"
    write_timeout: "60s"
    idle_timeout: "120s"
    shutdown_timeout: "5s"
    shutdown_delay: "5s"
    mode: "debug" # debug, release, test

    # 1. CORS Configuration
    enable_cors: true
    cors:
      allow_origins:
        - "*"
      allow_methods:
        - "GET"
        - "POST"
        - "PUT"
        - "PATCH"
        - "DELETE"
        - "HEAD"
        - "OPTIONS"
      allow_headers:
        - "Origin"
        - "Content-Type"
        - "Accept"
        - "Authorization"
        - "X-Requested-With"
        - "X-Session-Id"
        - "X-Trace-Id"
        - "X-Request-Id"
        - "X-Idempotency-Key"
        - "X-Signature"
        - "X-Timestamp"
        - "X-Client-Id"
      expose_headers:
        - "Content-Length"
        - "X-Session-Id"
        - "X-Trace-Id"
        - "X-Request-Id"
        - "X-Cache-Lookup"
        - "X-Idempotent-Replayed"
        - "X-Signature"
        - "X-Timestamp"
      allow_credentials: false
      max_age: "12h"

    # 2. Pentest Hardening
    disable_block_method_override: false
    disable_block_special_chars: false
    allowed_special_chars_regex: "^[a-zA-Z0-9., '/_@:-]*$"

    # 3. Security Headers
    security_headers:
      disabled: false
      hsts_max_age: "4320h" # 180 days
      disable_hsts_subdomains: false
      disable_hsts_preload: false
      content_security_policy: "frame-ancestors 'none'"
      frame_options: "DENY"
      referrer_policy: "strict-origin-when-cross-origin"
      disable_no_cache: false
      sanitize_response: true

    # 4. Declarative Request Metadata & Dynamic Baggage
    metadata:
      trace_id_header: "X-Trace-Id"         # Customizable for Company A/B (e.g. Traceparent)
      span_id_header: "X-Span-Id"
      request_id_header: "X-Request-Id"
      session_id_header: "X-Session-Id"
      user_id_header: "X-User-Id"
      capture_headers:
        - "X-Device-Id"
        - "X-App-Version"
        - "X-Tenant-Id"
        - "X-Channel-Id"

    # 5. Idempotency Configuration
    idempotency:
      enabled: true
      ttl: "1m"
      lock_timeout: "10s"
      header_name: "X-Idempotency-Key"

    # 6. Secure Transport (HMAC Signature & Response Signing)
    signature:
      enabled: true
      algorithm: "HMAC-SHA256" # HMAC-SHA256, HMAC-SHA512, RSA-SHA256
      encoding: "hex"          # hex, base64
      signature_header: "X-Signature"
      timestamp_header: "X-Timestamp"
      key_id_header: "X-Client-Id"
      max_clock_drift: "5m"
      sign_response: true
      response_signature_header: "X-Signature"
      response_timestamp_header: "X-Timestamp"
      secret: "my-secret-key-12345"

    # 7. Declarative Security & Authentication
    security:
      required: true
      enable_bearer: true
      enable_basic: false
      enable_api_key: true
      api_key_header: "X-API-Key"
      session_cookie: "SESSION_ID"

    # 8. Observability & Actuator
    enable_swagger: true
    show_error_detail: true
    logger_exclude_routes:
      - "/actuator"
      - "/favicon.ico"
    actuator:
      enabled: true
      path: "/actuator"
      health_path: "/health"
      liveness_path: "/health/liveness"
      readiness_path: "/health/readiness"
```

---

## Usage Guide

### 1. Server Initialization (with Uber Fx)

Initialize the server cleanly via Uber Fx dependency injection:

```go
package main

import (
    "github.com/agilsyofian/gosdk/core/observability/health"
    "github.com/agilsyofian/gosdk/core/observability/tracing"
    "github.com/agilsyofian/gosdk/core/observability/logger"
    "github.com/agilsyofian/gosdk/transport/server/http"
    "go.uber.org/fx"
)

func main() {
    fx.New(
        fx.Provide(
            NewConfig,
            logger.New,
            tracing.New,
            health.New,
            http.NewServer,
        ),
        fx.Invoke(RegisterRoutes),
    ).Run()
}
```

---

### 2. Route Registration & `BasePath` Support

The `*http.Server` struct directly embeds `gin.IRouter`. Any route methods registered on the server instance are automatically routed under the configured `base_path`:

```go
func RegisterRoutes(server *http.Server) {
    // Automatically routed under base_path (e.g. /example/v1/users)
    v1 := server.Group("/v1")
    {
        v1.GET("/users", userHandler.List)
        v1.POST("/users", userHandler.Create)
    }

    // To register directly at the root URL (bypassing base_path):
    server.Router().GET("/healthcheck", rootHealthHandler)
}
```

---

### 3. Using Idempotency Middleware

Attach `http.Idempotency` to transactional endpoints (e.g., payments, transfers, or order placements):

```go
import (
    "time"
    "github.com/agilsyofian/gosdk/transport/server/http"
    "github.com/gin-gonic/gin"
)

// Using cache storage (Redis or in-memory)
r.POST("/orders",
    http.Idempotency(cacheStorage,
        // Optional Canonical Key Customization
        http.WithCanonicalKeyFunc(func(c *gin.Context, body []byte) (string, error) {
            if key := c.GetHeader("X-Idempotency-Key"); key != "" {
                return "order:" + key, nil
            }
            return http.DefaultCanonicalKey(c, body)
        }),
        http.WithIdempotencyTTL(2 * time.Minute),
    ),
    orderHandler.Create,
)
```

**Behavior**:
- **First Request**: Executes normally; response status, headers, and body are cached.
- **Concurrent In-Flight Request**: Waits synchronously without busy polling until the first request completes, then returns the identical response.
- **Replayed Request**: Immediately returns the cached response with headers `X-Cache-Lookup: HIT` and `X-Idempotent-Replayed: true`.

---

### 4. Using Secure Transport (HMAC Signature & Response Signing)

For inter-system or third-party partner integrations requiring request/response cryptographic integrity:

```go
import (
    "github.com/agilsyofian/gosdk/security/signature"
    "github.com/agilsyofian/gosdk/transport/server/http"
)

// Initialize Secret Provider (Static, Map, or Dynamic Func from DB/Vault)
secretProvider := http.StaticSecret([]byte("partner-secret-key"))

r.POST("/disbursement",
    http.Signature(secretProvider,
        http.WithSignatureAlgorithm(signature.AlgorithmHMACSHA256),
        http.WithSignatureResponseSigning(true), // Automatically sign outgoing response
    ),
    disbursementHandler.Process,
)
```

**Behavior**:
- Verifies `X-Signature`, `X-Timestamp`, and `X-Client-Id` headers against the canonical string-to-sign.
- Rejects requests exceeding the allowed clock skew (`MaxClockDrift`).
- Injects verified `client_id` and `signature_key_id` into the Gin context.
- Automatically signs outgoing HTTP responses, attaching `X-Signature` and `X-Timestamp` response headers.

---

### 5. Using Declarative Security & RBAC Middleware

Bind your `security.SecurityManager` to the server and apply authentication/authorization middleware cleanly:

```go
func SetupRoutes(server *http.Server, securityMgr security.SecurityManager) {
    // 1. Bind SecurityManager to HTTP Server (defaults to server.http.security in env.yaml)
    server.SetSecurityManager(securityMgr)

    v1 := server.Group("/v1")

    // Public routes: No authentication middleware
    auth := v1.Group("/auth")
    {
        auth.POST("/login", authHandler.Login)
        auth.POST("/register", authHandler.Register)
    }

    // Protected routes: Authenticated via server.SecurityMiddleware()
    user := v1.Group("/user", server.SecurityMiddleware())
    {
        user.GET("/profile", userHandler.GetProfile)

        // Granular permission check via server.RequirePermission()
        user.DELETE("/profile", server.RequirePermission("user:delete"), userHandler.DeleteProfile)
    }
}
```

**Accessing Authenticated Identity in Handlers**:
```go
func (h *UserHandler) GetProfile(c *gin.Context) {
    // Retrieve authenticated SecurityContext from request context
    secCtx, ok := security.FromContext(c.Request.Context())
    if ok && secCtx.Principal != nil {
        userID := secCtx.Principal.Subject
        // Business logic with userID
    }
}
```

**Package-Level Helpers**:
You can also use package-level middleware functions directly on any Gin engine or group:
```go
// Authentication middleware with custom override config
r.Use(http.Security(securityMgr, http.SecurityConfig{
    Required:     true,
    EnableBearer: true,
}))

// Authorization middleware
r.DELETE("/admin/users/:id", http.RequirePermission(securityMgr, "admin:delete"), adminHandler.Delete)
```

---

### 6. Actuator & Health Probes (Kubernetes)

The HTTP server automatically registers standard Actuator endpoints:

| Endpoint | Purpose | Standard Response |
|---|---|---|
| `GET /actuator/health` | Composite health status (Liveness + Readiness) | `200 OK` (UP) / `503 Service Unavailable` (DOWN) |
| `GET /actuator/health/liveness` | Kubernetes Liveness Probe | `200 OK` (UP) / `503 Service Unavailable` (DOWN) |
| `GET /actuator/health/readiness` | Kubernetes Readiness Probe | `200 OK` (UP) / `503 Service Unavailable` (DOWN) |
| `GET /actuator/info` | Application metadata | `200 OK` with app name and mode |

*Note: When a shutdown signal (SIGTERM/SIGINT) is received, all probe endpoints immediately return `503 Service Unavailable` before listener teardown, allowing ingress controllers to remove the pod from active traffic.*

---

## Middleware Pipeline Architecture

The HTTP Server executes middlewares in a strictly defined order:

```text
Incoming Request
       │
       ▼
1. CORS Middleware (Handle OPTIONS preflight & inject Vary: Origin)
       │
       ▼
2. Recovery Middleware (Catch panics & format standardized error response)
       │
       ▼
3. Tracer Middleware (Extract OpenTelemetry trace context)
       │
       ▼
4. Metadata Middleware (Extract Client IP, Device, Session, & Request IDs)
       │
       ▼
5. Block Method Override (Block X-HTTP-Method-Override & query _method)
       │
       ▼
6. Block Special Characters (Scan for XSS / SQLi in JSON string payloads)
       │
       ▼
7. Security Headers (Inject HSTS, CSP, X-Frame-Options, Nosniff, Cache-Control)
       │
       ▼
8. Logger Middleware (Structured logging with dynamic route exclusions)
       │
       ▼
9. Route Handlers & Route-Level Security (SecurityMiddleware / RequirePermission / Idempotency / Signature)
```

---

## Running Tests

Run the complete test suite for the HTTP transport module and all its middlewares:

```bash
cd SDK/transport/server/http
go test -v ./...
```
