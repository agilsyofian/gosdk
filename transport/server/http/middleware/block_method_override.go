package middleware

import (
	"errors"
	"net/http"

	appError "github.com/agilsyofian/gosdk/core/errors"
	"github.com/gin-gonic/gin"
)

var overrideHeaders = []string{
	"X-HTTP-Method",
	"X-HTTP-Method-Override",
	"X-Method-Override",
}

// BlockMethodOverride returns a Gin middleware that rejects requests containing
// HTTP method override headers or query parameters with HTTP 403 Forbidden.
// This prevents HTTP Verb Tampering attacks.
func BlockMethodOverride(appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if hasMethodOverride(c) {
			appErr := appError.Forbidden(
				errors.New("HTTP method override is not allowed"),
				appName,
				"403",
				"HTTP method override is not allowed",
			)
			_ = c.Error(appErr)
			c.Status(http.StatusForbidden)
			c.Abort()
			return
		}
		c.Next()
	}
}

func hasMethodOverride(c *gin.Context) bool {
	for _, header := range overrideHeaders {
		if c.GetHeader(header) != "" {
			return true
		}
	}
	return c.Query("_method") != ""
}
