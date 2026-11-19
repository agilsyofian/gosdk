package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestAuthorizationBearer_Success(t *testing.T) {
	expectedToken := "test-jwt-token-123"
	provider := func(ctx context.Context) (string, error) {
		return expectedToken, nil
	}

	interceptor := AuthorizationBearer(provider)

	var capturedMD metadata.MD
	dummyInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		md, ok := metadata.FromOutgoingContext(ctx)
		require.True(t, ok)
		capturedMD = md
		return nil
	}

	ctx := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("x-custom-header", "custom-value"))
	err := interceptor(ctx, "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)

	assert.NoError(t, err)
	assert.Equal(t, []string{"Bearer " + expectedToken}, capturedMD.Get("authorization"))
	assert.Equal(t, []string{"custom-value"}, capturedMD.Get("x-custom-header"))
}

func TestAuthorizationBearer_Error(t *testing.T) {
	expectedErr := errors.New("token generation failed")
	provider := func(ctx context.Context) (string, error) {
		return "", expectedErr
	}

	interceptor := AuthorizationBearer(provider)

	invokerCalled := false
	dummyInvoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		invokerCalled = true
		return nil
	}

	err := interceptor(context.Background(), "/user.ServiceUser/GetUser", nil, nil, nil, dummyInvoker)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token generation failed")
	assert.False(t, invokerCalled, "invoker should not be called when token provider fails")
}

type clientTestLogger struct {
	lastLevel   string
	lastMsg     string
	lastPayload logger.LogPayload
}

func (m *clientTestLogger) Debug(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "debug"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *clientTestLogger) Info(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "info"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *clientTestLogger) Warn(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "warn"
	m.lastMsg = msg
	m.lastPayload = payload
}

func (m *clientTestLogger) Error(ctx context.Context, msg string, payload logger.LogPayload) {
	m.lastLevel = "error"
	m.lastMsg = msg
	m.lastPayload = payload
}

func TestClientLogger_Success(t *testing.T) {
	mockL := &clientTestLogger{}
	interceptor := Logger(mockL, "client-app")

	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return nil
	}

	req := map[string]string{"username": "test@example.com"}
	reply := map[string]string{"user_id": "123"}
	err := interceptor(context.Background(), "/user.ServiceUser/GetUser", req, reply, nil, invoker)

	assert.NoError(t, err)
	assert.Equal(t, "info", mockL.lastLevel)
	assert.Equal(t, 0, mockL.lastPayload.Status)
	assert.Equal(t, "INTEGRATION", mockL.lastPayload.Type)
	assert.Equal(t, "GRPC", mockL.lastPayload.Method)
	assert.Equal(t, "/user.ServiceUser/GetUser", mockL.lastPayload.Path)

	reqStr, ok := mockL.lastPayload.Request.(string)
	assert.True(t, ok)
	assert.Contains(t, reqStr, `"body":`)
	assert.Contains(t, reqStr, "test@example.com")
}

func TestClientLogger_Unauthenticated(t *testing.T) {
	mockL := &clientTestLogger{}
	interceptor := Logger(mockL, "client-app")

	unauthErr := status.Error(codes.Unauthenticated, "client unauthenticated")
	invoker := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
		return unauthErr
	}

	req := map[string]string{"username": "test@example.com"}
	err := interceptor(context.Background(), "/user.ServiceUser/GetUser", req, nil, nil, invoker)

	assert.Error(t, err)
	assert.Equal(t, "warn", mockL.lastLevel)
	assert.Equal(t, int(codes.Unauthenticated), mockL.lastPayload.Status)

	reqStr, ok := mockL.lastPayload.Request.(string)
	assert.True(t, ok)
	assert.Contains(t, reqStr, `"body":`)

	respStr, ok := mockL.lastPayload.Response.(string)
	assert.True(t, ok)
	assert.Contains(t, respStr, `"body":`)
	assert.Contains(t, respStr, "Unauthenticated")
	assert.Contains(t, respStr, "client unauthenticated")
}
