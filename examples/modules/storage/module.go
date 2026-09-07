package storage

import (
	serverHttp "github.com/agilsyofian/gosdk/example/app/server/httpserver"
	"github.com/agilsyofian/gosdk/example/modules/storage/handler/httphandler"
	"github.com/agilsyofian/gosdk/example/modules/storage/service"
	"go.uber.org/fx"
)

var Module = fx.Module("module-storage",
	fx.Provide(
		service.New,
		httphandler.New,
		serverHttp.AsRouteRegistrar(httphandler.NewRouteRegistrar),
	),
)
