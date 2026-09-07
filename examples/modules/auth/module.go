package auth

import (
	"github.com/agilsyofian/gosdk/example/app/server/httpserver"
	"github.com/agilsyofian/gosdk/example/modules/auth/handler/httphandler"
	"github.com/agilsyofian/gosdk/example/modules/auth/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-auth",
	fx.Provide(
		service.New,
		httphandler.New,
		httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
