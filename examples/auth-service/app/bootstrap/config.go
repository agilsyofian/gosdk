package bootstrap

import (
	"auth-service/app/config"

	"go.uber.org/fx"
)

var Config = fx.Module("config",
	fx.Provide(
		config.LoadConfig,
	),
)
