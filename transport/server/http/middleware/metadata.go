package middleware

import (
	requestmetadata "github.com/agilsyofian/gosdk/transport/server/http/metadata"
	"github.com/gin-gonic/gin"
)

// Metadata copies the supported incoming request headers into the standard
// request context, where handlers and services can retrieve them.
func Metadata() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := requestmetadata.WithContext(
			c.Request.Context(),
			requestmetadata.FromHeader(c.Request.Header),
		)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}
