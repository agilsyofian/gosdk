# JWKS / RS256 Security Implementation

## Objective

Implement production-grade JWT verification using **RS256 + JWKS (JSON Web Key Set)** in the Go SDK security module.

The implementation MUST support:

- RS256 JWT verification
- Configurable JWKS URI
- Configurable JWT issuer
- Configurable JWT audience
- `kid` based public key selection
- Local in-memory JWKS cache
- JWKS refresh
- Automatic refresh when an unknown `kid` is encountered
- Key rotation
- HTTP and gRPC usage
- Gin integration for HTTP
- Transport-agnostic security core
- No Redis dependency for JWKS cache
- No private key exposure
- Strict JWT algorithm validation
- Protection against `jku` / arbitrary JWKS URL attacks

The implementation must be suitable for high-TPS microservices running on Kubernetes / OpenShift.

---

# 1. Architecture

The architecture MUST follow:

```text
                         Auth / Identity Service
                                  │
                           Private RSA Key
                                  │
                                  ▼
                              Sign JWT
                                  │
                                  ▼
                                 JWT
                                  │
              ┌───────────────────┼───────────────────┐
              │                   │                   │
              ▼                   ▼                   ▼
          Service A           Service B           Service C
              │                   │                   │
              └───────────────────┼───────────────────┘
                                  │
                           GET configured JWKS URI
                                  │
                                  ▼
                       /.well-known/jwks.json
                                  │
                                  ▼
                              Public Keys
                                  │
                                  ▼
                           Local Memory Cache
                                  │
                                  ▼
                            Verify RS256
```

The JWKS endpoint is public by design.

The service MUST NOT retrieve JWKS on every request.

---

# 2. Important Security Rules

## 2.1 Only RS256 is allowed

The JWT verifier MUST explicitly allow:

```text
RS256
```

The implementation MUST reject:

```text
HS256
HS384
HS512
RS384
RS512
none
```

unless explicitly added in a future implementation.

Do NOT dynamically trust the `alg` header.

Example:

```go
if token.Method != jwt.SigningMethodRS256 {
    return error
}
```

The exact implementation may depend on the chosen JWT library.

---

# 2.2 Never trust `jku`

The JWT header MUST NOT control the JWKS URL.

Example malicious JWT:

```json
{
  "alg": "RS256",
  "kid": "attacker-key",
  "jku": "https://attacker.example.com/jwks.json"
}
```

The implementation MUST NOT fetch:

```text
https://attacker.example.com/jwks.json
```

The only trusted JWKS URI is the URI configured through application configuration.

---

# 2.3 JWKS URI MUST be configurable

Do NOT hardcode:

```text
/.well-known/jwks.json
```

The configuration MUST support arbitrary valid HTTPS paths.

Examples:

```text
https://auth.example.com/.well-known/jwks.json
https://identity.example.com/oauth2/jwks
https://gateway.example.com/security/jwks.json
```

Configuration:

```go
type JWTConfig struct {
    JWKSURL string
}
```

The application MUST be able to configure this through environment variables / ConfigMap.

Example:

```text
JWT_JWKS_URL=https://auth.example.com/.well-known/jwks.json
```

OpenShift / Kubernetes ConfigMap may contain:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: security-config
data:
  JWT_ISSUER: "https://auth.example.com"
  JWT_AUDIENCE: "payment-service"
  JWT_JWKS_URL: "https://auth.example.com/.well-known/jwks.json"
```

The ConfigMap stores configuration only.

It MUST NOT contain the JWKS public key itself.

---

# 3. JWT Configuration

Create or extend the JWT configuration.

Recommended structure:

```go
type JWTConfig struct {
    Enabled bool

    Issuer   string
    Audience string

    JWKSURL string

    CacheTTL       time.Duration
    RefreshTimeout time.Duration
}
```

Recommended defaults:

```text
CacheTTL = 1 hour
RefreshTimeout = 5 seconds
```

Do not silently use unsafe defaults for:

```text
Issuer
Audience
JWKSURL
```

Production configuration should explicitly provide them.

---

# 4. JWKS Manager

Create a dedicated JWKS manager.

Recommended interface:

```go
type JWKSManager interface {
    GetKey(ctx context.Context, kid string) (crypto.PublicKey, error)
    Refresh(ctx context.Context) error
}
```

The implementation may use a different interface if it better fits the existing SDK architecture.

Responsibilities:

1. Fetch JWKS
2. Parse JWKS
3. Convert JWK to RSA public keys
4. Store keys in memory
5. Find key by `kid`
6. Refresh keys
7. Handle key rotation
8. Avoid fetching JWKS per request

---

# 5. JWKS Cache

JWKS MUST be cached in process memory.

Do NOT use Redis for normal JWKS caching.

Reason:

```text
JWT request
    ↓
