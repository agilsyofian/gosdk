package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/observability/logger"
	httpServer "github.com/agilsyofian/gosdk/transport/server/http"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/fx/fxtest"
)

func TestServer_Actuator_HealthProbes(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()
	reg := health.NewRegistry()

	cfg := httpServer.Config{
		Addr:            ":0",
		ReadTimeout:     5 * time.Second,
		WriteTimeout:    5 * time.Second,
		IdleTimeout:     5 * time.Second,
		ShutdownTimeout: 5 * time.Second,
		Mode:            "test",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
			InfoPath:   "/info",
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, reg)

	// 1. Initial State: All UP
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/actuator/health", nil)
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	wLive := httptest.NewRecorder()
	reqLive, _ := http.NewRequest(http.MethodGet, "/actuator/liveness", nil)
	s.ServeHTTP(wLive, reqLive)
	assert.Equal(t, http.StatusOK, wLive.Code)

	wReady := httptest.NewRecorder()
	reqReady, _ := http.NewRequest(http.MethodGet, "/actuator/readiness", nil)
	s.ServeHTTP(wReady, reqReady)
	assert.Equal(t, http.StatusOK, wReady.Code)

	// 2. Register a Failing Readiness Check (e.g. database down)
	reg.RegisterReadinessFunc("database", func(ctx context.Context) health.Health {
		return health.DownWithDetails(map[string]any{"error": "connection failed"})
	})

	// Liveness must STILL BE 200 OK (Kubernetes should not restart container)
	wLive2 := httptest.NewRecorder()
	s.ServeHTTP(wLive2, reqLive)
	assert.Equal(t, http.StatusOK, wLive2.Code)

	// Readiness must BE 503 Service Unavailable (Kubernetes will remove from service endpoints)
	wReady2 := httptest.NewRecorder()
	s.ServeHTTP(wReady2, reqReady)
	assert.Equal(t, http.StatusServiceUnavailable, wReady2.Code)

	var readyResp health.Health
	err := json.Unmarshal(wReady2.Body.Bytes(), &readyResp)
	assert.NoError(t, err)
	assert.Equal(t, health.StatusDown, readyResp.Status)
	assert.Contains(t, readyResp.Details, "database")

	// Overall health must BE 503
	wHealth2 := httptest.NewRecorder()
	s.ServeHTTP(wHealth2, req)
	assert.Equal(t, http.StatusServiceUnavailable, wHealth2.Code)

	// Info endpoint must be 200 OK
	wInfo := httptest.NewRecorder()
	reqInfo, _ := http.NewRequest(http.MethodGet, "/actuator/info", nil)
	s.ServeHTTP(wInfo, reqInfo)
	assert.Equal(t, http.StatusOK, wInfo.Code)
}

func TestServer_Actuator_NilHealthRegistryFallback(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr: ":0",
		Mode: "test",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
		},
	}

	// hr is passed as nil
	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	assert.NotNil(t, s.HealthRegistry())

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/actuator/health", nil)
	s.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestServer_BasePath(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	cfg := httpServer.Config{
		Addr:     ":0",
		Mode:     "test",
		BasePath: "/api/v1",
		Actuator: httpServer.ActuatorConfig{
			Enabled:    true,
			Path:       "/actuator",
			HealthPath: "/health",
		},
	}

	s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
	assert.Equal(t, "/api/v1", s.BasePath())
	assert.NotNil(t, s.BaseRouter())

	// 1. Direct handler on s (embedded gin.IRouter)
	s.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// 2. Group under Router() / BaseRouter()
	users := s.Router().Group("/users")
	users.GET("/list", func(c *gin.Context) {
		c.String(http.StatusOK, "user-list")
	})

	// 3. Raw route on root Engine (bypassing BasePath)
	s.Engine().GET("/raw", func(c *gin.Context) {
		c.String(http.StatusOK, "raw-response")
	})

	// Test GET /api/v1/ping -> 200 OK
	wPing := httptest.NewRecorder()
	reqPing, _ := http.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	s.ServeHTTP(wPing, reqPing)
	assert.Equal(t, http.StatusOK, wPing.Code)
	assert.Equal(t, "pong", wPing.Body.String())

	// Test GET /ping (without base path) -> 404 Not Found
	wPingRoot := httptest.NewRecorder()
	reqPingRoot, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	s.ServeHTTP(wPingRoot, reqPingRoot)
	assert.Equal(t, http.StatusNotFound, wPingRoot.Code)

	// Test GET /api/v1/users/list -> 200 OK
	wUsers := httptest.NewRecorder()
	reqUsers, _ := http.NewRequest(http.MethodGet, "/api/v1/users/list", nil)
	s.ServeHTTP(wUsers, reqUsers)
	assert.Equal(t, http.StatusOK, wUsers.Code)
	assert.Equal(t, "user-list", wUsers.Body.String())

	// Test GET /raw (outside base path) -> 200 OK
	wRaw := httptest.NewRecorder()
	reqRaw, _ := http.NewRequest(http.MethodGet, "/raw", nil)
	s.ServeHTTP(wRaw, reqRaw)
	assert.Equal(t, http.StatusOK, wRaw.Code)
	assert.Equal(t, "raw-response", wRaw.Body.String())

	// Test Actuator under BasePath -> 200 OK
	wAct := httptest.NewRecorder()
	reqAct, _ := http.NewRequest(http.MethodGet, "/api/v1/actuator/health", nil)
	s.ServeHTTP(wAct, reqAct)
	assert.Equal(t, http.StatusOK, wAct.Code)
}

func TestServer_BasePath_Normalization(t *testing.T) {
	lc := fxtest.NewLifecycle(t)
	l := logger.Nop()

	tests := []struct {
		input    string
		expected string
	}{
		{input: "", expected: ""},
		{input: "/", expected: ""},
		{input: "   ", expected: ""},
		{input: "api/v1", expected: "/api/v1"},
		{input: "/api/v1", expected: "/api/v1"},
		{input: "/api/v1/", expected: "/api/v1"},
		{input: "///api/v1///", expected: "/api/v1"},
		{input: "/api/v2/service/", expected: "/api/v2/service"},
	}

	for _, tt := range tests {
		cfg := httpServer.Config{
			Addr:     ":0",
			Mode:     "test",
			BasePath: tt.input,
		}
		s := httpServer.NewServer("test-app", lc, l, cfg, nil, nil)
		assert.Equal(t, tt.expected, s.BasePath(), "for input: %s", tt.input)
	}
}
