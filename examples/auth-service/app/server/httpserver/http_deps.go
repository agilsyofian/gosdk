package httpserver

import (
	"auth-service/app/config"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"go.uber.org/fx"
)

type HttpHandlerDeps struct {
	fx.In

	AppConfig       config.AppConfig
	Server          *http.Server
	Health          health.Registry          `optional:"true"`
	Security        security.SecurityManager `optional:"true"`
	RouteRegistrars []RouteRegistrar         `group:"routes"`
}
