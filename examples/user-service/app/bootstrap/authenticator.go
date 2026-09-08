package bootstrap

import (
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"go.uber.org/fx"
	"user-service/app/config"
)

var AuthJWT = fx.Module("authJWT",
	fx.Provide(
		func(cfg config.AppConfig) jwt.Config {
			return cfg.Security.Authenticators.Jwt
		},
		fx.Annotate(
			jwt.New,
			fx.As(new(security.Authenticator)),
		),
	),
)
