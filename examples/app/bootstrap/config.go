package bootstrap

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	"go.uber.org/fx"
)

var Config = fx.Module("config",
	fx.Provide(
		config.LoadConfig,
	),
)
