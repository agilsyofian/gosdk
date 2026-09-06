package http

import (
	securityCore "github.com/agilsyofian/gosdk/security"
	httpsecurity "github.com/agilsyofian/gosdk/transport/server/http/security"
	"github.com/gin-gonic/gin"
)

// Security returns a Gin middleware for authentication bound to the given SecurityManager.
func Security(manager securityCore.SecurityManager, cfg ...httpsecurity.Config) gin.HandlerFunc {
	var c httpsecurity.Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	adapter := httpsecurity.New(manager, c)
	return adapter.Middleware()
}

// RequirePermission returns a Gin middleware that enforces a permission check via SecurityManager.Authorize.
// It must run after Security() or Server.SecurityMiddleware() in the handler chain.
func RequirePermission(manager securityCore.SecurityManager, permission string) gin.HandlerFunc {
	adapter := httpsecurity.New(manager, httpsecurity.Config{})
	return adapter.RequirePermission(permission)
}
