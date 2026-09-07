package auth

import (
	"github.com/agilsyofian/gosdk/example/app/server/http"
	serverHttp "github.com/agilsyofian/gosdk/example/modules/auth/handler/http"
	"github.com/agilsyofian/gosdk/example/modules/auth/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-auth",
	fx.Provide(
		service.NewService,
		serverHttp.NewHandler,
		http.AsRouteRegistrar(serverHttp.NewRouteRegistrar),
	),
)
