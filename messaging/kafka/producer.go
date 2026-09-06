package kafka

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
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
	writer           kafkaWriter
	topic            string
	appName          string
	logger           logger.Logger
	enableLogPayload bool

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

	if msg.Headers == nil {
		msg.Headers = make(map[string]string)
	}

	tracer := otel.Tracer("kafka-producer")
	spanName := fmt.Sprintf("%s publish", topic)
	spanAttrs := []attribute.KeyValue{
		attribute.String("messaging.system", "kafka"),
		attribute.String("messaging.destination.name", topic),
		attribute.String("messaging.operation", "publish"),
		attribute.String("messaging.message.id", msg.ID),
	}
	if len(msg.Key) > 0 {
		spanAttrs = append(spanAttrs, attribute.String("messaging.kafka.message.key", string(msg.Key)))
	}

	publishCtx, span := tracer.Start(ctx, spanName,
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(spanAttrs...),
	)
	defer span.End()

	otel.GetTextMapPropagator().Inject(publishCtx, propagation.MapCarrier(msg.Headers))

	traceID := p.resolveTraceId(msg, publishCtx)
	msg.Headers["x-trace-id"] = traceID

	if msg.Headers["x-session-id"] == "" {
		if sess := tracing.SessionID(publishCtx); sess != "" {
			msg.Headers["x-session-id"] = sess
		}
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
	err := p.writer.WriteMessages(publishCtx, kafkago.Message{
		Topic:   topic,
		Key:     msg.Key,
		Value:   msg.Payload,
		Headers: headers,
	})

	payload := newLogPayload(publishCtx, p.appName, "PUBLISH", "/kafka/"+topic).
		WithMetadata("topic", topic).
		WithMetadata("key", string(msg.Key)).
		WithMetadata("message_id", msg.ID)
	payload.Duration = time.Since(start).Milliseconds()
	if p.enableLogPayload {
		payload.Request = msg.Payload
	}

	if err != nil {
		span.RecordError(err)
		p.logger.Error(publishCtx, "kafka: publish failed", payload.WithError(err.Error()))
		return fmt.Errorf("kafka: publish topic=%s: %w", topic, err)
	}

	p.logger.Info(publishCtx, "kafka: publish", payload)
	return nil
}

func (p *Producer) resolveTraceId(msg *Message, publishCtx context.Context) string {
	traceID := msg.Headers["x-trace-id"]
	if traceID == "" {
		traceID = msg.Headers["traceparent"]
	}
	if traceID == "" {
		traceID = tracing.TraceID(publishCtx)
	}
	if traceID == "" {
		traceID = newMessageID()
	}
	return traceID
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
