package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/observability/tracing"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"
)

type CombinedRequest struct {
	Body  string            `json:"body"`
	Query map[string]string `json:"query"`
}

func Logger(l logger.Logger, appName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()

		reqBody := captureRequestBody(c)
		resBody := captureResponseBody(c)
		reqQuery := captureQueryParams(c)

		fullRequest, _ := json.Marshal(CombinedRequest{
			Body:  string(reqBody),
			Query: reqQuery,
		})

		c.Next()

		if strings.HasPrefix(c.Request.URL.Path, "/swagger") {
			return
		}

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
			Response:  resBody.body.String(),
			Duration:  time.Since(start).Milliseconds(),
		}

		l.Info(
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

func (w bodyLogWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
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
