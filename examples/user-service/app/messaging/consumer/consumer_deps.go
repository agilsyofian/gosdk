package consumer

import (
	"user-service/app/config"

	"github.com/agilsyofian/gosdk/messaging/kafka"
	"go.uber.org/fx"
)

type DependenciesConsumer struct {
	fx.In

	AppConfig   config.AppConfig
	Client      *kafka.Client
	Subscribers []TopicSubscriber `group:"kafka_subscribers"`
}
