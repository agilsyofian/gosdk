package grpc

import (
	"context"
	"testing"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"go.uber.org/fx/fxtest"
	"google.golang.org/grpc"
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
