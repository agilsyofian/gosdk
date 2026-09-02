package kafka

import "errors"

// Sentinel errors returned by this package. Callers should use errors.Is
// to check for these rather than comparing strings, since every returned
// error wraps the underlying cause (see messaging.md section 24).
var (
	// ErrInvalidConfig is returned by New, Client.Producer, and
	// Client.Consumer when the supplied configuration fails validation.
	ErrInvalidConfig = errors.New("kafka: invalid configuration")

	// ErrInvalidMessage is returned by Producer.Publish when the message
	// fails validation before being sent to the broker.
	ErrInvalidMessage = errors.New("kafka: invalid message")

	// ErrClosed is returned by operations attempted after Close has been
	// called on the owning Client, Producer, or Consumer.
	ErrClosed = errors.New("kafka: closed")

	// ErrHandlerFailed indicates a consumer Handler returned a non-nil
	// error for a message. The handler's original error is preserved and
	// reachable via errors.Unwrap/errors.As.
	ErrHandlerFailed = errors.New("kafka: handler failed")

	// ErrDeadLetterFailed indicates a message's Handler failed AND the
	// subsequent attempt to forward the message to the configured
	// dead-letter topic also failed. The original offset is not committed
	// in this case, so the message will be redelivered — see
	// messaging.md section 16.
	ErrDeadLetterFailed = errors.New("kafka: dead-letter publish failed")
)

// Note on broker-originated errors: WriteMessages, FetchMessage, and
// CommitMessages return kafka-go's own error values on transport or
// broker failures (leader-not-available, i/o timeout, connection refused,
// and so on). This package wraps those with operation context using
// fmt.Errorf("kafka: <op>: %w", err) rather than reclassifying them into
// a synthetic sentinel, because kafka-go does not expose a taxonomy rich
// enough to reclassify them correctly in every case. Callers needing to
// branch on a specific broker condition should use errors.As against the
// kafka-go error types directly; callers only needing to detect context
// cancellation/deadline can rely on errors.Is(err, context.Canceled) /
// errors.Is(err, context.DeadlineExceeded), which continue to work
// through the wrapped chain (messaging.md section 25).
