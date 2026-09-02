# Messaging Module Steering

This document defines the implementation rules for AI agents working on:

- `messaging/kafka`
- `messaging/rabbitmq`

The goal is to provide a consistent, production-grade messaging API across Kafka and RabbitMQ while preserving the protocol-specific semantics of each broker.

This module is **shared infrastructure**. Treat every public API and lifecycle decision as a long-term contract used by multiple squads.

---

## 1. Scope

`messaging/*` is a Layer 2 Communication module.

It is responsible for:

- Producer / publisher lifecycle
- Consumer lifecycle
- Message envelope
- Message headers / metadata
- Serialization boundaries
- Context propagation
- Consumer handler execution
- Acknowledgement semantics
- Graceful shutdown
- Broker connection lifecycle
- Error classification
- Dead-letter integration points
- Retry integration points
- Observability extension points

It is **not** responsible for:

- Business logic
- Domain event definitions
- Banking transaction rules
- Authentication / authorization
- Direct database access
- Application configuration loading
- Application bootstrap
- Vendor-specific observability wiring
- Hardcoded topic / queue names
- Squad-specific retry policies

---

## 2. Package Structure

Keep Kafka and RabbitMQ as independent leaf packages.

Preferred structure:

```text
messaging/
├── kafka/
│   ├── doc.go
│   ├── kafka.go
│   ├── options.go
│   ├── producer.go
│   ├── consumer.go
│   ├── message.go
│   ├── errors.go
│   ├── kafka_test.go
│   ├── producer_test.go
│   └── consumer_test.go
│
└── rabbitmq/
    ├── doc.go
    ├── rabbitmq.go
    ├── options.go
    ├── producer.go
    ├── consumer.go
    ├── message.go
    ├── errors.go
    ├── rabbitmq_test.go
    ├── producer_test.go
    └── consumer_test.go
```

Do not create:

```text
messaging/common/
messaging/base/
messaging/core/
```

unless a concrete duplication problem has been demonstrated and an architectural decision has been made.

Do not make Kafka depend on RabbitMQ or RabbitMQ depend on Kafka.

---

## 3. Dependency Rules

`messaging/*` may depend on:

- Go standard library
- `foundation/*`
- `internal/*`

Messaging packages must not directly import:

- `app`
- `config`
- `transport/*`
- `security/*`
- `resilience/*`
- `observability/*`

Cross-cutting functionality must be connected through interfaces / extension points.

Do not directly couple Kafka or RabbitMQ implementations to a concrete retry, tracing, metrics, or vendor observability package.

---

## 4. Kafka and RabbitMQ Must Have Conceptual API Parity

Kafka and RabbitMQ are different protocols and must not be forced into an identical abstraction where their semantics differ.

However, common concepts should use consistent naming and API shape.

Expected conceptual mapping:

| Concept | Kafka | RabbitMQ |
|---|---|---|
| Publish | Publish | Publish |
| Consume | Consume | Consume |
| Message | Message | Message |
| Producer | Producer | Producer |
| Consumer | Consumer | Consumer |
| Handler | Handler | Handler |
| Headers | Headers | Headers |
| Payload | Payload | Payload |
| Key | Key / partition key | Routing-related metadata |
| Topic | Topic | Exchange / Queue |
| Partition | Partition | N/A |
| Offset | Offset | Delivery tag |
| Consumer group | Group | Consumer |
| Ack | Commit offset | Ack |
| Nack | Consumer recovery / offset strategy | Nack |

Do not hide important broker semantics merely to make the APIs look identical.

---

## 5. Primary API Shape

Follow the framework API guidelines.

Primary types should be constructed with:

```go
func New(required Args, opts ...Option) (*T, error)
```

The primary object must be safe for concurrent use.

Conceptual shape:

```go
type Client struct {
    // ...
}

func New(cfg Config, opts ...Option) (*Client, error)
```

Mandatory principles:

- Constructor returns `(*T, error)`
- Invalid configuration returns an error
- Never panic for invalid user configuration
- No package-level mutable state
- One shared client may be used by multiple goroutines
- Lifecycle must be explicit
- Context must be accepted by I/O operations

