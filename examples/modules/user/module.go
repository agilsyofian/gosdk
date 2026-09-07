package user

import (
	"github.com/agilsyofian/gosdk/example/app/server/grpcserver"
	"github.com/agilsyofian/gosdk/example/app/server/httpserver"
	"github.com/agilsyofian/gosdk/example/modules/user/handler/grpchandler"
	"github.com/agilsyofian/gosdk/example/modules/user/handler/httphandler"
	"github.com/agilsyofian/gosdk/example/modules/user/repository"
	"github.com/agilsyofian/gosdk/example/modules/user/service"
	"go.uber.org/fx"
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
