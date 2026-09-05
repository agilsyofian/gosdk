package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/transport/server/http/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestSecurityHeaders_Default(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{}))
	r.GET("/secure", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=16070400")
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "includeSubDomains")
	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "preload")
	assert.Equal(t, "frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", w.Header().Get("Referrer-Policy"))
	assert.Equal(t, "no-store, no-cache, must-revalidate", w.Header().Get("Cache-Control"))
	assert.Equal(t, "no-cache", w.Header().Get("Pragma"))
}

func TestSecurityHeaders_CustomConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
		HSTSMaxAge:            365 * 24 * time.Hour,
		DisableHSTSSubdomains: true,
		DisableHSTSPreload:    true,
		ContentSecurityPolicy: "default-src 'self'",
		FrameOptions:          "SAMEORIGIN",
		ReferrerPolicy:        "no-referrer",
		DisableNoCache:        true,
	}))
	r.GET("/custom", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/custom", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "max-age=31536000", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "default-src 'self'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "SAMEORIGIN", w.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", w.Header().Get("Referrer-Policy"))
	assert.Empty(t, w.Header().Get("Cache-Control"))
}

func TestSecurityHeaders_EnvFallback(t *testing.T) {
	t.Setenv("HSTS_MAX_AGE_SECONDS", "86400")

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{}))
	r.GET("/env", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/env", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Contains(t, w.Header().Get("Strict-Transport-Security"), "max-age=86400")
}

func TestSecurityHeaders_Disabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.SecurityHeaders(middleware.SecurityHeadersConfig{
		Disabled: true,
	}))
	r.GET("/disabled", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/disabled", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Empty(t, w.Header().Get("Strict-Transport-Security"))
	assert.Empty(t, w.Header().Get("Content-Security-Policy"))
	assert.Empty(t, w.Header().Get("X-Content-Type-Options"))
	assert.Empty(t, w.Header().Get("Cache-Control"))
}

func TestStandaloneMiddlewares(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(
		middleware.NoCache(),
		middleware.ContentTypeNosniff(),
		middleware.HSTS(3600, true, false),
		middleware.SanitizeResponse(),
	)
	r.GET("/standalone", func(c *gin.Context) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, "<b>Hello & Welcome</b>")
	})

	req := httptest.NewRequest(http.MethodGet, "/standalone", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, "no-store, no-cache, must-revalidate", w.Header().Get("Cache-Control"))
	assert.Equal(t, "nosniff", w.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "max-age=3600; includeSubDomains", w.Header().Get("Strict-Transport-Security"))
	assert.Equal(t, "frame-ancestors 'none'", w.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "&lt;b&gt;Hello &amp; Welcome&lt;/b&gt;", w.Body.String())
}

func TestSetSecureCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/set-cookie", func(c *gin.Context) {
		middleware.SetSecureCookie(c, "session_token", "secret123", 3600, "/", "example.com")
		c.String(http.StatusOK, "cookie set")
	})

	req := httptest.NewRequest(http.MethodGet, "/set-cookie", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	cookieHeader := w.Header().Get("Set-Cookie")
	assert.Contains(t, cookieHeader, "session_token=secret123")
	assert.Contains(t, cookieHeader, "HttpOnly")
	assert.Contains(t, cookieHeader, "Secure")
	assert.Contains(t, cookieHeader, "SameSite=Strict")
}
