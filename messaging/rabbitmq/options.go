package rabbitmq

import (
	"crypto/tls"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/observability/logger"
)

// --- Client-level options ---------------------------------------------------

// Option configures a Client.
type Option func(*clientOptions)

type clientOptions struct {
	logger         logger.Logger
	tls            *tls.Config
	heartbeat      time.Duration
	healthRegistry health.Registry
}

func defaultClientOptions() clientOptions {
	return clientOptions{
		logger:    logger.Nop(),
		heartbeat: 10 * time.Second,
	}
}

// WithHealthRegistry injects a health.Registry to automatically register
// readiness checks for this RabbitMQ Client.
func WithHealthRegistry(hr health.Registry) Option {
	return func(o *clientOptions) { o.healthRegistry = hr }
}

// WithLogger injects the Logger used for every Producer/Consumer spawned
// from this Client. Defaults to a no-op logger.
func WithLogger(l logger.Logger) Option {
	return func(o *clientOptions) {
		if l != nil {
			o.logger = l
		}
	}
}

// WithTLS sets the TLS configuration used when the connection URL scheme
// is "amqps". It has no effect against a plain "amqp" URL — the scheme,
// not this option, is what decides whether TLS is used at all, so TLS
// stays explicit rather than silently toggled (messaging.md section 27).
func WithTLS(cfg *tls.Config) Option {
	return func(o *clientOptions) { o.tls = cfg }
}

// WithHeartbeat sets the AMQP heartbeat interval. Defaults to 10s,
// matching amqp091-go's own default.
func WithHeartbeat(d time.Duration) Option {
	return func(o *clientOptions) { o.heartbeat = d }
}

// --- Producer options --------------------------------------------------------

// ProducerOption configures a Producer.
type ProducerOption func(*producerOptions)

type producerOptions struct {
	declareExchangeKind string // empty means: assume the exchange already exists
	declareDurable      bool
	declareAutoDelete   bool
}

func defaultProducerOptions() producerOptions {
	return producerOptions{}
}

// WithExchangeDeclare declares the producer's exchange at construction
// time (kind is one of "direct", "fanout", "topic", "headers"). Off by
// default: this package assumes production topology is managed by
// infrastructure tooling, not application code, and only offers this as
// a convenience for local development, tests, and the runnable examples.
func WithExchangeDeclare(kind string, durable, autoDelete bool) ProducerOption {
	return func(o *producerOptions) {
		o.declareExchangeKind = kind
		o.declareDurable = durable
		o.declareAutoDelete = autoDelete
	}
}

// --- Consumer options ---------------------------------------------------------

// ConsumerOption configures a Consumer.
type ConsumerOption func(*consumerOptions)

type consumerOptions struct {
	concurrency      int
	prefetch         int
	requeueOnFailure bool
	consumerTag      string

	declareQueue    bool
	queueDurable    bool
	queueAutoDelete bool
	queueExclusive  bool
	queueArgs       amqp.Table

	bindExchange   string
	bindRoutingKey string
}

func defaultConsumerOptions() consumerOptions {
	return consumerOptions{
		concurrency: 1,
		prefetch:    10,
	}
}

// WithConcurrency bounds how many deliveries are handled at once by a
// worker pool (messaging.md section 20-21). Must be >= 1; validated by
// Client.Consumer. Defaults to 1.
func WithConcurrency(n int) ConsumerOption {
	return func(o *consumerOptions) { o.concurrency = n }
}

// WithPrefetch sets the channel's QoS prefetch count — RabbitMQ's native
// backpressure mechanism (messaging.md section 21). Must be >= 1;
// validated by Client.Consumer. Defaults to 10.
func WithPrefetch(n int) ConsumerOption {
	return func(o *consumerOptions) { o.prefetch = n }
}

// WithRequeueOnFailure controls whether a message whose Handler returned
// an error is requeued (Nack with requeue=true) or not (requeue=false).
// Defaults to false: without a dead-letter-exchange configured on the
// queue (see WithQueueDeclare), requeuing a message a Handler will keep
// failing on creates a tight redelivery loop, so the safer default drops
// it from this queue (to the queue's DLX if one is configured, or
// discarded by the broker if not) instead of looping forever
// (messaging.md section 8: avoid infinite retry loops).
func WithRequeueOnFailure(requeue bool) ConsumerOption {
	return func(o *consumerOptions) { o.requeueOnFailure = requeue }
}

// WithConsumerTag sets an explicit AMQP consumer tag. When unset, a
// random tag is generated.
func WithConsumerTag(tag string) ConsumerOption {
	return func(o *consumerOptions) { o.consumerTag = tag }
}

// WithQueueDeclare declares the consumer's queue at construction time.
// Off by default, for the same reason as WithExchangeDeclare. Pass
// amqp.Table{"x-dead-letter-exchange": "orders.dlx"} (and related
// x-dead-letter-* arguments) in args to wire up RabbitMQ's native
// dead-letter routing for messages Nacked without requeue — that broker
// mechanism is how this package achieves dead-lettering for RabbitMQ
// (messaging.md section 16), rather than a client-side republish like
// messaging/kafka uses.
func WithQueueDeclare(durable, autoDelete, exclusive bool, args amqp.Table) ConsumerOption {
	return func(o *consumerOptions) {
		o.declareQueue = true
		o.queueDurable = durable
		o.queueAutoDelete = autoDelete
		o.queueExclusive = exclusive
		o.queueArgs = args
	}
}

// WithBinding binds the consumer's queue to exchange using routingKey.
// Typically combined with WithQueueDeclare. Off by default.
func WithBinding(exchange, routingKey string) ConsumerOption {
	return func(o *consumerOptions) {
		o.bindExchange = exchange
		o.bindRoutingKey = routingKey
	}
}
