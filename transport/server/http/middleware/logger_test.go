package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type spyLogger struct {
	mu      sync.Mutex
	entries []logEntry
}

type logEntry struct {
	level   string
	msg     string
	payload logger.LogPayload
}

func (s *spyLogger) Debug(ctx context.Context, msg string, p logger.LogPayload) {
	s.record("debug", msg, p)
}
func (s *spyLogger) Info(ctx context.Context, msg string, p logger.LogPayload) {
	s.record("info", msg, p)
}
func (s *spyLogger) Warn(ctx context.Context, msg string, p logger.LogPayload) {
	s.record("warn", msg, p)
}
func (s *spyLogger) Error(ctx context.Context, msg string, p logger.LogPayload) {
	s.record("error", msg, p)
}
func (s *spyLogger) record(level, msg string, p logger.LogPayload) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, logEntry{level: level, msg: msg, payload: p})
}
func (s *spyLogger) getEntries() []logEntry {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]logEntry, len(s.entries))
	copy(copied, s.entries)
	return copied
}

func TestLogger_Exclusion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	spy := &spyLogger{}
	router := gin.New()
	router.Use(middleware.Logger(spy, "test-app", "/actuator", "^/internal/.*", "/health"))

	router.GET("/swagger/index.html", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/example/swagger/doc.json", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/actuator/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/actuator/readiness", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/v1/health", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/internal/metrics", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/api/v1/users", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	router.GET("/api/v1/bad", func(c *gin.Context) { c.Status(http.StatusBadRequest) })
	router.GET("/api/v1/err", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	// 1. Excluded routes -> must NOT produce logs
	excluded := []string{
		"/swagger/index.html",
		"/example/swagger/doc.json",
		"/actuator/health",
		"/actuator/readiness",
		"/api/v1/health",
		"/internal/metrics",
	}

	for _, path := range excluded {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest(http.MethodGet, path, nil)
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	assert.Empty(t, spy.getEntries(), "excluded routes should produce 0 log entries")

	// 2. Normal business route -> logged at INFO
	wUsers := httptest.NewRecorder()
	reqUsers, _ := http.NewRequest(http.MethodGet, "/api/v1/users", nil)
	router.ServeHTTP(wUsers, reqUsers)
	assert.Equal(t, http.StatusOK, wUsers.Code)

	entries := spy.getEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, "info", entries[0].level)
	assert.Equal(t, "/api/v1/users", entries[0].payload.Path)
	assert.Equal(t, http.StatusOK, entries[0].payload.Status)

	// 3. Status 400 -> logged at WARN
	wBad := httptest.NewRecorder()
	reqBad, _ := http.NewRequest(http.MethodGet, "/api/v1/bad", nil)
	router.ServeHTTP(wBad, reqBad)
	assert.Equal(t, http.StatusBadRequest, wBad.Code)

	entries = spy.getEntries()
	assert.Len(t, entries, 2)
	assert.Equal(t, "warn", entries[1].level)
	assert.Equal(t, http.StatusBadRequest, entries[1].payload.Status)

	// 4. Status 500 -> logged at ERROR
	wErr := httptest.NewRecorder()
	reqErr, _ := http.NewRequest(http.MethodGet, "/api/v1/err", nil)
	router.ServeHTTP(wErr, reqErr)
	assert.Equal(t, http.StatusInternalServerError, wErr.Code)

	entries = spy.getEntries()
	assert.Len(t, entries, 3)
	assert.Equal(t, "error", entries[2].level)
	assert.Equal(t, http.StatusInternalServerError, entries[2].payload.Status)
}

func TestLogger_EnvExclusion(t *testing.T) {
	gin.SetMode(gin.TestMode)

	_ = os.Setenv("ROUTES_EXCLUDE_LOGGER", "/env-excluded,/env-metrics")
	defer func() { _ = os.Unsetenv("ROUTES_EXCLUDE_LOGGER") }()

	spy := &spyLogger{}
	router := gin.New()
	router.Use(middleware.Logger(spy, "test-app"))

	router.GET("/env-excluded/status", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/env-metrics", func(c *gin.Context) { c.Status(http.StatusOK) })
	router.GET("/allowed", func(c *gin.Context) { c.Status(http.StatusOK) })

	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest(http.MethodGet, "/env-excluded/status", nil)
	router.ServeHTTP(w1, req1)

	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest(http.MethodGet, "/env-metrics", nil)
	router.ServeHTTP(w2, req2)

	assert.Empty(t, spy.getEntries(), "routes from ROUTES_EXCLUDE_LOGGER must not be logged")

	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest(http.MethodGet, "/allowed", nil)
	router.ServeHTTP(w3, req3)

	entries := spy.getEntries()
	assert.Len(t, entries, 1)
	assert.Equal(t, "/allowed", entries[0].payload.Path)
}

func TestRouteMatcher(t *testing.T) {
	rm := middleware.NewRouteMatcher("/actuator", "^/v[0-9]+/metrics$")

	assert.True(t, rm.Matches("/swagger"))
	assert.True(t, rm.Matches("/swagger/index.html"))
	assert.True(t, rm.Matches("/example/swagger/index.html"))
	assert.True(t, rm.Matches("/actuator"))
	assert.True(t, rm.Matches("/actuator/health"))
	assert.True(t, rm.Matches("/example/actuator/health"))
	assert.True(t, rm.Matches("/v1/metrics"))
	assert.True(t, rm.Matches("/v2/metrics"))

	assert.False(t, rm.Matches("/api/v1/users"))
	assert.False(t, rm.Matches("/v1/metrics/extra"))
	assert.False(t, rm.Matches("/shopping"))
}
