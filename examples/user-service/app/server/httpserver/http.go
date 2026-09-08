package httpserver

import (
	"context"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"user-service/app/config"
)

type IHttpHandler interface {
	SetupRouter()
}

type Handler struct {
	appConfig       config.AppConfig
	server          *http.Server
	health          health.Registry
	security        security.SecurityManager
	routeRegistrars []RouteRegistrar
}

func NewHttpHandler(deps HttpHandlerDeps) IHttpHandler {
	return &Handler{
		appConfig:       deps.AppConfig,
		server:          deps.Server,
		health:          deps.Health,
		security:        deps.Security,
		routeRegistrars: deps.RouteRegistrars,
	}
}

func StartHttp(h IHttpHandler) {
	h.SetupRouter()
}

func (h *Handler) SetupRouter() {
	// Register custom health checks
	if h.health != nil {
		h.health.Register("example_check", &exampleHealthCheck{})
	}

	// Wire security manager using declarative config from env.yaml (server.http.security)
	if h.security != nil {
		h.server.SetSecurityManager(h.security)
	}

	// Register decentralized routes provided by each module
	for _, registrar := range h.routeRegistrars {
		registrar.RegisterRoutes(h.server)
	}
}

type exampleHealthCheck struct{}

func (e *exampleHealthCheck) Check(ctx context.Context) health.Health {
	return health.Up()
}
