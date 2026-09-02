package service

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/user/repository"
	"github.com/agilsyofian/gosdk/observability/logger"
	"go.uber.org/fx"
)

type UserServiceDeps struct {
	fx.In

	AppConfig config.AppConfig
	Logger    logger.Logger
	RepoUser  repository.IUserRepository
}
