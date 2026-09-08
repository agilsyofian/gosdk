package storage

import (
	"go.uber.org/fx"
	serverHttp "user-service/app/server/httpserver"
	"user-service/modules/storage/handler/httphandler"
	"user-service/modules/storage/service"
)

var Module = fx.Module("module-storage",
	fx.Provide(
		service.New,
		httphandler.New,
		serverHttp.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
