package storage

import (
	serverHttp "github.com/agilsyofian/gosdk/example/app/server/http"
	"github.com/agilsyofian/gosdk/example/modules/storage/handler/http"
	"go.uber.org/fx"
)

var Module = fx.Module("module-storage",
	fx.Provide(
		http.NewHandler,
		serverHttp.AsRouteRegistrar(http.NewRouteRegistrar),
	),
)
