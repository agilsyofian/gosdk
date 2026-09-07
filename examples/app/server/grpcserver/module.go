package grpcserver

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/transport/server/grpc"
	"go.uber.org/fx"
)

var GRPC = fx.Module("server:grpc",
	fx.Provide(
		func(cfg config.AppConfig) grpc.Config {
			return cfg.Server.GrpcServer
		},
		fx.Annotate(
			func(appName string, lc fx.Lifecycle, l logger.Logger, cfg grpc.Config, t *tracing.Tracing) *grpc.Server {
				s := grpc.NewServer(appName, l, cfg, t)
				lc.Append(fx.Hook{
					OnStart: func(ctx context.Context) error {
						go func() {
							_ = s.Start()
						}()
						return nil
					},
					OnStop: func(ctx context.Context) error {
						if t != nil {
							_ = t.Shutdown(ctx)
						}
						return s.Shutdown(ctx)
					},
				})
				return s
			},
			fx.ParamTags(`name:"appName"`),
		),
		NewGrpcHandler,
	),
	fx.Invoke(StartGrpc),
)
