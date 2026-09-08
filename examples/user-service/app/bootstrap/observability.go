package bootstrap

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/metrics"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"go.uber.org/fx"
	"user-service/app/config"
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
			lCfg := cfg.Observability.Logger
			if lCfg.AppName == "" {
				lCfg.AppName = cfg.AppName
			}
			return lCfg
		},
		logger.NewZerologLogger,

		// Tracing
		func(cfg config.AppConfig) tracing.Config {
			return cfg.Observability.Tracer
		},
		fx.Annotate(
			func(lc fx.Lifecycle, cfg tracing.Config, appName, appVersion string) (*tracing.Tracing, error) {
				t, err := tracing.NewTracing(cfg, appName, appVersion)
				if err != nil {
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						return t.Shutdown(ctx)
					},
				})
				return t, nil
			},
			fx.ParamTags(``, ``, `name:"appName"`, `name:"appVersion"`),
		),

		// Health Check
		health.NewRegistry,

		// Metrics
		func(cfg config.AppConfig) metrics.Config {
			mCfg := cfg.Observability.Metrics
			if mCfg.Namespace == "" {
				mCfg.Namespace = cfg.AppName
			}
			return mCfg
		},
		metrics.New,
	),
)
