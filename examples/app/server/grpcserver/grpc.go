package grpcserver

import (
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
)

type IGrpcHandler interface {
	SetupServer()
}

type Handler struct {
	server      *grpc.Server
	services    []GRPCServiceRegistrar
	userHandler user.ServiceUserServer
}

func NewGrpcHandler(deps GrpcHandlerDeps) IGrpcHandler {
	// Wire transport-agnostic health checks into standard gRPC health protocol (grpc.health.v1)
	if deps.Health != nil {
		deps.Server.SetHealthRegistry(deps.Health)
	}

	// Auto-wire security manager with declarative config from env.yaml (server.grpc.security)
	if deps.Security != nil {
		deps.Server.SetSecurityManager(deps.Security)
	}

	return &Handler{
		server:      deps.Server,
		services:    deps.Services,
		userHandler: deps.UserHandler,
	}
}

func StartGrpc(h IGrpcHandler) {
	h.SetupServer()
}

func (g Handler) SetupServer() {
	for _, svc := range g.services {
		svc.RegisterService(g.server)
	}

	if g.userHandler != nil && len(g.services) == 0 {
		user.RegisterServiceUserServer(g.server, g.userHandler)
	}
}
