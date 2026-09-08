package bootstrap

import (
	"github.com/agilsyofian/gosdk/security/session/redis"
	"go.uber.org/fx"
)

var Session = fx.Module("session",
	fx.Provide(
		redis.New,
	),
)