Local memory
    ↓
RSA verify
```

instead of:

```text
JWT request
    ↓
Redis
    ↓
RSA verify
```

JWKS is public key material and does not require distributed mutable state.

Each pod can maintain its own cache.

This is expected.

---

# 6. JWKS Cache Behavior

Initial startup:

```text
Service starts
    ↓
Initialize JWKS Manager
    ↓
Fetch JWKS
    ↓
Parse keys
    ↓
Store in memory
```

Normal request:

```text
JWT
 ↓
Read kid
 ↓
JWKS cache
 ↓
Find public key
 ↓
Verify signature
```

NO HTTP request to JWKS endpoint should happen for every JWT.

---

# 7. Unknown kid Refresh

This is mandatory.

Example:

Cached keys:

```text
key-001
```

New JWT:

```text
kid=key-002
```

Flow:

```text
JWT
 ↓
kid=key-002
 ↓
Local cache
 ↓
key-002 not found
 ↓
Refresh JWKS
 ↓
Fetch latest JWKS
 ↓
key-002 found
 ↓
Verify JWT
```

This supports key rotation without restarting services.

---

# 8. Key Rotation

JWKS must support multiple active keys.

Example:

```json
{
  "keys": [
    {
      "kid": "key-001",
      "kty": "RSA",
      "alg": "RS256"
    },
    {
      "kid": "key-002",
      "kty": "RSA",
      "alg": "RS256"
    }
  ]
}
```

The verifier MUST select the correct public key using:

```text
JWT header kid
        ↓
JWKS key.kid
```

Do not assume the first key in JWKS is the active key.

Do not ignore `kid`.

---

# 9. JWKS Refresh Concurrency

The implementation MUST prevent a thundering herd.

Bad behavior:

```text
1000 requests
    ↓
kid not found
    ↓
1000 GET JWKS
```

Required behavior:

```text
1000 requests
    ↓
kid not found
    ↓
single refresh
    ↓
other requests wait/reuse result
```

Use an appropriate synchronization mechanism such as:

```go
sync.RWMutex
```

and/or:

```go
singleflight.Group
```

Preferred approach:

```text
unknown kid
    ↓
singleflight refresh
    ↓
one JWKS HTTP request
    ↓
update cache
    ↓
all waiting requests use updated cache
```

---

# 10. JWKS HTTP Client

The JWKS HTTP client MUST have:

- Configurable timeout
- Context support
- TLS verification enabled
- No insecure TLS by default
- Maximum acceptable response size
- HTTP status validation
- JSON parsing validation

Do not use:

```go
http.Get(...)
```

directly without timeout.

Use:

```go
http.Client{
    Timeout: configuredTimeout,
}
```

or equivalent context-aware implementation.

---

# 11. JWKS Response Validation

The implementation MUST validate:

```text
HTTP status == 200
Content-Type is JSON-compatible
JWKS contains keys
key.kty == RSA
key.alg == RS256 when present
key.use == sig when present
kid exists
```

Reject malformed keys.

Do not silently accept invalid JWK entries.

---

# 12. JWT Claims Validation

Signature verification alone is NOT enough.

The verifier MUST validate:

## Required

```text
signature
exp
issuer
audience
```

Recommended:

```text
nbf
iat
```

depending on the existing SDK architecture.

Example:

```json
{
  "iss": "https://auth.example.com",
  "aud": "payment-service",
  "sub": "123456",
  "exp": 1780000000,
  "iat": 1779996400
}
```

Validation:

```text
iss == configured issuer
aud == configured audience
exp > now
```

---

# 13. JWT Claims

Do not tightly couple the security package to a specific business user model.

Recommended:

```go
type Claims struct {
    Subject string

    Issuer   string
    Audience []string

    ExpiresAt int64
    IssuedAt  int64
    NotBefore int64

    JWTID string

    Roles       []string
    Permissions []string

    Custom map[string]any
}
```

The exact structure should follow the existing SDK conventions.

Security should expose identity information, not business-specific user objects.

---

# 14. Authentication Result

After successful JWT verification, security should expose authenticated identity through `context.Context`.

Example conceptual result:

```text
HTTP Request
    ↓
