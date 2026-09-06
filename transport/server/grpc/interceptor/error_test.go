package interceptor

import (
	"context"
	"errors"
	"testing"

	coreErrors "github.com/agilsyofian/gosdk/core/errors"
	coremetadata "github.com/agilsyofian/gosdk/core/metadata"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestErrorInterceptor_Success(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Error(mockLog, "test-app", false)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Success"}
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	resp, err := interceptor(ctx, "req", info, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.Empty(t, mockLog.lastLevel)
}

func TestErrorInterceptor_AppErrorMapping(t *testing.T) {
	tests := []struct {
		name         string
		httpStatus   coreErrors.HTTPStatus
		expectedCode codes.Code
	}{
		{"Bad Request", coreErrors.StatusBadRequest, codes.InvalidArgument},
		{"Unauthorized", coreErrors.StatusUnauthorized, codes.Unauthenticated},
		{"Forbidden", coreErrors.StatusForbidden, codes.PermissionDenied},
		{"Not Found", coreErrors.StatusNotFound, codes.NotFound},
		{"Too Many Requests", coreErrors.StatusTooManyRequests, codes.ResourceExhausted},
		{"Internal", coreErrors.StatusInternalServerError, codes.Internal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockLog := &interceptorTestLogger{}
			interceptor := Error(mockLog, "test-app", false)

			ctx := context.Background()
			meta := &coremetadata.Metadata{ClientIP: "127.0.0.1"}
			ctx = coremetadata.WithContext(ctx, meta)

			info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Test"}
			appErr := &coreErrors.AppError{
				Status:  tt.httpStatus,
				Message: "entity error",
				Cause:   errors.New("detail message"),
			}
			handler := func(ctx context.Context, req any) (any, error) {
				return nil, appErr
			}

			_, err := interceptor(ctx, "req", info, handler)
			assert.Error(t, err)

			st, ok := status.FromError(err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedCode, st.Code())
			assert.Equal(t, "entity error", st.Message())

			assert.Equal(t, "error", mockLog.lastLevel)
			assert.Equal(t, "127.0.0.1", mockLog.lastPayload.IpAddress)
			assert.Equal(t, int(tt.expectedCode), mockLog.lastPayload.Status)
		})
	}
}

func TestErrorInterceptor_PreservesNativeGRPCStatus(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Error(mockLog, "test-app", false)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/GetProfile"}
	nativeErr := status.Error(codes.NotFound, "user profile was not found")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, nativeErr
	}

	_, err := interceptor(ctx, "req", info, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Equal(t, "user profile was not found", st.Message())
	assert.Equal(t, int(codes.NotFound), mockLog.lastPayload.Status)
}

func TestErrorInterceptor_GenericError_NoDetail_NoTypo(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Error(mockLog, "test-app", false)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Crash"}
	genericErr := errors.New("raw unhandled error")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, genericErr
	}

	_, err := interceptor(ctx, "req", info, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	// Verify that typo "Internal server ExpectedError" is fixed to "Internal server error"
	assert.Equal(t, "Internal server error", st.Message())
	assert.NotContains(t, st.Message(), "ExpectedError")
}

func TestErrorInterceptor_GenericError_WithDetail(t *testing.T) {
	mockLog := &interceptorTestLogger{}
	interceptor := Error(mockLog, "test-app", true)

	ctx := context.Background()
	info := &grpc.UnaryServerInfo{FullMethod: "/test.Service/Crash"}
	genericErr := errors.New("raw unhandled error")
	handler := func(ctx context.Context, req any) (any, error) {
		return nil, genericErr
	}

	_, err := interceptor(ctx, "req", info, handler)
	assert.Error(t, err)

	st, ok := status.FromError(err)
	assert.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	assert.Contains(t, st.Message(), "raw unhandled error")
}
