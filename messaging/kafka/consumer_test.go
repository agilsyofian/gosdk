package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// fakeReader is the mock transport seam for Consumer tests (messaging.md
// section 32). It never talks to a real broker: it replays a fixed slice
// of messages, then either returns a configured error or blocks until
// the context is cancelled, mirroring how a real Reader waits on an idle
// partition.
type fakeReader struct {
	mu         sync.Mutex
	messages   []kafkago.Message
	next       int
	fetchErr   error
	fetchDelay time.Duration
	committed  []kafkago.Message
	commitErr  error
	closed     bool
	closeErr   error
}

func (f *fakeReader) FetchMessage(ctx context.Context) (kafkago.Message, error) {
	f.mu.Lock()
	if f.next < len(f.messages) {
		m := f.messages[f.next]
		f.next++
		f.mu.Unlock()
		if f.fetchDelay > 0 {
			select {
			case <-time.After(f.fetchDelay):
			case <-ctx.Done():
				return kafkago.Message{}, ctx.Err()
			}
		}
		return m, nil
	}
	err := f.fetchErr
	f.mu.Unlock()

	if err != nil {
		return kafkago.Message{}, err
	}
	// Messages exhausted and no error configured: behave like a real
	// Reader parked on an empty partition, waiting for ctx to end.
	<-ctx.Done()
	return kafkago.Message{}, ctx.Err()
}

func (f *fakeReader) CommitMessages(ctx context.Context, msgs ...kafkago.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.commitErr != nil {
		return f.commitErr
	}
	f.committed = append(f.committed, msgs...)
	return nil
}

func (f *fakeReader) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}

func newTestConsumer(r kafkaReader, concurrency int) *Consumer {
	return &Consumer{reader: r, topic: "orders", groupID: "billing", concurrency: concurrency, logger: logger.Nop()}
}

func headerMap(hs []kafkago.Header) map[string]string {
	m := make(map[string]string, len(hs))
	for _, h := range hs {
		m[h.Key] = string(h.Value)
	}
	return m
}

func TestConsumer_Consume_SuccessfulHandler_Commits(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 1, Value: []byte("a")}}}
	c := newTestConsumer(fr, 1)

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
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.committed) != 1 {
		t.Fatalf("committed = %d, want 1", len(fr.committed))
	}
}

func TestConsumer_Consume_HandlerError_NoDeadLetter_LeavesUncommitted(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 1, Value: []byte("a")}}}
	c := newTestConsumer(fr, 1)

	ctx, cancel := context.WithCancel(context.Background())
	handlerErr := errors.New("boom")

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			cancel()
			return handlerErr
		})
	}()

	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.committed) != 0 {
		t.Fatalf("committed = %d, want 0 (handler failed, no dead-letter topic configured)", len(fr.committed))
	}
}

func TestConsumer_Consume_HandlerError_WithDeadLetter_PublishesAndCommits(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Partition: 2, Offset: 7, Value: []byte("a")}}}
	dlqWriter := &fakeWriter{}
	c := newTestConsumer(fr, 1)
	c.dlq = newTestProducer(dlqWriter, "orders.dlq")

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

	fr.mu.Lock()
	committed := len(fr.committed)
	fr.mu.Unlock()
	if committed != 1 {
		t.Fatalf("committed = %d, want 1 (message was preserved in the dead-letter topic)", committed)
	}

	dlqWriter.mu.Lock()
	defer dlqWriter.mu.Unlock()
	if len(dlqWriter.messages) != 1 {
		t.Fatalf("dead-letter messages = %d, want 1", len(dlqWriter.messages))
	}
	h := headerMap(dlqWriter.messages[0].Headers)
	if h["x-original-topic"] != "orders" || h["x-original-partition"] != "2" || h["x-original-offset"] != "7" || h["x-error"] != "boom" {
		t.Errorf("dead-letter headers = %+v, missing expected original-message metadata", h)
	}
}

func TestConsumer_Consume_DeadLetterPublishFails_LeavesUncommitted(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 3, Value: []byte("a")}}}
	dlqWriter := &fakeWriter{writeErr: errors.New("dlq broker unreachable")}
	c := newTestConsumer(fr, 1)
	c.dlq = newTestProducer(dlqWriter, "orders.dlq")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			cancel()
			return errors.New("boom")
		})
	}()
	<-done

	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.committed) != 0 {
		t.Fatalf("committed = %d, want 0 (dead-letter publish itself failed)", len(fr.committed))
	}
}

