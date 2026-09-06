package rabbitmq

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// amqpConsumerChannel is the subset of *amqp.Channel this package's
// Consumer depends on — the consumer-side counterpart to amqpPublisher,
// and likewise a test seam only (messaging.md section 32).
type amqpConsumerChannel interface {
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Cancel(consumer string, noWait bool) error
	Close() error
}

// Handler processes a single delivered message. A non-nil return means
// the message was not successfully processed; see Consume's doc comment
// for exactly what happens next.
type Handler func(ctx context.Context, msg *Message) error

// Consumer reads messages from a single RabbitMQ queue. It is created via
// Client.Consumer.
type Consumer struct {
	channel          amqpConsumerChannel
	queue            string
	consumerTag      string
	concurrency      int
	appName          string
	requeueOnFailure bool
	logger           logger.Logger

	closed atomic.Bool
}

// logPayload builds this Consumer's log skeleton: the path identifies the
// topic/group pair the record belongs to.
func (c *Consumer) logPayload(ctx context.Context, method string) logger.LogPayload {
	return newLogPayload(ctx, c.appName, method, fmt.Sprintf("/rabbitmq/%s/%s", c.queue, c.consumerTag))
}

// Consume registers a consumer on the Consumer's queue and calls handler
// for each delivery until ctx is done or the delivery stream ends.
// Consume blocks; callers typically run it in its own goroutine and
// cancel ctx to stop it gracefully.
//
// Acknowledgement policy (messaging.md section 17-18): manual
// acknowledgement, at-least-once. A delivery is Acked once handler
// returns nil. If handler returns an error, the delivery is Nacked with
// requeue controlled by WithRequeueOnFailure (default false, to avoid a
// tight redelivery loop against a Handler that always fails the same
// message).
//
// Dead-lettering (section 16) is delegated to RabbitMQ's native
// dead-letter-exchange mechanism rather than reimplemented client-side
// the way messaging/kafka does: configure the queue with an
// "x-dead-letter-exchange" argument via WithQueueDeclare, and a
// Nack-without-requeue is routed there by the broker automatically. This
// package does not force Kafka's republish-based DLQ model onto
// RabbitMQ, since the broker already solves it natively here.
func (c *Consumer) Consume(ctx context.Context, handler Handler) error {
	if c.closed.Load() {
		return ErrClosed
	}
	if handler == nil {
		return fmt.Errorf("%w: handler must not be nil", ErrInvalidConfig)
	}

	deliveries, err := c.channel.Consume(c.queue, c.consumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("rabbitmq: consume queue=%s: %w", c.queue, err)
	}

	c.logger.Info(ctx, fmt.Sprintf("rabbitmq: consumer started. concurrency: %d", c.concurrency),
		c.logPayload(ctx, "START").WithMetadata("concurrency", strconv.Itoa(c.concurrency)))

	// The payload is deliberately built at deferred time, not at registration
	// time, so its Timestamp records when the consumer actually stopped.
	defer func() {
		c.logger.Info(ctx, "kafka: consumer stopped", c.logPayload(ctx, "SHUTDOWN"))
	}()

	// Ask the broker to stop delivering to this consumer once ctx is
	// done, so the range below drains naturally instead of being torn
	// down mid-delivery (messaging.md section 22: stop accepting new
	// work, then finish in-flight work).
	stopped := make(chan struct{})
	defer close(stopped)
	go func() {
		select {
		case <-ctx.Done():
			_ = c.channel.Cancel(c.consumerTag, false)
		case <-stopped:
		}
	}()

	var wg sync.WaitGroup
	sem := make(chan struct{}, c.concurrency)

	for d := range deliveries {
		d := d
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			c.process(ctx, d, handler)
		}()
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrConsumerStopped
}

func (c *Consumer) process(ctx context.Context, d amqp.Delivery, handler Handler) {
	msg := fromDelivery(d)

	start := time.Now()
	err := handler(ctx, msg)
	dur := time.Since(start)

	// One skeleton per delivered message; WithError returns a copy, so an
	// error attached to one log line never leaks into the next.
	base := c.logPayload(ctx, "CONSUME").
		WithMetadata("queue", c.queue).
		WithMetadata("delivery_tag", fmt.Sprintf("%d", d.DeliveryTag)).
		WithMetadata("message_id", msg.ID)
	base.Duration = dur.Milliseconds()

	if err == nil {
		if aerr := d.Ack(false); aerr != nil {
			c.logger.Error(ctx, "rabbitmq: ack failed", base.WithError(aerr.Error()))
		}
		c.logger.Debug(ctx, "rabbitmq: handled", base)
		return
	}

	c.logger.Warn(ctx, "rabbitmq: handler failed", base.WithError(err.Error()))

	if nerr := d.Nack(false, c.requeueOnFailure); nerr != nil {
		c.logger.Error(ctx, "rabbitmq: nack failed", base.WithError(nerr.Error()))
	}
}

// Close closes the underlying channel. It is idempotent and safe for
// concurrent use (messaging.md section 22). Closing the channel directly
// like this is a hard stop for any Consume call currently running against
// it; cancelling Consume's context is the graceful alternative.
func (c *Consumer) Close() error {
	if !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	if err := c.channel.Close(); err != nil {
		return fmt.Errorf("rabbitmq: close consumer channel queue=%s: %w", c.queue, err)
	}
	return nil
}

func fromDelivery(d amqp.Delivery) *Message {
	headers := make(map[string]string, len(d.Headers))
	for k, v := range d.Headers {
		if s, ok := v.(string); ok {
			headers[k] = s
		} else {
			headers[k] = fmt.Sprintf("%v", v)
		}
	}
	return &Message{
		ID:            d.MessageId,
		RoutingKey:    d.RoutingKey,
		Payload:       d.Body,
		Headers:       headers,
		ContentType:   d.ContentType,
		CorrelationID: d.CorrelationId,
		Exchange:      d.Exchange,
		DeliveryTag:   d.DeliveryTag,
		Redelivered:   d.Redelivered,
	}
}
