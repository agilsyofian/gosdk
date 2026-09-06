package server

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/example/app/config"
	authHandler "github.com/agilsyofian/gosdk/example/modules/auth/handler/http"
	userHandler "github.com/agilsyofian/gosdk/example/modules/user/handler/http"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"go.uber.org/fx"
)

type HttpHandlerDeps struct {
	fx.In

	AppConfig   config.AppConfig
	Server      *http.Server
	Health      health.Registry
	Security    security.SecurityManager
	Cache       cache.Cache
	AuthHandler authHandler.IAuthHandler
	UserHandler userHandler.IUserHandler
}
