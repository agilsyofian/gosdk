package grpchandler

import (
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	serverGrpc "user-service/app/server/grpcserver"
	"user-service/proto/user"
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
