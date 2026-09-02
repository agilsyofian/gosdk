package kafka

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/agilsyofian/gosdk/observability/logger"
)

// kafkaWriter is the subset of *kafkago.Writer this package depends on.
// It exists as a test seam only (messaging.md section 32: "mock transport
// seam") — production code always gets a real *kafkago.Writer via
// Client.Producer. It stays unexported so callers can never substitute
// their own broker client through the public API (messaging.md section
// 30 — no RawKafkaClient-style escape hatch).
type kafkaWriter interface {
	WriteMessages(ctx context.Context, msgs ...kafkago.Message) error
	Close() error
}

// Producer publishes messages to a single Kafka topic. It is created via
// Client.Producer and is safe for concurrent use by multiple goroutines
// (messaging.md section 5, section 20).
type Producer struct {
	writer  kafkaWriter
	topic   string
	appName string
	logger  logger.Logger

	closed atomic.Bool
}

// Publish sends msg to the producer's topic (or msg.Topic, if set) and
// blocks until the broker acknowledges it according to the configured
// RequiredAcks, or ctx is done. Publish performs no retries beyond what
// the broker/protocol does internally for a single attempt, and applies
// no hidden timeout beyond ctx and WithWriteTimeout (messaging.md section
// 7) — repeated attempts belong to resilience/retry, not this package.
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
	topic := msg.Topic
	if topic == "" {
		topic = p.topic
	}

	headers := make([]kafkago.Header, 0, len(msg.Headers)+2)
	for k, v := range msg.Headers {
		headers = append(headers, kafkago.Header{Key: k, Value: []byte(v)})
	}
	headers = append(headers, kafkago.Header{Key: "x-message-id", Value: []byte(msg.ID)})
	if msg.ContentType != "" {
		headers = append(headers, kafkago.Header{Key: "content-type", Value: []byte(msg.ContentType)})
	}

	start := time.Now()
	err := p.writer.WriteMessages(ctx, kafkago.Message{
		Topic:   topic,
		Key:     msg.Key,
		Value:   msg.Payload,
		Headers: headers,
	})

	payload := newLogPayload(ctx, p.appName, "PUBLISH", "/kafka/"+topic).
		WithMetadata("topic", topic).
		WithMetadata("key", string(msg.Key)).
		WithMetadata("message_id", msg.ID)
	payload.Duration = time.Since(start).Milliseconds()

	if err != nil {
		p.logger.Error(ctx, "kafka: publish failed", payload.WithError(err.Error()))
		return fmt.Errorf("kafka: publish topic=%s: %w", topic, err)
	}

	p.logger.Info(ctx, "kafka: publish", payload)
	return nil
}

// Close closes the underlying writer, flushing any buffered messages
// first. It is idempotent and safe for concurrent use — calling Close
// multiple times never panics (messaging.md section 22).
func (p *Producer) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("kafka: close producer topic=%s: %w", p.topic, err)
	}
	return nil
}
