package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS_DefaultWildcard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(CorsConfig{}))
	r.GET("/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})

	// Preflight OPTIONS request
	reqOpt := httptest.NewRequest(http.MethodOptions, "/ping", nil)
	reqOpt.Header.Set("Origin", "https://client.another-domain.com")
	reqOpt.Header.Set("Access-Control-Request-Method", "GET")
	wOpt := httptest.NewRecorder()
	r.ServeHTTP(wOpt, reqOpt)

	assert.Equal(t, http.StatusNoContent, wOpt.Code)
	assert.Equal(t, "*", wOpt.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, wOpt.Header().Get("Vary"), "Origin")

	// Standard GET request
	reqGet := httptest.NewRequest(http.MethodGet, "/ping", nil)
	reqGet.Header.Set("Origin", "https://client.another-domain.com")
	wGet := httptest.NewRecorder()
	r.ServeHTTP(wGet, reqGet)

	assert.Equal(t, http.StatusOK, wGet.Code)
	assert.Equal(t, "*", wGet.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, wGet.Header().Get("Vary"), "Origin")
}

func TestCORS_SpecificOrigins(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(CorsConfig{
		AllowOrigins:     []string{"https://app.company.com", "https://admin.company.com"},
		AllowMethods:     []string{"GET", "POST"},
		AllowCredentials: true,
		MaxAge:           10 * time.Minute,
	}))
	r.POST("/login", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	// Allowed origin
	reqAllowed := httptest.NewRequest(http.MethodOptions, "/login", nil)
	reqAllowed.Header.Set("Origin", "https://app.company.com")
	reqAllowed.Header.Set("Access-Control-Request-Method", "POST")
	wAllowed := httptest.NewRecorder()
	r.ServeHTTP(wAllowed, reqAllowed)

	assert.Equal(t, http.StatusNoContent, wAllowed.Code)
	assert.Equal(t, "https://app.company.com", wAllowed.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", wAllowed.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, wAllowed.Header().Get("Vary"), "Origin")

	// Disallowed origin
	reqDenied := httptest.NewRequest(http.MethodOptions, "/login", nil)
	reqDenied.Header.Set("Origin", "https://attacker.com")
	reqDenied.Header.Set("Access-Control-Request-Method", "POST")
	wDenied := httptest.NewRecorder()
	r.ServeHTTP(wDenied, reqDenied)

	assert.NotEqual(t, "https://attacker.com", wDenied.Header().Get("Access-Control-Allow-Origin"))
}

func TestCORS_WildcardWithCredentials(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(CorsConfig{
		AllowOrigins:     []string{"*"},
		AllowCredentials: true,
	}))
	r.GET("/secure", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Origin", "https://frontend.company.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://frontend.company.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
	assert.Contains(t, w.Header().Get("Vary"), "Origin")
}

func TestCORS_EnvFallback(t *testing.T) {
	t.Setenv("CORS_ALLOW_ORIGIN", "https://legacy.company.com, https://legacy2.company.com")
	t.Setenv("CORS_MAX_AGE_MINUTES", "30")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(CORS(CorsConfig{}))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "https://legacy.company.com")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "https://legacy.company.com", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, w.Header().Get("Vary"), "Origin")
}