JWT Middleware
    ↓
JWT Verify
    ↓
Security Principal
    ↓
context.Context
    ↓
Handler
```

Example:

```go
principal, err := security.Authenticate(ctx, token)
```

The HTTP and gRPC adapters should consume the same core authentication logic.

---

# 15. Transport Agnostic Design

Do NOT put JWT verification logic inside Gin middleware.

Correct architecture:

```text
security/core
    │
    ├── JWT verifier
    ├── JWKS manager
    ├── claims
    └── principal
          │
          ├───────────────┐
          ▼               ▼
      HTTP Adapter     gRPC Adapter
          │               │
        Gin            gRPC interceptor
```

Gin and gRPC are adapters.

They MUST NOT contain RSA/JWKS implementation details.

---

# 16. Gin Middleware

Provide a Gin middleware adapter.

Conceptual:

```go
func JWTMiddleware(
    authenticator Authenticator,
) gin.HandlerFunc
```

Flow:

```text
Authorization Header
        ↓
Bearer token
        ↓
Authenticator
        ↓
JWT verifier
        ↓
Principal
        ↓
Gin Context
```

Invalid token:

```text
401 Unauthorized
```

Do not return:

```text
500 Internal Server Error
```

for invalid authentication.

---

# 17. gRPC Interceptor

Provide unary and stream support if the SDK already supports both.

Conceptual:

```go
func UnaryServerInterceptor(
    authenticator Authenticator,
) grpc.UnaryServerInterceptor
```

Authentication failure should map to:

```text
codes.Unauthenticated
```

Authorization failure should map to:

```text
codes.PermissionDenied
```

Do not confuse authentication with authorization.

---

# 18. Error Model

Use the existing SDK error model.

Recommended internal errors:

```text
ErrInvalidToken
ErrTokenExpired
ErrInvalidSignature
ErrInvalidAlgorithm
ErrInvalidIssuer
ErrInvalidAudience
ErrMissingKid
ErrUnknownKid
ErrJWKSFetch
ErrJWKSParse
ErrJWKSKeyNotFound
```

External response MUST NOT expose sensitive details such as:

```text
RSA public key internals
JWKS response
private key
internal HTTP errors
stack trace
```

---

# 19. Logging

Do NOT log:

```text
Authorization header
JWT token
private key
full JWKS response
```

Safe logs:

```text
JWT verification failed
kid=key-002
reason=unknown_kid
```

Even better, avoid logging the complete token.

---

# 20. Metrics

If the SDK has observability support, expose metrics such as:

```text
security_jwks_fetch_total
security_jwks_fetch_error_total
security_jwks_refresh_total
security_jwks_cache_hit_total
security_jwks_cache_miss_total
security_jwt_verify_total
security_jwt_verify_error_total
```

Recommended labels:

```text
service
issuer
result
reason
```

Avoid high-cardinality labels such as:

```text
user_id
jwt
jti
```

---

# 21. JWKS Cache Design

Recommended internal structure:

```go
type cachedKey struct {
    Key       crypto.PublicKey
    ExpiresAt time.Time
}

type jwksCache struct {
    mu   sync.RWMutex
    keys map[string]cachedKey
}
```

The implementation may use a more appropriate structure.

The cache MUST support:

```text
kid → RSA Public Key
```

---

# 22. Refresh Strategy

Support two refresh mechanisms.

## TTL refresh

Example:

```text
Cache TTL = 1 hour
```

After expiration:

```text
refresh JWKS
```

## Unknown kid refresh

If:

```text
kid not found
```

then immediately trigger:

```text
JWKS refresh
```

This is necessary for key rotation.

---

# 23. Startup Behavior

The implementation should support configurable startup behavior.

Recommended:

```text
Fail Fast = configurable
```

Option A:

```text
Service startup
 ↓
Fetch JWKS
 ↓
Failed
 ↓
Service startup fails
```

Option B:

```text
Service startup
 ↓
Initialize JWKS client
 ↓
Fetch lazily
```

The preferred default should be determined based on the existing SDK startup conventions.

Do not introduce breaking startup behavior without checking the existing code.

---

# 24. Configuration Example

Environment:

```text
JWT_ENABLED=true

JWT_ISSUER=https://auth.example.com

JWT_AUDIENCE=payment-service

JWT_JWKS_URL=https://auth.example.com/.well-known/jwks.json

JWT_CACHE_TTL=1h

