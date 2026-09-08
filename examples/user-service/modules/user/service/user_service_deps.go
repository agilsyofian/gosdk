package service

import (
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"go.uber.org/fx"
	"user-service/app/config"
	"user-service/modules/user/repository"
)

type UserServiceDeps struct {
	fx.In

	AppConfig config.AppConfig
	Logger    logger.Logger
	RepoUser  repository.IUserRepository
}
