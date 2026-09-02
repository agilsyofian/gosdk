package publisher

import (
	"context"

	"github.com/agilsyofian/gosdk/errors"
	"github.com/agilsyofian/gosdk/messaging/kafka"
)

type IPublisher interface {
	UserEvent(ctx context.Context, value UserEvent) error
}

type Publisher struct {
	appName           string
	userEvent         *kafka.Producer
	notificationEvent *kafka.Producer
}

func NewPublisher(deps DependenciesPublisher) IPublisher {

	useEvent, err := deps.Client.Producer(
		deps.AppConfig.Messaging.Topics["user_event"],
		kafka.WithRequiredAcks(-1),
	)
	if err != nil {
		panic(err)
	}

	notificationEvent, err := deps.Client.Producer(
		deps.AppConfig.Messaging.Topics["notification_event"],
		kafka.WithRequiredAcks(-1),
	)
	if err != nil {
		panic(err)
	}

	return &Publisher{
		appName:           deps.AppConfig.AppName,
		userEvent:         useEvent,
		notificationEvent: notificationEvent,
	}
}

func (p Publisher) UserEvent(ctx context.Context, value UserEvent) error {
	payload, err := value.Marshal()
	if err != nil {
		return errors.InternalServerError(err, p.appName, errors.StatusInternalServerError.String(), "an error occurred")
	}
	return p.userEvent.Publish(ctx, &kafka.Message{
		Key:         []byte(value.EventID),
		Payload:     payload,
		ContentType: "application/json",
	})
}

func (p Publisher) NotificationEvent(ctx context.Context, value NotificationEvent) error {
	payload, err := value.Marshal()
	if err != nil {
		return errors.InternalServerError(err, p.appName, errors.StatusInternalServerError.String(), "an error occurred")
	}
	return p.notificationEvent.Publish(ctx, &kafka.Message{
		Key:         []byte(value.EventID),
		Payload:     payload,
		ContentType: "application/json",
	})
}