JWT_REFRESH_TIMEOUT=5s
```

OpenShift ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: payment-security-config
data:
  JWT_ENABLED: "true"
  JWT_ISSUER: "https://auth.example.com"
  JWT_AUDIENCE: "payment-service"
  JWT_JWKS_URL: "https://auth.example.com/.well-known/jwks.json"
  JWT_CACHE_TTL: "1h"
  JWT_REFRESH_TIMEOUT: "5s"
```

IMPORTANT:

Do NOT store:

```text
JWT private key
JWT signing secret
RSA private key
```

inside the ConfigMap.

JWKS verification only requires public keys.

---

# 25. Example Runtime Flow

Initial startup:

```text
Payment Service
      │
      ▼
Load ConfigMap
      │
      ▼
JWT_JWKS_URL
      │
      ▼
Initialize JWKS Manager
      │
      ▼
GET configured JWKS URL
      │
      ▼
Parse public keys
      │
      ▼
Store in local memory
```

Request:

```text
Client
  │
  │ Authorization: Bearer JWT
  ▼
Gin
  │
  ▼
Security Middleware
  │
  ▼
Parse JWT
  │
  ▼
alg == RS256?
  │
  ▼
read kid
  │
  ▼
JWKS Cache
  │
  ▼
RSA Public Key
  │
  ▼
Verify Signature
  │
  ▼
Validate exp
  │
  ▼
Validate iss
  │
  ▼
Validate aud
  │
  ▼
Create Principal
  │
  ▼
Handler
```

---

# 26. Unknown kid Runtime Flow

```text
Request
  │
  ▼
JWT kid=key-002
  │
  ▼
Cache lookup
  │
  ├── found → verify
  │
  └── not found
          │
          ▼
      singleflight
          │
          ▼
      GET JWKS
          │
          ▼
      update cache
          │
          ▼
      lookup key-002
          │
          ▼
       verify JWT
```

---

# 27. Security Tests

Create unit and integration tests for at least:

## Valid token

```text
valid RS256 JWT
→ success
```

## Wrong algorithm

```text
HS256 JWT
→ rejected
```

## Invalid signature

```text
modified payload
→ rejected
```

## Expired token

```text
exp < now
→ rejected
```

## Wrong issuer

```text
iss != configured issuer
→ rejected
```

## Wrong audience

```text
aud != configured audience
→ rejected
```

## Unknown kid

```text
unknown kid
→ refresh JWKS
→ if still missing → rejected
```

## Key rotation

```text
key-001
→ valid

rotate to key-002

key-002
→ valid

key-001
→ remains valid while still published
```

## JWKS unavailable

Test behavior according to configured cache/startup strategy.

## Malicious jku

JWT:

```json
{
  "alg": "RS256",
  "kid": "attacker",
  "jku": "https://attacker.example.com/jwks.json"
}
```

Expected:

```text
DO NOT fetch attacker URL
```

## Empty JWKS

```json
{
  "keys": []
}
```

Expected:

```text
JWKS error
```

## Malformed JWKS

Expected:

```text
JWKS parse error
```

## Concurrent refresh

Simulate:

```text
100 concurrent requests
same unknown kid
```

Expected:

```text
1 JWKS network request
```

or the minimum number allowed by the implementation's refresh semantics.

---

# 28. Integration Test

Create an integration test environment containing:

```text
Mock Auth Server
      │
      └── /.well-known/jwks.json

Test Service
      │
      └── JWT Middleware
```

Test:

```text
Auth generates JWT using private key
        ↓
Test Service retrieves JWKS
        ↓
Test Service verifies JWT
        ↓
Request succeeds
```

Then rotate key:

```text
key-001 → key-002
```

and verify:

```text
JWT signed with key-002
        ↓
unknown kid
        ↓
JWKS refresh
        ↓
verification succeeds
```

---

# 29. HTTP Client Security

The JWKS HTTP client MUST:

- Use HTTPS in production
- Validate TLS certificates
- Support context cancellation
- Have a finite timeout
- Avoid redirects to arbitrary hosts if practical
- Avoid unlimited response bodies
- Not follow arbitrary `jku` values
- Not send Authorization headers to JWKS unless explicitly configured

Do not copy incoming request headers into the JWKS request.

Especially:

```text
Authorization
Cookie
X-API-Key
```

must not be forwarded.

---

# 30. Do Not Use Redis

Do not implement:

```text
JWT verification
    ↓
Redis
    ↓
JWKS
```

unless there is an explicit future requirement.

Default:

