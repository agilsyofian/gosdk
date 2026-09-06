package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	"github.com/agilsyofian/gosdk/transport/server/grpc/interceptor"
	"go.uber.org/fx"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type serviceRegistration struct {
	sd *grpc.ServiceDesc
	ss any
}

type Server struct {
	mu                 sync.RWMutex
	grpcServer         *grpc.Server
	logger             logger.Logger
	addr               string
	appName            string
	cfg                Config
	tracing            *tracing.Tracing
	interceptors       []grpc.UnaryServerInterceptor
	streamInterceptors []grpc.StreamServerInterceptor
	opts               []grpc.ServerOption
	services           []serviceRegistration
	started            bool
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
					l.Error(ctx, "gRPC server failed to start", logger.LogPayload{
						Type:      "GRPC_SERVER",
						Method:    "START",
						IpAddress: s.addr,
						Error:     err.Error(),
					})
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
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.logger.Warn(context.Background(), "cannot modify gRPC interceptors after server has started", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "CONFIG",
		})
		return
	}

	s.interceptors = append(s.interceptors, interceptors...)
	s.rebuildServer()
}

// UseStream appends stream interceptors to the gRPC server pipeline.
func (s *Server) UseStream(interceptors ...grpc.StreamServerInterceptor) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.logger.Warn(context.Background(), "cannot modify gRPC stream interceptors after server has started", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "CONFIG",
		})
		return
	}

	s.streamInterceptors = append(s.streamInterceptors, interceptors...)
	s.rebuildServer()
}

func (s *Server) SetServerOptions(opts ...grpc.ServerOption) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.logger.Warn(context.Background(), "cannot modify gRPC options after server has started", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "CONFIG",
		})
		return
	}

	s.opts = opts
	s.rebuildServer()
}

func (s *Server) rebuildServer() {
	// Re-add default interceptors to ensure they are always present
	allInterceptors := []grpc.UnaryServerInterceptor{
		interceptor.Recovery(s.logger, s.appName),
		interceptor.Metadata(s.cfg.Metadata),
		interceptor.Tracer(s.appName),
		interceptor.Logger(s.logger, s.appName),
		interceptor.Error(s.logger, s.appName, s.cfg.ShowErrorDetail),
	}
	allInterceptors = append(allInterceptors, s.interceptors...)

	allStreamInterceptors := []grpc.StreamServerInterceptor{
		interceptor.RecoveryStream(s.logger, s.appName),
		interceptor.MetadataStream(s.cfg.Metadata),
		interceptor.TracerStream(s.appName),
		interceptor.LoggerStream(s.logger, s.appName),
		interceptor.ErrorStream(s.logger, s.appName, s.cfg.ShowErrorDetail),
	}
	allStreamInterceptors = append(allStreamInterceptors, s.streamInterceptors...)

	opts := append([]grpc.ServerOption{}, s.opts...)
	opts = append(opts,
		grpc.ChainUnaryInterceptor(allInterceptors...),
		grpc.ChainStreamInterceptor(allStreamInterceptors...),
	)

	s.grpcServer = grpc.NewServer(opts...)

	// Re-register reflection if it was enabled in the original config
	if s.cfg.ReflectionEnabled {
		reflection.Register(s.grpcServer)
	}

	// Re-register all previously registered services
	for _, reg := range s.services {
		s.grpcServer.RegisterService(reg.sd, reg.ss)
	}
}

// RegisterService registers a service and its implementation to the gRPC server.
// Service registrations are persisted so they survive any subsequent interceptor or option rebuilds.
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.services = append(s.services, serviceRegistration{sd: sd, ss: ss})
	if s.grpcServer != nil {
		s.grpcServer.RegisterService(sd, ss)
	}
}

// Engine returns the underlying *grpc.Server instance.
func (s *Server) Engine() *grpc.Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.grpcServer
}

// GRPCServer returns the underlying *grpc.Server instance (alias for Engine()).
func (s *Server) GRPCServer() *grpc.Server {
	return s.Engine()
}

func (s *Server) Start() error {
	s.mu.Lock()
	s.started = true
	srv := s.grpcServer
	s.mu.Unlock()

	lis, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("gRPC listen: %w", err)
	}

	s.logger.Info(context.Background(), "starting gRPC server", logger.LogPayload{
		Type:      "GRPC_SERVER",
		Method:    "START",
		IpAddress: s.addr,
	})
	if err = srv.Serve(lis); err != nil {
		return fmt.Errorf("gRPC server: %w", err)
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info(ctx, "stopping gRPC server", logger.LogPayload{
		Type:   "GRPC_SERVER",
		Method: "SHUTDOWN",
	})

	s.mu.RLock()
	srv := s.grpcServer
	s.mu.RUnlock()

	if srv == nil {
		return nil
	}

	timeout := s.cfg.ShutdownTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	shutdownCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		s.logger.Info(ctx, "gRPC server stopped gracefully", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "SHUTDOWN",
		})
		return nil
	case <-shutdownCtx.Done():
		s.logger.Warn(ctx, "grace period exceeded, forcing gRPC server stop", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "SHUTDOWN",
		})
		srv.Stop()
		return shutdownCtx.Err()
	}
}
