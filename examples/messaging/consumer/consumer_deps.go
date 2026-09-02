package consumer

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/notifications/handler/messaging"
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"go.uber.org/fx"
)

type DependenciesConsumer struct {
	fx.In

	AppConfig config.AppConfig
	Client    *kafka.Client
	Handler   messaging.IMessagingHandler
}