```text
JWT verification
    ↓
Local memory JWKS cache
    ↓
RSA verification
```

Redis is unnecessary for public key distribution.

---

# 31. Expected Package Structure

Adapt to the existing repository instead of blindly creating duplicate packages.

Recommended target:

```text
security/
├── jwt/
│   ├── config.go
│   ├── claims.go
│   ├── verifier.go
│   ├── jwks.go
│   ├── cache.go
│   ├── refresh.go
│   └── errors.go
│
├── http/
│   └── gin_jwt.go
│
└── grpc/
    └── jwt_interceptor.go
```

If the existing SDK already has a different package structure, preserve the existing architectural convention.

---

# 32. Dependency Rules

Before adding dependencies:

1. Inspect existing `go.mod`
2. Reuse existing JWT/JWK dependencies if already present
3. Avoid duplicate libraries
4. Prefer mature and actively maintained Go libraries
5. Keep JWT/JWKS implementation isolated

Do not introduce unnecessary frameworks.

---

# 33. Backward Compatibility

Before implementation:

1. Inspect existing security interfaces.
2. Inspect existing JWT implementation.
3. Inspect existing authentication middleware.
4. Inspect Gin adapter.
5. Inspect gRPC adapter.
6. Preserve existing public interfaces where possible.

If breaking changes are unavoidable:

- clearly identify them
- update all affected implementations
- update tests
- update documentation

Do not silently break existing SDK consumers.

---

# 34. Documentation

Update README / security documentation with:

## Configuration

```text
JWT_ENABLED
JWT_ISSUER
JWT_AUDIENCE
JWT_JWKS_URL
JWT_CACHE_TTL
JWT_REFRESH_TIMEOUT
```

## Architecture

Explain:

```text
Auth Service
    ↓
Private Key
    ↓
JWT

Service
    ↓
JWKS
    ↓
Public Key
    ↓
Verify JWT
```

## Key Rotation

Document how `kid` and JWKS refresh work.

## OpenShift

Document that:

```text
ConfigMap
    ↓
JWKS URL
```

and NOT:

```text
ConfigMap
    ↓
Public Key content
```

---

# 35. Definition of Done

Implementation is complete only when:

- [ ] RS256 verification works
- [ ] JWKS URI is configurable
- [ ] JWKS URI is NOT hardcoded
- [ ] Issuer is configurable
- [ ] Audience is configurable
- [ ] `kid` is supported
- [ ] JWKS is cached in local memory
- [ ] JWKS is NOT fetched per request
- [ ] Unknown `kid` triggers refresh
- [ ] Concurrent refresh is protected
- [ ] Key rotation works
- [ ] `jku` is not trusted
- [ ] Arbitrary JWKS URLs are rejected
- [ ] JWT `alg` is strictly validated
- [ ] `exp` is validated
- [ ] `iss` is validated
- [ ] `aud` is validated
- [ ] Gin middleware uses the core verifier
- [ ] gRPC interceptor uses the core verifier
- [ ] No private key is exposed
- [ ] No JWT is logged
- [ ] JWKS HTTP client has timeout
- [ ] Unit tests are implemented
- [ ] Integration tests are implemented
- [ ] Key rotation test passes
- [ ] Concurrent refresh test passes
- [ ] Documentation is updated
- [ ] Existing SDK tests continue to pass
- [ ] `go test ./...` passes
- [ ] `go vet ./...` passes
- [ ] `go build ./...` passes

---

# 36. Final Implementation Principle

The final design MUST follow this principle:

```text
              AUTH SERVICE
                  │
            Private RSA Key
                  │
                SIGN
                  │
                  ▼
                 JWT
                  │
        ┌─────────┼─────────┐
        ▼         ▼         ▼
      Gin       gRPC      Other
        │         │         │
        └─────────┼─────────┘
                  │
             JWT Verifier
                  │
               kid lookup
                  │
                  ▼
           Local JWKS Cache
                  │
             cache miss
                  │
                  ▼
        Configured JWKS URL
                  │
                  ▼
        /.well-known/jwks.json
                  │
                  ▼
             Public Keys
                  │
                  ▼
           Verify RS256
                  │
                  ▼
       Validate iss/aud/exp
                  │
                  ▼
             Principal
```

The most important requirement is:

> **The JWKS URL MUST be configurable and MUST NEVER be taken from the JWT itself.**

The implementation should be production-oriented, transport-agnostic, high-TPS friendly, and compatible with the existing Go SDK architecture.
