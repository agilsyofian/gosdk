package server

import (
	"context"

	"github.com/agilsyofian/gosdk/example/app/config"
	authHandler "github.com/agilsyofian/gosdk/example/modules/auth/handler/http"
	userHandler "github.com/agilsyofian/gosdk/example/modules/user/handler/http"
	"github.com/agilsyofian/gosdk/observability/health"
	"github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/http"
	ginsecurity "github.com/agilsyofian/gosdk/transport/server/http/security"
)

type IHttpHandler interface {
	SetupRouter()
}

type HttpHandler struct {
	appConfig   config.AppConfig
	server      *http.Server
	health      health.Registry
	security    security.SecurityManager
	authHandler authHandler.IAuthHandler
	userHandler userHandler.IUserHandler
}

func NewHttpHandler(deps HttpHandlerDeps) IHttpHandler {
	return &HttpHandler{
		appConfig:   deps.AppConfig,
		server:      deps.Server,
		health:      deps.Health,
		security:    deps.Security,
		authHandler: deps.AuthHandler,
		userHandler: deps.UserHandler,
	}
}

func StartHttp(h IHttpHandler) {
	h.SetupRouter()
}

func (h *HttpHandler) SetupRouter() {

	// Register some custom health checks
	h.health.Register("example_check", &exampleHealthCheck{})

	r := h.server.Router().Group("/example")

	jwtSecurity := ginsecurity.New(h.security, ginsecurity.Config{
		Required:     true,
		EnableBearer: true,
	})

	r.GET("/auth/certs", h.authHandler.Certificates)

	v1 := r.Group("/v1")
	auth := v1.Group("/auth")
	{
		auth.POST("/password",
			h.authHandler.Password,
		)
		auth.POST("/logout",
			jwtSecurity.Middleware(),
			h.authHandler.Logout,
		)
		auth.POST("/refresh",
			h.authHandler.TokenRefresh,
		)
	}

	user := v1.Group("/user")
	{
		user.GET("/detail",
			jwtSecurity.Middleware(),
			jwtSecurity.RequirePermission("user:read"),
			h.userHandler.UserDetail,
		)
		user.GET("/update",
			jwtSecurity.Middleware(),
			jwtSecurity.RequirePermission("user:update"),
			h.userHandler.Update,
		)
	}

}

type exampleHealthCheck struct{}

func (e *exampleHealthCheck) Check(ctx context.Context) health.Health {
	return health.Health{
		Status: health.StatusUp,
		Details: map[string]any{
			"message": "all systems go",
		},
	}
}
