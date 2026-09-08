# examples

A single runnable app that wires this SDK's pieces together: HTTP + gRPC
servers, JWT auth with Redis-backed sessions, RBAC permissions, Postgres,
and Kafka pub/sub — all through one dependency-injection graph
([uber-go/fx](https://github.com/uber-go/fx)) following **Hexagonal Architecture (Ports & Adapters) / Clean Architecture**.

**If you're creating or extending a module: skip straight to [Anatomy of a Business Module](#anatomy-of-a-business-module) or [`modules/README.md`](modules/README.md).**

---

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

You should see Fx log each module starting, then the HTTP server
listening on `:8001` and gRPC on `:8081` (see `env.yaml`).

**4. Try the login flow end to end:**

```bash
curl -X POST localhost:8001/example/v1/auth/password \
  -H 'Content-Type: application/json' \
  -H 'X-Device-Id: LOickgx2ZMRZoMyeIjZAgcfl7p92' \
  -d '{"username":"demo@example.com","password":"password"}'
```

The `X-Device-Id` header matters here: the demo user's row in `tools/dbmigration/migrations/..._seed_test_user.up.sql`
is tied to that exact device identifier.

That call exercises almost the whole stack in one request: Gin → the
`auth` module → gRPC call to `user` module via `integration/grpc/user` →
Postgres → bcrypt password check → JWT + Redis session → a Kafka event
published to `user.event` via `integration/kafka/user_event`, which the `notifications` module
consumes asynchronously.

---

### Multi-Environment Configuration (`APP_ENV`)

The config loader supports layered environment overrides (`v.MergeInConfig()`):
- **Local development (default)**: `go run .` loads `env.yaml`.
- **Staging**: `APP_ENV=staging go run .` loads `env.yaml` and overlays `env.staging.yaml`.
- **Production**: `APP_ENV=production go run .` loads `env.yaml` and overlays `env.production.yaml`.

---

## Map of the Codebase

```
examples/
├── main.go               # Assembles every fx.Module — entry point of the app
├── env.yaml              # Local-dev configuration (ports, DSNs, JWT keys, topics)
│
├── app/                  # INFRASTRUCTURE & SHARED KERNEL
│   ├── config/           # env.yaml -> typed AppConfig (viper + validator)
│   ├── bootstrap/        # Fx modules for SDK building blocks (config, logger, tracer, DB, Redis, JWT, session)
│   ├── server/           # Inbound server engines
│   │   ├── httpserver/   #   HTTP engine with decentralized RouteRegistrar
│   │   └── grpcserver/   #   gRPC engine with decentralized GRPCServiceRegistrar
│   ├── messaging/
│   │   └── consumer/     # Inbound Kafka consumer worker engine (TopicSubscriber)
│   ├── security/         # Security & authorization manager
│   └── shared/           # Cross-cutting converters and utilities
│
├── integration/          # SECONDARY / OUTBOUND ADAPTERS (External network clients)
│   ├── grpc/user/        #   gRPC client implementing auth/port.IUserFinder
│   ├── http/httpbin/     #   HTTP client demonstrating retry & circuit-breaker
│   └── kafka/user_event/ #   Kafka event publisher implementing auth/port.IUserEventPublisher
│
├── proto/user/           # Protobuf contract for user gRPC service
│
└── modules/              # ★ BUSINESS MODULES (Core Domains & Ports)
    ├── auth/             #   Login, logout, token refresh, JWKS endpoint
    │   ├── domain/       #     Domain entities & commands (Login, Token, UserIdentity)
    │   ├── port/         #     Outbound port interfaces (IUserFinder, IUserEventPublisher)
    │   ├── service/      #     Business logic & use-cases (AuthService, AuthServiceDeps)
    │   └── handler/httphandler/ # Inbound delivery adapter (DTO, Gin handler, RouteRegistrar)
    ├── user/             #   User profile read/update backed by Postgres repository
    │   ├── domain/       #     Domain entities & models (User)
    │   ├── repository/   #     GORM database persistence (IUserRepository)
    │   ├── service/      #     Business logic & use-cases (UserService, UserServiceDeps)
    │   └── handler/      #     Inbound delivery: httphandler/ and grpchandler/
    ├── storage/          #   File upload/download/presigned-url via Object Storage SDK
    │   ├── domain/       #     Domain models (UploadFileResult, PresignedURLResult, etc.)
    │   ├── service/      #     Business logic (StorageService, StorageServiceDeps)
    │   └── handler/httphandler/ # HTTP endpoints for storage operations
    └── notifications/    #   Inbound event consumer for user events (Kafka subscriber)
        ├── service/      #     Push notification business logic (NotificationService)
        └── handler/messaging/ # Kafka consumer handler and topic subscribers
```

---

## Architecture Pattern: Hexagonal (Ports & Adapters)

Each business module in `modules/` is designed as an isolated bounded context:

```
                  ┌──────────────────────────────────────────────┐
                  │                 modules/<name>               │
                  │                                              │
 [HTTP Request] ─>│ ┌──────────────┐     ┌──────────────┐        │
 [gRPC Request] ─>│ │   handler/   │ ──> │   service/   │        │
 [Kafka Event]  ─>│ │  (Inbound    │     │  (Use Cases  │        │
                  │ │   Adapters)  │     │   & Logic)   │        │
                  │ └──────────────┘     └──────┬───────┘        │
                  │                             │                │
                  │                      ┌──────┴───────┐        │
                  │                      ▼              ▼        │
                  │               ┌────────────┐ ┌─────────────┐ │
                  │               │repository/ │ │   port/     │ │
                  │               │(Internal DB│ │(Outbound    │ │
                  │               │ Interface) │ │ Interfaces) │ │
                  │               └─────┬──────┘ └──────┬──────┘ │
                  └─────────────────────┼───────────────┼────────┘
                                        │               │
                                        ▼               ▼
                                 [Postgres DB]   [integration/]
                                                 • gRPC Client
                                                 • Kafka Producer
                                                 • External API
```

### Key Principles

1. **Dependency Inversion**:
   - `service/` **never imports** transport packages (`gin`, `net/http`, `google.golang.org/grpc`).
   - `service/` **never imports** external integration clients (`integration/grpc/...`, `proto/...`).
   - When a service needs external data or actions, it defines an **Outbound Port Interface** in `modules/<name>/port/`.
   - The integration adapter in `integration/` implements the port and translates wire data into domain `domain/`.
2. **Decoupled Models**:
   - `domain/` holds clean domain entities, commands, and value objects with zero transport or framework tags.
   - HTTP request validation tags (`binding:"required"`) live in `handler/httphandler/http_dto.go`, mapped to `domain/` via `http_mapper.go`.
3. **Accept Interfaces, Return Structs**:
   - Constructor functions in `integration/` return concrete structs (`*ClientUser, error`).
   - Uber Fx binds them to module ports using `fx.As(new(port.IUserFinder))`.

---

## Anatomy of a Business Module

Use this directory structure as a template when creating a new module:

```
modules/<name>/
├── domain/                # Domain Entities, Value Objects & Commands (pure Go structs)
│   ├── <entity>.go
│   └── command.go
│
├── port/                  # (Optional) Outbound Port Interfaces (external dependencies)
│   └── <service>_finder.go# e.g., type IUserFinder interface { ... }
│
├── repository/            # (Optional) Internal Database Persistence
│   └── <entity>_repo.go   # Interface & SQL/GORM implementation for local tables
│
├── service/               # Pure Business Logic & Use-Cases
│   ├── <name>_service.go  # Core implementation
│   ├── <name>_deps.go     # fx.In dependency container struct
│   ├── <name>_test.go     # In-memory unit tests using mock ports & repositories
│   └── validator.go       # Domain-level validation rules
│
├── handler/               # Inbound Delivery Adapters
│   ├── httphandler/       #   HTTP (Gin) delivery: http_dto.go, http_handler.go, http_mapper.go, http_routes.go
│   ├── grpchandler/       #   (Optional) gRPC delivery: grpc_handler.go, grpc_mapper.go, grpc_service.go
│   └── messaging/         #   (Optional) Kafka consumer: dto.go, handler.go, subscribers.go
│
└── module.go              # fx.Module bundle registering service, handlers & registrars
```

---

## Step-by-Step: Adding a New Business Module

Here is the exact recipe to build a new module (e.g. `order`):

### Step 1: Define Domain Models (`modules/order/domain/`)
Write clean domain structs and commands with zero external framework dependencies:
```go
package domain

type Order struct {
    ID          string
    UserID      string
    TotalAmount int64
    Status      string
}

type CreateOrderCmd struct {
    UserID      string
    TotalAmount int64
}
```

### Step 2: Define Outbound Ports (`modules/order/port/`)
If the order service needs to call another service (e.g., check user profile or debit balance), declare an interface in `port/`:
```go
package port

import (
    "context"
    "auth-service/modules/order/domain"
)

type IUserAccountFinder interface {
    GetUserAccount(ctx context.Context, userID string) (*domain.UserAccount, error)
}
```

### Step 3: Define Local Repository (`modules/order/repository/`)
If the module owns tables in the database:
```go
package repository

import (
    "context"
    "auth-service/modules/order/domain"
)

type IOrderRepository interface {
    Create(ctx context.Context, order *domain.Order) error
    FindByID(ctx context.Context, id string) (*domain.Order, error)
}
```

### Step 4: Write Service & Dependencies (`modules/order/service/`)
Bundle dependencies using `fx.In`:
```go
// order_deps.go
package service

import (
    "go.uber.org/fx"
    "github.com/agilsyofian/gosdk/core/observability/logger"
    "auth-service/modules/order/port"
    "auth-service/modules/order/repository"
)

type OrderServiceDeps struct {
    fx.In
    Logger     logger.Logger
    Repo       repository.IOrderRepository
    UserFinder port.IUserAccountFinder
}
```
Implement business logic in `order_service.go` against `IOrderService` interface, with constructor `func New(deps OrderServiceDeps) IOrderService`.

### Step 5: Implement Inbound HTTP Handler (`modules/order/handler/httphandler/`)
- `http_dto.go`: Request/response structs with Gin binding tags (`binding:"required"`).
- `http_mapper.go`: Converts DTO ↔ Domain model.
- `http_handler.go`: Binds request, invokes service, writes HTTP response. Provides `func New(service service.IOrderService, ...) *OrderHandler`.
- `http_routes.go`: Implements `httpserver.RouteRegistrar`:
  ```go
  package httphandler

  import (
      "auth-service/app/server/httpserver"
      "github.com/gin-gonic/gin"
  )

  type RouteRegistrar struct {
      handler *OrderHandler
  }

  func NewRouteRegistrar(h *OrderHandler) httpserver.RouteRegistrar {
      return &RouteRegistrar{handler: h}
  }

  func (r *RouteRegistrar) RegisterRoutes(router *gin.RouterGroup) {
      g := router.Group("/v1/orders")
      g.POST("", r.handler.CreateOrder)
  }
  ```

### Step 6: Assemble the Module (`modules/order/module.go`)
Provide all components and register routes into the Fx group:

```go
package order

import (
	"auth-service/app/server/httpserver"
	"auth-service/modules/order/handler/httphandler"
	"auth-service/modules/order/repository"
	"auth-service/modules/order/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-order",
	fx.Provide(
		repository.New,
		service.New,
		httphandler.New,
		httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
```

### Step 7: Wire in Outbound Adapter (`integration/`)
If using `port.IUserAccountFinder`, implement the method in `integration/grpc/user/client.go`:
```go
func (c *ClientUser) GetUserAccount(ctx context.Context, userID string) (*orderDomain.UserAccount, error) {
    // Call gRPC User Service and map proto to orderDomain.UserAccount
}
```
And bind in `integration/grpc/user/module.go`:
```go
fx.Provide(
    fx.Annotate(
        NewClientUser,
        fx.As(new(authPort.IUserFinder)),
        fx.As(new(orderPort.IUserAccountFinder)),
        fx.OnStop(func(ctx context.Context, f authPort.IUserFinder) error {
            if c, ok := f.(*ClientUser); ok { return c.Close() }
            return nil
        }),
    ),
)
```

### Step 8: Add to `main.go`
Simply add `order.Module` to the `fx.New(...)` list in `main.go`. Route registration and dependency resolution happen automatically!

