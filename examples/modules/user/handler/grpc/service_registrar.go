package grpc

import (
	serverGrpc "github.com/agilsyofian/gosdk/example/app/server/grpc"
	"github.com/agilsyofian/gosdk/example/proto/user"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
)

type UserServiceRegistrar struct {
	handler user.ServiceUserServer
}

func NewServiceRegistrar(h user.ServiceUserServer) serverGrpc.GRPCServiceRegistrar {
	return &UserServiceRegistrar{
		handler: h,
	}
}

func (r *UserServiceRegistrar) RegisterService(s *grpc.Server) {
	user.RegisterServiceUserServer(s, r.handler)
}
