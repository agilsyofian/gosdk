package http

import (
	"context"
	"net/http"
	"testing"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestHTTPClientTracer_InjectsTraceAndSessionID(t *testing.T) {
	otel.SetTextMapPropagator(propagation.TraceContext{})
	middleware := Tracer("test-app")

	var capturedReq *http.Request
	handler := middleware(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	ctx := context.Background()
	ctx = tracing.WithTraceID(ctx, "test-trace-id-123")
	ctx = tracing.WithSessionID(ctx, "test-session-id-456")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/test", nil)
	assert.NoError(t, err)

	_, err = handler(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, capturedReq)

	assert.Equal(t, "test-trace-id-123", capturedReq.Header.Get(MetadataTraceID))
	assert.Equal(t, "test-session-id-456", capturedReq.Header.Get(MetadataSessionID))
}

func TestHTTPClientTracer_FallbackToMetadata(t *testing.T) {
	middleware := Tracer("test-app")

	var capturedReq *http.Request
	handler := middleware(func(ctx context.Context, req *http.Request) (*http.Response, error) {
		capturedReq = req
		return &http.Response{StatusCode: http.StatusOK}, nil
	})

	md := coremetadata.New()
	md.TraceID = "meta-trace-789"
	md.SessionID = "meta-session-789"
	ctx := coremetadata.WithContext(context.Background(), md)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://localhost/test", nil)
	assert.NoError(t, err)

	_, err = handler(ctx, req)
	assert.NoError(t, err)
	assert.NotNil(t, capturedReq)

	assert.Equal(t, "meta-trace-789", capturedReq.Header.Get(MetadataTraceID))
	assert.Equal(t, "meta-session-789", capturedReq.Header.Get(MetadataSessionID))
}
