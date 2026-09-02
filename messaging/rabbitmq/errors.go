package rabbitmq

import "errors"

// Sentinel errors returned by this package. Callers should use errors.Is
// to check for these — every returned error wraps its underlying cause
// (messaging.md section 24).
var (
	// ErrInvalidConfig is returned by New, Client.Producer, and
	// Client.Consumer when the supplied configuration fails validation.
	ErrInvalidConfig = errors.New("rabbitmq: invalid configuration")

	// ErrInvalidMessage is returned by Producer.Publish when the message
	// fails validation before being sent to the broker.
	ErrInvalidMessage = errors.New("rabbitmq: invalid message")

	// ErrClosed is returned by operations attempted after Close has been
	// called on the owning Client, Producer, or Consumer.
	ErrClosed = errors.New("rabbitmq: closed")

	// ErrConsumerStopped is returned by Consume when the broker or
	// channel ended the delivery stream for a reason other than the
	// caller's context being done (for example the underlying channel
	// or connection was closed directly via Close, or the broker
	// cancelled the consumer). Compare with errors.Is; when the caller's
	// context caused the stop, Consume returns ctx.Err() instead.
	ErrConsumerStopped = errors.New("rabbitmq: consumer stopped")
)

// Note on broker-originated errors: PublishWithContext, Consume, Qos, and
// friends return amqp091-go's own error values (amqp.Error, connection
// and channel errors) on protocol or broker failures. This package wraps
// those with operation context using fmt.Errorf("rabbitmq: <op>: %w",
// err) rather than reclassifying them into a synthetic sentinel, for the
// same reason documented in messaging/kafka/errors.go. Callers needing to
// branch on a specific broker condition (for example an amqp.Error with
// a particular reply code) should use errors.As against amqp091-go's
// types directly.
