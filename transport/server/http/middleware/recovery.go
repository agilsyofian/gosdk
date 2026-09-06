package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/errors"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

const (
	// PanicDetailsKey is the context key for storing panic details in gin.Context.
	PanicDetailsKey = "panic_details"
	// PanicLoggedKey marks that a panic has been logged by Recovery middleware.
	PanicLoggedKey = "panic_logged"
)

// SourceLocation contains stack trace location details where a panic occurred.
type SourceLocation struct {
	File     string `json:"file,omitempty"`
	Function string `json:"function,omitempty"`
	Line     int    `json:"line,omitempty"`
}

// Recovery returns a gin.HandlerFunc that catches panics, extracts stack trace
// and source location, logs the structured error, and attaches a standardized error.
func Recovery(l logger.Logger, appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				loc, stackTrace := parseStackTrace(debug.Stack())

				details := map[string]any{
					"panic":    fmt.Sprintf("%v", r),
					"file":     loc.File,
					"function": loc.Function,
					"line":     loc.Line,
					"stack":    stackTrace,
				}
				c.Set(PanicDetailsKey, details)

				if l != nil {
					c.Set(PanicLoggedKey, true)
					l.Error(
						c.Request.Context(),
						fmt.Sprintf("panic recovered in HTTP %s %s", c.Request.Method, c.Request.URL.Path),
						logger.LogPayload{
							Timestamp: time.Now(),
							AppName:   appName,
							Type:      "ERROR",
							Method:    c.Request.Method,
							IpAddress: c.ClientIP(),
							Metadata:  c.Request.Header,
							TraceId:   tracing.TraceID(c.Request.Context()),
							SpanId:    trace.SpanContextFromContext(c.Request.Context()).SpanID().String(),
							SessionId: tracing.SessionID(c.Request.Context()),
							Path:      c.Request.URL.Path,
							Status:    http.StatusInternalServerError,
							Error:     details,
						},
					)
				}

				var err error
				switch x := r.(type) {
				case error:
					err = x
				default:
					err = fmt.Errorf("%v", x)
				}

				appErr := errors.InternalServerError(
					err,
					appName,
					errors.StatusInternalServerError.String(),
					"An error occurred",
				)

				_ = c.Error(appErr)
				c.Abort()
			}
		}()

		c.Next()
	}
}

func parseStackTrace(stack []byte) (SourceLocation, string) {
	stackStr := string(stack)
	lines := strings.Split(stackStr, "\n")
	return findSourceLocation(lines), stackStr
}

func findSourceLocation(lines []string) SourceLocation {
	for i := 1; i < len(lines)-1; i++ {
		line := strings.TrimSpace(lines[i])
		if isRuntimeOrFrameworkFrame(line) {
			continue
		}
		if i+1 < len(lines) && strings.Contains(lines[i+1], ".go:") {
			return extractLocationFromFrame(line, lines[i+1])
		}
	}
	return SourceLocation{}
}

func isRuntimeOrFrameworkFrame(line string) bool {
	return strings.HasPrefix(line, "goroutine ") ||
		strings.Contains(line, "runtime/debug.") ||
		strings.Contains(line, "runtime/panic.") ||
		strings.Contains(line, "middleware.Recovery") ||
		strings.Contains(line, "gin-gonic/gin")
}

func extractLocationFromFrame(fnLine, fileLine string) SourceLocation {
	loc := SourceLocation{Function: fnLine}
	cleaned := strings.TrimSpace(fileLine)
	if idx := strings.LastIndex(cleaned, " +0x"); idx != -1 {
		cleaned = cleaned[:idx]
	}
	parts := strings.Split(cleaned, ":")
	if len(parts) >= 2 {
		loc.File = parts[0]
		if l, err := strconv.Atoi(parts[1]); err == nil {
			loc.Line = l
		}
	}
	return loc
}
