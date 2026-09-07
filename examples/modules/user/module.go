package user

import (
	serverGrpc "github.com/agilsyofian/gosdk/example/app/server/grpc"
	serverHttp "github.com/agilsyofian/gosdk/example/app/server/http"
	"github.com/agilsyofian/gosdk/example/modules/user/handler/grpc"
	"github.com/agilsyofian/gosdk/example/modules/user/handler/http"
	"github.com/agilsyofian/gosdk/example/modules/user/repository"
	"github.com/agilsyofian/gosdk/example/modules/user/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-user",
	fx.Provide(
		repository.NewUserRepository,
		service.NewUserService,
		grpc.NewHandler,
		serverGrpc.AsGRPCServiceRegistrar(grpc.NewServiceRegistrar),
		http.NewHandler,
		serverHttp.AsRouteRegistrar(http.NewRouteRegistrar),
	),
)