---

## 6. Configuration

Messaging packages must receive typed configuration.

They must never read environment variables directly.

Do not implement:

```go
os.Getenv("KAFKA_BROKERS")
os.Getenv("RABBITMQ_URL")
```

inside messaging packages.

Configuration loading belongs to `config/`.

Messaging receives already-resolved configuration.

Required configuration must be explicit.

Optional configuration should use functional options where appropriate.

---

## 7. Producer Requirements

Producer APIs must:

- Accept `context.Context`
- Return errors from broker operations
- Preserve context cancellation
- Support message metadata
- Avoid hidden retries
- Avoid hidden timeouts
- Be safe for concurrent use
- Support graceful close
- Expose enough information for metrics/tracing hooks

Conceptual API:

```go
func (p *Producer) Publish(
    ctx context.Context,
    msg *Message,
) error
```

For APIs expected to grow:

```go
func (p *Producer) Publish(
    ctx context.Context,
    req *PublishRequest,
) (*PublishResponse, error)
```

Do not introduce multiple methods such as `Send`, `Publish`, `Produce`, and `Push` for the same operation.

Use `Publish` consistently unless protocol semantics require otherwise.

---

## 8. Consumer Requirements

Consumer APIs must:

- Accept a `context.Context`
- Support graceful shutdown
- Never leak goroutines
- Propagate cancellation
- Isolate handler execution from broker implementation details
- Define acknowledgement behavior explicitly
- Define what happens when a handler returns an error
- Avoid silently swallowing handler errors
- Avoid infinite retry loops
- Be safe for concurrent operation where supported by the broker

Conceptual handler:

```go
type Handler func(
    ctx context.Context,
    msg *Message,
) error
```

Conceptual consumer:

```go
func (c *Consumer) Consume(
    ctx context.Context,
    handler Handler,
) error
```

If the broker supports multiple consumption models, expose them explicitly rather than pretending they are identical.

---

## 9. Message Envelope

Messaging should provide a framework-level message envelope.

The envelope must contain transport-neutral metadata where possible.

Conceptual shape:

```go
type Message struct {
    ID          string
    Key         []byte
    Payload     []byte
    Headers     map[string]string
    ContentType string
}
```

The exact fields may differ between Kafka and RabbitMQ where protocol semantics require it.

Do not put domain-specific fields into the framework message.

Bad:

```go
type Message struct {
    UserID        string
    TransactionID string
    Amount        decimal.Decimal
}
```

Those belong to consuming services.

The framework should treat the payload as opaque.

---

## 10. Payload Serialization

Messaging must not impose a domain serialization format.

Support opaque payloads:

```go
[]byte
```

The messaging layer should not automatically assume JSON unless explicitly configured.

Preferred flow:

```text
Broker
  ↓
Messaging Message
  ↓
[]byte payload
  ↓
Consuming service
  ↓
Domain struct
```

If helper serialization is added later, it must remain optional and must not become a mandatory dependency of the core broker implementation.

---

## 11. Context Propagation

Every I/O operation must accept:

```go
context.Context
```

Context cancellation must propagate to:

- Broker connection operations
- Publish
- Consume
- Handler execution
- Shutdown

Do not create unrelated contexts inside the messaging implementation.

Avoid:

```go
context.Background()
```

inside request/message processing paths.

Use the incoming context.

---

## 12. Correlation and Trace Metadata

Messaging must support propagation of framework metadata through message headers.

At minimum, the design must allow propagation of:

- Trace ID
- Request ID
- Correlation ID

The messaging package should not depend directly on a tracing vendor.

Producer flow:

```text
Context
  ↓
Messaging
  ↓
Message Headers
  ↓
Broker
```

Consumer flow:

```text
Broker Headers
  ↓
Messaging
  ↓
Context
  ↓
Handler
```

Do not generate a new correlation identity if one already exists.

---

## 13. Observability Extension Points

Messaging must provide extension points for:

- Tracing
- Metrics
- Logging

