package middleware

import (
	"fmt"
	"html"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SecurityHeadersConfig defines configuration for standard pentest security headers.
type SecurityHeadersConfig struct {
	Disabled              bool          `mapstructure:"disabled"`
	HSTSMaxAge            time.Duration `mapstructure:"hsts_max_age"`
	DisableHSTSSubdomains bool          `mapstructure:"disable_hsts_subdomains"`
	DisableHSTSPreload    bool          `mapstructure:"disable_hsts_preload"`
	ContentSecurityPolicy string        `mapstructure:"content_security_policy"`
	FrameOptions          string        `mapstructure:"frame_options"`
	ReferrerPolicy        string        `mapstructure:"referrer_policy"`
	DisableNoCache        bool          `mapstructure:"disable_no_cache"`
	SanitizeResponse      bool          `mapstructure:"sanitize_response"`
}

// SecurityHeaders returns a Gin middleware that injects industry standard
// security headers (HSTS, CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy, Cache-Control)
// required for financial/fintech pentesting.
func SecurityHeaders(cfg SecurityHeadersConfig) gin.HandlerFunc {
	if cfg.Disabled {
		return func(c *gin.Context) {
			c.Next()
		}
	}

	hstsValue := buildHSTSHeader(cfg)
	cspValue := defaultIfEmpty(cfg.ContentSecurityPolicy, "frame-ancestors 'none'")
	frameOptions := defaultIfEmpty(cfg.FrameOptions, "DENY")
	referrerPolicy := defaultIfEmpty(cfg.ReferrerPolicy, "strict-origin-when-cross-origin")

	return func(c *gin.Context) {
		if hstsValue != "" {
			c.Header("Strict-Transport-Security", hstsValue)
		}
		c.Header("Content-Security-Policy", cspValue)
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", frameOptions)
		c.Header("Referrer-Policy", referrerPolicy)

		if !cfg.DisableNoCache {
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
			c.Header("Pragma", "no-cache")
		}

		if cfg.SanitizeResponse {
			c.Writer = &sanitizedWriter{ResponseWriter: c.Writer}
		}

		c.Next()
	}
}

func buildHSTSHeader(cfg SecurityHeadersConfig) string {
	maxAgeSec := int(cfg.HSTSMaxAge.Seconds())
	if maxAgeSec <= 0 {
		if envSec := os.Getenv("HSTS_MAX_AGE_SECONDS"); envSec != "" {
			if v, err := strconv.Atoi(envSec); err == nil && v > 0 {
				maxAgeSec = v
			}
		}
	}
	if maxAgeSec <= 0 {
		maxAgeSec = 16070400 // Default ~6 months (16070400s)
	}

	hsts := fmt.Sprintf("max-age=%d", maxAgeSec)
	if !cfg.DisableHSTSSubdomains {
		hsts += "; includeSubDomains"
	}
	if !cfg.DisableHSTSPreload {
		hsts += "; preload"
	}
	return hsts
}

// NoCache returns a middleware that adds cache prevention headers.
func NoCache() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Next()
	}
}

// ContentTypeNosniff returns a middleware that adds the X-Content-Type-Options: nosniff header.
func ContentTypeNosniff() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("X-Content-Type-Options", "nosniff")
		c.Next()
	}
}

// HSTS returns a middleware that adds the Strict-Transport-Security header.
func HSTS(maxAgeSeconds int, includeSubDomains, preload bool) gin.HandlerFunc {
	hsts := fmt.Sprintf("max-age=%d", maxAgeSeconds)
	if includeSubDomains {
		hsts += "; includeSubDomains"
	}
	if preload {
		hsts += "; preload"
	}
	return func(c *gin.Context) {
		c.Header("Strict-Transport-Security", hsts)
		c.Header("Content-Security-Policy", "frame-ancestors 'none'")
		c.Next()
	}
}

// SanitizeResponse returns a middleware that HTML-escapes text/html and application/json responses.
func SanitizeResponse() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer = &sanitizedWriter{ResponseWriter: c.Writer}
		c.Next()
	}
}

type sanitizedWriter struct {
	gin.ResponseWriter
}

func (w *sanitizedWriter) Write(data []byte) (int, error) {
	contentType := w.Header().Get("Content-Type")
	if strings.HasPrefix(contentType, "text/html") || strings.HasPrefix(contentType, "application/json") {
		safe := html.EscapeString(string(data))
		return w.ResponseWriter.Write([]byte(safe))
	}
	return w.ResponseWriter.Write(data)
}

// SetSecureCookie sets an HTTP cookie with security attributes SameSite=Strict, Secure=true, and HttpOnly=true.
func SetSecureCookie(c *gin.Context, name, value string, maxAge int, path, domain string) {
	c.SetSameSite(http.SameSiteStrictMode)
	c.SetCookie(name, value, maxAge, path, domain, true, true)
}

// SecureCookie returns a middleware enforcing SameSite=Strict.
func SecureCookie() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.SetSameSite(http.SameSiteStrictMode)
		c.Next()
	}
}

func defaultIfEmpty(val, def string) string {
	if val == "" {
		return def
	}
	return val
}
