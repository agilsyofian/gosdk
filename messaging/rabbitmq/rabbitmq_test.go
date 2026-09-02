package rabbitmq

import (
	"errors"
	"testing"

	amqp "github.com/rabbitmq/amqp091-go"
)

// fakeConnection is the mock transport seam for Client tests (messaging.md
// section 32). Channel() intentionally errors: a real *amqp.Channel can
// only come from a live connection, so Producer/Consumer construction
// success is exercised by integration tests instead — see doc.go.
type fakeConnection struct {
	closeErr error
	closed   bool
}

func (f *fakeConnection) Channel() (*amqp.Channel, error) {
	return nil, errors.New("fakeConnection: Channel requires a real broker; see integration tests")
}

func (f *fakeConnection) Close() error {
	f.closed = true
	return f.closeErr
}

func TestNew_ValidatesConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  Config
	}{
		{"empty URL", Config{}},
		{"blank URL", Config{URL: "   "}},
		{"wrong scheme", Config{URL: "http://localhost:5672"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := New(tt.cfg); !errors.Is(err, ErrInvalidConfig) {
				t.Errorf("New(%+v) error = %v, want ErrInvalidConfig", tt.cfg, err)
			}
		})
	}
}

func TestClient_Consumer_RequiresQueue(t *testing.T) {
	c := &Client{conn: &fakeConnection{}, opts: defaultClientOptions()}
	if _, err := c.Consumer(""); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf(`Consumer("") error = %v, want ErrInvalidConfig`, err)
	}
}

func TestClient_Consumer_ValidatesConcurrencyAndPrefetch(t *testing.T) {
	c := &Client{conn: &fakeConnection{}, opts: defaultClientOptions()}
	if _, err := c.Consumer("orders.created", WithConcurrency(0)); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Consumer with WithConcurrency(0) error = %v, want ErrInvalidConfig", err)
	}
	if _, err := c.Consumer("orders.created", WithPrefetch(0)); !errors.Is(err, ErrInvalidConfig) {
		t.Errorf("Consumer with WithPrefetch(0) error = %v, want ErrInvalidConfig", err)
	}
}

func TestClient_ProducerAndConsumer_FailAfterClose(t *testing.T) {
	c := &Client{conn: &fakeConnection{}, opts: defaultClientOptions()}
	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if _, err := c.Producer("orders"); !errors.Is(err, ErrClosed) {
		t.Errorf("Producer() after Close() error = %v, want ErrClosed", err)
	}
	if _, err := c.Consumer("orders.created"); !errors.Is(err, ErrClosed) {
		t.Errorf("Consumer() after Close() error = %v, want ErrClosed", err)
	}
}

func TestClient_Close_Idempotent(t *testing.T) {
	fc := &fakeConnection{}
	c := &Client{conn: fc, opts: defaultClientOptions()}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil", err)
	}
	if !fc.closed {
		t.Error("underlying connection was not closed")
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}
}

func TestClient_Close_IgnoresAlreadyClosedConnectionError(t *testing.T) {
	fc := &fakeConnection{closeErr: amqp.ErrClosed}
	c := &Client{conn: fc, opts: defaultClientOptions()}

	if err := c.Close(); err != nil {
		t.Fatalf("Close() error = %v, want nil (amqp.ErrClosed should be tolerated)", err)
	}
}

func TestClient_Close_PropagatesConnectionError(t *testing.T) {
	wantErr := errors.New("tcp: connection reset")
	fc := &fakeConnection{closeErr: wantErr}
	c := &Client{conn: fc, opts: defaultClientOptions()}

	if err := c.Close(); !errors.Is(err, wantErr) {
		t.Errorf("Close() error = %v, want it to wrap %v", err, wantErr)
	}
}

// fakeChildCloser lets the "failure during shutdown" scenario be tested:
// Client.Close must join every child's error rather than stopping at the
// first one or swallowing it.
type fakeChildCloser struct{ err error }

func (f fakeChildCloser) Close() error { return f.err }

func TestClient_Close_PropagatesChildErrors(t *testing.T) {
	c := &Client{conn: &fakeConnection{}, opts: defaultClientOptions()}
	wantErr := errors.New("producer channel close failed")
	c.children = append(c.children, fakeChildCloser{err: wantErr})

	if err := c.Close(); !errors.Is(err, wantErr) {
		t.Errorf("Close() error = %v, want it to wrap %v", err, wantErr)
	}
}
