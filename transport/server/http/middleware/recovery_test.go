package middleware_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/http/dto"
	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRecoveryMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	appName := "test-app"
	logInstance := logger.Nop()

	router := gin.New()
	router.Use(
		middleware.Error(logInstance, appName, true),
		middleware.Recovery(appName),
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
}

func TestRecoveryMiddleware_ErrorType(t *testing.T) {
	gin.SetMode(gin.TestMode)

	appName := "test-app"
	logInstance := logger.Nop()

	router := gin.New()
	router.Use(
		middleware.Error(logInstance, appName, true),
		middleware.Recovery(appName),
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
}
