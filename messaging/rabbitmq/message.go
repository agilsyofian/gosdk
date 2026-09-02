package rabbitmq

import (
	"crypto/rand"
	"encoding/hex"
)

// Message is this package's transport-neutral envelope. Payload is
// treated as opaque (messaging.md section 10). Do not add domain-specific
// fields here — those belong in the consuming service's own type,
// decoded from Payload.
//
// This type intentionally differs from messaging/kafka.Message: Kafka's
// Key/Partition/Offset map to RabbitMQ's RoutingKey/Exchange/DeliveryTag,
// but the concepts are not identical (messaging.md section 4), so each
// package gets its own envelope shaped for its own protocol rather than
// a shared struct with fields that only make sense for one broker.
type Message struct {
	// ID uniquely identifies this message for logging and tracing.
	// Publish generates one automatically when left empty. Carried over
	// AMQP as the message-id property.
	ID string

	// RoutingKey determines how the exchange routes this message. Against
	// the default (empty-name) exchange, RoutingKey must equal the target
	// queue name.
	RoutingKey string

	// Payload is the opaque message body.
	Payload []byte

	// Headers carries framework and application metadata (for example
	// trace/correlation IDs — messaging.md section 12), transported as
	// AMQP headers.
	Headers map[string]string

	// ContentType optionally describes the encoding of Payload. Never
	// interpreted by this package.
	ContentType string

	// CorrelationID is carried as the AMQP correlation-id property,
	// distinct from Headers so RPC-style request/response patterns that
	// rely on it interoperate with non-Go AMQP clients.
	CorrelationID string

	// Exchange, DeliveryTag, and Redelivered are populated by Consumer on
	// delivery. They are ignored by Publish.
	Exchange    string
	DeliveryTag uint64
	Redelivered bool
}

// newMessageID returns a random hex-encoded identifier, generated locally
// rather than pulled from a shared helper — messaging.md section 2
// forbids a messaging/common package, so a few duplicated lines here and
// in messaging/kafka/message.go is the intended trade-off.
func newMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unavailable-id"
	}
	return hex.EncodeToString(b[:])
}
