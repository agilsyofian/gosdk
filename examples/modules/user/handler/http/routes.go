package http

import (
	"github.com/agilsyofian/gosdk/example/app/config"
	serverHttp "github.com/agilsyofian/gosdk/example/app/server/http"
	"github.com/agilsyofian/gosdk/transport/server/http"
)

type UserRouteRegistrar struct {
	handler IUserHandler
	appCfg  config.AppConfig
}

func NewRouteRegistrar(handler IUserHandler, appCfg config.AppConfig) serverHttp.RouteRegistrar {
	return &UserRouteRegistrar{
		handler: handler,
		appCfg:  appCfg,
	}
}

func (r *UserRouteRegistrar) RegisterRoutes(s *http.Server) {
	v1 := s.Router().Group("/v1/user")
	sigMiddleware := serverHttp.CreateSignatureMiddleware(r.appCfg.Server.HttpServer.Signature)

	v1.GET("/detail",
		s.SecurityMiddleware(),
		s.RequirePermission("user:read"),
		r.handler.UserDetail,
	)
	v1.GET("/update",
		sigMiddleware,
		s.SecurityMiddleware(),
		s.RequirePermission("user:update"),
		r.handler.Update,
	)
}
