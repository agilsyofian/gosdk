package middleware

import (
	"net/http"

	"github.com/agilsyofian/gosdk/core/metadata"
	"github.com/gin-gonic/gin"
)

type headerCarrier http.Header

func (h headerCarrier) Get(key string) string {
	return http.Header(h).Get(key)
}

func (h headerCarrier) Keys() []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	return keys
}

// Metadata extracts incoming HTTP request headers into the standardized
// core/metadata container attached to the request context.
func Metadata(cfgs ...metadata.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		m := metadata.Extract(headerCarrier(c.Request.Header), cfgs...)
		c.Request = c.Request.WithContext(metadata.WithContext(c.Request.Context(), m))
		c.Next()
	}
}
