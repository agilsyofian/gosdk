package auth

import (
	"auth-service/app/server/httpserver"
	"auth-service/module/auth/handler/httphandler"
	"auth-service/module/auth/service"

	"go.uber.org/fx"
)

var Module = fx.Module("module-auth",
	fx.Provide(
		service.New,
		httphandler.New,
		httpserver.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
