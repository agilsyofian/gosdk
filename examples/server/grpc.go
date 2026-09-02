package server

import (
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	grpcsecurity "github.com/agilsyofian/gosdk/transport/server/grpc/security"
)

type IGrpcHandler interface {
	SetupServer()
}

type GrpcHandler struct {
	server      *grpc.Server
	userHandler user.ServiceUserServer
}

func NewGrpcHandler(deps GrpcHandlerDeps) IGrpcHandler {

	interceptor := grpcsecurity.New(deps.Security, grpcsecurity.Config{
		Required:     true,
		EnableBearer: true,
	})

	deps.Server.Use(interceptor.UnaryServerInterceptor())

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
