package http

import (
	"encoding/json"
	"time"

	serverHttp "github.com/agilsyofian/gosdk/example/app/server/http"
	"github.com/agilsyofian/gosdk/storage/cache"
	"github.com/agilsyofian/gosdk/transport/server/http"
	"github.com/gin-gonic/gin"
)

type AuthRouteRegistrar struct {
	handler IAuthHandler
	cache   cache.Cache
}

func NewRouteRegistrar(handler IAuthHandler, c cache.Cache) serverHttp.RouteRegistrar {
	return &AuthRouteRegistrar{
		handler: handler,
		cache:   c,
	}
}

func (r *AuthRouteRegistrar) RegisterRoutes(s *http.Server) {
	engine := s.Router()

	engine.GET("/auth/certs", r.handler.Certificates)

	v1 := engine.Group("/v1/auth")
	{
		v1.POST("/password",
			http.Idempotency(r.cache,
				// Custom canonical key: prioritaskan header X-Idempotency-Key jika ada,
				// atau kombinasikan dengan username dari request body,
				// atau fallback ke DefaultCanonicalKey.
				http.WithCanonicalKeyFunc(func(c *gin.Context, body []byte) (string, error) {
					if key := c.GetHeader("X-Idempotency-Key"); key != "" {
						return "auth:password:" + key, nil
					}

					var req struct {
						Username string `json:"username"`
					}
					if err := json.Unmarshal(body, &req); err == nil && req.Username != "" {
						return "auth:password:" + req.Username, nil
					}

					return http.DefaultCanonicalKey(c, body)
				}),
				http.WithIdempotencyTTL(1*time.Minute),
			),
			r.handler.Password,
		)
		v1.POST("/logout",
			s.SecurityMiddleware(),
			r.handler.Logout,
		)
		v1.POST("/refresh",
			r.handler.TokenRefresh,
		)
	}
}
