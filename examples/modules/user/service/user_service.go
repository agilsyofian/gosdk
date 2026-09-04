package service

import (
	"context"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/modules/user/model"
	"github.com/agilsyofian/gosdk/example/modules/user/repository"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/security"
)

type IUserService interface {
	GetUser(ctx context.Context, username, deviceId string) (model.User, error)
	Detail(ctx context.Context) (model.User, error)
	Update(ctx context.Context) (res model.User, err error)
}

type UserService struct {
	appConfig config.AppConfig
	logger    logger.Logger
	repoUser  repository.IUserRepository
}

func NewUserService(deps UserServiceDeps) IUserService {
	return &UserService{
		appConfig: deps.AppConfig,
		logger:    deps.Logger,
		repoUser:  deps.RepoUser,
	}
}

func (s UserService) GetUser(ctx context.Context, username, deviceId string) (model.User, error) {
	res, err := s.repoUser.GetUser(ctx, username, deviceId)
	if err != nil {
		return model.User{}, err
	}
	return *res, err
}

func (s UserService) Detail(ctx context.Context) (model.User, error) {
	secContext, err := security.MustFromContext(ctx)
	if err != nil {
		return model.User{}, errors.Unauthorized(err, s.appConfig.AppName, "4001", "no active session")
	}

	return model.UserFromPrincipal(secContext.Session.Principal)
}

func (s UserService) Update(ctx context.Context) (res model.User, err error) {
	_, err = security.MustFromContext(ctx)
	if err != nil {
		return model.User{}, errors.Unauthorized(err, s.appConfig.AppName, "4001", "no active session")
	}
	return res, err
}
