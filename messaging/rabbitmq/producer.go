package rabbitmq

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// amqpPublisher is the subset of *amqp.Channel this package's Producer
// depends on — the mock transport seam for tests (messaging.md section
// 32). Kept unexported so callers can never substitute their own broker
// client through the public API (section 30, section 38 — no leaking
// vendor clients).
type amqpPublisher interface {
	PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	Close() error
}

// Producer publishes messages to a single exchange (the empty string
// names the broker's default exchange, where the routing key must equal
// the target queue name). Producer is created via Client.Producer and is
// safe for concurrent use by multiple goroutines.
type Producer struct {
	channel  amqpPublisher
	exchange string
	appName  string
	logger   logger.Logger

	closed atomic.Bool
}

// Publish sends msg to the producer's exchange using msg.RoutingKey and
// blocks until the broker accepts it onto the channel, or ctx is done.
// mandatory and immediate are always false: this version does not wire
// NotifyReturn to observe unroutable messages, and RabbitMQ no longer
// honors the immediate flag at all on modern brokers (messaging.md
// section 31 notwithstanding — this is a deliberately small v1 surface,
// see doc.go).
func (p *Producer) Publish(ctx context.Context, msg *Message) error {
	if p.closed.Load() {
		return ErrClosed
	}
	if msg == nil || len(msg.Payload) == 0 {
		return fmt.Errorf("%w: payload must not be empty", ErrInvalidMessage)
	}

	if msg.ID == "" {
		msg.ID = newMessageID()
	}

	headers := make(amqp.Table, len(msg.Headers))
	for k, v := range msg.Headers {
		headers[k] = v
	}

	start := time.Now()
	pub := amqp.Publishing{
		Headers:       headers,
		ContentType:   msg.ContentType,
		CorrelationId: msg.CorrelationID,
		MessageId:     msg.ID,
		Body:          msg.Payload,
	}

	payload := newLogPayload(ctx, p.appName, "PUBLISH", "/rabbitmq/"+p.exchange).
		WithMetadata("exchange", p.exchange).
		WithMetadata("routing_key", msg.RoutingKey).
		WithMetadata("message_id", msg.ID)
	payload.Duration = time.Since(start).Milliseconds()

	err := p.channel.PublishWithContext(ctx, p.exchange, msg.RoutingKey, false, false, pub)
	if err != nil {
		p.logger.Error(ctx, "rabbitmq: publish failed", payload.WithError(err.Error()))
		return fmt.Errorf("rabbitmq: publish exchange=%s routing_key=%s: %w", p.exchange, msg.RoutingKey, err)
	}

	p.logger.Debug(ctx, "rabbitmq: publish", payload)
	return nil
}

// Close closes the underlying channel. It is idempotent and safe for
// concurrent use — calling Close multiple times never panics (messaging.md
// section 22).
func (p *Producer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := p.channel.Close(); err != nil {
		return fmt.Errorf("rabbitmq: close producer channel exchange=%s: %w", p.exchange, err)
	}
	return nil
}
