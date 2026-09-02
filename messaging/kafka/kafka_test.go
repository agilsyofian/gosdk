package kafka

import (
	"context"
	"errors"
	"testing"
)

func TestNew_ValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"no brokers", Config{}},
		{"empty broker string", Config{Brokers: []string{""}}},
		{"blank broker string", Config{Brokers: []string{"   "}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("New(%+v) error = %v, want ErrInvalidConfig", tt.cfg, err)
			}
		})
	}
}

func TestNew_Success(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v, want nil", err)
	}
	if c == nil {
		t.Fatal("New() returned a nil Client with a nil error")
	}
}

func TestClient_Producer_RequiresTopic(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := c.Producer(""); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Producer(\"\") error = %v, want ErrInvalidConfig", err)
	}
}

func TestClient_Consumer_RequiresTopicAndGroup(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := c.Consumer("", "billing"); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Consumer(\"\", \"billing\") error = %v, want ErrInvalidConfig", err)
	}
	if _, err := c.Consumer("orders", ""); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Consumer(\"orders\", \"\") error = %v, want ErrInvalidConfig", err)
	}
}

func TestClient_Consumer_ValidatesConcurrency(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if _, err := c.Consumer("orders", "billing", WithConcurrency(0)); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Consumer with WithConcurrency(0) error = %v, want ErrInvalidConfig", err)
	}
}

func TestClient_Start_Stop(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	p, err := c.Producer("orders")
	if err != nil {
		t.Fatalf("Producer() error = %v", err)
	}
	cons, err := c.Consumer("orders", "billing")
	if err != nil {
		t.Fatalf("Consumer() error = %v", err)
	}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}

	// Client.Close cascades to everything it spawned.
	if err := p.Publish(context.Background(), &Message{Payload: []byte("x")}); !errors.Is(err, ErrClosed) {
		t.Errorf("Publish() after Client.Close() error = %v, want ErrClosed", err)
	}
	if err := cons.Consume(context.Background(), func(context.Context, *Message) error { return nil }); !errors.Is(err, ErrClosed) {
		t.Errorf("Consume() after Client.Close() error = %v, want ErrClosed", err)
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}
}

func TestClient_ProducerAndConsumer_FailAfterClose(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := c.Producer("orders"); !errors.Is(err, ErrClosed) {
		t.Errorf("Producer() after Close() error = %v, want ErrClosed", err)
	}
	if _, err := c.Consumer("orders", "billing"); !errors.Is(err, ErrClosed) {
		t.Errorf("Consumer() after Close() error = %v, want ErrClosed", err)
	}
}

// fakeCloser lets the "failure during shutdown" scenario be tested
// without a real broker: Client.Close must join every child's error
// rather than stopping at the first one or swallowing it.
type fakeCloser struct{ err error }

func (f fakeCloser) Close() error { return f.err }

func TestClient_Close_PropagatesChildErrors(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	wantErr := errors.New("dead-letter producer close failed")
	c.children = append(c.children, fakeCloser{err: wantErr})

	if err := c.Close(); !errors.Is(err, wantErr) {
		t.Errorf("Close() error = %v, want it to wrap %v", err, wantErr)
	}
}

func TestClient_Consumer_WithDeadLetterTopic_ClosesDLQProducer(t *testing.T) {
	c, err := New(Config{Brokers: []string{"localhost:9092"}})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	cons, err := c.Consumer("orders", "billing", WithDeadLetterTopic("orders.dlq"))
	if err != nil {
		t.Fatalf("Consumer() error = %v", err)
	}
	if cons.dlq == nil {
		t.Fatal("Consumer with WithDeadLetterTopic has a nil dlq producer")
	}
	if cons.dlq.topic != "orders.dlq" {
		t.Errorf("dlq producer topic = %q, want %q", cons.dlq.topic, "orders.dlq")
	}
}
