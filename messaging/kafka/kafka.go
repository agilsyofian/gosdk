package kafka

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/agilsyofian/gosdk/core/observability/health"
)

// Ensure Client implements health.Check interface
var _ health.Check = (*Client)(nil)

// Config is the fully-resolved configuration required to construct a
// Client. Messaging never reads environment variables directly
// (messaging.md section 6) — config loading/resolution belongs to the
// application's config package; Client.New receives already-resolved
// values.
type Config struct {
	// Brokers is the list of Kafka bootstrap broker addresses
	// ("host:port"). Required.
	Brokers []string
}

func (c Config) validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("%w: at least one broker address is required", ErrInvalidConfig)
	}
	for _, b := range c.Brokers {
		if strings.TrimSpace(b) == "" {
			return fmt.Errorf("%w: broker address must not be empty", ErrInvalidConfig)
		}
	}
	return nil
}

// closer is satisfied by both Producer and Consumer, letting Client
// cascade Close across everything it spawned.
type closer interface {
	Close() error
}

// Client holds shared broker connectivity settings (brokers, TLS, SASL,
// dial timeout, logger) and constructs Producer and Consumer instances
// from them. A Client is meant to be constructed once and shared —
// "do not create a new broker connection for every Publish call; prefer
// long-lived producer/consumer instances" (messaging.md section 29).
// Client is safe for concurrent use.
type Client struct {
	cfg  Config
	opts clientOptions

	mu       sync.Mutex
	closed   bool
	children []closer
}

// New validates cfg and returns a Client. Kafka's client model connects
// lazily per Producer/Consumer, so New itself does not dial a broker —
// but it fails fast on invalid configuration rather than deferring the
// error to the first Publish/Consume call (messaging.md section 26).
func New(cfg Config, opts ...Option) (*Client, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	o := defaultClientOptions()
	for _, opt := range opts {
		opt(&o)
	}
	client := &Client{cfg: cfg, opts: o}
	if o.healthRegistry != nil {
		o.healthRegistry.RegisterReadiness("kafka", client)
	}
	return client, nil
}

func (c *Client) transport() *kafkago.Transport {
	return &kafkago.Transport{
		DialTimeout: c.opts.dialTimeout,
		TLS:         c.opts.tls,
		SASL:        c.opts.sasl,
	}
}

func (c *Client) dialer() *kafkago.Dialer {
	return &kafkago.Dialer{
		Timeout:       c.opts.dialTimeout,
		TLS:           c.opts.tls,
		SASLMechanism: c.opts.sasl,
	}
}

// Producer returns a Producer that publishes to topic using this
// Client's shared connection settings. The returned Producer is owned by
// this Client: Client.Close also closes it.
func (c *Client) Producer(topic string, opts ...ProducerOption) (*Producer, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, fmt.Errorf("%w: producer topic is required", ErrInvalidConfig)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}

	po := defaultProducerOptions()
	for _, opt := range opts {
		opt(&po)
	}

	p := c.newProducerLocked(topic, po)
	c.children = append(c.children, p)
	return p, nil
}

func (c *Client) newProducerLocked(topic string, po producerOptions) *Producer {
	w := &kafkago.Writer{
		Addr:         kafkago.TCP(c.cfg.Brokers...),
		Balancer:     po.balancer,
		RequiredAcks: po.requiredAcks,
		BatchTimeout: po.batchTimeout,
		WriteTimeout: po.writeTimeout,
		Compression:  po.compression,
		Transport:    c.transport(),
	}
	return &Producer{writer: w, topic: topic, appName: c.opts.appName, logger: c.opts.logger}
}

// Consumer returns a Consumer that reads topic as part of groupID, using
// this Client's shared connection settings. The returned Consumer is
// owned by this Client: Client.Close also closes it.
func (c *Client) Consumer(topic, groupID string, opts ...ConsumerOption) (*Consumer, error) {
	if strings.TrimSpace(topic) == "" {
		return nil, fmt.Errorf("%w: consumer topic is required", ErrInvalidConfig)
	}
	if strings.TrimSpace(groupID) == "" {
		return nil, fmt.Errorf("%w: consumer group id is required", ErrInvalidConfig)
	}

	co := defaultConsumerOptions()
	for _, opt := range opts {
		opt(&co)
	}
	if co.concurrency < 1 {
		return nil, fmt.Errorf("%w: concurrency must be >= 1", ErrInvalidConfig)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, ErrClosed
	}

	r := kafkago.NewReader(kafkago.ReaderConfig{
		Brokers:     c.cfg.Brokers,
		Topic:       topic,
		GroupID:     groupID,
		Dialer:      c.dialer(),
		MinBytes:    co.minBytes,
		MaxBytes:    co.maxBytes,
		StartOffset: co.startOffset,
	})

	cons := &Consumer{
		reader:      r,
		topic:       topic,
		groupID:     groupID,
		concurrency: co.concurrency,
		appName:     c.opts.appName,
		logger:      c.opts.logger,
	}

	if co.deadLetterTopic != "" {
		cons.dlq = c.newProducerLocked(co.deadLetterTopic, defaultProducerOptions())
	}

	c.children = append(c.children, cons)
	return cons, nil
}

// Close closes every Producer and Consumer spawned from this Client. It
// is idempotent and safe for concurrent use (messaging.md section 22):
// calling it multiple times, including concurrently, never panics and
// only the first call does any work.
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
	return errors.Join(errs...)
}

// Check implements health.Check for Kafka readiness.
func (c *Client) Check(ctx context.Context) health.Health {
	c.mu.Lock()
	closed := c.closed
	c.mu.Unlock()

	if closed {
		return health.DownWithDetails(map[string]any{"error": "kafka client is closed"})
	}

	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	var errs []string
	reachable := 0

	dialer := c.dialer()

	for _, broker := range c.cfg.Brokers {
		conn, err := dialer.DialContext(checkCtx, "tcp", broker)
		if err == nil {
			_ = conn.Close()
			reachable++
			// At least one broker in the cluster is reachable -> cluster is considered UP
			break
		}
		errs = append(errs, fmt.Sprintf("%s: %v", broker, err))
	}

	if reachable > 0 {
		return health.Up()
	}

	return health.DownWithDetails(map[string]any{
		"error":   "no reachable kafka brokers",
		"details": errs,
	})
}
