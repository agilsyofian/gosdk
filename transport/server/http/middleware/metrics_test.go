package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/metrics"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMetricsMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	m := metrics.New(metrics.Config{
		Enabled:   true,
		Namespace: "test",
		Subsystem: "http",
	})

	router := gin.New()
	router.Use(Metrics(m, "/actuator/prometheus"))

	router.GET("/users/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "user "+c.Param("id"))
	})
	router.GET("/actuator/prometheus", func(c *gin.Context) {
		c.String(http.StatusOK, "metrics")
	})

	// 1. Request to /users/123
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users/123", nil)
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// 2. Request to excluded route
	wExcluded := httptest.NewRecorder()
	reqExcluded := httptest.NewRequest(http.MethodGet, "/actuator/prometheus", nil)
	router.ServeHTTP(wExcluded, reqExcluded)
	assert.Equal(t, http.StatusOK, wExcluded.Code)

	// 3. Scrape Prometheus handler directly
	wMetrics := httptest.NewRecorder()
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	m.HTTPHandler().ServeHTTP(wMetrics, reqMetrics)

	output := wMetrics.Body.String()
	assert.Contains(t, output, `test_http_http_requests_total{method="GET",path="/users/:id",status="200"} 1`)
	assert.NotContains(t, output, `/users/123`) // normalized to :id
}
