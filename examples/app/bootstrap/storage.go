package bootstrap

import (
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
		fx.Annotate(
			redis.NewRedisCache,
			fx.As(new(cache.Cache)),
		),

		func(cfg config.AppConfig) postgres.Config { return cfg.Storage.Postgres },
		fx.Annotate(
			postgres.NewPostgresDB,
			fx.As(new(database.Database)),
		),
	),
)
