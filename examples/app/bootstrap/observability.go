package bootstrap

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/example/app/config"
	"go.uber.org/fx"
)

var Observability = fx.Module("observability",
	fx.Provide(

		// App FullName
		fx.Annotate(
			func(cfg config.AppConfig) string {
				return cfg.AppName
			},
			fx.ResultTags(`name:"appName"`),
		),

		// App Version
		fx.Annotate(
			func(cfg config.AppConfig) string {
				return cfg.AppVersion
			},
			fx.ResultTags(`name:"appVersion"`),
		),

		// Logger
		func(cfg config.AppConfig) logger.Config {
			return cfg.Observability.Logger
		},
		logger.NewZerologLogger,

		// Tracing
		func(cfg config.AppConfig) tracing.Config {
			return cfg.Observability.Tracer
		},
		fx.Annotate(
			tracing.NewTracing,
			fx.ParamTags(``, `name:"appName"`, `name:"appVersion"`),
		),

		// Health Check
		health.NewRegistry,
	),
)
