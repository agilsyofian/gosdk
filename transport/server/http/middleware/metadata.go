package middleware

import (
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/agilsyofian/gosdk/core/metadata"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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
// If client IP or tracing IDs are missing, it falls back to gin.Context.ClientIP()
// and auto-generated IDs.
func Metadata(cfgs ...metadata.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		m := metadata.Extract(headerCarrier(c.Request.Header), cfgs...)

		if m.ClientIP == "" {
			m.ClientIP = c.ClientIP()
		}
		if idx := strings.Index(m.ClientIP, ","); idx != -1 {
			m.ClientIP = strings.TrimSpace(m.ClientIP[:idx])
		}

		ensureTraceAndSpanIDs(m)

		c.Request = c.Request.WithContext(metadata.WithContext(c.Request.Context(), m))
		c.Next()
	}
}

func ensureTraceAndSpanIDs(m *metadata.Metadata) {
	if m == nil {
		return
	}
	if m.TraceID == "" {
		u := uuid.New()
		m.TraceID = hex.EncodeToString(u[:])
	}
	if m.SpanID == "" {
		u := uuid.New()
		m.SpanID = hex.EncodeToString(u[:8])
	}
}
