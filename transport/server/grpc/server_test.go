package grpc

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"go.uber.org/fx/fxtest"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestServer_SetServerOptions_Panic(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr: ":0",
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)

	// This should not panic now as it replaces s.opts and rebuilds
	s.SetServerOptions(grpc.MaxRecvMsgSize(1024))
}

func TestServer_Use_Interceptor(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr: ":0",
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)

	// This should properly chain interceptors
	s.Use(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	})
}

func TestServer_UseStream_Interceptor(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr: ":0",
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)

	s.UseStream(func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return handler(srv, ss)
	})
}

func TestServer_Shutdown_Graceful(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr:            ":0",
		ShutdownTimeout: 2 * time.Second,
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	go func() {
		_ = s.grpcServer.Serve(lis)
	}()

	// Graceful shutdown with no active requests should succeed immediately
	err = s.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on graceful shutdown, got: %v", err)
	}
}

func TestServer_Shutdown_TimeoutFallback(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr:            ":0",
		ShutdownTimeout: 50 * time.Millisecond,
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)

	// Register a dummy streaming service that blocks until released
	blockCh := make(chan struct{})
	defer close(blockCh)

	handlerEntered := make(chan struct{})
	sd := &grpc.ServiceDesc{
		ServiceName: "test.BlockingService",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Block",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/test.BlockingService/Block",
					}
					handler := func(ctx context.Context, req any) (any, error) {
						close(handlerEntered)
						<-blockCh
						return &emptypb.Empty{}, nil
					}
					if interceptor != nil {
						return interceptor(ctx, &emptypb.Empty{}, info, handler)
					}
					return handler(ctx, &emptypb.Empty{})
				},
			},
		},
	}

	s.RegisterService(sd, struct{}{})

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	go func() {
		_ = s.grpcServer.Serve(lis)
	}()

	// Connect client to trigger in-flight blocking call
	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	invokeErrCh := make(chan error, 1)
	go func() {
		var in, out emptypb.Empty
		err := conn.Invoke(context.Background(), "/test.BlockingService/Block", &in, &out)
		invokeErrCh <- err
	}()

	select {
	case <-handlerEntered:
	case err := <-invokeErrCh:
		t.Fatalf("invoke returned immediately with error: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("handler was not entered")
	}

	// Trigger shutdown with short timeout
	start := time.Now()
	err = s.Shutdown(context.Background())
	duration := time.Since(start)

	if err == nil {
		t.Fatal("expected error due to shutdown timeout fallback, got nil")
	}
	if duration > 1*time.Second {
		t.Fatalf("shutdown took too long (%v), timeout fallback didn't kick in", duration)
	}
}

func TestServer_ServicePreservationAcrossRebuild(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr:            ":0",
		ShutdownTimeout: 2 * time.Second,
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)

	serviceCalled := false
	sd := &grpc.ServiceDesc{
		ServiceName: "test.EchoService",
		HandlerType: (*any)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "Echo",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					info := &grpc.UnaryServerInfo{
						Server:     srv,
						FullMethod: "/test.EchoService/Echo",
					}
					handler := func(ctx context.Context, req any) (any, error) {
						serviceCalled = true
						return &emptypb.Empty{}, nil
					}
					if interceptor != nil {
						return interceptor(ctx, &emptypb.Empty{}, info, handler)
					}
					return handler(ctx, &emptypb.Empty{})
				},
			},
		},
	}

	// 1. Register service BEFORE calling Use or SetServerOptions
	s.RegisterService(sd, struct{}{})

	// 2. Call Use & SetServerOptions (which trigger rebuildServer)
	customInterceptorCalled := false
	s.Use(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		customInterceptorCalled = true
		return handler(ctx, req)
	})
	s.SetServerOptions(grpc.MaxRecvMsgSize(4 * 1024 * 1024))

	// Verify Engine accessor works
	if s.Engine() == nil || s.GRPCServer() == nil {
		t.Fatal("expected non-nil grpcServer from Engine/GRPCServer")
	}

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	go func() {
		_ = s.Engine().Serve(lis)
	}()

	conn, err := grpc.Dial(lis.Addr().String(), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		t.Fatalf("failed to dial: %v", err)
	}
	defer conn.Close()

	var in, out emptypb.Empty
	err = conn.Invoke(context.Background(), "/test.EchoService/Echo", &in, &out)
	if err != nil {
		t.Fatalf("expected successful invoke, got: %v", err)
	}

	if !serviceCalled {
		t.Fatal("expected service to be preserved and called across server rebuilds")
	}
	if !customInterceptorCalled {
		t.Fatal("expected custom interceptor from Use() to be invoked")
	}

	_ = s.Shutdown(context.Background())
}

func TestServer_PostStartModificationProtection(t *testing.T) {
	l := logger.NewZerologLogger(logger.Config{Output: "stdout"})
	cfg := Config{
		Addr:            ":0",
		ShutdownTimeout: 2 * time.Second,
	}

	s := NewServer("test-app", fxtest.NewLifecycle(t), l, cfg, nil)
	s.started = true

	// Modifying options after start should be safely rejected
	s.Use(func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		return handler(ctx, req)
	})
	s.SetServerOptions(grpc.MaxRecvMsgSize(1024))
}
