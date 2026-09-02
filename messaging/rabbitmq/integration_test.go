//go:build integration

// These integration tests exercise messaging/rabbitmq against a real
// RabbitMQ broker. They are excluded from the default `go test ./...`
// run (messaging.md sections 32-33 — unit tests must not require a
// broker): run them explicitly with
//
//	RABBITMQ_URL=amqp://guest:guest@localhost:5672/ go test -tags=integration ./...
//
// They skip automatically when RABBITMQ_URL is unset, so a normal CI or
// local run without a broker configured is unaffected.
package rabbitmq

import (
	"context"
	"os"
	"testing"
	"time"
)

func integrationURL(t *testing.T) string {
	t.Helper()
	v := os.Getenv("RABBITMQ_URL")
	if v == "" {
		t.Skip("RABBITMQ_URL not set; skipping integration test")
	}
	return v
}

func TestIntegration_PublishAndConsume_RoundTrip(t *testing.T) {
	url := integrationURL(t)
	queue := "messaging-sdk-integration-" + newMessageID()

	client, err := New(Config{URL: url})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	// The consumer declares the queue before anything is published, since
	// publishing through the default exchange requires the target queue
	// to already exist. autoDelete=true cleans it up when the channel
	// closes at the end of this test.
	consumer, err := client.Consumer(queue, WithQueueDeclare(false, true, false, nil))
	if err != nil {
		t.Fatalf("Consumer() error = %v", err)
	}

	// The default (nameless) exchange routes directly to the queue named
	// by RoutingKey.
	producer, err := client.Producer("")
	if err != nil {
		t.Fatalf("Producer() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	want := []byte("integration-test-payload")
	if err := producer.Publish(ctx, &Message{RoutingKey: queue, Payload: want}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	received := make(chan []byte, 1)
	consumeCtx, cancelConsume := context.WithCancel(ctx)
	defer cancelConsume()

	go func() {
		_ = consumer.Consume(consumeCtx, func(ctx context.Context, msg *Message) error {
			received <- msg.Payload
			cancelConsume()
			return nil
		})
	}()

	select {
	case got := <-received:
		if string(got) != string(want) {
			t.Errorf("received payload = %q, want %q", got, want)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for the published message to be consumed")
	}
}
