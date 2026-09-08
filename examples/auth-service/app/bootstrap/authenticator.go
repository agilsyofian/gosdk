package bootstrap

import (
	"auth-service/app/config"

	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/security/jwks"
	"go.uber.org/fx"
)

var AuthJWT = fx.Module("authJWT",
	fx.Provide(
		func(cfg config.AppConfig) jwt.Config {
			return cfg.Security.Authenticators.Jwt
		},
		func(cfg config.AppConfig) jwks.Jwks {
			return cfg.Security.JWKS
		},
		fx.Annotate(
			jwt.New,
			fx.As(new(security.Authenticator)),
		),
	),
)
