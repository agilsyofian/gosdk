package httpserver

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/metrics"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"go.uber.org/fx"
	"user-service/app/config"
)

var HTTP = fx.Module("server:http",
	fx.Provide(
		func(cfg config.AppConfig) http.Config {
			return cfg.Server.HttpServer
		},
		fx.Annotate(
			func(appName string, lc fx.Lifecycle, l logger.Logger, cfg http.Config, t *tracing.Tracing, hr health.Registry, m *metrics.Metrics) *http.Server {
				s := http.NewServer(appName, l, cfg, t, hr, m)
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							_ = s.Start()
						}()
						return nil
					},
					OnStop: func(ctx context.Context) error {
						return s.Shutdown(ctx)
					},
				})
				return s
			},
			fx.ParamTags(`name:"appName"`),
		),
		NewHttpHandler,
	),
	fx.Invoke(StartHttp),
)
