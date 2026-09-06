package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	healthcheck "github.com/agilsyofian/gosdk/core/observability/health"
	"github.com/agilsyofian/gosdk/core/observability/logger"
	"github.com/agilsyofian/gosdk/core/observability/tracing"
	securityCore "github.com/agilsyofian/gosdk/security"
	"github.com/agilsyofian/gosdk/transport/server/grpc/interceptor"
	grpcSecurity "github.com/agilsyofian/gosdk/transport/server/grpc/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
)

type serviceRegistration struct {
	sd *grpc.ServiceDesc
	ss any
}

type Server struct {
	mu                  sync.RWMutex
	grpcServer          *grpc.Server
	logger              logger.Logger
	addr                string
	appName             string
	cfg                 Config
	tracing             *tracing.Tracing
	healthRegistry      healthcheck.Registry
	healthServer        *health.Server
	securityManager     securityCore.SecurityManager
	securityInterceptor *grpcSecurity.Interceptor
	interceptors        []grpc.UnaryServerInterceptor
	streamInterceptors  []grpc.StreamServerInterceptor
	opts                []grpc.ServerOption
	services            []serviceRegistration
	started             bool
	inShutdown          atomic.Bool
}

// NewServer creates an enterprise-ready gRPC server instance.
// Call s.Start() to listen and serve, and s.Shutdown(ctx) for graceful shutdown.
func NewServer(
	appName string,
	l logger.Logger,
	cfg Config,
	t *tracing.Tracing,
	opts ...grpc.ServerOption,
) *Server {
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = 30 * time.Second
	}

	s := &Server{
		logger:  l,
		addr:    cfg.Addr,
		appName: appName,
		cfg:     cfg,
		tracing: t,
	}

	s.SetServerOptions(opts...)

	return s
}

// SetHealthRegistry wires transport-agnostic health checks into gRPC.
func (s *Server) SetHealthRegistry(hr healthcheck.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.healthRegistry = hr
	if hr != nil {
		hr.RegisterReadiness(s.appName+"-grpc", healthcheck.CheckFunc(func(ctx context.Context) healthcheck.Health {
			if s.started && !s.inShutdown.Load() {
				return healthcheck.Up()
			}
			return healthcheck.Down()
		}))
	}
}

// SetSecurityManager binds security manager and auto-wires security interceptors.
func (s *Server) SetSecurityManager(mgr securityCore.SecurityManager, cfgs ...grpcSecurity.Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		s.logger.Warn(context.Background(), "cannot modify gRPC security manager after server has started", logger.LogPayload{
			Type:   "GRPC_SERVER",
			Method: "CONFIG",
		})
		return
	}

	s.securityManager = mgr
	var cfg grpcSecurity.Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	} else if s.cfg.Security != nil {
		cfg = *s.cfg.Security
	}
	s.securityInterceptor = grpcSecurity.New(mgr, cfg)
	s.rebuildServer()
}

// HealthServer returns the underlying gRPC standard health server instance.
func (s *Server) HealthServer() *health.Server {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.healthServer
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
	unaryInterceptors := s.buildUnaryInterceptors()
	streamInterceptors := s.buildStreamInterceptors()

	allOpts := buildConfigServerOptions(s.cfg)
	allOpts = append(allOpts, s.opts...)
	allOpts = append(allOpts,
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
	)

	s.grpcServer = grpc.NewServer(allOpts...)

	// Standard health check protocol (grpc.health.v1)
	if s.cfg.HealthCheckEnabled {
		s.healthServer = health.NewServer()
		grpc_health_v1.RegisterHealthServer(s.grpcServer, s.healthServer)
		s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)
		s.healthServer.SetServingStatus(s.appName, grpc_health_v1.HealthCheckResponse_SERVING)
	}

	if s.cfg.ReflectionEnabled {
		reflection.Register(s.grpcServer)
	}

	for _, reg := range s.services {
		s.grpcServer.RegisterService(reg.sd, reg.ss)
	}
}

func (s *Server) buildUnaryInterceptors() []grpc.UnaryServerInterceptor {
	interceptors := []grpc.UnaryServerInterceptor{
		interceptor.Recovery(s.logger, s.appName),
		interceptor.Metadata(s.cfg.Metadata),
		interceptor.Tracer(s.appName),
		interceptor.Logger(s.logger, s.appName),
	}
	if s.securityInterceptor != nil {
		interceptors = append(interceptors, s.securityInterceptor.UnaryServerInterceptor())
	}
	interceptors = append(interceptors, interceptor.Error(s.logger, s.appName, s.cfg.ShowErrorDetail))
	interceptors = append(interceptors, s.interceptors...)
	return interceptors
}

func (s *Server) buildStreamInterceptors() []grpc.StreamServerInterceptor {
	streamInterceptors := []grpc.StreamServerInterceptor{
		interceptor.RecoveryStream(s.logger, s.appName),
		interceptor.MetadataStream(s.cfg.Metadata),
		interceptor.TracerStream(s.appName),
		interceptor.LoggerStream(s.logger, s.appName),
	}
	if s.securityInterceptor != nil {
		streamInterceptors = append(streamInterceptors, s.securityInterceptor.StreamServerInterceptor())
	}
	streamInterceptors = append(streamInterceptors, interceptor.ErrorStream(s.logger, s.appName, s.cfg.ShowErrorDetail))
	streamInterceptors = append(streamInterceptors, s.streamInterceptors...)
	return streamInterceptors
}

func buildConfigServerOptions(cfg Config) []grpc.ServerOption {
	var opts []grpc.ServerOption
	if cfg.MaxConcurrentStreams > 0 {
		opts = append(opts, grpc.MaxConcurrentStreams(cfg.MaxConcurrentStreams))
	}
	if cfg.MaxReceiveMessageSize > 0 {
		opts = append(opts, grpc.MaxRecvMsgSize(cfg.MaxReceiveMessageSize))
	}
	if cfg.MaxSendMessageSize > 0 {
		opts = append(opts, grpc.MaxSendMsgSize(cfg.MaxSendMessageSize))
	}
	if cfg.ConnectionTimeout > 0 {
		opts = append(opts, grpc.ConnectionTimeout(cfg.ConnectionTimeout))
	}
	if cfg.Keepalive != nil {
		kp := keepalive.ServerParameters{
			MaxConnectionIdle:     cfg.Keepalive.MaxConnectionIdle,
			MaxConnectionAge:      cfg.Keepalive.MaxConnectionAge,
			MaxConnectionAgeGrace: cfg.Keepalive.MaxConnectionAgeGrace,
			Time:                  cfg.Keepalive.Time,
			Timeout:               cfg.Keepalive.Timeout,
		}
		kep := keepalive.EnforcementPolicy{
			MinTime:             cfg.Keepalive.MinTime,
			PermitWithoutStream: cfg.Keepalive.PermitWithoutStream,
		}
		opts = append(opts, grpc.KeepaliveParams(kp), grpc.KeepaliveEnforcementPolicy(kep))
	}
	return opts
}

// RegisterService registers a service and its implementation to the gRPC server.
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
	s.inShutdown.Store(true)
	if s.healthServer != nil {
		s.healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_NOT_SERVING)
		s.healthServer.SetServingStatus(s.appName, grpc_health_v1.HealthCheckResponse_NOT_SERVING)
	}

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
