package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/observability/logger"
)

// fakeAcknowledger backs amqp.Delivery.Ack/Nack/Reject in tests.
// amqp.Delivery is a concrete struct whose Ack/Nack/Reject methods
// simply delegate to its embedded Acknowledger field, so supplying a
// fake one here is the real seam — no need to fake Delivery itself.
type fakeAcknowledger struct {
	mu      sync.Mutex
	acked   []uint64
	nacked  []nackCall
	ackErr  error
	nackErr error
}

type nackCall struct {
	tag      uint64
	multiple bool
	requeue  bool
}

func (f *fakeAcknowledger) Ack(tag uint64, multiple bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ackErr != nil {
		return f.ackErr
	}
	f.acked = append(f.acked, tag)
	return nil
}

func (f *fakeAcknowledger) Nack(tag uint64, multiple, requeue bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.nackErr != nil {
		return f.nackErr
	}
	f.nacked = append(f.nacked, nackCall{tag, multiple, requeue})
	return nil
}

func (f *fakeAcknowledger) Reject(tag uint64, requeue bool) error { return nil }

// fakeConsumerChannel is the mock transport seam for Consumer tests
// (messaging.md section 32). It never talks to a real broker.
type fakeConsumerChannel struct {
	mu           sync.Mutex
	deliveries   chan amqp.Delivery
	consumeErr   error
	cancelCalled bool
	cancelErr    error
	closed       bool
	closeErr     error
}

func (f *fakeConsumerChannel) Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error) {
	if f.consumeErr != nil {
		return nil, f.consumeErr
	}
	return f.deliveries, nil
}

func (f *fakeConsumerChannel) Cancel(consumer string, noWait bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cancelCalled = true
	// Real broker behavior: once a Cancel is acknowledged, the
	// deliveries channel closes. Simulate that so Consume's range loop
	// ends, the same way it would against a real channel.
	if f.deliveries != nil {
		close(f.deliveries)
		f.deliveries = nil
	}
	return f.cancelErr
}

func (f *fakeConsumerChannel) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	if f.deliveries != nil {
		close(f.deliveries)
		f.deliveries = nil
	}
	return f.closeErr
}

func newTestConsumer(ch amqpConsumerChannel, concurrency int, requeueOnFailure bool) *Consumer {
	return &Consumer{
		channel:          ch,
		queue:            "orders.created",
		consumerTag:      "test-consumer",
		concurrency:      concurrency,
		requeueOnFailure: requeueOnFailure,
		logger:           logger.Nop(),
	}
}

func TestConsumer_Consume_SuccessfulHandler_Acks(t *testing.T) {
	ack := &fakeAcknowledger{}
	deliveries := make(chan amqp.Delivery, 1)
	deliveries <- amqp.Delivery{Acknowledger: ack, DeliveryTag: 1, Body: []byte("a")}
	fc := &fakeConsumerChannel{deliveries: deliveries}
	c := newTestConsumer(fc, 1, false)

	ctx, cancel := context.WithCancel(context.Background())
	var handled int32
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			atomic.AddInt32(&handled, 1)
			cancel()
			return nil
		})
	}()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	if atomic.LoadInt32(&handled) != 1 {
		t.Fatalf("handled = %d, want 1", handled)
	}
	ack.mu.Lock()
	defer ack.mu.Unlock()
	if len(ack.acked) != 1 || ack.acked[0] != 1 {
		t.Fatalf("acked = %v, want [1]", ack.acked)
	}
	if len(ack.nacked) != 0 {
		t.Fatalf("nacked = %v, want none", ack.nacked)
	}
}

func TestConsumer_Consume_HandlerError_NacksWithoutRequeue_ByDefault(t *testing.T) {
	ack := &fakeAcknowledger{}
	deliveries := make(chan amqp.Delivery, 1)
	deliveries <- amqp.Delivery{Acknowledger: ack, DeliveryTag: 5, Body: []byte("a")}
	fc := &fakeConsumerChannel{deliveries: deliveries}
	c := newTestConsumer(fc, 1, false) // requeueOnFailure defaults to false

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			cancel()
			return errors.New("boom")
		})
	}()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}

	ack.mu.Lock()
	defer ack.mu.Unlock()
	if len(ack.acked) != 0 {
		t.Errorf("acked = %v, want none", ack.acked)
	}
	if len(ack.nacked) != 1 || ack.nacked[0].tag != 5 || ack.nacked[0].requeue {
		t.Fatalf("nack call = %+v, want tag=5 requeue=false", ack.nacked)
	}
}

