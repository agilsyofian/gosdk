package grpcserver

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	"go.uber.org/fx"
	"user-service/app/config"
	"user-service/proto/user"
)

type GrpcHandlerDeps struct {
	fx.In

	AppConfig   config.AppConfig
	Server      *grpc.Server
	Health      health.Registry          `optional:"true"`
	Security    security.SecurityManager `optional:"true"`
	Services    []GRPCServiceRegistrar   `group:"grpc_services"`
	UserHandler user.ServiceUserServer   `optional:"true"`
}