Do not import a concrete vendor SDK into the messaging package merely for observability.

Keep interfaces small and focused.

Do not create a large interface containing unrelated observability responsibilities.

---

## 14. Logging

Use the injected `foundation/logger` abstraction.

Never use:

```go
fmt.Println()
log.Println()
fmt.Printf()
```

for framework operational logging.

Minimum useful lifecycle events:

### Debug

- Publish metadata
- Consume metadata
- Topic / queue
- Operation duration
- Message ID
- Correlation metadata

Never log raw payload by default.

### Info

- Client started
- Client stopped
- Consumer started
- Consumer stopped
- Consumer rebalance where applicable
- Connection established

### Warn

- Retry attempt
- Temporary broker failure
- Reconnection
- Recoverable handler failure

### Error

- Final publish failure
- Consumer termination
- Non-recoverable broker error
- Final retry exhaustion

Never log:

- credentials
- passwords
- broker URLs containing credentials
- access tokens
- raw sensitive payloads
- unrestricted message bodies

---

## 15. Retry

Messaging must not silently implement its own generic retry policy.

Retry belongs to `resilience/retry`.

Messaging may expose the extension point necessary for retry composition.

The desired architecture is:

```text
Broker
   ↓
messaging/kafka
   ↓
retry policy
   ↓
consumer handler
```

or:

```text
Broker
   ↓
messaging/rabbitmq
   ↓
retry policy
   ↓
consumer handler
```

Do not hardcode retry counts or delays inside the messaging implementation.

If broker-level retry is unavoidable due to protocol semantics, clearly distinguish:

1. Broker transport recovery
2. Handler retry
3. Message redelivery
4. Application-level retry

These are different concepts.

---

## 16. Dead Letter Handling

Dead-letter behavior must be explicit.

Never silently discard a failed message.

Kafka may use:

```text
source topic
    ↓
consumer
    ↓
handler
    ↓
retry
    ↓
dead-letter topic
```

RabbitMQ may use:

```text
exchange
    ↓
queue
    ↓
consumer
    ↓
handler
    ↓
dead-letter exchange
    ↓
dead-letter queue
```

Do not force Kafka's DLQ model onto RabbitMQ or vice versa.

The implementation must document:

- When a message is dead-lettered
- Who owns the retry count
- How retry metadata is represented
- Whether original message metadata is preserved
- Whether the original error is preserved
- Whether acknowledgement / offset commit happens before or after DLQ publication

A message must not be acknowledged/committed before the implementation has established the intended delivery outcome.

---

## 17. Acknowledgement Semantics

Acknowledgement is one of the most important protocol-specific behaviors.

AI agents must not implement acknowledgement based on assumptions.

### Kafka

Kafka acknowledgement generally maps to offset management.

The implementation must clearly define:

- When offsets are committed
- What happens on handler success
- What happens on handler failure
- What happens on process shutdown
- Whether processing is at-least-once
- How duplicate delivery is expected to be handled

Do not claim exactly-once semantics unless the implementation actually guarantees them.

### RabbitMQ

RabbitMQ acknowledgement must clearly define:

- Auto-ack vs manual ack
- Ack after successful handler
- Nack behavior
- Requeue behavior
- Dead-letter behavior
- Consumer cancellation behavior

Default toward explicit/manual acknowledgement when message processing success determines acknowledgement.

---

## 18. Delivery Semantics

The public documentation must state the delivery guarantee.

Prefer conservative claims.

If the implementation provides:

```text
at-least-once delivery
```

document that consumers must be idempotent.

Do not claim:

```text
exactly-once
```

merely because the underlying broker has features related to transactions or idempotent producers.

Exactly-once semantics are end-to-end properties and must not be inferred from one broker feature.

---

## 19. Ordering

Ordering must be documented according to the actual broker semantics.

### Kafka

Ordering is normally scoped to a partition.

Never document Kafka as globally ordered unless the implementation explicitly enforces a single partition.

### RabbitMQ

Ordering depends on queue, consumer, acknowledgement, prefetch, concurrency, redelivery, and topology behavior.

