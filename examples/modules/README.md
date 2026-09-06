# modules — business logic

Every business capability in this app lives here as its own module:
`auth`, `user`, `notifications`. This page explains the shape all three
share, walks through what actually happens on a login, and shows how to
add a new module of your own.

## Anatomy of a module

```
modules/<name>/
├── model/            # plain domain types — no SDK types, no fx, no json tags
│                      # that belong to a transport
├── repository/        # (optional) talks to storage/database, storage/cache, etc.
│                       # returns model types; the only layer that knows SQL
├── service/            # business logic. Depends on repository + SDK
│                        # interfaces (security.SessionManager, a messaging
│                        # publisher, an integration client, ...).
│                        # Depends on nothing transport-specific.
├── handler/
│   ├── http/            # gin handlers: bind request -> call service -> map
│   │                     # response. One dto.go, one mapper.go/mapper_gen.go.
│   └── grpc/             # same idea, for a gRPC-exposed service
└── module.go             # fx.Module: fx.Provide everything above
```

**Dependency direction is one-way: `handler -> service -> repository`.**
A repository never imports its service; a service never imports a
handler. This is the same rule core SDK packages follow (providers
depend on core, never the reverse) — it's what lets you swap the HTTP
handler for a CLI, or the Postgres repository for an in-memory fake in a
test, without touching the layers around it.

`module.go` is always the smallest file and always looks like this:

```go
var Module = fx.Module("module-<name>",
    fx.Provide(
        repository.NewXRepository,
        service.NewXService,
        http.NewHandler,
        grpc.NewHandler, // if this module exposes gRPC too
    ),
)
```

fx resolves the constructor order from their argument types — you never
call one constructor from another by hand.

## Walkthrough 1: `auth` — a write path (`POST /example/v1/auth/password`)

This is the most complete example in the app, because a login touches
almost everything the SDK provides:

1. **`handler/http/handler.go`** binds the JSON body into `LoginPassword`
   (`binding:"required"` on username/password), then calls the service.
2. **`service/auth_service.go`** (`LoginPassword`):
   - Reads the device identifier from request metadata
     (`core/metadata`) — this is why the quickstart
     curl in the parent README sends an `X-Device-Id` header.
   - Calls `ValidateAuthCommand` — a second, service-level check,
     independent of the HTTP binding tags (see that function's doc
     comment for why).
   - Calls `integration/user`, a **gRPC client that connects to this
     same process's own gRPC server** — see the parent README for why.
     That round trip returns the user's row from Postgres, including
     `PasswordHash` — a field the *HTTP-facing* `UserDetail` DTO
     explicitly excludes (`handler/http/mapper.go`'s `-ignore` flag). A
     service-to-service contract and a client-facing contract are
     allowed to differ; the trust boundary is the transport, not the Go
     type.
   - Verifies the password with `bcrypt.CompareHashAndPassword` against
     that hash.
   - Issues a JWT access + refresh token pair (`security/authenticator/jwt`)
     and creates a session (`security.SessionManager`, Redis-backed) —
     the session, not the token, is this SDK's source of truth for who's
     logged in.
   - Publishes a `user.event` Kafka message (`messaging/publisher`).
3. That published event is picked up asynchronously by **Walkthrough 2**
   below — a different module, decoupled through Kafka rather than a
   direct call.

Every failure branch in `LoginPassword` returns its own error code
(`errors.Unauthorized(err, appName, code, message)`) and a message
that's safe to show a client — the real cause (`err`) is still attached
for logs, it just isn't echoed back on the wire. Worth reading if you're
about to write your own service method: it's the pattern to copy.

## Walkthrough 2: `notifications` — a read path (Kafka consumer)

1. **`messaging/consumer`** runs two Kafka consumers on their own
   goroutines (one per topic), started/stopped via the app's fx
   lifecycle.
2. A message on `user.event` reaches **`handler/messaging/handler.go`**,
   which decodes it into a local `UserEvent` struct
   (`handler/messaging/dto.go`) — deliberately *not* the same Go type
   `messaging/publisher` uses to publish it. A consumer decodes the wire
   format (JSON); in a real system producer and consumer are usually
   separate services entirely and have no Go package to share.
3. The handler calls **`service.PushFCM`**, which (in this example) logs
   what it would have sent. Swap that one method for a real push
   provider call and nothing upstream of it needs to change — that's
   the seam a real implementation fills.

## Adding a new module

1. `mkdir -p modules/<name>/{model,service,handler/http}` (add
   `repository/` or `handler/grpc/` only if you need them).
2. Write `model` first — plain Go types, no framework imports.
3. Write `service` against an interface (`type IXService interface {...}`)
   even with one implementation — it's what makes the handler and any
   test able to depend on the interface instead of a concrete struct.
4. Write the handler: bind request → call service → map the response.
   Look at `modules/auth/handler/http` for the smallest complete
   example, or `modules/user/handler/http/mapper.go` if your response
   DTO is large enough that hand-writing the mapping would be tedious
   (that's what `mapgen` is for).
5. Write `module.go` with one `fx.Provide` listing every constructor.
6. Register the module in `../main.go`, under the "Business modules"
   section, and — if it's HTTP/gRPC-facing — add its routes in
   `../server/http.go` or `../server/grpc.go`.
7. If it needs a permission check, add `jwtSecurity.RequirePermission("resource:action")`
   on the route and make sure that permission code actually exists in
   `iam_permissions` and is granted to a role the test user has (see
   `tools/dbmigration/migrations/..._seed_test_user.up.sql` for a
   worked example: it creates its own role and permissions rather than
   relying on the generic seeded ones, specifically so `user:read`/
   `user:update` exist and are granted).
