package grpc

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	"go.uber.org/fx"
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
