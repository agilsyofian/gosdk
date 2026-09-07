# modules — business logic

Every business capability in this app lives here as its own module:
`auth`, `user`, `notifications`, `storage`. This page explains the shape all modules
share, walks through what actually happens on a login, and shows how to
add a new module of your own following **Hexagonal Architecture (Ports & Adapters)**.

## Anatomy of a module

```
modules/<name>/
├── domain/           # Plain domain entities & commands — no SDK/transport types,
│                     # no fx tags, no web binding tags.
├── port/             # (Optional) Outbound port interfaces — defines contracts for
│                     # external services (gRPC user lookup, event publisher, 3rd party APIs).
├── repository/       # (Optional) Internal database persistence (Postgres/MySQL).
│                     # Returns domain types; the only layer that knows SQL/GORM.
├── service/          # Pure use-case business logic. Depends on repository +
│                     # port interfaces + SDK core (security.SessionManager, cache).
│                     # Never imports transport packages (gin, grpc) or integration packages.
├── handler/
│   ├── httphandler/  # HTTP delivery: http_dto.go, http_handler.go, http_mapper.go, http_routes.go (httpserver.RouteRegistrar)
│   ├── grpchandler/  # (Optional) gRPC delivery: grpc_handler.go, grpc_mapper.go, grpc_service.go (grpcserver.GRPCServiceRegistrar)
│   └── messaging/    # (Optional) Kafka event handlers: dto.go, handler.go, subscribers.go (consumer.TopicSubscriber)
└── module.go         # fx.Module: fx.Provide everything above + decentralized registrars
```

**Dependency direction is strictly decoupled:**
- Inbound: `handler -> service`
- Outbound: `service -> repository` (internal storage) and `service -> port` (external dependencies).
- A service never imports its handler.
- A repository never imports its service.
- An outbound integration client (`integration/`) implements the module's `port` interface, never vice versa.
- This is what lets you swap Gin for a CLI, Postgres for an in-memory mock, or gRPC for REST in tests without touching business logic.

`module.go` is always concise and self-registers routes, gRPC services, and Kafka subscribers into Fx groups without touching central server/messaging packages:

```go
var Module = fx.Module("module-<name>",
    fx.Provide(
        repository.New,
        service.New,
        httphandler.New,
        httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar), // decentralizes HTTP route setup
        grpchandler.New,                                           // if this module exposes gRPC too
        grpcserver.AsGRPCServiceRegistrar(grpchandler.NewServiceRegistrar),
        messaging.New,                                             // if this module consumes Kafka events
        consumer.AsTopicSubscriber(messaging.NewUserEventSubscriber),
    ),
)
```

Fx resolves the constructor order from their argument types — you never
call one constructor from another by hand, and you never need to edit
`app/server/httpserver/http.go` or `app/messaging/consumer/consumer.go` when adding new modules.

## Walkthrough 1: `auth` — a write path (`POST /example/v1/auth/password`)

This is the most complete example in the app, because a login touches
almost everything the SDK provides:

1. **`handler/httphandler/http_handler.go`** binds the JSON body into `LoginPasswordDTO`
   (`binding:"required"` on username/password), then maps it to `domain.Login` via `http_mapper.go`.
2. **`service/auth_service.go`** (`LoginPassword`):
   - Reads the device identifier from request metadata
     (`core/metadata`) — this is why the quickstart
     curl in the parent README sends an `X-Device-Id` header.
   - Calls `ValidateAuthCommand` — a second, service-level check,
     independent of the HTTP binding tags.
   - Calls **`port.IUserFinder`**, which is satisfied by `integration/grpc/user` (a
     real gRPC client that connects to this same process's own gRPC server).
     That round trip returns the user's row from Postgres as a clean domain `domain.UserIdentity`.
   - Issues a JWT access + refresh token pair (`security/authenticator/jwt`)
     and creates a session (`security.SessionManager`, Redis-backed).
   - Publishes a `user.event` Kafka message via its outbound port **`port.IUserEventPublisher`**,
     implemented by `integration/kafka/user_event`.
3. That published event is picked up asynchronously by **Walkthrough 2**
   below — a different module, decoupled through Kafka rather than a direct call.

Every failure branch in `LoginPassword` returns its own error code
(`errors.Unauthorized(err, appName, code, message)`) and a message
that's safe to show a client — the real cause (`err`) is still attached
for logs, it just isn't echoed back on the wire.

## Walkthrough 2: `notifications` — a read path (Kafka consumer)

1. **`app/messaging/consumer`** runs Kafka consumers on their own
   goroutines, started/stopped via the app's Fx lifecycle.
2. A message on `user.event` reaches **`handler/messaging/handler.go`**,
   which decodes it into a local `UserEvent` struct
   (`handler/messaging/dto.go`) — deliberately *not* the same Go type
   the publisher uses to emit it. A consumer decodes the wire
   format (JSON); in a real system producer and consumer are usually
   separate services entirely and have no Go package to share.
3. The handler calls **`service.NotificationService.PushFCM`**, which (in this example) logs
   what it would have sent. Swap that one method for a real push
   provider call and nothing upstream of it needs to change.

## Adding a new module

1. `mkdir -p modules/<name>/{domain,service,handler/httphandler}` (add
   `port/`, `repository/` or `handler/grpchandler/` only if you need them).
2. Write `domain` first — plain Go types, no framework imports.
3. If calling other services, declare outbound port interfaces in `port/` (e.g. `IUserFinder`).
4. If querying local database, declare repository interface in `repository/` (e.g. `IOrderRepository`).
5. Write `service` against an interface (`type I<Name>Service interface {...}`)
   with an `_deps.go` struct declaring all dependencies via `fx.In`, with constructor `service.New`.
6. Write the handler in `handler/httphandler/`: bind request (`http_dto.go`) → call service → map the response (`http_mapper.go`).
   Implement `httpserver.RouteRegistrar` in `http_routes.go` to decentralize route registration.
7. Write `module.go` with one `fx.Provide` listing every constructor and registrar (`repository.New`, `service.New`, `httphandler.New`, `httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar)`).
8. Register the module in `../main.go` under the "Business modules" list.
9. If it needs a permission check, add `jwtSecurity.RequirePermission("resource:action")`
   on the route and make sure that permission code actually exists in
   `iam_permissions` and is granted to a role the test user has.

