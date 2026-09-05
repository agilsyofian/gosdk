package bootstrap

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/messaging/kafka"
	"github.com/agilsyofian/gosdk/observability/logger"
	"go.uber.org/fx"
)

var Messaging = fx.Module("messaging",
	fx.Provide(
		func(cfg config.AppConfig) kafka.Config {
			return kafka.Config{
				Brokers: cfg.Messaging.Brokers,
			}
		},
		fx.Annotate(
			func(appName string, logger logger.Logger, hr health.Registry) []kafka.Option {
				res := []kafka.Option{
					kafka.WithAppName(appName),
					kafka.WithLogger(logger),
					kafka.WithHealthRegistry(hr),
				}
				return res
			},
			fx.ParamTags(`name:"appName"`, ``, ``),
			fx.ResultTags(`name:"kafkaClientOptions"`),
		),
		fx.Annotate(
			kafka.New,
			fx.ParamTags(``, `name:"kafkaClientOptions"`),
			fx.OnStop(func(ctx context.Context, c *kafka.Client) error {
				return c.Close()
			}),
		),
	),
)
