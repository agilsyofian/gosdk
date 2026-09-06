package security

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type mockSecurityManager struct {
	authFunc        func(ctx context.Context, cred security.Credential) (*security.SecurityContext, error)
	authSessionFunc func(ctx context.Context, sessionID string) (*security.SecurityContext, error)
	authorizeFunc   func(ctx context.Context, permission string) error
}

func (m *mockSecurityManager) Authenticate(ctx context.Context, cred security.Credential) (*security.SecurityContext, error) {
	if m.authFunc != nil {
		return m.authFunc(ctx, cred)
	}
	return &security.SecurityContext{Authenticated: true}, nil
}

func (m *mockSecurityManager) AuthenticateSession(ctx context.Context, sessionID string) (*security.SecurityContext, error) {
	if m.authSessionFunc != nil {
		return m.authSessionFunc(ctx, sessionID)
	}
	return &security.SecurityContext{Authenticated: true}, nil
}

func (m *mockSecurityManager) Authorize(ctx context.Context, permission string) error {
	if m.authorizeFunc != nil {
		return m.authorizeFunc(ctx, permission)
	}
	return nil
}

func TestExtractor_Bearer(t *testing.T) {
	cfg := Config{
		EnableBearer: true,
	}

	md := metadata.Pairs("authorization", "Bearer test-token-123")
	sessionID, cred, err := extractCredential(md, cfg)
	require.NoError(t, err)
	assert.Empty(t, sessionID)
	require.NotNil(t, cred)
	assert.Equal(t, security.CredentialBearer, cred.Type)
	assert.Equal(t, "test-token-123", cred.Value)
}

func TestExtractor_Basic(t *testing.T) {
	cfg := Config{
		EnableBasic: true,
	}

	md := metadata.Pairs("authorization", "Basic dXNlcjpwYXNz")
	sessionID, cred, err := extractCredential(md, cfg)
	require.NoError(t, err)
	assert.Empty(t, sessionID)
	require.NotNil(t, cred)
	assert.Equal(t, security.CredentialBasic, cred.Type)
	assert.Equal(t, "dXNlcjpwYXNz", cred.Value)
}

func TestExtractor_APIKey(t *testing.T) {
	cfg := Config{
		EnableAPIKey: true,
	}

	md := metadata.Pairs("x-api-key", "my-secret-api-key")
	sessionID, cred, err := extractCredential(md, cfg)
	require.NoError(t, err)
	assert.Empty(t, sessionID)
	require.NotNil(t, cred)
	assert.Equal(t, security.CredentialAPIKey, cred.Type)
	assert.Equal(t, "my-secret-api-key", cred.Value)
}

func TestExtractor_Session(t *testing.T) {
	cfg := Config{
		SessionMetadataKey: "session-id",
	}

	md := metadata.Pairs("session-id", "sess-xyz")
	sessionID, cred, err := extractCredential(md, cfg)
	require.NoError(t, err)
	assert.Equal(t, "sess-xyz", sessionID)
	assert.Nil(t, cred)
}

func TestExtractor_Conflicting(t *testing.T) {
	cfg := Config{
		EnableBearer: true,
		EnableAPIKey: true,
	}

	md := metadata.Pairs(
		"authorization", "Bearer test-token",
		"x-api-key", "test-key",
	)
	_, _, err := extractCredential(md, cfg)
	assert.ErrorIs(t, err, security.ErrConflictingCredential)
}

func TestExtractor_NoCredential(t *testing.T) {
	cfg := Config{
		EnableBearer: true,
	}

	md := metadata.Pairs()
	_, _, err := extractCredential(md, cfg)
	assert.ErrorIs(t, err, security.ErrNoCredential)
}

func TestInterceptor_Unary_Success(t *testing.T) {
	mgr := &mockSecurityManager{
		authFunc: func(ctx context.Context, cred security.Credential) (*security.SecurityContext, error) {
			assert.Equal(t, "valid-token", cred.Value)
			return &security.SecurityContext{
				Authenticated: true,
				Principal: &security.Principal{
					Subject: "user1",
				},
			}, nil
		},
	}
	cfg := Config{
		Required:     true,
		EnableBearer: true,
	}
	interceptor := New(mgr, cfg)

	md := metadata.Pairs("authorization", "Bearer valid-token")
	ctx := metadata.NewIncomingContext(context.Background(), md)

	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		secCtx, ok := security.FromContext(ctx)
		require.True(t, ok)
		require.NotNil(t, secCtx)
		require.NotNil(t, secCtx.Principal)
		assert.Equal(t, "user1", secCtx.Principal.Subject)
		return "ok", nil
	}

	resp, err := interceptor.UnaryServerInterceptor()(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called)
}

