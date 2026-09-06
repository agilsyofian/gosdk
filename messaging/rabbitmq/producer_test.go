package rabbitmq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// fakePublisher is the mock transport seam for Producer tests
// (messaging.md section 32). It never talks to a real broker.
type fakePublisher struct {
	mu           sync.Mutex
	published    []amqp.Publishing
	exchanges    []string
	routingKeys  []string
	publishErr   error
	publishDelay time.Duration
	closed       bool
	closeErr     error
}

func (f *fakePublisher) PublishWithContext(ctx context.Context, exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	if f.publishDelay > 0 {
		select {
		case <-time.After(f.publishDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publishErr != nil {
		return f.publishErr
	}
	f.published = append(f.published, msg)
	f.exchanges = append(f.exchanges, exchange)
	f.routingKeys = append(f.routingKeys, key)
	return nil
}

func (f *fakePublisher) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}

func newTestProducer(pub amqpPublisher, exchange string) *Producer {
	return &Producer{channel: pub, exchange: exchange, logger: logger.Nop()}
}

func TestProducer_Publish_Success(t *testing.T) {
	fp := &fakePublisher{}
	p := newTestProducer(fp, "orders")

	msg := &Message{RoutingKey: "orders.created", Payload: []byte("hello"), Headers: map[string]string{"trace-id": "abc"}}
	if err := p.Publish(context.Background(), msg); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
	if msg.ID == "" {
		t.Error("Publish() left msg.ID empty, want auto-generated ID")
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.published) != 1 {
		t.Fatalf("published %d messages, want 1", len(fp.published))
	}
	if fp.exchanges[0] != "orders" || fp.routingKeys[0] != "orders.created" {
		t.Errorf("published to exchange=%q key=%q, want exchange=orders key=orders.created", fp.exchanges[0], fp.routingKeys[0])
	}
	if string(fp.published[0].Body) != "hello" {
		t.Errorf("body = %q, want hello", fp.published[0].Body)
	}
	if fp.published[0].Headers["trace-id"] != "abc" {
		t.Errorf("header trace-id = %v, want abc", fp.published[0].Headers["trace-id"])
	}
	if fp.published[0].MessageId != msg.ID {
		t.Errorf("MessageId = %q, want %q", fp.published[0].MessageId, msg.ID)
	}
}

func TestProducer_Publish_BrokerError(t *testing.T) {
	wantErr := errors.New("channel/connection not open")
	fp := &fakePublisher{publishErr: wantErr}
	p := newTestProducer(fp, "orders")

	err := p.Publish(context.Background(), &Message{RoutingKey: "k", Payload: []byte("x")})
	if !errors.Is(err, wantErr) {
		t.Errorf("Publish() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestProducer_Publish_ContextCancelled(t *testing.T) {
	fp := &fakePublisher{publishDelay: 50 * time.Millisecond}
	p := newTestProducer(fp, "orders")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Publish(ctx, &Message{RoutingKey: "k", Payload: []byte("x")})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Publish() error = %v, want context.Canceled", err)
	}
}

func TestProducer_Publish_ContextDeadlineExceeded(t *testing.T) {
	fp := &fakePublisher{publishDelay: 50 * time.Millisecond}
	p := newTestProducer(fp, "orders")

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := p.Publish(ctx, &Message{RoutingKey: "k", Payload: []byte("x")})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Publish() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestProducer_Publish_InvalidMessage(t *testing.T) {
	fp := &fakePublisher{}
	p := newTestProducer(fp, "orders")

	for _, msg := range []*Message{nil, {Payload: nil}, {Payload: []byte{}}} {
		if err := p.Publish(context.Background(), msg); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("Publish(%v) error = %v, want ErrInvalidMessage", msg, err)
		}
	}
}

func TestProducer_Close(t *testing.T) {
	fp := &fakePublisher{}
	p := newTestProducer(fp, "orders")

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	fp.mu.Lock()
	closedOnce := fp.closed
	fp.mu.Unlock()
	if !closedOnce {
		t.Error("underlying channel was not closed")
	}

	if err := p.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	if err := p.Publish(context.Background(), &Message{RoutingKey: "k", Payload: []byte("x")}); !errors.Is(err, ErrClosed) {
		t.Errorf("Publish() after Close() error = %v, want ErrClosed", err)
	}
}

func TestProducer_Publish_Concurrent(t *testing.T) {
	fp := &fakePublisher{}
	p := newTestProducer(fp, "orders")

	const n = 50
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = p.Publish(context.Background(), &Message{RoutingKey: "k", Payload: []byte(fmt.Sprintf("msg-%d", i))})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent Publish() #%d error = %v, want nil", i, err)
		}
	}
	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.published) != n {
		t.Errorf("published %d messages, want %d", len(fp.published), n)
	}
}
