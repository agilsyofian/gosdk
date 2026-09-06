package server

import (
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
)

type IGrpcHandler interface {
	SetupServer()
}

type GrpcHandler struct {
	server      *grpc.Server
	userHandler user.ServiceUserServer
}

func NewGrpcHandler(deps GrpcHandlerDeps) IGrpcHandler {
	// Phase 5: Wire transport-agnostic health checks into standard gRPC health protocol (grpc.health.v1)
	if deps.Health != nil {
		deps.Server.SetHealthRegistry(deps.Health)
	}

	// Phase 6: Auto-wire security manager with declarative config from env.yaml (server.grpc.security)
	if deps.Security != nil {
		deps.Server.SetSecurityManager(deps.Security)
	}

	return &GrpcHandler{
		server:      deps.Server,
		userHandler: deps.UserHandler,
	}
}

func StartGrpc(h IGrpcHandler) {
	h.SetupServer()
}

func (g GrpcHandler) SetupServer() {
	user.RegisterServiceUserServer(g.server, g.userHandler)
}
