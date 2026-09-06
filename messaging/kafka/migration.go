package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// DualConsumer coordinates message consumption across two Kafka clusters
// (e.g. Primary/Old and Secondary/New cluster) using a single business Handler.
// This enables zero-downtime cluster migrations.
type DualConsumer struct {
	primary   *Consumer
	secondary *Consumer

	closed atomic.Bool
}

// NewDualConsumer creates a DualConsumer. Primary consumer is required;
// secondary is optional. If secondary is nil, DualConsumer delegates all work
// solely to the primary consumer.
func NewDualConsumer(primary, secondary *Consumer) (*DualConsumer, error) {
	if primary == nil {
		return nil, fmt.Errorf("%w: primary consumer is required", ErrInvalidConfig)
	}
	return &DualConsumer{
		primary:   primary,
		secondary: secondary,
	}, nil
}

// Consume starts consuming messages. If both primary and secondary consumers
// are configured, it runs both concurrently until ctx is cancelled or an
// unrecoverable error occurs. If one consumer fails with a fatal error,
// the coordinated context is cancelled to stop the other consumer cleanly.
func (dc *DualConsumer) Consume(ctx context.Context, handler Handler) error {
	if dc.closed.Load() {
		return ErrClosed
	}
	if handler == nil {
		return fmt.Errorf("%w: handler must not be nil", ErrInvalidConfig)
	}

	if dc.secondary == nil {
		return dc.primary.Consume(ctx, handler)
	}

	childCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	runConsumer := func(c *Consumer) {
		defer wg.Done()
		if err := c.Consume(childCtx, handler); err != nil && !errors.Is(err, context.Canceled) {
			mu.Lock()
			errs = append(errs, err)
			mu.Unlock()
			cancel()
		}
	}

	wg.Add(2)
	go runConsumer(dc.primary)
	go runConsumer(dc.secondary)
	wg.Wait()

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// Close closes both consumers. It is idempotent and safe for concurrent use.
func (dc *DualConsumer) Close() error {
	if !dc.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	if err := dc.primary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close primary consumer: %w", err))
	}
	if dc.secondary != nil {
		if err := dc.secondary.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close secondary consumer: %w", err))
		}
	}
	return errors.Join(errs...)
}

// MigrationProducer manages publishing across two Kafka clusters during a migration.
// It supports switching active traffic between Primary and Secondary clusters, as well
// as optional dual-publishing (shadow writing to the other cluster).
type MigrationProducer struct {
	primary   *Producer
	secondary *Producer

	useSecondary atomic.Bool
	dualPublish  atomic.Bool
	closed       atomic.Bool
}

// NewMigrationProducer creates a MigrationProducer. Primary producer is required;
// secondary is optional.
func NewMigrationProducer(primary, secondary *Producer) (*MigrationProducer, error) {
	if primary == nil {
		return nil, fmt.Errorf("%w: primary producer is required", ErrInvalidConfig)
	}
	return &MigrationProducer{
		primary:   primary,
		secondary: secondary,
	}, nil
}

// UseSecondary switches active publishing to the secondary cluster if enable is true,
// or back to the primary cluster if false. Has no effect if secondary is nil.
func (mp *MigrationProducer) UseSecondary(enable bool) {
	mp.useSecondary.Store(enable)
}

// IsUsingSecondary returns whether publishing is directed to the secondary cluster.
func (mp *MigrationProducer) IsUsingSecondary() bool {
	return mp.useSecondary.Load() && mp.secondary != nil
}

// SetDualPublish enables or disables shadow publishing to the inactive cluster.
// Has no effect if secondary is nil.
func (mp *MigrationProducer) SetDualPublish(enable bool) {
	mp.dualPublish.Store(enable)
}

// IsDualPublish returns whether dual publishing is active.
func (mp *MigrationProducer) IsDualPublish() bool {
	return mp.dualPublish.Load() && mp.secondary != nil
}

// Publish publishes msg to the active cluster. If dual publish is enabled, it also
// publishes a clone of the message to the other cluster.
func (mp *MigrationProducer) Publish(ctx context.Context, msg *Message) error {
	if mp.closed.Load() {
		return ErrClosed
	}

	active, shadow := mp.getTargets()
	if err := active.Publish(ctx, msg); err != nil {
		return err
	}

	if mp.IsDualPublish() && shadow != nil {
		shadowMsg := &Message{
			ID:          msg.ID,
			Key:         msg.Key,
			Payload:     msg.Payload,
			ContentType: msg.ContentType,
			Topic:       msg.Topic,
			Headers:     cloneHeaders(msg.Headers),
		}
		_ = shadow.Publish(ctx, shadowMsg)
	}

	return nil
}

func (mp *MigrationProducer) getTargets() (active *Producer, shadow *Producer) {
	if mp.IsUsingSecondary() {
		return mp.secondary, mp.primary
	}
	return mp.primary, mp.secondary
}

// Close closes both producers. It is idempotent and safe for concurrent use.
func (mp *MigrationProducer) Close() error {
	if !mp.closed.CompareAndSwap(false, true) {
		return nil
	}

	var errs []error
	if err := mp.primary.Close(); err != nil {
		errs = append(errs, fmt.Errorf("close primary producer: %w", err))
	}
	if mp.secondary != nil {
		if err := mp.secondary.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close secondary producer: %w", err))
		}
	}
	return errors.Join(errs...)
}
