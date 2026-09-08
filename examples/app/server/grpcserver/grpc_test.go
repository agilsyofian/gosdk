package grpcserver

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/example/app/config"
	"github.com/agilsyofian/gosdk/example/proto/user"
	securityCore "github.com/agilsyofian/gosdk/security"
	serverGrpc "github.com/agilsyofian/gosdk/transport/server/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type dummyUserServer struct {
	user.UnimplementedServiceUserServer
}

func (d *dummyUserServer) GetUser(ctx context.Context, req *user.Request) (*user.Response, error) {
	return &user.Response{
		UserId: "user-123",
		Status: "ACTIVE",
	}, nil
}

type mockSecurityManager struct{}

func (m *mockSecurityManager) Authenticate(ctx context.Context, cred securityCore.Credential) (*securityCore.SecurityContext, error) {
	return &securityCore.SecurityContext{
		Authenticated: true,
		Principal: &securityCore.Principal{
			Subject: "test-user",
		},
	}, nil
}

func (m *mockSecurityManager) AuthenticateSession(ctx context.Context, sessionID string) (*securityCore.SecurityContext, error) {
	return &securityCore.SecurityContext{
		Authenticated: true,
		Principal: &securityCore.Principal{
			Subject: "test-user",
		},
	}, nil
}

func (m *mockSecurityManager) Authorize(ctx context.Context, permission string) error {
	return nil
}

func TestGrpcHandler_SetupServer_WiresHealthAndSecurity(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := serverGrpc.DefaultConfig()
	cfg.Addr = "127.0.0.1:0"

	srv := serverGrpc.NewServer("example-app", l, cfg, nil)
	healthReg := health.NewRegistry()
	secMgr := &mockSecurityManager{}
	userHandler := &dummyUserServer{}

	deps := GrpcHandlerDeps{
		Server:      srv,
		Health:      healthReg,
		Security:    secMgr,
		UserHandler: userHandler,
	}

	h := NewGrpcHandler(deps)
	require.NotNil(t, h)

	// Invoke setup
	StartGrpc(h)

	// Verify health check protocol is running and returns SERVING
	healthSrv := srv.HealthServer()
	require.NotNil(t, healthSrv)

	resp, err := healthSrv.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: ""})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)

	resp, err = healthSrv.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: "example-app"})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, resp.Status)
}

func TestGrpcHandler_ConfigVerificationFromEnvYaml(t *testing.T) {
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	_ = os.Chdir("..")

	appCfg, err := config.LoadConfig()
	require.NoError(t, err)

	grpcCfg := appCfg.Server.GrpcServer

	// Phase 5 Config Parity
	assert.Equal(t, ":8081", grpcCfg.Addr)
	assert.True(t, grpcCfg.ReflectionEnabled)
	assert.True(t, grpcCfg.ShowErrorDetail)
	assert.Equal(t, 30*time.Second, grpcCfg.ShutdownTimeout)
	assert.Equal(t, uint32(1000), grpcCfg.MaxConcurrentStreams)
	assert.Equal(t, 16*1024*1024, grpcCfg.MaxReceiveMessageSize)
	assert.Equal(t, 16*1024*1024, grpcCfg.MaxSendMessageSize)
	assert.Equal(t, 120*time.Second, grpcCfg.ConnectionTimeout)
	assert.True(t, grpcCfg.HealthCheckEnabled)

	require.NotNil(t, grpcCfg.Keepalive)
	assert.Equal(t, 15*time.Second, grpcCfg.Keepalive.MaxConnectionIdle)
	assert.Equal(t, 30*time.Second, grpcCfg.Keepalive.MaxConnectionAge)
	assert.Equal(t, 5*time.Second, grpcCfg.Keepalive.MaxConnectionAgeGrace)
	assert.Equal(t, 10*time.Second, grpcCfg.Keepalive.Time)
	assert.Equal(t, 3*time.Second, grpcCfg.Keepalive.Timeout)
	assert.Equal(t, 5*time.Second, grpcCfg.Keepalive.MinTime)
	assert.True(t, grpcCfg.Keepalive.PermitWithoutStream)

	// Phase 6 Declarative Security Config
	require.NotNil(t, grpcCfg.Security)
	assert.True(t, grpcCfg.Security.Required)
	assert.True(t, grpcCfg.Security.EnableBearer)
	assert.False(t, grpcCfg.Security.EnableBasic)
	assert.True(t, grpcCfg.Security.EnableAPIKey)
	assert.Equal(t, "x-api-key", grpcCfg.Security.APIKeyMetadataKey)
	assert.Empty(t, grpcCfg.Security.SessionMetadataKey)
}

func TestGrpcHandler_FullInvocationWithSecurityAndHealth(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := serverGrpc.DefaultConfig()
	cfg.Addr = "127.0.0.1:0"

	srv := serverGrpc.NewServer("example-app", l, cfg, nil)
	healthReg := health.NewRegistry()
	secMgr := &mockSecurityManager{}
	userHandler := &dummyUserServer{}

	deps := GrpcHandlerDeps{
		Server:      srv,
		Health:      healthReg,
		Security:    secMgr,
		UserHandler: userHandler,
	}

	h := NewGrpcHandler(deps)
	h.SetupServer()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer lis.Close()

	go func() {
		_ = srv.Engine().Serve(lis)
	}()

	// Verify health check via standard client
	clientConn, err := grpc.NewClient(lis.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer clientConn.Close()

	healthClient := grpc_health_v1.NewHealthClient(clientConn)
	hResp, err := healthClient.Check(context.Background(), &grpc_health_v1.HealthCheckRequest{Service: ""})
	require.NoError(t, err)
	assert.Equal(t, grpc_health_v1.HealthCheckResponse_SERVING, hResp.Status)

	// Verify user service invocation
	userClient := user.NewServiceUserClient(clientConn)
	uResp, err := userClient.GetUser(context.Background(), &user.Request{
		Username: "alice",
		DeviceId: "device-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "user-123", uResp.UserId)
	assert.Equal(t, "ACTIVE", uResp.Status)

	_ = srv.Shutdown(context.Background())
}
