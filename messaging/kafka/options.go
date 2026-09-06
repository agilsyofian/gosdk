package kafka

import (
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/segmentio/kafka-go/sasl/scram"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// --- Client-level options -------------------------------------------------

// Option configures a Client. Optional configuration uses functional
// options per messaging.md section 6.
type Option func(*clientOptions)

type clientOptions struct {
	appName        string
	logger         logger.Logger
	dialTimeout    time.Duration
	tls            *tls.Config
	sasl           sasl.Mechanism
	healthRegistry health.Registry
	err            error
}

func defaultClientOptions() clientOptions {
	return clientOptions{
		logger:      logger.Nop(),
		dialTimeout: 10 * time.Second,
	}
}

// WithHealthRegistry injects a health.Registry to automatically register
// readiness checks for this Kafka Client.
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

// WithAppName sets the application name stamped on every log line emitted
// by Producers and Consumers spawned from this Client, so messaging records
// are attributable to a service. Defaults to empty (omitted from the log).
func WithAppName(name string) Option {
	return func(o *clientOptions) { o.appName = name }
}

// WithDialTimeout bounds how long broker dials may take. Defaults to 10s.
func WithDialTimeout(d time.Duration) Option {
	return func(o *clientOptions) { o.dialTimeout = d }
}

// WithTLS enables TLS using the supplied configuration. TLS is opt-in and
// explicit, never silently disabled or enabled (messaging.md section 27).
func WithTLS(cfg *tls.Config) Option {
	return func(o *clientOptions) { o.tls = cfg }
}

// WithSASL enables SASL authentication using the supplied mechanism (see
// github.com/segmentio/kafka-go/sasl/plain and .../scram for
// implementations).
func WithSASL(m sasl.Mechanism) Option {
	return func(o *clientOptions) { o.sasl = m }
}

func validateSASLCredentials(username, password string) (bool, error) {
	if username == "" && password == "" {
		return false, nil
	}
	if username == "" || password == "" {
		return false, errors.New("both username and password are required for SASL")
	}
	return true, nil
}

// WithSASLPlain enables SASL PLAIN authentication with the given credentials.
// If both username and password are empty (""), this option is a no-op.
// If only one is provided, Client construction will fail with ErrInvalidConfig.
func WithSASLPlain(username, password string) Option {
	return func(o *clientOptions) {
		valid, err := validateSASLCredentials(username, password)
		if err != nil {
			o.err = err
			return
		}
		if valid {
			o.sasl = plain.Mechanism{
				Username: username,
				Password: password,
			}
		}
	}
}

// WithSASLScramSHA256 enables SASL SCRAM-SHA-256 authentication with the given credentials.
// If both username and password are empty (""), this option is a no-op.
// If only one is provided or mechanism initialization fails, Client construction will fail with ErrInvalidConfig.
func WithSASLScramSHA256(username, password string) Option {
	return func(o *clientOptions) {
		valid, err := validateSASLCredentials(username, password)
		if err != nil {
			o.err = err
			return
		}
		if !valid {
			return
		}
		m, err := scram.Mechanism(scram.SHA256, username, password)
		if err != nil {
			o.err = fmt.Errorf("init scram-sha-256: %w", err)
			return
		}
		o.sasl = m
	}
}

// WithSASLScramSHA512 enables SASL SCRAM-SHA-512 authentication with the given credentials.
// If both username and password are empty (""), this option is a no-op.
// If only one is provided or mechanism initialization fails, Client construction will fail with ErrInvalidConfig.
func WithSASLScramSHA512(username, password string) Option {
	return func(o *clientOptions) {
		valid, err := validateSASLCredentials(username, password)
		if err != nil {
			o.err = err
			return
		}
		if !valid {
			return
		}
		m, err := scram.Mechanism(scram.SHA512, username, password)
		if err != nil {
			o.err = fmt.Errorf("init scram-sha-512: %w", err)
			return
		}
		o.sasl = m
	}
}

// --- Producer options ------------------------------------------------------

// ProducerOption configures a Producer.
type ProducerOption func(*producerOptions)

type producerOptions struct {
	balancer         kafkago.Balancer
	requiredAcks     kafkago.RequiredAcks
	batchTimeout     time.Duration
	writeTimeout     time.Duration
	compression      kafkago.Compression
	enableLogPayload bool
}

func defaultProducerOptions() producerOptions {
	return producerOptions{
		balancer:     &kafkago.LeastBytes{},
		requiredAcks: kafkago.RequireOne,
		batchTimeout: 10 * time.Millisecond,
		writeTimeout: 10 * time.Second,
	}
}

// WithBalancer sets the partition balancer. Defaults to LeastBytes.
func WithBalancer(b kafkago.Balancer) ProducerOption {
	return func(o *producerOptions) { o.balancer = b }
}

// WithRequiredAcks sets the acknowledgement level the broker must reach
// before Publish returns. Defaults to RequireOne. Use RequireAll for
// stronger durability at the cost of latency; this package never chooses
// that trade-off silently.
func WithRequiredAcks(a kafkago.RequiredAcks) ProducerOption {
	return func(o *producerOptions) { o.requiredAcks = a }
}

// WithBatchTimeout bounds how long Publish may wait for the writer to
// fill a batch before sending. Defaults to 10ms.
func WithBatchTimeout(d time.Duration) ProducerOption {
	return func(o *producerOptions) { o.batchTimeout = d }
}

// WithWriteTimeout bounds a single broker write. This is not a retry
// budget — it is a single attempt's deadline (messaging.md section 7:
// avoid hidden retries).
func WithWriteTimeout(d time.Duration) ProducerOption {
	return func(o *producerOptions) { o.writeTimeout = d }
}

// WithCompression enables compression of published batches. Disabled
// (Compression(0), meaning none) by default — messaging.md section 28
// requires compression to be explicit, never automatic.
func WithCompression(c kafkago.Compression) ProducerOption {
	return func(o *producerOptions) { o.compression = c }
}

// WithProducerLogPayload controls whether the message payload body is
// attached to structured log entries. When enabled, sensitive fields (passwords,
// tokens, PINs) are automatically masked by logger.FilterSensitiveData.
// Defaults to false.
func WithProducerLogPayload(enabled bool) ProducerOption {
	return func(o *producerOptions) { o.enableLogPayload = enabled }
}

// --- Consumer options -------------------------------------------------------

// ConsumerOption configures a Consumer.
type ConsumerOption func(*consumerOptions)

type consumerOptions struct {
	concurrency          int
	startOffset          int64
	minBytes             int
	maxBytes             int
	deadLetterTopic      string
	maxConsecutiveErrors int
	backoffInitialDelay  time.Duration
	backoffMaxDelay      time.Duration
	enableLogPayload     bool
}

func defaultConsumerOptions() consumerOptions {
	return consumerOptions{
		concurrency:          1,
		startOffset:          kafkago.FirstOffset,
		minBytes:             1,
		maxBytes:             10e6, // 10MB
		maxConsecutiveErrors: 10,
		backoffInitialDelay:  200 * time.Millisecond,
		backoffMaxDelay:      2 * time.Second,
	}
}

// WithConsumerLogPayload controls whether the consumed message payload body is
// attached to structured log entries. When enabled, sensitive fields are
// automatically masked by logger.FilterSensitiveData. Defaults to false.
func WithConsumerLogPayload(enabled bool) ConsumerOption {
	return func(o *consumerOptions) { o.enableLogPayload = enabled }
}

// WithConcurrency bounds how many messages are handled at once by a
// worker pool (messaging.md section 20-21). Must be >= 1; validated by
// Client.Consumer. Defaults to 1 (strictly sequential, the safest
// default given Kafka's per-partition ordering guarantee). Values above 1
// relax ordering across in-flight messages and require idempotent
// handlers, same as any at-least-once consumer.
func WithConcurrency(n int) ConsumerOption {
	return func(o *consumerOptions) { o.concurrency = n }
}

// WithStartOffset controls where a brand-new consumer group begins
// reading (kafkago.FirstOffset or kafkago.LastOffset). Has no effect once
// the group has committed offsets. Defaults to FirstOffset.
func WithStartOffset(offset int64) ConsumerOption {
	return func(o *consumerOptions) { o.startOffset = offset }
}

// WithFetchBounds sets the reader's min/max fetch size in bytes. Defaults
// to 1 byte / 10MB, matching kafka-go's own conservative defaults.
func WithFetchBounds(minBytes, maxBytes int) ConsumerOption {
	return func(o *consumerOptions) {
		o.minBytes = minBytes
		o.maxBytes = maxBytes
	}
}

// WithDeadLetterTopic configures the topic that messages are forwarded to
// when a Handler returns an error (messaging.md section 16). When unset,
// a failed message is left uncommitted instead, so the broker redelivers
// it on restart/rebalance rather than it being silently dropped.
func WithDeadLetterTopic(topic string) ConsumerOption {
	return func(o *consumerOptions) { o.deadLetterTopic = topic }
}

// WithMaxConsecutiveErrors sets the maximum number of consecutive fetch errors
// allowed before the consumer loop terminates. Defaults to 10. Must be >= 1.
func WithMaxConsecutiveErrors(n int) ConsumerOption {
	return func(o *consumerOptions) {
		if n > 0 {
			o.maxConsecutiveErrors = n
		}
	}
}

// WithFetchBackoff bounds the backoff delay when retrying after a transient
// fetch error. Defaults to 200ms initial and 2s maximum.
func WithFetchBackoff(initial, max time.Duration) ConsumerOption {
	return func(o *consumerOptions) {
		if initial > 0 {
			o.backoffInitialDelay = initial
		}
		if max > 0 {
			o.backoffMaxDelay = max
		}
	}
}
