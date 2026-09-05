package middleware

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

type CorsConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// CORS returns a Gin middleware that handles Cross-Origin Resource Sharing.
func CORS(cfg CorsConfig) gin.HandlerFunc {
	corsHandler := cors.New(buildCorsConfig(cfg))
	return func(c *gin.Context) {
		corsHandler(c)
		if _, exists := c.Writer.Header()["Access-Control-Allow-Origin"]; exists {
			addVaryOrigin(c)
		}
	}
}

// VaryOrigin ensures the Vary: Origin header is set whenever Access-Control-Allow-Origin is present.
func VaryOrigin() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if _, exists := c.Writer.Header()["Access-Control-Allow-Origin"]; exists {
			addVaryOrigin(c)
		}
	}
}

func addVaryOrigin(c *gin.Context) {
	vary := c.Writer.Header().Get("Vary")
	if vary == "" {
		c.Writer.Header().Set("Vary", "Origin")
		return
	}
	if !strings.Contains(vary, "Origin") {
		c.Writer.Header().Set("Vary", vary+", Origin")
	}
}

func buildCorsConfig(cfg CorsConfig) cors.Config {
	allowAll, origins, originFunc := resolveOrigins(cfg.AllowOrigins, cfg.AllowCredentials)

	return cors.Config{
		AllowAllOrigins:  allowAll,
		AllowOrigins:     origins,
		AllowOriginFunc:  originFunc,
		AllowMethods:     resolveAllowMethods(cfg.AllowMethods),
		AllowHeaders:     resolveAllowHeaders(cfg.AllowHeaders),
		ExposeHeaders:    resolveExposeHeaders(cfg.ExposeHeaders),
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           resolveMaxAge(cfg.MaxAge),
	}
}

func resolveOrigins(origins []string, allowCredentials bool) (bool, []string, func(string) bool) {
	resolved := origins
	if len(resolved) == 0 {
		resolved = parseEnvOrigins()
	}
	if len(resolved) == 0 {
		resolved = []string{"*"}
	}

	if containsWildcard(resolved) {
		if allowCredentials {
			// CORS spec forbids '*' with credentials; reflect requesting origin.
			return false, nil, func(origin string) bool { return true }
		}
		return true, nil, nil
	}

	return false, resolved, nil
}

func parseEnvOrigins() []string {
	envVal := strings.TrimSpace(os.Getenv("CORS_ALLOW_ORIGIN"))
	if envVal == "" {
		return nil
	}
	parts := strings.Split(envVal, ",")
	var result []string
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func containsWildcard(origins []string) bool {
	for _, o := range origins {
		if o == "*" {
			return true
		}
	}
	return false
}

func resolveAllowMethods(methods []string) []string {
	if len(methods) > 0 {
		return methods
	}
	return []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
}

func resolveAllowHeaders(headers []string) []string {
	if len(headers) > 0 {
		return headers
	}
	return []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Requested-With", "X-Session-Id", "X-Trace-Id", "X-Request-Id"}
}

func resolveExposeHeaders(headers []string) []string {
	if len(headers) > 0 {
		return headers
	}
	return []string{"Content-Length", "X-Session-Id", "X-Trace-Id", "X-Request-Id"}
}

func resolveMaxAge(d time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	if minStr := os.Getenv("CORS_MAX_AGE_MINUTES"); minStr != "" {
		if min, err := strconv.Atoi(minStr); err == nil && min > 0 {
			return time.Duration(min) * time.Minute
		}
	}
	return 12 * time.Hour
}
