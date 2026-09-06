package kafka

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	kafkago "github.com/segmentio/kafka-go"
)

// kafkaReader is the subset of *kafkago.Reader this package depends on —
// the consumer-side counterpart to kafkaWriter, and likewise a test seam
// only (messaging.md section 32).
type kafkaReader interface {
	FetchMessage(ctx context.Context) (kafkago.Message, error)
	CommitMessages(ctx context.Context, msgs ...kafkago.Message) error
	Close() error
}

// Handler processes a single delivered message. A non-nil return means
// the message was not successfully processed; see Consume's doc comment
// for exactly what happens next.
type Handler func(ctx context.Context, msg *Message) error

// Consumer reads messages from a single Kafka topic within a consumer
// group. It is created via Client.Consumer.
type Consumer struct {
	reader      kafkaReader
	topic       string
	groupID     string
	concurrency int
	appName     string
	logger      logger.Logger
	dlq         *Producer // set only when WithDeadLetterTopic was used

	closed atomic.Bool
}

// logPayload builds this Consumer's log skeleton: the path identifies the
// topic/group pair the record belongs to.
func (c *Consumer) logPayload(ctx context.Context, method string) logger.LogPayload {
	return newLogPayload(ctx, c.appName, method, fmt.Sprintf("/kafka/%s/%s", c.topic, c.groupID))
}

// Consume fetches messages and calls handler for each one until ctx is
// done or the reader returns an unrecoverable error. Consume blocks;
// callers typically run it in its own goroutine and cancel ctx to stop
// it gracefully.
//
// Delivery and acknowledgement semantics (messaging.md sections 17-18):
// this is an at-least-once consumer scoped to Kafka's per-partition
// ordering. The offset for a message is committed only after handler
// returns nil, or after the message has been safely forwarded to the
// dead-letter topic following a handler error. A message is never
// committed before its outcome is established (section 16). If handler
// fails and no dead-letter topic is configured (WithDeadLetterTopic),
// the offset is left uncommitted, so the broker redelivers that message
// on the next restart or rebalance — handlers must be idempotent, and a
// message that always fails will keep blocking that partition's progress
// across restarts until either the handler is fixed or a dead-letter
// topic is configured. This package does not implement its own retry
// loop around Handler (retry belongs to resilience/retry, section 15).
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	if c.closed.Load() {
		return ErrClosed
	}
	if handler == nil {
		return fmt.Errorf("%w: handler must not be nil", ErrInvalidConfig)
	}

	c.logger.Info(ctx, fmt.Sprintf("kafka: consumer started. concurrency: %d", c.concurrency),
		c.logPayload(ctx, "START").WithMetadata("concurrency", strconv.Itoa(c.concurrency)))

	// The payload is deliberately built at deferred time, not at registration
	// time, so its Timestamp records when the consumer actually stopped.
	defer func() {
		c.logger.Info(ctx, "kafka: consumer stopped", c.logPayload(ctx, "SHUTDOWN"))
	}()

	sem := make(chan struct{}, c.concurrency)
	var wg sync.WaitGroup

	for {
		km, err := c.reader.FetchMessage(ctx)
		if err != nil {
			// Stop accepting new work and let in-flight handlers finish
			// before returning (messaging.md section 22).
			wg.Wait()
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return err
			}
			if c.closed.Load() {
				return ErrClosed
			}
			return fmt.Errorf("kafka: fetch topic=%s: %w", c.topic, err)
		}

		msg := fromKafkaMessage(km)

		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c.process(ctx, km, msg, handler)
		}()
	}
}

