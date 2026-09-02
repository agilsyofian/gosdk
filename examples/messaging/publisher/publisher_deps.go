package publisher

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"github.com/agilsyofian/gosdk/observability/logger"
	"go.uber.org/fx"
)

type DependenciesPublisher struct {
	fx.In

	Logger    logger.Logger
	AppConfig config.AppConfig
	Client    *kafka.Client
}
