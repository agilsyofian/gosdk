package service

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"go.uber.org/fx"
)

type StorageServiceDeps struct {
	fx.In

	AppConfig config.AppConfig
	Logger    logger.Logger
	Storage   objectstorage.Storage `optional:"true"`
}
