package grpchandler

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"user-service/modules/user/service"
	"user-service/proto/user"
)

type UserHandler struct {
	user.UnimplementedServiceUserServer
	logger  logger.Logger
	service service.IUserService
}

func New(logger logger.Logger, service service.IUserService) user.ServiceUserServer {
	return &UserHandler{
		logger:  logger,
		service: service,
	}
}

func (s *UserHandler) GetUser(ctx context.Context, req *user.Request) (*user.Response, error) {
	res, err := s.service.GetUser(ctx, req.Username, req.DeviceId)
	if err != nil {
		return nil, err
	}
	return UserToResponse(res), nil
}
