# examples

A single runnable app that wires this SDK's pieces together: HTTP + gRPC
servers, JWT auth with Redis-backed sessions, RBAC permissions, Postgres,
and Kafka pub/sub — all through one dependency-injection graph
([uber-go/fx](https://github.com/uber-go/fx)).

**If you're new here: skip straight to [`modules/`](./modules/README.md).**
Everything else on this page is the plumbing you wire up *once* and
rarely touch again. `modules/` is where the actual product code lives,
and it's the part worth reading closely as a template for your own SDK
integration.

## Quickstart

**1. Start the infrastructure** (Postgres, Redis, Kafka):

```bash
# from the repo root, not from examples/
docker compose up -d postgres redis zookeeper kafka
```

**2. Run the database migration** (creates the RBAC schema + a demo user):

```bash
cd tools/dbmigration
DB_DSN="postgres://root:root@localhost:5432/postgres?sslmode=disable" go run ./cmd/migrate up
```

**3. Run the app:**

```bash
cd examples
go run .
```

You should see fx log each module starting, then the HTTP server
listening on `:8001` and gRPC on `:8081` (see `env.yaml`).

**4. Try the login flow end to end:**

```bash
curl -X POST localhost:8001/example/v1/auth/password \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: LOickgx2ZMRZoMyeIjZAgcfl7p92' \
  -d '{"username":"demo@example.com","password":"password"}'
```

The `X-Device-Id` header matters here, not just as a header for its own
sake: the demo user's row in `tools/dbmigration/migrations/..._seed_test_user.up.sql`
is tied to that exact device identifier, and the lookup query joins on
it — a login with no `X-Device-Id`, or the wrong one, correctly fails as
"invalid login credentials", the same as a wrong password would.

That call exercises almost the whole stack in one request: gin → the
`auth` module → a gRPC call to this same process's `user` module →
Postgres → bcrypt password check → JWT + Redis session → a Kafka event
published to `user.event`, which the `notifications` module then
consumes asynchronously. See [`modules/README.md`](./modules/README.md)
for the full walkthrough of that path.

All config (ports, DSNs, topic names, retry/circuit-breaker tuning) is
in `env.yaml`. Nothing there is secret — it's a local-dev fixture, not
production config.

### Multi-Environment Configuration (`APP_ENV`)

The config loader supports layered environment overrides (`v.MergeInConfig()`):
- **Local development (default)**: `go run .` loads `env.yaml`.
- **Staging**: `APP_ENV=staging go run .` loads `env.yaml` and overlays `env.staging.yaml`.
- **Production**: `APP_ENV=production go run .` loads `env.yaml` and overlays `env.production.yaml`.

You only need to specify the keys that differ in `env.<env>.yaml`; all other settings are automatically inherited from `env.yaml`.

## Map of the codebase

```
examples/
├── main.go            # assembles every fx.Module below — read this first for the big picture
├── env.yaml           # local-dev configuration (ports, DSNs, JWT keys, topics)
│
├── app/
│   ├── config/        # env.yaml -> typed AppConfig (viper + validator)
│   ├── bootstrap/     # one fx.Module per SDK building block: config, logger,
│   │                  # tracer, Postgres, Redis, JWT authenticator, session, etc.
│   ├── server/        # INBOUND: self-contained transport servers
│   │   ├── http/      #   HTTP server, routing engine & RouteRegistrar (http.HTTP)
│   │   └── grpc/      #   gRPC server, service engine & GRPCServiceRegistrar (grpc.GRPC)
│   ├── messaging/
│   │   └── consumer/  # INBOUND: Kafka consumer background worker engine
│   ├── security/      # assembles this app's security.SecurityManager
│   └── shared/        # shared cross-cutting helpers (converters, etc)
│
├── integration/       # OUTBOUND: outbound network clients to external services & brokers
│   ├── grpc/user/     #   gRPC client — calls User Service
│   ├── http/httpbin/  #   HTTP client demonstrating retry/circuit-breaker
│   └── kafka/user_event/ # Kafka event publisher implementing auth.IUserEventPublisher
│
├── proto/user/        # protobuf contract for the internal user gRPC service
│
└── modules/           # ★ business logic & domain ports — start here
    ├── auth/          #   login, logout, token refresh, JWKS endpoint
    ├── user/          #   user profile read/update, backed by Postgres
    ├── storage/       #   file upload/download/presigned-url via objectstorage
    └── notifications/ #   consumes user.event, would push to FCM/APNs
```

### Why does `auth` call a gRPC client that connects to this same app?

`integration/user` is a real gRPC client pointed at `localhost:8081` —
which is this same binary's own gRPC server (`server/grpc.go` registers
the `user` module's handler there). In a real system `user` would
usually be a separate service; here it's kept in the same process so the
whole example runs with `go run .` and no extra deployment, while still
demonstrating a genuine client → network → server round trip (JWT
propagation, timeouts, retry, circuit breaker) rather than a plain
in-process function call. If you split `user` into its own service
later, `integration/user`'s client code does not need to change — only
`grpc_integrations.user.addr` in `env.yaml`.

### Setup layer vs. business layer

| | `app/`, `server/`, `integration/`, `messaging/` | `modules/` |
|---|---|---|
| **What it is** | SDK wiring: config, connections, auth, transport | Your product's actual logic |
| **How often you touch it** | Once per app, then rarely | Constantly — this is where you work |
| **What to do with it** | Copy it, understand the shape, move on | Read `modules/README.md` and use it as your template |

`main.go` groups its `fx.Module` list the same way, with a comment
marking where the business modules start.

## Adding a new business module

See [`modules/README.md`](./modules/README.md) for the anatomy of a
module and a step-by-step guide to adding a new one.

## Known gaps in this example (by design)

These are intentionally left incomplete rather than half-implemented
with unverified logic — see the doc comment at each for what a real
implementation would need:

- **`modules/auth.TokenRefresh`** — returns a clear "not implemented"
  error. Refresh-token rotation is security-sensitive enough that this
  example doesn't guess at it.
- **`modules/notifications.PushFCM`** — logs instead of calling a real
  push provider, since pulling in an FCM/APNs SDK is out of scope for
  demonstrating this SDK's messaging layer.
