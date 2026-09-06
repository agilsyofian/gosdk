package interceptor

import (
	"context"
	"testing"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	coreContext "github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func TestTracerInterceptor_WithExistingMetadata(t *testing.T) {
	interceptor := Tracer("test-app")

	md := metadata.Pairs(
		MetadataTraceID, "grpc-trace-123",
		MetadataSessionID, "grpc-session-456",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	meta := &coremetadata.Metadata{}
	ctx = coremetadata.WithContext(ctx, meta)

	var recordedTraceID, recordedSessionID string
	var recordedMetaTraceID, recordedMetaSessionID string

	handler := func(handlerCtx context.Context, req any) (any, error) {
		recordedTraceID = coreContext.TraceID(handlerCtx)
		recordedSessionID = coreContext.SessionID(handlerCtx)
		if m, ok := coremetadata.FromContext(handlerCtx); ok {
			recordedMetaTraceID = m.TraceID
			recordedMetaSessionID = m.SessionID
		}
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/TestMethod"}
	resp, err := interceptor(ctx, nil, info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Equal(t, "grpc-trace-123", recordedTraceID)
	assert.Equal(t, "grpc-session-456", recordedSessionID)
	assert.Equal(t, "grpc-trace-123", recordedMetaTraceID)
	assert.Equal(t, "grpc-session-456", recordedMetaSessionID)
}

func TestTracerInterceptor_FallbackUUIDGeneration(t *testing.T) {
	interceptor := Tracer("test-app")

	ctx := context.Background()
	meta := &coremetadata.Metadata{}
	ctx = coremetadata.WithContext(ctx, meta)

	var recordedTraceID, recordedSessionID string

	handler := func(handlerCtx context.Context, req any) (any, error) {
		recordedTraceID = coreContext.TraceID(handlerCtx)
		recordedSessionID = coreContext.SessionID(handlerCtx)
		return "ok", nil
	}

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/TestMethod"}
	resp, err := interceptor(ctx, nil, info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.NotEmpty(t, recordedTraceID)
	assert.NotEmpty(t, recordedSessionID)
	assert.Equal(t, meta.TraceID, recordedTraceID)
	assert.Equal(t, meta.SessionID, recordedSessionID)
}

func TestTracerStream_Parity(t *testing.T) {
	interceptor := TracerStream("test-app")

	md := metadata.Pairs(
		MetadataTraceID, "stream-trace-999",
		MetadataSessionID, "stream-session-888",
	)
	ctx := metadata.NewIncomingContext(context.Background(), md)
	meta := &coremetadata.Metadata{}
	ctx = coremetadata.WithContext(ctx, meta)

	var recordedTraceID, recordedSessionID string
	handler := func(srv any, ss grpc.ServerStream) error {
		recordedTraceID = coreContext.TraceID(ss.Context())
		recordedSessionID = coreContext.SessionID(ss.Context())
		return nil
	}

	ss := &mockServerStream{ctx: ctx}
	info := &grpc.StreamServerInfo{FullMethod: "/test.StreamService/Stream"}
	err := interceptor(nil, ss, info, handler)

	assert.NoError(t, err)
	assert.Equal(t, "stream-trace-999", recordedTraceID)
	assert.Equal(t, "stream-session-888", recordedSessionID)
	assert.Equal(t, "stream-trace-999", meta.TraceID)
	assert.Equal(t, "stream-session-888", meta.SessionID)
}
