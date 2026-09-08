package bootstrap

import (
	"go.uber.org/fx"
	"user-service/app/config"
)

var Config = fx.Module("config",
	fx.Provide(
		config.LoadConfig,
	),
)
