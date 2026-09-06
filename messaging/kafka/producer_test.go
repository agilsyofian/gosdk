package kafka

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/agilsyofian/gosdk/core/observability/logger"
)

// fakeWriter is the mock transport seam for Producer tests (messaging.md
// section 32). It never talks to a real broker.
type fakeWriter struct {
	mu         sync.Mutex
	messages   []kafkago.Message
	writeErr   error
	writeDelay time.Duration
	closed     bool
	closeErr   error
}

func (f *fakeWriter) WriteMessages(ctx context.Context, msgs ...kafkago.Message) error {
	if f.writeDelay > 0 {
		select {
		case <-time.After(f.writeDelay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.writeErr != nil {
		return f.writeErr
	}
	f.messages = append(f.messages, msgs...)
	return nil
}

func (f *fakeWriter) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return f.closeErr
}

func newTestProducer(w kafkaWriter, topic string) *Producer {
	return &Producer{writer: w, topic: topic, logger: logger.Nop()}
}

func TestProducer_Publish_Success(t *testing.T) {
	fw := &fakeWriter{}
	p := newTestProducer(fw, "orders")

	msg := &Message{Payload: []byte("hello"), Headers: map[string]string{"trace-id": "abc"}}
	if err := p.Publish(context.Background(), msg); err != nil {
		t.Fatalf("Publish() error = %v, want nil", err)
	}
	if msg.ID == "" {
		t.Error("Publish() left msg.ID empty, want auto-generated ID")
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()
	if len(fw.messages) != 1 {
		t.Fatalf("writer received %d messages, want 1", len(fw.messages))
	}
	got := fw.messages[0]
	if got.Topic != "orders" || string(got.Value) != "hello" {
		t.Errorf("writer received %+v, want topic=orders value=hello", got)
	}
	foundTrace, foundID := false, false
	for _, h := range got.Headers {
		if h.Key == "trace-id" && string(h.Value) == "abc" {
			foundTrace = true
		}
		if h.Key == "x-message-id" && string(h.Value) == msg.ID {
			foundID = true
		}
	}
	if !foundTrace {
		t.Error("application header trace-id was not propagated")
	}
	if !foundID {
		t.Error("x-message-id header was not set to the generated message ID")
	}
}

func TestProducer_Publish_BrokerError(t *testing.T) {
	wantErr := errors.New("leader not available")
	fw := &fakeWriter{writeErr: wantErr}
	p := newTestProducer(fw, "orders")

	err := p.Publish(context.Background(), &Message{Payload: []byte("x")})
	if err == nil {
		t.Fatal("Publish() error = nil, want non-nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("Publish() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestProducer_Publish_ContextCancelled(t *testing.T) {
	fw := &fakeWriter{writeDelay: 50 * time.Millisecond}
	p := newTestProducer(fw, "orders")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := p.Publish(ctx, &Message{Payload: []byte("x")})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Publish() error = %v, want context.Canceled", err)
	}
}

func TestProducer_Publish_ContextDeadlineExceeded(t *testing.T) {
	fw := &fakeWriter{writeDelay: 50 * time.Millisecond}
	p := newTestProducer(fw, "orders")

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()

	err := p.Publish(ctx, &Message{Payload: []byte("x")})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Publish() error = %v, want context.DeadlineExceeded", err)
	}
}

func TestProducer_Publish_InvalidMessage(t *testing.T) {
	fw := &fakeWriter{}
	p := newTestProducer(fw, "orders")

	for _, msg := range []*Message{nil, {Payload: nil}, {Payload: []byte{}}} {
		if err := p.Publish(context.Background(), msg); !errors.Is(err, ErrInvalidMessage) {
			t.Errorf("Publish(%v) error = %v, want ErrInvalidMessage", msg, err)
		}
	}
}

func TestProducer_Close(t *testing.T) {
	fw := &fakeWriter{}
	p := newTestProducer(fw, "orders")

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	fw.mu.Lock()
	closedOnce := fw.closed
	fw.mu.Unlock()
	if !closedOnce {
		t.Error("underlying writer was not closed")
	}

	// Close must be idempotent: no panic, no error, and it must not
	// attempt to close the underlying writer a second time.
	if err := p.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}

	if err := p.Publish(context.Background(), &Message{Payload: []byte("x")}); !errors.Is(err, ErrClosed) {
		t.Errorf("Publish() after Close() error = %v, want ErrClosed", err)
	}
}

func TestProducer_Publish_Concurrent(t *testing.T) {
	fw := &fakeWriter{}
	p := newTestProducer(fw, "orders")

	const n = 50
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = p.Publish(context.Background(), &Message{Payload: []byte(fmt.Sprintf("msg-%d", i))})
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("concurrent Publish() #%d error = %v, want nil", i, err)
		}
	}
	fw.mu.Lock()
	defer fw.mu.Unlock()
	if len(fw.messages) != n {
		t.Errorf("writer received %d messages, want %d", len(fw.messages), n)
	}
}

type spyLogger struct {
	mu       sync.Mutex
	payloads []logger.LogPayload
}

func (s *spyLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads = append(s.payloads, payload)
}
func (s *spyLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads = append(s.payloads, payload)
}
func (s *spyLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads = append(s.payloads, payload)
}
func (s *spyLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.payloads = append(s.payloads, payload)
}

func TestProducer_Publish_OpenTelemetryPropagation(t *testing.T) {
	tp := sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	fw := &fakeWriter{}
	p := newTestProducer(fw, "orders")

	msg := &Message{Payload: []byte("trace-me")}
	if err := p.Publish(context.Background(), msg); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	fw.mu.Lock()
	defer fw.mu.Unlock()
	if len(fw.messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(fw.messages))
	}
	headers := headerMap(fw.messages[0].Headers)
	if headers["traceparent"] == "" {
		t.Error("expected traceparent to be injected into Kafka headers by OpenTelemetry")
	}
	if headers["x-trace-id"] == "" {
		t.Error("expected x-trace-id to be populated in Kafka headers")
	}
}

func TestProducer_Publish_LogPayloadOption(t *testing.T) {
	t.Run("log payload disabled by default", func(t *testing.T) {
		fw := &fakeWriter{}
		spy := &spyLogger{}
		p := &Producer{writer: fw, topic: "orders", logger: spy}

		if err := p.Publish(context.Background(), &Message{Payload: []byte("secret-data")}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if len(spy.payloads) != 1 {
			t.Fatalf("payloads = %d, want 1", len(spy.payloads))
		}
		if spy.payloads[0].Request != nil {
			t.Errorf("expected Request to be nil when enableLogPayload is false, got %v", spy.payloads[0].Request)
		}
	})

	t.Run("log payload enabled", func(t *testing.T) {
		fw := &fakeWriter{}
		spy := &spyLogger{}
		p := &Producer{writer: fw, topic: "orders", logger: spy, enableLogPayload: true}

		if err := p.Publish(context.Background(), &Message{Payload: []byte("secret-data")}); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
		if len(spy.payloads) != 1 {
			t.Fatalf("payloads = %d, want 1", len(spy.payloads))
		}
		if string(spy.payloads[0].Request.([]byte)) != "secret-data" {
			t.Errorf("expected Request = %q, got %v", "secret-data", spy.payloads[0].Request)
		}
	})
}
