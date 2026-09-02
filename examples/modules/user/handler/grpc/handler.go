package grpc

import (
	"context"

	"github.com/agilsyofian/gosdk/example/modules/user/service"
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/observability/logger"
)

type UserHandler struct {
	user.UnimplementedServiceUserServer
	logger  logger.Logger
	service service.IUserService
}

func NewHandler(logger logger.Logger, service service.IUserService) user.ServiceUserServer {
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
