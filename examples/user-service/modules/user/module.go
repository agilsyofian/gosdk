package user

import (
	"go.uber.org/fx"
	"user-service/app/server/grpcserver"
	"user-service/app/server/httpserver"
	"user-service/modules/user/handler/grpchandler"
	"user-service/modules/user/handler/httphandler"
	"user-service/modules/user/repository"
	"user-service/modules/user/service"
)

var Module = fx.Module("module-user",
	fx.Provide(
		repository.New,
		service.New,

		grpchandler.New,
		grpcserver.AsGRPCServiceRegistrar(grpchandler.NewServiceRegistrar),

		httphandler.New,
		httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
