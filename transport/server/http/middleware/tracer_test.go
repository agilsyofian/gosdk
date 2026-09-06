package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestTracer_WithExistingHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	var recordedTraceID, recordedSessionID string
	var recordedMetaTraceID, recordedMetaSessionID string

	router.Use(func(c *gin.Context) {
		meta := &coremetadata.Metadata{}
		ctx := coremetadata.WithContext(c.Request.Context(), meta)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(Tracer("test-app"))
	router.GET("/test", func(c *gin.Context) {
		recordedTraceID = coreContext.TraceID(c.Request.Context())
		recordedSessionID = coreContext.SessionID(c.Request.Context())

		if m, ok := coremetadata.FromContext(c.Request.Context()); ok {
			recordedMetaTraceID = m.TraceID
			recordedMetaSessionID = m.SessionID
		}
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(MetadataTraceID, "custom-trace-123")
	req.Header.Set(MetadataSessionID, "custom-session-456")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "custom-trace-123", rec.Header().Get(MetadataTraceID))
	assert.Equal(t, "custom-session-456", rec.Header().Get(MetadataSessionID))
	assert.Equal(t, "custom-trace-123", recordedTraceID)
	assert.Equal(t, "custom-session-456", recordedSessionID)
	assert.Equal(t, "custom-trace-123", recordedMetaTraceID)
	assert.Equal(t, "custom-session-456", recordedMetaSessionID)
}

func TestTracer_FallbackUUIDGeneration(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	var recordedTraceID, recordedSessionID string

	router.Use(func(c *gin.Context) {
		meta := &coremetadata.Metadata{}
		ctx := coremetadata.WithContext(c.Request.Context(), meta)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	router.Use(Tracer("test-app"))
	router.GET("/test-fallback", func(c *gin.Context) {
		recordedTraceID = coreContext.TraceID(c.Request.Context())
		recordedSessionID = coreContext.SessionID(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test-fallback", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotEmpty(t, rec.Header().Get(MetadataTraceID))
	assert.NotEmpty(t, rec.Header().Get(MetadataSessionID))
	assert.Equal(t, rec.Header().Get(MetadataTraceID), recordedTraceID)
	assert.Equal(t, rec.Header().Get(MetadataSessionID), recordedSessionID)
}
