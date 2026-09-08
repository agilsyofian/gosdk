package bootstrap

import (
	"context"
	"fmt"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/storage/cache/redis"
	"github.com/agilsyofian/gosdk/storage/database"
	"github.com/agilsyofian/gosdk/storage/database/postgres"
	"github.com/agilsyofian/gosdk/storage/objectstorage"
	"github.com/agilsyofian/gosdk/storage/objectstorage/s3"
	"go.uber.org/fx"
	"user-service/app/config"
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

		func(cfg config.AppConfig) postgres.Config { return cfg.Storage.Postgres },
		func(lc fx.Lifecycle, l logger.Logger, cfg postgres.Config, hr health.Registry) (database.Database, error) {
			db, err := postgres.NewPostgresDB(l, cfg, hr)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					if err := db.Connect(ctx); err != nil {
						l.Error(ctx, fmt.Sprintf("failed to connect database: %s", err.Error()), logger.LogPayload{
							Type: "DATABASE",
						})
						return err
					}
					l.Info(ctx, "database connection established", logger.LogPayload{
						Type:   "DATABASE",
						Method: "START",
					})
					return nil
				},
				OnStop: func(ctx context.Context) error {
					l.Info(ctx, "stopping database", logger.LogPayload{
						Type:   "DATABASE",
						Method: "SHUTDOWN",
					})
					return db.Close()
				},
			})
			return db, nil
		},

		// Object Storage (S3 / MinIO)
		func(cfg config.AppConfig) s3.Config {
			return cfg.Storage.ObjectStorage
		},
		func(lc fx.Lifecycle, l logger.Logger, cfg s3.Config, hr health.Registry) (objectstorage.Storage, error) {
			store, err := s3.NewS3Storage(l, cfg, hr)
			if err != nil {
				return nil, err
			}
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return store.Close(ctx)
				},
			})
			return store, nil
		},
	),
)
