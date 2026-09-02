//go:build integration

// These integration tests exercise messaging/kafka against a real Kafka
// broker. They are excluded from the default `go test ./...` run
// (messaging.md sections 32-33 — unit tests must not require a broker):
// run them explicitly with
//
//	KAFKA_BROKERS=localhost:9092 go test -tags=integration ./...
//
// They skip automatically when KAFKA_BROKERS is unset, so a normal CI or
// local run without a broker configured is unaffected.
package kafka

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	kafkago "github.com/segmentio/kafka-go"
)

func integrationBrokers(t *testing.T) []string {
	t.Helper()
	v := os.Getenv("KAFKA_BROKERS")
	if v == "" {
		t.Skip("KAFKA_BROKERS not set; skipping integration test")
	}
	return strings.Split(v, ",")
}

func ensureTopic(t *testing.T, brokers []string, topic string) {
	t.Helper()
	conn, err := kafkago.Dial("tcp", brokers[0])
	if err != nil {
		t.Logf("warning: could not dial broker for topic creation: %v", err)
		return
	}
	defer conn.Close()

	_ = conn.CreateTopics(kafkago.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
}

func TestIntegration_PublishAndConsume_RoundTrip(t *testing.T) {
	brokers := integrationBrokers(t)

	// A fresh topic/group per run avoids interference from previous runs.
	// Ensure topic exists (Kafka's auto-create is unreliable with kafka-go).
	topic := "messaging-sdk-integration-" + newMessageID()
	groupID := "messaging-sdk-integration-" + newMessageID()
	ensureTopic(t, brokers, topic)

	client, err := New(Config{Brokers: brokers})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer client.Close()

	producer, err := client.Producer(topic)
	if err != nil {
		t.Fatalf("Producer() error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	want := []byte("integration-test-payload")
	if err := producer.Publish(ctx, &Message{Payload: want}); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	consumer, err := client.Consumer(topic, groupID)
	if err != nil {
		t.Fatalf("Consumer() error = %v", err)
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
