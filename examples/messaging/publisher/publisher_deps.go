package publisher

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"go.uber.org/fx"
)

type DependenciesPublisher struct {
	fx.In

	Logger    logger.Logger
	AppConfig config.AppConfig
	Client    *kafka.Client
}
