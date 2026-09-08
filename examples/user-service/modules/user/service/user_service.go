package service

import (
	"context"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security"
	"user-service/app/config"
	"user-service/modules/user/domain"
	"user-service/modules/user/repository"
)

type IUserService interface {
	GetUser(ctx context.Context, username, deviceId string) (domain.User, error)
	Detail(ctx context.Context) (domain.User, error)
	Update(ctx context.Context) (res domain.User, err error)
}

type UserService struct {
	appConfig config.AppConfig
	logger    logger.Logger
	repoUser  repository.IUserRepository
}

func New(deps UserServiceDeps) IUserService {
	return &UserService{
		appConfig: deps.AppConfig,
		logger:    deps.Logger,
		repoUser:  deps.RepoUser,
	}
}

func (s UserService) GetUser(ctx context.Context, username, deviceId string) (domain.User, error) {
	res, err := s.repoUser.GetUser(ctx, username, deviceId)
	if err != nil {
		return domain.User{}, err
	}
	return *res, err
}

func (s UserService) Detail(ctx context.Context) (domain.User, error) {
	secContext, err := security.MustFromContext(ctx)
	if err != nil {
		return domain.User{}, errors.Unauthorized(err, s.appConfig.AppName, "4001", "no active session")
	}

	return domain.UserFromPrincipal(secContext.Session.Principal)
}

func (s UserService) Update(ctx context.Context) (res domain.User, err error) {
	_, err = security.MustFromContext(ctx)
	if err != nil {
		return domain.User{}, errors.Unauthorized(err, s.appConfig.AppName, "4001", "no active session")
	}
	return res, err
}
