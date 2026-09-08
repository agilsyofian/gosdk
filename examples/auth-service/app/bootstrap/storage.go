package bootstrap

import (
	"auth-service/app/config"
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/storage/cache/redis"
	"go.uber.org/fx"
)

var Storage = fx.Module("storage",
	fx.Provide(
		func(cfg config.AppConfig) redis.Config {
			return cfg.Storage.Cache
		},
		func(lc fx.Lifecycle, l logger.Logger, cfg redis.Config, hr health.Registry) (cache.Cache, error) {
			c, err := redis.NewRedisCache(l, cfg, hr)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					return c.Connect(ctx)
				},
				OnStop: func(ctx context.Context) error {
					return c.Close(ctx)
				},
			})
			return c, nil
		},
	),
)
