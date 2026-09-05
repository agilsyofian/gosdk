package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/core/observability/health"
)

// Ensure Client implements health.Check interface
var _ health.Check = (*Client)(nil)

// Config is the fully-resolved configuration required to construct a
// Client. Messaging never reads environment variables directly
// (messaging.md section 6) — config loading/resolution belongs to the
// application's config package.
type Config struct {
	// URL is the AMQP connection URI, e.g.
	// "amqp://user:pass@localhost:5672/vhost" or "amqps://...". Required.
	URL string
}

func (c Config) validate() error {
	u := strings.TrimSpace(c.URL)
	if u == "" {
		return fmt.Errorf("%w: URL is required", ErrInvalidConfig)
	}
	if !strings.HasPrefix(u, "amqp://") && !strings.HasPrefix(u, "amqps://") {
		return fmt.Errorf("%w: URL must use the amqp:// or amqps:// scheme", ErrInvalidConfig)
	}
	return nil
}

// closer is satisfied by both Producer and Consumer, letting Client
// cascade Close across everything it spawned.
type closer interface {
	Close() error
}

// amqpConnection is the subset of *amqp.Connection Client depends on. It
// exists as a test seam for Client.Close (messaging.md section 32):
// *amqp.Channel returned by the real Channel() method has no
// unexported-free way to construct outside a live connection, so
// Producer/Consumer construction success is exercised by integration
// tests, while Client's own validation and shutdown logic is exercised
// here against a fake.
type amqpConnection interface {
	Channel() (*amqp.Channel, error)
	Close() error
}

// Client owns a single AMQP connection and constructs Producer and
// Consumer instances as channels multiplexed over it — the natural
// mapping for RabbitMQ's connection model, where one TCP connection
// commonly serves many channels (messaging.md section 23, section 29:
// prefer long-lived instances over reconnecting per operation). Client
// is safe for concurrent use.
//
// Unlike messaging/kafka.Client, New here dials the broker immediately:
// RabbitMQ's connection is a real stateful resource up front, not a lazy
// per-partition abstraction, so failing fast here is the more honest
// match for the protocol (messaging.md section 4 — do not force
// identical abstractions where semantics differ).
//
// Reconnection is not implemented in this version: if the connection
// drops, in-flight Producer/Consumer instances will start returning
// errors, and the caller is expected to construct a new Client. Adding
// bounded/backoff reconnection (messaging.md section 23) is a natural
// extension point, deliberately left out for now rather than shipped
// half-specified — see doc.go.
type Client struct {
	conn amqpConnection
	opts clientOptions

	mu       sync.Mutex
	closed   bool
	children []closer
}

// New validates cfg, dials the broker, and returns a Client.
func New(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(&o)
	}

	conn, err := amqp.DialConfig(cfg.URL, amqp.Config{
		Heartbeat:       o.heartbeat,
		TLSClientConfig: o.tls,
	})
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: connect: %w", err)
	}

	client := &Client{conn: conn, opts: o}
	if o.healthRegistry != nil {
		o.healthRegistry.RegisterReadiness("rabbitmq", client)
	}
	return client, nil
}

// Producer returns a Producer that publishes to exchange using this
// Client's connection. exchange may be "" to use the broker's default
// exchange, where a message's RoutingKey must equal the target queue
// name. The returned Producer is owned by this Client: Client.Close also
// closes it.
func (c *Client) Producer(exchange string, opts ...ProducerOption) (*Producer, error) {
	po := defaultProducerOptions()
	for _, opt := range opts {
		opt(&po)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: open producer channel: %w", err)
	}

	if po.declareExchangeKind != "" {
		if err := ch.ExchangeDeclare(exchange, po.declareExchangeKind, po.declareDurable, po.declareAutoDelete, false, false, nil); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("rabbitmq: declare exchange=%s: %w", exchange, err)
		}
	}

	p := &Producer{channel: ch, exchange: exchange, logger: c.opts.logger}
	c.children = append(c.children, p)
	return p, nil
}

// Consumer returns a Consumer that reads queue using this Client's
// connection. The returned Consumer is owned by this Client: Client.Close
// also closes it.
func (c *Client) Consumer(queue string, opts ...ConsumerOption) (*Consumer, error) {
	if strings.TrimSpace(queue) == "" {
		return nil, fmt.Errorf("%w: queue is required", ErrInvalidConfig)
	}

	co := defaultConsumerOptions()
	for _, opt := range opts {
		opt(&co)
	}
	if co.concurrency < 1 {
		return nil, fmt.Errorf("%w: concurrency must be >= 1", ErrInvalidConfig)
	}
	if co.prefetch < 1 {
		return nil, fmt.Errorf("%w: prefetch must be >= 1", ErrInvalidConfig)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}

	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq: open consumer channel: %w", err)
	}

	if co.declareQueue {
		if _, err := ch.QueueDeclare(queue, co.queueDurable, co.queueAutoDelete, co.queueExclusive, false, co.queueArgs); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("rabbitmq: declare queue=%s: %w", queue, err)
		}
	}
	if co.bindExchange != "" {
		if err := ch.QueueBind(queue, co.bindRoutingKey, co.bindExchange, false, nil); err != nil {
			_ = ch.Close()
			return nil, fmt.Errorf("rabbitmq: bind queue=%s exchange=%s: %w", queue, co.bindExchange, err)
		}
	}
	if err := ch.Qos(co.prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("rabbitmq: set qos queue=%s: %w", queue, err)
	}

	tag := co.consumerTag
	if tag == "" {
		tag = "consumer-" + newMessageID()
	}

	cons := &Consumer{
		channel:          ch,
		queue:            queue,
		consumerTag:      tag,
		concurrency:      co.concurrency,
		requeueOnFailure: co.requeueOnFailure,
		logger:           c.opts.logger,
	}
	c.children = append(c.children, cons)
	return cons, nil
}

// Close closes every Producer and Consumer spawned from this Client and
// then the underlying connection. It is idempotent and safe for
// concurrent use (messaging.md section 22).
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true

	var errs []error
	for _, child := range c.children {
		if err := child.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if err := c.conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
		errs = append(errs, fmt.Errorf("rabbitmq: close connection: %w", err))
	}
	return errors.Join(errs...)
}

// Check implements health.Check for RabbitMQ readiness.
func (c *Client) Check(ctx context.Context) health.Health {
	c.mu.Lock()
	closed := c.closed
	conn := c.conn
	c.mu.Unlock()

	if closed || conn == nil {
		return health.DownWithDetails(map[string]any{"error": "rabbitmq client is closed"})
	}

	ch, err := conn.Channel()
	if err != nil {
		return health.DownWithDetails(map[string]any{"error": fmt.Sprintf("rabbitmq channel error: %v", err)})
	}
	_ = ch.Close()

	return health.Up()
}
