package server

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
	Health      health.Registry
	Security    security.SecurityManager
	UserHandler user.ServiceUserServer
}