Do not promise ordering when concurrent consumers or requeue behavior can change delivery order.

---

## 20. Concurrency

Messaging clients must be designed for concurrent use.

Producer:

```text
multiple goroutines
       ↓
     Producer
       ↓
     Broker
```

Consumer concurrency must be explicit.

Do not accidentally create unbounded goroutines:

```go
go handler(ctx, msg)
```

for every message without a bounded concurrency strategy.

If concurrency is configurable:

```go
Concurrency int
```

validate it.

Do not allow invalid concurrency configuration to create undefined behavior.

---

## 21. Backpressure

Consumer implementations must have a defined backpressure strategy.

Examples:

- Kafka partition consumption
- RabbitMQ prefetch
- bounded worker pool
- bounded internal channel

Avoid unbounded buffering.

Never introduce an unbounded queue simply to prevent the consumer loop from blocking.

Backpressure is preferable to uncontrolled memory growth.

---

## 22. Shutdown

Graceful shutdown is mandatory.

Expected lifecycle:

```text
Start
  ↓
Running
  ↓
Shutdown signal
  ↓
Stop accepting new work
  ↓
Finish in-flight work
  ↓
Commit/Ack according to outcome
  ↓
Close consumer
  ↓
Close producer
  ↓
Close broker connection
```

Shutdown must respect `context.Context`.

Do not use `time.Sleep()` as a shutdown synchronization mechanism.

Use:

- channels
- `sync.WaitGroup`
- context cancellation
- broker lifecycle primitives

Shutdown should be idempotent where practical.

Calling `Close()` multiple times must not panic.

---

## 23. Connection Management

Connection lifecycle belongs to the messaging implementation.

The implementation should handle:

- Initial connection
- Connection failure
- Reconnection where supported
- Graceful close
- Context cancellation

Do not continuously reconnect in a tight loop.

If reconnection is implemented, use a bounded/backoff strategy and make the behavior observable.

Connection recovery must not create CPU/network storms.

---

## 24. Error Design

Errors must follow `api-guidelines.md`.

Every broker-originated error should retain the underlying error:

```go
fmt.Errorf("kafka.publish: %w", err)
```

or through a typed module error.

Errors must allow callers to distinguish:

- Invalid configuration
- Invalid message
- Context cancellation
- Context deadline
- Broker unavailable
- Publish failure
- Consumer failure
- Handler failure
- Shutdown
- Retry exhaustion

Do not convert every failure into:

```go
errors.New("messaging error")
```

because that destroys operational information.

Preserve:

```go
errors.Is(...)
errors.As(...)
```

behavior.

---

## 25. Context Errors

Never wrap context errors in a way that prevents inspection.

This must continue to work:

```go
errors.Is(err, context.Canceled)
```

and:

```go
errors.Is(err, context.DeadlineExceeded)
```

This is important for resilience and operational handling.

---

## 26. Configuration Validation

Constructors must validate configuration before opening broker connections where practical.

### Kafka

Validate:

- Brokers are present
- Required topic/group configuration exists
- Invalid numeric configuration is rejected
- Invalid security configuration is rejected

### RabbitMQ

Validate:

- URL / connection information exists
- Required exchange/queue configuration exists
- Invalid consumer configuration is rejected

Do not defer obvious configuration errors until the first Publish/Consume call.

---

## 27. Security

Messaging credentials are secrets.

Never:

- log broker passwords
- expose credentials in errors
- include credentials in message headers
- store credentials in source code
- print connection URLs containing passwords

TLS configuration must be explicit.

Do not silently disable TLS certificate verification in production-oriented configuration.

---

## 28. Message Size

Do not assume messages are arbitrarily large.

Kafka and RabbitMQ have different practical and configured limits.

The implementation should:

- Validate configured message limits where appropriate
- Return an explicit error when the payload cannot be published
- Avoid copying payloads unnecessarily
- Avoid buffering unlimited data

Do not automatically compress payloads unless compression is explicitly configured.

---

## 29. Resource Ownership

The component that creates the messaging client owns its lifecycle.

Example:

