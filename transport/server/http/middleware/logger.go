package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

type CombinedRequest struct {
	Body  string            `json:"body"`
	Query map[string]string `json:"query"`
}

type CombinedResponse struct {
	Body string `json:"body"`
}

// RouteMatcher matches request paths against exact, prefix, and regex patterns.
type RouteMatcher struct {
	patterns []string
	regexes  []*regexp.Regexp
}

// NewRouteMatcher creates a new RouteMatcher preloaded with default exclusions ("/swagger")
// and any user-specified routes.
func NewRouteMatcher(routes ...string) *RouteMatcher {
	rm := &RouteMatcher{
		patterns: []string{"/swagger"},
	}
	rm.Add(routes...)
	return rm
}

// Add adds routes/patterns to the matcher.
func (rm *RouteMatcher) Add(routes ...string) {
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if strings.ContainsAny(r, `^$*+?|()[]`) {
			if re, err := regexp.Compile(r); err == nil {
				rm.regexes = append(rm.regexes, re)
				continue
			}
		}
		rm.patterns = append(rm.patterns, r)
	}
}

// Matches checks if the path matches any excluded pattern or regex.
func (rm *RouteMatcher) Matches(path string) bool {
	for _, p := range rm.patterns {
		if matchPath(p, path) {
			return true
		}
	}
	for _, re := range rm.regexes {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func matchPath(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	if path == pattern {
		return true
	}
	patternWithSlash := strings.TrimRight(pattern, "/") + "/"
	if strings.HasPrefix(path, patternWithSlash) {
		return true
	}
	trimmedPattern := "/" + strings.Trim(pattern, "/")
	if strings.Contains(path, trimmedPattern+"/") || strings.HasSuffix(path, trimmedPattern) {
		return true
	}
	return false
}

// Logger creates a gin middleware for request/response logging.
// Routes matching any pattern in excludeRoutes (or ROUTES_EXCLUDE_LOGGER env) or "/swagger" are excluded from logging.
func Logger(l logger.Logger, appName string, excludeRoutes ...string) gin.HandlerFunc {
	matcher := NewRouteMatcher(excludeRoutes...)
	if envRoutes := os.Getenv("ROUTES_EXCLUDE_LOGGER"); envRoutes != "" {
		for _, r := range strings.Split(envRoutes, ",") {
			matcher.Add(r)
		}
	}

	return func(c *gin.Context) {
		if matcher.Matches(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()

		reqBody := captureRequestBody(c)
		resBody := captureResponseBody(c)
		reqQuery := captureQueryParams(c)

		fullRequest, _ := json.Marshal(CombinedRequest{
			Body:  string(reqBody),
			Query: reqQuery,
		})

		c.Next()

		fullResponse, _ := json.Marshal(CombinedResponse{
			Body: resBody.body.String(),
		})

		payload := logger.LogPayload{
			Timestamp: start,
			AppName:   appName,
			Type:      "TRANSPORT",
			Method:    c.Request.Method,
			IpAddress: c.ClientIP(),
			Metadata:  c.Request.Header,
			TraceId:   tracing.TraceID(c.Request.Context()),
			SpanId:    trace.SpanContextFromContext(c.Request.Context()).SpanID().String(),
			SessionId: tracing.SessionID(c.Request.Context()),
			Path:      c.Request.URL.Path,
			Status:    c.Writer.Status(),
			Request:   string(fullRequest),
			Response:  string(fullResponse),
			Duration:  time.Since(start).Milliseconds(),
		}

		logFunc := l.Info
		if payload.Status >= 500 {
			logFunc = l.Error
		} else if payload.Status >= 400 {
			logFunc = l.Warn
		}

		logFunc(
			c.Request.Context(),
			fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
			payload,
		)
	}
}

type bodyLogWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

func (w *bodyLogWriter) WriteString(s string) (int, error) {
	w.body.WriteString(s)
	return w.ResponseWriter.WriteString(s)
}

// captureRequestBody reads and resets the request body for further use.
func captureRequestBody(c *gin.Context) []byte {
	if c.Request.Body == nil {
		return nil
	}

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	return bodyBytes
}

// captureResponseBody creates a bodyLogWriter to capture response data.
func captureResponseBody(c *gin.Context) *bodyLogWriter {
	blw := &bodyLogWriter{
		ResponseWriter: c.Writer,
		body:           &bytes.Buffer{},
	}
	c.Writer = blw
	return blw
}

// Helper function to extract query parameters from the request
func captureQueryParams(c *gin.Context) map[string]string {
	query := c.Request.URL.Query()
	if len(query) == 0 {
		return nil
	}

	queryParams := make(map[string]string)
	for key, values := range query {
		queryParams[key] = strings.Join(values, ",")
	}

	return queryParams
}
