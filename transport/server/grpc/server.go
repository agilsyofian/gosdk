package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/observability/logger"
	"github.com/agilsyofian/gosdk/transport/server/grpc/interceptor"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type Server struct {
	grpcServer   *grpc.Server
	logger       logger.Logger
	addr         string
	appName      string
	cfg          Config
	tracing      *tracing.Tracing
	interceptors []grpc.UnaryServerInterceptor
	opts         []grpc.ServerOption
}

func NewServer(
	appName string,
	lc fx.Lifecycle,
	l logger.Logger,
	cfg Config,
	t *tracing.Tracing,
	opts ...grpc.ServerOption,
) *Server {
	s := &Server{
		logger:  l,
		addr:    cfg.Addr,
		appName: appName,
		cfg:     cfg,
		tracing: t,
	}

	s.SetServerOptions(opts...)

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := s.Start(); err != nil {
					l.Error(ctx, "http server failed to start", logger.LogPayload{})
				}
			}()
			return nil
		},
		OnStop: func(ctx context.Context) error {
			if t != nil {
				_ = t.Shutdown(ctx)
			}
			return s.Shutdown(ctx)
		},
	})

	return s
}

func (s *Server) Use(interceptors ...grpc.UnaryServerInterceptor) {
	s.interceptors = append(s.interceptors, interceptors...)
	s.rebuildServer()
}

func (s *Server) SetServerOptions(opts ...grpc.ServerOption) {
	s.opts = opts
	s.rebuildServer()
}

func (s *Server) rebuildServer() {
	// Re-add default interceptors to ensure they are always present
	allInterceptors := []grpc.UnaryServerInterceptor{
		interceptor.Recovery(s.appName),
		interceptor.Metadata(),
		interceptor.Tracer(s.appName),
		interceptor.Logger(s.logger, s.appName),
		interceptor.Error(s.logger, s.appName, s.cfg.ShowErrorDetail),
	}
	allInterceptors = append(allInterceptors, s.interceptors...)

	opts := append([]grpc.ServerOption{}, s.opts...)
	opts = append(opts, grpc.ChainUnaryInterceptor(allInterceptors...))

	s.grpcServer = grpc.NewServer(opts...)

	// Re-register reflection if it was enabled in the original config
	if s.cfg.ReflectionEnabled {
		reflection.Register(s.grpcServer)
	}
}

func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss any) {
	s.grpcServer.RegisterService(sd, ss)
}

func (s *Server) Start() error {
	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}

	s.logger.Info(context.Background(), "starting gRPC server", logger.LogPayload{
		Type:      "GRPC_SERVER",
		Method:    "START",
		IpAddress: s.addr,
	})
	if err = s.grpcServer.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info(ctx, "stopping gRPC server", logger.LogPayload{
		Type:   "GRPC_SERVER",
		Method: "SHUTDOWN",
	})
	s.grpcServer.GracefulStop()
	return nil
}
