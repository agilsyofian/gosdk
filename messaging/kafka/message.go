package kafka

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

// Message is this package's transport-neutral envelope. Payload is treated
// as opaque: the messaging layer never assumes a serialization format
// (messaging.md section 10). Do not add domain-specific fields here —
// those belong in the consuming service's own type, decoded from Payload.
type Message struct {
	// ID uniquely identifies this message for logging and tracing.
	// Publish generates one automatically when left empty.
	ID string

	// Key is the Kafka partition/routing key. Messages sharing a key are
	// routed to the same partition, which is what gives them relative
	// order. Leave nil to let the broker/balancer choose a partition.
	Key []byte

	// Payload is the opaque message body.
	Payload []byte

	// Headers carries framework and application metadata (for example
	// trace/correlation IDs — see messaging.md section 12). Keys and
	// values are transported as Kafka record headers.
	Headers map[string]string

	// ContentType optionally describes the encoding of Payload (e.g.
	// "application/json"). It is never interpreted by this package.
	ContentType string

	// Topic overrides the Producer's default topic for this message when
	// set. Consumers populate it with the topic the message was read
	// from.
	Topic string

	// Partition and Offset are populated by Consumer on delivery. They
	// are ignored by Publish — Kafka's balancer chooses the partition,
	// and the broker assigns the offset.
	Partition int
	Offset    int64

	// Time is the broker-assigned record timestamp. Populated by Consumer
	// on delivery; ignored by Publish.
	Time time.Time
}

// newMessageID returns a random hex-encoded identifier. It is generated
// locally (rather than pulled from a shared helper) because messaging.md
// section 2 forbids a messaging/common package — a few duplicated lines
// per provider is the intended trade-off over a shared abstraction two
// packages would otherwise have to agree on.
func newMessageID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		// crypto/rand.Read failing means the platform's CSPRNG is broken;
		// fall back to a fixed-but-distinguishable marker rather than
		// panicking from inside a library.
		return "unavailable-id"
	}
	return hex.EncodeToString(b[:])
}
