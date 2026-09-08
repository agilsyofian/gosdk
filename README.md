# Go SDK

> Production-ready, modular enterprise Go SDK & building blocks for microservices.

---

## ⚡ What is this SDK? (30-Second Overview)

**`gosdk`** is an enterprise-grade toolkit for building reliable, scalable microservices in Go. Instead of a rigid monolithic framework, it is designed as a collection of **independent, plug-and-play Go modules** organized via a Go workspace (`go.work`).

### 🧱 Core Capabilities

| Pillar | What's Included |
|---|---|
| 🌐 **Transport** | Clean HTTP (Gin) & gRPC servers/clients, middleware, decentralized route registrars, idempotency engine |
| 🔒 **Security** | JWT authenticator, RBAC authorization, Redis & Memory session managers, JWKS, HMAC signatures |
| 💾 **Storage** | SQL databases (PostgreSQL, MySQL, Oracle), Caching (Redis, Zero-dep Sharded Local Cache), Object Storage (S3/MinIO, GCS) |
| 📨 **Messaging** | Kafka & RabbitMQ publishers and topic consumer workers |
| 📊 **Observability** | Structured logging (Zerolog), OpenTelemetry tracing context, Kubernetes health & readiness checks |
| 🛡️ **Resilience** | Configurable Retry, Circuit Breaker, Rate Limiting, and Timeout policies |

---

## 🚀 Quickstart: Run the Full Example App (1 Minute)

The fastest way to see the SDK in action is the complete runnable application in **[`examples/`](examples/user-service/)**:

```bash
# 1. Start infrastructure (Postgres, Redis, Kafka, MinIO)
docker compose up -d

# 2. Run database migration (creates schema & seed user)
cd tools/dbmigration
DB_DSN="postgres://root:root@localhost:5432/postgres?sslmode=disable" go run ./cmd/migrate up

# 3. Run the application
cd ../../examples
go run .
```

*HTTP server starts on `:8001` and gRPC on `:8081`.*

👉 **See [`examples/README.md`](examples/user-service/README.md)** for the complete end-to-end walkthrough of login, gRPC calls, Redis sessions, and Kafka event delivery.

---

## 🏛️ Architecture & Best Practices

All business modules in the SDK follow **Hexagonal Architecture (Ports & Adapters)** and are assembled using **[Uber Fx](https://github.com/uber-go/fx)**:

```text
[HTTP / gRPC / Kafka Inbound]
             │
             ▼
      handler/ (httphandler, grpchandler, messaging)
             │
             ▼
      service/ (Pure Business Logic & Use Cases)
       ├──> repository/ (Internal DB Persistence via GORM/SQL)
       └──> port/       (Outbound Interfaces -> implemented by integration/)
             │
             ▼
  [Postgres / External APIs / Kafka Outbound]
```

### 3 Key Principles for Developers
1. **Dependency Inversion**: Domain models (`domain/`) and business logic (`service/`) **never** import transport packages (Gin, gRPC) or outbound network clients.
2. **Decentralized Registration**: Modules self-register their own HTTP routes, gRPC services, and Kafka subscribers into Fx groups (`httpserver.AsRouteRegistrar`, `grpcserver.AsGRPCServiceRegistrar`, `consumer.AsTopicSubscriber`).
3. **Accept Interfaces, Return Structs**: Outbound network adapters in `integration/` return concrete structs and satisfy module `port/` interfaces via `fx.As`.

👉 **See [`examples/modules/README.md`](examples/user-service/modules/README.md)** for a 5-minute recipe to build and wire a new business module.

---

## 📁 Repository Layout at a Glance

```text
SDK/
├── core/              # Standard errors, resilience, logger, tracing, health
├── transport/         # HTTP & gRPC server/client engines, idempotency
├── security/          # JWT, sessions, RBAC, JWKS, crypto
├── storage/           # Database (Postgres, MySQL, Oracle), Cache (Redis, Local), Object Storage (S3, GCS)
├── messaging/         # Kafka & RabbitMQ integrations
├── utils/             # Concurrency & cryptographic helpers
├── tools/             # dbmigration, mapgen
└── examples/          # ★ Gold-standard runnable reference app & module blueprints
```

---

## 🛠️ Common Commands

```bash
# Run tests across all modules
make test

# Format and vet code
make fmt && make vet

# Run all pre-commit quality checks (lint, test, vuln)
make check
```

---

## 📚 Further Reading

- [Examples Reference Guide](examples/user-service/README.md) — Comprehensive guide to the runnable example application.
- [Creating Business Modules](examples/user-service/modules/README.md) — Step-by-step recipe for building new modules.
- [Release & Tagging Guide](.instructions/release.md) — Monorepo SemVer tagging and consumption instructions for maintainers.
- [Architecture & Coding Instructions](.instructions/) — Detailed coding standards and guidelines for contributors.