func TestInterceptor_Unary_Required_Fail(t *testing.T) {
	mgr := &mockSecurityManager{}
	cfg := Config{
		Required:     true,
		EnableBearer: true,
	}
	interceptor := New(mgr, cfg)

	ctx := context.Background() // no metadata
	handler := func(ctx context.Context, req any) (any, error) {
		return "ok", nil
	}

	_, err := interceptor.UnaryServerInterceptor()(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestInterceptor_Unary_NotRequired_PassThrough(t *testing.T) {
	mgr := &mockSecurityManager{}
	cfg := Config{
		Required:     false,
		EnableBearer: true,
	}
	interceptor := New(mgr, cfg)

	ctx := context.Background() // no metadata
	called := false
	handler := func(ctx context.Context, req any) (any, error) {
		called = true
		return "ok", nil
	}

	resp, err := interceptor.UnaryServerInterceptor()(ctx, "req", &grpc.UnaryServerInfo{}, handler)
	assert.NoError(t, err)
	assert.Equal(t, "ok", resp)
	assert.True(t, called)
}

type mockServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (m *mockServerStream) Context() context.Context {
	return m.ctx
}

func TestInterceptor_Stream_Success(t *testing.T) {
	mgr := &mockSecurityManager{
		authSessionFunc: func(ctx context.Context, sessionID string) (*security.SecurityContext, error) {
			assert.Equal(t, "sess-123", sessionID)
			return &security.SecurityContext{
				Authenticated: true,
				Principal: &security.Principal{
					Subject: "user-stream",
				},
			}, nil
		},
	}
	cfg := Config{
		Required:           true,
		SessionMetadataKey: "session-id",
	}
	interceptor := New(mgr, cfg)

	md := metadata.Pairs("session-id", "sess-123")
	ctx := metadata.NewIncomingContext(context.Background(), md)
	stream := &mockServerStream{ctx: ctx}

	called := false
	handler := func(srv any, ss grpc.ServerStream) error {
		called = true
		secCtx, ok := security.FromContext(ss.Context())
		require.True(t, ok)
		require.NotNil(t, secCtx)
		require.NotNil(t, secCtx.Principal)
		assert.Equal(t, "user-stream", secCtx.Principal.Subject)
		return nil
	}

	err := interceptor.StreamServerInterceptor()(nil, stream, &grpc.StreamServerInfo{}, handler)
	assert.NoError(t, err)
	assert.True(t, called)
}

func TestInterceptor_Stream_Required_Fail(t *testing.T) {
	mgr := &mockSecurityManager{}
	cfg := Config{
		Required:     true,
		EnableBearer: true,
	}
	interceptor := New(mgr, cfg)

	stream := &mockServerStream{ctx: context.Background()}
	handler := func(srv any, ss grpc.ServerStream) error {
		return nil
	}

	err := interceptor.StreamServerInterceptor()(nil, stream, &grpc.StreamServerInfo{}, handler)
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestInterceptor_RequirePermission(t *testing.T) {
	t.Run("Allowed", func(t *testing.T) {
		mgr := &mockSecurityManager{
			authorizeFunc: func(ctx context.Context, permission string) error {
				assert.Equal(t, "read:users", permission)
				return nil
			},
		}
		interceptor := New(mgr, Config{})
		err := interceptor.RequirePermission(context.Background(), "read:users")
		assert.NoError(t, err)
	})

	t.Run("Forbidden", func(t *testing.T) {
		mgr := &mockSecurityManager{
			authorizeFunc: func(ctx context.Context, permission string) error {
				return security.ErrForbidden
			},
		}
		interceptor := New(mgr, Config{})
		err := interceptor.RequirePermission(context.Background(), "write:users")
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.PermissionDenied, st.Code())
	})
}
