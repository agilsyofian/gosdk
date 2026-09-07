package user_event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/auth/service"
	"github.com/agilsyofian/gosdk/messaging/kafka"
)

type UserEventPublisher struct {
	producer *kafka.Producer
}

func NewUserEventPublisher(client *kafka.Client, cfg config.AppConfig) (service.IUserEventPublisher, error) {
	if client == nil {
		return nil, fmt.Errorf("integration user_event: kafka client is nil")
	}

	topic := cfg.Messaging.Topics["user_event"]
	if topic == "" {
		topic = "user.event"
	}

	prod, err := client.Producer(
		topic,
		kafka.WithRequiredAcks(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("integration user_event: create producer: %w", err)
	}

	return &UserEventPublisher{producer: prod}, nil
}

func (p *UserEventPublisher) PublishUserEvent(ctx context.Context, event service.UserEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("integration user_event: marshal event: %w", err)
	}

	return p.producer.Publish(ctx, &kafka.Message{
		Key:         []byte(event.EventID),
		Payload:     payload,
		ContentType: "application/json",
	})
}
