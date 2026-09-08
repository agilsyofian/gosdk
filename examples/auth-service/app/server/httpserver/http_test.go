package httpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"auth-service/app/config"
	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	serverHttp "github.com/agilsyofian/gosdk/transport/server/http"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type dummyRouteRegistrar struct {
	called bool
}

func (d *dummyRouteRegistrar) RegisterRoutes(s *serverHttp.Server) {
	d.called = true
	s.Router().GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
}

func TestHttpHandler_SetupRouter_InvokesRouteRegistrars(t *testing.T) {
	gin.SetMode(gin.TestMode)
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := serverHttp.Config{
		BasePath:        "/example",
		Addr:            ":8080",
		Mode:            "test",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
	}
	hr := health.NewRegistry()
	srv := serverHttp.NewServer("test-app", l, cfg, nil, hr)

	registrar := &dummyRouteRegistrar{}
	h := NewHttpHandler(HttpHandlerDeps{
		AppConfig: config.AppConfig{},
		Server:    srv,
		Health:    hr,
		RouteRegistrars: []RouteRegistrar{
			registrar,
		},
	})

	h.SetupRouter()

	assert.True(t, registrar.called, "expected RouteRegistrar to be invoked during SetupRouter")

	// Perform a request to ensure route was registered onto gin engine
	req := httptest.NewRequest("GET", "/example/ping", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "pong", w.Body.String())

	// Verify health check was registered
	res := hr.CheckReadiness(context.Background())
	assert.Equal(t, health.StatusUp, res.Status)
	assert.Contains(t, res.Details, "example_check")
}
