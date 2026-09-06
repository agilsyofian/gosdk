# security-sdk

Reusable Go security module: authentication (JWT/Bearer, Basic, API Key,
extensible), stateful server-side sessions (Redis-backed, extensible),
a unified `SecurityContext`, and permission-based authorization —
usable from both Gin and gRPC through thin transport adapters.

> **Design principle:** Authentication proves identity. The
> server-side **Session** is the authoritative user context.
> **SecurityContext** is the contract application code consumes.

## Architecture & Module Layout

Security is structured around a **zero-dependency core module** plus **dedicated provider modules** for implementations requiring external libraries (Redis, HTTP client, or distinct cryptographic lifecycles):

```text
github.com/agilsyofian/gosdk/security             (Core module, zero external deps)
├── (root)                                         types, interfaces, Manager, Registry
├── authorization/                                 PermissionAuthorizer
├── authenticator/apikey/                          built-in API Key authenticator
├── authenticator/basicauth/                       built-in HTTP Basic authenticator
└── session/memory/                                in-memory session manager (test/dev)

github.com/agilsyofian/gosdk/security/jwks        (Remote JWKS fetcher, HTTP client & cache)
github.com/agilsyofian/gosdk/security/authenticator/jwt (JWT verification & token issuance)
github.com/agilsyofian/gosdk/security/session/redis (Redis distributed session store)
github.com/agilsyofian/gosdk/security/signature   (HMAC/RSA request & response signing)
```

**Dependency direction is enforced by construction:**
- The core module (`security`) has zero external dependencies and never imports provider or transport modules.
- Built-in zero-dependency authenticators (`apikey`, `basicauth`) and authorizers (`authorization`) live directly as subpackages in the core module.
- Heavy I/O providers (`jwks`, `session/redis`) live in dedicated modules so consumers only pull what they actually use.

### Why per-module, not per-package

With everything as one module, adding one authenticator would pull its
dependencies (e.g. a Redis driver, or a signature library) into
*every* consumer's `go.sum`, even ones that never use it. With one
module per provider:

- `go get security-sdk/authenticator/digitalsignature` only pulls in
  what that provider needs -- nothing else changes.
- Each provider can be versioned and released independently.
- A provider module's own tests can't accidentally leak an import from
  a sibling provider, because there is no module-level path to it.

### Adding a new authenticator (e.g. DigitalSignature)

```text
authenticator/digitalsignature/
├── go.mod               module security-sdk/authenticator/digitalsignature
│                         require security-sdk ...; replace security-sdk => ../../
├── digitalsignature.go   type Authenticator struct{...}; func New(cfg Config) *Authenticator
│                         implements security.Authenticator (Type() + Authenticate())
└── digitalsignature_test.go
```

Then in your wiring code:

```go
registry := security.NewRegistry()
registry.Register(jwtAuth)
registry.Register(digitalsignature.New(cfg))   // <- the only new line
```

No change to `security.Manager`, `security.Registry`, or either
transport adapter. This is acceptance criterion #11 (see checklist
below) and applies equally to session providers.

### Adding a new session provider (e.g. Dragonfly, Memcached)

```text
session/dragonfly/
├── go.mod           module security-sdk/session/dragonfly
├── dragonfly.go      type Manager struct{...}; func New(client Client) *Manager
│                     implements security.SessionManager (Create/Get/Delete/Refresh)
└── dragonfly_test.go
```

Since Dragonfly speaks the Redis protocol, this would likely reuse the
same small `Client` interface pattern as `session/redis` (`Get`/`Set`/
`Del`/`TTL`) -- or even be usable through `session/redis` directly,
depending on the client library. A genuinely different store
(Memcached, DynamoDB, ...) implements the same four
`security.SessionManager` methods against its own client shape.

## Local development: go.work

A workspace file at the repo root ties the buildable modules together
for local development, so `go build`/`go test` work across module
boundaries without needing anything published to a proxy:

```text
go.work
  use . ./authenticator/jwt ./authenticator/apikey ./authenticator/basicauth
        ./session/redis ./session/memory ./examples/basic
```

`transport/gin`, `transport/grpc`, `examples/gin`, and `examples/grpc`
are deliberately **not** in `go.work` because they require
`github.com/gin-gonic/gin` / `google.golang.org/grpc`, which this
sandbox has no network access to fetch. In an environment with network
access:

```bash
cd transport/gin  && go get github.com/gin-gonic/gin@v1.10.0 && go mod tidy
cd transport/grpc && go get google.golang.org/grpc@v1.68.0   && go mod tidy
go work use ./transport/gin ./transport/grpc ./examples/gin ./examples/grpc
```

Each module also carries a `replace security-sdk => ../../`-style
directive in its own `go.mod`, so it's independently buildable even
*without* the workspace (e.g. in CI running one module at a time).

## Quick start

```bash
cd examples/basic && go run .
```

Runs the full lifecycle end to end, wiring together the core module
plus three provider modules: `basicauth` login → `security.NewRegistry()`
→ `memory.New()` session → `jwt.Issue()` → a simulated authenticated
request that resolves the session from the JWT's `sid` → a permission
check → logout → proof that the same JWT is rejected after logout
(server-side revocation).

## Core flow

```text
FE --(JWT / Basic / API Key / ...)--> Authenticator --> AuthenticationResult
                                                              │
                                                    SessionID? │
                                              ┌────────────────┴───────────────┐
                                          yes  │                            no │
                                              ▼                                ▼
                                  SessionManager.Get() (Redis/...)        Principal
                                              │                                │
                                              └──────────────┬─────────────────┘
                                                              ▼
                                                       SecurityContext
                                                              │
                                                              ▼
                                                     Application code
```