```go
client, err := kafka.New(...)
if err != nil {
    return err
}

defer client.Close()
```

Do not create hidden global broker clients.

Do not create a new broker connection for every Publish call.

Prefer long-lived producer/consumer instances.

---

## 30. Public API Minimalism

Before adding an exported method, ask:

1. Is it required by an actual use case?
2. Can an existing method satisfy the use case?
3. Is it protocol-specific?
4. Does it expose an implementation detail?
5. Can it be added later without breaking callers?

Avoid speculative APIs.

Do not expose the underlying Kafka/RabbitMQ client merely because it is convenient.

Bad:

```go
func (c *Client) RawKafkaClient() *kafka.Client
```

This leaks the implementation and defeats the abstraction.

---

## 31. Protocol-Specific Escape Hatches

Protocol-specific features are allowed when genuinely necessary.

Kafka examples:

- Partition
- Offset
- Consumer group
- Rebalance
- Transactional producer configuration

RabbitMQ examples:

- Exchange
- Routing key
- Queue
- Delivery tag
- Prefetch
- Nack/Requeue

Protocol-specific features must remain clearly scoped.

Do not pollute a common conceptual API with fields that only make sense for one broker.

---

## 32. Testing

Every messaging implementation must have unit tests.

Unit tests must not require a real Kafka or RabbitMQ broker.

Use:

- Fake producer
- Fake consumer
- Mock transport seam
- In-memory test helper

Required scenarios include:

### Producer

- Successful publish
- Broker publish error
- Context cancellation
- Context deadline
- Invalid message
- Close behavior
- Concurrent publish

### Consumer

- Successful handler
- Handler error
- Context cancellation
- Ack/commit behavior
- Nack/requeue behavior
- DLQ behavior
- Shutdown while processing
- Concurrent processing
- Broker connection failure

### Lifecycle

- Start
- Stop
- Double close
- Failure during startup
- Failure during shutdown

---

## 33. Integration Tests

Real broker tests must be separated from normal unit tests.

Use a build tag such as:

```go
//go:build integration
```

Integration tests may use:

```text
Kafka
RabbitMQ
```

but must never use production credentials or production infrastructure.

The default:

```bash
go test ./...
```

must not require Kafka or RabbitMQ to be running.

---

## 34. Race Testing

Messaging is concurrency-heavy.

Changes involving:

- consumer workers
- producer concurrency
- lifecycle
- shared state
- connection recovery

should be validated with:

```bash
go test -race ./messaging/...
```

Do not introduce package-level mutable state to avoid race detector failures.

---

## 35. Examples

Every messaging implementation must have a runnable example.

Expected:

```text
examples/
└── kafka/
```

and, when RabbitMQ has a runnable integration story:

```text
examples/
└── rabbitmq/
```

Examples should demonstrate:

- Configuration
- Client construction
- Producer
- Consumer
- Context
- Graceful shutdown
- Error handling

Examples must not contain business-specific banking logic.

---

## 36. Documentation Requirements

When implementing a messaging feature, update relevant documentation when behavior changes.

At minimum document:

- Delivery semantics
- Acknowledgement semantics
- Retry behavior
- Dead-letter behavior
- Ordering guarantees
- Concurrency behavior
- Shutdown behavior
- Configuration
- Security
- Operational considerations

Significant lifecycle changes should also update architecture documentation / Mermaid flow.

---

## 37. AI Implementation Workflow

When asked to implement a messaging feature, follow this order.

### Step 1 — Read project rules

Read:

```text
AI_AGENT.md
.instructions/project-principles.md
.instructions/architecture.md
.instructions/package-rules.md
.instructions/coding-style.md
.instructions/api-guidelines.md
.instructions/testing.md
.instructions/observability.md
.instructions/security.md
.instructions/roadmap.md
.instructions/messaging.md
```

If any referenced file does not exist, do not invent its content. Continue using the available project rules.

---

### Step 2 — Inspect the existing package

Before creating code, inspect:

```text
messaging/kafka/
messaging/rabbitmq/
```

Inspect existing:

