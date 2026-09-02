// Package kafka provides a production-oriented Kafka Producer and
// Consumer built on github.com/segmentio/kafka-go, following the rules in
// messaging.md. It is a Layer 2 Communication module: it owns producer
// and consumer lifecycle, the message envelope, and broker connection
// lifecycle, and stays out of business logic, domain event definitions,
// and application bootstrap.
//
// # Getting started
//
//	client, err := kafka.New(kafka.Config{
//		Brokers: []string{"localhost:9092"},
//	})
//	if err != nil {
//		return err
//	}
//	defer client.Close()
//
//	producer, err := client.Producer("orders.created")
//	if err != nil {
//		return err
//	}
//	err = producer.Publish(ctx, &kafka.Message{Payload: []byte(`{"id":1}`)})
//
//	consumer, err := client.Consumer("orders.created", "billing-service")
//	if err != nil {
//		return err
//	}
//	err = consumer.Consume(ctx, func(ctx context.Context, msg *kafka.Message) error {
//		return nil // ack: commits the offset
//	})
//
// # Delivery semantics
//
// At-least-once. A message may be delivered more than once after a
// consumer restart, rebalance, or a commit that failed after a handler
// otherwise succeeded. Handlers must be idempotent. This package never
// claims exactly-once delivery, because that is an end-to-end property
// of the whole pipeline, not something a single client library can
// guarantee on its own (messaging.md section 18).
//
// # Acknowledgement semantics
//
// Acknowledgement is offset commit. Consumer commits the offset for a
// message only after its Handler returns nil, or after the message has
// been safely forwarded to a configured dead-letter topic following a
// Handler error. See Consumer.Consume's doc comment for the full policy.
//
// # Ordering
//
// Ordering is scoped to a partition, matching Kafka's own guarantee —
// not global across a topic. Messages published with the same Key land
// on the same partition and are read back in that order by a single
// consumer instance. Setting WithConcurrency above 1 lets multiple
// in-flight handlers race within the shared fetch loop, which relaxes
// completion order (though not the order messages are *read* from a
// partition) and, as with any at-least-once consumer, requires
// idempotent handlers.
//
// # Retry and dead-letter
//
// This package does not implement a generic retry policy — repeated
// handler attempts belong to resilience/retry, composed by the caller
// around Handler. What this package does own is the dead-letter
// integration point: configure it with WithDeadLetterTopic, and a
// message whose Handler fails is republished to that topic (with
// x-original-topic/partition/offset, x-error, and x-failed-at headers
// describing the failure) before its original offset is committed. Kafka
// has no broker-native redelivery/DLQ primitive the way RabbitMQ does, so
// this republish is how dead-lettering is achieved here (messaging.md
// section 16 — do not force one broker's DLQ model onto the other).
//
// # Concurrency and backpressure
//
// Producer and Consumer are safe for concurrent use. Consumer bounds
// in-flight handler goroutines with a semaphore sized by WithConcurrency
// (default 1); it never spawns an unbounded goroutine per message.
//
// # Shutdown
//
// Cancel the context passed to Consume to stop it: it stops fetching new
// messages, waits for in-flight handlers to finish, and returns. Close on
// Client, Producer, or Consumer is idempotent and safe for concurrent
// use.
//
// # Configuration
//
// Config carries only what every deployment needs (broker addresses);
// TLS, SASL, dial timeout, and per-producer/consumer tuning are
// functional options with documented defaults. This package never reads
// environment variables itself.
//
// # Security
//
// TLS and SASL are opt-in via WithTLS/WithSASL and never silently
// disabled. Credentials are never logged; only operational metadata
// (topic, partition, offset, message ID, duration) is logged, never raw
// payloads.
//
// # Operational considerations
//
// Every Producer/Consumer is meant to be long-lived and shared, not
// recreated per call. A Consumer configured without a dead-letter topic
// can get stuck reprocessing the same poison message across restarts,
// since a persistently failing Handler never advances the committed
// offset — configure WithDeadLetterTopic for any consumer where that is
// unacceptable.
package kafka
