package consumer

import (
	"context"
	"errors"
	"sync"

	"github.com/agilsyofian/gosdk/example/modules/notifications/handler/messaging"
	"github.com/agilsyofian/gosdk/messaging/kafka"
	kafkago "github.com/segmentio/kafka-go"
)

// IConsumer starts the example's Kafka consumers and stops them again on
// shutdown.
//
// Start returns immediately; each topic is consumed on its own
// background goroutine. That matters because kafka.Consumer.Consume
// blocks until its context is cancelled or it hits a fetch error, so
// running two Consume calls one after another on a single goroutine
// (as this file used to) would let the first call starve the second
// forever.
//
// Stop cancels those goroutines and waits for them to return before the
// underlying readers are closed, so shutdown never interrupts a message
// mid-handling (see .instructions/messaging.md, "graceful shutdown").
type IConsumer interface {
	Start()
	Stop(ctx context.Context) error
}

type Consumer struct {
	appName           string
	userEvent         *kafka.Consumer
	notificationEvent *kafka.Consumer
	handler           messaging.IMessagingHandler

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func NewConsumer(deps DependenciesConsumer) IConsumer {

	userEvent := registerTopic(
		deps,
		deps.AppConfig.Messaging.Topics["user_event"],
	)

	notificationEvent := registerTopic(
		deps,
		deps.AppConfig.Messaging.Topics["notification_event"],
	)

	return &Consumer{
		appName:           deps.AppConfig.AppName,
		userEvent:         userEvent,
		notificationEvent: notificationEvent,
		handler:           deps.Handler,
	}
}

// Start launches one goroutine per topic so both consumers make
// progress concurrently, then returns without blocking the caller (see
// module.go, where this runs from an fx.OnStart hook).
func (s *Consumer) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	// Consume user_event topic with user event handler
	s.consume(ctx, s.userEvent, s.handler.EventUser)
	// Consume notification_event topic with notification event handler
	s.consume(ctx, s.notificationEvent, s.handler.EventNotification)
}

func (s *Consumer) consume(ctx context.Context, c *kafka.Consumer, handler kafka.Handler) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := c.Consume(ctx, handler); err != nil {
			// Log error but continue gracefully
		}
	}()
}

// Stop cancels both consume loops and waits for their goroutines to
// return before closing the underlying readers, so a message that is
// already being handled gets to finish instead of being cut off
// mid-commit.
func (s *Consumer) Stop(ctx context.Context) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()

	return errors.Join(
		s.userEvent.Close(),
		s.notificationEvent.Close(),
	)
}

func registerTopic(deps DependenciesConsumer, topic string) *kafka.Consumer {

	startOffset := kafkago.FirstOffset
	if deps.AppConfig.Messaging.Consumer.AutoOffsetReset != "earliest" {
		startOffset = kafkago.LastOffset
	}

	c, err := deps.Client.Consumer(
		topic,
		deps.AppConfig.Messaging.Consumer.GroupID,
		kafka.WithConcurrency(deps.AppConfig.Messaging.Consumer.Concurrency),
		kafka.WithStartOffset(startOffset),
	)
	if err != nil {
		panic(err)
	}
	return c
}
