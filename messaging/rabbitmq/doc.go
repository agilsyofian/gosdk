// Package rabbitmq provides a production-oriented RabbitMQ Producer and
// Consumer built on github.com/rabbitmq/amqp091-go, following the rules
// in messaging.md. Like messaging/kafka, it is a Layer 2 Communication
// module scoped to producer/consumer lifecycle, the message envelope,
// and broker connection lifecycle — never business logic.
//
// # Getting started
//
//	client, err := rabbitmq.New(rabbitmq.Config{
//		URL: "amqp://guest:guest@localhost:5672/",
//	})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	producer, err := client.Producer("orders") // exchange name
//	if err != nil {
//		return err
//	}
//	err = producer.Publish(ctx, &rabbitmq.Message{
//		RoutingKey: "orders.created",
//		Payload:    []byte(`{"id":1}`),
//	})
//
//	consumer, err := client.Consumer("orders.created") // queue name
//	if err != nil {
//		return err
//	}
//	err = consumer.Consume(ctx, func(ctx context.Context, msg *rabbitmq.Message) error {
//		return nil // ack
//	})
//
// # Delivery semantics
//
// At-least-once. A redelivered message is marked Redelivered. Handlers
// must be idempotent. This package never claims exactly-once delivery
// (messaging.md section 18).
//
// # Acknowledgement semantics
//
// Manual acknowledgement. Consumer Acks a delivery once its Handler
// returns nil, and Nacks it (without multiple) when Handler returns an
// error. Whether a Nack requeues is controlled by WithRequeueOnFailure
// (default false).
//
// # Ordering
//
// Ordering in RabbitMQ depends on queue, consumer count, acknowledgement,
// prefetch, and redelivery behavior — this package makes no ordering
// promise beyond "single consumer, WithConcurrency(1), no redelivery"
// (messaging.md section 19). Raising WithConcurrency or WithPrefetch, or
// running multiple consumer instances on the same queue, trades ordering
// for throughput, same as it would with any AMQP client.
//
// # Retry and dead-letter
//
// This package does not implement a generic retry policy — repeated
// handler attempts belong to resilience/retry. Dead-lettering is
// delegated to RabbitMQ's native dead-letter-exchange mechanism: declare
// the consumer's queue with an "x-dead-letter-exchange" argument (see
// WithQueueDeclare) and a Nack-without-requeue is routed there by the
// broker automatically. This deliberately differs from messaging/kafka,
// which republishes to a dead-letter topic itself — Kafka has no
// broker-native equivalent, RabbitMQ does, and messaging.md section 16
// is explicit that one broker's DLQ model must not be forced onto the
// other.
//
// # Concurrency and backpressure
//
// Producer and Consumer are safe for concurrent use. Consumer bounds
// in-flight handler goroutines with a semaphore sized by WithConcurrency
// (default 1), and uses the channel's QoS prefetch count (WithPrefetch,
// default 10) as RabbitMQ-native backpressure — the broker will not push
// more unacknowledged deliveries than that.
//
// # Shutdown
//
// Cancel the context passed to Consume to stop it gracefully: it asks
// the broker to stop delivering to that consumer tag, drains in-flight
// handlers, and returns ctx.Err(). Close on Client, Producer, or Consumer
// is a harder, immediate stop — it closes the channel directly — and is
// idempotent and safe for concurrent use.
//
// # Configuration
//
// Config carries only the connection URL; TLS, heartbeat, and
// per-producer/consumer tuning are functional options with documented
// defaults. This package never reads environment variables itself.
// Unlike messaging/kafka.New, New here dials the broker immediately,
// matching RabbitMQ's connection-oriented protocol (messaging.md section
// 4).
//
// # Security
//
// TLS is used only when the connection URL uses the "amqps" scheme;
// WithTLS supplies the *tls.Config for that case but does not by itself
// turn TLS on. Credentials embedded in the connection URL are never
// logged; only operational metadata (exchange, routing key, queue,
// delivery tag, message ID, duration) is logged, never raw payloads.
//
// # Known limitations
//
// This version does not implement automatic reconnection after the
// broker connection drops (messaging.md section 23 leaves this as an
// explicit extension point rather than shipping a half-specified backoff
// policy) and does not wire NotifyReturn for unroutable mandatory
// publishes. Both are natural additions if a concrete need for them
// shows up.
package rabbitmq
