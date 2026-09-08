package httpserver

import (
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"go.uber.org/fx"
	"user-service/app/config"
)

type HttpHandlerDeps struct {
	fx.In

	AppConfig       config.AppConfig
	Server          *http.Server
	Health          health.Registry          `optional:"true"`
	Security        security.SecurityManager `optional:"true"`
	RouteRegistrars []RouteRegistrar         `group:"routes"`
}