func TestConsumer_Consume_ContextDeadline_NoMessages(t *testing.T) {
	fr := &fakeReader{}
	c := newTestConsumer(fr, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	err := c.Consume(ctx, func(ctx context.Context, msg *Message) error { return nil })
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Consume() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestConsumer_Consume_BrokerFetchError(t *testing.T) {
	wantErr := errors.New("connection refused")
	fr := &fakeReader{fetchErr: wantErr}
	c := newTestConsumer(fr, 1)

	err := c.Consume(context.Background(), func(ctx context.Context, msg *Message) error { return nil })
	if !errors.Is(err, wantErr) {
		t.Fatalf("Consume() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestConsumer_Consume_ShutdownWaitsForInFlightHandler(t *testing.T) {
	fr := &fakeReader{messages: []kafkago.Message{{Topic: "orders", Offset: 1, Value: []byte("a")}}}
	c := newTestConsumer(fr, 1)

	ctx, cancel := context.WithCancel(context.Background())
	handlerStarted := make(chan struct{})
	handlerFinished := make(chan struct{})

	done := make(chan error, 1)
	go func() {
		done <- c.Consume(ctx, func(ctx context.Context, msg *Message) error {
			close(handlerStarted)
			time.Sleep(30 * time.Millisecond) // simulate slow work in flight
			close(handlerFinished)
			return nil
		})
	}()

	<-handlerStarted
	cancel() // ask Consume to stop while the handler above is still running

	err := <-done
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Consume() error = %v, want context.Canceled", err)
	}
	select {
	case <-handlerFinished:
		// Consume() only returned above after this closed, since it
		// waits on the same WaitGroup the handler goroutine holds.
	default:
		t.Fatal("Consume() returned before the in-flight handler finished")
	}
}

func TestConsumer_Consume_ConcurrentProcessing(t *testing.T) {
	const n = 20
	msgs := make([]kafkago.Message, n)
	for i := range msgs {
		msgs[i] = kafkago.Message{Topic: "orders", Offset: int64(i), Value: []byte(fmt.Sprintf("m%d", i))}
	}
	exhausted := errors.New("exhausted")
	fr := &fakeReader{messages: msgs, fetchErr: exhausted}
	c := newTestConsumer(fr, 5)

	var handled int32
	err := c.Consume(context.Background(), func(ctx context.Context, msg *Message) error {
		atomic.AddInt32(&handled, 1)
		return nil
	})
	if !errors.Is(err, exhausted) {
		t.Fatalf("Consume() error = %v, want it to wrap %v", err, exhausted)
	}
	if int(atomic.LoadInt32(&handled)) != n {
		t.Fatalf("handled = %d, want %d", handled, n)
	}
	fr.mu.Lock()
	defer fr.mu.Unlock()
	if len(fr.committed) != n {
		t.Fatalf("committed = %d, want %d", len(fr.committed), n)
	}
}

func TestConsumer_Consume_NilHandlerRejected(t *testing.T) {
	fr := &fakeReader{}
	c := newTestConsumer(fr, 1)

	if err := c.Consume(context.Background(), nil); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("Consume(nil) error = %v, want ErrInvalidConfig", err)
	}
}

func TestConsumer_Close(t *testing.T) {
	fr := &fakeReader{}
	dlqWriter := &fakeWriter{}
	c := newTestConsumer(fr, 1)
	c.dlq = newTestProducer(dlqWriter, "orders.dlq")

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	fr.mu.Lock()
	closedOnce := fr.closed
	fr.mu.Unlock()
	if !closedOnce {
		t.Error("underlying reader was not closed")
	}
	dlqWriter.mu.Lock()
	dlqClosedOnce := dlqWriter.closed
	dlqWriter.mu.Unlock()
	if !dlqClosedOnce {
		t.Error("dead-letter producer was not closed")
	}

	// Idempotent: no panic, no error.
	if err := c.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	if err := c.Consume(context.Background(), func(context.Context, *Message) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Errorf("Consume() after Close() error = %v, want ErrClosed", err)
	}
}
