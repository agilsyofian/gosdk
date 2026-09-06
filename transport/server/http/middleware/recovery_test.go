package middleware_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

type mockLogger struct {
	mu           sync.Mutex
	errorCalls   []string
	lastPayloads []logger.LogPayload
}

func (m *mockLogger) Debug(context.Context, string, logger.LogPayload) {}
func (m *mockLogger) Info(context.Context, string, logger.LogPayload)  {}
func (m *mockLogger) Warn(context.Context, string, logger.LogPayload)  {}
func (m *mockLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.errorCalls = append(m.errorCalls, msg)
	m.lastPayloads = append(m.lastPayloads, payload)
}

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	appName := "test-app"
	mockLog := &mockLogger{}

	router := gin.New()
	router.Use(
		middleware.Error(mockLog, appName, true),
		middleware.Recovery(mockLog, appName),
	)

	router.GET("/panic", func(c *gin.Context) {
		panic("something went wrong unexpectedly")
	})

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp dto.BaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, appName, resp.Source)
	assert.Equal(t, "An error occurred", resp.Message)
	assert.Contains(t, resp.Error, "something went wrong unexpectedly")

	// Verify structured logger received panic details with stack trace
	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()

	assert.Len(t, mockLog.errorCalls, 1, "expected exactly 1 error log call without duplicates")
	assert.True(t, strings.Contains(mockLog.errorCalls[0], "/panic"))

	payload := mockLog.lastPayloads[0]
	errMap, ok := payload.Error.(map[string]any)
	assert.True(t, ok, "expected payload.Error to be map[string]any")
	assert.Equal(t, "something went wrong unexpectedly", errMap["panic"])
	assert.NotEmpty(t, errMap["file"])
	assert.NotEmpty(t, errMap["stack"])
}

func TestRecoveryMiddleware_ErrorType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	appName := "test-app"
	mockLog := &mockLogger{}

	router := gin.New()
	router.Use(
		middleware.Error(mockLog, appName, true),
		middleware.Recovery(mockLog, appName),
	)

	router.GET("/panic-err", func(c *gin.Context) {
		panic(assert.AnError)
	})

	req := httptest.NewRequest(http.MethodGet, "/panic-err", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp dto.BaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, appName, resp.Source)
	assert.Equal(t, "An error occurred", resp.Message)
	assert.Contains(t, resp.Error, assert.AnError.Error())

	mockLog.mu.Lock()
	defer mockLog.mu.Unlock()
	assert.Len(t, mockLog.errorCalls, 1)
}
