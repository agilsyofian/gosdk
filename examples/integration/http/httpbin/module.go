package httpbin

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	clientHttp "github.com/agilsyofian/gosdk/transport/client/http"
	"go.uber.org/fx"
)

var Integration = fx.Module("integration-httpbin",
	fx.Provide(
		func(cfg config.AppConfig) clientHttp.Config {
			return cfg.HttpIntegrations["httpbin"]
		},
		fx.Annotate(
			NewClientHttpBin,
		),
	),
)