func TestConsumer_Consume_HandlerError_RequeueOnFailureTrue(t *testing.T) {
	ack := &fakeAcknowledger{}
	deliveries := make(chan amqp.Delivery, 1)
	deliveries <- amqp.Delivery{Acknowledger: ack, DeliveryTag: 9, Body: []byte("a")}
	fc := &fakeConsumerChannel{deliveries: deliveries}
	c := newTestConsumer(fc, 1, true) // requeueOnFailure = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			cancel()
			return errors.New("boom")
		})
	}()
	<-done

	ack.mu.Lock()
	defer ack.mu.Unlock()
	if len(ack.nacked) != 1 || !ack.nacked[0].requeue {
		t.Fatalf("nack call = %+v, want requeue=true", ack.nacked)
	}
}

func TestConsumer_Consume_ContextDeadline_NoDeliveries(t *testing.T) {
	fc := &fakeConsumerChannel{deliveries: make(chan amqp.Delivery)}
	c := newTestConsumer(fc, 1, false)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := c.Consume(ctx, func(ctx context.Context, msg *Message) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Consume() error = %v, want context.DeadlineExceeded", err)
	}
	fc.mu.Lock()
	defer fc.mu.Unlock()
	if !fc.cancelCalled {
		t.Error("expected channel.Cancel to be called once ctx ended")
	}
}

func TestConsumer_Consume_BrokerConsumeError(t *testing.T) {
	wantErr := errors.New("channel/connection not open")
	fc := &fakeConsumerChannel{consumeErr: wantErr}
	c := newTestConsumer(fc, 1, false)

	err := c.Consume(context.Background(), func(ctx context.Context, msg *Message) error { return nil })
	if !errors.Is(err, wantErr) {
		t.Fatalf("Consume() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestConsumer_Consume_ShutdownWaitsForInFlightHandler(t *testing.T) {
	ack := &fakeAcknowledger{}
	deliveries := make(chan amqp.Delivery, 1)
	deliveries <- amqp.Delivery{Acknowledger: ack, DeliveryTag: 1, Body: []byte("a")}
	fc := &fakeConsumerChannel{deliveries: deliveries}
	c := newTestConsumer(fc, 1, false)

	ctx, cancel := context.WithCancel(context.Background())
	handlerStarted := make(chan struct{})
	handlerFinished := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			close(handlerStarted)
			time.Sleep(30 * time.Millisecond)
			close(handlerFinished)
			return nil
		})
	}()

	<-handlerStarted
	cancel()

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	select {
	case <-handlerFinished:
	default:
		t.Fatal("Consume() returned before the in-flight handler finished")
	}
}

func TestConsumer_Consume_ConcurrentProcessing(t *testing.T) {
	const n = 20
	ack := &fakeAcknowledger{}
	deliveries := make(chan amqp.Delivery, n)
	for i := 0; i < n; i++ {
		deliveries <- amqp.Delivery{Acknowledger: ack, DeliveryTag: uint64(i + 1), Body: []byte(fmt.Sprintf("m%d", i))}
	}
	fc := &fakeConsumerChannel{deliveries: deliveries}
	c := newTestConsumer(fc, 5, false)

	ctx, cancel := context.WithCancel(context.Background())
	var handled int32
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			if atomic.AddInt32(&handled, 1) == int32(n) {
				cancel()
			}
			return nil
		})
	}()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	if int(atomic.LoadInt32(&handled)) != n {
		t.Fatalf("handled = %d, want %d", handled, n)
	}
	ack.mu.Lock()
	defer ack.mu.Unlock()
	if len(ack.acked) != n {
		t.Fatalf("acked = %d, want %d", len(ack.acked), n)
	}
}

func TestConsumer_Consume_NilHandlerRejected(t *testing.T) {
	fc := &fakeConsumerChannel{deliveries: make(chan amqp.Delivery)}
	c := newTestConsumer(fc, 1, false)
	if err := c.Consume(context.Background(), nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Consume(nil) error = %v, want ErrInvalidConfig", err)
	}
}

func TestConsumer_Close(t *testing.T) {
	fc := &fakeConsumerChannel{deliveries: make(chan amqp.Delivery)}
	c := newTestConsumer(fc, 1, false)

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	fc.mu.Lock()
	closedOnce := fc.closed
	fc.mu.Unlock()
	if !closedOnce {
		t.Error("underlying channel was not closed")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	if err := c.Consume(context.Background(), func(context.Context, *Message) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Errorf("Consume() after Close() error = %v, want ErrClosed", err)
	}
}
