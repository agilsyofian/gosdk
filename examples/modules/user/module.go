package user

import (
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
		http.NewHandler,
	),
)
