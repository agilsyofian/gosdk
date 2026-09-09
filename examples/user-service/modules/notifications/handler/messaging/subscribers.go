package messaging

import (
	"user-service/app/config"
	"user-service/app/messaging/consumer"

	"github.com/agilsyofian/gosdk/messaging/kafka"
)

type userEventSubscriber struct {
	cfg     config.ConsumerConfig
	handler IMessagingHandler
}

func NewUserEventSubscriber(cfg config.AppConfig, handler IMessagingHandler) consumer.TopicSubscriber {
	return &userEventSubscriber{
		cfg:     cfg.Messaging.Consumer["user_event"],
		handler: handler,
	}
}

func (s *userEventSubscriber) Topic() string {
	return s.cfg.Topic
}

func (s *userEventSubscriber) GroupID() string {
	return s.cfg.GroupID
}

func (s *userEventSubscriber) Concurrency() int {
	return s.cfg.Concurrency
}

func (s *userEventSubscriber) AutoOffsetReset() string {
	return s.cfg.AutoOffsetReset
}

func (s *userEventSubscriber) Handler() kafka.Handler {
	return s.handler.EventUser
}

type notificationEventSubscriber struct {
	cfg     config.ConsumerConfig
	handler IMessagingHandler
}

func NewNotificationEventSubscriber(cfg config.AppConfig, handler IMessagingHandler) consumer.TopicSubscriber {
	return &notificationEventSubscriber{
		cfg:     cfg.Messaging.Consumer["notification_event"],
		handler: handler,
	}
}

func (s *notificationEventSubscriber) Topic() string {
	return s.cfg.Topic
}

func (s *notificationEventSubscriber) GroupID() string {
	return s.cfg.GroupID
}

func (s *notificationEventSubscriber) Concurrency() int {
	return s.cfg.Concurrency
}

func (s *notificationEventSubscriber) AutoOffsetReset() string {
	return s.cfg.AutoOffsetReset
}

func (s *notificationEventSubscriber) Handler() kafka.Handler {
	return s.handler.EventNotification
}
