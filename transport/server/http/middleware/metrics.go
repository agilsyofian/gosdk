package middleware

import (
	"time"

	"github.com/agilsyofian/gosdk/core/observability/metrics"
	"github.com/gin-gonic/gin"
)

// Metrics returns a Gin middleware that records Prometheus RED metrics
// (http_requests_total, http_request_duration_seconds, and http_requests_in_flight).
// Uses c.FullPath() to normalize route paths and prevent high-cardinality label explosion.
func Metrics(m *metrics.Metrics, excludeRoutes ...string) gin.HandlerFunc {
	matcher := NewRouteMatcher(excludeRoutes...)

	return func(c *gin.Context) {
		if m == nil {
			c.Next()
			return
		}

		path := c.Request.URL.Path
		if matcher.Matches(path) {
			c.Next()
			return
		}

		method := c.Request.Method
		m.IncHTTPInFlight(method)
		start := time.Now()

		c.Next()

		m.DecHTTPInFlight(method)

		matchedPath := c.FullPath()
		if matchedPath == "" {
			matchedPath = "unmatched"
		}

		duration := time.Since(start)
		status := c.Writer.Status()

		m.RecordHTTPRequest(method, matchedPath, status, duration)
	}
}
