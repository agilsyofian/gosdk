package interceptor

import (
	"context"
	"errors"
	"testing"

	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type interceptorTestLogger struct {
	lastLevel   string
	lastMsg     string
	lastPayload logger.LogPayload
}

func (m *interceptorTestLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "debug"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *interceptorTestLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "info"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *interceptorTestLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "warn"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *interceptorTestLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "error"
	m.lastMsg = msg
	m.lastPayload = payload
}

func TestLoggerInterceptor_Success(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Logger(mockLog, "test-app")

	ctx := context.Background()
	meta := &coremetadata.Metadata{ClientIP: "192.168.1.50"}
	ctx = coremetadata.WithContext(ctx, meta)

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/GetItem"}
	handler := func(ctx context.Context, req any) (any, error) {
		return "item-data", nil
	}

	resp, err := interceptor(ctx, "req-data", info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "item-data", resp)

	assert.Equal(t, "info", mockLog.lastLevel)
	assert.Equal(t, "GRPC /test.Service/GetItem", mockLog.lastMsg)
	assert.Equal(t, "192.168.1.50", mockLog.lastPayload.IpAddress)
	assert.Equal(t, 0, mockLog.lastPayload.Status)
	assert.Equal(t, "/test.Service/GetItem", mockLog.lastPayload.Path)
	assert.Nil(t, mockLog.lastPayload.Error)
}

func TestLoggerInterceptor_ClientError_WarnLevel(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Logger(mockLog, "test-app")

	ctx := context.Background()
	meta := &coremetadata.Metadata{ClientIP: "10.0.0.1"}
	ctx = coremetadata.WithContext(ctx, meta)

	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/CreateItem"}
	clientErr := status.Error(codes.InvalidArgument, "invalid item id")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, clientErr
	}

	_, err := interceptor(ctx, "bad-req", info, handler)
	assert.Error(t, err)

	assert.Equal(t, "warn", mockLog.lastLevel)
	assert.Equal(t, "10.0.0.1", mockLog.lastPayload.IpAddress)
	assert.Equal(t, int(codes.InvalidArgument), mockLog.lastPayload.Status)
	assert.Equal(t, clientErr.Error(), mockLog.lastPayload.Error)
}

func TestLoggerInterceptor_ServerError_ErrorLevel(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Logger(mockLog, "test-app")

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/InternalMethod"}
	serverErr := errors.New("database connection failure")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, serverErr
	}

	_, err := interceptor(ctx, "req", info, handler)
	assert.Error(t, err)

	assert.Equal(t, "error", mockLog.lastLevel)
	assert.Equal(t, int(codes.Unknown), mockLog.lastPayload.Status)
	assert.Equal(t, "database connection failure", mockLog.lastPayload.Error)
}
