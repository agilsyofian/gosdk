package consumer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/agilsyofian/gosdk/messaging/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

// IConsumer starts the example's Kafka consumers and stops them again on
// shutdown.
type IConsumer interface {
	Start()
	Stop(ctx context.Context) error
}

type subscription struct {
	consumer *kafka.Consumer
	handler  kafka.Handler
}

type Consumer struct {
	appName       string
	subscriptions []subscription

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewConsumer(deps DependenciesConsumer) (IConsumer, error) {
	var subs []subscription

	for _, sub := range deps.Subscribers {
		topic := sub.Topic()
		if topic == "" {
			continue
		}

		c, err := createConsumer(deps, sub)
		if err != nil {
			// Clean up any previously created consumers before failing
			for _, prev := range subs {
				_ = prev.consumer.Close()
			}
			return nil, fmt.Errorf("consumer: create consumer for topic %q: %w", topic, err)
		}

		subs = append(subs, subscription{
			consumer: c,
			handler:  sub.Handler(),
		})
	}

	return &Consumer{
		appName:       deps.AppConfig.AppName,
		subscriptions: subs,
	}, nil
}

// Start launches one background goroutine per subscribed topic.
func (s *Consumer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	for _, sub := range s.subscriptions {
		s.consume(ctx, sub.consumer, sub.handler)
	}
}

func (s *Consumer) consume(ctx context.Context, c *kafka.Consumer, handler kafka.Handler) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := c.Consume(ctx, handler); err != nil {
			// Context cancellation or fetch error handled gracefully
		}
	}()
}

// Stop cancels all consume loops and waits for their goroutines to
// return before closing the underlying readers.
func (s *Consumer) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()

	var errs []error
	for _, sub := range s.subscriptions {
		if err := sub.consumer.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func createConsumer(deps DependenciesConsumer, sub TopicSubscriber) (*kafka.Consumer, error) {
	if deps.Client == nil {
		return nil, fmt.Errorf("kafka client is nil")
	}

	topic := sub.Topic()
	groupID := sub.GroupID()
	if groupID == "" {
		return nil, fmt.Errorf("group_id is required for topic %q", topic)
	}

	startOffset := kafkago.FirstOffset
	if sub.AutoOffsetReset() != "earliest" {
		startOffset = kafkago.LastOffset
	}

	concurrency := sub.Concurrency()
	if concurrency <= 0 {
		concurrency = 1
	}

	return deps.Client.Consumer(
		topic,
		groupID,
		kafka.WithConcurrency(concurrency),
		kafka.WithStartOffset(startOffset),
	)
}