func (c *Consumer) process(ctx context.Context, km kafkago.Message, msg *Message, handler Handler) {
	traceID := ""
	if msg.Headers != nil {
		traceID = msg.Headers["x-trace-id"]
		if traceID == "" {
			traceID = msg.Headers["traceparent"]
		}
	}
	if traceID == "" {
		traceID = newMessageID()
	}

	sessionID := ""
	if msg.Headers != nil {
		sessionID = msg.Headers["x-session-id"]
	}

	msgCtx := tracing.WithAppName(ctx, c.appName)
	msgCtx = tracing.WithTraceID(msgCtx, traceID)
	if sessionID != "" {
		msgCtx = tracing.WithSessionID(msgCtx, sessionID)
	}

	start := time.Now()
	err := handler(msgCtx, msg)
	dur := time.Since(start)

	// Finalization (commit / dead-letter publish) must be able to finish
	// even when ctx was cancelled to signal shutdown while handler was
	// still running above — otherwise "finish in-flight work" at
	// shutdown (messaging.md section 22) would race against the very
	// commit that in-flight work is supposed to end with. finalizeCtx
	// keeps any values ctx carries (trace/correlation metadata) but is
	// bounded by its own timeout instead of the caller's cancellation.
	finalizeCtx, cancel := context.WithTimeout(context.WithoutCancel(msgCtx), 10*time.Second)
	defer cancel()

	// One skeleton per delivered message; WithError returns a copy, so an
	// error attached to one log line never leaks into the next.
	base := c.logPayload(msgCtx, "CONSUME").
		WithMetadata("topic", msg.Topic).
		WithMetadata("key", string(msg.Key)).
		WithMetadata("partition", strconv.Itoa(msg.Partition)).
		WithMetadata("offset", strconv.FormatInt(msg.Offset, 10)).
		WithMetadata("message_id", msg.ID)
	base.Duration = dur.Milliseconds()

	if err == nil {
		if cerr := c.reader.CommitMessages(finalizeCtx, km); cerr != nil {
			c.logger.Error(msgCtx, "kafka: commit failed", base.WithError(cerr.Error()))
		}
		c.logger.Info(msgCtx, "kafka: handled", base)
		return
	}

	c.logger.Warn(msgCtx, "kafka: handler failed", base.WithError(err.Error()))

	if c.dlq == nil {
		// No dead-letter topic configured: leave the offset uncommitted
		// so the broker redelivers this message rather than it being
		// silently dropped (messaging.md section 16).
		return
	}

	dlqMsg := &Message{
		Payload:     msg.Payload,
		ContentType: msg.ContentType,
		Headers:     cloneHeaders(msg.Headers),
	}
	if dlqMsg.Headers == nil {
		dlqMsg.Headers = make(map[string]string, 5)
	}
	dlqMsg.Headers["x-original-topic"] = msg.Topic
	dlqMsg.Headers["x-original-partition"] = strconv.Itoa(msg.Partition)
	dlqMsg.Headers["x-original-offset"] = strconv.FormatInt(msg.Offset, 10)
	dlqMsg.Headers["x-error"] = err.Error()
	dlqMsg.Headers["x-failed-at"] = time.Now().UTC().Format(time.RFC3339)

	if derr := c.dlq.Publish(finalizeCtx, dlqMsg); derr != nil {
		c.logger.Error(ctx, "kafka: dead-letter publish failed", base.WithError(derr.Error()))
		// Do not commit: the intended delivery outcome (preservation in
		// the dead-letter topic) was not established (section 16).
		return
	}

	// The failed message is now preserved in the dead-letter topic, so
	// it is safe to advance past it on the source partition.
	if cerr := c.reader.CommitMessages(finalizeCtx, km); cerr != nil {
		c.logger.Error(ctx, "kafka: commit after dead-letter failed", base.WithError(cerr.Error()))
	}
}

// Close closes the underlying reader (and the dead-letter producer, if
// one was configured). It is idempotent and safe for concurrent use
// (messaging.md section 22).
func (c *Consumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	var errs []error
	if err := c.reader.Close(); err != nil {
		errs = append(errs, fmt.Errorf("kafka: close consumer topic=%s: %w", c.topic, err))
	}
	if c.dlq != nil {
		if err := c.dlq.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func fromKafkaMessage(km kafkago.Message) *Message {
	headers := make(map[string]string, len(km.Headers))
	for _, h := range km.Headers {
		headers[h.Key] = string(h.Value)
	}
	id := headers["x-message-id"]
	contentType := headers["content-type"]
	delete(headers, "x-message-id")
	delete(headers, "content-type")

	return &Message{
		ID:          id,
		Key:         km.Key,
		Payload:     km.Value,
		Headers:     headers,
		ContentType: contentType,
		Topic:       km.Topic,
		Partition:   km.Partition,
		Offset:      km.Offset,
		Time:        km.Time,
	}
}

func cloneHeaders(h map[string]string) map[string]string {
	if h == nil {
		return nil
	}
	out := make(map[string]string, len(h))
	for k, v := range h {
		out[k] = v
	}
	return out
}
