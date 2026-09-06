package kafka

import (
	"context"
	"errors"
	"sync"
	"testing"

	kafkago "github.com/segmentio/kafka-go"
)

func TestDualConsumer_RequiresPrimary(t *testing.T) {
	_, err := NewDualConsumer(nil, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewDualConsumer(nil, nil) error = %v, want ErrInvalidConfig", err)
	}
}

func TestDualConsumer_PrimaryOnly(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 1, Value: []byte("msg1")}}}
	c := newTestConsumer(fr, 1)

	dc, err := NewDualConsumer(c, nil)
	if err != nil {
		t.Fatalf("NewDualConsumer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var handled bool
	done := make(chan error, 1)
	go func() {
		done <- dc.Consume(ctx, func(ctx context.Context, msg *Message) error {
			handled = true
			cancel()
			return nil
		})
	}()

	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	if !handled {
		t.Fatal("expected message to be handled by primary-only DualConsumer")
	}
}

func TestDualConsumer_BothActive(t *testing.T) {
	fr1 := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 1, Value: []byte("from-primary")}}}
	fr2 := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 100, Value: []byte("from-secondary")}}}

	c1 := newTestConsumer(fr1, 1)
	c2 := newTestConsumer(fr2, 1)

	dc, err := NewDualConsumer(c1, c2)
	if err != nil {
		t.Fatalf("NewDualConsumer() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	var received []string

	done := make(chan error, 1)
	go func() {
		done <- dc.Consume(ctx, func(ctx context.Context, msg *Message) error {
			mu.Lock()
			received = append(received, string(msg.Payload))
			count := len(received)
			mu.Unlock()

			if count == 2 {
				cancel()
			}
			return nil
		})
	}()

	err = <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 2 {
		t.Fatalf("received = %d, want 2", len(received))
	}
}

func TestDualConsumer_FatalErrorInOne_CancelsOther(t *testing.T) {
	transientErr := errors.New("cluster broker died")
	fr1 := &fakeReader{fetchErr: transientErr}
	fr2 := &fakeReader{} // parks waiting on context

	c1 := newTestConsumer(fr1, 1)
	c2 := newTestConsumer(fr2, 1)

	dc, err := NewDualConsumer(c1, c2)
	if err != nil {
		t.Fatalf("NewDualConsumer() error = %v", err)
	}

	err = dc.Consume(context.Background(), func(ctx context.Context, msg *Message) error { return nil })
	if !errors.Is(err, transientErr) {
		t.Fatalf("Consume() error = %v, want it to wrap transientErr", err)
	}
}

func TestDualConsumer_Close(t *testing.T) {
	fr1 := &fakeReader{}
	fr2 := &fakeReader{}
	c1 := newTestConsumer(fr1, 1)
	c2 := newTestConsumer(fr2, 1)

	dc, err := NewDualConsumer(c1, c2)
	if err != nil {
		t.Fatalf("NewDualConsumer() error = %v", err)
	}

	if err := dc.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if !fr1.closed || !fr2.closed {
		t.Fatal("expected both underlying readers to be closed")
	}

	// Calling Close again is idempotent
	if err := dc.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	// Consume after Close fails with ErrClosed
	if err := dc.Consume(context.Background(), func(ctx context.Context, msg *Message) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Fatalf("Consume() after Close() error = %v, want ErrClosed", err)
	}
}

func TestMigrationProducer_RequiresPrimary(t *testing.T) {
	_, err := NewMigrationProducer(nil, nil)
	if !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("NewMigrationProducer(nil, nil) error = %v, want ErrInvalidConfig", err)
	}
}

func TestMigrationProducer_Switching(t *testing.T) {
	fw1 := &fakeWriter{}
	fw2 := &fakeWriter{}
	p1 := newTestProducer(fw1, "orders")
	p2 := newTestProducer(fw2, "orders")

	mp, err := NewMigrationProducer(p1, p2)
	if err != nil {
		t.Fatalf("NewMigrationProducer() error = %v", err)
	}

	ctx := context.Background()

	// 1. By default, writes to Primary
	if err := mp.Publish(ctx, &Message{Payload: []byte("msg1")}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	fw1.mu.Lock()
	p1Count := len(fw1.messages)
	fw1.mu.Unlock()
	if p1Count != 1 {
		t.Fatalf("primary count = %d, want 1", p1Count)
	}

	// 2. Switch to Secondary
	mp.UseSecondary(true)
	if !mp.IsUsingSecondary() {
		t.Fatal("expected IsUsingSecondary() to be true")
	}

	if err := mp.Publish(ctx, &Message{Payload: []byte("msg2")}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	fw2.mu.Lock()
	p2Count := len(fw2.messages)
	fw2.mu.Unlock()
	if p2Count != 1 {
		t.Fatalf("secondary count = %d, want 1", p2Count)
	}

	// 3. Switch back to Primary
	mp.UseSecondary(false)
	if mp.IsUsingSecondary() {
		t.Fatal("expected IsUsingSecondary() to be false")
	}

	if err := mp.Publish(ctx, &Message{Payload: []byte("msg3")}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	fw1.mu.Lock()
	p1Count = len(fw1.messages)
	fw1.mu.Unlock()
	if p1Count != 2 {
		t.Fatalf("primary count = %d, want 2", p1Count)
	}
}

func TestMigrationProducer_DualPublish(t *testing.T) {
	fw1 := &fakeWriter{}
	fw2 := &fakeWriter{}
	p1 := newTestProducer(fw1, "orders")
	p2 := newTestProducer(fw2, "orders")

	mp, err := NewMigrationProducer(p1, p2)
	if err != nil {
		t.Fatalf("NewMigrationProducer() error = %v", err)
	}

	mp.SetDualPublish(true)
	if !mp.IsDualPublish() {
		t.Fatal("expected IsDualPublish() to be true")
	}

	ctx := context.Background()
	if err := mp.Publish(ctx, &Message{Payload: []byte("shadow-test")}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	fw1.mu.Lock()
	p1Count := len(fw1.messages)
	fw1.mu.Unlock()
	fw2.mu.Lock()
	p2Count := len(fw2.messages)
	fw2.mu.Unlock()

	if p1Count != 1 || p2Count != 1 {
		t.Fatalf("dual publish: primary=%d, secondary=%d, want 1 each", p1Count, p2Count)
	}
}

func TestMigrationProducer_Close(t *testing.T) {
	fw1 := &fakeWriter{}
	fw2 := &fakeWriter{}
	p1 := newTestProducer(fw1, "orders")
	p2 := newTestProducer(fw2, "orders")

	mp, err := NewMigrationProducer(p1, p2)
	if err != nil {
		t.Fatalf("NewMigrationProducer() error = %v", err)
	}

	if err := mp.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if !fw1.closed || !fw2.closed {
		t.Fatal("expected both underlying writers to be closed")
	}

	// Idempotent close
	if err := mp.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	// Publish after close fails with ErrClosed
	if err := mp.Publish(context.Background(), &Message{Payload: []byte("a")}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Publish() after Close() error = %v, want ErrClosed", err)
	}
}
