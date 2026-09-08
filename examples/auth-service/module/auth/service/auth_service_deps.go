package service

import (
	"auth-service/app/config"
	"auth-service/module/auth/port"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/security/authenticator/jwt"
	"github.com/agilsyofian/gosdk/security/jwks"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/utils/crypto"
	"go.uber.org/fx"
)

type AuthServiceDeps struct {
	fx.In

	AppConfig      config.AppConfig
	Logger         logger.Logger
	Cache          cache.Cache
	JwtConfig      jwt.Config
	Jwks           jwks.Jwks
	Session        security.SessionManager
	UserFinder     port.IUserFinder
	Publisher      port.IUserEventPublisher
	FieldDecryptor *crypto.FieldDecryptor
}
