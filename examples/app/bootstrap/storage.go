package bootstrap

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/storage/database"
	"github.com/agilsyofian/gosdk/storage/postgres"
	"github.com/agilsyofian/gosdk/storage/redis"
	"go.uber.org/fx"
)

var Storage = fx.Module("storage",
	fx.Provide(
		func(cfg config.AppConfig) redis.Config {
			return cfg.Storage.Cache
		},
		func(lc fx.Lifecycle, l logger.Logger, cfg redis.Config, hr health.Registry) (cache.Cache, error) {
			return redis.NewRedisCache(lc, l, cfg, hr)
		},

		func(cfg config.AppConfig) postgres.Config { return cfg.Storage.Postgres },
		func(lc fx.Lifecycle, l logger.Logger, cfg postgres.Config, hr health.Registry) (database.Database, error) {
			return postgres.NewPostgresDB(lc, l, cfg, hr)
		},
	),
)
