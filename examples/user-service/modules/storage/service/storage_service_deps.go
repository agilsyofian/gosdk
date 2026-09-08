package service

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"go.uber.org/fx"
	"user-service/app/config"
)

type StorageServiceDeps struct {
	fx.In

	AppConfig config.AppConfig
	Logger    logger.Logger
	Storage   objectstorage.Storage `optional:"true"`
}
