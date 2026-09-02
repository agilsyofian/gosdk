package grpc

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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
