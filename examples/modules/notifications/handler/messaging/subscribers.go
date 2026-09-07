package messaging

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/app/messaging/consumer"
	"github.com/agilsyofian/gosdk/messaging/kafka"
)

type userEventSubscriber struct {
	topic   string
	handler IMessagingHandler
}

func NewUserEventSubscriber(cfg config.AppConfig, handler IMessagingHandler) consumer.TopicSubscriber {
	return &userEventSubscriber{
		topic:   cfg.Messaging.Topics["user_event"],
		handler: handler,
	}
}

func (s *userEventSubscriber) Topic() string {
	return s.topic
}

func (s *userEventSubscriber) Handler() kafka.Handler {
	return s.handler.EventUser
}

type notificationEventSubscriber struct {
	topic   string
	handler IMessagingHandler
}

func NewNotificationEventSubscriber(cfg config.AppConfig, handler IMessagingHandler) consumer.TopicSubscriber {
	return &notificationEventSubscriber{
		topic:   cfg.Messaging.Topics["notification_event"],
		handler: handler,
	}
}

func (s *notificationEventSubscriber) Topic() string {
	return s.topic
}

func (s *notificationEventSubscriber) Handler() kafka.Handler {
	return s.handler.EventNotification
}
