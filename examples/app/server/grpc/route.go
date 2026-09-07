package grpc

import (
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	"go.uber.org/fx"
)

// GRPCServiceRegistrar is implemented by any module that exposes a gRPC service.
// Modules register their gRPC server implementation directly onto *grpc.Server.
type GRPCServiceRegistrar interface {
	RegisterService(s *grpc.Server)
}

// AsGRPCServiceRegistrar annotates a constructor or instance as a GRPCServiceRegistrar in the "grpc_services" fx group.
func AsGRPCServiceRegistrar(target any) any {
	return fx.Annotate(
		target,
		fx.As(new(GRPCServiceRegistrar)),
		fx.ResultTags(`group:"grpc_services"`),
	)
}
