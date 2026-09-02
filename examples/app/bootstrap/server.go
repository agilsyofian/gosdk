package bootstrap

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"go.uber.org/fx"
)

var Server = fx.Module("server",
	fx.Provide(

		// HTTP server
		func(cfg config.AppConfig) http.Config {
			return cfg.Server.HttpServer
		},
		fx.Annotate(
			http.NewServer,
			fx.ParamTags(`name:"appName"`),
		),

		// GRPC server
		func(cfg config.AppConfig) grpc.Config {
			return cfg.Server.GrpcServer
		},
		fx.Annotate(
			grpc.NewServer,
			fx.ParamTags(`name:"appName"`),
		),
	),
)
