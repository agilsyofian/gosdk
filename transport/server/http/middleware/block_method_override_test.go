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

func setupTestRouter(appName string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(
		middleware.Error(logger.Nop(), appName, true),
		middleware.BlockMethodOverride(appName),
	)
	r.POST("/resource", func(c *gin.Context) {
		c.String(http.StatusOK, "resource created")
	})
	r.GET("/resource", func(c *gin.Context) {
		c.String(http.StatusOK, "resource fetched")
	})
	return r
}

func TestBlockMethodOverride_Allowed(t *testing.T) {
	r := setupTestRouter("test-app")

	req := httptest.NewRequest(http.MethodPost, "/resource", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "resource created", w.Body.String())
}

func TestBlockMethodOverride_BlockedHeaders(t *testing.T) {
	headersToTest := []string{
		"X-HTTP-Method",
		"X-HTTP-Method-Override",
		"X-Method-Override",
		"x-http-method-override", // case insensitivity check
	}

	for _, header := range headersToTest {
		t.Run(header, func(t *testing.T) {
			r := setupTestRouter("test-app")

			req := httptest.NewRequest(http.MethodPost, "/resource", nil)
			req.Header.Set(header, "DELETE")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, http.StatusForbidden, w.Code)

			var resp dto.BaseResponse
			err := json.Unmarshal(w.Body.Bytes(), &resp)
			assert.NoError(t, err)
			assert.Equal(t, "test-app", resp.Source)
			assert.Equal(t, "403", resp.Code)
			assert.Equal(t, "HTTP method override is not allowed", resp.Message)
		})
	}
}

func TestBlockMethodOverride_BlockedQueryParam(t *testing.T) {
	r := setupTestRouter("test-app")

	req := httptest.NewRequest(http.MethodGet, "/resource?_method=DELETE", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)

	var resp dto.BaseResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	assert.NoError(t, err)
	assert.Equal(t, "test-app", resp.Source)
	assert.Equal(t, "403", resp.Code)
	assert.Equal(t, "HTTP method override is not allowed", resp.Message)
}