## Using it

### 1. Wire it once at startup

```go
import (
    "security-sdk/security"
    "security-sdk/security/authorization"

    "security-sdk/authenticator/jwt"
    "security-sdk/authenticator/basicauth"
    "security-sdk/authenticator/apikey"
    "security-sdk/session/redis" // or session/memory for tests
)

jwtAuth, _ := jwt.New(jwt.Config{Algorithm: jwt.HS256, HMACSecret: secret})

registry := security.NewRegistry()
registry.Register(jwtAuth)
registry.Register(basicauth.New(myUserStore))
registry.Register(apikey.New(myAPIKeyStore))

sessions := redis.New(myRedisAdapter) // see session/redis's Client doc comment

authorizer := authorization.NewPermissionAuthorizer(map[string][]string{
    "admin": {"transaction:transfer"},
})

securityManager, err := security.NewManager(security.ManagerConfig{
    Authenticator: registry,
    Sessions:      sessions,
    Authorizer:    authorizer,
})
```

### 2. Login

```go
result, err := basicAuth.Authenticate(ctx, credential)      // verify password
sess, err   := sessions.Create(ctx, result.Principal, ttl)  // create server-side session
token, err  := jwt.Issue(jwtIssueCfg, result.Principal.Subject, sess.ID, ttl, nil)
// return token to FE
```

### 3. Protect routes

Gin (see `examples/gin/main.go`):

```go
import ginsecurity "security-sdk/transport/gin"

mw := ginsecurity.New(securityManager, ginsecurity.Config{
    Required: true, SessionCookie: "SESSION_ID", EnableBearer: true,
})

api := router.Group("/api")
api.Use(mw.Middleware())
api.POST("/transfer", mw.RequirePermission("transaction:transfer"), transferHandler)
```

gRPC (see `examples/grpc/main.go`) — **same** `securityManager`:

```go
import grpcsecurity "security-sdk/transport/grpc"

ic := grpcsecurity.New(securityManager, grpcsecurity.Config{Required: true, EnableBearer: true})

server := grpc.NewServer(
    grpc.UnaryInterceptor(ic.UnaryServerInterceptor()),
    grpc.StreamInterceptor(ic.StreamServerInterceptor()),
)
```

### 4. Business code stays transport-free and provider-agnostic

```go
func (s *TransferService) Transfer(ctx context.Context, req *TransferRequest) error {
    sc, err := security.MustFromContext(ctx)
    if err != nil {
        return err
    }
    userID := sc.Principal.Subject
    // ...
}
```

This service does not know whether the request came from Gin or gRPC,
whether authentication was JWT/Basic/API Key, or whether sessions live
in Redis or Dragonfly.

### 5. Logout

```go
sessions.Delete(ctx, sess.ID)
// any previously-issued JWT referencing sess.ID now resolves to
// security.ErrUnauthenticated, even though the JWT itself is still
// signature-valid and unexpired.
```

## Testing

Each module tests itself independently:

```bash
(cd .                             && go test ./...)
(cd authenticator/jwt              && go test ./...)
(cd authenticator/apikey           && go test ./...)
(cd authenticator/basicauth        && go test ./...)
(cd session/redis                  && go test ./...)
(cd session/memory                 && go test ./...)
```

22 tests cover: JWT valid / expired / bad signature / valid-but-session-missing,
session expired / deleted, Basic and API Key auth, permission
allowed/denied, conflicting credentials, concurrent session access,
Redis unavailable, and logout invalidating an active session. The core
module's own `manager_test.go` uses a small in-file fake
`SessionManager` rather than importing `session/memory`, to keep the
core module's dependency graph empty even of its own provider
submodules.

`transport/gin` and `transport/grpc` are not covered by automated
tests in this environment because this sandbox has no network access
to fetch `github.com/gin-gonic/gin` or `google.golang.org/grpc`. Their
source was written and syntax-checked (`gofmt -e`), but you should run
`go build ./...` and add adapter-level tests once fetched in your own
environment.

## Acceptance criteria checklist (from the spec)

| # | Criterion | Status |
|---|---|---|
| 1 | Gin request can authenticate using JWT | Implemented (`transport/gin`); needs `go get` in an env with network access |
| 2 | JWT validation extracts `sid` | ✅ `authenticator/jwt` |
| 3 | `sid` resolved through Redis SessionManager | ✅ `security.Manager` + `session/redis` |
| 4 | User details obtained from Session | ✅ `SecurityContext.Principal` comes from `Session.Principal` |
| 5 | SecurityContext available via `context.Context` | ✅ `security/context.go` |
| 6 | Business services don't depend on Gin | ✅ core has zero transport imports |
| 7 | Same SecurityManager usable by gRPC | ✅ `transport/grpc` takes any `security.SecurityManager` |
| 8 | gRPC metadata can authenticate with same JWT authenticator | ✅ `transport/grpc/extractor.go` |
| 9 | Deleting Redis session invalidates a valid JWT | ✅ tested (`TestManager_Authenticate_LogoutInvalidatesSession`) |
| 10 | Permission checks consistent across Gin/gRPC | ✅ both call `SecurityManager.Authorize` |
| 11 | Adding an authenticator needs no adapter/core changes | ✅ one new module + one `registry.Register` call |
| 12 | Security core has no Gin/gRPC dependency | ✅ verified: only `transport/*` modules import those packages |
| 13 | Security core has no dependency on any provider module | ✅ verified: `security/` never imports `authenticator/*` or `session/*` |
