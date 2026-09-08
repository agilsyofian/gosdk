package consumer

import (
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"go.uber.org/fx"
	"user-service/app/config"
)

type DependenciesConsumer struct {
	fx.In

	AppConfig   config.AppConfig
	Client      *kafka.Client
	Subscribers []TopicSubscriber `group:"kafka_subscribers"`
}