- Public APIs
- Tests
- Dependencies
- Documentation
- Examples
- Configuration
- Error types

Never overwrite an existing design without understanding its compatibility impact.

---

### Step 3 — Decide whether the feature is generic or protocol-specific

Ask:

```text
Can Kafka and RabbitMQ both support this concept?
```

If yes:

- Keep conceptual API names aligned.

If no:

- Implement it in the protocol-specific package.
- Do not invent a fake common abstraction.

---

### Step 4 — Design the contract before implementation

Define:

- Primary type
- Constructor
- Request / response
- Errors
- Lifecycle
- Context behavior
- Concurrency behavior
- Acknowledgement semantics

Only then implement broker-specific code.

---

### Step 5 — Implement the smallest viable API

Follow YAGNI.

Do not implement:

- speculative interfaces
- unused abstractions
- generic broker frameworks
- future protocol support
- unnecessary adapters

---

### Step 6 — Add tests immediately

Every public behavior must have tests.

Tests must verify behavior rather than implementation details.

---

### Step 7 — Validate concurrency and lifecycle

For consumer-related changes explicitly verify:

```text
Start
Consume
Cancel
Shutdown
Close
```

and ensure:

```text
no goroutine leak
no panic
no double close failure
no unbounded buffering
```

---

### Step 8 — Verify observability

Confirm:

- No sensitive payload logging
- Correlation metadata is preserved
- Errors contain operation context
- Metrics/tracing hooks remain possible
- Broker failures are observable

---

### Step 9 — Verify dependency boundaries

Before finalizing, confirm that messaging did not import:

```text
app
config
transport
security
resilience
observability
```

unless an explicit architectural decision allows it.

---

## 38. Anti-Patterns

AI agents must reject or flag designs containing:

### Global broker client

```go
var kafkaClient *Client
```

### Hidden environment access

```go
os.Getenv("KAFKA_BROKERS")
```

### Hidden timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
```

### Hidden retry

```go
for i := 0; i < 3; i++ {
    publish()
}
```

### Unbounded consumer goroutines

```go
go handler(ctx, msg)
```

for every message without bounded concurrency.

### Silent message loss

```go
if err != nil {
    return nil
}
```

### Raw payload logging

```go
logger.Info(ctx, "message", "payload", msg.Payload)
```

### Leaking vendor clients

```go
func (c *Client) RawClient() any
```

### Business logic in messaging

```go
if msg.Event == "transfer.created" {
    updateBalance()
}
```

### Fake exactly-once guarantee

```go
// Guaranteed exactly once.
```

unless the entire implementation actually establishes that guarantee.

---

## 39. Definition of Done

A messaging implementation is not complete until:

- [ ] Public API follows framework API guidelines
- [ ] Kafka/RabbitMQ semantics are correctly represented
- [ ] Constructor validates configuration
- [ ] Context is propagated
- [ ] Client is concurrency-safe
- [ ] Lifecycle is explicit
- [ ] Graceful shutdown works
- [ ] No hidden timeout exists
- [ ] No hidden retry policy exists
- [ ] Acknowledgement behavior is documented
- [ ] Delivery semantics are documented
- [ ] DLQ behavior is explicit
- [ ] Errors preserve underlying causes
- [ ] Sensitive data is not logged
- [ ] Observability extension points exist where required
- [ ] Unit tests do not require real brokers
- [ ] Integration tests are isolated behind an integration tag
- [ ] Race-sensitive code has been considered
- [ ] Runnable example exists or is updated
- [ ] Architecture documentation is updated when flow changes
- [ ] No forbidden dependency was introduced
- [ ] No unnecessary public API was introduced

---

## 40. Guiding Principle

The messaging layer should make the **common path easy and the dangerous path explicit**.

A consuming squad should be able to follow:

```text
Create client
    ↓
Publish
    ↓
Consume
    ↓
Handle
    ↓
Ack / Commit
    ↓
Graceful shutdown
```

without understanding the internals of Kafka or RabbitMQ.

At the same time, the framework must never hide important delivery semantics.

**Abstraction must remove accidental complexity, not operational truth.**
