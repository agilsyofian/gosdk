package http

import (
	"github.com/agilsyofian/gosdk/transport/server/http"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
)

// RouteRegistrar is implemented by any module that exposes HTTP endpoints.
// Modules register routes directly onto the provided *http.Server instance.
type RouteRegistrar interface {
	RegisterRoutes(s *http.Server)
}

// AsRouteRegistrar annotates a constructor or instance as a RouteRegistrar in the "routes" fx group.
func AsRouteRegistrar(target any) any {
	return fx.Annotate(
		target,
		fx.As(new(RouteRegistrar)),
		fx.ResultTags(`group:"routes"`),
	)
}

// CreateSignatureMiddleware builds an HMAC signature verification middleware from SignatureConfig.
func CreateSignatureMiddleware(cfg http.SignatureConfig) gin.HandlerFunc {
	secret := []byte(cfg.Secret)
	if len(secret) == 0 {
		secret = []byte("example-secret-key-12345")
	}

	opts := []http.SignatureOption{
		http.WithSignatureResponseSigning(cfg.SignResponse),
	}
	if cfg.MaxClockDrift > 0 {
		opts = append(opts, http.WithSignatureMaxClockDrift(cfg.MaxClockDrift))
	}

	return http.Signature(http.StaticSecret(secret), opts...)
}
